//! Build script: compile the Slint UI for `arcthumb-config.exe` and
//! embed a Windows resource file containing the app manifest + icon.
//!
//! The manifest declares Per-Monitor DPI v2 and Common Controls v6
//! so `arcthumb-config.exe` scales correctly on mixed-DPI setups and
//! picks up the modern visual style.
//!
//! Cargo links the compiled `.res` into every output artifact, so
//! `arcthumb.dll` also carries the manifest. This is harmless —
//! the shell extension DLL ignores its own manifest.
//!
//! Both steps are Windows-only, and so are the crates they need:
//! `slint`, `embed-resource` and the `windows` bindings hang off
//! `[target.'cfg(windows)'.dependencies]`, which is what lets the same
//! package build the cross-platform core for macOS. The gate reads
//! `CARGO_CFG_WINDOWS` rather than `cfg!(windows)` because a build
//! script is compiled for the *host*, while the dependency table (and
//! the code it feeds) is chosen by the *target*.

fn target_is_windows() -> bool {
    std::env::var_os("CARGO_CFG_WINDOWS").is_some()
}

/// Is the `arcthumb-config` GUI being built? Its dependencies (`slint`) are
/// optional, so the codegen below must not run when they are switched off.
fn config_gui_enabled() -> bool {
    std::env::var_os("CARGO_FEATURE_CONFIG_GUI").is_some()
}

fn main() {
    // Only rerun when the resources change.
    println!("cargo:rerun-if-changed=resources/arcthumb-config.rc");
    println!("cargo:rerun-if-changed=resources/arcthumb-config.manifest");
    println!("cargo:rerun-if-changed=assets/icon.ico");
    println!("cargo:rerun-if-changed=ui/main.slint");

    // The Windows manifest is only for a Windows target.
    if target_is_windows() {
        embed_resource::compile("resources/arcthumb-config.rc", embed_resource::NONE)
            .manifest_required()
            .expect("failed to embed Windows resource (manifest + icon)");
    }

    // The manifest is mandatory (it declares DPI awareness and Common
    // Controls v6), so treat a missing RC compiler or a failed compile as
    // a hard build error rather than silently shipping a binary without
    // it.
    if config_gui_enabled() {
        // Compile the Slint UI for arcthumb-config. Generated Rust code
        // lands in OUT_DIR and is pulled in by `slint::include_modules!()`.
        slint_build::compile("ui/main.slint").expect("failed to compile Slint UI");
    }
}
