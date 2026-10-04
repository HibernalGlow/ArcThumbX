//! The native Win32 error box.
//!
//! This is the one dialog the panel cannot draw: it exists for the case where
//! the panel never started — no WebView2 Runtime, a webview that refused to come
//! up — so there is no surface left to paint a CRT screen on. Release builds run
//! as a `windows` subsystem binary, so without it a failed launch would produce
//! no visible symptom at all (microsoft/winget-pkgs#364519).
//!
//! Everything else that used to live here — the information box and the
//! confirm-with-warning box — moved into the panel as the CRT overlay, which is
//! the same screen the settings are on and needs no second event loop.

use windows::Win32::UI::WindowsAndMessaging::{MB_ICONERROR, MB_OK, MessageBoxW};
use windows::core::PCWSTR;

fn to_wide(s: &str) -> Vec<u16> {
    s.encode_utf16().chain(std::iter::once(0)).collect()
}

pub fn error(title: &str, content: &str) {
    let title_w = to_wide(title);
    let content_w = to_wide(content);
    unsafe {
        MessageBoxW(
            None,
            PCWSTR(content_w.as_ptr()),
            PCWSTR(title_w.as_ptr()),
            MB_OK | MB_ICONERROR,
        );
    }
}
