# ArcThumb on macOS — Quick Look thumbnail extension

Finder shows the cover image inside `.cbz`, `.cbr`, `.cb7`, `.cbt`,
`.zip`, `.rar`, `.7z`, `.tar`, `.epub`, `.fb2`, `.mobi` and `.azw3`,
using the **same Rust code** Explorer uses on Windows. Nothing about
archives, cover selection or image decoding is re-implemented in Swift:

```text
Finder → QLThumbnailProvider (Swift, ~90 lines)
       → arc_thumbnail_generate (C ABI, macos/arcthumb-ffi)
       → arcthumb::thumbnail::render (shared core)
       → premultiplied RGBA8 → CGImage → QLThumbnailReply
```

No temporary image files are written and pixels are copied once (Rust →
`CGDataProvider`, which takes ownership of the Rust allocation).

## Getting a release build

Release assets are one zip per architecture —
`ArcThumb-<version>-macOS-arm64.zip` (Apple Silicon) and, when an Intel
runner is available, `…-macOS-x86_64.zip`. There is deliberately no
"universal" binary: the bundled dav1d is built by meson, and the crate that
vendors it (`libdav1d-sys`) has no macOS cross-compile configuration, so a
cross-built slice silently ends up holding host-architecture objects. Each
artifact is therefore compiled on a machine of its own architecture.

The builds are **ad-hoc signed**, so Gatekeeper will refuse them on
another Mac until you tell it otherwise — right-click the app and choose
**Open** the first time, or clear the quarantine flag:

```sh
xattr -dr com.apple.quarantine ~/Applications/ArcThumb.app
```

A notarized Developer ID build needs an Apple signing identity; the release
workflow signs with `vars.MACOS_SIGN_IDENTITY` when that repository variable
is set and falls back to ad-hoc otherwise.

Intel Macs have not been verified on real hardware — the x86_64 CI leg is
marked best-effort, so treat that asset as untested until someone runs it.

## Prerequisites

| Need | Why |
|---|---|
| Xcode or CLT (`swiftc`, `clang`, `codesign`) | the extension binary |
| Rust stable | the core |
| `cmake`, `meson`, `ninja` (`brew install cmake meson ninja`) | the vendored libavif / dav1d C sources |
| `nasm` (x86_64 builds only) | dav1d assembly |

`unrar`'s C++ sources compile with `cc` alone. Nothing links against a
system image library, so a built extension runs on a Mac with no
Homebrew at all.

## Build

```sh
./macos/build-appex.sh                 # debug, host arch, ad-hoc signed
./macos/build-appex.sh --release                 # host arch
./macos/build-appex.sh --release --arch=x86_64   # only meaningful on an Intel Mac
./macos/build-appex.sh --release --install     # + register with the system
./macos/build-appex.sh --sign="Developer ID: …" # distribution signing
```

Output is `macos/build/ArcThumb.app`, containing
`Contents/PlugIns/ArcThumbThumbnail.appex`. The host app has no UI yet —
it exists because an app extension must be embedded in an app bundle for
Launch Services to see it. Sign order matters: extension first, then
bundle, never `--deep`.

## Install / uninstall

```sh
./macos/build-appex.sh --release --install
# or by hand, after copying ArcThumb.app to /Applications or ~/Applications:
/System/Library/Frameworks/CoreServices.framework/Versions/A/Frameworks/\
LaunchServices.framework/Versions/A/Support/lsregister -f ~/Applications/ArcThumb.app
pluginkit -a ~/Applications/ArcThumb.app/Contents/PlugIns/ArcThumbThumbnail.appex
pluginkit -e use -i com.citrussoda.ArcThumb.thumbnail   # enable it
qlmanage -r && qlmanage -r cache                        # re-scan + drop the cache
```

`lsregister` alone is **not** enough: without `pluginkit -a` the extension
is invisible to Quick Look, and without `-e use` it is registered but
disabled. The GUI equivalent is System Settings → Privacy & Security →
Extensions → Thumbnailing Extensions.

Uninstall:

```sh
pluginkit -r ~/Applications/ArcThumb.app/Contents/PlugIns/ArcThumbThumbnail.appex
rm -rf ~/Applications/ArcThumb.app ~/Library/Containers/com.citrussoda.ArcThumb.thumbnail
qlmanage -r cache
```

## Settings

Open **ArcThumb.app** — the same Slint window as on Windows (`arcthumb-config`
is the app's executable). Everything there writes one file:

```text
~/Library/Containers/com.citrussoda.ArcThumb.thumbnail/Data/
    Library/Application Support/ArcThumb/settings
```

| Row | Effect on macOS |
|---|---|
| Enabled extensions | Bitmask over the container types the extension will thumbnail. On Windows the same row writes/removed `ShellEx` bindings, i.e. Explorer stops asking; a Quick Look extension claims its types statically in `Info.plist`, so the filter lives here instead. |
| Image formats | Same `EnabledImageExts` semantics as Windows: which files inside an archive are eligible as the cover. |
| Sort order / Cover image / Border / Label / Logging | Identical meaning, stored in the file above. |
| Enable preview pane | **Disabled**: the preview pane is the Windows `IPreviewHandler`; Finder has no counterpart in this build yet. |
| Regenerate thumbnails | `qlmanage -r cache`, the counterpart of clearing the Explorer icon cache. |

A settings file rather than `UserDefaults`, because the extension is
sandboxed: an unsandboxed helper cannot write into its preference domain
(`CFPreferences` with the bundle id lands in `~/Library/Preferences/`, which
the sandbox never reads), while the container path above is readable by the
extension and writable by the user's own apps. Unknown keys are ignored, so a
newer file still parses in an older extension.

The same binary has a headless mode, useful in scripts and for checking what
the extension will actually apply:

```sh
ArcThumb.app/Contents/MacOS/ArcThumb --get          # effective settings, clamped
ArcThumb.app/Contents/MacOS/ArcThumb --log-on       # or --log-off
ArcThumb.app/Contents/MacOS/ArcThumb --regenerate
ArcThumb.app/Contents/MacOS/ArcThumb --lang zh      # en | ja | zh
```

## Testing without opening Finder

```sh
./macos/make-fixtures.sh ~/Desktop/arcthumb-fixtures   # one cover per format
xcrun swiftc -O -framework AppKit -framework QuickLookThumbnailing \
  macos/tools/qlthumbprobe.swift -o /tmp/qlthumbprobe
/tmp/qlthumbprobe /tmp/out.png ~/Desktop/arcthumb-fixtures/*.cbz
```

`qlthumbprobe` calls `QLThumbnailGenerator` — the same client API Finder
uses — so a `HIT` line proves the extension was discovered, matched by
UTI, launched in its sandbox, and returned real pixels (the mean colour
and distinct-colour count make a blank or placeholder bitmap obvious).

Quick Look caches by file, so a repeat probe of the same archive can be
answered from the cache without the extension running at all. To force a
real generation, give the file a fresh identity:

```sh
cp ~/Desktop/arcthumb-fixtures/test.cbz /tmp/take2.cbz   # new path + mtime
printf 'x%s' "$RANDOM" | zip -z /tmp/take2.cbz           # and new bytes
```

With `LogEnabled` set, the extension's log also echoes the settings that
actually crossed the FFI (`sort=Natural cover=Prefer mask=0b11111111111
border=false label=false`), which is what distinguishes "the setting was
ignored" from "the setting was never read".

The core itself is tested headlessly:

```sh
cargo test                                        # shared core, 233 tests
cargo test --manifest-path macos/arcthumb-ffi/Cargo.toml   # through the C ABI
```

The FFI suite builds archives on the fly and asserts cover selection,
aspect ratio, the premultiplied invariant, AVIF/JXL/PNG/JPEG decoding,
corrupt and empty archives, disabled formats, and 8-way concurrency.

## Diagnostics

Logging is off in release builds; turn it on with
`ArcThumb --log-on` (or `ARCTHUMB_LOG=1`). The extension is sandboxed, so its
log is inside its container:

```sh
tail -f ~/Library/Containers/com.citrussoda.ArcThumb.thumbnail/Data/tmp/arcthumb.log
```

`NSLog` output from the Swift layer goes to the unified log:

```sh
log stream --predicate 'eventMessage CONTAINS "ArcThumb"' --style compact
```

If Finder shows nothing: `pluginkit -m -i com.citrussoda.ArcThumb.thumbnail`
must print the id with a leading `+` (registered *and* enabled), then
`ArcThumb --regenerate` and read the container log above for the reason the
core declined. Note that Quick Look caches per file, so re-testing the same
archive can be answered from cache without the extension running at all —
copy it to a new name to force a real generation.
