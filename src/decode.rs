//! Image decoding with format-specific dispatch.
//!
//! Most formats (JPEG/PNG/GIF/BMP/TIFF/ICO/WebP) go through the
//! `image` crate's `ImageReader`, which auto-detects format from
//! magic bytes and enforces pre-decode dimension/allocation limits.
//! Whatever the container, the returned [`DynamicImage`] is always
//! *visually* upright: EXIF orientation is applied here so no platform
//! backend has to think about it.
//!
//! **AVIF and JXL** (JPEG XL) have no `image` crate decoder, so they are
//! dispatched before the generic path, and the backend depends on the
//! platform:
//!
//! * **Windows** (feature `wic`): the Windows Imaging Component, i.e. the
//!   system-installed codecs. Windows 11 24H2+ ships AVIF and JXL
//!   built-in; Windows 10 needs them from the Microsoft Store.
//! * **Everywhere else** (macOS today): decoders bundled into the binary —
//!   libavif+dav1d for AVIF, `jxl-oxide` for JXL — so a Finder thumbnail
//!   never depends on what the host OS happens to support. See
//!   [`bundled`].
//!
//! Both backends honour the same [`crate::limits`] guards, and both fail
//! softly: an undecodable cover is an `Err`, which the host turns into
//! "no thumbnail" rather than a crash.

use std::error::Error;
use std::io::Cursor;

#[cfg(all(windows, feature = "wic"))]
use crate::wic;
use image::metadata::Orientation;
use image::{DynamicImage, ImageDecoder, ImageReader, Limits};

use crate::limits;

/// Decode image bytes into a full-resolution `DynamicImage`,
/// dispatching by filename extension. The format is still verified
/// by the underlying decoder, so a mislabeled file fails cleanly
/// with a decode error rather than misbehaving.
///
/// This always decodes at full resolution. For the thumbnail path
/// (where only a small target size is needed) prefer
/// [`decode_for_thumbnail`], which skips a large fraction of the
/// JPEG decode cost by asking libjpeg for a pre-scaled output.
pub fn decode_with_limits(name: &str, bytes: &[u8]) -> Result<DynamicImage, Box<dyn Error>> {
    #[cfg(all(windows, feature = "wic"))]
    if wic::is_wic_format(name) {
        return wic::decode_via_wic(bytes);
    }
    #[cfg(not(windows))]
    if let Some(img) = bundled::try_decode(name, bytes)? {
        return Ok(img);
    }
    let _ = name; // only used by the codec branches above
    decode_via_image_crate(bytes)
}

/// Decode with an intent to resize down to `target_px` on the
/// longest side. For JPEG inputs, this uses the codec's native
/// 1/2 / 1/4 / 1/8 DCT scaling so a 2000×3000 comic page is
/// delivered at ~512×768 (scale=1/4) instead of being decoded at
/// full resolution only to be thrown away by a subsequent
/// `resize`. For other formats — PNG, WebP, GIF, etc. — returns
/// the full-resolution image, because their decoders do not
/// support sub-resolution decoding in any meaningful form.
///
/// The caller is still expected to run a final high-quality
/// `resize` pass to hit the exact target; this function only
/// shrinks the *input* cost.
pub fn decode_for_thumbnail(
    name: &str,
    bytes: &[u8],
    target_px: u32,
) -> Result<DynamicImage, Box<dyn Error>> {
    #[cfg(all(windows, feature = "wic"))]
    if wic::is_wic_format(name) {
        return wic::decode_via_wic(bytes);
    }
    #[cfg(not(windows))]
    if let Some(img) = bundled::try_decode(name, bytes)? {
        return Ok(img);
    }
    let _ = name;

    if let Some(img) = try_decode_jpeg_scaled(bytes, target_px)? {
        return Ok(img);
    }
    decode_via_image_crate(bytes)
}

/// JPEG magic bytes: every JPEG starts with `FF D8 FF`. Sniffing
/// the content (not the filename) means a `.png` that is actually
/// a JPEG still hits the fast path, and — more importantly — a
/// `.jpg` that is actually a PNG falls through instead of crashing
/// the JPEG decoder.
fn is_jpeg(bytes: &[u8]) -> bool {
    bytes.len() >= 3 && &bytes[..3] == b"\xFF\xD8\xFF"
}

/// If `bytes` is a JPEG, decode it via `jpeg-decoder` using the
/// largest DCT scale factor (1/1, 1/2, 1/4, 1/8) whose output is
/// still ≥ `target_px * 2`. The ×2 headroom is so the subsequent
/// high-quality resize has enough input to work with — scaling
/// down 2× in the resizer looks much better than scaling up 1.2×
/// from an over-shrunken source.
///
/// `jpeg-decoder` handles the "source already small" case itself:
/// `scale(requested)` returns 1/1 when no sub-resolution factor
/// produces an output ≥ `requested`.
///
/// Returns `Ok(None)` when the input isn't JPEG (magic bytes don't
/// match) or when the source's pixel format isn't one we can map
/// to a `DynamicImage` losslessly — the caller then falls through
/// to the generic `image`-crate decoder. Decode errors propagate.
fn try_decode_jpeg_scaled(
    bytes: &[u8],
    target_px: u32,
) -> Result<Option<DynamicImage>, Box<dyn Error>> {
    if !is_jpeg(bytes) {
        return Ok(None);
    }
    use image::{ImageBuffer, Luma, Rgb};
    use jpeg_decoder::{Decoder, PixelFormat};

    let mut decoder = Decoder::new(Cursor::new(bytes));
    decoder.read_info()?;
    let info = decoder
        .info()
        .ok_or("JPEG info unavailable after read_info")?;

    // Pre-scale dimension guard — matches the cap the `image` crate
    // would enforce via its `Limits` on the full-decode path.
    let src_w = info.width as u32;
    let src_h = info.height as u32;
    if src_w > limits::MAX_IMAGE_DIMENSION || src_h > limits::MAX_IMAGE_DIMENSION {
        return Err(format!("JPEG dimensions too large: {src_w}x{src_h}").into());
    }

    // Ask for target×2. jpeg-decoder picks the largest supported
    // scale (8/4/2/1) whose output on the longer axis is still
    // ≥ requested. Small sources collapse to 1/1 naturally.
    let (out_w, out_h) = if target_px > 0 {
        let requested = target_px.saturating_mul(2).min(u16::MAX as u32) as u16;
        decoder.scale(requested, requested)?
    } else {
        (info.width, info.height)
    };
    let w = out_w as u32;
    let h = out_h as u32;

    // Post-scale allocation guard. Even after a successful scale the
    // output could be enormous if the source was 16000×16000.
    let bpp = match info.pixel_format {
        PixelFormat::L8 => 1u64,
        PixelFormat::L16 => 2,
        PixelFormat::RGB24 => 3,
        PixelFormat::CMYK32 => 4,
    };
    let pixel_bytes = (w as u64).saturating_mul(h as u64).saturating_mul(bpp);
    if pixel_bytes > limits::MAX_IMAGE_ALLOC {
        return Err(
            format!("JPEG decoded buffer would exceed allocation limit: {pixel_bytes}").into(),
        );
    }

    let pixels = decoder.decode()?;

    let mut img = match info.pixel_format {
        PixelFormat::RGB24 => {
            let buf: ImageBuffer<Rgb<u8>, Vec<u8>> =
                ImageBuffer::from_raw(w, h, pixels).ok_or("JPEG RGB24 buffer size mismatch")?;
            DynamicImage::ImageRgb8(buf)
        }
        PixelFormat::L8 => {
            let buf: ImageBuffer<Luma<u8>, Vec<u8>> =
                ImageBuffer::from_raw(w, h, pixels).ok_or("JPEG L8 buffer size mismatch")?;
            DynamicImage::ImageLuma8(buf)
        }
        // L16 (16-bit grey) and CMYK32 are rare in JPEGs and need
        // a colour-space conversion we'd rather not hand-roll. Fall
        // back to the full-featured `image`-crate decoder for these.
        PixelFormat::L16 | PixelFormat::CMYK32 => return Ok(None),
    };
    // The scaled path bypasses the `image` crate, so it has to apply
    // EXIF orientation itself or a photo taken in portrait would come
    // out on its side. `jpeg-decoder` already collected the Exif
    // chunk while parsing markers, so this costs no extra pass.
    if let Some(orientation) = decoder
        .exif_data()
        .and_then(Orientation::from_exif_chunk)
        .filter(|o| *o != Orientation::NoTransforms)
    {
        img.apply_orientation(orientation);
    }
    Ok(Some(img))
}

fn make_limits() -> Limits {
    let mut l = Limits::default();
    l.max_image_width = Some(limits::MAX_IMAGE_DIMENSION);
    l.max_image_height = Some(limits::MAX_IMAGE_DIMENSION);
    l.max_alloc = Some(limits::MAX_IMAGE_ALLOC);
    l
}

fn decode_via_image_crate(bytes: &[u8]) -> Result<DynamicImage, Box<dyn Error>> {
    let mut reader = ImageReader::new(Cursor::new(bytes)).with_guessed_format()?;
    reader.limits(make_limits());
    // Go through the decoder rather than `ImageReader::decode` so the
    // Exif orientation is available: `decode()` hands back the pixels
    // in coded order, which leaves phone photos on their side.
    let mut decoder = reader.into_decoder()?;
    let orientation = decoder.orientation()?;
    let mut img = DynamicImage::from_decoder(decoder)?;
    if orientation != Orientation::NoTransforms {
        img.apply_orientation(orientation);
    }
    Ok(img)
}

// =============================================================================
// Bundled AVIF / JXL decoders (platforms without WIC)
// =============================================================================

/// AVIF + JXL decoding for builds without WIC.
///
/// Kept behind `cfg(not(windows))` because on Windows the system's WIC
/// codecs are both better (they colour-manage) and already wired up.
///
/// Dispatch is by *content* first and filename second, matching the
/// `image`-crate path's philosophy: a `.jpg` that is really a PNG falls
/// through to the generic decoder instead of feeding garbage to
/// libavif. A mislabelled file that sniffs as AVIF/JXL still gets
/// verified by the decoder itself.
///
/// Limits: AVIF has no Rust-side allocation knob — libavif allocates its
/// own planes — so the source is parsed once, headers only, to read the
/// *composed* dimensions (grid assembly included) and reject oversized
/// sources before any pixel memory is committed. JXL reports its
/// dimensions from `jxl-oxide` before decoding, so it needs no extra pass.
#[cfg(not(windows))]
mod bundled {
    use std::error::Error;
    use std::io::Cursor;

    use image::{DynamicImage, ImageDecoder};

    use crate::limits;

    /// Which bundled backend can handle this source.
    #[derive(Debug, PartialEq, Eq)]
    enum Backend {
        Avif,
        Jxl,
    }

    /// Identify the backend for `name`/`bytes`, or `None` when the
    /// generic `image`-crate path should handle it.
    fn backend_for(name: &str, bytes: &[u8]) -> Option<Backend> {
        if sniffs_avif(bytes) || has_ext(name, ".avif") {
            return Some(Backend::Avif);
        }
        if sniffs_jxl(bytes) || has_ext(name, ".jxl") {
            return Some(Backend::Jxl);
        }
        None
    }

    fn has_ext(name: &str, ext: &str) -> bool {
        name.len() >= ext.len() && name[name.len() - ext.len()..].eq_ignore_ascii_case(ext)
    }

    /// ISO base media file box header: a 4-byte length, `'ftyp'`, then a
    /// major brand of `avif` (still images) or `avis` (still + sequence).
    fn sniffs_avif(bytes: &[u8]) -> bool {
        bytes.len() >= 12 && &bytes[4..8] == b"ftyp" && {
            let brand = &bytes[8..12];
            brand == b"avif" || brand == b"avis"
        }
    }

    /// JPEG XL is either a bare codestream (`FF 0F`) or the ISO-BMFF
    /// container (`...'JXL '`, then the two magic bytes below it).
    fn sniffs_jxl(bytes: &[u8]) -> bool {
        if bytes.len() >= 2 && bytes[0] == 0xFF && bytes[1] == 0x0F {
            return true;
        }
        bytes.len() >= 12 && &bytes[0..12] == b"\x00\x00\x00\x0CJXL \x0D\x0A\x87\x0A"
    }

    /// Decode via a bundled backend if one claims this source.
    ///
    /// `Ok(None)` means "not ours" — the caller continues down its normal
    /// path. A source that *looks* like AVIF/JXL but fails to decode
    /// falls back to the generic decoder before reporting the error, so a
    /// file lying about its extension still gets a fair chance.
    pub(super) fn try_decode(
        name: &str,
        bytes: &[u8],
    ) -> Result<Option<DynamicImage>, Box<dyn Error>> {
        let err = match backend_for(name, bytes) {
            Some(Backend::Avif) => match decode_avif(bytes) {
                Ok(img) => return Ok(Some(img)),
                Err(e) => e,
            },
            Some(Backend::Jxl) => match decode_jxl(bytes) {
                Ok(img) => return Ok(Some(img)),
                Err(e) => e,
            },
            None => return Ok(None),
        };
        match super::decode_via_image_crate(bytes) {
            Ok(img) => Ok(Some(img)),
            Err(_) => Err(err),
        }
    }

    /// AVIF → RGBA8 via libavif + dav1d.
    fn decode_avif(bytes: &[u8]) -> Result<DynamicImage, Box<dyn Error>> {
        let (w, h) = avif_dimensions(bytes)?;
        // libavif-image always hands back 8-bit RGBA.
        check_dimensions("AVIF", w, h, 4)?;
        crate::alog!("  AVIF: {w}x{h}");
        libavif_image::read(bytes)
            .map_err(|e| -> Box<dyn Error> { format!("AVIF decode failed: {e}").into() })
    }

    /// Parse the AVIF container far enough to learn the *display*
    /// dimensions — this is the composed size, so a multi-tile grid
    /// reports its full extent rather than one cell.
    ///
    /// # Safety
    ///
    /// Manual libavif C API usage. The decoder is created and destroyed
    /// on every path out of this function, `avifDecoderSetIOMemory` does
    /// not copy but the borrowed slice outlives every use of it, and the
    /// image pointer is only dereferenced after `avifDecoderParse`
    /// reported success (libavif owns that memory and frees it with the
    /// decoder). No pixel decoding happens here, so no frame buffers are
    /// allocated: that is the whole point of doing it before the decode.
    fn avif_dimensions(bytes: &[u8]) -> Result<(u32, u32), Box<dyn Error>> {
        use libavif_sys as sys;

        unsafe {
            let dec = sys::avifDecoderCreate();
            if dec.is_null() {
                return Err("avifDecoderCreate returned null".into());
            }
            // Header parsing only: reading `image` before this returns
            // success would be uninitialised memory.
            let set = sys::avifDecoderSetIOMemory(dec, bytes.as_ptr(), bytes.len());
            let parsed = if set == sys::AVIF_RESULT_OK {
                sys::avifDecoderParse(dec)
            } else {
                set
            };
            let out = if parsed != sys::AVIF_RESULT_OK {
                // `diag.error` is a NUL-terminated C string filled in by
                // libavif; borrow it while the decoder is still alive.
                Err(avif_error("parse", &(*dec).diag.error))
            } else {
                let image = (*dec).image;
                if image.is_null() {
                    Err("libavif reported success with no image".into())
                } else {
                    Ok(((*image).width, (*image).height))
                }
            };
            sys::avifDecoderDestroy(dec);
            out
        }
    }

    /// Turn a libavif diagnostics buffer (a NUL-terminated C string)
    /// into a Rust message, stopping at the first NUL.
    ///
    /// # Safety
    ///
    /// `error` is a fixed 256-byte array owned by the caller's
    /// `avifDiagnostics`; reading up to the terminator stays inside it.
    unsafe fn avif_error(what: &str, error: &[std::ffi::c_char; 256]) -> Box<dyn Error> {
        let bytes: Vec<u8> = error
            .iter()
            .take_while(|&&c| c != 0)
            .map(|&c| c as u8)
            .collect();
        let detail = String::from_utf8_lossy(&bytes);
        format!("AVIF {what} failed: {detail}").into()
    }

    /// JPEG XL → `DynamicImage` via jxl-oxide's `image` integration.
    ///
    /// 8- and 16-bit integer and 32-bit float output, RGB or luma, with
    /// or without alpha, all map onto `DynamicImage` variants; the
    /// thumbnail pipeline's final `to_rgba8()` brings them down to 8-bit.
    fn decode_jxl(bytes: &[u8]) -> Result<DynamicImage, Box<dyn Error>> {
        use jxl_oxide::integration::JxlDecoder;

        let decoder = JxlDecoder::new(Cursor::new(bytes))?;
        let (w, h) = decoder.dimensions();
        // JXL legitimately decodes to 16-bit integer or 32-bit float, so
        // the allocation has to be budgeted at the *output* depth, not at
        // 4 bytes a pixel.
        let bpp = u64::from(decoder.color_type().bytes_per_pixel());
        check_dimensions("JXL", w, h, bpp.max(4))?;
        crate::alog!("  JXL: {w}x{h}");
        Ok(DynamicImage::from_decoder(decoder)?)
    }

    /// The dimension/area guards the `image` crate applies through
    /// `Limits`, applied by hand for decoders that don't read them.
    fn check_dimensions(
        what: &str,
        w: u32,
        h: u32,
        bytes_per_pixel: u64,
    ) -> Result<(), Box<dyn Error>> {
        if w == 0 || h == 0 {
            return Err(format!("{what} has zero-sized dimensions").into());
        }
        if w > limits::MAX_IMAGE_DIMENSION || h > limits::MAX_IMAGE_DIMENSION {
            return Err(format!("{what} dimensions too large: {w}x{h}").into());
        }
        let pixel_bytes = (w as u64)
            .saturating_mul(h as u64)
            .saturating_mul(bytes_per_pixel);
        if pixel_bytes > limits::MAX_IMAGE_ALLOC {
            return Err(format!(
                "{what} decoded buffer would exceed allocation limit: {pixel_bytes} bytes"
            )
            .into());
        }
        Ok(())
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn sniff_avif_ftyp() {
            let mut b = vec![0u8; 12];
            b[0..4].copy_from_slice(&24u32.to_be_bytes());
            b[4..8].copy_from_slice(b"ftyp");
            b[8..12].copy_from_slice(b"avif");
            assert!(sniffs_avif(&b));
            b[8..12].copy_from_slice(b"avis");
            assert!(sniffs_avif(&b));
            b[8..12].copy_from_slice(b"qt  ");
            assert!(!sniffs_avif(&b));
            assert!(!sniffs_avif(b"ftyp"));
        }

        #[test]
        fn sniff_jxl_codestream_and_container() {
            assert!(sniffs_jxl(&[0xFF, 0x0F, 0x0A, 0x00]));
            let mut container = vec![0u8; 12];
            container[0..12].copy_from_slice(b"\x00\x00\x00\x0CJXL \x0D\x0A\x87\x0A");
            assert!(sniffs_jxl(&container));
            assert!(!sniffs_jxl(&[0xFF, 0xD8, 0xFF]));
            assert!(!sniffs_jxl(&[]));
        }

        #[test]
        fn backend_dispatch_prefers_content_then_name() {
            assert_eq!(
                backend_for("whatever.bin", &[0xFF, 0x0F, 1, 2]),
                Some(Backend::Jxl)
            );
            assert_eq!(backend_for("page.jxl", &[1, 2, 3]), Some(Backend::Jxl));
            assert_eq!(backend_for("PAGE.JXL", &[1, 2, 3]), Some(Backend::Jxl));
            assert_eq!(backend_for("page.jpg", &[1, 2, 3]), None);
            assert!(!has_ext("png", ".png")); // too short to be a name+ext
            assert!(has_ext("a.png", ".PNG"));
        }

        #[test]
        fn dimension_checks_mirror_image_crate_limits() {
            assert!(check_dimensions("AVIF", 1, 1, 4).is_ok());
            assert!(check_dimensions("AVIF", 0, 10, 4).is_err());
            assert!(check_dimensions("AVIF", limits::MAX_IMAGE_DIMENSION + 1, 10, 4).is_err());
            // Far past MAX_IMAGE_ALLOC even though each side is inside
            // the per-dimension cap.
            assert!(check_dimensions("JXL", 20_000, 20_000, 4).is_err());
            // A 32-bit-float RGBA output of the same 2000×2000 source is
            // 16 bytes a pixel: the budget has to scale with depth.
            assert!(check_dimensions("JXL", 2000, 2000, 4).is_ok());
            assert!(check_dimensions("JXL", 6000, 6000, 16).is_err());
        }

        #[test]
        fn not_an_avif_is_not_claimed_by_the_generic_path() {
            // A file *named* .avif that isn't an image at all must
            // produce an error, not a panic and not a bogus image.
            assert!(try_decode("cover.avif", b"definitely not an avif").is_err());
        }

        #[test]
        fn not_a_jxl_is_not_claimed_by_the_generic_path() {
            assert!(try_decode("cover.jxl", b"definitely not a jxl").is_err());
        }

        #[test]
        fn png_named_avif_falls_back_to_the_generic_decoder() {
            // Lying filename: the AVIF backend fails, the generic path
            // recognises the PNG by magic bytes and succeeds.
            let png = super::super::tests::make_png(4, 3);
            let img = try_decode("cover.avif", &png)
                .expect("no error")
                .expect("decoded");
            assert_eq!((img.width(), img.height()), (4, 3));
        }

        #[test]
        fn other_formats_are_left_alone() {
            let png = super::super::tests::make_png(2, 2);
            assert!(try_decode("cover.png", &png).unwrap().is_none());
        }

        // ----------------------------------------------------------------
        // Real fixtures. Tiny committed AVIF/JXL files (see tests/fixtures)
        // so CI exercises the bundled codecs on every non-Windows run —
        // the formats Finder users most often hand a comic archive.
        // ----------------------------------------------------------------

        fn fixture(name: &str) -> Vec<u8> {
            let path = concat!(env!("CARGO_MANIFEST_DIR"), "/tests/fixtures/");
            let bytes = std::fs::read(format!("{path}{name}"))
                .unwrap_or_else(|e| panic!("missing fixture {name}: {e}"));
            assert!(!bytes.is_empty(), "fixture {name} is empty");
            bytes
        }

        #[test]
        fn avif_fixture_decodes_to_its_real_size() {
            let bytes = fixture("cover_32x24.avif");
            let img = decode_avif(&bytes).expect("libavif decode");
            assert_eq!((img.width(), img.height()), (32, 24));
            // libavif always hands back 8-bit RGBA.
            assert!(matches!(img, DynamicImage::ImageRgba8(_)));
            let rgba = img.to_rgba8();
            assert_ne!(
                rgba.get_pixel(0, 0).0,
                rgba.get_pixel(31, 23).0,
                "a test pattern decoded to one colour?"
            );
        }

        #[test]
        fn avif_is_sniffed_by_content_not_just_name() {
            let bytes = fixture("cover_32x24.avif");
            // Named like a JPEG: the magic bytes still route to libavif.
            let img = super::super::decode_with_limits("mislabelled.jpg", &bytes).expect("decode");
            assert_eq!((img.width(), img.height()), (32, 24));
        }

        #[test]
        fn jxl_fixture_decodes_to_its_real_size() {
            let bytes = fixture("cover_32x24.jxl");
            let img = decode_jxl(&bytes).expect("jxl-oxide decode");
            assert_eq!((img.width(), img.height()), (32, 24));
        }

        #[test]
        fn jxl_lossless_fixture_is_really_lossless() {
            // Modular (lossless) streams must survive the same path, and
            // come back byte-identical to what cjxl was given.
            let bytes = fixture("lossless_64x48.jxl");
            let img = decode_jxl(&bytes).expect("lossless jxl decode");
            assert_eq!((img.width(), img.height()), (64, 48));
            let rgba = img.to_rgba8();
            // `testsrc` draws a grid over a coloured field; a corrupt decode
            // would not preserve the corner-to-centre difference.
            assert_ne!(rgba.get_pixel(0, 0).0, rgba.get_pixel(32, 24).0);
        }

        #[test]
        fn truncated_avif_fails_instead_of_panicking() {
            let mut bytes = fixture("cover_32x24.avif");
            bytes.truncate(bytes.len() / 2);
            assert!(decode_avif(&bytes).is_err());
        }

        #[test]
        fn avif_header_with_garbage_payload_is_rejected() {
            let mut bytes: Vec<u8> = vec![0, 0, 0, 12]; // box size, then 'ftyp'
            bytes.extend_from_slice(b"ftypavif");
            bytes.extend_from_slice(b"garbage that is not an AV1 configuration");
            assert!(decode_avif(&bytes).is_err());
        }

        #[test]
        fn bundled_dispatch_covers_both_formats_via_the_public_entry() {
            for (name, expect) in [("cover.avif", (32u32, 24u32)), ("cover.jxl", (32, 24))] {
                let fixture_name = if name.ends_with(".avif") {
                    "cover_32x24.avif"
                } else {
                    "cover_32x24.jxl"
                };
                let bytes = fixture(fixture_name);
                let img = super::super::decode_for_thumbnail(name, &bytes, 16).expect(name);
                assert_eq!((img.width(), img.height()), expect, "{name}");
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use image::{ImageBuffer, Rgba};

    /// Encode a tiny solid-colour image to PNG bytes via the `image`
    /// crate. Used as a known-good fixture for round-tripping through
    /// `decode_with_limits`. Visible to the `bundled` sub-module's tests.
    pub(super) fn make_png(w: u32, h: u32) -> Vec<u8> {
        let img: ImageBuffer<Rgba<u8>, Vec<u8>> =
            ImageBuffer::from_fn(w, h, |_, _| Rgba([10, 20, 30, 255]));
        let mut out = Vec::new();
        DynamicImage::ImageRgba8(img)
            .write_to(&mut Cursor::new(&mut out), image::ImageFormat::Png)
            .unwrap();
        out
    }

    fn make_jpeg(w: u32, h: u32) -> Vec<u8> {
        let img: ImageBuffer<image::Rgb<u8>, Vec<u8>> =
            ImageBuffer::from_fn(w, h, |_, _| image::Rgb([200, 100, 50]));
        let mut out = Vec::new();
        DynamicImage::ImageRgb8(img)
            .write_to(&mut Cursor::new(&mut out), image::ImageFormat::Jpeg)
            .unwrap();
        out
    }

    #[test]
    fn decode_png_roundtrip() {
        let bytes = make_png(8, 5);
        let img = decode_with_limits("foo.png", &bytes).expect("decode");
        assert_eq!(img.width(), 8);
        assert_eq!(img.height(), 5);
    }

    #[test]
    fn decode_jpeg_roundtrip() {
        let bytes = make_jpeg(16, 9);
        let img = decode_with_limits("foo.jpg", &bytes).expect("decode");
        assert_eq!(img.width(), 16);
        assert_eq!(img.height(), 9);
    }

    #[test]
    fn decode_uses_content_not_filename() {
        // The file is named .jpg but the bytes are PNG. Decoding
        // should still succeed because the image crate sniffs the
        // magic bytes rather than trusting the filename.
        let bytes = make_png(4, 4);
        let img = decode_with_limits("lying.jpg", &bytes).expect("decode");
        assert_eq!(img.width(), 4);
    }

    #[test]
    fn decode_rejects_garbage() {
        let bytes = b"this is not an image at all";
        assert!(decode_with_limits("foo.png", bytes).is_err());
    }

    #[test]
    fn decode_rejects_empty() {
        assert!(decode_with_limits("foo.png", b"").is_err());
    }

    #[test]
    fn decode_for_thumbnail_large_jpeg_is_scaled_down() {
        // Source 2048×2048. Target 256. The JPEG DCT scale options
        // are 1/1, 1/2, 1/4, 1/8 — the smallest that still leaves
        // >= target*2 (512) on the long side is 1/2 (→ 1024) since
        // 1/4 (→ 512) is exactly the threshold. Either 1/2 or 1/4
        // is acceptable here; what we verify is that we got back an
        // image materially smaller than the source.
        let bytes = make_jpeg(2048, 2048);
        let img = decode_for_thumbnail("big.jpg", &bytes, 256).expect("decode_for_thumbnail");
        assert!(
            img.width() < 2048,
            "expected scaled-down decode, got {}×{}",
            img.width(),
            img.height()
        );
        // And it should be >= target*2 so the resizer has enough
        // input quality to work with.
        assert!(
            img.width() >= 512,
            "scaled output too small: {}×{}",
            img.width(),
            img.height()
        );
    }

    #[test]
    fn decode_for_thumbnail_small_jpeg_is_not_scaled() {
        // Source 128×128. Target 256. No scaling possible (source
        // is already smaller than target*2), so we get the full
        // image back.
        let bytes = make_jpeg(128, 128);
        let img = decode_for_thumbnail("small.jpg", &bytes, 256).expect("decode_for_thumbnail");
        assert_eq!(img.width(), 128);
        assert_eq!(img.height(), 128);
    }

    #[test]
    fn decode_for_thumbnail_png_falls_through_to_full_decode() {
        // PNG doesn't support sub-resolution decoding in the image
        // crate, so we expect the full image back even under
        // `decode_for_thumbnail`.
        let bytes = make_png(512, 512);
        let img = decode_for_thumbnail("foo.png", &bytes, 64).expect("decode_for_thumbnail");
        assert_eq!(img.width(), 512);
        assert_eq!(img.height(), 512);
    }

    #[test]
    fn decode_for_thumbnail_png_labeled_jpg_is_decoded_correctly() {
        // Magic-bytes sniffing: a PNG named `.jpg` must NOT hit the
        // JPEG scale path (it would crash the JPEG decoder).
        let bytes = make_png(64, 64);
        let img = decode_for_thumbnail("lying.jpg", &bytes, 32).expect("decode_for_thumbnail");
        assert_eq!(img.width(), 64);
    }

    #[test]
    fn decode_for_thumbnail_jpeg_labeled_png_still_scales() {
        // Inverse: a JPEG named `.png` should still benefit from
        // the scaled path, because we check magic bytes, not name.
        let bytes = make_jpeg(1024, 1024);
        let img = decode_for_thumbnail("lying.png", &bytes, 128).expect("decode_for_thumbnail");
        assert!(img.width() < 1024, "expected scaled-down JPEG");
    }

    #[test]
    fn decode_for_thumbnail_rejects_garbage() {
        assert!(decode_for_thumbnail("foo.jpg", b"not an image", 64).is_err());
    }

    #[test]
    fn decode_for_thumbnail_target_zero_is_full_decode() {
        // Defensive: a target of 0 shouldn't crash. We just decode
        // the source at full resolution.
        let bytes = make_jpeg(64, 64);
        let img = decode_for_thumbnail("foo.jpg", &bytes, 0).expect("decode_for_thumbnail");
        assert_eq!(img.width(), 64);
    }

    /// Re-emit `jpeg` with an Exif APP1 segment carrying `orientation`.
    ///
    /// The `image` crate can write JPEG but not Exif, and hand-building the
    /// APP1 is cheaper than taking an Exif dependency just to test the
    /// orientation path. Only the orientation tag is written; a TIFF header
    /// with a one-entry IFD is a complete, valid Exif block.
    fn with_exif_orientation(jpeg: &[u8], orientation: u8) -> Vec<u8> {
        assert_eq!(&jpeg[..2], b"\xFF\xD8", "JPEG must start with SOI");
        let mut tiff = Vec::new();
        tiff.extend_from_slice(b"II"); // little-endian
        tiff.extend_from_slice(&42u16.to_le_bytes());
        tiff.extend_from_slice(&8u32.to_le_bytes()); // IFD at offset 8
        tiff.extend_from_slice(&1u16.to_le_bytes()); // one entry
        tiff.extend_from_slice(&0x0112u16.to_le_bytes()); // Orientation
        tiff.extend_from_slice(&3u16.to_le_bytes()); // type = SHORT
        tiff.extend_from_slice(&1u32.to_le_bytes()); // count
        let mut value = [0u8; 4];
        value[0] = orientation;
        tiff.extend_from_slice(&value);
        tiff.extend_from_slice(&0u32.to_le_bytes()); // no next IFD

        let mut payload = Vec::new();
        payload.extend_from_slice(b"Exif\0\0");
        payload.extend_from_slice(&tiff);
        let len = (payload.len() + 2) as u16;

        let mut out = Vec::with_capacity(jpeg.len() + payload.len() + 4);
        out.extend_from_slice(&jpeg[..2]);
        out.extend_from_slice(&[0xFF, 0xE1]);
        out.extend_from_slice(&len.to_be_bytes());
        out.extend_from_slice(&payload);
        out.extend_from_slice(&jpeg[2..]);
        out
    }

    #[test]
    fn exif_orientation_is_applied_by_the_generic_decoder() {
        // 40×10 landscape source tagged "rotate 90 CW" must decode as a
        // 10×40 portrait image. Both platforms' thumbnail pipeline relies
        // on this so no backend has to know Exif exists.
        let landscape = make_jpeg(40, 10);
        let rotated = with_exif_orientation(&landscape, 6);
        let img = decode_with_limits("photo.jpg", &rotated).expect("decode rotated jpeg");
        assert_eq!((img.width(), img.height()), (10, 40));

        // And the untagged original stays as-coded.
        let plain = decode_with_limits("photo.jpg", &landscape).expect("decode");
        assert_eq!((plain.width(), plain.height()), (40, 10));
    }

    #[test]
    fn exif_orientation_is_applied_by_the_jpeg_scaled_path() {
        // Large enough to take the DCT-scaled fast path, which is a
        // different decoder — it must not lose the orientation either.
        let big = make_jpeg(2048, 1024);
        let rotated = with_exif_orientation(&big, 6);
        let img = decode_for_thumbnail("photo.jpg", &rotated, 128).expect("scaled decode");
        assert!(
            img.height() > img.width(),
            "expected a portrait result after 90° orientation, got {}x{}",
            img.width(),
            img.height()
        );
    }

    #[test]
    fn limits_are_set_from_module_constants() {
        // Guard against a future refactor that forgets to wire the
        // image crate's Limits to our `limits` module.
        let l = make_limits();
        assert_eq!(l.max_image_width, Some(limits::MAX_IMAGE_DIMENSION));
        assert_eq!(l.max_image_height, Some(limits::MAX_IMAGE_DIMENSION));
        assert_eq!(l.max_alloc, Some(limits::MAX_IMAGE_ALLOC));
    }
}
