# macOS implementation notes

How the Quick Look backend is wired to the shared core, and which
decisions were forced by the platform. `docs/WIC_IMPLEMENTATION.md` is the
Windows counterpart; the core underneath both is unchanged.

## Layering

```text
                    ┌──────────────────────────────────────┐
 shared core (Rust) │ arcthumb                             │
                    │  archive/{zip,rar,sevenz,tar,fb2,mobi}│
                    │  ebook/{epub,fb2,mobi}  cover::pick   │
                    │  decode.rs  limits.rs  overlay.rs     │
                    │  thumbnail.rs  →  RGBA8               │
                    └───────────────┬──────────┬────────────┘
                        Windows       │          │      macOS
                    ┌─────────────────┴──┐  ┌───┴──────────────────────────┐
                    │ com.rs IThumbnail- │  │ arc_thumbnail_generate (C ABI)│
                    │ Provider + IStream │  │ macos/arcthumb-ffi            │
                    │ bitmap.rs → HBITMAP│  │  ↓ premultiplied RGBA8        │
                    │ wic.rs (AVIF, JXL) │  │ ThumbnailProvider.swift       │
                    │ registry.rs        │  │ → CGImage → QLThumbnailReply  │
                    └────────────────────┘  └───────────────────────────────┘
```

`src/thumbnail.rs::render` is the seam. It owns the whole pipeline that
decides which pixels a thumbnail is made of — container read, cover pick,
decode, resize, overlay — and returns an `image::RgbaImage`. Explorer
wraps that in a DIB section; the extension hands it to a
`CGDataProvider`. Both convert to premultiplied alpha (`src/pixel.rs`),
because `WTSAT_ARGB` and `kCGImageAlphaPremultipliedLast` both want it.

Everything Windows-only stayed platform-specific and moved behind
`cfg(windows)`: `com`, `bitmap`, `stream`, `preview`, `registry`,
`elevation`, `wic`, plus the `arcthumb-config` binary and the `windows`,
`winreg`, `slint`, `ureq` dependencies (now under
`[target.'cfg(windows)'.dependencies]` — the `windows` crate's bindings do
not compile for other targets at all, so this was mandatory, not
stylistic).

## Why the AVIF backend is what it is

Windows has no WIC-free AVIF path in this project, and macOS has no WIC.
Options considered, with the deciding factor:

| Option | Verdict |
|---|---|
| `image` crate `avif-native` | Needs a **system** dav1d (`dav1d-sys` uses pkg-config `system-deps`). A released `.appex` cannot assume Homebrew. |
| `avif-decode` (libaom) | Its container parser rejects grid-based AVIF. 2×2 grids are the norm for photos above ~1024 px, verified against the crate's source. |
| `libavif-image` + `libavif-sys` + `libdav1d-sys` | **Chosen.** libavif 1.0.4 and dav1d 1.4.3 vendored as C sources, statically linked, BSD-2-Clause. Handles grids, alpha, 8/10/12-bit. Needs `cmake` + `meson`/`ninja` at build time only. |
| pure-Rust AVIF (`rav1d`, `avif-rust`, `zenavif`) | `zenavif` is AGPL. The others are pre-1.0 hand-written AV1 decoders — a worse trade than a build-tool requirement for a component that parses hostile input. |
| macOS ImageIO / VideoToolbox | Excluded by the brief: AVIF and JXL must decode identically to the Rust core, not depend on which codecs the host OS happens to ship. |

Verified locally, not assumed: single-tile AVIF, a 2×2 grid AVIF, and an
alpha-bearing AVIF each decode through the finished extension.

JXL uses `jxl-oxide` (pure Rust, MIT) with its `image` integration —
literally the crate this repository used before commit `4feaef1` moved
Windows onto WIC. It covers 8/16-bit integer and 32-bit float, RGB and
luma, with and without alpha; lossy and lossless streams are both
exercised by the test fixtures.

## Pre-decode limits

`image`'s own `Limits` do not apply to either bundled codec, so
`decode.rs`'s `bundled` module enforces the same guards by hand:

* **AVIF**: parsed once, headers only, to read the *composed* dimensions
  (grid assembly included) through libavif's C API. A source that would
  exceed `MAX_IMAGE_DIMENSION` or `MAX_IMAGE_ALLOC` is rejected before a
  single pixel buffer is allocated. Failures carry libavif's own
  diagnostic string.
* **JXL**: dimensions come from the decoder before `read_image`, and the
  allocation budget is scaled by the reported colour type, because a
  16-bit or float JXL costs 2–4× what an 8-bit one does.
* Everything else keeps the existing `image::Limits` path, and archive
  entry caps (`MAX_ENTRY_SIZE`, `MAX_ARCHIVE_ENTRIES`) are unchanged.

The extension runs in its own process, so a killed allocator costs a
missing thumbnail rather than a dead Finder — but the guard is there so
the failure is a clean `nil` reply.

## FFI contract

`macos/arcthumb-ffi` exposes six functions and three plain `#[repr(C)]`
structs (`include/arcthumb_ffi.h` mirrors them by hand;
`struct_layout_matches_the_c_header` fails if the two drift).

* Settings cross as a struct the **caller** fills, from `UserDefaults`.
  The core therefore never touches a platform config API, and the
  Windows registry loader stays untouched. One trap is documented in
  code: a zero-initialised C struct means "no image format is eligible",
  so `arc_settings_default()` exists and Swift seeds from it.
* The result is an owned `ArcThumbnail` whose `data` points into the same
  allocation (`OwnedThumbnail` keeps the struct at offset zero). Swift
  retains it in a `CGDataProvider`; CoreGraphics calls
  `arc_thumbnail_free` from the release callback. One allocation, no copy,
  freed exactly once.
* Every entry point is wrapped in `catch_unwind`: a panic becomes a null
  return. This is what turns `libavif_image`'s internal `expect()` into a
  missing thumbnail instead of a crash in the host's process.
* State is either immutable or per-thread (the error message), so Finder
  can request thumbnails concurrently; `concurrent_calls_do_not_interfere`
  covers it with 8 threads × 8 requests.

## Quick Look specifics (macOS 27 SDK, verified against the headers)

* `QLFileThumbnailRequest` carries `fileURL`, `maximumSize`, `minimumSize`
  and `scale` — there is no `targetSize`/`targetDPI` (those belong to the
  deprecated `QLGenerator` era). `maximumSize` is in points, so the pixel
  target is `maximumSize × scale`.
* `QLThumbnailReply` offers exactly three factories:
  `contextSize:drawingBlock:`, `contextSize:currentContextDrawingBlock:`
  and `imageFileURL:`. There is no in-memory-image initialiser, so the
  extension draws its `CGImage` into the supplied `CGContext` — the only
  option that avoids writing a temp file.
* Matching is by **UTI**, never by extension. `.cbz/.cbr/.cb7/.cbt` have
  no system type (they resolve to `dyn.ah62…`), so the extension exports
  real UTIs for them; archive types somebody else owns
  (`com.rarlab.rar-archive`, `org.7-zip.7-zip-archive`,
  `org.idpf.epub-container`) are imported rather than re-exported, so we
  do not shadow another app's declaration.
* `NSExtensionPrincipalClass` is the ObjC-visible class name, fixed with
  `@objc(ThumbnailProvider)` so renaming the Swift module can't break
  registration.
* Registration needs three commands (`lsregister -f`, `pluginkit -a`,
  `pluginkit -e use`) — see `macos/README.md`.

## Known limitations

* **ICC**: libavif's YUV→RGB conversion and `jxl-oxide`'s `image`
  integration both leave wide-gamut or non-sRGB profiles untransformed,
  so a P3-tagged cover can look slightly dull. Windows' WIC path
  colour-manages. Fixing this means applying the returned ICC profile with
  a CMS; it changes nothing about which pixels are chosen.
* **HDR JXL/AVIF** are mapped to 8-bit by simple range scaling, so
  wide-gamut HDR covers can clip.
* **Animation**: only the first frame of animated GIF/WebP/JXL/AVIF, same
  as Windows.
* **AVIF `irot`/`imir`** transforms are not applied (rare in practice);
  Exif orientation is applied for every format that carries it, on both
  platforms.
* Preview pane equivalent of Windows' `IPreviewHandler` does not exist
  here yet — Finder gets thumbnails only.
