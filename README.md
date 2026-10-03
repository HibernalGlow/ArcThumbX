<p align="center">
  <img src="./assets/readme/hero.svg" width="100%"
       alt="ArcThumbX — archive and ebook cover thumbnails rendered inside Windows Explorer and macOS Finder. The board shows six cover tiles tagged CBZ, EPUB, CBR, FB2, 7Z and AZW3.">
</p>

<p align="center">
  <a href="#license"><img src="https://img.shields.io/badge/license-MIT%20OR%20Apache--2.0-blue.svg" alt="Dual-licensed MIT or Apache-2.0"></a>
  <a href="https://github.com/HibernalGlow/ArcThumbX/releases"><img src="https://img.shields.io/github/v/release/HibernalGlow/ArcThumbX?label=release&color=green" alt="Latest release"></a>
  <a href="https://github.com/HibernalGlow/ArcThumbX/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/HibernalGlow/ArcThumbX/ci.yml?branch=develop&label=CI" alt="CI status"></a>
  <img src="https://img.shields.io/badge/platform-Windows%2010%2F11%20%C2%B7%20macOS%2011%2B-lightgrey.svg" alt="Windows 10/11 and macOS 11 and later">
  <a href="./README.zh-CN.md"><img src="https://img.shields.io/badge/%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-3A7CA5.svg" alt="简体中文说明"></a>
</p>

ArcThumbX puts archive and ebook **covers where the file list is** — as thumbnails in
Windows Explorer and in macOS Finder. Both front ends are thin shells over one Rust core,
so a `.cbz` and its `.epub` sibling get the same cover, the same sort rules and the same
decoders on either platform. It reads comic archives (ZIP, CBZ, RAR, CBR, 7Z, CB7, TAR,
CBT) and ebooks (EPUB, FB2, MOBI, AZW, AZW3).

This repository is a fork of [citrussoda-com/ArcThumb](https://github.com/citrussoda-com/ArcThumb);
see [About this fork](#about-this-fork) for what changed.

## See it

Explorer, thumbnails on, preview pane open (`Alt+P`):

<img src="./assets/explorer.png" width="100%" alt="Windows Explorer showing cover thumbnails for an EPUB, an AZW3, two 7Z archives, a MOBI, three ZIP archives, an FB2, a RAR and a CBT, with the selected EPUB's cover filling the preview pane on the right">

The same folder with the optional identification overlay enabled — a format-coloured border
plus a corner chip, so an archive cover never passes for a loose image:

<img src="./assets/explorer_with_overlay.png" width="100%" alt="The same Explorer folder with overlays on: each thumbnail carries an amber border and chip for ZIP-family archives and a blue one for ebooks, and the preview pane repeats the tag">

## What you get

- **A cover, not a generic icon.** The first image inside the archive becomes the file's
  thumbnail; `cover.*`, `folder.*`, `thumb.*`, `thumbnail.*` and `front.*` win when present.
- **Ebook covers found by the format's own rules.** EPUB through its OPF manifest, FB2
  through `<coverpage>`, MOBI/AZW/AZW3 through the EXTH 201 `CoverOffset` record — so the
  real cover is picked instead of whatever image happens to sort first.
- **Preview pane on Windows.** An `IPreviewHandler` shows the same cover at full pane height
  and rescales when you drag the splitter.
- **An optional overlay** (off by default): amber frame and chip for archive families, blue
  for ebooks, with the chip reading `CBZ`, `EPUB`, …
- **One settings window on both platforms** — per-extension and per-image-format toggles,
  sort order, cover preference, overlay switches, diagnostic logging, and a
  Regenerate-thumbnails button. English, 日本語 and 简体中文.
- **Nothing on disk.** Pixels are handed to the shell in memory; no scratch images are
  written while thumbnails are generated.
- **Panic-safe on Windows.** Every COM entry point is wrapped in `catch_unwind`, so a
  decoder panic cannot take Explorer, `dllhost.exe` or `prevhost.exe` down with it.

## Quick start

### Windows

Releases ship a portable zip — no installer, no admin rights.

```powershell
# 1. Extract ArcThumb-<version>-portable-x64.zip somewhere permanent,
#    e.g. %LOCALAPPDATA%\Programs\ArcThumb
# 2. Register the shell extension (writes to HKCU):
arcthumb-config.exe --install
```

New thumbnails appear immediately; press `Alt+P` for the preview pane. Run the same command
**as administrator** to register machine-wide under `HKLM` instead — needed when Explorer
runs at high integrity, e.g. Windows Sandbox. To remove: `arcthumb-config.exe --uninstall`
(cleans both hives), then delete the folder.

### macOS

```sh
# 1. Unzip ArcThumb-<version>-macOS-arm64.zip into /Applications or ~/Applications
# 2. Release builds are ad-hoc signed, so clear quarantine once:
xattr -dr com.apple.quarantine ~/Applications/ArcThumb.app
# 3. Let Finder use the extension:
pluginkit -e use -i com.citrussoda.ArcThumb.thumbnail
qlmanage -r cache
```

You can toggle the same setting under **System Settings → Privacy & Security → Extensions →
Thumbnailing Extensions**. Intel Macs get a separate `…-macOS-x86_64.zip`; the two slices are
built on their own architecture rather than lipo'd, because the bundled AVIF decoder has no
macOS cross-compile configuration. The x86_64 leg has not been verified on real Intel
hardware yet — [macos/README.md](./macos/README.md) says so plainly.

Prefer building it yourself? `./macos/build-appex.sh --release --install` compiles and
registers in one step — details in [macos/README.md](./macos/README.md).

## How it is put together

<img src="./assets/readme/architecture.svg" width="100%"
     alt="System map: the shared Rust core (src/archive, src/ebook, src/decode, src/overlay, joined in src/thumbnail.rs) feeds two thin backends — a Windows COM backend that hands pixels over as an HBITMAP, and a macOS Quick Look backend that goes through a six-function C ABI and a CGImage.">

Everything platform-neutral lives in the core: container readers, ebook cover locators, image
decoding (including AVIF and JXL), resizing and overlay drawing. `src/thumbnail.rs` joins them
and returns one RGBA bitmap. Each backend adds only what its file manager demands:

| Platform | Backend | Pixel hand-off |
|---|---|---|
| Windows | two COM classes in one DLL: `IThumbnailProvider` + `IPreviewHandler`, bound per extension in the registry | `RgbaImage` → premultiplied BGRA DIB section (`HBITMAP`) |
| macOS | `QLThumbnailProvider` app extension matched by UTI (`macos/`), plus the settings window | `RgbaImage` → premultiplied RGBA8 → `CGDataProvider` → `CGImage`, over a six-function C ABI (`macos/arcthumb-ffi`) |

Neither backend re-implements archive or image logic. On Windows both CLSIDs register under
`HKCU` by default, so installing and removing ArcThumbX never touches the machine-wide
registry; elevated installs switch to `HKLM`. Deeper dives:
[docs/WIC_IMPLEMENTATION.md](./docs/WIC_IMPLEMENTATION.md) and
[docs/MACOS_IMPLEMENTATION.md](./docs/MACOS_IMPLEMENTATION.md).

## Supported formats

<img src="./assets/readme/formats.svg" width="100%"
     alt="Format board: four archive containers (ZIP/CBZ, RAR/CBR, 7Z/CB7, TAR/CBT) tagged amber, three ebook formats (EPUB, FB2, MOBI/AZW/AZW3) tagged blue with the metadata each uses to locate its cover, a dashed cell for HEIC, SVG and DjVu which are not supported yet, and two codec cards contrasting Windows' WIC path with the statically linked libavif, dav1d and jxl-oxide used on macOS.">

The core opens 13 container extensions:

| Extension | Family | How the cover is chosen |
|---|---|---|
| `.zip` `.cbz` `.rar` `.cbr` `.7z` `.cb7` `.tar` `.cbt` | archives | first eligible image by sort order, cover-named files preferred |
| `.epub` | EPUB 2 / 3 | OPF manifest cover reference |
| `.fb2` | FictionBook 2 | `<coverpage>` reference, inline base64 image |
| `.mobi` `.azw` `.azw3` | Kindle | EXTH 201 `CoverOffset` record |

Windows registers ShellEx bindings for 12 of them (bare `.tar` is not bound, `.cbt` is); the
macOS extension declares all 13 through UTIs.

Images inside those containers: `.jpg` `.jpeg` `.png` `.gif` `.bmp` `.tiff` `.tif` `.ico`
`.webp` decode in pure Rust everywhere. `.avif` and `.jxl` depend on the platform:

- **Windows** — the default `wic` feature decodes them through the Windows Imaging Component,
  i.e. the system codecs. Windows 11 24H2+ generally has AVIF built in; Windows 10 needs the
  [AV1 Image Extensions](https://apps.microsoft.com/detail/9n26s50ln705) and/or a JPEG XL
  extension from the Microsoft Store. Build with `--no-default-features` to ship without them.
- **macOS and other targets** — libavif + dav1d and `jxl-oxide` are statically linked into the
  extension, so no system codec and no user setup is involved.

## Settings

Open **ArcThumb Configuration** from the Start menu. On macOS the identical window is
**ArcThumb.app** — the same Slint UI, reading and writing the extension's settings file.

<img src="./assets/screenshot.png" width="420" alt="The ArcThumb Configuration window: extension checkboxes, image format checkboxes, a sort-order dropdown, cover preference, preview pane, the two overlay switches and a Regenerate thumbnails button">

- **Enabled extensions** — turn the thumbnail provider on or off per file extension.
- **Image formats used for thumbnails** — which formats are eligible as a thumbnail source
  inside an archive. Ebooks are unaffected: they use their own metadata.
- **Sort order** — what counts as "first". Natural sort treats `page2.jpg` as smaller than
  `page10.jpg` and is the default; alphabetical is the opposite.
- **Cover image** — *Use cover if present, else first page* (default), *Cover only* (no
  thumbnail at all when the archive has no cover-named image, so an unrelated ZIP keeps the
  plain archive icon), or *Always use first page*.
- **Enable preview pane** — one switch that registers or unregisters the `IPreviewHandler`
  for every supported extension. Shown disabled on macOS, where Finder has no equivalent yet.
- **Coloured border / format label** — the two overlay switches. The label uses the file's
  extension when readable and otherwise the detected format, and is dropped on icons too
  small to render it; the border stays.
- **Enable diagnostic logging** — writes a trace to `arcthumb.log` in the system temp
  directory (`%TEMP%` on Windows, `$TMPDIR` on macOS). Also toggleable with
  `arcthumb-config --log-on` / `--log-off`.
- **Language** — English, 日本語, 简体中文. Windows picks from `GetUserDefaultLocaleName` on
  first run and keeps the choice in `HKCU\Software\ArcThumb\Language`; on macOS pass
  `--lang en|ja|zh` to `ArcThumb.app`.
- **Regenerate thumbnails** — clears Explorer's thumbnail and icon caches on Windows, and runs
  `qlmanage -r cache` on macOS.

Overlay changes need that button: the shell caches the rendered bitmap, so a previously
generated thumbnail keeps its old look until the cache is rebuilt.

## Known limits

- HEIC, SVG and DjVu are not supported, and encrypted archives are not opened.
- Animated GIF and animated WebP show their first frame only.
- On macOS there is no preview pane — Quick Look thumbnails only — and wide-gamut AVIF/JXL
  covers are not ICC-transformed yet ([details](./docs/MACOS_IMPLEMENTATION.md)).
- Safety limits skip oversized input: ZIP and 7z scale to any practical size, TAR and RAR are
  capped at 2 GiB, and image decoding stops at 512 MiB against decompression bombs.
- The Windows preview pane shows the cover image only; there is no multi-page gallery.

## Build from source

```sh
cargo build --release        # Windows: arcthumb.dll + arcthumb-config.exe
cargo test                   # shared core; add --no-default-features to check the lean build
./macos/build-appex.sh --release --install   # macOS: .app + .appex, then register
```

You need a stable Rust toolchain (2024 edition); on Windows also the *Desktop development with
C++* workload from Visual Studio Build Tools, and `brew install cmake meson ninja` on macOS to
compile the bundled AVIF decoder. Reinstalling a fresh DLL, building the Inno Setup installer,
the update/donation dialog test hooks and icon regeneration are covered in
[docs/DEVELOPMENT.md](./docs/DEVELOPMENT.md).

## Troubleshooting

**Thumbnails did not appear after installing.** The shell caches "this file has no thumbnail"
too, so files you opened before installing keep the old icon. Press **Regenerate thumbnails**
in the settings window. New files are unaffected, and you only need this once.

**The preview pane is empty.** Check that **Enable preview pane** is on and the pane is visible
(`Alt+P` or **View → Preview pane**). If both hold, kill `prevhost.exe` in Task Manager and
reselect the file — the surrogate sometimes keeps a stale handler.

**Something looks wrong and you want proof.** Turn on diagnostic logging, restart Explorer (or
let Finder recycle the extension), then read `arcthumb.log` in the system temp directory.

## About this fork

[ArcThumbX](https://github.com/HibernalGlow/ArcThumbX) continues
[citrussoda-com/ArcThumb](https://github.com/citrussoda-com/ArcThumb), a Windows-only shell
extension, and keeps merging upstream. What this fork adds:

- **macOS support** — a `QLThumbnailProvider` app extension plus a Swift shim over a
  six-function C ABI (`macos/`, `macos/arcthumb-ffi`), built and signed per architecture in CI.
- **A platform-neutral core** — container, cover-selection, decode, resize and overlay logic
  split out of the Windows backend into `src/thumbnail.rs`, so neither platform re-implements it.
- **AVIF/JXL through WIC on Windows** — the `wic` feature replaces the pure-Rust JXL path with
  the system's imaging codecs ([implementation notes](./docs/WIC_IMPLEMENTATION.md)).
- **The settings window on macOS**, driving the sandboxed extension through a plain
  `key = value` file in its container rather than the registry.
- **简体中文 UI strings** alongside the existing English and Japanese, and a
  `--log-on` / `--log-off` diagnostic logging switch on both platforms.

## License

Dual-licensed under your choice of [MIT](./LICENSE-MIT) or [Apache 2.0](./LICENSE-APACHE).

Third-party components redistributed with `arcthumb-config` are listed in
[THIRD_PARTY_LICENSES.md](./THIRD_PARTY_LICENSES.md). The settings window uses
[Slint](https://slint.dev/) under the Slint Royalty-Free License 2.0, attributed via the
**About** button.

## Credits

The idea comes from [CBXShell](https://github.com/T800G/CBXShell) by T800 Productions and
[DarkThumbs](https://github.com/fire-eggs/DarkThumbs) (originally by kaioa, now maintained by
fire-eggs); the Windows codebase from [ArcThumb](https://github.com/citrussoda-com/ArcThumb).
Implementation uses [windows-rs](https://github.com/microsoft/windows-rs) for COM,
[image](https://github.com/image-rs/image) for decoding,
[zip](https://github.com/zip-rs/zip2) / [unrar](https://github.com/muja/unrar.rs) /
[sevenz-rust](https://crates.io/crates/sevenz-rust) /
[tar](https://github.com/alexcrichton/tar-rs) for archives,
[jxl-oxide](https://crates.io/crates/jxl-oxide) and libavif/dav1d for modern codecs, and
[Slint](https://slint.dev/) for the settings dialog.

Bug reports and feature requests: [GitHub Issues](https://github.com/HibernalGlow/ArcThumbX/issues).
