//! The macOS side of the panel: the Quick Look extension's settings file in,
//! settings file out.
//!
//! Split from the Windows host on purpose. That side writes registry keys,
//! (un)registers `ShellEx` handlers and restarts Explorer — none of which exist
//! here — and folding both into one file meant threading `#[cfg]` through every
//! one of those calls and putting the Windows install path at risk for a
//! cosmetic gain. What the two front ends share is now the view, the string
//! tables and [`Panel`], which is what makes the windows identical.
//!
//! | Row | Windows | macOS |
//! |---|---|---|
//! | Enabled extensions | `ShellEx\<ext>` bindings | bitmask in the extension's settings file; a cleared type produces no thumbnail |
//! | Image formats | `EnabledImageExts` | same field, same meaning |
//! | Sort / cover | registry | settings file |
//! | Preview pane | registers `IPreviewHandler` | drawn inert — Finder has no counterpart |
//! | Border / label overlay | registry | settings file |
//! | Diagnostic logging | registry | settings file |
//! | Language / theme | `HKCU\Software\ArcThumb` | the same file, keys the extension ignores |
//! | Regenerate thumbnails | clears the Explorer icon cache | `qlmanage -r cache` |

use arcthumb::settings::{SUPPORTED_ARCHIVE_EXTS, SUPPORTED_IMAGE_EXTS};

use crate::app::panel::{Panel, Row, Theme};
use crate::locale::Locale;
use crate::settings_store::{self, StoredSettings};

/// Read the extension's settings file into the view model.
///
/// The archive list is drawn with a leading dot, matching how Finder spells
/// them in the UI; the mask index still follows
/// `arcthumb::settings::SUPPORTED_ARCHIVE_EXTS` order, which is what the
/// extension filters on.
pub fn load_panel() -> Panel {
    panel_from(&settings_store::load())
}

/// The mapping itself, so the tests below can drive it without a container.
fn panel_from(stored: &StoredSettings) -> Panel {
    Panel {
        archive: dotted(SUPPORTED_ARCHIVE_EXTS, stored.enabled_archive_exts),
        image: Panel::from_masks(SUPPORTED_IMAGE_EXTS, stored.enabled_image_exts),
        sort: stored.sort_order,
        cover: stored.cover_mode,
        // There is no preview handler here, and a switch that could be on but
        // does nothing would be a lie, so it loads off and is drawn inert.
        preview: false,
        border: stored.overlay_border,
        label: stored.overlay_label,
        log: stored.log_enabled,
    }
}

fn dotted(names: &[&'static str], mask: u32) -> Vec<Row> {
    Panel::from_masks(names, mask)
        .into_iter()
        .map(|mut row| {
            row.name = format!(".{}", row.name);
            row
        })
        .collect()
}

/// Fold the panel back into the stored form. Language and theme are preserved
/// from the file: this writes the settings the extension reads, nothing else.
pub fn save_panel(panel: &Panel) -> Result<(), String> {
    let mut stored = settings_store::load();
    stored.sort_order = panel.sort;
    stored.cover_mode = panel.cover;
    stored.enabled_image_exts = panel.image_mask();
    stored.enabled_archive_exts = panel.archive_mask();
    stored.overlay_border = panel.border;
    stored.overlay_label = panel.label;
    stored.log_enabled = panel.log;
    settings_store::save(&stored).map_err(|e| e.to_string())
}

/// The two GUI-only preferences. Unlike the settings above they are written
/// back through `settings_store`, so `--get` and the panel can never disagree
/// about which language the last run chose.
pub fn save_prefs(locale: Locale, theme: Theme) -> Result<(), String> {
    let mut stored = settings_store::load();
    stored.language = Some(locale.tag().to_string());
    stored.theme = Some(theme.tag().to_string());
    settings_store::save(&stored).map_err(|e| e.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;
    use arcthumb::settings::{CoverMode, SortOrder, default_enabled_archive_exts_mask};

    fn sample() -> StoredSettings {
        StoredSettings {
            sort_order: SortOrder::Alphabetical,
            cover_mode: CoverMode::Ignore,
            enabled_image_exts: 0b101,
            enabled_archive_exts: default_enabled_archive_exts_mask(),
            overlay_border: true,
            overlay_label: false,
            log_enabled: true,
            language: Some("zh".into()),
            theme: Some("light".into()),
        }
    }

    #[test]
    fn archive_rows_are_dotted_and_bit_aligned() {
        let panel = panel_from(&sample());
        assert_eq!(panel.archive.len(), SUPPORTED_ARCHIVE_EXTS.len());
        assert!(
            panel.archive[0].name.starts_with('.'),
            "Finder users see dotted types: {}",
            panel.archive[0].name
        );
        // Bit order must survive the dot, or the mask means a different type.
        assert_eq!(panel.archive_mask(), sample().enabled_archive_exts);
    }

    #[test]
    fn panel_round_trips_every_field_the_file_carries() {
        let stored = sample();
        let panel = panel_from(&stored);
        let mut back = stored.clone();
        back.sort_order = panel.sort;
        back.cover_mode = panel.cover;
        back.enabled_image_exts = panel.image_mask();
        back.enabled_archive_exts = panel.archive_mask();
        back.overlay_border = panel.border;
        back.overlay_label = panel.label;
        back.log_enabled = panel.log;
        assert_eq!(
            back, stored,
            "nothing in the file may be lost on a round trip"
        );
    }

    #[test]
    fn preview_always_loads_off() {
        let mut stored = sample();
        stored.log_enabled = true;
        assert!(
            !panel_from(&stored).preview,
            "Finder has no preview handler here"
        );
    }

    #[test]
    fn prefs_survive_the_text_format() {
        let stored = sample();
        let text = stored.to_text();
        let parsed = StoredSettings::parse(&text);
        assert_eq!(parsed.language.as_deref(), Some("zh"));
        assert_eq!(parsed.theme.as_deref(), Some("light"));
        // An older file without the two keys must still parse, with the panel
        // falling back rather than failing.
        let stripped = StoredSettings::parse("sort_order = 1\nlog_enabled = 1\n");
        assert_eq!(stripped.language, None);
        assert!(stripped.log_enabled);
    }
}
