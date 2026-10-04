//! The Windows side of the panel: registry in, registry out.
//!
//! This module used to be widget wiring: property setters, callbacks, model
//! plumbing, and round-trip tests that could only run behind a GUI backend
//! pinned to one thread. The widget tree is now the shared Dioxus view in
//! `app/`, so what is left here is only what is genuinely Windows-specific:
//! reading a `UiModel` out of the registry, folding an edited [`Panel`] back
//! into an apply plan, and clearing Explorer's thumbnail cache.
//!
//! The plan/apply logic itself stays in `apply.rs`, untouched: it was already
//! platform-neutral and already tested, and the panel is just a new caller.

use arcthumb::registry::EXTENSIONS;
use arcthumb::settings::{SUPPORTED_IMAGE_EXTS, Settings};

use crate::app::panel::Panel;
use crate::apply::{self, RealRegistryOps};
use crate::cache;
use crate::state::{self, EXT_COUNT, UiModel};

/// Snapshot the live registry into the view model.
pub fn load_panel() -> Panel {
    Panel::from(&UiModel::load())
}

impl From<&UiModel> for Panel {
    fn from(model: &UiModel) -> Self {
        let model_settings = &model.settings;
        Panel {
            // Index order is bit order on both grids, and `EXTENSIONS` is the list
            // the registry layer binds against — so the rows cannot drift from what
            // Explorer is actually told.
            archive: Panel::from_flags(EXTENSIONS, &model.ext_enabled),
            image: Panel::from_flags(SUPPORTED_IMAGE_EXTS, &model.image_ext_enabled),
            sort: model_settings.sort_order,
            cover: model_settings.cover_mode,
            preview: model.preview_enabled,
            border: model_settings.overlay_border,
            label: model_settings.overlay_label,
            log: model_settings.log_enabled,
        }
    }
}

/// Which hive the dialog is editing. Reported to the panel and used to pick the
/// `RealRegistryOps` scope, so an Apply on a machine install does not silently
/// bifurcate into HKCU.
pub fn scope() -> arcthumb::registry::Scope {
    UiModel::load().scope
}

/// Write the panel back. Returns the technical details of anything that failed;
/// the caller prefixes them with the localized "failed to save" sentence, so a
/// locked hive and a refused registration read differently on screen.
pub fn save_panel(panel: &Panel) -> Result<(), String> {
    let current = UiModel::load();
    let ext_enabled: [bool; EXT_COUNT] = Panel::flags(&panel.archive);
    let image_flags: Vec<bool> = panel.image.iter().map(|row| row.on).collect();

    let settings = Settings {
        sort_order: panel.sort,
        cover_mode: panel.cover,
        enabled_image_exts_mask: state::image_ext_vec_to_mask(&image_flags),
        overlay_border: panel.border,
        overlay_label: panel.label,
        log_enabled: panel.log,
        // `enabled_archive_exts_mask` deliberately stays at its default: on
        // Windows the per-extension checkboxes drive the `ShellEx` bindings, so
        // the mask remains all-on and only the macOS backend filters with it.
        ..Settings::default()
    };

    let plan = apply::compute_apply_plan(&current, settings, ext_enabled, panel.preview);
    let outcome = apply::apply_plan(&plan, &RealRegistryOps::new(current.scope));

    let mut errors = Vec::new();
    if let Some(detail) = outcome.settings_save_error {
        errors.push(detail);
    }
    if !outcome.failed_extensions.is_empty() {
        errors.push(format!("failed: {}", outcome.failed_extensions.join(", ")));
    }
    if let Some(detail) = outcome.preview_error {
        errors.push(detail);
    }
    if errors.is_empty() {
        Ok(())
    } else {
        Err(errors.join("; "))
    }
}

/// Clear Explorer's icon and thumbnail caches, and restart the shell.
/// A partially-wiped cache is reported as a failure with the count of locked
/// files, which is the only part of the outcome the user can act on.
pub fn regenerate() -> Result<(), String> {
    match cache::wipe_thumbnail_cache() {
        Ok(report) if report.failed.is_empty() => Ok(()),
        Ok(report) => Err(format!(
            "{} cache file(s) were locked and could not be deleted",
            report.failed.len()
        )),
        Err(detail) => Err(detail),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use arcthumb::registry::Scope;

    fn model_with(ext_on: &[usize]) -> UiModel {
        let mut ext_enabled = [false; EXT_COUNT];
        for &i in ext_on {
            ext_enabled[i] = true;
        }
        let settings = Settings::default();
        UiModel {
            image_ext_enabled: state::image_ext_mask_to_vec(settings.enabled_image_exts_mask),
            settings,
            scope: Scope::PerUser,
            ext_enabled,
            preview_enabled: true,
        }
    }

    /// Re-loading what the panel just read must produce an empty apply plan, so
    /// a stray OK click cannot touch HKCU.
    #[test]
    fn an_unchanged_panel_produces_an_empty_plan() {
        let model = model_with(&[0, 2, 7]);
        let panel = Panel::from(&model);

        let ext_enabled: [bool; EXT_COUNT] = Panel::flags(&panel.archive);
        let image_flags: Vec<bool> = panel.image.iter().map(|row| row.on).collect();
        let settings = Settings {
            sort_order: panel.sort,
            cover_mode: panel.cover,
            enabled_image_exts_mask: state::image_ext_vec_to_mask(&image_flags),
            overlay_border: panel.border,
            overlay_label: panel.label,
            log_enabled: panel.log,
            ..Settings::default()
        };
        let plan = apply::compute_apply_plan(&model, settings, ext_enabled, panel.preview);
        assert!(
            plan.is_empty(),
            "round trip should change nothing, got {plan:?}"
        );
    }

    /// The grids are index-for-index parallel to the registry lists; if
    /// `EXTENSIONS` grew past the panel's row count the mask would silently
    /// lose a type.
    #[test]
    fn every_extension_has_a_row() {
        let model = model_with(&[0]);
        let panel = Panel::from(&model);
        assert_eq!(panel.archive.len(), EXTENSIONS.len());
        assert_eq!(panel.image.len(), SUPPORTED_IMAGE_EXTS.len());
    }
}
