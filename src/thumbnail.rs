//! The platform-independent half of thumbnail generation.
//!
//! Every host — Explorer's `IThumbnailProvider`, the macOS Quick Look
//! extension — ends up calling [`render`]. It owns the whole pipeline
//! that decides *what pixels* a thumbnail is made of:
//!
//! ```text
//! archive → cover pick → decode → resize → overlay → RGBA8 bitmap
//! ```
//!
//! Nothing in here knows about `HBITMAP`, `CGImage`, COM or XPC. The
//! Windows backend wraps the result in a DIB section; the macOS backend
//! copies it across a C ABI into a `CGDataProvider`. Keeping that seam
//! clean is the point of this module.
//!
//! Cost discipline (Finder and Explorer both ask for small bitmaps and
//! cache aggressively, and both can call us for many files at once):
//! only the one chosen entry is read out of the archive, JPEG sources
//! are pre-scaled by the decoder's own DCT scaling, and no temporary
//! files or re-encoding are involved.

use std::error::Error;
use std::io::{Read, Seek};

use image::RgbaImage;

use crate::{alog, archive, decode, overlay, settings};

/// A rendered thumbnail bitmap and the archive kind it came from.
///
/// The kind is part of the result because hosts may want it for their
/// own badge drawing, and because callers logging a summary shouldn't
/// have to re-derive it.
#[derive(Debug)]
pub struct Thumbnail {
    /// Straight (non-premultiplied) RGBA8 pixels, tightly packed
    /// (`stride == width * 4`).
    pub image: RgbaImage,
    /// Which container the cover was extracted from.
    pub kind: archive::ContentKind,
}

/// Read an archive and render a thumbnail that fits inside
/// `max_width × max_height`, preserving aspect ratio.
///
/// `file_ext` is the lowercased extension of the source file without
/// the dot (`"cbz"`), when the host knows it. It only affects the
/// identification overlay's label; passing `None` falls back to the
/// format detected from the archive's contents.
///
/// Errors cover every way a file can fail to produce a thumbnail: not
/// an archive, no eligible image, encrypted or corrupt container, decode
/// failure, or a source that trips one of the [`crate::limits`] guards.
/// Callers are expected to treat an error as "show the default icon".
pub fn render<R: Read + Seek>(
    reader: R,
    max_width: u32,
    max_height: u32,
    file_ext: Option<&str>,
    settings: &settings::Settings,
) -> Result<Thumbnail, Box<dyn Error>> {
    // Cheapest possible rejection, before touching the file: the user
    // switched this container type off.
    if !settings.accepts_archive_ext(file_ext) {
        let ext = file_ext.unwrap_or("");
        alog!("  skipped: .{ext} disabled by settings");
        return Err(format!("archive type .{ext} is disabled").into());
    }

    let extracted = match archive::read_first_image_with_kind(reader, settings) {
        Ok(e) => e,
        Err(e) => {
            alog!("  ERROR picking image: {e}");
            return Err(e);
        }
    };
    alog!(
        "  picked: {} ({} bytes, ext={:?})",
        extracted.name,
        extracted.bytes.len(),
        file_ext
    );

    // Format-dispatching decoder with pre-decode size guards against
    // decompression bombs. `decode_for_thumbnail` additionally asks
    // the JPEG decoder to drop to a 1/2, 1/4 or 1/8 DCT scale when
    // the source is much larger than the requested thumbnail — a
    // multi-megapixel comic page is delivered at roughly twice the
    // target size instead of at full resolution, cutting the decode
    // cost by up to ~16×.
    let target_px = max_width.max(max_height);
    let img = match decode::decode_for_thumbnail(&extracted.name, &extracted.bytes, target_px) {
        Ok(img) => img,
        Err(e) => {
            alog!("  ERROR decoding '{}': {e}", extracted.name);
            return Err(e);
        }
    };
    alog!("  decoded: {}x{}", img.width(), img.height());

    // Preserve aspect ratio, fit inside max_width × max_height.
    // `Triangle` (bilinear) is a good default — fast and visually fine
    // at thumbnail sizes.
    let mut resized = img
        .resize(max_width, max_height, image::imageops::FilterType::Triangle)
        .to_rgba8();
    alog!("  resized: {}x{}", resized.width(), resized.height());

    // Bake the identification overlay (border / format label) when the
    // user has opted in. A no-op by default, so hosts keep the bare
    // cover image.
    overlay::apply_overlay(&mut resized, extracted.kind, file_ext, settings);

    Ok(Thumbnail {
        image: resized,
        kind: extracted.kind,
    })
}

/// Smallest thumbnail side we'll produce. Hosts never ask for less, but
/// a bogus request shouldn't turn into a zero-sized bitmap.
pub const MIN_SIZE: u32 = crate::limits::MIN_THUMBNAIL_SIZE;

/// Largest thumbnail side we'll produce. 2560 covers the biggest bucket
/// either host asks for (Extra Large icons at high DPI on Windows;
/// Retina grid previews on macOS).
pub const MAX_SIZE: u32 = crate::limits::MAX_THUMBNAIL_SIZE;

/// Clamp a requested thumbnail dimension into [`MIN_SIZE`]..=[`MAX_SIZE`].
pub fn clamp_size(px: u32) -> u32 {
    px.clamp(MIN_SIZE, MAX_SIZE)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    /// A ZIP holding one solid-colour PNG, as `Vec<u8>` ready to wrap
    /// in a `Cursor`.
    fn zip_with_png(w: u32, h: u32, colour: [u8; 4]) -> Vec<u8> {
        use image::{DynamicImage, ImageBuffer, Rgba};
        let img: ImageBuffer<Rgba<u8>, Vec<u8>> = ImageBuffer::from_fn(w, h, |_, _| Rgba(colour));
        let mut png = Vec::new();
        DynamicImage::ImageRgba8(img)
            .write_to(&mut Cursor::new(&mut png), image::ImageFormat::Png)
            .unwrap();

        use zip::write::SimpleFileOptions;
        let mut buf = Vec::new();
        {
            let mut zw = zip::ZipWriter::new(Cursor::new(&mut buf));
            let opts =
                SimpleFileOptions::default().compression_method(zip::CompressionMethod::Stored);
            zw.start_file("cover.png", opts).unwrap();
            std::io::Write::write_all(&mut zw, &png).unwrap();
            zw.finish().unwrap();
        }
        buf
    }

    #[test]
    fn render_produces_rgba_bitmap_of_requested_size() {
        // 4×4 source asked for at 2×2 → exact fit (aspect preserved).
        let bytes = zip_with_png(64, 64, [255, 0, 0, 255]);
        let t = render(
            Cursor::new(bytes),
            64,
            64,
            Some("cbz"),
            &settings::Settings::default(),
        )
        .expect("render");
        assert_eq!((t.image.width(), t.image.height()), (64, 64));
        assert_eq!(t.kind, archive::ContentKind::Zip);
        let px = t.image.get_pixel(3, 3);
        assert_eq!(px.0, [255, 0, 0, 255]);
    }

    #[test]
    fn render_respects_aspect_ratio_and_both_bounds() {
        // 200×100 into a 40×40 box → 40×20, never 40×40.
        let bytes = zip_with_png(200, 100, [0, 255, 0, 255]);
        let t = render(
            Cursor::new(bytes),
            40,
            40,
            None,
            &settings::Settings::default(),
        )
        .expect("render");
        assert_eq!((t.image.width(), t.image.height()), (40, 20));
    }

    #[test]
    fn render_fails_cleanly_on_non_archive() {
        let bytes = b"definitely not an archive".to_vec();
        let r = render(
            Cursor::new(bytes),
            32,
            32,
            None,
            &settings::Settings::default(),
        );
        assert!(r.is_err());
    }

    #[test]
    fn disabled_archive_extension_is_refused_before_reading() {
        let bytes = zip_with_png(64, 64, [0, 0, 255, 255]);
        let settings = settings::Settings {
            enabled_archive_exts_mask: 0,
            ..settings::Settings::default()
        };
        let err = render(Cursor::new(bytes), 32, 32, Some("cbz"), &settings)
            .expect_err("a disabled type must not render");
        assert!(err.to_string().contains("disabled"), "got: {err}");

        // An unknown extension is allowed through: on Windows Explorer
        // hands over a nameless stream and the registration layer has
        // already filtered.
        let unknown = settings::Settings {
            enabled_archive_exts_mask: 0,
            ..settings::Settings::default()
        };
        assert!(
            render(
                Cursor::new(zip_with_png(64, 64, [0, 0, 255, 255])),
                32,
                32,
                None,
                &unknown
            )
            .is_ok()
        );
    }

    #[test]
    fn default_mask_admits_every_supported_archive_type() {
        let s = settings::Settings::default();
        for ext in settings::SUPPORTED_ARCHIVE_EXTS {
            assert!(
                s.accepts_archive_ext(Some(ext)),
                "{ext} should be on by default"
            );
        }
        assert!(!s.accepts_archive_ext(Some("exe")));
        assert!(!s.accepts_archive_ext(Some("")));
    }

    #[test]
    fn clamp_size_bounds_requests() {
        assert_eq!(clamp_size(0), MIN_SIZE);
        assert_eq!(clamp_size(MIN_SIZE), MIN_SIZE);
        assert_eq!(clamp_size(256), 256);
        assert_eq!(clamp_size(u32::MAX), MAX_SIZE);
    }
}
