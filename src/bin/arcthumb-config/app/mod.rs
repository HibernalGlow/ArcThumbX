//! The ArcThumb config panel: one Dioxus tree over a platform seam.
//!
//! ## Why a webview
//!
//! The panel is Dioxus because the Cassette-Futurism language this fork asks for
//! is a *surface* language — hairlines, scanlines, phosphor glow, engraved
//! captions, hard right angles — and that is CSS's home ground, not an
//! immediate-mode toolkit's. The cost is stated once and not hidden: the desktop
//! renderer embeds the system web view, so Windows needs the WebView2 Runtime and
//! macOS needs WKWebView. A software-rendered toolkit would paint on a machine
//! with no GPU and no extra runtime; this one needs the runtime present. If it is
//! missing, `run()` fails before any window exists,
//! `main` reports it with the localized `error_gui_init`, and `--install`,
//! `--uninstall`, `--log-on` and `--get` in `cli.rs` — which never needed the
//! GUI — still work.
//!
//! ## What the panel is
//!
//! One instrument face, not a form. The function strip carries the identity, the
//! language keycaps, the theme rocker and the update indicator. The body is five
//! screwed-down panels. The LCD strip under them says what the last operation
//! actually did to the store. The CRT overlay is the only screen allowed to
//! interrupt. There is deliberately no native menu bar: everything it would have
//! held is already a key on the panel, and a second way to press the same thing
//! is not a feature.

pub mod fonts;
pub mod host;
pub mod panel;
pub mod view;

use std::any::Any;
use std::error::Error;

use dioxus::prelude::*;
use dioxus_desktop::{Config, LogicalSize, WindowBuilder, icon_from_memory, use_window};
#[cfg(windows)]
use futures_util::StreamExt;

use crate::app::panel::{Panel, Status, Theme};
use crate::locale::{self, Locale, Strings};

use self::host::Host;
#[cfg(windows)]
use self::host::Prompt;
use self::view::{Msg, Overlay, View};

/// Everything the first frame needs, decided before the window exists so there
/// is no theme flash and no half-translated paint.
#[derive(Clone, PartialEq)]
struct Startup {
    strings: Strings,
    locale: Locale,
    theme: Theme,
    panel: Panel,
    store: String,
    version: String,
    preview_available: bool,
}

/// The embedded faces plus the panel stylesheet, as served HTML head content.
/// They arrive here rather than through the tree so the first painted frame
/// already has the right type and the right chassis colour.
fn head() -> String {
    format!(
        "<meta charset=\"utf-8\"><style>{}</style><style>{}</style><style>{}</style>",
        fonts::face_css(),
        include_str!("style.css"),
        include_str!("screen.css"),
    )
}

/// Open the settings panel. Blocks in the webview event loop until the last
/// window closes, which is how OK, Cancel, the strip's Exit key and the close box
/// all leave.
pub fn run() -> Result<(), Box<dyn Error>> {
    let platform = Host::platform();
    let chosen = locale::preferred_locale(Host::stored_locale());
    let theme = Host::stored_theme().unwrap_or(Theme::DEFAULT);
    let strings = Strings::resolve(chosen, platform);
    let startup = Startup {
        strings,
        locale: chosen,
        theme,
        panel: Host::load(),
        store: Host::store_label(),
        version: Host::version(),
        preview_available: Host::preview_available(),
    };

    let config = Config::new()
        .with_window(
            WindowBuilder::new()
                .with_title(strings.window_title)
                .with_inner_size(LogicalSize::new(620.0, 700.0))
                .with_min_inner_size(LogicalSize::new(480.0, 420.0))
                // A debug build would otherwise pin the dialog above everything
                // else on the user's desktop.
                .with_always_on_top(false)
                .with_resizable(true),
        )
        .with_custom_head(head())
        // `Config` installs a default menu bar unless told otherwise; the strip
        // already holds every action it would have contained.
        .with_menu(None)
        .with_background_color(match theme {
            Theme::Dark => (0x12, 0x13, 0x16, 0xFF),
            Theme::Light => (0xEC, 0xE7, 0xDC, 0xFF),
        });

    // A window icon the loader cannot produce is not a reason to refuse to open
    // the dialog, so this is the one fallible step that is allowed to fail soft.
    let config = match icon_from_memory(include_bytes!("../../../../assets/icon.png")) {
        Ok(icon) => config.with_icon(icon),
        Err(_) => config,
    };

    let contexts: Vec<Box<dyn Fn() -> Box<dyn Any> + Send + Sync>> = {
        let startup = startup.clone();
        vec![Box::new(move || Box::new(startup.clone()) as Box<dyn Any>)]
    };
    dioxus_desktop::launch::launch(App, contexts, vec![Box::new(config)]);
}

#[component]
fn App() -> Element {
    let startup = use_context::<Startup>();
    let desktop = use_window();

    let mut panel = use_signal(|| startup.panel.clone());
    // What the store holds, re-read after every Apply. `dirty` is the distance
    // between the two, which is the only honest reason to press Apply.
    let stored = use_signal(|| startup.panel.clone());
    let mut locale = use_signal(|| startup.locale);
    let mut theme = use_signal(|| startup.theme);
    let mut strings = use_signal(|| startup.strings);
    let mut status = use_signal(|| Status::Ready);
    let mut overlay = use_signal(|| Overlay::None);
    // On macOS nothing can ever report back: the mailbox, its writer and these
    // two signals are all Windows-only, so the read path stays shared while the
    // write path does not exist.
    #[cfg(windows)]
    let mut checking = use_signal(|| false);
    #[cfg(not(windows))]
    let checking = use_signal(|| false);
    #[cfg(windows)]
    let mut update_found = use_signal(|| false);
    #[cfg(not(windows))]
    let update_found = use_signal(|| false);

    // Background news arrives through one mailbox. Writing a signal from the
    // updater's thread would touch the runtime off-thread, so that thread sends
    // a [`Prompt`] and this coroutine — which runs on the runtime — is the only
    // reader. `use_coroutine` builds its channel during the first render, so the
    // sender the host receives below is already live.
    #[cfg(windows)]
    let prompts = use_coroutine(move |mut rx| {
        async move {
            while let Some(prompt) = rx.next().await {
                match prompt {
                    // Every producer of these lives behind `#[cfg(windows)]`,
                    // so on macOS the enum has no variants and the match below
                    // is empty rather than unreachable.
                    #[cfg(windows)]
                    Prompt::Update {
                        latest,
                        current,
                        url,
                    } => {
                        checking.set(false);
                        update_found.set(true);
                        overlay.set(Overlay::Update {
                            latest,
                            current,
                            url,
                            skip: false,
                        });
                    }
                    #[cfg(windows)]
                    Prompt::Donation { version } => {
                        overlay.set(Overlay::Donation {
                            version,
                            dismissed: false,
                        });
                        // Recorded when it is drawn, not when it is queued: a panel
                        // closed before the user saw it must not count as shown.
                        Host::acknowledge_donation_shown();
                    }
                    #[cfg(windows)]
                    Prompt::CheckFinished => checking.set(false),
                }
            }
        }
    });

    // Started after the render, once: the panel must paint before a network
    // thread can talk to it.
    #[cfg(windows)]
    use_effect(move || {
        // Lit only while something is genuinely in flight, so the indicator is a
        // reading and not decoration.
        checking.set(true);
        Host::start_prompts(prompts.tx());
    });

    // The title bar is the one label the operating system draws, so it follows
    // the language like everything else.
    use_effect({
        // `DesktopContext` is an `Rc`, so the title effect takes its own clone
        // rather than moving the handle the key handler also needs.
        let desktop = desktop.clone();
        move || {
            desktop.window.set_title(strings.read().window_title);
        }
    });

    let onmsg = move |msg: Msg| match msg {
        Msg::Toggle(grid, index) => {
            panel.write().toggle(grid, index);
            status.set(Status::Ready);
        }
        Msg::SetSort(index) => panel.write().sort = panel::sort_from_index(index),
        Msg::SetCover(index) => panel.write().cover = panel::cover_from_index(index),
        Msg::SetPreview(on) => panel.write().preview = on,
        Msg::SetBorder(on) => panel.write().border = on,
        Msg::SetLabel(on) => panel.write().label = on,
        Msg::SetLog(on) => panel.write().log = on,
        Msg::Apply => apply(panel, stored, status),
        Msg::Ok => {
            if panel.read().is_dirty_vs(&stored.read()) {
                apply(panel, stored, status);
            }
            desktop.close();
        }
        Msg::Cancel => desktop.close(),
        Msg::AskRegenerate => overlay.set(Overlay::ConfirmRegenerate),
        Msg::Regenerate => {
            overlay.set(Overlay::None);
            status.set(match Host::regenerate() {
                Ok(()) => Status::Saved,
                Err(detail) => Status::Error(detail),
            });
        }
        Msg::About => overlay.set(Overlay::About),
        Msg::Exit => desktop.close(),
        Msg::SetLocale(want) => {
            if want != *locale.read() {
                locale.set(want);
                strings.set(Strings::resolve(want, Host::platform()));
                store_prefs(locale, theme, status);
            }
        }
        Msg::ToggleTheme => {
            let next = theme.read().toggled();
            theme.set(next);
            store_prefs(locale, theme, status);
        }
        Msg::CloseOverlay => overlay.set(Overlay::None),
        // Updater-facing: see `Msg`'s gating of these variants.
        #[cfg(windows)]
        Msg::OpenRelease(skip) => {
            let Overlay::Update { latest, url, .. } = overlay.read().clone() else {
                return;
            };
            Host::open_release(&latest, &url);
            if skip {
                Host::update_later(&latest, true);
            }
            overlay.set(Overlay::None);
        }
        // Updater-facing: see `Msg`'s gating of these variants.
        #[cfg(windows)]
        Msg::LaterOnUpdate(skip) => {
            let Overlay::Update { latest, .. } = overlay.read().clone() else {
                return;
            };
            Host::update_later(&latest, skip);
            overlay.set(Overlay::None);
        }
        // Updater-facing: see `Msg`'s gating of these variants.
        #[cfg(windows)]
        Msg::OpenSponsor(dismiss) => {
            Host::open_sponsor(dismiss);
            overlay.set(Overlay::None);
        }
        // Updater-facing: see `Msg`'s gating of these variants.
        #[cfg(windows)]
        Msg::LaterOnDonation(dismiss) => {
            Host::donation_later(dismiss);
            overlay.set(Overlay::None);
        }
    };

    let dirty = panel.read().is_dirty_vs(&stored.read());
    rsx! {
        Chassis {
            strings: *strings.read(),
            panel: panel.read().clone(),
            locale: *locale.read(),
            theme: *theme.read(),
            status: status.read().clone(),
            overlay: overlay.read().clone(),
            preview_available: startup.preview_available,
            store: startup.store.clone(),
            version: startup.version.clone(),
            checking: *checking.read(),
            update_found: *update_found.read(),
            dirty,
            onmsg,
        }
    }
}

/// Write the panel to the store, then read it back. The status line reports what
/// the store said afterwards, not what the panel hoped — which is also what
/// makes a partially-applied change (a locked cache, a refused hive) visible
/// instead of silently optimistic.
fn apply(mut panel: Signal<Panel>, mut stored: Signal<Panel>, mut status: Signal<Status>) {
    let candidate = panel.read().clone();
    match Host::save(&candidate) {
        Ok(()) => {
            let reread = Host::load();
            stored.set(reread.clone());
            panel.set(reread);
            status.set(Status::Saved);
        }
        Err(detail) => status.set(Status::Error(detail)),
    }
}

/// Language and theme are preferences, not settings: they change nothing the
/// extension reads. Persisting them is the host's job, and the panel only trusts
/// the value back once the write succeeded.
fn store_prefs(locale: Signal<Locale>, theme: Signal<Theme>, mut status: Signal<Status>) {
    if let Err(detail) = Host::store_prefs(*locale.read(), *theme.read()) {
        status.set(Status::Error(detail));
    }
}

/// Props bridge: the component takes owned data so Dioxus can diff it, and the
/// view layer borrows that data for one frame.
#[component]
fn Chassis(
    strings: Strings,
    panel: Panel,
    locale: Locale,
    theme: Theme,
    status: Status,
    overlay: Overlay,
    preview_available: bool,
    store: String,
    version: String,
    checking: bool,
    update_found: bool,
    dirty: bool,
    onmsg: EventHandler<Msg>,
) -> Element {
    let view = View {
        strings: &strings,
        panel: &panel,
        locale,
        theme,
        status: &status,
        overlay: &overlay,
        preview_available,
        store: &store,
        version: &version,
        checking,
        update_found,
        dirty,
    };
    view::render(&view, onmsg)
}
