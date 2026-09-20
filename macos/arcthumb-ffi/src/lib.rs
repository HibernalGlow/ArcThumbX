//! C ABI surface over the ArcThumb thumbnail core.
//!
//! The macOS Quick Look extension is meant to be thin: it receives a file
//! URL and a target size from Finder, calls into Rust here, and wraps the
//! returned pixels in a `CGImage`. Everything that decides *what* the
//! thumbnail looks like — archive parsing, cover selection, AVIF/JXL
//! decoding, resizing, the identification overlay — stays in the shared
//! core that Explorer also uses.
//!
//! # ABI rules
//!
//! * Plain `#[repr(C)]` data across the boundary; no Rust types and no
//!   opaque handle types (Swift cannot name a forward-declared C struct).
//! * Every entry point catches panics — unwinding out of an `extern "C"`
//!   frame is undefined behaviour and would take the host process down.
//! * [`arc_thumbnail_generate`] returns an allocation the caller owns and
//!   must release exactly once with [`arc_thumbnail_free`], which is what
//!   the `CGDataProvider` release callback on the Swift side calls. Pixels
//!   are never copied and no image file is ever written.
//! * Failures return null and leave a message behind, readable with
//!   [`arc_last_error`]. Failing is normal: "this archive holds no images"
//!   is not an emergency, just no thumbnail.
//! * Everything here is thread-safe and callable concurrently; the only
//!   mutable state is the per-thread error message.

use std::cell::RefCell;
use std::error::Error;
use std::ffi::{CStr, c_char};

use arcthumb::settings::{CoverMode, Settings, SortOrder, default_enabled_image_exts_mask};
use arcthumb::thumbnail;

/// Bumped on any incompatible change to the structs or functions below.
///
/// 2 added [`ArcSettings::enabled_archive_exts_mask`].
pub const ARC_ABI_VERSION: u32 = 2;

/// [`ArcThumbnail::format`]: 8 bits per channel, R,G,B,A byte order, alpha
/// premultiplied.
///
/// Premultiplied because that is what `kCGImageAlphaPremultipliedLast`
/// wants, so Swift can wrap the buffer without a conversion pass.
pub const ARC_PIXEL_FORMAT_RGBA8_PREMULTIPLIED: u32 = 1;

/// [`ArcSettings::sort_order`]: plain byte-wise ordering.
pub const ARC_SORT_ALPHABETICAL: u32 = 0;
/// [`ArcSettings::sort_order`]: digit-aware ordering (`page2` before
/// `page10`), the default on both platforms.
pub const ARC_SORT_NATURAL: u32 = 1;

/// [`ArcSettings::cover_mode`]: ignore cover-named entries.
pub const ARC_COVER_IGNORE: u32 = 0;
/// [`ArcSettings::cover_mode`]: prefer a cover-named entry, otherwise take
/// the first image by sort order.
pub const ARC_COVER_PREFER: u32 = 1;
/// [`ArcSettings::cover_mode`]: only accept a cover-named entry.
pub const ARC_COVER_ONLY: u32 = 2;

/// Settings snapshot handed to the core.
///
/// The extension owns the storage (it reads `UserDefaults`), which keeps
/// platform configuration out of the core. Passing null to
/// [`arc_thumbnail_generate`] uses the built-in defaults.
#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ArcSettings {
    /// `sizeof(ArcSettings)` from the caller, so the core can detect an
    /// older struct instead of reading fields that were never set.
    pub struct_size: u32,
    /// `ARC_SORT_*`.
    pub sort_order: u32,
    /// `ARC_COVER_*`.
    pub cover_mode: u32,
    /// Bitmask over the core's supported image extensions; a cleared bit
    /// makes that format ineligible as a cover. `u32::MAX` enables all.
    pub enabled_image_exts_mask: u32,
    /// Bitmask over the core's supported archive extensions; a cleared bit
    /// makes that container type produce no thumbnail at all. This is the
    /// macOS equivalent of Windows' per-extension `ShellEx` registration,
    /// where unchecking a type unbinds it from Explorer instead.
    pub enabled_archive_exts_mask: u32,
    /// Non-zero draws the format-coloured border.
    pub overlay_border: u32,
    /// Non-zero draws the `CBZ`/`EPUB`/… corner label.
    pub overlay_label: u32,
    /// Ignored: the log decision is process-wide and latched once by
    /// [`arc_log_enabled`].
    pub log_enabled: u32,
}

impl Default for ArcSettings {
    fn default() -> Self {
        Self {
            struct_size: std::mem::size_of::<Self>() as u32,
            sort_order: ARC_SORT_NATURAL,
            cover_mode: ARC_COVER_PREFER,
            enabled_image_exts_mask: default_enabled_image_exts_mask(),
            enabled_archive_exts_mask: arcthumb::settings::default_enabled_archive_exts_mask(),
            overlay_border: 0,
            overlay_label: 0,
            log_enabled: 0,
        }
    }
}

impl ArcSettings {
    /// Translate the C enums into the core's types. Out-of-range values are
    /// rejected rather than silently defaulted, so a caller-side typo
    /// surfaces in the log instead of as puzzling behaviour.
    fn to_core(self) -> Result<Settings, Box<dyn Error>> {
        let sort_order = match self.sort_order {
            ARC_SORT_ALPHABETICAL => SortOrder::Alphabetical,
            ARC_SORT_NATURAL => SortOrder::Natural,
            other => return Err(format!("unknown sort_order {other}").into()),
        };
        let cover_mode = match self.cover_mode {
            ARC_COVER_IGNORE => CoverMode::Ignore,
            ARC_COVER_PREFER => CoverMode::Prefer,
            ARC_COVER_ONLY => CoverMode::Only,
            other => return Err(format!("unknown cover_mode {other}").into()),
        };
        Ok(Settings {
            sort_order,
            cover_mode,
            enabled_image_exts_mask: self.enabled_image_exts_mask
                & default_enabled_image_exts_mask(),
            enabled_archive_exts_mask: self.enabled_archive_exts_mask
                & arcthumb::settings::default_enabled_archive_exts_mask(),
            overlay_border: self.overlay_border != 0,
            overlay_label: self.overlay_label != 0,
            ..Settings::default()
        })
    }
}

/// A rendered thumbnail: pixel buffer plus everything needed to read it.
///
/// A plain data struct rather than an opaque handle, so the caller can
/// touch the fields directly. `data` points into an allocation owned by
/// this value and stays valid until [`arc_thumbnail_free`].
#[repr(C)]
#[derive(Debug, Clone, Copy)]
pub struct ArcThumbnail {
    /// `sizeof(ArcThumbnail)`, so a caller built against a different header
    /// notices before reading past the end.
    pub struct_size: u32,
    pub width: u32,
    pub height: u32,
    /// Bytes per row.
    pub stride: u32,
    /// `ARC_PIXEL_FORMAT_*`.
    pub format: u32,
    /// Length of `data` in bytes.
    pub data_len: u32,
    /// Premultiplied RGBA8, `stride` bytes per row.
    pub data: *const u8,
}

/// Internal owner of an [`ArcThumbnail`]'s pixels.
///
/// `thumb` must stay the first field: [`arc_thumbnail_free`] receives the
/// pointer the caller saw (that of `thumb`) and casts it back to this
/// struct, which only holds because `#[repr(C)]` puts field zero at offset
/// zero. Asserted by `struct_layout_matches_the_c_header`.
#[repr(C)]
struct OwnedThumbnail {
    thumb: ArcThumbnail,
    pixels: Vec<u8>,
}

/// Core version as a NUL-terminated C string, built at compile time.
static VERSION: &[u8] = concat!("v", env!("CARGO_PKG_VERSION"), "\0").as_bytes();

thread_local! {
    static LAST_ERROR: RefCell<String> = const { RefCell::new(String::new()) };
}

fn set_error(msg: &str) {
    LAST_ERROR.with(|e| *e.borrow_mut() = msg.to_string());
}

fn clear_error() {
    LAST_ERROR.with(|e| e.borrow_mut().clear());
}

/// Run `f`, funneling panics and `Err`s into one path: return the neutral
/// value and keep a message.
fn guard<T: Copy, F: FnOnce() -> Result<T, Box<dyn Error>>>(neutral: T, f: F) -> T {
    match std::panic::catch_unwind(std::panic::AssertUnwindSafe(f)) {
        Ok(Ok(v)) => {
            clear_error();
            v
        }
        Ok(Err(e)) => {
            arcthumb::log::log(&format!("  arc: error: {e}"));
            set_error(&e.to_string());
            neutral
        }
        Err(_) => {
            arcthumb::log::log("  arc: PANIC caught at FFI boundary");
            set_error("internal error (panicked)");
            neutral
        }
    }
}

/// ABI version; the caller should compare it against its header.
#[unsafe(no_mangle)]
pub extern "C" fn arc_abi_version() -> u32 {
    ARC_ABI_VERSION
}

/// Core version string, e.g. `"v0.10.1"`. Points at static storage; do not
/// free.
#[unsafe(no_mangle)]
pub extern "C" fn arc_core_version() -> *const c_char {
    VERSION.as_ptr() as *const c_char
}

/// The core's built-in settings, for callers that want to override a field
/// or two.
///
/// A C or Swift caller that declares `ArcSettings s = {0}` gets *zero* for
/// `enabled_image_exts_mask`, which means "no image format is eligible" and
/// therefore no thumbnail at all. Seed from this instead of filling the
/// struct by hand.
#[unsafe(no_mangle)]
pub extern "C" fn arc_settings_default() -> ArcSettings {
    ArcSettings::default()
}

/// Turn the core's diagnostic logging on or off, before the first thumbnail
/// request.
///
/// Process-wide and irreversible ([`arcthumb::log::set_enabled`] latches on
/// the first call), because the core caches the decision; only the first
/// call has an effect.
#[unsafe(no_mangle)]
pub extern "C" fn arc_log_enabled(on: u32) {
    arcthumb::log::set_enabled(on != 0);
}

/// Copy the most recent error message into `buf` as UTF-8, truncated at a
/// char boundary to fit. No NUL terminator is written.
///
/// Returns the number of bytes written — 0 when the previous call
/// succeeded. A null or zero-length `buf` writes nothing and returns 0.
///
/// # Safety
///
/// `buf` must be null or point at `buf_len` writable bytes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn arc_last_error(buf: *mut c_char, buf_len: u32) -> u32 {
    if buf.is_null() || buf_len == 0 {
        return 0;
    }
    let out = unsafe { std::slice::from_raw_parts_mut(buf as *mut u8, buf_len as usize) };
    let written = LAST_ERROR.with(|e| {
        let msg = e.borrow();
        let mut n = out.len().min(msg.len());
        while !msg.is_char_boundary(n) {
            n -= 1;
        }
        out[..n].copy_from_slice(&msg.as_bytes()[..n]);
        n
    });
    written as u32
}

/// Generate a thumbnail for the archive at `path`.
///
/// * `path` — NUL-terminated UTF-8 filesystem path, never null.
/// * `max_width`, `max_height` — target box in **pixels** (DPI already
///   applied by the caller). Clamped to [`thumbnail::MIN_SIZE`]..=
///   [`thumbnail::MAX_SIZE`].
/// * `settings` — an [`ArcSettings`], or null for the defaults.
///
/// Returns an owned [`ArcThumbnail`], or null when no thumbnail could be
/// produced: not an archive, no eligible image, encrypted or corrupt
/// container, undecodable cover, or a source tripping a safety limit. Null
/// is an ordinary outcome — the host should show its own icon. On null,
/// [`arc_last_error`] explains why. Free a non-null result with
/// [`arc_thumbnail_free`].
///
/// # Safety
///
/// `path` must be a valid NUL-terminated C string and `settings` must be
/// null or point at a live, aligned `ArcSettings` whose `struct_size` is at
/// least `sizeof(ArcSettings)`. Neither needs to outlive the call.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn arc_thumbnail_generate(
    path: *const c_char,
    max_width: u32,
    max_height: u32,
    settings: *const ArcSettings,
) -> *mut ArcThumbnail {
    if path.is_null() {
        set_error("path is null");
        return std::ptr::null_mut();
    }
    // Checked outside the panic guard: `CStr::from_ptr` on a dangling
    // pointer is the caller's undefined behaviour, but a non-UTF-8 path is
    // merely an error worth reporting.
    let cpath = unsafe { CStr::from_ptr(path) };
    if cpath.to_str().is_err() {
        set_error("path is not valid UTF-8");
        return std::ptr::null_mut();
    }
    let settings = match unsafe { read_settings(settings) } {
        Ok(s) => s,
        Err(e) => {
            set_error(&e.to_string());
            return std::ptr::null_mut();
        }
    };

    let null = std::ptr::null_mut::<ArcThumbnail>();
    guard(null, || {
        let (mut thumb, pixels) = generate(cpath, max_width, max_height, &settings)?;
        thumb.data = pixels.as_ptr();
        let owned = OwnedThumbnail { thumb, pixels };
        // `thumb` is field zero of `OwnedThumbnail`, so the same address
        // serves as both the readable struct and the allocation to free.
        Ok(Box::into_raw(Box::new(owned)) as *mut ArcThumbnail)
    })
}

/// Validate and copy the caller's settings struct.
///
/// # Safety
///
/// `p` must be null or point at a live, aligned `ArcSettings`.
unsafe fn read_settings(p: *const ArcSettings) -> Result<ArcSettings, Box<dyn Error>> {
    if p.is_null() {
        return Ok(ArcSettings::default());
    }
    let s = unsafe { *p };
    if (s.struct_size as usize) < std::mem::size_of::<ArcSettings>() {
        return Err(format!(
            "ArcSettings too small: caller sent {}, core expects {}",
            s.struct_size,
            std::mem::size_of::<ArcSettings>()
        )
        .into());
    }
    Ok(s)
}

fn generate(
    path: &CStr,
    max_width: u32,
    max_height: u32,
    settings: &ArcSettings,
) -> Result<(ArcThumbnail, Vec<u8>), Box<dyn Error>> {
    let path_str = path.to_str()?;
    if path_str.is_empty() {
        return Err("empty path".into());
    }
    let core_settings = settings.to_core()?;
    // Echo the settings that actually crossed the boundary: when a
    // preference "does nothing", the first question is whether the host
    // read it at all (sandboxed UserDefaults has its own domain).
    arcthumb::log::log(&format!(
        "---- arc_thumbnail_generate {path_str} (sort={:?} cover={:?} mask={:#b} border={} label={}) ----",
        core_settings.sort_order,
        core_settings.cover_mode,
        core_settings.enabled_image_exts_mask,
        core_settings.overlay_border,
        core_settings.overlay_label,
    ));

    let width = thumbnail::clamp_size(max_width);
    let height = thumbnail::clamp_size(max_height);

    // Name the file in the error: `io::Error`'s Display leaves it out, and
    // "No such file or directory" in a log with no path is useless when
    // Finder is asking about one of fifty archives.
    let file = std::fs::File::open(path_str)
        .map_err(|e| -> Box<dyn Error> { format!("{path_str}: {e}").into() })?;
    let thumbnail::Thumbnail { image, .. } = thumbnail::render(
        std::io::BufReader::new(file),
        width,
        height,
        extension_of(path_str).as_deref(),
        &core_settings,
    )?;

    let (w, h) = (image.width(), image.height());
    let mut pixels = image.into_raw();
    arcthumb::pixel::premultiply_rgba8(&mut pixels);
    arcthumb::log::log(&format!(
        "  arc: {path_str} -> {w}x{h} rgba8 (premultiplied)"
    ));

    let thumb = ArcThumbnail {
        struct_size: std::mem::size_of::<ArcThumbnail>() as u32,
        width: w,
        height: h,
        stride: w.saturating_mul(4),
        format: ARC_PIXEL_FORMAT_RGBA8_PREMULTIPLIED,
        data_len: pixels.len() as u32,
        data: std::ptr::null(), // filled in once `pixels` has its owner
    };
    Ok((thumb, pixels))
}

/// Lowercased extension without the dot, so the overlay can label a `.cbz`
/// as "CBZ" rather than the generic "ZIP".
fn extension_of(path: &str) -> Option<String> {
    let base = path.rsplit('/').next().unwrap_or(path);
    let (_, ext) = base.rsplit_once('.')?;
    if ext.is_empty() || ext.len() > 8 {
        return None;
    }
    Some(ext.to_ascii_lowercase())
}

/// Release a thumbnail returned by [`arc_thumbnail_generate`], returning its
/// pixel buffer to the allocator. Null is a no-op.
///
/// CoreGraphics calls this exactly once, from the data provider's release
/// callback.
///
/// # Safety
///
/// `thumb` must be null or a pointer from [`arc_thumbnail_generate`] that
/// has not already been freed.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn arc_thumbnail_free(thumb: *mut ArcThumbnail) {
    if !thumb.is_null() {
        drop(unsafe { Box::from_raw(thumb as *mut OwnedThumbnail) });
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::ffi::CString;

    /// The hand-written C header mirrors this file; these are the numbers a
    /// mismatch would silently corrupt.
    #[test]
    fn struct_layout_matches_the_c_header() {
        assert_eq!(std::mem::size_of::<ArcSettings>(), 32);
        assert_eq!(
            std::mem::size_of::<ArcThumbnail>(),
            std::mem::size_of::<u32>() * 6 + std::mem::size_of::<*const u8>(),
            "six u32 fields plus one pointer, no padding surprises"
        );
        assert_eq!(
            std::mem::offset_of!(OwnedThumbnail, thumb),
            0,
            "arc_thumbnail_free casts through this offset"
        );
    }

    fn generate_for(path: &str, settings: &ArcSettings) -> *mut ArcThumbnail {
        let c = CString::new(path).unwrap();
        unsafe { arc_thumbnail_generate(c.as_ptr(), 64, 64, settings as *const ArcSettings) }
    }

    fn take_error() -> String {
        let mut buf = [0i8; 256];
        let n = unsafe { arc_last_error(buf.as_mut_ptr(), buf.len() as u32) } as usize;
        let bytes: Vec<u8> = buf[..n].iter().map(|&c| c as u8).collect();
        String::from_utf8_lossy(&bytes).into_owned()
    }

    #[test]
    fn null_path_is_reported_not_crashed() {
        let s = ArcSettings::default();
        let p =
            unsafe { arc_thumbnail_generate(std::ptr::null(), 64, 64, &s as *const ArcSettings) };
        assert!(p.is_null());
        assert!(take_error().contains("null"));
    }

    #[test]
    fn missing_file_yields_no_thumbnail_and_a_message() {
        let s = ArcSettings::default();
        assert!(generate_for("/nonexistent/arcthumb-test.cbz", &s).is_null());
        assert!(take_error().contains("arcthumb-test"));
    }

    #[test]
    fn undersized_settings_struct_is_rejected() {
        let s = ArcSettings {
            struct_size: 4,
            ..ArcSettings::default()
        };
        assert!(generate_for("/tmp/whatever.cbz", &s).is_null());
        assert!(take_error().contains("too small"));
    }

    #[test]
    fn unknown_enum_values_are_rejected() {
        let bad_cover = ArcSettings {
            cover_mode: 99,
            ..ArcSettings::default()
        };
        assert!(bad_cover.to_core().is_err());
        let bad_sort = ArcSettings {
            sort_order: 7,
            ..ArcSettings::default()
        };
        assert!(bad_sort.to_core().is_err());
    }

    /// The trap the macOS extension almost walked into: a zero-initialised
    /// C struct is a *valid* `ArcSettings` whose mask rejects every image.
    #[test]
    fn zero_filled_struct_means_no_eligible_images() {
        // What `{0}` looks like in C: `struct_size` filled in (the only
        // field a caller is likely to set) and everything else zero.
        let zeroed = ArcSettings {
            struct_size: std::mem::size_of::<ArcSettings>() as u32,
            sort_order: ARC_SORT_NATURAL,
            cover_mode: 0,
            enabled_image_exts_mask: 0,
            enabled_archive_exts_mask: 0,
            overlay_border: 0,
            overlay_label: 0,
            log_enabled: 0,
        };
        let settings = zeroed.to_core().unwrap();
        assert_eq!(settings.enabled_image_exts_mask, 0);
        assert!(
            !settings.accepts_image_ext("cover.jpg"),
            "a zero mask must make every extension ineligible"
        );
        // …which is why `arc_settings_default` exists.
        assert!(
            arc_settings_default()
                .to_core()
                .unwrap()
                .accepts_image_ext("cover.jpg")
        );
    }

    #[test]
    fn archive_mask_round_trips_and_clamps() {
        // Clearing the `cbz` bit must reach the core, and bits beyond the
        // supported list must be dropped rather than trusted.
        let cbz_bit = 1u32 << 1;
        let s = ArcSettings {
            enabled_archive_exts_mask: !cbz_bit,
            ..ArcSettings::default()
        };
        let core = s.to_core().unwrap();
        assert!(!core.accepts_archive_ext(Some("cbz")));
        assert!(core.accepts_archive_ext(Some("zip")));
        let junk = ArcSettings {
            enabled_archive_exts_mask: u32::MAX,
            ..ArcSettings::default()
        };
        assert_eq!(
            junk.to_core().unwrap().enabled_archive_exts_mask,
            arcthumb::settings::default_enabled_archive_exts_mask()
        );
    }

    #[test]
    fn null_settings_mean_defaults() {
        let s = unsafe { read_settings(std::ptr::null()) }.unwrap();
        assert_eq!(s, ArcSettings::default());
        assert_eq!(s.sort_order, ARC_SORT_NATURAL);
        assert_eq!(s.cover_mode, ARC_COVER_PREFER);
    }

    #[test]
    fn extension_of_mirrors_the_core_rules() {
        assert_eq!(extension_of("/x/test.cbz").as_deref(), Some("cbz"));
        assert_eq!(extension_of("/x/TEST.CBZ").as_deref(), Some("cbz"));
        assert_eq!(extension_of("/x/noext"), None);
        assert_eq!(extension_of("/x/toolongabcdefg"), None);
        assert_eq!(extension_of("/x/trailing."), None);
    }

    #[test]
    fn version_string_is_nul_terminated() {
        assert_eq!(VERSION.last(), Some(&0u8));
        let v = unsafe { CStr::from_ptr(arc_core_version()) };
        assert!(v.to_str().unwrap().starts_with('v'));
    }

    #[test]
    fn last_error_truncates_on_char_boundary() {
        set_error("损坏的压缩包");
        let mut buf = [0u8; 7];
        let n = unsafe { arc_last_error(buf.as_mut_ptr() as *mut c_char, buf.len() as u32) };
        let s = String::from_utf8(buf[..n as usize].to_vec()).expect("valid utf8 prefix");
        assert!(!s.is_empty());
    }
}
