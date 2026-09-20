// ThumbnailProvider.swift — the Quick Look entry point.
//
// This is the entire macOS-side behaviour: Finder asks for a thumbnail,
// we ask the Rust core for pixels, and we draw those pixels into the
// context Quick Look hands us. No archive parsing, no image decoding, no
// cover selection happens here — see `ArcThumbCore.swift` for the FFI
// wrapper and `src/` for the logic both platforms share.
//
// The `@objc` name matters: `Info.plist`'s `NSExtensionPrincipalClass`
// looks this class up by string, and a plain Swift declaration would name
// it `<module>.ThumbnailProvider`, which breaks if the module is renamed.

import Foundation
import QuickLookThumbnailing

@objc(ThumbnailProvider)
final class ThumbnailProvider: QLThumbnailProvider {

    override func provideThumbnail(
        for request: QLFileThumbnailRequest,
        _ reply: @escaping (QLThumbnailReply?, Error?) -> Void
    ) {
        let url = request.fileURL

        // `maximumSize` is in points; the context we are later asked to
        // draw into is `maximumSize × scale` pixels. Render at the pixel
        // size so the cover is never upsampled, and never at more than the
        // caller intended.
        let scale = max(request.scale, 1.0)
        let maxWidth = Self.pixelSide(request.maximumSize.width, scale: scale)
        let maxHeight = Self.pixelSide(request.maximumSize.height, scale: scale)
        guard maxWidth > 0, maxHeight > 0 else {
            NSLog("ArcThumb: ignoring degenerate request \(request.maximumSize)@\(scale) for \(url.lastPathComponent)")
            reply(nil, nil)
            return
        }

        let settings = ArcSettings.current()

        // Quick Look calls us on its own queue, but a cold archive (7z
        // footer + a multi-megapixel cover) can take long enough that the
        // queue backs up behind us. Async keeps the host's requests flowing.
        DispatchQueue.global(qos: .userInitiated).async {
            guard let thumb = ArcThumbCore.thumbnail(
                url: url,
                maxPixelWidth: maxWidth,
                maxPixelHeight: maxHeight,
                settings: settings
            ) else {
                // No thumbnail is the normal answer for a zip without
                // images, an encrypted archive, or a corrupt container.
                // Nil reply = Finder keeps its own icon; never an error.
                reply(nil, nil)
                return
            }

            // Hand back points, not pixels: the system multiplies this by
            // `request.scale` when it creates the bitmap context.
            let contextSize = CGSize(
                width: CGFloat(thumb.pixelSize.width) / scale,
                height: CGFloat(thumb.pixelSize.height) / scale
            )
            let image = thumb.cgImage

            let replyObject = QLThumbnailReply(contextSize: contextSize) { context in
                context.draw(image, in: CGRect(origin: .zero, size: contextSize))
                return true
            }
            reply(replyObject, nil)
        }
    }

    /// Points → whole pixels, floored at zero for a non-finite or negative
    /// request rather than wrapping into a huge `UInt32`.
    static func pixelSide(_ points: CGFloat, scale: CGFloat) -> UInt32 {
        guard points.isFinite, points > 0, scale.isFinite, scale > 0 else { return 0 }
        let pixels = (points * scale).rounded(.down)
        return pixels > 0 && pixels < Double(UInt32.max) ? UInt32(pixels) : 0
    }
}
