# Development notes

The reader-facing install and configuration steps live in
[README.md](../README.md) / [README.zh-CN.md](../README.zh-CN.md). This file keeps the
material that only matters while you are building or debugging the extension.

## Prerequisites

- A stable Rust toolchain (2024 edition).
- Windows: the *Desktop development with C++* workload from Visual Studio Build Tools.
- Windows installer builds: [Inno Setup 6](https://jrsoftware.org/isinfo.php).
- macOS: `brew install cmake meson ninja` to compile the bundled AVIF decoder (dav1d).

## Cargo features

`default = ["wic", "config-gui"]` — both are Windows-only backends and resolve to nothing on
other targets, so no flag juggling is needed when building the core elsewhere.

- `wic` — AVIF and JXL decoding through the Windows Imaging Component instead of a bundled
  decoder. See [WIC_IMPLEMENTATION.md](./WIC_IMPLEMENTATION.md).
- `config-gui` — the `arcthumb-config` settings panel (Dioxus) and its toolkit.
  Its embedded font subsets are generated ahead of time by `tools/fonts/build.sh` and
  committed; `build.rs` refuses to compile a label the committed subset cannot draw. The thumbnail
  backends (Explorer's and Quick Look's) build without it; use `--no-default-features` to
  check or ship just the extension.

The macOS FFI shim under `macos/arcthumb-ffi` is deliberately **not** a workspace member: it
depends on this crate with `default-features = false`, and keeping it out means
`cargo build` / `cargo test` on Windows always compile exactly the same targets. Build it from
its own directory.

## Windows

```sh
cargo build --release                          # DLL + config GUI

target\release\arcthumb-config.exe --install   # register (HKLM if elevated, otherwise HKCU)
target\release\arcthumb-config.exe --uninstall # undo (cleans both hives best-effort)

iscc installer\arcthumb.iss                    # optional: build the installer
# output: target\installer\ArcThumb-Setup.exe
```

Release assets are the portable zip instead; the Inno Setup script remains for a
machine-wide installer. The installer writes no CLSID keys itself — it runs
`arcthumb-config.exe --install` after copying files and `--uninstall` before removing them,
which keeps the installer ignorant of the COM details and lets a developer re-register a
fresh build with one CLI command.

Exit codes from `arcthumb-config`: `0` success, `2` DLL not found (`--install`),
`3` CLSID registration failed, `4` extension binding failed, `5` GUI init failed.

## macOS

```sh
cargo test                                               # shared core
cargo test --manifest-path macos/arcthumb-ffi/Cargo.toml  # through the C ABI
./macos/build-appex.sh --release                         # .app + .appex in macos/build/
./macos/build-appex.sh --release --universal             # arm64 + x86_64
./macos/build-appex.sh --release --install               # build, then register with Finder
```

`macos/README.md` covers Pluginkit registration, the settings file, and troubleshooting.

## Reinstalling after a DLL change

`arcthumb.dll` runs inside `explorer.exe`, the `dllhost.exe` COM Surrogate, and (when the
preview pane is open) `prevhost.exe`. While any of those have it loaded, Windows refuses to
overwrite the file and the installer falls back to "queue for next reboot". The COM Surrogate
is the easiest one to forget — it can stay resident for several minutes after the last
thumbnail request.

The reliable way to refresh both binaries during local development:

```powershell
# 1. Build the new DLL + config GUI, then re-bundle the installer.
#    Skip step (b) and you'll be running an installer that contains
#    the previous build's exe.
cargo build --release                                        # (a)
iscc installer\arcthumb.iss                                  # (b)

# 2. Release every host process that holds the old DLL.
Stop-Process -Name explorer -Force -ErrorAction SilentlyContinue
Stop-Process -Name dllhost  -Force -ErrorAction SilentlyContinue
Stop-Process -Name prevhost -Force -ErrorAction SilentlyContinue

# 3. Run the freshly built installer. Same AppId, so it upgrades
#    the existing install in place. Tick "Launch ArcThumb
#    Configuration" on the Finish page.
.\target\installer\ArcThumb-Setup.exe

# 4. Bring Explorer back if the installer didn't already.
Start-Process explorer
```

If steps 1-4 still leave you with the old GUI or "file in use" errors, the install state is
wedged. To recover:

```powershell
# Kill the host processes again, then nuke the install dir by hand.
Stop-Process -Name explorer -Force -ErrorAction SilentlyContinue
Stop-Process -Name dllhost  -Force -ErrorAction SilentlyContinue
Stop-Process -Name prevhost -Force -ErrorAction SilentlyContinue
Remove-Item -Path "$env:LOCALAPPDATA\Programs\ArcThumb" -Recurse -Force -ErrorAction SilentlyContinue

# Belt-and-braces registry cleanup (the uninstaller normally handles
# this, but if it errored mid-run there can be leftovers).
Remove-Item -Path "HKCU:\Software\Classes\CLSID\{0F4F5659-D383-4945-A534-01E1EED1D23F}" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -Path "HKCU:\Software\Classes\CLSID\{8C7C1E5F-3D4A-4E2B-9F1A-7B5D6E8F9A0C}" -Recurse -Force -ErrorAction SilentlyContinue

Start-Process explorer
.\target\installer\ArcThumb-Setup.exe
```

If even that fails, **sign out and back in** — that guarantees every per-user `dllhost.exe`
(and any other stragglers) is torn down.

## The two COM classes

| Class | CLSID | Purpose |
|---|---|---|
| `ArcThumbProvider` | `{0F4F5659-D383-4945-A534-01E1EED1D23F}` | `IThumbnailProvider`, hosted in Explorer |
| `ArcThumbPreviewHandler` | `{8C7C1E5F-3D4A-4E2B-9F1A-7B5D6E8F9A0C}` | `IPreviewHandler`, hosted in `prevhost.exe` |

## Resetting the Windows thumbnail cache by hand

Explorer caches rendered bitmaps *and* the "this file has no thumbnail" answer, so a file
opened before ArcThumbX was installed keeps its old icon until the cache is rebuilt. The
**Regenerate thumbnails** button does the equivalent of:

```powershell
Stop-Process -Name explorer -Force
Stop-Process -Name dllhost  -Force -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\Microsoft\Windows\Explorer\thumbcache_*.db" -Force -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\Microsoft\Windows\Explorer\iconcache_*.db" -Force -ErrorAction SilentlyContinue
Start-Process explorer
```

On macOS the equivalent is `qlmanage -r cache`.

## Tests and coverage

```sh
cargo test
cargo test --no-default-features    # the lean build, as CI runs it
cargo llvm-cov --summary-only
```

## Testing the update / donation dialogs

The Windows config GUI checks for updates on startup and shows a donation prompt after a
version upgrade. `ARCTHUMB_FAKE_VERSION` overrides the compiled-in version at runtime, so both
dialogs can be exercised without rebuilding.

```powershell
# --- Update notification dialog ---
# Pretend the running build is v0.0.1 so the latest GitHub release
# looks like a new version.
$env:ARCTHUMB_FAKE_VERSION = "0.0.1"
Remove-ItemProperty -Path 'HKCU:\Software\ArcThumb' -Name 'LastUpdateCheck' -ErrorAction SilentlyContinue
Remove-ItemProperty -Path 'HKCU:\Software\ArcThumb' -Name 'SkippedVersion' -ErrorAction SilentlyContinue
target\release\arcthumb-config.exe

# --- Donation prompt dialog ---
# Set LastSeenVersion older than the current build so the app thinks
# the user just upgraded.
Remove-Item Env:\ARCTHUMB_FAKE_VERSION -ErrorAction SilentlyContinue
Set-ItemProperty -Path 'HKCU:\Software\ArcThumb' -Name 'LastSeenVersion' -Value '0.1.0' -Type String
Set-ItemProperty -Path 'HKCU:\Software\ArcThumb' -Name 'DonationDismissed' -Value 0 -Type DWord
Set-ItemProperty -Path 'HKCU:\Software\ArcThumb' -Name 'DonationSkipCount' -Value 0 -Type DWord
target\release\arcthumb-config.exe
```

To disable the update check entirely:

```powershell
Set-ItemProperty -Path 'HKCU:\Software\ArcThumb' -Name 'UpdateCheckEnabled' -Value 0 -Type DWord
```

## Regenerating the icon

If you change `assets/icon.png`, run `cargo run --example make_icon` to rebuild the
multi-resolution `assets/icon.ico` that gets embedded into the DLL and the config exe.

## Where the settings live

| Platform | Store |
|---|---|
| Windows | `HKCU\Software\ArcThumb` (`HKLM` for machine-wide registration) |
| macOS | `~/Library/Containers/com.citrussoda.ArcThumb.thumbnail/Data/Library/Application Support/ArcThumb/settings` — `key = value` lines, unknown keys ignored so an older extension still reads a newer file |

The macOS extension is sandboxed, so an unsandboxed helper cannot reach its `UserDefaults`
domain; the shared file is what lets the settings window and the extension agree. Key names
mirror the Windows registry values.

Diagnostic logging writes `arcthumb.log` into `std::env::temp_dir()`. Besides the settings
switch, the `ARCTHUMB_LOG` environment variable (any value) forces logging on, which is how
you trace a build that predates the switch — set it for the host process (`explorer.exe`, or
the Quick Look extension's environment) rather than for your shell. Debug builds always log.
