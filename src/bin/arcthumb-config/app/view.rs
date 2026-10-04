//! The panel's view layer: chassis, keycap grids, rocker switches, LCD status
//! strip and the CRT overlay. Pure rendering — it reads [`View`] and emits
//! [`Msg`], and holds no state of its own.
//!
//! Design law, from the catfu contract, applied to a settings dialog rather
//! than a landing page: no radius on structure, no drop shadows, 1px hairlines
//! do the zoning, cobalt is the only action colour, amber/green only ever mean
//! something, and nothing eases. The hero device, the 16-step sequencer and the
//! marquee from the reference page are deliberately absent — an instrument
//! panel earns the aesthetic by being dense and legible, not by decorating.
//!
//! Two rules that shape real choices here:
//!   * Screens (the LCD strip, the CRT overlay) never invert with the theme.
//!   * Every control must say what it is: a row the platform cannot honour is
//!     shown, labelled and inert, never hidden.

use dioxus::prelude::*;

use crate::app::panel::{Grid, Panel, Status, Theme};
use crate::locale::{Locale, Strings};

/// Silk-screen engraving: fixed, uppercase, never translated. A real panel's
/// legends are moulded into the plastic, and a Chinese user reading `EXT` is
/// reading the same instrument as an English one.
pub const SILK_EXTENSIONS: &str = "EXT";
pub const SILK_IMAGE_FORMATS: &str = "IMG";
pub const SILK_SORT: &str = "SORT";
pub const SILK_COVER: &str = "COVER";
pub const SILK_OTHER: &str = "MISC";
pub const SILK_LANGUAGE: &str = "LANG";
pub const SILK_MODE: &str = "MODE";
pub const SILK_MODIFIED: &str = "MODIFIED";
pub const SILK_UPDATE: &str = "UPDATE";

/// Which single toggle row a message is about.
#[derive(Debug, Clone, PartialEq)]
pub enum Msg {
    Toggle(Grid, usize),
    SetSort(usize),
    SetCover(usize),
    SetPreview(bool),
    SetBorder(bool),
    SetLabel(bool),
    SetLog(bool),
    Apply,
    Ok,
    Cancel,
    /// Open the CRT confirmation for "clear the thumbnail cache".
    AskRegenerate,
    /// The user accepted that confirmation.
    Regenerate,
    About,
    Exit,
    SetLocale(Locale),
    ToggleTheme,
    CloseOverlay,
    /// Updater-facing messages; see [`Overlay::Update`] for why they are gated.
    #[cfg(windows)]
    OpenRelease(bool),
    #[cfg(windows)]
    LaterOnUpdate(bool),
    #[cfg(windows)]
    OpenSponsor(bool),
    #[cfg(windows)]
    LaterOnDonation(bool),
}

/// Everything the view reads. Borrowed, because it is a projection of state the
/// `App` component owns.
pub struct View<'a> {
    pub strings: &'a Strings,
    pub panel: &'a Panel,
    pub locale: Locale,
    pub theme: Theme,
    pub status: &'a Status,
    pub overlay: &'a Overlay,
    /// Whether the preview-pane row can be toggled. The preview pane is an
    /// Explorer feature with no Finder counterpart, so the macOS build reports
    /// false: the row stays visible, grey, and honest about what it is.
    pub preview_available: bool,
    /// "HKCU" / "HKLM" on Windows, the settings file name on macOS.
    pub store: &'a str,
    pub version: &'a str,
    /// The background update check is in flight.
    pub checking: bool,
    /// An update was found (or the check finished empty).
    pub update_found: bool,
    /// Unsaved edits relative to what the store holds.
    pub dirty: bool,
}

/// What the CRT overlay is currently showing. One overlay at a time: the panel
/// is a single-screen instrument, and stacking dialogs would hide state.
#[derive(Debug, Clone, PartialEq)]
pub enum Overlay {
    None,
    /// "Clear the thumbnail cache" — destructive enough to confirm.
    ConfirmRegenerate,
    About,
    /// Updater-facing pages. The updater is Windows-only, so on macOS these
    /// variants do not exist at all rather than existing unproduced.
    #[cfg(windows)]
    Update {
        latest: String,
        current: String,
        url: String,
        skip: bool,
    },
    #[cfg(windows)]
    Donation {
        version: String,
        dismissed: bool,
    },
}

impl Overlay {
    fn is_open(&self) -> bool {
        self != &Overlay::None
    }
}

pub fn render(view: &View<'_>, out: EventHandler<Msg>) -> Element {
    let theme_class = match view.theme {
        Theme::Dark => "chassis",
        Theme::Light => "chassis t-light",
    };
    rsx! {
        div { class: theme_class,
            {strip(view, out)}
            {body(view, out)}
            {lcd(view)}
            {footer(view, out)}
            if view.overlay.is_open() {
                {overlay(view, out)}
            }
        }
    }
}

// =============================================================================
// Function strip
// =============================================================================

fn strip(view: &View<'_>, out: EventHandler<Msg>) -> Element {
    let version = view.version.to_string();
    let locale = view.locale;
    let strings = *view.strings;
    rsx! {
        div { class: "strip",
            div { class: "seg",
                div { class: "mark", title: "ArcThumb" }
                div { class: "nameplate",
                    "ARCTHUMB"
                    span { class: "sub", " {version}" }
                }
            }
            div { class: "seg hide-narrow",
                span { class: "lbl dim", "STORE" }
                span { class: "num", "{view.store}" }
            }
            // A live indicator of the background check, rather than a second
            // way to press "check for updates": blinking means in flight, lit
            // means something came back.
            div { class: "seg hide-narrow",
                span { class: "lbl dim", "{SILK_UPDATE}" }
                div {
                    class: if view.checking { "led rec" } else if view.update_found { "led on" } else { "led" },
                }
            }
            div { class: "seg grow" }
            div { class: "seg",
                span { class: "lbl dim", "{SILK_LANGUAGE}" }
                div { class: "keygroup",
                    for want in Locale::ALL {
                        button {
                            class: if want == locale { "btn tiny on" } else { "btn tiny" },
                            title: want.endonym(),
                            aria_label: want.endonym(),
                            onclick: move |_| out.call(Msg::SetLocale(want)),
                            "{want.endonym()}"
                        }
                    }
                }
            }
            div { class: "seg",
                span { class: "lbl dim", "{SILK_MODE}" }
                {theme_rocker(view, out)}
            }
            div { class: "seg",
                button {
                    class: "btn",
                    onclick: move |_| out.call(Msg::About),
                    "{strings.btn_about}"
                }
                button {
                    class: "btn",
                    onclick: move |_| out.call(Msg::Exit),
                    "{strings.btn_exit}"
                }
            }
        }
    }
}

/// The theme control is a panel switch with the two positions engraved, not a
/// floating sun/moon icon: it has to look like part of the chassis.
fn theme_rocker(view: &View<'_>, out: EventHandler<Msg>) -> Element {
    let dark = view.theme == Theme::Dark;
    let strings = *view.strings;
    rsx! {
        div { class: "tog",
            button {
                class: "tog-track",
                role: "switch",
                aria_label: if dark { strings.theme_dark } else { strings.theme_light },
                aria_checked: if dark { "true" } else { "false" },
                title: if dark { strings.theme_dark } else { strings.theme_light },
                onclick: move |_| out.call(Msg::ToggleTheme),
                span { class: "tog-glyph moon", "☾" }
                span { class: "tog-glyph sun", "☀" }
                div { class: "tog-knob" }
            }
        }
    }
}

// =============================================================================
// Body
// =============================================================================

fn body(view: &View<'_>, out: EventHandler<Msg>) -> Element {
    let strings = *view.strings;
    rsx! {
        div { class: "body",
            div { class: "pnl reveal reveal-1", {keycap_grid(
                SILK_EXTENSIONS,
                strings.group_extensions,
                Some(strings.hint_extensions),
                Grid::Archive,
                &view.panel.archive,
                out,
            )} }
            div { class: "pnl reveal reveal-2", {keycap_grid(
                SILK_IMAGE_FORMATS,
                strings.group_image_exts,
                Some(strings.hint_image_exts),
                Grid::Image,
                &view.panel.image,
                out,
            )} }
            div { class: "pnl reveal reveal-3", {sort_panel(view, out)} }
            div { class: "pnl reveal reveal-4", {cover_panel(view, out)} }
            div { class: "pnl reveal reveal-5", {other_panel(view, out)} }
        }
    }
}

/// A section: ordinal, silk-screen code, title, then the controls. The
/// hairline under the header is the only thing separating header from body.
fn section_head(code: &str, ordinal: &str, title: &str) -> Element {
    rsx! {
        div { class: "pnl-head",
            span { class: "idx", "{ordinal}" }
            span { class: "lbl", "{code}" }
            span { class: "pnl-title", "{title}" }
            span { class: "spacer" }
        }
    }
}

fn keycap_grid(
    code: &'static str,
    title: &str,
    hint: Option<&str>,
    grid: Grid,
    rows: &[crate::app::panel::Row],
    out: EventHandler<Msg>,
) -> Element {
    let ordinal = if grid == Grid::Archive { "01" } else { "02" };
    rsx! {
        {section_head(code, ordinal, title)}
        div { class: "pnl-body",
            div { class: "caps",
                for (index , row) in rows.iter().enumerate() {
                    button {
                        class: if row.on { "cap on" } else { "cap" },
                        aria_pressed: if row.on { "true" } else { "false" },
                        title: "{row.name}",
                        onclick: move |_| out.call(Msg::Toggle(grid, index)),
                        span { class: "cap-led" }
                        span { class: "cap-name", "{row.name}" }
                    }
                }
            }
            if let Some(hint) = hint {
                div { class: "hint", "{hint}" }
            }
        }
    }
}

/// Sort order and cover policy read as position switches: a `<select>` would
/// open a menu painted by the toolkit, which is the one thing this panel cannot
/// absorb.
fn labelled_options(strings: &Strings, which: &str) -> Vec<String> {
    let options: &[&str] = match which {
        "sort" => &[strings.sort_natural, strings.sort_alphabetical],
        _ => &[
            strings.cover_prefer,
            strings.cover_only,
            strings.cover_ignore,
        ],
    };
    options.iter().map(|label| (*label).to_string()).collect()
}

fn segmented(
    options: Vec<String>,
    selected: usize,
    out: EventHandler<Msg>,
    pick: impl Fn(usize) -> Msg + Copy + 'static,
) -> Element {
    rsx! {
        div { class: "segbar",
            // Owned rows, consumed by the loop: an event handler must be
            // `'static`, so nothing inside it may borrow the option list this
            // frame just built from the string table.
            for (index , label) in options.into_iter().enumerate() {
                button {
                    class: if index == selected { "seg-btn on" } else { "seg-btn" },
                    aria_pressed: if index == selected { "true" } else { "false" },
                    title: "{label}",
                    onclick: move |_| out.call(pick(index)),
                    span { class: "cap-led" }
                    "{label}"
                }
            }
        }
    }
}

fn sort_panel(view: &View<'_>, out: EventHandler<Msg>) -> Element {
    let strings = *view.strings;
    let selected = crate::app::panel::sort_index(view.panel.sort);
    let options = labelled_options(&strings, "sort");
    rsx! {
        {section_head(SILK_SORT, "03", strings.group_sort)}
        div { class: "pnl-body",
            div { class: "seg-row", {segmented(options, selected, out, Msg::SetSort)} }
        }
    }
}

fn cover_panel(view: &View<'_>, out: EventHandler<Msg>) -> Element {
    let strings = *view.strings;
    let selected = crate::app::panel::cover_index(view.panel.cover);
    let options = labelled_options(&strings, "cover");
    rsx! {
        {section_head(SILK_COVER, "04", strings.group_cover)}
        div { class: "pnl-body",
            div { class: "seg-row", {segmented(options, selected, out, Msg::SetCover)} }
        }
    }
}

fn other_panel(view: &View<'_>, out: EventHandler<Msg>) -> Element {
    let strings = *view.strings;
    let panel = view.panel.clone();
    let preview = panel.preview;
    let border = panel.border;
    let label = panel.label;
    let log = panel.log;
    let available = view.preview_available;
    rsx! {
        {section_head(SILK_OTHER, "05", strings.group_other)}
        div { class: "pnl-body",
            {rocker(
                strings.cb_enable_preview,
                preview,
                available,
                (!available).then_some(strings.tag_windows_only),
                Msg::SetPreview(!preview), out,
            )}
            {rocker(
                strings.cb_overlay_border,
                border,
                true,
                None,
                Msg::SetBorder(!border), out,
            )}
            {rocker(
                strings.cb_overlay_label,
                label,
                true,
                None,
                Msg::SetLabel(!label), out,
            )}
            {rocker(
                strings.cb_log_enabled,
                log,
                true,
                None,
                Msg::SetLog(!log), out,
            )}
        }
    }
}

/// One hardware rocker row. When the platform cannot honour the switch the row
/// keeps its label, goes inert, and carries the reason as a chip — a control
/// that silently does nothing is worse than one that admits it cannot act.
fn rocker(
    label: &str,
    checked: bool,
    enabled: bool,
    tag: Option<&str>,
    toggle: Msg,
    out: EventHandler<Msg>,
) -> Element {
    rsx! {
        div { class: if enabled { "sw-row" } else { "sw-row inert" },
            div { class: "sw-text",
                span { class: "sw-label", "{label}" }
                if let Some(tag) = tag {
                    span { class: "chip", "{tag}" }
                }
            }
            button {
                class: "sw",
                role: "switch",
                aria_label: "{label}",
                aria_checked: if checked { "true" } else { "false" },
                disabled: !enabled,
                onclick: move |_| out.call(toggle.clone()),
                div { class: "knob" }
            }
        }
    }
}

// =============================================================================
// LCD status strip
// =============================================================================

/// The readout that makes the panel an instrument rather than a form: it says
/// what the last operation did to the store, in the store's own words. The
/// latin prefix is engraving; the message after it is localized.
fn lcd(view: &View<'_>) -> Element {
    let strings = *view.strings;
    let silk = view.status.silk();
    let (message, error) = match view.status {
        Status::Ready => (strings.lcd_ready.to_string(), false),
        Status::Saved => (strings.lcd_saved.to_string(), false),
        Status::Error(detail) => (detail.clone(), true),
    };
    let dirty = view.dirty;
    rsx! {
        div { class: if error { "lcd err" } else { "lcd" },
            span { class: "silk", "{silk}" }
            span { class: "msg", "{message}" }
            if dirty {
                span { class: "chip", "{SILK_MODIFIED}" }
            }
            div { class: "cursor" }
        }
    }
}

// =============================================================================
// Footer keys
// =============================================================================

fn footer(view: &View<'_>, out: EventHandler<Msg>) -> Element {
    let strings = *view.strings;
    rsx! {
        div { class: "footer",
            button {
                class: "btn",
                title: strings.regen_confirm,
                onclick: move |_| out.call(Msg::AskRegenerate),
                "{strings.btn_regenerate}"
            }
            div { class: "spacer" }
            button {
                class: "btn primary",
                onclick: move |_| out.call(Msg::Ok),
                "{strings.btn_ok}"
            }
            button {
                class: "btn",
                onclick: move |_| out.call(Msg::Cancel),
                "{strings.btn_cancel}"
            }
            button {
                class: "btn",
                disabled: !view.dirty,
                onclick: move |_| out.call(Msg::Apply),
                "{strings.btn_apply}"
            }
        }
    }
}

// =============================================================================
// CRT overlay
// =============================================================================

/// The About / Update / Donation / confirmation screens. Inside a single-screen
/// instrument these are the same display showing a different page rather than
/// separate OS windows, which also means the panel's state stays visible
/// behind them.
fn overlay(view: &View<'_>, out: EventHandler<Msg>) -> Element {
    let strings = *view.strings;
    match view.overlay {
        Overlay::None => rsx! {},
        Overlay::ConfirmRegenerate => {
            let body = strings.regen_confirm;
            rsx! {
                {crt_shell(
                    strings.error_title,
                    rsx! { {crt_paragraph(body)} },
                    rsx! {
                        button {
                            class: "btn",
                            onclick: move |_| out.call(Msg::CloseOverlay),
                            "{strings.btn_cancel}"
                        }
                        button {
                            class: "btn primary",
                            onclick: move |_| out.call(Msg::Regenerate),
                            "{strings.btn_ok}"
                        }
                    },
                    out,
                )}
            }
        }
        Overlay::About => {
            let body = strings.about_body;
            let title = strings.about_title;
            let version = view.version.to_string();
            rsx! {
                {crt_shell(
                    title,
                    rsx! {
                        div { class: "kv",
                            span { class: "k", "VER" }
                            span { class: "rule" }
                            span { class: "v", "{version}" }
                        }
                        {crt_paragraph(body)}
                    },
                    rsx! {
                        button {
                            class: "btn primary",
                            onclick: move |_| out.call(Msg::CloseOverlay),
                            "{strings.btn_close}"
                        }
                    },
                    out,
                )}
            }
        }
        #[cfg(windows)]
        Overlay::Update {
            latest,
            current,
            skip,
            ..
        } => {
            let message = strings
                .update_available
                .replacen("{}", latest.as_str(), 1)
                .replacen("{}", current.as_str(), 1);
            let skip_label = strings.update_skip_checkbox;
            let skip_now = *skip;
            rsx! {
                {crt_shell(
                    strings.update_title,
                    rsx! {
                        {crt_paragraph(&message)}
                        button {
                            class: "tick",
                            role: "checkbox",
                            aria_checked: if skip_now { "true" } else { "false" },
                            aria_label: "{skip_label}",
                            onclick: move |_| out.call(Msg::LaterOnUpdate(!skip_now)),
                            span { class: "box" }
                            "{skip_label}"
                        }
                    },
                    rsx! {
                        button {
                            class: "btn",
                            onclick: move |_| out.call(Msg::LaterOnUpdate(skip_now)),
                            "{strings.update_btn_later}"
                        }
                        button {
                            class: "btn primary",
                            onclick: move |_| out.call(Msg::OpenRelease(skip_now)),
                            "{strings.update_btn_open}"
                        }
                    },
                    out,
                )}
            }
        }
        #[cfg(windows)]
        Overlay::Donation { version, dismissed } => {
            let message = strings.donation_prompt.replacen("{}", version, 1);
            let tick_label = strings.donation_dont_show_checkbox;
            let dismissed_now = *dismissed;
            rsx! {
                {crt_shell(
                    strings.donation_title,
                    rsx! {
                        {crt_paragraph(&message)}
                        button {
                            class: "tick",
                            role: "checkbox",
                            aria_checked: if dismissed_now { "true" } else { "false" },
                            aria_label: "{tick_label}",
                            onclick: move |_| out.call(Msg::LaterOnDonation(!dismissed_now)),
                            span { class: "box" }
                            "{tick_label}"
                        }
                    },
                    rsx! {
                        button {
                            class: "btn",
                            onclick: move |_| out.call(Msg::LaterOnDonation(dismissed_now)),
                            "{strings.donation_btn_later}"
                        }
                        button {
                            class: "btn primary",
                            onclick: move |_| out.call(Msg::OpenSponsor(dismissed_now)),
                            "{strings.donation_btn_sponsor}"
                        }
                    },
                    out,
                )}
            }
        }
    }
}

fn crt_paragraph(text: &str) -> Element {
    rsx! { div { class: "crt-text", "{text}" } }
}

fn crt_shell(title: &str, body: Element, actions: Element, out: EventHandler<Msg>) -> Element {
    rsx! {
        div { class: "scrim", onclick: move |_| out.call(Msg::CloseOverlay),
            // Clicking the screen itself must not dismiss: the pointer has to
            // cross a keycap to get out, which is the point of a confirmation.
            div { class: "crt", onclick: move |evt: MouseEvent| evt.stop_propagation(),
                div { class: "crt-head",
                    span { class: "crt-title", "{title}" }
                    span { class: "spacer" }
                    span { class: "led rec" }
                }
                div { class: "crt-body", {body} }
                div { class: "crt-foot",
                    span { class: "spacer" }
                    {actions}
                }
            }
        }
    }
}
