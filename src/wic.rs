//! AVIF / JXL decoding via the Windows Imaging Component (WIC).
//!
//! WIC is the native Windows COM image codec framework.  By delegating
//! to WIC we get AVIF and JPEG XL support with zero new Rust dependencies
//! — the codecs live on the host system (Windows 11 24H2+ ships them
//! built-in; Windows 10 requires the AV1 Image Extensions and/or JPEG
//! XL Extensions from the Microsoft Store).
//!
//! Gated behind the `wic` Cargo feature.  Without it, the module does
//! not exist and the crate compiles with its original format set.

use std::error::Error;
use std::ptr;

use image::{DynamicImage, ImageBuffer, Rgba};
use windows::Win32::Foundation::HGLOBAL;
use windows::Win32::Graphics::Imaging::{
    CLSID_WICImagingFactory, GUID_WICPixelFormat32bppRGBA, IWICBitmapDecoder,
    IWICBitmapFrameDecode, IWICFormatConverter, IWICImagingFactory, WICBitmapDitherType,
    WICBitmapPaletteType, WICDecodeOptions, WICRect,
};
use windows::Win32::System::Com::StructuredStorage::CreateStreamOnHGlobal;
use windows::Win32::System::Com::{
    CLSCTX_INPROC_SERVER, CoCreateInstance, IStream, STREAM_SEEK_SET,
};

use crate::limits;

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/// Return `true` if the filename (case-insensitive) ends with an
/// extension that should be decoded via WIC — currently `.avif` and
/// `.jxl`.
pub fn is_wic_format(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower.ends_with(".avif") || lower.ends_with(".jxl")
}

/// Decode arbitrary image bytes through WIC.  Format is auto-detected
/// from magic bytes by the WIC codec, so a mislabelled file will still
/// decode (or fail) correctly.
///
/// Returns a full-resolution `DynamicImage::ImageRgba8`.
///
/// # Errors
///
/// - `WINCODEC_ERR_COMPONENTNOTFOUND` (0x88982F50) → the system has no
///   WIC codec installed for this image format.  Turned into a
///   human-readable message like *"No WIC codec found"*.
/// - Other WIC HRESULT errors are forwarded with their hex code.
/// - Dimensions exceeding [`limits::MAX_IMAGE_DIMENSION`] or pixel
///   buffer exceeding [`limits::MAX_IMAGE_ALLOC`] cause an error.
pub fn decode_via_wic(bytes: &[u8]) -> Result<DynamicImage, Box<dyn Error>> {
    // ── 1. Wrap bytes in an IStream ────────────────────────────────────
    let stream = create_stream_over_bytes(bytes)?;

    // ── 2. Create the WIC imaging factory ─────────────────────────────
    let factory: IWICImagingFactory = unsafe {
        CoCreateInstance(&CLSID_WICImagingFactory, None, CLSCTX_INPROC_SERVER)?
    };

    // ── 3. Auto-detect format and create a decoder ────────────────────
    let decoder: IWICBitmapDecoder = unsafe {
        factory
            .CreateDecoderFromStream(
                &stream,
                ptr::null(),
                WICDecodeOptions(0),
            )
            .map_err(|e| wic_error("CreateDecoderFromStream", e))?
    };

    // ── 4. Get the first (and usually only) frame ─────────────────────
    let frame: IWICBitmapFrameDecode = unsafe {
        decoder
            .GetFrame(0)
            .map_err(|e| wic_error("GetFrame", e))?
    };

    // ── 5. Check dimensions before decoding ────────────────────────────
    let (w, h) = unsafe { get_frame_size(&frame)? };
    if w > limits::MAX_IMAGE_DIMENSION || h > limits::MAX_IMAGE_DIMENSION {
        return Err(format!(
            "WIC image dimensions too large: {w}x{h} (max {})",
            limits::MAX_IMAGE_DIMENSION
        )
        .into());
    }
    let pixel_bytes = (w as u64).saturating_mul(h as u64).saturating_mul(4);
    if pixel_bytes > limits::MAX_IMAGE_ALLOC {
        return Err(format!(
            "WIC decoded buffer would exceed allocation limit: {pixel_bytes} bytes"
        )
        .into());
    }

    // ── 6. Convert to 32bpp RGBA via format converter ─────────────────
    let converter: IWICFormatConverter = unsafe {
        factory
            .CreateFormatConverter()
            .map_err(|e| wic_error("CreateFormatConverter", e))?
    };
    unsafe {
        converter
            .Initialize(
                &frame,
                &GUID_WICPixelFormat32bppRGBA,
                WICBitmapDitherType(0), // WICBitmapDitherTypeNone
                None,                   // no palette
                0.0,
                WICBitmapPaletteType(0), // WICBitmapPaletteTypeCustom
            )
            .map_err(|e| wic_error("Initialize format converter", e))?
    };

    // ── 7. Copy pixels into a Vec<u8> ─────────────────────────────────
    // For GUID_WICPixelFormat32bppRGBA, stride = width * 4 exactly
    // (WIC guarantees no padding for this pixel format).
    let stride = w * 4;
    let buf_len = stride * h;
    let mut buf = vec![0u8; buf_len as usize];
    let rect = WICRect {
        X: 0,
        Y: 0,
        Width: w as i32,
        Height: h as i32,
    };
    unsafe {
        converter
            .CopyPixels(&rect, stride, &mut buf)
            .map_err(|e| wic_error("CopyPixels", e))?
    };

    // ── 8. Build image::RgbaImage ────────────────────────────────────
    let img = ImageBuffer::<Rgba<u8>, Vec<u8>>::from_raw(w, h, buf)
        .ok_or("WIC output buffer size mismatch")?;

    Ok(DynamicImage::ImageRgba8(img))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Wrap a byte slice in an HGLOBAL-backed COM `IStream` that WIC can
/// read from.
fn create_stream_over_bytes(bytes: &[u8]) -> Result<IStream, Box<dyn Error>> {
    // CreateStreamOnHGlobal(NULL, TRUE) allocates an HGLOBAL internally
    // and will free it when the IStream is released.  We then write the
    // bytes via IStream::Write and seek back to the start.
    let stream: IStream = unsafe {
        CreateStreamOnHGlobal(HGLOBAL::default(), true)
            .map_err(|e| wic_error("CreateStreamOnHGlobal", e))?
    };

    // Write image data into the stream.
    // IStream::Write returns HRESULT directly; check for failure.
    unsafe {
        let hr = stream.Write(
            bytes.as_ptr() as *const _,
            bytes.len() as u32,
            None,
        );
        hr.ok().map_err(|e| wic_error("IStream::Write", e))?;
    }

    // Rewind to the beginning so WIC sees the full data from the start.
    unsafe {
        stream
            .Seek(0, STREAM_SEEK_SET, None)
            .map_err(|e| wic_error("IStream::Seek", e))?;
    }

    Ok(stream)
}

/// Query a WIC bitmap frame for its pixel dimensions.
unsafe fn get_frame_size(
    frame: &IWICBitmapFrameDecode,
) -> Result<(u32, u32), Box<dyn Error>> {
    let mut w = 0u32;
    let mut h = 0u32;
    unsafe {
        frame
            .GetSize(&mut w, &mut h)
            .map_err(|e| wic_error("GetSize", e))?;
    }
    Ok((w, h))
}

/// Wrap a WIC error into a `Box<dyn Error>` with a descriptive prefix.
fn wic_error(context: &str, hr: windows::core::Error) -> Box<dyn Error> {
    // WINCODEC_ERR_COMPONENTNOTFOUND = 0x88982F50
    if hr.code().0 == 0x88982F50_u32 as i32 {
        format!("{context}: no WIC codec installed for this image format").into()
    } else {
        format!("{context}: {hr}").into()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn is_wic_format_recognises_avif() {
        assert!(is_wic_format("cover.avif"));
        assert!(is_wic_format("COVER.AVIF"));
        assert!(is_wic_format("path/to/image.AViF"));
    }

    #[test]
    fn is_wic_format_recognises_jxl() {
        assert!(is_wic_format("photo.jxl"));
        assert!(is_wic_format("PHOTO.JXL"));
        assert!(is_wic_format("folder/page.JxL"));
    }

    #[test]
    fn is_wic_format_rejects_other_formats() {
        assert!(!is_wic_format("photo.jpg"));
        assert!(!is_wic_format("photo.png"));
        assert!(!is_wic_format("photo.webp"));
        assert!(!is_wic_format("photo.avif.txt")); // suffix match only
        assert!(!is_wic_format(""));
    }
}
