# Third-party licenses

ArcThumb itself is distributed under **MIT OR Apache-2.0** (see
`LICENSE-MIT` and `LICENSE-APACHE`). The following third-party
components are redistributed with ArcThumb (the `arcthumb.dll` shell
extension, `arcthumb-config.exe`, and the macOS Quick Look extension)
and require separate acknowledgement.

## Dioxus

[Dioxus](https://dioxuslabs.com) is the GUI toolkit for `arcthumb-config` —
the settings panel on Windows and macOS — under the **MIT license**. The
thumbnail extension itself does not link it: it sits behind the `config-gui`
Cargo feature, so `arcthumb.dll` and the macOS Quick Look extension build
without it.

Dioxus's own tree (tao, wry, muda, winit, glutin/skia-free by way of the system
web view) is MIT or Apache-2.0 as well; the panel embeds the system web view
(WebView2 on Windows, WKWebView on macOS) rather than shipping one.

## Fonts

The panel's interface faces are subsets of four open-licensed families, built by
`tools/fonts/build.sh` and embedded in the binary as woff2:

| Embedded family | Derived from | Licence |
|---|---|---|
| ArcThumb Mono | IBM Plex Mono 2.5.0 (IBM) | SIL Open Font License 1.1 |
| ArcThumb Panel, ArcThumb Panel Expanded | Archivo (Google Fonts) | SIL Open Font License 1.1 |
| ArcThumb Pixel | Press Start 2P (Codefaces) | SIL Open Font License 1.1 |
| ArcThumb Sans SC | Noto Sans SC (Google) | SIL Open Font License 1.1 |

Each subset is a modified version, and the OFL forbids a modified version from
keeping a Reserved Font Name — which is why the families above are renamed. The
upstream OFL text for each family ships next to the subsets in
`assets/fonts/licences/`, and the unmodified originals are available from the
sources above.


## Roboto (font)

`arcthumb.dll` embeds an A–Z / 0–9 subset of **Roboto Bold**
(Copyright 2015 Google Inc.) to draw the format labels in the
identification overlay. Roboto is licensed under the **Apache License
2.0**.

The subset font and a copy of its license live in `assets/fonts/`
(`Roboto-Bold-subset.ttf`, `LICENSE-Roboto.txt`); the subsetting
command is recorded in `assets/fonts/README.md`. Only the glyph data
is reduced — the outlines themselves are unmodified.

---

Other Rust crates used by ArcThumb (both the DLL and
`arcthumb-config.exe`) are redistributed under their respective MIT,
Apache-2.0, BSD, or similarly permissive licenses. Running
`cargo tree --format '{p} {l}'` from the repository root will list
every dependency together with its SPDX license identifier.

## libavif + dav1d (AVIF decoding, macOS)

On non-Windows platforms `ArcThumbThumbnail.appex` statically links
[libavif](https://github.com/AOMediaCodec/libavif) and the
[dav1d](https://code.videolan.org/videolan/dav1d) AV1 decoder, brought in
by the `libavif-image` / `libavif-sys` / `libdav1d-sys` crates. Both are
**BSD-2-Clause**, which is compatible with this project's dual licence;
their sources are vendored inside the published crates, so a macOS build
needs no system copy of either library.

Full license text:
https://github.com/AOMediaCodec/libavif/blob/main/LICENSE
https://github.com/videolan/dav1d/blob/master/COPYING

## jxl-oxide (JPEG XL decoding, macOS)

JPEG XL is decoded by [jxl-oxide](https://github.com/tirr-c/jxl-oxide), a
pure-Rust implementation under the **MIT licence**, statically linked into
the extension binary. This is the same crate ArcThumb used for JXL before
the Windows build moved to WIC.

Full license text: https://github.com/tirr-c/jxl-oxide/blob/LICENSE

## unrar (RAR / CBR reading, both platforms)

The `unrar` crate embeds the unRAR source (Copyright Eugene Roshal),
licensed under the **unRAR licence**, which permits use in a thumbnail
generator but forbids making a RAR-compatible *archiver*. ArcThumb only
reads archives, which is squarely inside that permission.

Full license text: https://www.rarlab.com/rar_add/license.txt
