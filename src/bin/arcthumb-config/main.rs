//! ArcThumb Configuration — dual-mode binary.
//!
//! ## GUI mode (default)
//!
//! Running with no arguments launches a Slint-based settings window
//! where the user can enable/disable individual file extensions and
//! tweak the thumbnail selection behaviour (sort order, cover-name
//! preference).
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
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod apply;
mod cache;
mod cli;
mod dialogs;
mod dll_path;
mod extension_model;
mod locale;
mod message_box;
mod state;
mod ui;
mod update;
mod update_check;

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
            if let Err(e) = ui::run_gui() {
                let strings = locale::current();
                message_box::error(
                    strings.error_title,
                    &format!("{}\n\n{e}", strings.error_gui_init),
                );
                std::process::exit(5);
            }
        }
    }
}

/// Set the LogEnabled registry value and print feedback.
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
