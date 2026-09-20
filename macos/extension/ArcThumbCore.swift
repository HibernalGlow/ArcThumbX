// ArcThumbCore.swift — the only file that talks to the Rust core.
//
// Everything platform-specific on the macOS side lives here: reading the
// user's settings from `UserDefaults`, calling the C ABI, and wrapping the
// returned pixels in a `CGImage`. It contains no image logic: archives,
// cover selection, decoding and scaling all happen in Rust
// (`macos/arcthumb-ffi` on top of the shared core), the same code Explorer
// runs on Windows.

import CoreGraphics
import Foundation

/// Owns one `ArcThumbnail` from the core and frees it on deinit.
///
/// The `CGImage` we hand to Quick Look references the buffer directly, so
/// the buffer has to outlive the image. `CGDataProvider` holds a retained
/// handle to this object; CoreGraphics releases it when the provider goes
/// away, and that is the moment the Rust allocation comes back. No pixel
/// copy, no temp file.
private final class ThumbnailOwner {
    let thumbnail: ArcThumbnail
    private let pointer: UnsafeMutablePointer<ArcThumbnail>

    init(_ pointer: UnsafeMutablePointer<ArcThumbnail>, thumbnail: ArcThumbnail) {
        self.pointer = pointer
        self.thumbnail = thumbnail
    }

    deinit {
        arc_thumbnail_free(pointer)
    }
}

/// A thumbnail as the core produced it.
struct ArcThumbImage {
    let cgImage: CGImage
    /// The core's output size, in pixels.
    var pixelSize: CGSize { CGSize(width: cgImage.width, height: cgImage.height) }
}

enum ArcThumbCore {
    /// ABI version this file was written against.
    static let expectedABIVersion = UInt32(ARC_ABI_VERSION)

    static var coreVersion: String {
        guard let c = arc_core_version() else { return "unknown" }
        return String(cString: c)
    }

    /// Ask the core for a thumbnail fitting inside `maxPixelWidth` ×
    /// `maxPixelHeight`.
    ///
    /// Returns nil for every "no thumbnail" outcome — not an archive, no
    /// eligible image, encrypted, corrupt, over a safety limit, undecodable
    /// cover. Those are ordinary results; `lastError` is for the log.
    static func thumbnail(
        url: URL,
        maxPixelWidth: UInt32,
        maxPixelHeight: UInt32,
        settings: ArcSettings
    ) -> ArcThumbImage? {
        let pointer = url.path.withCString { cPath in
            withUnsafePointer(to: settings) { cSettings in
                arc_thumbnail_generate(cPath, maxPixelWidth, maxPixelHeight, cSettings)
            }
        }
        guard let pointer else {
            NSLog("ArcThumb: no thumbnail for \(url.lastPathComponent): \(lastError())")
            return nil
        }

        let thumb = pointer.pointee
        guard validate(thumb, path: url.path) else {
            arc_thumbnail_free(pointer)
            return nil
        }

        let owner = ThumbnailOwner(pointer, thumbnail: thumb)
        guard let image = makeCGImage(owner: owner) else {
            // `owner` has one strong reference left and frees the bitmap as
            // it goes.
            NSLog("ArcThumb: could not wrap \(thumb.width)x\(thumb.height) pixels in a CGImage")
            return nil
        }
        return ArcThumbImage(cgImage: image)
    }

    /// Trust-but-check the geometry the core reported before any of it is
    /// used to size a bitmap.
    private static func validate(_ thumb: ArcThumbnail, path: String) -> Bool {
        guard thumb.struct_size == UInt32(MemoryLayout<ArcThumbnail>.size) else {
            NSLog("ArcThumb: core ABI mismatch for \(path) (struct size \(thumb.struct_size))")
            return false
        }
        guard thumb.format == ARC_PIXEL_FORMAT_RGBA8_PREMULTIPLIED else {
            NSLog("ArcThumb: unexpected pixel format \(thumb.format) for \(path)")
            return false
        }
        guard thumb.data != nil, thumb.width > 0, thumb.height > 0 else {
            NSLog("ArcThumb: empty bitmap for \(path)")
            return false
        }
        // Widened so a corrupt 0xFFFF_FFFF can't wrap into a small bound.
        let rowBytes = UInt64(thumb.width) * 4
        let needed = UInt64(thumb.stride) * UInt64(thumb.height)
        guard UInt64(thumb.stride) >= rowBytes, needed <= UInt64(thumb.data_len) else {
            NSLog("ArcThumb: implausible geometry \(thumb.width)x\(thumb.height) stride \(thumb.stride) for \(path)")
            return false
        }
        return true
    }

    /// Wrap the core's buffer in a `CGImage` whose data provider owns the
    /// buffer's lifetime.
    private static func makeCGImage(owner: ThumbnailOwner) -> CGImage? {
        let thumb = owner.thumbnail
        // The provider gets a +1 handle on the owner; the C callback below
        // balances it with takeRetainedValue, whose deinit frees the Rust
        // allocation. `data` stays valid because nothing else can drop the
        // last reference before that callback runs.
        let info = Unmanaged.passRetained(owner).toOpaque()
        let release: CGDataProviderReleaseDataCallback = { rawInfo, _, _ in
            guard let rawInfo else { return }
            _ = Unmanaged<ThumbnailOwner>.fromOpaque(rawInfo).takeRetainedValue()
        }
        guard let provider = CGDataProvider(
            dataInfo: info,
            data: UnsafeMutableRawPointer(mutating: thumb.data),
            size: Int(thumb.data_len),
            releaseData: release
        ) else {
            _ = Unmanaged<ThumbnailOwner>.fromOpaque(info).takeRetainedValue()
            return nil
        }

        return CGImage(
            width: Int(thumb.width),
            height: Int(thumb.height),
            bitsPerComponent: 8,
            bitsPerPixel: 32,
            bytesPerRow: Int(thumb.stride),
            space: CGColorSpace(name: CGColorSpace.sRGB) ?? CGColorSpaceCreateDeviceRGB(),
            bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedLast.rawValue),
            provider: provider,
            decode: nil,
            shouldInterpolate: true,
            intent: .defaultIntent
        )
    }

    /// The core's message about the most recent failure on this thread.
    static func lastError() -> String {
        var buffer = [CChar](repeating: 0, count: 512)
        let written = buffer.withUnsafeMutableBufferPointer { ptr in
            arc_last_error(ptr.baseAddress, UInt32(ptr.count))
        }
        guard written > 0 else { return "no error recorded" }
        let bytes = buffer.prefix(Int(written)).map { UInt8(bitPattern: $0) }
        return String(decoding: bytes, as: UTF8.self)
    }
}

extension ArcSettings {
    /// Where the config app (`arcthumb-config`, bundled as ArcThumb.app)
    /// writes the user's choices, and where we read them from.
    ///
    /// A plain file rather than `UserDefaults`, because the extension is
    /// sandboxed: an unsandboxed helper cannot write into this process's
    /// preference domain (CFPreferences routes it to
    /// `~/Library/Preferences/…`, which the sandbox never reads), while the
    /// container path below is writable by the user's own apps and readable
    /// here. `key = value` lines, unknown keys ignored, so an older
    /// extension still reads a newer file.
    static let settingsFile = "Library/Application Support/ArcThumb/settings"

    /// The user's settings, re-read on every request: cheap (one small file)
    /// and it means a change in the config app takes effect without waiting
    /// for the extension process to be recycled.
    static func current() -> ArcSettings {
        var settings = arc_settings_default()
        settings.struct_size = UInt32(MemoryLayout<ArcSettings>.size)
        guard let text = readSettingsFile() else { return settings }

        let values = parse(text)
        if let sort = values["sort_order"] {
            settings.sort_order = sort == "alphabetical" ? ARC_SORT_ALPHABETICAL : ARC_SORT_NATURAL
        }
        if let cover = values["cover_mode"] {
            switch cover {
            case "ignore": settings.cover_mode = ARC_COVER_IGNORE
            case "only": settings.cover_mode = ARC_COVER_ONLY
            default: settings.cover_mode = ARC_COVER_PREFER
            }
        }
        if let mask = values["enabled_image_exts"].flatMap(UInt32.init) {
            settings.enabled_image_exts_mask = mask
        }
        if let mask = values["enabled_archive_exts"].flatMap(UInt32.init) {
            settings.enabled_archive_exts_mask = mask
        }
        settings.overlay_border = boolFlag(values["overlay_border"])
        settings.overlay_label = boolFlag(values["overlay_label"])
        // Only ever latch the log *on*: leaving the call out keeps the core's
        // own rules (debug builds, `ARCTHUMB_LOG=1`) working.
        if boolFlag(values["log_enabled"]) != 0
            || ProcessInfo.processInfo.environment["ARCTHUMB_LOG"] != nil {
            arc_log_enabled(1)
        }
        return settings
    }

    private static func readSettingsFile() -> String? {
        let url = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent(settingsFile)
        guard let data = try? Data(contentsOf: url) else { return nil }
        return String(data: data, encoding: .utf8)
    }

    private static func parse(_ text: String) -> [String: String] {
        var out: [String: String] = [:]
        for line in text.split(separator: "\n") {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if trimmed.isEmpty || trimmed.hasPrefix("#") { continue }
            guard let eq = trimmed.firstIndex(of: "=") else { continue }
            let key = trimmed[..<eq].trimmingCharacters(in: .whitespaces)
            let value = trimmed[trimmed.index(after: eq)...].trimmingCharacters(in: .whitespaces)
            out[key] = value
        }
        return out
    }

    private static func boolFlag(_ value: String?) -> UInt32 {
        guard let value else { return 0 }
        return ["1", "true", "yes", "on"].contains(value.lowercased()) ? 1 : 0
    }
}
