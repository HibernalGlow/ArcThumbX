//! COM objects: `ArcThumbClassFactory` and `ArcThumbProvider`.
//!
//! - `ArcThumbClassFactory` implements `IClassFactory`. It is the thing
//!   `DllGetClassObject` hands back, and its only job is to create
//!   fresh `ArcThumbProvider` instances on demand.
//!
//! - `ArcThumbProvider` implements `IInitializeWithStream` (Explorer
//!   gives us a stream over the target file) and `IThumbnailProvider`
//!   (Explorer asks us for an HBITMAP of a given size).
//!
//! Phase 1 ignores the stream entirely and always returns a solid-color
//! dummy bitmap. Phase 2 will actually parse the ZIP from the stream
//! and decode the first image.

use std::cell::RefCell;
use std::error::Error as StdError;
use std::ffi::c_void;

use windows::Win32::Foundation::{CLASS_E_NOAGGREGATION, E_FAIL, E_POINTER};
use windows::Win32::Graphics::Gdi::HBITMAP;
use windows::Win32::System::Com::{
    CoTaskMemFree, IClassFactory, IClassFactory_Impl, IStream, STATFLAG_DEFAULT, STATSTG,
};
use windows::Win32::UI::Shell::PropertiesSystem::{
    IInitializeWithStream, IInitializeWithStream_Impl,
};
use windows::Win32::UI::Shell::{
    IThumbnailProvider, IThumbnailProvider_Impl, WTS_ALPHATYPE, WTSAT_ARGB,
};
use windows::core::{BOOL, GUID, IUnknown, Interface, Ref, Result, implement};

use crate::{alog, archive, bitmap, decode, limits, overlay, settings, stream::ComStreamReader};

/// End-to-end: stream → archive → first image bytes → decode → resize → HBITMAP.
///
/// Any failure propagates as `Err`; the caller logs it and returns
/// `E_FAIL` so Explorer falls back to the default icon.
fn try_generate_thumbnail(
    stream: IStream,
    cx: u32,
) -> std::result::Result<HBITMAP, Box<dyn StdError>> {
    let settings = settings::current();

    // Recover the on-disk extension (when the host exposes a name on
    // the stream) so the identification label can read "CBZ" instead
    // of the generic "ZIP". Best-effort: `None` falls back to the
    // content-detected format. Done before the reader takes the stream.
    let file_ext = stream_file_ext(&stream);

    let reader = ComStreamReader::new(stream);
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
    let img = match decode::decode_for_thumbnail(&extracted.name, &extracted.bytes, cx) {
        Ok(img) => img,
        Err(e) => {
            alog!("  ERROR decoding '{}': {e}", extracted.name);
            return Err(e);
        }
    };
    alog!("  decoded: {}x{}", img.width(), img.height());

    // Preserve aspect ratio, fit inside cx × cx. `Triangle` (bilinear)
    // is a good default — fast and visually fine at thumbnail sizes.
    let mut resized = img
        .resize(cx, cx, image::imageops::FilterType::Triangle)
        .to_rgba8();
    alog!("  resized: {}x{}", resized.width(), resized.height());

    // Bake the identification overlay (border / format label) when the
    // user has opted in. A no-op by default, so existing installs keep
    // the bare cover image.
    overlay::apply_overlay(&mut resized, extracted.kind, file_ext.as_deref(), settings);

    let hbmp = bitmap::from_rgba(&resized)?;
    Ok(hbmp)
}

/// Best-effort lookup of the source file's extension via the stream's
/// `Stat` name. Explorer initialises us with a bare `IStream`
/// (`IInitializeWithStream`), so this is the only handle we have on
/// the original file name — and many stream backends leave it unset,
/// in which case we return `None` and the overlay falls back to the
/// content-detected format.
///
/// Returns the extension lowercased and without the dot, e.g. `"cbz"`.
fn stream_file_ext(stream: &IStream) -> Option<String> {
    let mut stat = STATSTG::default();
    // STATFLAG_DEFAULT asks the stream to populate `pwcsName`.
    unsafe { stream.Stat(&mut stat, STATFLAG_DEFAULT).ok()? };
    if stat.pwcsName.is_null() {
        return None;
    }
    // `pwcsName` is a COM-allocated NUL-terminated wide string we now
    // own: copy it out, then return the buffer to the task allocator.
    let name = unsafe { stat.pwcsName.to_string().ok() };
    unsafe { CoTaskMemFree(Some(stat.pwcsName.0 as *const std::ffi::c_void)) };
    extension_of(&name?)
}

/// Extract the lowercased extension (no dot) from a file name or path.
/// Splits on the last path separator first so a dotted directory name
/// can't be mistaken for the extension.
fn extension_of(name: &str) -> Option<String> {
    let base = name.rsplit(['/', '\\']).next().unwrap_or(name);
    let (_, ext) = base.rsplit_once('.')?;
    if ext.is_empty() {
        None
    } else {
        Some(ext.to_ascii_lowercase())
    }
}

/// CLSID for the ArcThumb thumbnail provider COM class.
/// **DO NOT CHANGE** — baked into users' registries on install.
pub const CLSID_ARCTHUMB_PROVIDER: GUID = GUID::from_u128(0x0F4F5659_D383_4945_A534_01E1EED1D23F);

// =============================================================================
// IClassFactory
// =============================================================================

#[implement(IClassFactory)]
pub struct ArcThumbClassFactory;

impl IClassFactory_Impl for ArcThumbClassFactory_Impl {
    fn CreateInstance(
        &self,
        punkouter: Ref<'_, IUnknown>,
        riid: *const GUID,
        ppvobject: *mut *mut c_void,
    ) -> Result<()> {
        // COM aggregation is an advanced feature we don't support.
        if !punkouter.is_null() {
            return Err(CLASS_E_NOAGGREGATION.into());
        }
        if ppvobject.is_null() || riid.is_null() {
            return Err(E_POINTER.into());
        }

        unsafe {
            *ppvobject = std::ptr::null_mut();
            // Create a fresh provider and hand it to the caller under
            // whatever interface they asked for (QueryInterface).
            let provider = ArcThumbProvider::default();
            let unknown: IUnknown = provider.into();
            unknown.query(&*riid, ppvobject).ok()
        }
    }

    fn LockServer(&self, _flock: BOOL) -> Result<()> {
        // No-op: we don't care whether the server is locked.
        Ok(())
    }
}

// =============================================================================
// ArcThumbProvider — IThumbnailProvider + IInitializeWithStream
// =============================================================================

/// The COM object Explorer actually talks to for each thumbnail request.
///
/// `stream` is populated by `IInitializeWithStream::Initialize`, then
/// consumed (eventually) by `IThumbnailProvider::GetThumbnail`. Phase 1
/// stores it but never reads from it.
#[implement(IThumbnailProvider, IInitializeWithStream)]
#[derive(Default)]
pub struct ArcThumbProvider {
    stream: RefCell<Option<IStream>>,
}

impl IInitializeWithStream_Impl for ArcThumbProvider_Impl {
    fn Initialize(&self, pstream: Ref<'_, IStream>, _grfmode: u32) -> Result<()> {
        // Initialize is trivial but we still guard it: the #[implement]
        // glue calls it across the COM ABI, so a panic here would be UB.
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            *self.this.stream.borrow_mut() = pstream.cloned();
            Ok(())
        }));
        match result {
            Ok(r) => r,
            Err(_) => {
                alog!("PANIC caught in Initialize");
                Err(windows::core::Error::from_hresult(E_FAIL))
            }
        }
    }
}

impl IThumbnailProvider_Impl for ArcThumbProvider_Impl {
    fn GetThumbnail(
        &self,
        cx: u32,
        phbmp: *mut HBITMAP,
        pdwalpha: *mut WTS_ALPHATYPE,
    ) -> Result<()> {
        // catch_unwind turns any panic inside our code (image decoder,
        // archive parser, allocator, …) into a clean COM error instead
        // of undefined behaviour across the C ABI boundary.
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            self.get_thumbnail_inner(cx, phbmp, pdwalpha)
        }));
        match result {
            Ok(r) => r,
            Err(_) => {
                alog!("PANIC caught in GetThumbnail");
                Err(windows::core::Error::from_hresult(E_FAIL))
            }
        }
    }
}

impl ArcThumbProvider_Impl {
    fn get_thumbnail_inner(
        &self,
        cx: u32,
        phbmp: *mut HBITMAP,
        pdwalpha: *mut WTS_ALPHATYPE,
    ) -> Result<()> {
        if phbmp.is_null() || pdwalpha.is_null() {
            return Err(E_POINTER.into());
        }

        alog!("---- GetThumbnail cx={cx} ----");

        // Clamp to Windows's standard icon range. Explorer's largest
        // bucket is 2560 (Extra Large × high DPI); the lower bound is
        // defensive.
        let size = clamp_thumbnail_size(cx);

        let stream = self.this.stream.borrow().clone().ok_or_else(|| {
            alog!("  no stream attached");
            windows::core::Error::from_hresult(E_FAIL)
        })?;

        // On any failure (not-an-archive, no images inside, decode
        // error, …) we return an error HRESULT. Explorer then falls
        // back to the built-in handler's icon, which is the right UX:
        // archives without images should look like normal zips, not
        // like broken thumbnails.
        let hbmp = try_generate_thumbnail(stream, size).map_err(|e| {
            alog!("  no thumbnail: {e}");
            windows::core::Error::from_hresult(E_FAIL)
        })?;

        unsafe {
            *phbmp = hbmp;
            *pdwalpha = WTSAT_ARGB;
        }
        Ok(())
    }
}

/// Clamp a requested thumbnail size to the allowed range.
fn clamp_thumbnail_size(cx: u32) -> u32 {
    cx.clamp(limits::MIN_THUMBNAIL_SIZE, limits::MAX_THUMBNAIL_SIZE)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn extension_of_handles_names_and_paths() {
        assert_eq!(extension_of("book.cbz").as_deref(), Some("cbz"));
        assert_eq!(extension_of("BOOK.CBZ").as_deref(), Some("cbz"));
        // Last extension wins; the FB2/EPUB content kind overrides this
        // upstream anyway.
        assert_eq!(extension_of("novel.fb2.zip").as_deref(), Some("zip"));
        // A dotted folder must not be read as the extension.
        assert_eq!(extension_of("C:\\my.archives\\comic").as_deref(), None);
        assert_eq!(extension_of("comic").as_deref(), None);
        assert_eq!(extension_of("trailing.").as_deref(), None);
    }

    #[test]
    fn clamp_within_range_is_identity() {
        assert_eq!(clamp_thumbnail_size(64), 64);
        assert_eq!(clamp_thumbnail_size(256), 256);
        assert_eq!(
            clamp_thumbnail_size(limits::MIN_THUMBNAIL_SIZE),
            limits::MIN_THUMBNAIL_SIZE
        );
        assert_eq!(
            clamp_thumbnail_size(limits::MAX_THUMBNAIL_SIZE),
            limits::MAX_THUMBNAIL_SIZE
        );
    }

    #[test]
    fn clamp_below_minimum() {
        assert_eq!(clamp_thumbnail_size(0), limits::MIN_THUMBNAIL_SIZE);
        assert_eq!(clamp_thumbnail_size(1), limits::MIN_THUMBNAIL_SIZE);
        assert_eq!(
            clamp_thumbnail_size(limits::MIN_THUMBNAIL_SIZE - 1),
            limits::MIN_THUMBNAIL_SIZE
        );
    }

    #[test]
    fn clamp_above_maximum() {
        assert_eq!(clamp_thumbnail_size(u32::MAX), limits::MAX_THUMBNAIL_SIZE);
        assert_eq!(
            clamp_thumbnail_size(limits::MAX_THUMBNAIL_SIZE + 1),
            limits::MAX_THUMBNAIL_SIZE
        );
        assert_eq!(clamp_thumbnail_size(10000), limits::MAX_THUMBNAIL_SIZE);
    }

    #[test]
    fn clamp_standard_explorer_sizes() {
        // Explorer's common thumbnail size buckets.
        for size in [16, 32, 48, 64, 96, 128, 256, 512, 1024, 2560] {
            let clamped = clamp_thumbnail_size(size);
            assert_eq!(clamped, size, "standard size {size} should pass through");
        }
    }

    #[test]
    fn try_generate_thumbnail_rejects_garbage_stream() {
        use windows::Win32::UI::Shell::SHCreateMemStream;
        let garbage = b"this is not an archive at all";
        let stream: IStream =
            unsafe { SHCreateMemStream(Some(garbage)) }.expect("SHCreateMemStream");
        let result = try_generate_thumbnail(stream, 64);
        assert!(result.is_err(), "garbage data should fail");
    }

    #[test]
    fn try_generate_thumbnail_rejects_empty_stream() {
        use windows::Win32::UI::Shell::SHCreateMemStream;
        let stream: IStream = unsafe { SHCreateMemStream(Some(&[])) }.expect("SHCreateMemStream");
        let result = try_generate_thumbnail(stream, 64);
        assert!(result.is_err(), "empty stream should fail");
    }

    #[test]
    fn try_generate_thumbnail_succeeds_with_valid_zip() {
        use std::io::Cursor;
        use windows::Win32::Graphics::Gdi::{DeleteObject, HGDIOBJ};
        use windows::Win32::UI::Shell::SHCreateMemStream;

        // Build a valid ZIP containing a PNG
        let png = {
            use image::{DynamicImage, ImageBuffer, Rgba};
            let img: ImageBuffer<Rgba<u8>, Vec<u8>> =
                ImageBuffer::from_fn(4, 4, |_, _| Rgba([255, 0, 0, 255]));
            let mut out = Vec::new();
            DynamicImage::ImageRgba8(img)
                .write_to(&mut Cursor::new(&mut out), image::ImageFormat::Png)
                .unwrap();
            out
        };
        let zip_bytes = {
            use zip::write::SimpleFileOptions;
            let mut buf = Vec::new();
            {
                let mut w = zip::ZipWriter::new(Cursor::new(&mut buf));
                let opts =
                    SimpleFileOptions::default().compression_method(zip::CompressionMethod::Stored);
                w.start_file("test.png", opts).unwrap();
                std::io::Write::write_all(&mut w, &png).unwrap();
                w.finish().unwrap();
            }
            buf
        };

        let stream: IStream =
            unsafe { SHCreateMemStream(Some(&zip_bytes)) }.expect("SHCreateMemStream");
        let hbmp = try_generate_thumbnail(stream, 64).expect("should succeed");
        assert!(!hbmp.is_invalid());
        // Clean up
        unsafe {
            let _ = DeleteObject(HGDIOBJ(hbmp.0));
        }
    }
}
