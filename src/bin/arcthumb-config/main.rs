//! ArcThumb Configuration — dual-mode binary.
//!
//! ## GUI mode (default)
//!
//! Running with no arguments launches the settings panel (Dioxus, over the
//! system web view) where the user can enable individual file extensions,
//! pick the thumbnail selection behaviour (sort order, cover-name preference),
//! and choose the panel's own language and theme.
//!
//! ## CLI mode
//!
//! ```text
//! arcthumb-config.exe --install
//!     Write the full shell-extension registration. Hive is picked
//!     automatically by elevation: HKLM when the process is elevated
//!     (per-machine install), HKCU otherwise (per-user install).
//!     Called by the Inno Setup installer as a post-install step.
//!
//! arcthumb-config.exe --uninstall
//!     Remove every ShellEx binding and the CLSID key from BOTH
//!     hives (best effort) so a per-user → per-machine switch or
//!     vice versa doesn't leave stale entries behind.
//!     Called by the uninstaller as a pre-uninstall step.
//!
//! arcthumb-config.exe --log-on
//!     Enable diagnostic logging by writing LogEnabled=1 to
//!     HKCU\Software\ArcThumb. Restart Explorer to take effect.
//!
//! arcthumb-config.exe --log-off
//!     Disable diagnostic logging by writing LogEnabled=0.
//! ```
//!
//! Exit codes:
//! - `0` success
//! - `2` DLL not found (for --install)
//! - `3` CLSID registration failed
//! - `4` extension binding failed
//! - `5` GUI init failed (very rare)

// Hide the console on release builds. Debug builds keep the console
// so `cargo run` output is visible.
#![cfg_attr(all(not(debug_assertions), windows), windows_subsystem = "windows")]

// Every module below is either the Windows settings panel's plumbing (registry
// keys, shell-DLL registration) or the panel itself. The GUI half is gated on
// `config-gui` so the thumbnail backends — Explorer's `arcthumb.dll` and the
// macOS Quick Look shim — keep building without Dioxus in the graph at all.
#[cfg(feature = "config-gui")]
mod app;
#[cfg(all(windows, feature = "config-gui"))]
mod apply;
#[cfg(all(windows, feature = "config-gui"))]
mod cache;
#[cfg(windows)]
mod cli;
#[cfg(windows)]
mod dll_path;
// Language and theme resolution stays unbuilt-for-none of that: the settings
// file format validates the same two keys, and `--lang` exists so a Finder
// launch with no `LANG` can still be steered.
mod locale;
#[cfg(windows)]
mod message_box;
#[cfg(all(windows, feature = "config-gui"))]
mod state;
#[cfg(all(windows, feature = "config-gui"))]
mod ui;
#[cfg(all(windows, feature = "config-gui"))]
mod update;
#[cfg(all(windows, feature = "config-gui"))]
mod update_check;

#[cfg(all(not(windows), feature = "config-gui"))]
mod macos_ui;
#[cfg(not(windows))]
mod settings_store;

/// Arguments the macOS front end understands. `--lang` is honoured on both
/// platforms before any window is built.
#[cfg(not(windows))]
fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    match args.first().map(String::as_str) {
        Some("--lang") => {
            set_language_override(args.get(1).map(String::as_str));
            gui().unwrap_or_else(|e| {
                eprintln!("error: {e}");
                std::process::exit(5);
            });
        }
        Some("--log-on" | "--log-off") => {
            let on = args[0] == "--log-on";
            let mut s = settings_store::load();
            s.log_enabled = on;
            match settings_store::save(&s) {
                Ok(()) => println!(
                    "Diagnostic logging {}. The extension re-reads its settings on \
                     the next request; the log lands in its sandbox container.",
                    if on { "enabled" } else { "disabled" }
                ),
                Err(e) => {
                    eprintln!("error: {e}");
                    std::process::exit(1);
                }
            }
        }
        Some("--get") => {
            // Prints what the extension will actually apply, after the same
            // clamping and fallbacks it applies when reading the file — so a
            // hand-edited settings file can be checked without opening Finder.
            let s = settings_store::load().to_core();
            println!("sort_order              = {:?}", s.sort_order);
            println!("cover_mode              = {:?}", s.cover_mode);
            println!(
                "enabled_image_exts      = {:#014b}",
                s.enabled_image_exts_mask
            );
            println!(
                "enabled_archive_exts    = {:#014b}",
                s.enabled_archive_exts_mask
            );
            println!("overlay_border          = {}", s.overlay_border);
            println!("overlay_label           = {}", s.overlay_label);
            println!("log_enabled             = {}", s.log_enabled);
        }
        Some("--regenerate") => {
            if let Err(e) = settings_store::regenerate_thumbnails() {
                eprintln!("error: {e}");
                std::process::exit(1);
            }
            println!("Thumbnail cache cleared.");
        }
        Some("--help" | "-h") => println!(
            "usage: arcthumb-config [--lang en|ja|zh] [--log-on|--log-off]\n\
               \t     [--get] [--regenerate]\n\
            \n\
            With no arguments, opens the settings window."
        ),
        None => {
            gui().unwrap_or_else(|e| {
                eprintln!("error: {e}");
                std::process::exit(5);
            });
        }
        Some(other) => {
            eprintln!("unknown argument: {other} (try --help)");
            std::process::exit(2);
        }
    }
}

#[cfg(windows)]
fn main() {
    let args: Vec<String> = std::env::args().collect();
    match args.get(1).map(|s| s.as_str()) {
        Some("--install") => {
            attach_console();
            let code = cli::run_install(&cli::RealCliOps);
            match code {
                cli::EXIT_OK => println!("ArcThumb installed successfully."),
                cli::EXIT_DLL_NOT_FOUND => eprintln!("Error: arcthumb.dll not found."),
                cli::EXIT_CLSID_FAILED => eprintln!("Error: CLSID registration failed."),
                cli::EXIT_EXTENSION_FAILED => eprintln!("Error: extension binding failed."),
                _ => eprintln!("Error: unknown failure (exit code {code})."),
            }
            std::process::exit(code);
        }
        Some("--uninstall") => {
            attach_console();
            let code = cli::run_uninstall(&cli::RealCliOps);
            println!("ArcThumb uninstalled.");
            std::process::exit(code);
        }
        Some("--log-on") => {
            attach_console();
            set_log_enabled(true);
        }
        Some("--log-off") => {
            attach_console();
            set_log_enabled(false);
        }
        _ => {
            // Surface the failure with a native MessageBox before
            // exiting. Release builds run as `windows_subsystem =
            // "windows"`, so without this the user sees nothing —
            // not even a console line — and reports the binary as
            // broken. Reported in microsoft/winget-pkgs#364519.
            if let Err(e) = gui() {
                report_gui_failure(&e);
                std::process::exit(5);
            }
        }
    }
}

/// `--lang` is only meaningful when there is a panel to speak to.
#[cfg(all(not(windows), feature = "config-gui"))]
fn set_language_override(lang: Option<&str>) {
    locale::set_language_override(lang);
}

#[cfg(all(not(windows), not(feature = "config-gui")))]
fn set_language_override(_lang: Option<&str>) {}

/// Report a panel that could not start. A release build runs as a windows
/// subsystem binary, so without this the user sees nothing at all and files the
/// binary as broken (microsoft/winget-pkgs#364519). The localized wording needs
/// the string tables, which only the GUI build carries.
#[cfg(all(windows, feature = "config-gui"))]
fn report_gui_failure(detail: &str) {
    let strings = locale::Strings::resolve(
        locale::preferred_locale(locale::Locale::from_tag("en")),
        locale::Platform::Windows,
    );
    message_box::error(
        strings.error_title,
        &format!("{}\n\n{detail}", strings.error_gui_init),
    );
}

#[cfg(all(windows, not(feature = "config-gui")))]
fn report_gui_failure(detail: &str) {
    message_box::error(
        "ArcThumb",
        &format!("This build has no settings panel (built without config-gui).\n\n{detail}"),
    );
}

/// Open the settings panel. The two shapes below exist so a build with
/// `--no-default-features` still links: that build has no Dioxus in its graph,
/// and `cargo test --no-default-features` in CI compiles this binary.
#[cfg(feature = "config-gui")]
fn gui() -> Result<(), Box<dyn std::error::Error>> {
    app::run()
}

#[cfg(not(feature = "config-gui"))]
fn gui() -> Result<(), Box<dyn std::error::Error>> {
    Err(
        "this build of arcthumb-config has no settings panel (built without the \
config-gui feature); the command-line options still work"
            .into(),
    )
}

/// Set the LogEnabled registry value and print feedback.
#[cfg(windows)]
fn set_log_enabled(enabled: bool) {
    use winreg::RegKey;
    use winreg::enums::*;
    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let (key, _) = match hkcu.create_subkey("Software\\ArcThumb") {
        Ok(r) => r,
        Err(e) => {
            eprintln!("Error: failed to open registry key: {e}");
            std::process::exit(1);
        }
    };
    let val: u32 = if enabled { 1 } else { 0 };
    if let Err(e) = key.set_value("LogEnabled", &val) {
        eprintln!("Error: failed to write LogEnabled: {e}");
        std::process::exit(1);
    }
    if enabled {
        let log_path = std::env::temp_dir().join("arcthumb.log");
        println!("Logging enabled. Restart Explorer to apply.");
        println!("Log file: {}", log_path.display());
    } else {
        println!("Logging disabled. Restart Explorer to apply.");
    }
}

/// Attach to the parent process's console so `println!` / `eprintln!`
/// are visible when running from PowerShell or cmd.exe. No-op when
/// the process already has a console (debug builds) or when there is
/// no parent console to attach to (double-click launch).
#[cfg(windows)]
fn attach_console() {
    #[cfg(not(debug_assertions))]
    unsafe {
        // AttachConsole(ATTACH_PARENT_PROCESS) — raw FFI to avoid
        // pulling Win32_System_Console into the DLL's windows features.
        unsafe extern "system" {
            fn AttachConsole(dw_process_id: u32) -> i32;
        }
        let _ = AttachConsole(u32::MAX);
    }
}
