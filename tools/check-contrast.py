#!/usr/bin/env python3
"""Verify the panel's text/surface pairs against WCAG AA, by number.

The catfu contract makes contrast load-bearing rather than stylistic: its
labels are 9-12px, and "eyeballed" is exactly how a 9px silk-screen caption ends
up at 3.1:1. So the ratios are computed from the token values as they appear in
the stylesheet — the script parses the two token blocks instead of restating
them, which means it goes red when somebody darkens a token to "look more
subtle".

Threshold: 4.5:1 for text under 18px (everything here is), 3:1 for the lit
screen text that the contract pins at 11-12px but renders bold and glowing.

Usage: python3 tools/check-contrast.py [--self-test]
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

CSS = [
    Path(__file__).resolve().parents[1] / "src/bin/arcthumb-config/app/style.css",
    Path(__file__).resolve().parents[1] / "src/bin/arcthumb-config/app/screen.css",
]

# (label, ink token, surface token, minimum ratio)
PAIRS = [
    ("body copy", "ink", "bg", 4.5),
    ("panel title on header", "ink", "panel-2", 4.5),
    ("hint copy", "ink-dim", "panel", 4.5),
    ("silk-screen label", "ink-dim", "panel", 4.5),
    ("strip label", "ink-dim", "chrome", 4.5),
    ("fine print", "ink-faint", "chrome", 4.5),
    ("keycap caption (off)", "ink-dim", "panel-2", 4.5),
    ("keycap caption (lit)", "ink-on-blue", "blue", 4.5),
    ("switch label", "ink", "panel", 4.5),
    ("inert row", "ink-faint", "panel", 4.5),
    ("LCD text", "screen-blue", "lcd-bg", 4.5),
    ("LCD alert text", "amber", "lcd-bg", 4.5),
    ("CRT phosphor", "amber", "crt-bg", 4.5),
    ("CRT highlight", "amber-hi", "crt-bg", 4.5),
    ("CRT key column", "screen-gray", "crt-bg", 4.5),
    ("chip", "amber", "crt-bg", 4.5),
    ("LED against its bezel", "green", "led-bezel", 3.0),
    ("lit blue LED against bezel", "screen-blue", "led-bezel", 3.0),
]

# The contract's screen-internal grays are not CSS tokens (they are fixed on
# purpose), so they are restated here as the sanctioned constants.
SCREEN_CONSTANTS = {"screen-gray": "#9a9b95"}


def block(text: str, header: str) -> str:
    """The declarations of one token block, from `header` to its closing brace."""
    start = text.index(header) + len(header)
    depth = 1
    i = start
    while depth:
        if text[i] == "{":
            depth += 1
        elif text[i] == "}":
            depth -= 1
        i += 1
    return text[start:i]


def tokens(text: str) -> tuple[dict[str, str], dict[str, str]]:
    """(dark tokens, light tokens), each name -> value."""
    joined = "\n".join(path.read_text() for path in CSS)
    dark = dict(re.findall(r"--([\w-]+):\s*([^;]+);", block(joined, ".chassis {")))
    light = dict(re.findall(r"--([\w-]+):\s*([^;]+);", block(joined, '.chassis.t-light {')))
    # The light block only restates what changes; the rest inherits.
    merged = {**dark, **light}
    for name, value in SCREEN_CONSTANTS.items():
        dark.setdefault(name, value)
        merged.setdefault(name, value)
    return dark, merged


def resolve(name: str, table: dict[str, str], depth: int = 0) -> str:
    value = table[name].strip()
    if value.startswith("var("):
        inner = value[4:-1].strip().removeprefix("--")
        if depth > 8:
            sys.exit(f"token cycle at --{name}")
        return resolve(inner, table, depth + 1)
    return value


def rgb(color: str) -> tuple[int, int, int]:
    color = color.strip()
    if color.startswith("#"):
        hexpart = color[1:]
        if len(hexpart) == 3:
            hexpart = "".join(c * 2 for c in hexpart)
        if len(hexpart) not in (6, 8):
            sys.exit(f"not a hex colour: {color!r}")
        return tuple(int(hexpart[i : i + 2], 16) for i in (0, 2, 4))  # type: ignore[return-value]
    if color.startswith("rgba(") or color.startswith("rgb("):
        parts = [p.strip() for p in color[color.index("(") + 1 : color.index(")")].split(",")]
        r, g, b = (int(float(p)) for p in parts[:3])
        alpha = float(parts[3]) if len(parts) > 3 else 1.0
        return r, g, b  # callers composite where it matters
    sys.exit(f"unsupported colour: {color!r}")


def luminance(color: tuple[int, int, int]) -> float:
    def channel(value: int) -> float:
        s = value / 255
        return s / 12.92 if s <= 0.03928 else ((s + 0.055) / 1.055) ** 2.4

    r, g, b = (channel(c) for c in color)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def ratio(fg: str, bg: str, table: dict[str, str]) -> float:
    f, b = resolve(fg, table), resolve(bg, table)
    # A translucent ink over a lit screen is the one case that needs
    # compositing; here both sides are opaque, so this stays simple.
    if "rgba" in f:
        return 1.0
    return (max(luminance(rgb(f)), luminance(rgb(b))) + 0.05) / (
        min(luminance(rgb(f)), luminance(rgb(b))) + 0.05
    )


def check(theme: str, table: dict[str, str]) -> list[str]:
    bad = []
    for label, fg, bg, minimum in PAIRS:
        try:
            got = ratio(fg, bg, table)
        except KeyError as e:
            bad.append(f"{theme}: {label} references unknown token {e}")
            continue
        if got < minimum:
            bad.append(
                f"{theme}: {label}  {resolve(fg, table)} on {resolve(bg, table)} "
                f"= {got:.2f}:1, needs {minimum}:1"
            )
    return bad


def self_test() -> None:
    """The checker must be able to see a violation, or it proves nothing."""
    fake = {"ink": "#909089", "bg": "#ece7dc", "ink-on-blue": "#04101c", "blue": "#2e7dc4"}
    got = ratio("ink", "bg", fake)
    assert got < 4.5, f"control failed: washed-out grey measured {got:.2f}:1, should fail"
    good = ratio("ink-on-blue", "blue", {"ink-on-blue": "#f6fbff", "blue": "#1f6fb8"})
    assert good > 4.5, f"control failed: white on cobalt measured {good:.2f}:1"
    print(f"self-test ok — a bad pair reads {got:.2f}:1, a good pair {good:.2f}:1")


def main() -> int:
    dark, light = tokens("")
    if "--self-test" in sys.argv:
        self_test()
        return 0
    failures = check("dark", dark) + check("light", light)
    rows = []
    for theme, table in (("dark", dark), ("light", light)):
        for label, fg, bg, minimum in PAIRS:
            try:
                rows.append(f"{label:24s} {theme:5s} {ratio(fg, bg, table):5.2f}:1  (min {minimum})")
            except Exception:
                pass
    print("\n".join(rows))
    if failures:
        print("\n".join(failures))
        print(f"FAIL: {len(failures)} pair(s) under AA")
        return 1
    print("PASS: every text/surface pair clears its threshold in both themes")
    self_test()
    return 0


if __name__ == "__main__":
    sys.exit(main())
