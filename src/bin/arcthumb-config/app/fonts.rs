//! The panel's type, embedded.
//!
//! A webview with no network cannot reach a font CDN, and a settings dialog
//! that paints in the system sans is not a Cassette-Futurist instrument panel —
//! the faces *are* the aesthetic. So the four families compile into the binary
//! as base64 woff2 and ship in the served HTML head, before the first paint.
//!
//! They are subsets, not the full families: see `tools/fonts/subset.py` for how
//! the glyph closure is derived from the panel's own strings, and `build.rs` for
//! the gate that refuses to compile a label the embedded subset cannot draw.
//! The derived faces also carry renamed families, because the OFL forbids a
//! modified version from keeping a Reserved Font Name.

use base64::Engine as _;

struct Face {
    family: &'static str,
    style: &'static str,
    weight: u16,
    bytes: &'static [u8],
}

/// Declaration order is `@font-face` order; the CSS stacks in `style.css`
/// reference these families by name.
const FACES: &[Face] = &[
    Face {
        family: "ArcThumb Mono",
        style: "normal",
        weight: 300,
        bytes: include_bytes!("../../../../assets/fonts/mono-light.woff2"),
    },
    Face {
        family: "ArcThumb Mono",
        style: "normal",
        weight: 400,
        bytes: include_bytes!("../../../../assets/fonts/mono-regular.woff2"),
    },
    Face {
        family: "ArcThumb Mono",
        style: "normal",
        weight: 500,
        bytes: include_bytes!("../../../../assets/fonts/mono-medium.woff2"),
    },
    Face {
        family: "ArcThumb Mono",
        style: "normal",
        weight: 600,
        bytes: include_bytes!("../../../../assets/fonts/mono-semibold.woff2"),
    },
    Face {
        family: "ArcThumb Panel",
        style: "normal",
        weight: 700,
        bytes: include_bytes!("../../../../assets/fonts/panel-bold.woff2"),
    },
    Face {
        family: "ArcThumb Panel Expanded",
        style: "normal",
        weight: 800,
        bytes: include_bytes!("../../../../assets/fonts/panel-expanded.woff2"),
    },
    Face {
        family: "ArcThumb Pixel",
        style: "normal",
        weight: 400,
        bytes: include_bytes!("../../../../assets/fonts/pixel.woff2"),
    },
    Face {
        family: "ArcThumb Sans SC",
        style: "normal",
        weight: 400,
        bytes: include_bytes!("../../../../assets/fonts/sans-sc.woff2"),
    },
    Face {
        family: "ArcThumb Sans SC",
        style: "normal",
        weight: 700,
        bytes: include_bytes!("../../../../assets/fonts/sans-sc-bold.woff2"),
    },
];

/// `@font-face` rules for every embedded face.
///
/// `font-display: block` rather than `swap`: the alternative shows labels in a
/// fallback face for a frame or two, which on a panel sized in 9px silk-screen
/// type reads as a flicker of the wrong instrument.
pub fn face_css() -> String {
    let mut css = String::new();
    for face in FACES {
        let payload = base64::engine::general_purpose::STANDARD.encode(face.bytes);
        css.push_str(&format!(
            "@font-face{{font-family:'{family}';font-style:{style};font-weight:{weight};\
             font-display:block;src:url(data:font/woff2;base64,{payload}) format('woff2');}}\n",
            family = face.family,
            style = face.style,
            weight = face.weight,
            payload = payload,
        ));
    }
    css
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Positive control for the whole embed: a subset that was never built, or
    /// a download that returned an HTML error page, still `include_bytes!`s
    /// happily and then renders as tofu at runtime. Check the container magic.
    #[test]
    fn every_embedded_face_is_a_real_woff2() {
        assert!(!FACES.is_empty());
        for face in FACES {
            assert!(
                face.bytes.len() > 64,
                "{} {} looks like a placeholder ({} bytes)",
                face.family,
                face.style,
                face.bytes.len()
            );
            assert_eq!(
                &face.bytes[..4],
                b"wOF2",
                "{} {} is not a woff2 payload — run tools/fonts/build.sh",
                face.family,
                face.style
            );
        }
    }

    #[test]
    fn face_css_declares_every_family_once_per_weight() {
        let css = face_css();
        for face in FACES {
            assert!(css.contains(&format!("font-family:'{}'", face.family)));
            assert!(css.contains(&format!("font-weight:{}", face.weight)));
        }
        assert_eq!(
            css.matches("@font-face").count(),
            FACES.len(),
            "one rule per face, no more"
        );
        assert!(css.matches("data:font/woff2;base64,").count() == FACES.len());
    }
}
