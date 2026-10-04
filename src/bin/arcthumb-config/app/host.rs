//! The one seam between the panel and the platform.
//!
//! Each front end used to own its widget wiring (`ui.rs` for the registry,
//! `macos_ui.rs` for the settings file), which meant two copies of every label
//! setter, every toggle callback and every layout tweak, kept in step by hand.
//! The view is now one Dioxus tree, so what actually differs is *storage*: where
//! a setting is read from, what writing it costs, and which platform can honour
//! which row.
//!
//! Everything in this module is that difference and nothing else. Both builds
//! get the same function list; `#[cfg]` chooses the body.

use crate::app::panel::Panel;
use crate::locale::{Locale, Platform, Theme};

/// A reason for the CRT overlay to appear on its own, after startup.
///
/// Windows-only, because the updater is: there is no polling and no donation
/// prompt on macOS, so the type, the channel and the reader are all gated
/// rather than existing on a platform where nothing could ever produce one.
#[cfg(windows)]
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Prompt {
    /// The background update check found something newer.
    #[cfg(windows)]
    Update {
        latest: String,
        current: String,
        url: String,
    },
    /// ArcThumb just moved to `version` and has not thanked the user yet.
    #[cfg(windows)]
    Donation { version: String },
    /// The check ran and found nothing; only the indicator changes.
    #[cfg(windows)]
    CheckFinished,
}

/// The platform behind the panel. Deliberately a unit type: the state that
/// matters lives in the registry or the settings file, and re-reading it after
/// every Apply is what keeps the dialog telling the truth about what is stored.
pub struct Host;

#[cfg(windows)]
impl Host {
    pub fn platform() -> Platform {
        Platform::Windows
    }

    pub fn load() -> Panel {
        crate::ui::load_panel()
    }

    pub fn acknowledge_donation_shown() {
        crate::update::record_donation_shown();
    }

    /// Which hive the extension is actually registered in — the dialog edits
    /// that one, so the panel says it out loud.
    pub fn store_label() -> String {
        match crate::ui::scope() {
            arcthumb::registry::Scope::PerMachine => "HKLM".to_string(),
            arcthumb::registry::Scope::PerUser => "HKCU".to_string(),
        }
    }

    pub fn version() -> String {
        format!("{} — windows", env!("CARGO_PKG_VERSION"))
    }

    pub fn save(panel: &Panel) -> Result<(), String> {
        crate::ui::save_panel(panel)
    }

    pub fn regenerate() -> Result<(), String> {
        crate::ui::regenerate()
    }

    pub fn preview_available() -> bool {
        true
    }

    pub fn stored_locale() -> Option<Locale> {
        crate::locale::saved_locale()
    }

    pub fn stored_theme() -> Option<Theme> {
        crate::locale::saved_theme()
    }

    pub fn store_prefs(locale: Locale, theme: Theme) -> Result<(), String> {
        // Both writes are attempted even if the first fails: a half-stored
        // preference is recoverable, a silently skipped one is not.
        let language = crate::locale::save_locale(locale).err();
        let theme = crate::locale::save_theme(theme).err();
        match (language, theme) {
            (None, None) => Ok(()),
            (Some(e), None) | (None, Some(e)) => Err(e),
            (Some(a), Some(b)) => Err(format!("{a}; {b}")),
        }
    }

    /// Hand the panel's mailbox to the updater: the worker thread sends back into
    /// it, and the panel's own coroutine is the only reader.
    pub fn start_prompts(tx: futures_channel::mpsc::UnboundedSender<Prompt>) {
        crate::update_check::start_prompts(tx);
    }

    pub fn open_release(latest: &str, url: &str) {
        crate::update::open_url(url);
        // Opening the download page counts as deciding about this version.
        crate::update::skip_version(latest);
    }

    pub fn update_later(latest: &str, skip: bool) {
        if skip {
            crate::update::skip_version(latest);
        }
    }

    pub fn open_sponsor(dismiss: bool) {
        if dismiss {
            crate::update::dismiss_donation();
        }
        crate::update::open_url(crate::update::sponsor_url());
    }

    pub fn donation_later(dismiss: bool) {
        if dismiss {
            crate::update::dismiss_donation();
        } else {
            crate::update::record_donation_skip();
        }
    }
}

#[cfg(not(windows))]
impl Host {
    pub fn platform() -> Platform {
        Platform::MacOS
    }

    pub fn load() -> Panel {
        crate::macos_ui::load_panel()
    }

    /// The file name rather than a path: the panel strip is 38px tall and a
    /// container path would not fit without ellipsis.
    pub fn store_label() -> String {
        "settings".to_string()
    }

    pub fn version() -> String {
        format!("{} — {}", env!("CARGO_PKG_VERSION"), std::env::consts::OS)
    }

    pub fn save(panel: &Panel) -> Result<(), String> {
        crate::macos_ui::save_panel(panel)
    }

    pub fn regenerate() -> Result<(), String> {
        crate::settings_store::regenerate_thumbnails().map_err(|e| e.to_string())
    }

    /// Finder has no preview pane handler here, so the row is drawn inert.
    pub fn preview_available() -> bool {
        false
    }

    pub fn stored_locale() -> Option<Locale> {
        Locale::from_tag(crate::settings_store::load().language?.as_str())
    }

    pub fn stored_theme() -> Option<Theme> {
        Theme::from_tag(crate::settings_store::load().theme?.as_str())
    }

    pub fn store_prefs(locale: Locale, theme: Theme) -> Result<(), String> {
        crate::macos_ui::save_prefs(locale, theme)
    }
}
