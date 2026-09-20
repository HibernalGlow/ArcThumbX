//! End-to-end tests through the C ABI — the exact call sequence the macOS
//! Quick Look extension performs, minus CoreGraphics.
//!
//! These run the *whole* pipeline (container → cover pick → decode →
//! resize → overlay → premultiplied RGBA) against archives built on the
//! fly, so a regression in the shared core shows up here on every macOS CI
//! run rather than only when someone opens Finder.

use std::ffi::{CStr, CString};
use std::io::{Cursor, Write};
use std::path::{Path, PathBuf};

use arcthumb_ffi::{
    ARC_COVER_IGNORE, ARC_COVER_PREFER, ARC_SORT_NATURAL, ArcSettings, ArcThumbnail,
    arc_last_error, arc_settings_default, arc_thumbnail_free, arc_thumbnail_generate,
};
use image::{DynamicImage, ImageBuffer, Rgba};

/// Unique directory per test process so parallel tests never collide.
fn scratch_dir(tag: &str) -> PathBuf {
    let pid = std::process::id();
    let nanos = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("arcthumb_ffi_{tag}_{pid}_{nanos}"));
    std::fs::create_dir_all(&dir).unwrap();
    dir
}

fn solid_png(w: u32, h: u32, colour: [u8; 4]) -> Vec<u8> {
    let img: ImageBuffer<Rgba<u8>, Vec<u8>> = ImageBuffer::from_fn(w, h, |_, _| Rgba(colour));
    let mut out = Vec::new();
    DynamicImage::ImageRgba8(img)
        .write_to(&mut Cursor::new(&mut out), image::ImageFormat::Png)
        .unwrap();
    out
}

fn solid_jpeg(w: u32, h: u32, colour: [u8; 3]) -> Vec<u8> {
    let img: ImageBuffer<image::Rgb<u8>, Vec<u8>> =
        ImageBuffer::from_fn(w, h, |_, _| image::Rgb(colour));
    let mut out = Vec::new();
    DynamicImage::ImageRgb8(img)
        .write_to(&mut Cursor::new(&mut out), image::ImageFormat::Jpeg)
        .unwrap();
    out
}

/// Write a ZIP/CBZ containing `entries` and return its path.
fn write_cbz(dir: &Path, name: &str, entries: &[(&str, Vec<u8>)]) -> PathBuf {
    use zip::write::SimpleFileOptions;
    let mut buf = Vec::new();
    {
        let mut zw = zip::ZipWriter::new(Cursor::new(&mut buf));
        let opts = SimpleFileOptions::default().compression_method(zip::CompressionMethod::Stored);
        for (entry, body) in entries {
            zw.start_file(*entry, opts).unwrap();
            zw.write_all(body).unwrap();
        }
        zw.finish().unwrap();
    }
    let path = dir.join(name);
    std::fs::write(&path, buf).unwrap();
    path
}

fn fixture(name: &str) -> Vec<u8> {
    let dir = concat!(env!("CARGO_MANIFEST_DIR"), "/../../tests/fixtures");
    let path = format!("{dir}/{name}");
    std::fs::read(&path).unwrap_or_else(|e| panic!("missing fixture {path}: {e}"))
}

/// Call the ABI the way Swift does, and take ownership of the result.
fn generate(path: &Path, settings: &ArcSettings, max_w: u32, max_h: u32) -> Option<Generated> {
    let c = CString::new(path.to_str().unwrap()).unwrap();
    let ptr =
        unsafe { arc_thumbnail_generate(c.as_ptr(), max_w, max_h, settings as *const ArcSettings) };
    if ptr.is_null() {
        return None;
    }
    Some(Generated {
        thumb: unsafe { *ptr },
        raw: ptr,
    })
}

/// Owns a returned thumbnail so it is freed exactly once, which is what
/// `CGDataProvider`'s release callback does on the Swift side. Running the
/// suite under a leak checker would catch a double free or a lost box.
struct Generated {
    thumb: ArcThumbnail,
    raw: *mut ArcThumbnail,
}

impl Drop for Generated {
    fn drop(&mut self) {
        unsafe { arc_thumbnail_free(self.raw) };
    }
}

impl Generated {
    fn pixel(&self, x: u32, y: u32) -> [u8; 4] {
        let bytes =
            unsafe { std::slice::from_raw_parts(self.thumb.data, self.thumb.data_len as usize) };
        let off = (y as usize * self.thumb.stride as usize) + x as usize * 4;
        [bytes[off], bytes[off + 1], bytes[off + 2], bytes[off + 3]]
    }
    /// Every colour channel must be ≤ alpha: the definition of
    /// premultiplied, and what `kCGImageAlphaPremultipliedLast` requires.
    fn assert_premultiplied(&self) {
        let bytes =
            unsafe { std::slice::from_raw_parts(self.thumb.data, self.thumb.data_len as usize) };
        let (pixels, _trailing) = bytes.as_chunks::<4>();
        for px in pixels {
            let (r, g, b, a) = (px[0], px[1], px[2], px[3]);
            assert!(r <= a && g <= a && b <= a, "not premultiplied: {px:?}");
        }
    }
    fn geometry_is_sane(&self, max_w: u32, max_h: u32) -> bool {
        self.thumb.width > 0
            && self.thumb.height > 0
            && self.thumb.width <= max_w
            && self.thumb.height <= max_h
            && self.thumb.stride == self.thumb.width * 4
            && self.thumb.data_len == self.thumb.stride * self.thumb.height
            && self.thumb.struct_size == std::mem::size_of::<ArcThumbnail>() as u32
    }
}

fn last_error() -> String {
    let mut buf = [0i8; 512];
    let n = unsafe { arc_last_error(buf.as_mut_ptr(), buf.len() as u32) } as usize;
    let bytes: Vec<u8> = buf[..n].iter().map(|&c| c as u8).collect();
    String::from_utf8_lossy(&bytes).into_owned()
}

#[test]
fn jpeg_cover_becomes_a_premultiplied_thumbnail() {
    let dir = scratch_dir("jpeg");
    let path = write_cbz(
        &dir,
        "test.cbz",
        &[("cover.jpg", solid_jpeg(200, 100, [10, 200, 30]))],
    );

    let settings = ArcSettings::default();
    let t = generate(&path, &settings, 160, 160).expect("thumbnail");
    assert!(t.geometry_is_sane(160, 160), "bad geometry");
    // 200×100 fitted into 160×160 keeps the aspect ratio.
    assert_eq!((t.thumb.width, t.thumb.height), (160, 80));
    let px = t.pixel(80, 40);
    assert!(
        px[1] > px[0] && px[1] > px[2],
        "expected the green cover, got {px:?}"
    );
    assert_eq!(px[3], 255, "opaque source must stay opaque");
    t.assert_premultiplied();
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn avif_cover_decodes_without_system_support() {
    let dir = scratch_dir("avif");
    let path = write_cbz(
        &dir,
        "cover.avif.cbz",
        &[("cover.avif", fixture("cover_32x24.avif"))],
    );
    let t = generate(&path, &ArcSettings::default(), 128, 128).expect("avif thumbnail");
    assert!(t.geometry_is_sane(128, 128));
    assert_eq!(
        (t.thumb.width, t.thumb.height),
        (128, 96),
        "32:24 aspect kept"
    );
    t.assert_premultiplied();
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn jxl_cover_decodes_in_rust_not_via_the_os() {
    let dir = scratch_dir("jxl");
    let path = write_cbz(
        &dir,
        "cover.jxl.cbz",
        &[("cover.jxl", fixture("cover_32x24.jxl"))],
    );
    let t = generate(&path, &ArcSettings::default(), 128, 128).expect("jxl thumbnail");
    assert_eq!((t.thumb.width, t.thumb.height), (128, 96));
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn lossless_jxl_is_decoded_too() {
    let dir = scratch_dir("jxllossless");
    let path = write_cbz(&dir, "l.cbz", &[("001.jxl", fixture("lossless_64x48.jxl"))]);
    let t = generate(&path, &ArcSettings::default(), 64, 64).expect("lossless jxl thumbnail");
    assert_eq!((t.thumb.width, t.thumb.height), (64, 48));
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn cover_named_image_wins_over_an_earlier_page() {
    let dir = scratch_dir("coverpick");
    let path = write_cbz(
        &dir,
        "c.cbz",
        &[
            ("001.png", solid_png(64, 64, [220, 10, 10, 255])),
            ("cover.png", solid_png(64, 64, [10, 10, 220, 255])),
            ("002.png", solid_png(64, 64, [10, 220, 10, 255])),
        ],
    );
    let prefer = ArcSettings {
        cover_mode: ARC_COVER_PREFER,
        ..ArcSettings::default()
    };
    let t = generate(&path, &prefer, 64, 64).unwrap();
    let px = t.pixel(32, 32);
    assert!(
        px[2] > 150 && px[0] < 100,
        "cover.png should win, got {px:?}"
    );

    // Same archive, cover names ignored: the first page by sort order wins.
    let ignore = ArcSettings {
        cover_mode: ARC_COVER_IGNORE,
        ..ArcSettings::default()
    };
    let t = generate(&path, &ignore, 64, 64).unwrap();
    let px = t.pixel(32, 32);
    assert!(
        px[0] > 150 && px[2] < 100,
        "001.png should win in ignore mode, got {px:?}"
    );
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn natural_sort_picks_page2_before_page10() {
    let dir = scratch_dir("naturalsort");
    let path = write_cbz(
        &dir,
        "p.cbz",
        &[
            ("page10.png", solid_png(64, 64, [10, 10, 220, 255])),
            ("page2.png", solid_png(64, 64, [220, 10, 10, 255])),
        ],
    );
    let t = generate(&path, &ArcSettings::default(), 64, 64).unwrap();
    let px = t.pixel(32, 32);
    assert!(px[0] > 150, "page2 should sort first, got {px:?}");
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn archive_without_images_yields_no_thumbnail_and_a_reason() {
    let dir = scratch_dir("noimages");
    let path = write_cbz(
        &dir,
        "n.cbz",
        &[("readme.txt", b"no pictures here".to_vec())],
    );
    assert!(generate(&path, &ArcSettings::default(), 64, 64).is_none());
    assert!(
        !last_error().is_empty(),
        "an error message should explain it"
    );
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn corrupt_and_truncated_archives_fail_cleanly() {
    let dir = scratch_dir("corrupt");
    let junk = dir.join("corrupt.cbz");
    std::fs::write(&junk, b"this is not an archive, not even close").unwrap();
    assert!(generate(&junk, &ArcSettings::default(), 64, 64).is_none());

    // A real CBZ with its tail cut off: the central directory survives, the
    // entry payload does not.
    let good = write_cbz(
        &dir,
        "good.cbz",
        &[("cover.png", solid_png(64, 64, [1, 2, 3, 255]))],
    );
    let mut bytes = std::fs::read(&good).unwrap();
    bytes.truncate(bytes.len() / 2);
    let truncated = dir.join("trunc.cbz");
    std::fs::write(&truncated, &bytes).unwrap();
    assert!(generate(&truncated, &ArcSettings::default(), 64, 64).is_none());
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn empty_archive_and_empty_path_are_handled() {
    let dir = scratch_dir("empty");
    let path = write_cbz(&dir, "e.cbz", &[]);
    assert!(generate(&path, &ArcSettings::default(), 64, 64).is_none());
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn disabled_image_format_falls_through_to_an_eligible_sibling() {
    let dir = scratch_dir("mask");
    let path = write_cbz(
        &dir,
        "m.cbz",
        &[
            ("a.jpg", solid_jpeg(64, 64, [220, 10, 10])),
            ("b.png", solid_png(64, 64, [10, 10, 220, 255])),
        ],
    );
    let all = ArcSettings::default().enabled_image_exts_mask;
    // Clear the .jpg bit (index 0 of SUPPORTED_IMAGE_EXTS) → b.png wins.
    let settings = ArcSettings {
        enabled_image_exts_mask: all & !1,
        cover_mode: ARC_COVER_IGNORE,
        ..ArcSettings::default()
    };
    let t = generate(&path, &settings, 64, 64).unwrap();
    let px = t.pixel(32, 32);
    assert!(
        px[2] > 150 && px[0] < 100,
        "jpeg should be skipped, got {px:?}"
    );
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn zero_mask_produces_no_thumbnail() {
    let dir = scratch_dir("zeromask");
    let path = write_cbz(
        &dir,
        "z.cbz",
        &[("cover.png", solid_png(64, 64, [1, 2, 3, 255]))],
    );
    let settings = ArcSettings {
        enabled_image_exts_mask: 0,
        ..ArcSettings::default()
    };
    assert!(generate(&path, &settings, 64, 64).is_none());
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn disabled_archive_type_gets_no_thumbnail() {
    // The macOS stand-in for un-ticking ".cbz" in the Windows config GUI,
    // which unbinds the extension from Explorer instead. Bit 1 of the
    // archive mask is `cbz`; `zip` (bit 0) must keep working.
    let dir = scratch_dir("archivemask");
    let cbz = write_cbz(
        &dir,
        "a.cbz",
        &[("cover.png", solid_png(64, 64, [9, 9, 200, 255]))],
    );
    let zip = write_cbz(
        &dir,
        "a.zip",
        &[("cover.png", solid_png(64, 64, [9, 9, 200, 255]))],
    );
    let settings = ArcSettings {
        enabled_archive_exts_mask: arc_settings_default().enabled_archive_exts_mask & !(1 << 1),
        ..arc_settings_default()
    };
    assert!(
        generate(&cbz, &settings, 64, 64).is_none(),
        "cbz is switched off and must yield nothing"
    );
    assert!(
        generate(&zip, &settings, 64, 64).is_some(),
        "zip is still switched on"
    );
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn overlay_changes_the_rendered_pixels() {
    let dir = scratch_dir("overlay");
    let body = solid_png(128, 128, [40, 40, 40, 255]);
    let path = write_cbz(&dir, "o.cbz", &[("cover.png", body)]);
    let plain = generate(&path, &ArcSettings::default(), 128, 128).unwrap();
    let labelled = generate(
        &path,
        &ArcSettings {
            overlay_border: 1,
            overlay_label: 1,
            ..ArcSettings::default()
        },
        128,
        128,
    )
    .unwrap();
    assert_ne!(
        plain.pixel(2, 2),
        labelled.pixel(2, 2),
        "the border should repaint the corner pixel"
    );
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn requests_are_clamped_instead_of_rejected() {
    let dir = scratch_dir("clamp");
    let path = write_cbz(
        &dir,
        "c.cbz",
        &[("cover.png", solid_png(64, 64, [9, 9, 9, 255]))],
    );
    let settings = ArcSettings::default();
    // Absurd sizes come back inside the supported band rather than failing
    // or allocating a giant bitmap.
    let t = generate(&path, &settings, 0, 0).unwrap();
    assert!(t.thumb.width >= 16 && t.thumb.height >= 16);
    let big = generate(&path, &settings, u32::MAX, u32::MAX).unwrap();
    assert!(
        big.thumb.width <= arcthumb::limits::MAX_THUMBNAIL_SIZE,
        "clamped to the documented maximum"
    );
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn concurrent_calls_do_not_interfere() {
    // Finder asks for many thumbnails at once; per-thread error state and
    // the shared codec latches must not cross over.
    let dir = scratch_dir("threads");
    let path = write_cbz(
        &dir,
        "t.cbz",
        &[("cover.avif", fixture("cover_32x24.avif"))],
    );
    let settings = ArcSettings {
        sort_order: ARC_SORT_NATURAL,
        ..ArcSettings::default()
    };
    let mut handles = Vec::new();
    for _ in 0..8 {
        let path = path.clone();
        handles.push(std::thread::spawn(move || {
            for _ in 0..8 {
                let t = generate(&path, &settings, 64, 64).expect("thumbnail");
                assert!(t.geometry_is_sane(64, 64));
            }
        }));
    }
    for h in handles {
        h.join().expect("a worker thread panicked");
    }
    std::fs::remove_dir_all(&dir).ok();
}

#[test]
fn paths_that_are_not_valid_utf8_are_refused() {
    // The boundary is a `CStr`, so a byte sequence that isn't UTF-8 must be
    // reported as a bad argument rather than decoded into a bogus path.
    let mut raw: Vec<u8> = b"/tmp/arcthumb-".to_vec();
    raw.push(0xFF); // not a UTF-8 leading byte
    raw.extend_from_slice(b".cbz");
    raw.push(0);
    let cpath = unsafe { CStr::from_bytes_with_nul_unchecked(&raw) };
    let settings = ArcSettings::default();
    let p =
        unsafe { arc_thumbnail_generate(cpath.as_ptr(), 64, 64, &settings as *const ArcSettings) };
    assert!(p.is_null());
    assert!(last_error().contains("UTF-8"), "got: {}", last_error());
}
