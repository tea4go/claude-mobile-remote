// Run from repository root: swift -module-cache-path /tmp/cmr-icon-module-cache apps/harmony/branding/generate-icons.swift
// Shared polygon source for vector artwork and raster exports. No raster embedded in SVG.
import Foundation
import CoreGraphics
import ImageIO
import UniformTypeIdentifiers

let root = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
let branding = root.appendingPathComponent("apps/harmony/branding")
let shapes: [(String, [[CGFloat]])] = [
    ("#53AEFF", [[93,50],[122.5,79.5],[152,50],[166.5,64.5],[137,94],[166.5,123.5],[152,138],[78.5,64.5]]),
    ("#70C9FF", [[64.5,78.5],[138,152],[123.5,166.5],[94,137],[64.5,166.5],[50,152],[79.5,122.5],[50,93]])
]
let paths = shapes.map { color, points in
    let d = points.enumerated().map { index, p in "\(index == 0 ? "M" : "L")\(p[0]) \(p[1])" }.joined(separator: " ") + " Z"
    return "  <path fill=\"\(color)\" d=\"\(d)\"/>"
}.joined(separator: "\n")
let svg = """
<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 216 216">
  <title>Claude Mobile Remote — Harmony application icon</title>
  <rect width="216" height="216" fill="#FFFFFF"/>
\(paths)
</svg>

"""
try svg.write(to: branding.appendingPathComponent("app-icon.svg"), atomically: true, encoding: .utf8)

func render(size: Int, background: Bool, foreground: Bool) -> CGImage {
    let context = CGContext(data: nil, width: size, height: size, bitsPerComponent: 8,
        bytesPerRow: size * 4, space: CGColorSpaceCreateDeviceRGB(),
        bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
    context.translateBy(x: 0, y: CGFloat(size))
    context.scaleBy(x: CGFloat(size) / 216, y: -CGFloat(size) / 216)
    if background {
        context.setFillColor(CGColor(red: 1, green: 1, blue: 1, alpha: 1))
        context.fill(CGRect(x: 0, y: 0, width: 216, height: 216))
    }
    if foreground {
        for (hex, points) in shapes {
            let value = UInt32(hex.dropFirst(), radix: 16)!
            context.setFillColor(CGColor(red: CGFloat((value >> 16) & 255) / 255,
                green: CGFloat((value >> 8) & 255) / 255, blue: CGFloat(value & 255) / 255, alpha: 1))
            context.beginPath()
            context.move(to: CGPoint(x: points[0][0], y: points[0][1]))
            for p in points.dropFirst() { context.addLine(to: CGPoint(x: p[0], y: p[1])) }
            context.closePath(); context.fillPath()
        }
    }
    return context.makeImage()!
}
func write(_ image: CGImage, _ path: String) {
    let url = root.appendingPathComponent(path)
    let destination = CGImageDestinationCreateWithURL(url as CFURL, UTType.png.identifier as CFString, 1, nil)!
    CGImageDestinationAddImage(destination, image, nil)
    precondition(CGImageDestinationFinalize(destination))
}
for dir in ["apps/harmony/AppScope/resources/base/media", "apps/harmony/entry/src/main/resources/base/media"] {
    write(render(size: 1024, background: true, foreground: false), "\(dir)/background.png")
    write(render(size: 1024, background: false, foreground: true), "\(dir)/foreground.png")
}
write(render(size: 1024, background: true, foreground: true), "apps/harmony/entry/src/main/resources/base/media/startIcon.png")
write(render(size: 216, background: true, foreground: true), "apps/harmony/branding/app-icon-216.png")
print("Generated vector SVG, 216px export, 1024px layered icons and startup icon.")
