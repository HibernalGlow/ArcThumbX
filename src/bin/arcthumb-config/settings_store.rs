//! Where the macOS config GUI and the Quick Look extension exchange
//! settings.
//!
//! The extension is sandboxed, so its preferences live in *its* container
//! and an unsandboxed helper cannot reach them through the normal
//! `UserDefaults` domain: `CFPreferences` with the extension's bundle id
//! writes `~/Library/Preferences/…`, which the sandbox never reads. Rather
//! than fight that, the config app writes one small file into the
//! extension's container and the extension reads it on each request:
//!
//! ```text
//! ~/Library/Containers/com.citrussoda.ArcThumb.thumbnail/Data/
//!     Library/Application Support/ArcThumb/settings
//! ```
//!
//! The format is deliberately `key = value` lines rather than JSON: no new
//! dependency in the shipped DLL, `cat` shows you the live configuration,
//! and an unrecognised key is ignored instead of failing the whole file —
//! which is what lets an older extension read a newer config.
//!
//! Key names mirror the Windows registry values under
//! `HKCU\Software\ArcThumb` so one mental model covers both platforms.

use std::error::Error;
use std::path::{Path, PathBuf};

use arcthumb::settings::{
    CoverMode, Settings, SortOrder, default_enabled_archive_exts_mask,
    default_enabled_image_exts_mask,
};

/// Bundle id of the Quick Look extension, i.e. the container we write into.
pub const EXTENSION_BUNDLE_ID: &str = "com.citrussoda.ArcThumb.thumbnail";

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StoredSettings {
    pub sort_order: SortOrder,
    pub cover_mode: CoverMode,
    /// Bitmask over [`SUPPORTED_IMAGE_EXTS`].
    pub enabled_image_exts: u32,
    /// Bitmask over [`SUPPORTED_ARCHIVE_EXTS`].
    pub enabled_archive_exts: u32,
    pub overlay_border: bool,
    pub overlay_label: bool,
    pub log_enabled: bool,
}

impl Default for StoredSettings {
    fn default() -> Self {
        Self {
            sort_order: SortOrder::Natural,
            cover_mode: CoverMode::Prefer,
            enabled_image_exts: default_enabled_image_exts_mask(),
            enabled_archive_exts: default_enabled_archive_exts_mask(),
            overlay_border: false,
            overlay_label: false,
            log_enabled: false,
        }
    }
}

impl StoredSettings {
    pub fn to_core(&self) -> Settings {
        Settings {
            sort_order: self.sort_order,
            cover_mode: self.cover_mode,
            enabled_image_exts_mask: self.enabled_image_exts & default_enabled_image_exts_mask(),
            enabled_archive_exts_mask: self.enabled_archive_exts
                & default_enabled_archive_exts_mask(),
            overlay_border: self.overlay_border,
            overlay_label: self.overlay_label,
            log_enabled: self.log_enabled,
        }
    }

    /// Serialise to the on-disk form, with a header comment so a user who
    /// finds the file knows what they are looking at.
    pub fn to_text(&self) -> String {
        let mut out = String::from(
            "# ArcThumb — Finder thumbnail extension settings.\n\
             # Written by ArcThumb.app; safe to edit, but the extension\n\
             # re-reads this file on every request.\n",
        );
        out.push_str(&format!(
            "sort_order = {}\n",
            self.sort_order.as_registry_value()
        ));
        out.push_str(&format!(
            "cover_mode = {}\n",
            self.cover_mode.as_registry_value()
        ));
        out.push_str(&format!(
            "enabled_image_exts = {}\n",
            self.enabled_image_exts
        ));
        out.push_str(&format!(
            "enabled_archive_exts = {}\n",
            self.enabled_archive_exts
        ));
        out.push_str(&format!("overlay_border = {}\n", flag(self.overlay_border)));
        out.push_str(&format!("overlay_label = {}\n", flag(self.overlay_label)));
        out.push_str(&format!("log_enabled = {}\n", flag(self.log_enabled)));
        out
    }

    /// Parse the on-disk form. Unknown keys and malformed values are
    /// skipped, never fatal: a bad line must not cost the user every
    /// thumbnail in Finder.
    pub fn parse(text: &str) -> Self {
        let mut out = Self::default();
        for line in text.lines() {
            let line = line.trim();
            if line.is_empty() || line.starts_with('#') {
                continue;
            }
            let Some((key, value)) = line.split_once('=') else {
                continue;
            };
            let (key, value) = (key.trim(), value.trim());
            match key {
                "sort_order" => {
                    if let Some(order) = parse_sort(value) {
                        out.sort_order = order;
                    }
                }
                "cover_mode" => {
                    if let Some(mode) = parse_cover(value) {
                        out.cover_mode = mode;
                    }
                }
                "enabled_image_exts" => {
                    if let Ok(mask) = value.parse::<u32>() {
                        out.enabled_image_exts = mask;
                    }
                }
                "enabled_archive_exts" => {
                    if let Ok(mask) = value.parse::<u32>() {
                        out.enabled_archive_exts = mask;
                    }
                }
                "overlay_border" => out.overlay_border = parse_flag(value),
                "overlay_label" => out.overlay_label = parse_flag(value),
                "log_enabled" => out.log_enabled = parse_flag(value),
                _ => {}
            }
        }
        out
    }
}

fn flag(on: bool) -> u8 {
    u8::from(on)
}

fn parse_flag(value: &str) -> bool {
    matches!(value, "1" | "true" | "yes" | "on")
}

fn parse_sort(value: &str) -> Option<SortOrder> {
    match value.to_ascii_lowercase().as_str() {
        "alphabetical" | "alpha" => Some(SortOrder::Alphabetical),
        "natural" | "nat" => Some(SortOrder::Natural),
        _ => None,
    }
}

fn parse_cover(value: &str) -> Option<CoverMode> {
    match value.to_ascii_lowercase().as_str() {
        "ignore" => Some(CoverMode::Ignore),
        "prefer" => Some(CoverMode::Prefer),
        "only" => Some(CoverMode::Only),
        _ => None,
    }
}

/// `~/Library/Containers/<extension>/Data`, or `None` when `$HOME` cannot be
/// resolved (i.e. we are not running as a user process).
fn extension_container() -> Option<PathBuf> {
    let home = std::env::var_os("HOME")?;
    Some(
        Path::new(&home)
            .join("Library/Containers")
            .join(EXTENSION_BUNDLE_ID)
            .join("Data"),
    )
}

/// Full path of the settings file. Its parent directory is created on
/// demand: a fresh install has never had one.
pub fn settings_path() -> Result<PathBuf, Box<dyn Error>> {
    let container = extension_container().ok_or("HOME is not set")?;
    Ok(container
        .join("Library/Application Support/ArcThumb")
        .join("settings"))
}

/// Read the extension's settings, or the defaults when no file exists yet
/// (nobody has opened the config GUI on this account).
pub fn load() -> StoredSettings {
    settings_path()
        .ok()
        .map(|path| load_at(&path))
        .unwrap_or_default()
}

/// [`load`] against an explicit path, so the parsing and the write protocol
/// can be tested without redirecting `$HOME`.
pub fn load_at(path: &Path) -> StoredSettings {
    std::fs::read_to_string(path)
        .map(|text| StoredSettings::parse(&text))
        .unwrap_or_default()
}

/// Write atomically — temp file in the same directory, then rename — so the
/// extension can never observe a half-written file.
pub fn save(settings: &StoredSettings) -> Result<(), Box<dyn Error>> {
    save_at(&settings_path()?, settings)
}

/// [`save`] against an explicit path.
pub fn save_at(path: &Path, settings: &StoredSettings) -> Result<(), Box<dyn Error>> {
    let dir = path.parent().ok_or("settings path has no parent")?;
    std::fs::create_dir_all(dir)?;
    let tmp = dir.join(format!("settings.tmp.{}", std::process::id()));
    std::fs::write(&tmp, settings.to_text())?;
    std::fs::rename(&tmp, path)?;
    Ok(())
}

/// Ask Quick Look to drop its cached thumbnails so Finder re-asks the
/// extension. macOS has no public API for this; `qlmanage -r cache` is the
/// supported CLI, and the counterpart of what the Windows GUI does through
/// the Explorer icon cache.
pub fn regenerate_thumbnails() -> Result<(), Box<dyn Error>> {
    let status = std::process::Command::new("/usr/bin/qlmanage")
        .args(["-r", "cache"])
        .status()?;
    if !status.success() {
        return Err(format!("qlmanage -r cache exited with {status}").into());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use arcthumb::settings::{SUPPORTED_ARCHIVE_EXTS, SUPPORTED_IMAGE_EXTS};

    #[test]
    fn defaults_match_the_core_defaults() {
        assert_eq!(StoredSettings::default().to_core(), Settings::default());
    }

    #[test]
    fn text_round_trip_preserves_every_field() {
        let original = StoredSettings {
            sort_order: SortOrder::Alphabetical,
            cover_mode: CoverMode::Only,
            enabled_image_exts: 0b101,
            enabled_archive_exts: 0b1010,
            overlay_border: true,
            overlay_label: false,
            log_enabled: true,
        };
        let text = original.to_text();
        assert_eq!(StoredSettings::parse(&text), original);
    }

    #[test]
    fn junk_values_degrade_to_defaults_instead_of_failing() {
        let parsed = StoredSettings::parse(
            "sort_order = sideways\ncover_mode = banana\nenabled_image_exts = not-a-number\n\
             overlay_border = maybe\nunknown_key = whatever\n",
        );
        assert_eq!(parsed, StoredSettings::default());
    }

    #[test]
    fn missing_keys_use_field_defaults() {
        // Written by an older config app that predates the archive mask.
        let parsed = StoredSettings::parse("cover_mode = only\n");
        assert_eq!(parsed.cover_mode, CoverMode::Only);
        assert_eq!(parsed.sort_order, SortOrder::Natural);
        assert_eq!(
            parsed.enabled_archive_exts,
            default_enabled_archive_exts_mask()
        );
    }

    #[test]
    fn masks_are_clamped_to_the_supported_bits() {
        let stored = StoredSettings {
            enabled_image_exts: u32::MAX,
            enabled_archive_exts: u32::MAX,
            ..StoredSettings::default()
        };
        let core = stored.to_core();
        assert_eq!(
            core.enabled_image_exts_mask,
            default_enabled_image_exts_mask()
        );
        assert_eq!(
            core.enabled_archive_exts_mask,
            default_enabled_archive_exts_mask()
        );
    }

    #[test]
    fn row_order_matches_the_core_lists() {
        // The GUI builds one row per entry in these constants and treats the
        // row index as the bit index, so the two must stay in lockstep.
        assert_eq!(SUPPORTED_ARCHIVE_EXTS[0], "zip");
        assert_eq!(SUPPORTED_ARCHIVE_EXTS[1], "cbz");
        assert_eq!(SUPPORTED_IMAGE_EXTS[0], ".jpg");
        assert_eq!(SUPPORTED_IMAGE_EXTS[2], ".png");
    }

    #[test]
    fn settings_path_points_at_the_extension_container() {
        let path = settings_path().expect("HOME is set in tests");
        let text = path.display().to_string();
        assert!(text.contains(EXTENSION_BUNDLE_ID), "got {text}");
        assert!(text.ends_with("/settings"), "got {text}");
    }

    #[test]
    fn save_and_load_round_trip_through_a_file() {
        // An explicit path rather than a redirected $HOME: mutating the
        // process environment from a parallel test is UB in edition 2024.
        let dir = std::env::temp_dir().join(format!(
            "arcthumb_store_{}_{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        let path = dir.join("nested").join("settings");
        let wanted = StoredSettings {
            cover_mode: CoverMode::Only,
            enabled_archive_exts: 0b101,
            log_enabled: true,
            ..StoredSettings::default()
        };
        save_at(&path, &wanted).expect("save");
        let got = load_at(&path);
        std::fs::remove_dir_all(&dir).ok();
        assert_eq!(got, wanted);
    }

    #[test]
    fn a_missing_file_reads_as_the_defaults() {
        let missing = std::env::temp_dir().join("arcthumb-definitely-absent/settings");
        assert_eq!(load_at(&missing), StoredSettings::default());
    }
}
