//! ArcThumb — archive cover-thumbnail engine.
//!
//! The crate is split in two halves:
//!
//! * a **platform-independent core** (`archive`, `ebook`, `decode`,
//!   `settings`, `overlay`, `limits`, `log`, `thumbnail`) that reads
//!   archives, picks the cover image, decodes it (including AVIF and
//!   JPEG XL) and renders an RGBA bitmap; and
//! * a **platform backend** that plugs that bitmap into the host's
//!   thumbnail API. On Windows that is the COM `IThumbnailProvider`
//!   shell extension in `com`/`bitmap`/`stream`/`preview`/`wic`; on
//!   macOS it is the Quick Look extension in `macos/`, which reaches the
//!   core through the C ABI shim in `macos/arcthumb-ffi`.
//!
//! Everything Windows-specific is gated behind `cfg(windows)` so the core
//! can be built for other targets unchanged.

#![allow(non_snake_case)]

// =========================================================================
// Platform-independent core
// =========================================================================

pub mod archive;
pub mod decode;
pub mod ebook;
pub mod limits;
pub mod log;
pub mod overlay;
pub mod pixel;
pub mod settings;
pub mod thumbnail;

// =========================================================================
// Windows shell-extension backend
// =========================================================================

#[cfg(windows)]
mod bitmap;
#[cfg(windows)]
mod com;
#[cfg(windows)]
pub mod elevation;
#[cfg(windows)]
mod preview;
#[cfg(windows)]
pub mod registry;
#[cfg(windows)]
mod stream;
#[cfg(all(windows, feature = "wic"))]
mod wic;

#[cfg(windows)]
use std::panic::catch_unwind;

#[cfg(windows)]
use windows::Win32::Foundation::{E_FAIL, E_POINTER, S_FALSE, S_OK};
#[cfg(windows)]
use windows::Win32::System::Com::IClassFactory;
#[cfg(windows)]
use windows::core::{GUID, HRESULT, Interface};

#[cfg(windows)]
pub use com::CLSID_ARCTHUMB_PROVIDER;
#[cfg(windows)]
pub use preview::CLSID_ARCTHUMB_PREVIEW;

/// COM error: "no class factory for the requested CLSID".
#[cfg(windows)]
const CLASS_E_CLASSNOTAVAILABLE: HRESULT = HRESULT(0x80040111u32 as i32);

/// Catch any panic inside `f` and turn it into `E_FAIL`.
///
/// Rust panics propagating across `extern "system"` are undefined
/// behaviour — on Windows they'd crash Explorer. Every COM entry
/// point in this DLL funnels through this helper.
#[cfg(windows)]
fn guard<F: FnOnce() -> HRESULT + std::panic::UnwindSafe>(f: F) -> HRESULT {
    match catch_unwind(f) {
        Ok(hr) => hr,
        Err(_) => {
            crate::log::log("PANIC caught at DLL entry point");
            E_FAIL
        }
    }
}

/// Called by COM when a client asks this DLL for a class factory.
///
/// Our job:
/// 1. Check the requested CLSID is ours (we only host one class).
/// 2. Hand back an `IClassFactory` that knows how to create `ArcThumbProvider`s.
///
/// # Safety
///
/// This is the COM ABI surface, called by the OLE runtime. Callers
/// must guarantee:
/// - `rclsid`, `riid`, and `ppv` are either null or point to valid,
///   properly aligned objects of the right type for the duration of
///   the call. The function explicitly checks for null and returns
///   `E_POINTER` in that case, so unaligned/dangling pointers are
///   the only remaining UB risk.
/// - `*ppv` will be initialised by this function on success and
///   left untouched on failure; callers must not rely on its value
///   if the returned `HRESULT` is a failure code.
/// - The returned interface follows COM reference counting rules:
///   on success, the caller owns one reference and must `Release`
///   it. The OLE runtime handles this for normal `CoCreateInstance`
///   paths.
#[unsafe(no_mangle)]
#[cfg(windows)]
pub unsafe extern "system" fn DllGetClassObject(
    rclsid: *const GUID,
    riid: *const GUID,
    ppv: *mut *mut core::ffi::c_void,
) -> HRESULT {
    guard(|| {
        if rclsid.is_null() || riid.is_null() || ppv.is_null() {
            return E_POINTER;
        }
        unsafe {
            if *rclsid == CLSID_ARCTHUMB_PROVIDER {
                let factory: IClassFactory = com::ArcThumbClassFactory.into();
                factory.query(&*riid, ppv)
            } else if *rclsid == CLSID_ARCTHUMB_PREVIEW {
                let factory: IClassFactory = preview::ArcThumbPreviewClassFactory.into();
                factory.query(&*riid, ppv)
            } else {
                CLASS_E_CLASSNOTAVAILABLE
            }
        }
    })
}

/// Called by COM to ask whether this DLL can be unloaded.
///
/// Returning `S_FALSE` tells the host "please keep me loaded". Tracking
/// live object counts correctly is finicky; for a shell extension this
/// is a reasonable default — Explorer unloads us when it shuts down
/// or when the DLL idle timer fires.
#[unsafe(no_mangle)]
#[cfg(windows)]
pub extern "system" fn DllCanUnloadNow() -> HRESULT {
    guard(|| S_FALSE)
}

/// Called by `regsvr32 arcthumb.dll`.
#[unsafe(no_mangle)]
#[cfg(windows)]
pub extern "system" fn DllRegisterServer() -> HRESULT {
    guard(|| match registry::register() {
        Ok(()) => S_OK,
        Err(_) => E_FAIL,
    })
}

/// Called by `regsvr32 /u arcthumb.dll`.
#[unsafe(no_mangle)]
#[cfg(windows)]
pub extern "system" fn DllUnregisterServer() -> HRESULT {
    guard(|| match registry::unregister() {
        Ok(()) => S_OK,
        Err(_) => E_FAIL,
    })
}
