//! Pixel-format helpers shared by the platform backends.
//!
//! The core renders thumbnails as straight (non-premultiplied) RGBA8 —
//! that is what `image` produces. Both host APIs want *premultiplied*
//! pixels (Windows `WTSAT_ARGB`, macOS
//! `kCGImageAlphaPremultipliedLast`), so each backend converts on the
//! way out. The conversion itself is platform-independent, hence here.

/// Integer premultiply: `(c * a + 127) / 255`, rounded.
#[inline]
pub fn premul(c: u8, a: u8) -> u8 {
    ((c as u16 * a as u16 + 127) / 255) as u8
}

/// Convert an RGBA8 buffer of straight (non-premultiplied) pixels to
/// premultiplied alpha, in place.
///
/// `buf` must be a multiple of 4 bytes long; any trailing bytes are
/// left untouched.
pub fn premultiply_rgba8(buf: &mut [u8]) {
    let (pixels, _trailing_bytes) = buf.as_chunks_mut::<4>();
    for px in pixels {
        let (r, g, b, a) = (px[0], px[1], px[2], px[3]);
        px[0] = premul(r, a);
        px[1] = premul(g, a);
        px[2] = premul(b, a);
        // Alpha is unchanged; keep whatever the decoder produced.
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn premul_fully_opaque_is_identity() {
        for c in [0u8, 1, 64, 127, 128, 200, 254, 255] {
            assert_eq!(premul(c, 255), c, "c={c}");
        }
    }

    #[test]
    fn premul_fully_transparent_is_zero() {
        for c in [0u8, 1, 64, 128, 255] {
            assert_eq!(premul(c, 0), 0, "c={c}");
        }
    }

    #[test]
    fn premul_half_alpha() {
        // (255 * 128 + 127) / 255 = 32767 / 255 = 128 (rounded).
        assert_eq!(premul(255, 128), 128);
        // (200 * 128 + 127) / 255 = 25727 / 255 = 100.
        assert_eq!(premul(200, 128), 100);
    }

    #[test]
    fn premul_never_overflows_u8() {
        // Exhaustive over the entire 8-bit × 8-bit space — cheap and
        // proves the (c*a+127)/255 expression stays in u8 range.
        for c in 0u16..=255 {
            for a in 0u16..=255 {
                let p = premul(c as u8, a as u8);
                // Result must never exceed the un-premultiplied colour.
                assert!(p as u16 <= c, "c={c} a={a} p={p}");
            }
        }
    }

    #[test]
    fn premultiply_rgba8_touches_only_colour_channels() {
        let mut buf = vec![
            200u8, 100, 50, 128, // half-transparent pixel
            10, 20, 30, 255, //   opaque pixel
        ];
        premultiply_rgba8(&mut buf);
        assert_eq!(buf[3], 128, "alpha preserved");
        assert_eq!(buf[0], 100);
        assert_eq!(buf[1], 50);
        assert_eq!(buf[2], 25);
        // Opaque pixel must be a perfect identity transform.
        assert_eq!(&buf[4..8], &[10, 20, 30, 255]);
    }

    #[test]
    fn premultiply_rgba8_ignores_trailing_bytes() {
        let mut buf = vec![1u8, 2, 3];
        premultiply_rgba8(&mut buf);
        assert_eq!(buf, vec![1, 2, 3]);
    }
}
