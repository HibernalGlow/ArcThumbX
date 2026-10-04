#!/usr/bin/env python3
"""Build the panel's embedded font subset.

The config window is an offline webview, so the Cassette-Futurism faces cannot
come from a Google Fonts <link>: they ship as woff2 inside the binary. The
upstream families are 7 MB (IBM Plex Mono) and 17 MB (Noto Sans SC), so each
face is subset to the codepoint closure of the Rust/CSS sources under
src/bin/arcthumb-config/. That set is closed and owned by us, and build.rs
refuses to compile a GUI that uses a glyph the embedded subset lacks -- which
is what keeps a new translation from rendering as tofu.

Each derived face is also renamed to an ArcThumb family. That is a licence
requirement, not branding: IBM Plex and Archivo carry a Reserved Font Name in
their OFL entries, and a subset is a modified version, which the OFL forbids
from being distributed under the reserved name.

Run through tools/fonts/build.sh, which fetches the sources first.

Usage:  python3 tools/fonts/subset.py [--src DIR] [--out DIR]
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
GUI_SRC = REPO / "src" / "bin" / "arcthumb-config"

SCAN_SUFFIXES = (".rs", ".css")

# Non-ASCII is harvested from every scanned file, comments included. build.rs
# only reads string literals, so this set is deliberately the wider one: a
# superset can never turn the build gate red spuriously, and each extra glyph
# costs a few hundred bytes.
EXTRA_PRINTABLE = "0123456789ABCDEFXZ#%@.-:/→"

ALWAYS = list(range(0x20, 0x180)) + [
    0x00B7,  # ·
    0x2013, 0x2014,  # dashes
    0x2018, 0x2019, 0x201C, 0x201D,  # curly quotes
    0x2022, 0x2026,  # bullet, ellipsis
    0x2190, 0x2192,  # arrows
    0x2500, 0x2502,  # box drawing
    0x2580, 0x2584, 0x2588, 0x2591, 0x2592, 0x2593,  # block elements
    0x25A0, 0x25A1, 0x2B1B, 0x2B1C,  # squares
    0x25B6, 0x23F9,  # play, stop
    0x2600, 0x263D, 0x263E,  # sun, moons
    0x2713, 0x2717, 0x2731,  # check, cross, heavy asterisk
    0xFE0E, 0xFE0F,  # variation selectors
    0xFF08, 0xFF09, 0xFF0C, 0xFF1A,  # fullwidth punctuation
]

# One entry per shipped face.
#   out       -> written as <out>.woff2
#   source    -> file name (or its stem) inside target/font-src
#   family    -> new name/records after subsetting
#   axes      -> variable-axis instances to freeze; absent = static source
FACES = [
    dict(out="mono-light", source="IBMPlexMono-Light.ttf", family="ArcThumb Mono", style="Light"),
    dict(out="mono-regular", source="IBMPlexMono-Regular.ttf", family="ArcThumb Mono", style="Regular"),
    dict(out="mono-medium", source="IBMPlexMono-Medium.ttf", family="ArcThumb Mono", style="Medium"),
    dict(out="mono-semibold", source="IBMPlexMono-SemiBold.ttf", family="ArcThumb Mono", style="SemiBold"),
    dict(out="panel-bold", source="Archivo[wdth,wght].ttf", family="ArcThumb Panel", style="Bold", axes={"wght": 700, "wdth": 100}),
    dict(out="panel-expanded", source="Archivo", family="ArcThumb Panel Expanded", style="Regular", axes={"wght": 800, "wdth": 125}),
    dict(out="pixel", source="PressStart2P-Regular", family="ArcThumb Pixel", style="Regular"),
    dict(out="sans-sc", source="NotoSansSC[wght].ttf", family="ArcThumb Sans SC", style="Regular", axes={"wght": 400}),
    dict(out="sans-sc-bold", source="NotoSansSC", family="ArcThumb Sans SC", style="Bold", axes={"wght": 700}),
]

WEIGHT_CLASS = {"Light": 300, "Regular": 400, "Medium": 500, "SemiBold": 600, "Bold": 700}

# google/fonts holds every family below at a stable OFL path. The brackets in
# variable-font file names are percent-encoded.
GOOGLE_FONTS = "https://raw.githubusercontent.com/google/fonts/main/ofl"

# Where each source face actually comes from. Homebrew already carries every
# one of them as a cask, and its download cache holds the artifact by content
# hash — so staging from the cache means no 24 MB of re-download for a rebuild.
# `member` is the path inside the cached zip / git-sparse checkout, or None when
# the cached artifact *is* the font file.
CASK_SOURCES = {
    "IBMPlexMono-Light.ttf": ("font-ibm-plex-mono", "ibm-plex-mono/fonts/complete/ttf/IBMPlexMono-Light.ttf"),
    "IBMPlexMono-Regular.ttf": ("font-ibm-plex-mono", "ibm-plex-mono/fonts/complete/ttf/IBMPlexMono-Regular.ttf"),
    "IBMPlexMono-Medium.ttf": ("font-ibm-plex-mono", "ibm-plex-mono/fonts/complete/ttf/IBMPlexMono-Medium.ttf"),
    "IBMPlexMono-SemiBold.ttf": ("font-ibm-plex-mono", "ibm-plex-mono/fonts/complete/ttf/IBMPlexMono-SemiBold.ttf"),
    "Archivo[wdth,wght].ttf": ("font-archivo", "ofl/archivo/Archivo[wdth,wght].ttf"),
    "PressStart2P-Regular.ttf": ("font-press-start-2p", None),
    "NotoSansSC[wght].ttf": ("font-noto-sans-sc", None),
}
# Fallback URLs, used only for whatever staging could not provide.
FALLBACK_URLS = {
    "Archivo[wdth,wght].ttf": f"{GOOGLE_FONTS}/archivo/Archivo%5Bwdth,wght%5D.ttf",
    "PressStart2P-Regular.ttf": f"{GOOGLE_FONTS}/pressstart2p/PressStart2P-Regular.ttf",
    "NotoSansSC[wght].ttf": f"{GOOGLE_FONTS}/notosanssc/NotoSansSC%5Bwght%5D.ttf",
}
TTF_MAGICS = ("00010000", "4f54544f", "74746366")  # \x00\x01\x00\x00, OTTO, ttcf
LICENCES = {
    "ibmplexmono.txt": f"{GOOGLE_FONTS}/ibmplexmono/OFL.txt",
    "archivo.txt": f"{GOOGLE_FONTS}/archivo/OFL.txt",
    "pressstart2p.txt": f"{GOOGLE_FONTS}/pressstart2p/OFL.txt",
    "notosanssc.txt": f"{GOOGLE_FONTS}/notosanssc/OFL.txt",
}


def download(url: str) -> bytes:
    import urllib.request

    with urllib.request.urlopen(url, timeout=180) as response:
        blob = response.read()
    # A 404 page would otherwise be subset into a font with no glyphs.
    if blob[:4].hex() not in TTF_MAGICS and not url.endswith(".txt"):
        raise ValueError(f"{url} did not return a TrueType font ({blob[:16]!r})")
    return blob


def brew_cache(cask: str) -> Path | None:
    import subprocess

    out = subprocess.run(
        ["brew", "--cache", "--cask", cask], capture_output=True, text=True
    )
    if out.returncode != 0:
        return None
    path = Path(out.stdout.strip())
    return path if path.exists() else None


def stage(name: str, cask: str, member: str | None, dest: Path) -> bool:
    """Copy or unpack one face out of Homebrew's download cache."""
    import zipfile

    artifact = brew_cache(cask)
    if artifact is None:
        return False
    dest.parent.mkdir(parents=True, exist_ok=True)
    if artifact.is_dir():
        found = next((x for x in artifact.rglob(Path(member).name) if x.is_file()), None)
        if found is None:
            return False
        dest.write_bytes(found.read_bytes())
        print(f"staged  {name:32s} from {cask} checkout")
        return True
    if zipfile.is_zipfile(artifact):
        with zipfile.ZipFile(artifact) as zf:
            wanted = [n for n in zf.namelist() if n.endswith(Path(member).name)]
            if not wanted:
                return False
            dest.write_bytes(zf.read(wanted[0]))
        print(f"staged  {name:32s} from {cask} zip")
        return True
    if artifact.suffix == ".ttf":
        dest.write_bytes(artifact.read_bytes())
        print(f"staged  {name:32s} from {cask}")
        return True
    return False


def ensure_sources(src_dir: Path, licences_dir: Path, refresh: bool) -> None:
    src_dir.mkdir(parents=True, exist_ok=True)
    licences_dir.mkdir(parents=True, exist_ok=True)
    missing = []
    for name, (cask, member) in CASK_SOURCES.items():
        dest = src_dir / name
        if dest.is_file() and not refresh:
            continue
        if not stage(name, cask, member, dest):
            missing.append(name)
    for name in missing:
        url = FALLBACK_URLS.get(name)
        if url is None:
            sys.exit(
                f"cannot obtain {name}: not in Homebrew's cache and no fallback URL.\n"
                f"run:  brew fetch --cask font-ibm-plex-mono"
            )
        print(f"fetch   {name:32s} {url}")
        (src_dir / name).write_bytes(download(url))
    for name, url in LICENCES.items():
        dest = licences_dir / name
        if dest.is_file() and not refresh:
            continue
        dest.write_bytes(download(url))
        print(f"licence {name:32s} {dest.stat().st_size // 1024} KB")


def codepoints() -> set[int]:
    cps = set(ALWAYS)
    cps.update(ord(c) for c in EXTRA_PRINTABLE)
    for path in sorted(GUI_SRC.rglob("*")):
        if path.is_file() and path.suffix in SCAN_SUFFIXES:
            cps.update(ord(c) for c in path.read_text(encoding="utf-8") if ord(c) > 0x7E)
    return cps


def find_source(src_dir: Path, hint: str) -> Path:
    exact = src_dir / hint
    if exact.is_file():
        return exact
    for path in sorted(src_dir.glob("*.ttf")) + sorted(src_dir.glob("*.otf")):
        if hint in path.name:
            return path
    sys.exit(
        f"no source font matching {hint!r} in {src_dir}\n"
        f"have: {', '.join(p.name for p in sorted(src_dir)) or 'nothing'}\n"
        "run tools/fonts/build.sh to fetch the upstream families"
    )


def freeze(source: Path, axes: dict[str, int] | None, work: Path) -> Path:
    """Pin a variable font to one instance, or copy a static source through."""
    from fontTools.ttLib import TTFont
    from fontTools.varLib.instancer import instantiateVariableFont

    if not axes:
        work.write_bytes(source.read_bytes())
        return work
    tt = TTFont(source)
    present = {a.axisTag for a in tt["fvar"].axes} if "fvar" in tt else set()
    wanted = {k: v for k, v in axes.items() if k in present}
    if wanted:
        instantiateVariableFont(tt, wanted, inplace=True, updateFontNames=False)
    drop_tables(tt)
    tt.save(work)
    return work


def drop_tables(tt) -> None:
    """Leave no variation machinery behind in a frozen instance."""
    for tag in ("fvar", "gvar", "HVAR", "VVAR", "MVAR", "STAT", "avar", "cvar", "vvar"):
        tt.reader.tables.pop(tag, None)
        if tag in tt:
            del tt[tag]


def rename(font: Path, family: str, style: str) -> None:
    from fontTools.ttLib import TTFont

    tt = TTFont(font)
    ps_name = f"{family.replace(' ', '')}-{style.replace(' ', '')}"
    records = {1: family, 2: style, 4: f"{family} {style}", 6: ps_name, 16: family, 17: style}
    for entry in tt["name"].names:
        if entry.nameID in records:
            entry.string = records[entry.nameID]
        elif entry.nameID == 3:
            entry.string = f"{ps_name}:Derivative"
        elif entry.nameID == 5:
            entry.string = "Subset 1.0"
    tt["OS/2"].usWeightClass = WEIGHT_CLASS.get(style, 400)
    drop_tables(tt)
    tt.save(font)


def subset(font: Path, cps: set[int], out: Path) -> None:
    import subprocess

    unicodes = ",".join(f"U+{c:04X}" for c in sorted(cps))
    subprocess.run(
        [
            sys.executable, "-m", "fontTools.subset",
            # The input font is positional: `pyftsubset` has `--font-file=`, not
            # `--font=`, and an unknown option exits 2 with a usage dump.
            str(font),
            f"--unicodes={unicodes}",
            "--layout-features=*",
            "--name-IDs=0,1,2,3,4,5,6,7,8,9,13,14",
            "--notdef-outline",
            "--flavor=woff2",
            f"--output-file={out}",
        ],
        check=True,
    )


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--src", type=Path, default=REPO / "target" / "font-src")
    ap.add_argument("--out", type=Path, default=REPO / "assets" / "fonts")
    ap.add_argument("--refresh", action="store_true", help="re-download the upstream families")
    args = ap.parse_args()

    ensure_sources(args.src, args.out / "licences", refresh=args.refresh)

    cps = codepoints()
    args.out.mkdir(parents=True, exist_ok=True)
    (args.out / "coverage.txt").write_text(
        "".join(f"{c:04X}\n" for c in sorted(cps)), encoding="utf-8"
    )
    cjk = sum(1 for c in cps if c > 0x2E80)
    print(f"{len(cps)} codepoints ({cjk} CJK) -> {args.out / 'coverage.txt'}")

    work_dir = args.src / "work"
    work_dir.mkdir(exist_ok=True)
    total = 0
    for face in FACES:
        source = find_source(args.src, face["source"])
        work = work_dir / f"{face['out']}.ttf"
        freeze(source, face.get("axes"), work)
        rename(work, face["family"], face["style"])
        out = args.out / f"{face['out']}.woff2"
        subset(work, cps, out)
        total += out.stat().st_size
        print(f"  {str(out.relative_to(REPO)):38s} {out.stat().st_size / 1024:7.1f} KB  {face['family']} {face['style']}")
    print(f"total embedded: {total / 1024:.1f} KB")
    return 0


if __name__ == "__main__":
    sys.exit(main())
