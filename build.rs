//! Build script: embed the Windows app manifest and icon, and gate the panel's
//! embedded fonts against the labels it can actually show.
//!
//! The manifest declares Per-Monitor DPI v2 and Common Controls v6 so
//! `arcthumb-config.exe` scales correctly on mixed-DPI setups and picks up the
//! modern visual style. Cargo links the compiled `.res` into every output
//! artifact, so `arcthumb.dll` also carries the manifest; that is harmless, the
//! shell extension ignores its own manifest.
//!
//! The font gate is the other half. The panel's faces are subsets — see
//! `tools/fonts/subset.py` — so a glyph nobody cut into them renders as a tofu
//! box in the middle of a localized label. Rather than discover that by eye on
//! a translated row, every `config-gui` build re-derives the codepoints that
//! appear in the config GUI's string literals and refuses to compile unless the
//! committed subset covers them.

use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

fn target_is_windows() -> bool {
    std::env::var_os("CARGO_CFG_WINDOWS").is_some()
}

/// Is the `arcthumb-config` GUI being built? Its dependencies (`dioxus`) are
/// optional, so the font gate must not run when they are switched off — an
/// extension-only build shows no labels and needs no glyphs.
fn config_gui_enabled() -> bool {
    std::env::var_os("CARGO_FEATURE_CONFIG_GUI").is_some()
}

fn main() {
    println!("cargo:rerun-if-changed=resources/arcthumb-config.rc");
    println!("cargo:rerun-if-changed=resources/arcthumb-config.manifest");
    println!("cargo:rerun-if-changed=assets/icon.ico");
    println!("cargo:rerun-if-changed=assets/fonts/coverage.txt");
    println!("cargo:rerun-if-changed=src/bin/arcthumb-config");

    // The manifest is mandatory (it declares DPI awareness and Common Controls
    // v6), so treat a missing RC compiler or a failed compile as a hard build
    // error rather than silently shipping a binary without it. Both steps are
    // Windows-only, and so are the crates they need.
    if target_is_windows() {
        embed_resource::compile("resources/arcthumb-config.rc", embed_resource::NONE)
            .manifest_required()
            .expect("failed to embed Windows resource (manifest + icon)");
    }

    if config_gui_enabled() {
        check_font_coverage();
    }
}

/// `\u{…}` — the one escape that can reach a non-ASCII glyph without the source
/// file containing that glyph. Returns the codepoint and the index just past the
/// closing brace, or `None` when this backslash starts some other escape.
fn unicode_escape(bytes: &[char], at: usize) -> Option<(u32, usize)> {
    if bytes.get(at)? != &'\\' || bytes.get(at + 1)? != &'u' || bytes.get(at + 2)? != &'{' {
        return None;
    }
    let offset = at + 3;
    let end = offset + bytes[offset..].iter().position(|c| *c == '}')?;
    let hex: String = bytes[offset..end].iter().collect();
    Some((u32::from_str_radix(&hex, 16).ok()?, end + 1))
}

/// Every codepoint the panel can print, harvested from the string literals of
/// its own sources.
///
/// Deliberately narrower than the generator, which reads whole files including
/// comments: this is the set that must be a subset of `coverage.txt`, and the
/// only way a new label can turn the build red is by using a glyph nobody cut.
fn check_font_coverage() {
    let root = PathBuf::from(std::env::var_os("CARGO_MANIFEST_DIR").expect("cargo sets this"));
    let gui = root.join("src/bin/arcthumb-config");
    let coverage_path = root.join("assets/fonts/coverage.txt");

    let covered = match read_coverage(&coverage_path) {
        Ok(set) => set,
        Err(reason) => fail(&format!(
            "{}: {reason}\n\
             The panel's faces are embedded subsets and must be regenerated.\n\
             Fix:  tools/fonts/build.sh",
            coverage_path.display()
        )),
    };

    // Two controls, because a gate that measures nothing is worse than no gate:
    // the subset must contain CJK at all, and the scanner must find *something*
    // non-ASCII. If the string tables move elsewhere, or the literal scanner
    // breaks, the second one is what says so instead of passing quietly.
    let mut needed: BTreeSet<u32> = BTreeSet::new();
    let mut offenders: Vec<(String, char)> = Vec::new();
    for file in sources(&gui) {
        let text = std::fs::read_to_string(&file)
            .unwrap_or_else(|e| fail(&format!("read {}: {e}", file.display())));
        let relative = file
            .strip_prefix(&root)
            .unwrap_or(file.as_path())
            .display()
            .to_string();
        let found = literal_codepoints(&text);
        if relative.ends_with("locale.rs") && found.is_empty() {
            fail(
                "the literal scanner found no non-ASCII in locale.rs, so the font gate \
                  is measuring nothing — check tools/fonts' scanner before trusting it",
            );
        }
        for ch in found {
            needed.insert(ch);
            if !covered.contains(&ch) && !offenders.iter().any(|(_, c)| *c as u32 == ch) {
                offenders.push((relative.clone(), char::from_u32(ch).unwrap_or('\u{FFFD}')));
            }
        }
    }

    if !covered.iter().any(|cp| *cp > 0x2E80) {
        fail(
            "assets/fonts/coverage.txt lists no CJK codepoint: the panel cannot draw a \
              Chinese label. Fix:  tools/fonts/build.sh",
        );
    }
    if needed.is_empty() {
        fail(
            "no non-ASCII literals found anywhere under src/bin/arcthumb-config — the \
              font gate has nothing to check, which is itself the failure",
        );
    }
    if !offenders.is_empty() {
        let list = offenders
            .iter()
            .take(12)
            .map(|(file, ch)| format!("  U+{:04X} '{}' in {}", *ch as u32, ch, file))
            .collect::<Vec<_>>()
            .join("\n");
        fail(&format!(
            "the embedded font subsets cannot draw {count} glyph(s) the panel prints:\n\
             {list}\n\
             Fix:  tools/fonts/build.sh   (then commit assets/fonts/)",
            count = offenders.len(),
        ));
    }
}

fn read_coverage(path: &Path) -> Result<BTreeSet<u32>, String> {
    let text = std::fs::read_to_string(path).map_err(|e| e.to_string())?;
    let set: BTreeSet<u32> = text
        .lines()
        .map(str::trim)
        .filter(|line| !line.is_empty() && !line.starts_with('#'))
        .filter_map(|line| u32::from_str_radix(line, 16).ok())
        .collect();
    if set.is_empty() {
        return Err("no codepoints listed".into());
    }
    Ok(set)
}

/// `.rs` and `.css` files under the config GUI's directory.
fn sources(dir: &Path) -> Vec<PathBuf> {
    let mut out = Vec::new();
    let mut stack = vec![dir.to_path_buf()];
    while let Some(dir) = stack.pop() {
        let Ok(entries) = std::fs::read_dir(&dir) else {
            continue;
        };
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_dir() {
                stack.push(path);
            } else if matches!(
                path.extension().and_then(|e| e.to_str()),
                Some("rs") | Some("css")
            ) {
                out.push(path);
            }
        }
    }
    out.sort();
    out
}

/// Non-ASCII characters that appear inside string literals, and nowhere else.
///
/// Handles `"…"` (with `\\` and `\"` escapes), `b"…"`, raw strings `r"…"`,
/// `r#"…"#` and their `b` forms. Byte strings and raw-byte strings are scanned
/// byte-wise by the same routine, which is what `tools/fonts/subset.py` expects:
/// it harvests more than this does, so an over-eager here is harmless while an
/// under-eager one would let a label render as tofu.
fn literal_codepoints(text: &str) -> BTreeSet<u32> {
    let bytes: Vec<char> = text.chars().collect();
    let mut found = BTreeSet::new();
    let mut i = 0usize;

    while i < bytes.len() {
        // A `#` or `b` prefix may sit immediately before the opening quote.
        let mut start = i;
        if bytes[i] == 'b' && i + 1 < bytes.len() && bytes[i + 1] == '"' {
            start = i + 1;
        } else if bytes[i] == 'r' {
            let mut j = i + 1;
            while j < bytes.len() && bytes[j] == '#' {
                j += 1;
            }
            // `r"…"`, `r#"…"#`, `br#"…"#`
            let hash_count = j - (i + 1);
            if j < bytes.len() && bytes[j] == '"' {
                let body = j + 1;
                let mut k = body;
                while k < bytes.len() {
                    if bytes[k] == '"' && bytes[k + 1..].iter().take(hash_count).all(|c| *c == '#')
                    {
                        break;
                    }
                    k += 1;
                }
                for ch in bytes[body..k.min(bytes.len())].iter().copied() {
                    if ch as u32 > 0x7E {
                        found.insert(ch as u32);
                    }
                }
                i = k + 1 + hash_count;
                continue;
            }
        }

        if bytes[start] != '"' {
            i += 1;
            continue;
        }

        // Ordinary string: walk to the closing quote, honouring escapes.
        let mut k = start + 1;
        while k < bytes.len() {
            match bytes[k] {
                '\\' => {
                    if let Some((cp, next)) = unicode_escape(&bytes, k) {
                        found.insert(cp);
                        k = next;
                        continue;
                    }
                    k += 2;
                }
                '"' => break,
                ch if ch as u32 > 0x7E => {
                    found.insert(ch as u32);
                    k += 1;
                }
                _ => k += 1,
            }
        }
        i = k + 1;
    }

    found
}

fn fail(message: &str) -> ! {
    // `error:` on its own line is what turns this into a hard failure with a
    // message the user can act on, rather than a build that ships a panel with
    // squares in place of words.
    println!("cargo:error=true");
    println!("cargo:warning={message}");
    eprintln!("error: {message}");
    std::process::exit(1);
}
