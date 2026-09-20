/*
 * arcthumb_ffi.h — C ABI of the ArcThumb thumbnail core.
 *
 * Hand-maintained mirror of macos/arcthumb-ffi/src/lib.rs. Swift imports it
 * through module.modulemap; the Rust side has compile-time layout canaries
 * in its tests, and `test_header_layout_matches` fails if the two drift.
 *
 * Ownership: arc_thumbnail_generate returns a thumbnail the caller owns and
 * must release exactly once with arc_thumbnail_free. CoreGraphics calls that
 * free from the CGDataProvider release callback, so the pixel buffer is
 * never copied and no temporary image file is written.
 */

#ifndef ARCTHUMB_FFI_H
#define ARCTHUMB_FFI_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Bumped on any incompatible change to the structs or functions below. */
#define ARC_ABI_VERSION 2u

/* 8 bits per channel, R,G,B,A byte order, alpha premultiplied. */
#define ARC_PIXEL_FORMAT_RGBA8_PREMULTIPLIED 1u

/* ArcSettings.sort_order */
#define ARC_SORT_ALPHABETICAL 0u
#define ARC_SORT_NATURAL 1u

/* ArcSettings.cover_mode */
#define ARC_COVER_IGNORE 0u
#define ARC_COVER_PREFER 1u
#define ARC_COVER_ONLY 2u

typedef struct ArcSettings {
    /* sizeof(struct ArcSettings), from the caller. */
    uint32_t struct_size;
    /* ARC_SORT_* */
    uint32_t sort_order;
    /* ARC_COVER_* */
    uint32_t cover_mode;
    /* Bitmask over the core's supported image extensions. ~0u enables all. */
    uint32_t enabled_image_exts_mask;
    /* Bitmask over the core's supported archive extensions; a cleared bit
     * means that container type gets no thumbnail at all. macOS stand-in
     * for Windows' per-extension ShellEx registration. */
    uint32_t enabled_archive_exts_mask;
    /* Non-zero draws the format-coloured border. */
    uint32_t overlay_border;
    /* Non-zero draws the "CBZ"/"EPUB"/… corner label. */
    uint32_t overlay_label;
    /* Ignored; see arc_log_enabled. */
    uint32_t log_enabled;
} ArcSettings;

/* Opaque: read through the accessors below. */
/* A rendered thumbnail. Plain data, readable field by field.
 *
 * `data` points into an allocation owned by this struct and stays valid
 * until arc_thumbnail_free(). Never copy it by hand into a buffer you then
 * free yourself. */
typedef struct ArcThumbnail {
    /* sizeof(struct ArcThumbnail), written by the core. */
    uint32_t struct_size;
    uint32_t width;
    uint32_t height;
    /* Bytes per row. */
    uint32_t stride;
    /* ARC_PIXEL_FORMAT_* */
    uint32_t format;
    /* Length of data in bytes. */
    uint32_t data_len;
    const uint8_t *data;
} ArcThumbnail;

uint32_t arc_abi_version(void);

/* NUL-terminated version string in static storage; do not free. */
const char *arc_core_version(void);

/* The core's built-in settings. Seed a local struct from this and override
 * the fields you care about: a zero-initialised ArcSettings means "no image
 * format is eligible", i.e. never a thumbnail. */
ArcSettings arc_settings_default(void);

/* Latches the process-wide diagnostic log on/off. Only the first call
 * has an effect. */
void arc_log_enabled(uint32_t on);

/* Copy the last error message into buf (UTF-8, unterminated). Returns
 * bytes written, 0 if the previous call succeeded. */
uint32_t arc_last_error(char *buf, uint32_t buf_len);

/* Render a thumbnail for the archive at `path`, fitting inside
 * max_width x max_height device pixels. Returns NULL when the file
 * yields no thumbnail - an ordinary outcome, not a fault. The non-NULL
 * result is owned by the caller. */
ArcThumbnail *arc_thumbnail_generate(const char *path,
                                    uint32_t max_width,
                                    uint32_t max_height,
                                    const ArcSettings *settings);

/* Release a thumbnail and its pixel buffer. NULL is a no-op. CoreGraphics
 * calls this exactly once, from the data provider's release callback. */
void arc_thumbnail_free(ArcThumbnail *thumbnail);

#ifdef __cplusplus
}
#endif

#endif /* ARCTHUMB_FFI_H */
