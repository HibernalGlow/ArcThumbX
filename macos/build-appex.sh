#!/bin/zsh
#
# Build ArcThumb's macOS Quick Look thumbnail extension.
#
#   ./macos/build-appex.sh                 # debug, arm64, ad-hoc signed
#   ./macos/build-appex.sh --release --arch=x86_64   # cross *from* an Intel Mac
#   ./macos/build-appex.sh --release                 # release, host arch
#   ./macos/build-appex.sh --install       # build, then register + launch
#
# No Xcode project is involved: the extension is two Swift files, so the
# script drives `swiftc` directly and `codesign` assembles the signature.
# Everything lands in macos/build/.
#
# Prereqs beyond the Rust toolchain: the crates in
# `[target.'cfg(not(windows))'.dependencies]` vendor libavif and dav1d as
# C sources, so `cmake`, `meson`, `ninja` and a working `cc` must be
# installed (plus `nasm` for x86_64 builds).

set -euo pipefail

ROOT="${0:A:h:h}"
BUILD="$ROOT/macos/build"
HOST_BUNDLE_ID="com.citrussoda.ArcThumb"
EXT_BUNDLE_ID="com.citrussoda.ArcThumb.thumbnail"
HOST_NAME="ArcThumb"
EXT_NAME="ArcThumbThumbnail"
VERSION="$(sed -n 's/^version = "\([^"]*\)"/\1/p' "$ROOT/Cargo.toml" | head -1)"
BUILDnumber="$(git -C "$ROOT" rev-list --count HEAD 2>/dev/null || echo 0)"

release=0
# Target architecture. Defaults to the host's, and that is deliberate: the
# bundled dav1d is built by meson, which has no macOS cross-compile config in
# `libdav1d-sys`, so asking for the other arch here would quietly produce
# objects for the host. Build each artifact on a machine (or runner) of its
# own architecture.
arch="$(uname -m)"
install=0
sign_id="-"

for arg in "$@"; do
  case "$arg" in
    --release) release=1 ;;
    --arch=*) arch="${arg#--arch=}" ;;
    --install) install=1 ;;
    --sign=*)
      sign_id="${arg#--sign=}"
      # An empty value means "no identity configured" (CI passes
      # `--sign="${{ vars.MACOS_SIGN_IDENTITY }}"`, which expands to nothing
      # when the repo variable is unset). Fall back to ad-hoc rather than
      # handing codesign an empty -s argument.
      [[ -z "$sign_id" ]] && sign_id="-"
      ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

case "$arch" in
  arm64) rust_triples=(aarch64-apple-darwin) ;;
  x86_64) rust_triples=(x86_64-apple-darwin) ;;
  *) echo "unknown --arch: $arch (expected arm64 or x86_64)" >&2; exit 2 ;;
esac
if [[ "$arch" != "$(uname -m)" ]]; then
  say "note: host is $(uname -m) but --arch=$arch was requested; the vendored"
  say "      dav1d cannot be cross-built (no meson cross file upstream)."
fi
host_bins=()

profile=""
rust_profile="debug"
if (( release )); then profile="--release"; rust_profile="release"; fi

say() { print -u2 -- "── $*" }

# Pick a cargo that can actually see the targets we ask for. Homebrew ships a
# bare `cargo` that ignores `rustup target add`, so a build would install
# the std into the rustup toolchain and then fail to find it. When
# `cargo` is not the rustup proxy but rustup is installed, run through the
# stable toolchain explicitly.
CARGO=(cargo)
if ! cargo -vV 2>/dev/null | rg -q "rustup" && (( $+commands[rustup] )); then
  # A standalone cargo (Homebrew's) cannot see targets installed with
  # `rustup target add`, and `rustup run stable cargo` does not help either:
  # it leaves the Homebrew cargo *and rustc* ahead on PATH, so the build
  # still uses the wrong sysroot. Call the toolchain's own binaries and pin
  # RUSTC to the same directory.
  if tc_cargo=$(rustup which --toolchain stable cargo 2>/dev/null); then
    CARGO=("$tc_cargo")
    export RUSTC="${tc_cargo:h}/rustc"
  fi
fi
say "using: ${CARGO[*]} (rustc: ${RUSTC:-$(command -v rustc)})"

# ── 1. Rust core → staticlib ──────────────────────────────────────────────
libs=()
for rust_target in "${rust_triples[@]}"; do
  if [[ "$rust_target" != "$(rustc -vV | sed -n 's/^host: //p')" ]]; then
    rustup target add "$rust_target" >/dev/null
  fi
  # Pin the deployment target for the whole tree (rustc, cc, cmake and meson
# all honour it), so the vendored C objects do not come back built for the
# build host's OS version. 11.0 is the floor for a Swift 5 + Rust extension.
export MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-11.0}"
say "cargo build --target $rust_target $profile"
  ( cd "$ROOT/macos/arcthumb-ffi" && cargo build ${=profile} --target "$rust_target" )
  libs+=("$ROOT/macos/arcthumb-ffi/target/$rust_target/$rust_profile/libarcthumb_ffi.a")
  rust_triples+=("$rust_target")
done

staticlib="${libs[1]}"
mkdir -p "$BUILD"
say "static library: $arch $(du -h "$staticlib" | cut -f1)"

# ── 2. Swift sources → extension binary ──────────────────────────────────
appex="$BUILD/$HOST_NAME.app/Contents/PlugIns/$EXT_NAME.appex"
rm -rf "$appex"
mkdir -p "$appex/Contents/MacOS" "$appex/Contents/Resources"

subst() {
  sed -e "s|\$(PRODUCT_BUNDLE_IDENTIFIER)|$EXT_BUNDLE_ID|g" \
      -e "s|\$(HOST_BUNDLE_ID)|$HOST_BUNDLE_ID|g" \
      -e "s|\$(MARKETING_VERSION)|$VERSION|g" \
      -e "s|\$(CURRENT_PROJECT_VERSION)|$BUILDnumber|g" "$1"
}

swiftc_opts=(
  -target "$(uname -m)-apple-macos${MACOSX_DEPLOYMENT_TARGET}"
  -sdk "$(xcrun --sdk macosx --show-sdk-path)"
  -framework QuickLookThumbnailing -framework CoreGraphics -framework Foundation
  -Xlinker -L"$(dirname "$staticlib")" -Xlinker -larcthumb_ffi
  # The Rust staticlib pulls in C++ (the unrar backend) and the system
  # allocator, so link the C++ runtime explicitly.
  -Xlinker -lc++
)
if (( release )); then swiftc_opts+=(-O); else swiftc_opts+=(-g -Onone); fi

say "clang → extension_main.o"
xcrun clang -c -target "$arch-apple-macos$MACOSX_DEPLOYMENT_TARGET" \
  -isysroot "$(xcrun --sdk macosx --show-sdk-path)" \
  -o "$BUILD/extension_main.o" "$ROOT/macos/extension/extension_main.c"

# The extension binary and the host binary are both built for $arch only;
# see the note on `--arch` above for why there is no fat-binary mode.
say "swiftc → $EXT_NAME ($arch)"
xcrun swiftc "${swiftc_opts[@]}" "$BUILD/extension_main.o" \
  -target "$arch-apple-macos$MACOSX_DEPLOYMENT_TARGET" \
  -import-objc-header "$ROOT/macos/arcthumb-ffi/include/arcthumb_ffi.h" \
  -module-name "$EXT_NAME" \
  -o "$appex/Contents/MacOS/$EXT_NAME" \
  "$ROOT/macos/extension/ThumbnailProvider.swift" \
  "$ROOT/macos/extension/ArcThumbCore.swift"

subst "$ROOT/macos/extension/Info.plist.in" > "$appex/Contents/Info.plist"
printf 'APPL????' > "$appex/Contents/PkgInfo"

# ── 3. Host app that carries the extension ───────────────────────────────
host="$BUILD/$HOST_NAME.app"
mkdir -p "$host/Contents/MacOS" "$host/Contents/Resources"
subst "$ROOT/macos/host/Info.plist.in" > "$host/Contents/Info.plist"
# The host executable is the shared `arcthumb-config` binary — the same
# Slint window as on Windows, driving the macOS settings store. Built from
# the workspace root, so the UI file and string tables are not duplicated.
triple="${rust_triples[1]}"
say "cargo build --bin arcthumb-config --target $triple $profile"
( cd "$ROOT" && "${CARGO[@]}" build ${=profile} --bin arcthumb-config --target "$triple" )
cp "$ROOT/target/$triple/$rust_profile/arcthumb-config" "$host/Contents/MacOS/$HOST_NAME"

# Finder wants a .icns; the repo ships a PNG. Skipped quietly when the
# source is too small to downsample from, since a missing icon is cosmetic.
if [[ -f "$ROOT/assets/icon.png" ]]; then
  width=$(sips -g pixelWidth "$ROOT/assets/icon.png" 2>/dev/null | awk '/pixelWidth/{print $2}')
  if (( ${width:-0} >= 512 )); then
    iconset="$BUILD/AppIcon.iconset"
    rm -rf "$iconset"; mkdir -p "$iconset"
    for size in 16 32 64 128 256 512; do
      sips -z $size $size "$ROOT/assets/icon.png" \
        --out "$iconset/icon_${size}x${size}.png" >/dev/null
      sips -z $((size * 2)) $((size * 2)) "$ROOT/assets/icon.png" \
        --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
    done
    iconutil -c icns "$iconset" -o "$host/Contents/Resources/AppIcon.icns" \
      && rm -rf "$iconset" \
      || say "warning: iconutil failed; the app will use the generic icon"
  else
    say "note: assets/icon.png is ${width:-0}px wide; needs >=512px for an .icns"
  fi
fi
printf 'APPL????' > "$host/Contents/PkgInfo"

# ── 4. Sign: extension first, then the bundle that embeds it ─────────────
say "codesign (id: $sign_id)"
codesign --force --sign "$sign_id" --timestamp=none \
  --entitlements "$ROOT/macos/extension/ArcThumbThumbnail.entitlements" "$appex"
codesign --force --sign "$sign_id" --timestamp=none "$host"
codesign --verify --verbose=2 "$host"

say "built $host"

# ── 5. Optional: register with the system ────────────────────────────────
if (( install )); then
  dest="$HOME/Applications/$HOST_NAME.app"
  mkdir -p "$HOME/Applications"
  [[ -d "$dest" ]] && rm -rf "$dest"
  cp -R "$host" "$dest"
  LSREGISTER=/System/Library/Frameworks/CoreServices.framework/Versions/A/Frameworks/LaunchServices.framework/Versions/A/Support/lsregister
  "$LSREGISTER" -f "$dest"
  # `lsregister` alone leaves a thumbnail extension invisible to Quick Look;
  # the plug-in has to be announced to the plug-ins daemon as well.
  pluginkit -a "$dest/Contents/PlugIns/$EXT_NAME.appex"
  qlmanage -r cache 2>/dev/null || true
  say "installed $dest"
  print -u2 -- "Registered. Verify with:  pluginkit -m -A | grep $EXT_BUNDLE_ID"
fi
