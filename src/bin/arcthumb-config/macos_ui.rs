//! macOS front end for the same `ui/main.slint` window the Windows config
//! dialog uses.
//!
//! Deliberately a *separate* controller rather than a cross-platform edit of
//! `ui.rs`: `ui.rs` writes registry keys, (un)registers `ShellEx` handlers and
//! pokes Explorer, none of which exist here. Rewriting it in place would have
//! meant threading `#[cfg]` through every one of those calls and putting the
//! Windows install path at risk for a cosmetic gain. What the two front ends
//! share is the UI file, the string tables, and the settings model — which is
//! what makes the windows look identical.
//!
//! Mapping from the Windows dialog's rows:
//!
//! | Row | Windows | macOS |
//! |---|---|---|
//! | Enabled extensions | writes/removed `ShellEx\\<ext>` bindings | bitmask in the extension's settings file; a cleared type produces no thumbnail |
//! | Image formats | `EnabledImageExts` | same field, same meaning |
//! | Sort order / cover mode | registry | settings file |
//! | Preview pane | registers `IPreviewHandler` | shown disabled — Finder has no equivalent here yet |
//! | Border / label overlay | registry | settings file |
//! | Diagnostic logging | registry | settings file |
//! | Regenerate thumbnails | clears the Explorer icon cache | `qlmanage -r cache` |

slint::include_modules!();

use std::rc::Rc;

use slint::{ComponentHandle, Model, SharedString, VecModel};

use arcthumb::settings::{SUPPORTED_ARCHIVE_EXTS, SUPPORTED_IMAGE_EXTS};

use crate::locale::{self, Strings};
use crate::settings_store::{self, StoredSettings};

/// Entry point for `arcthumb-config` on macOS.
pub fn run() -> Result<(), Box<dyn std::error::Error>> {
    let strings = Rc::new(locale::for_macos(locale::current()));
    let window = MainWindow::new()?;
    let model = settings_store::load();

    apply_strings(&window, &strings);
    push(&window, &model);
    window.set_preview_available(false);

    let weak = window.as_weak();
    window.on_cancel_clicked(move || {
        if let Some(window) = weak.upgrade() {
            window.hide().ok();
        }
    });

    let weak = window.as_weak();
    window.on_exit_clicked(move || {
        if let Some(window) = weak.upgrade() {
            window.hide().ok();
        }
    });

    // ---- Apply / OK ----------------------------------------------------
    let weak = window.as_weak();
    let strings_for_apply = Rc::clone(&strings);
    window.on_apply_clicked({
        let strings_for_apply = Rc::clone(&strings_for_apply);
        move || {
            let Some(window) = weak.upgrade() else { return };
            let model = collect(&window);
            if let Err(e) = settings_store::save(&model) {
                notice(
                    &strings_for_apply,
                    strings_for_apply.error_title,
                    &format!("{}\n\n{e}", strings_for_apply.error_save),
                    None,
                );
            }
        }
    });

    let weak = window.as_weak();
    let strings_for_ok = Rc::clone(&strings);
    window.on_ok_clicked(move || {
        let Some(window) = weak.upgrade() else { return };
        let model = collect(&window);
        match settings_store::save(&model) {
            Ok(()) => {
                window.hide().ok();
            }
            Err(e) => notice(
                &strings_for_ok,
                strings_for_ok.error_title,
                &format!("{}\n\n{e}", strings_for_ok.error_save),
                None,
            ),
        }
    });

    // ---- Regenerate thumbnails (two-step, no blocking dialog) ----------
    let strings_for_regen = Rc::clone(&strings);
    window.on_regenerate_clicked(move || {
        let for_notice = Rc::clone(&strings_for_regen);
        let for_action = Rc::clone(&strings_for_regen);
        let confirm = for_notice.regen_confirm;
        let title = for_notice.error_title;
        notice(
            &for_notice,
            title,
            confirm,
            Some(Rc::new(
                move || match settings_store::regenerate_thumbnails() {
                    Ok(()) => {
                        let s = Rc::clone(&for_action);
                        notice(&s, s.error_title, s.regen_done, None);
                    }
                    Err(e) => {
                        let s = Rc::clone(&for_action);
                        let text = format!("{}\n\n{e}", s.regen_partial);
                        notice(&s, s.error_title, &text, None);
                    }
                },
            )),
        );
    });

    // ---- About ---------------------------------------------------------
    let strings_for_about = Rc::clone(&strings);
    window.on_about_clicked(move || {
        let strings = Rc::clone(&strings_for_about);
        if let Ok(dialog) = AboutDialog::new() {
            dialog.set_dialog_title(SharedString::from(strings.about_title));
            dialog.set_version_text(SharedString::from(version_text()));
            dialog.set_body_text(SharedString::from(strings.about_body));
            dialog.set_btn_close(SharedString::from(strings.btn_close));
            let weak = dialog.as_weak();
            dialog.on_close_clicked(move || {
                if let Some(dialog) = weak.upgrade() {
                    dialog.hide().ok();
                }
            });
            dialog.show().ok();
        }
    });

    // ---- Per-row toggles ------------------------------------------------
    let archive = Rc::new(VecModel::from(archive_rows(&model)));
    let image = Rc::new(VecModel::from(image_rows(&model)));
    window.set_extensions(archive.clone().into());
    window.set_image_extensions(image.clone().into());

    window.on_toggle_extension({
        let archive = Rc::clone(&archive);
        move |row: i32| flip(&archive, row)
    });
    window.on_toggle_image_extension({
        let image = Rc::clone(&image);
        move |row: i32| flip(&image, row)
    });

    window.show()?;
    slint::run_event_loop().map_err(Into::into)
}

/// Fold the window's current state into a settings struct.
fn collect(window: &MainWindow) -> StoredSettings {
    use arcthumb::settings::{CoverMode, SortOrder};
    let mut settings = settings_store::load();
    settings.sort_order = if window.get_sort_index() == 1 {
        SortOrder::Alphabetical
    } else {
        SortOrder::Natural
    };
    settings.cover_mode = match window.get_cover_mode() {
        1 => CoverMode::Only,
        2 => CoverMode::Ignore,
        _ => CoverMode::Prefer,
    };
    settings.overlay_border = window.get_overlay_border();
    settings.overlay_label = window.get_overlay_label();
    settings.log_enabled = window.get_log_enabled();
    settings.enabled_archive_exts = mask_of(&window.get_extensions());
    settings.enabled_image_exts = mask_of(&window.get_image_extensions());
    settings
}

fn mask_of(rows: &slint::ModelRc<ExtensionEntry>) -> u32 {
    let mut mask = 0u32;
    for (i, row) in rows.iter().enumerate() {
        if row.enabled && i < 32 {
            mask |= 1 << i;
        }
    }
    mask
}

fn archive_rows(settings: &StoredSettings) -> Vec<ExtensionEntry> {
    SUPPORTED_ARCHIVE_EXTS
        .iter()
        .enumerate()
        .map(|(i, ext)| ExtensionEntry {
            name: format!(".{ext}").into(),
            enabled: (settings.enabled_archive_exts & (1 << i)) != 0,
        })
        .collect()
}

fn image_rows(settings: &StoredSettings) -> Vec<ExtensionEntry> {
    SUPPORTED_IMAGE_EXTS
        .iter()
        .enumerate()
        .map(|(i, ext)| ExtensionEntry {
            name: (*ext).into(),
            enabled: (settings.enabled_image_exts & (1 << i)) != 0,
        })
        .collect()
}

fn push(window: &MainWindow, settings: &StoredSettings) {
    use arcthumb::settings::{CoverMode, SortOrder};
    window.set_sort_index(match settings.sort_order {
        SortOrder::Natural => 0,
        SortOrder::Alphabetical => 1,
    });
    window.set_cover_mode(match settings.cover_mode {
        CoverMode::Prefer => 0,
        CoverMode::Only => 1,
        CoverMode::Ignore => 2,
    });
    window.set_enable_preview(false);
    window.set_overlay_border(settings.overlay_border);
    window.set_overlay_label(settings.overlay_label);
    window.set_log_enabled(settings.log_enabled);
}

/// Flip one row of a toggle list and let Slint re-read it. The row index is
/// also the bit index, which is why both lists are built straight from the
/// core's `SUPPORTED_*_EXTS` constants. The model *is* the live state for
/// these two groups; `collect()` reads it back when Apply runs.
fn flip(model: &Rc<VecModel<ExtensionEntry>>, row: i32) {
    let Ok(index) = usize::try_from(row) else {
        return;
    };
    if index >= model.row_count() {
        return;
    }
    let Some(mut entry) = model.row_data(index) else {
        return;
    };
    entry.enabled = !entry.enabled;
    model.set_row_data(index, entry);
}

/// Copy the string table into the window's label properties.
fn apply_strings(window: &MainWindow, s: &Strings) {
    window.set_window_title(SharedString::from(s.window_title));
    window.set_menu_file(SharedString::from(s.menu_file));
    window.set_menu_file_exit(SharedString::from(s.menu_file_exit));
    window.set_menu_help(SharedString::from(s.menu_help));
    window.set_menu_help_about(SharedString::from(s.menu_help_about));
    window.set_group_extensions(SharedString::from(s.group_extensions));
    window.set_group_image_exts(SharedString::from(s.group_image_exts));
    window.set_group_sort(SharedString::from(s.group_sort));
    window.set_sort_natural_label(SharedString::from(s.sort_natural));
    window.set_sort_alpha_label(SharedString::from(s.sort_alphabetical));
    window.set_group_cover(SharedString::from(s.group_cover));
    window.set_cover_prefer_label(SharedString::from(s.cover_prefer));
    window.set_cover_only_label(SharedString::from(s.cover_only));
    window.set_cover_ignore_label(SharedString::from(s.cover_ignore));
    window.set_group_other(SharedString::from(s.group_other));
    window.set_enable_preview_label(SharedString::from(s.cb_enable_preview));
    window.set_overlay_border_label(SharedString::from(s.cb_overlay_border));
    window.set_overlay_label_label(SharedString::from(s.cb_overlay_label));
    window.set_log_enabled_label(SharedString::from(s.cb_log_enabled));
    window.set_btn_ok(SharedString::from(s.btn_ok));
    window.set_btn_cancel(SharedString::from(s.btn_cancel));
    window.set_btn_apply(SharedString::from(s.btn_apply));
    window.set_btn_regenerate(SharedString::from(s.btn_regenerate));
}

fn version_text() -> String {
    format!(
        "ArcThumb {} — {}",
        env!("CARGO_PKG_VERSION"),
        std::env::consts::OS
    )
}

/// Show a notice window. Non-blocking by design: the Windows build can block
/// on `MessageBoxW`, but nesting an event loop inside a Slint callback is not
/// portable, so confirmation arrives as a callback instead.
fn notice(s: &Strings, title: &str, text: &str, on_accept: Option<Rc<dyn Fn()>>) {
    let Ok(dialog) = NoticeDialog::new() else {
        return;
    };
    dialog.set_dialog_title(SharedString::from(title));
    dialog.set_message_text(SharedString::from(text));
    dialog.set_btn_ok(SharedString::from(s.btn_ok));
    dialog.set_btn_cancel(SharedString::from(s.btn_cancel));
    dialog.set_has_cancel(on_accept.is_some());

    let weak = dialog.as_weak();
    dialog.on_rejected(move || {
        if let Some(dialog) = weak.upgrade() {
            dialog.hide().ok();
        }
    });

    let weak = dialog.as_weak();
    if let Some(on_accept) = on_accept {
        dialog.on_accepted(move || {
            if let Some(dialog) = weak.upgrade() {
                dialog.hide().ok();
            }
            on_accept();
        });
    } else {
        dialog.on_accepted(move || {
            if let Some(dialog) = weak.upgrade() {
                dialog.hide().ok();
            }
        });
    }
    dialog.show().ok();
}
