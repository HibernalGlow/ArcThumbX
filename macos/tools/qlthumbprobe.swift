// qlthumbprobe — ask the *real* Quick Look thumbnail service for a file's
// thumbnail and write what comes back, exactly the way Finder does.
//
// This is the headless equivalent of "open the folder in Finder and look":
// QLThumbnailGenerator is the client API Finder uses, so a thumbnail here
// means the extension was discovered, matched by UTI, launched, and returned
// pixels. It also proves the sandbox granted us the file.
//
//   qlthumbprobe <out.png> <file> [<file> …]
//
// Exit code is the number of files that produced no thumbnail.

import AppKit
import Foundation
import QuickLookThumbnailing

let args = CommandLine.arguments
guard args.count >= 3 else {
    FileHandle.standardError.write("usage: qlthumbprobe <out.png> <file> …\n".data(using: .utf8)!)
    exit(2)
}
let out = URL(fileURLWithPath: args[1])
let files = Array(args[2...])
let scale = NSScreen.main?.backingScaleFactor ?? 2.0

func probe(_ url: URL) -> QLThumbnailRepresentation? {
    let request = QLThumbnailGenerator.Request(
        fileAt: url, size: CGSize(width: 256, height: 256), scale: scale,
        representationTypes: .thumbnail)
    let sem = DispatchSemaphore(value: 0)
    var result: QLThumbnailRepresentation?
    QLThumbnailGenerator.shared.generateBestRepresentation(for: request) { rep, error in
        if let error {
            FileHandle.standardError.write("  \(url.lastPathComponent): \(error.localizedDescription)\n".data(using: .utf8)!)
        }
        result = rep
        sem.signal()
    }
    _ = sem.wait(timeout: .now() + 30)
    return result
}

/// Mean colour and distinct-colour count, so a "hit" can be told apart
/// from a blank or placeholder bitmap.
func pixelStats(_ image: CGImage) -> (mean: String, distinct: Int) {
    let w = image.width, h = image.height
    var bytes = [UInt8](repeating: 0, count: w * h * 4)
    let space = CGColorSpace(name: CGColorSpace.sRGB)!
    guard let ctx = CGContext(data: &bytes, width: w, height: h, bitsPerComponent: 8,
                              bytesPerRow: w * 4, space: space,
                              bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue) else {
        return ("?", 0)
    }
    ctx.draw(image, in: CGRect(x: 0, y: 0, width: w, height: h))
    var sum = (0, 0, 0)
    var colors = Set<UInt32>()
    for i in stride(from: 0, to: bytes.count, by: 4) {
        sum.0 += Int(bytes[i]); sum.1 += Int(bytes[i + 1]); sum.2 += Int(bytes[i + 2])
        colors.insert(UInt32(bytes[i]) << 16 | UInt32(bytes[i + 1]) << 8 | UInt32(bytes[i + 2]))
    }
    let n = max(w * h, 1)
    return ("\(sum.0 / n),\(sum.1 / n),\(sum.2 / n)", colors.count)
}

var failures = 0
for file in files {
    let url = URL(fileURLWithPath: file)
    guard let rep = probe(url) else {
        print("MISS  \(url.lastPathComponent)")
        failures += 1
        continue
    }
    let cg = rep.cgImage
    let stats = pixelStats(cg)
    print("HIT   \(url.lastPathComponent) \(cg.width)x\(cg.height) mean=\(stats.mean) distinct=\(stats.distinct) type=\(rep.type.rawValue)")
    if file == files.first {
        let repPNG = NSBitmapImageRep(cgImage: cg)
        if let data = repPNG.representation(using: .png, properties: [:]) {
            try? data.write(to: out)
        }
    }
}
exit(Int32(min(failures, 125)))
