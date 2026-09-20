#!/bin/zsh
#
# Build the sample archives used to verify the macOS thumbnail path, and a
# decent cross-platform smoke test of the core.
#
# Each archive holds exactly one cover in a different image format, so a
# missing thumbnail points at one specific codec. Two archives differ on
# purpose: `cover.*` names exercise the cover picker, numbered pages
# exercise the sort order.
#
# Needs: zip, tar (both on macOS) and, for the AVIF / JXL cases, `avifenc`
# from libavif and `cjxl` from jxl-toolbox — `brew install libavif jpeg-xl`.
# Formats whose tooling is missing are skipped with a notice, not an error.
#
#   ./macos/make-fixtures.sh [outdir]     # default: ~/Desktop/arcthumb-fixtures

set -eu
out="${1:-$HOME/Desktop/arcthumb-fixtures}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

have() { command -v "$1" >/dev/null 2>&1; }
say() { print -u2 -- "· $*"; }
skip() { print -u2 -- "! skipped $1 ($2)"; }

have zip || { echo "missing tool: zip" >&2; exit 1; }
mkdir -p "$out"
rm -rf "$out"; mkdir -p "$out"

# ── source images ───────────────────────────────────────────────────────
# Distinct, recognisable pictures so a thumbnail can be checked by eye.
img() { # name w h pattern
  # ${…} braces are load-bearing: a bare $2:rate=1 makes zsh apply the :r
  # modifier to the parameter instead of passing the colon to ffmpeg.
  ffmpeg -hide_banner -loglevel error -y -f lavfi -i "${3}=size=${2}:rate=1" -frames:v 1 "$tmp/$1.png"
}
if have ffmpeg; then
  img grid 512x512 testsrc
  img blobs 800x600 mandelbrot
  img pages 640x480 rgbtestsrc
  img big 3000x2400 testsrc2
else
  say "ffmpeg missing — falling back to sips-generated colour swatches"
  for n in grid blobs pages big; do
    sips -s format png --resampleWidth 512 /System/Library/CoreServices/DefaultDesktop.heic →/dev/null 2>/dev/null || true
  done
  echo "install ffmpeg to generate the fixtures" >&2
  exit 1
fi

# ── covers in every format the core claims ──────────────────────────────
mk_avif() { # src dst extra-args…
  if ! have avifenc; then skip "avif" "no avifenc"; return 1; fi
  avifenc -q 34 "$@" >/dev/null 2>&1
}
mk_jxl() { # src dst extra-args…
  if ! have cjxl; then skip "jxl" "no cjxl"; return 1; fi
  cjxl "$@" >/dev/null 2>&1
}

mk_avif "$tmp/grid.png" "$tmp/cover.avif"                 && say "cover.avif (single tile)"
avifenc -q 32 --grid 2x2 "$tmp/big.png" "$tmp/grid.avif" >/dev/null 2>&1 \
  && say "grid.avif (2×2 grid — naive decoders choke on these)"
mk_jxl "$tmp/blobs.png" "$tmp/cover.jxl"                  && say "cover.jxl (lossy)"
cjxl -d 0 -e 9 "$tmp/blobs.png" "$tmp/lossless.jxl" >/dev/null 2>&1 \
  && say "lossless.jxl"
cjxl -Q 100 "$tmp/grid.png" "$tmp/alpha.jxl" >/dev/null 2>&1 || true

# An alpha-bearing AVIF: PNG with real transparency → premultiply path.
ffmpeg -hide_banner -loglevel error -y -f lavfi -i "testsrc=size=256x256:rate=1" \
  -pix_fmt rgba -frames:v 1 "$tmp/alpha_src.png"
if have avifenc; then
  avifenc -q 34 "$tmp/alpha_src.png" "$tmp/alpha.avif" >/dev/null 2>&1 && say "alpha.avif"
fi

cp "$tmp/grid.png"  "$tmp/cover.png"
ffmpeg -hide_banner -loglevel error -y -i "$tmp/blobs.png" -q:v 4 "$tmp/cover.jpg"
ffmpeg -hide_banner -loglevel error -y -i "$tmp/pages.png" -vf format=yuv420p -frames:v 1 "$tmp/cover.webp" 2>/dev/null || cp "$tmp/pages.png" "$tmp/cover.webp"
ffmpeg -hide_banner -loglevel error -y -i "$tmp/grid.png" -frames:v 1 "$tmp/cover.gif"
ffmpeg -hide_banner -loglevel error -y -i "$tmp/grid.png" -frames:v 1 "$tmp/cover.tif"
ffmpeg -hide_banner -loglevel error -y -i "$tmp/grid.png" -frames:v 1 "$tmp/cover.bmp"

# A rotated JPEG: EXIF Orientation=6 (90° CW) with landscape pixels, so the
# thumbnail must come back portrait-orientated.
ffmpeg -hide_banner -loglevel error -y -i "$tmp/pages.png" -q:v 4 -metadata orientation=6 "$tmp/rotated.jpg" 2>/dev/null || cp "$tmp/cover.jpg" "$tmp/rotated.jpg"

# ── packers ─────────────────────────────────────────────────────────────
# zip / cbz / epub
pack_zip() { # archive entry=source…
  local name="$1"; shift
  local d="$tmp/z-$RANDOM"; mkdir -p "$d"
  local e
  for pair in "$@"; do
    e="${pair%%=*}"; src="${pair#*=}"
    mkdir -p "$d/$(dirname "$e")"
    cp "$src" "$d/$e"
  done
  (cd "$d" && find . -type f -print | sed 's|^\./||' | zip -q -X "$out/$name" -@)
  rm -rf "$d"
  say "$name"
}
pack_tar() { # archive entry=source…
  local name="$1"; shift
  local d="$tmp/t-$RANDOM"; mkdir -p "$d"
  local pair e src
  for pair in "$@"; do e="${pair%%=*}"; src="${pair#*=}"; mkdir -p "$d/$(dirname "$e")"; cp "$src" "$d/$e"; done
  (cd "$d" && tar -cf "$out/$name" .)
  rm -rf "$d"
  say "$name"
}

pack_zip test.cbz        cover.jpg="$tmp/cover.jpg"
pack_zip cover-avif.cbz  cover.avif="$tmp/cover.avif"
pack_zip cover-jxl.cbz   cover.jxl="$tmp/cover.jxl"
pack_zip grid-avif.cbz   001.jpg="$tmp/grid.avif"          # grid AVIF, page name
pack_zip lossless-jxl.cbz 001.jxl="$tmp/lossless.jxl"
pack_zip alpha.avif.cbz  cover.avif="$tmp/alpha.avif"
pack_zip formats.cbz     001.png="$tmp/cover.png" 002.webp="$tmp/cover.webp" 003.gif="$tmp/cover.gif" 004.tif="$tmp/cover.tif" 005.bmp="$tmp/cover.bmp"
pack_zip rotated.cbz     cover.jpg="$tmp/rotated.jpg"
pack_zip pages.zip       ch1/page2.jpg="$tmp/cover.jpg" ch1/page10.jpg="$tmp/pages.png"   # natural sort: page2 wins
pack_zip nocover.zip     notes.txt=/etc/hosts README.md=/etc/hosts                        # no image at all
pack_tar test.cbt        cover.png="$tmp/cover.png"
printf 'not an archive, not an archive, not an archive' > "$tmp/garbage.bin"
cp "$tmp/garbage.bin" "$out/corrupt-file.zip"
: > "$out/empty.zip"

# A truncated CBZ: valid central directory, clipped payload.
python3 - "$out/test.cbz" "$out/truncated.cbz" <<'PY' || true
import sys
src, dst = sys.argv[1], sys.argv[2]
data = open(src, 'rb').read()
open(dst, 'wb').write(data[: max(64, len(data) // 3)])
PY

say "fixtures in $out"
ls -1 "$out"
