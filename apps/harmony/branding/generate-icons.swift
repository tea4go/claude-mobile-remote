// Run from repo root after: qlmanage -t -s 1024 -o /private/tmp ccBuddy.svg
// Then: swift -module-cache-path /private/tmp/cmr-icon-module-cache apps/harmony/branding/generate-icons.swift
import AppKit
import Foundation
import ImageIO
import UniformTypeIdentifiers
let root = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
let source = NSImage(contentsOfFile: "/private/tmp/ccBuddy.svg.png")!
func write(_ path: String, _ size: Int, _ inset: CGFloat = 0) {
 let image = NSImage(size: NSSize(width: size, height: size))
 image.lockFocus()
 NSColor.white.setFill(); NSRect(x: 0, y: 0, width: size, height: size).fill()
 let d = CGFloat(size), margin = d * inset
 source.draw(in: NSRect(x: margin, y: margin, width: d - 2 * margin, height: d - 2 * margin), from: .zero, operation: .copy, fraction: 1)
 image.unlockFocus()
 let rep = NSBitmapImageRep(data: image.tiffRepresentation!)!
 try! rep.representation(using: .png, properties: [:])!.write(to: root.appendingPathComponent(path))
}
write("apps/mobile/ios/App/App/Assets.xcassets/AppIcon.appiconset/AppIcon-512@2x.png", 1024)
for (density, size) in [("mdpi",48),("hdpi",72),("xhdpi",96),("xxhdpi",144),("xxxhdpi",192)] {
 for name in ["ic_launcher", "ic_launcher_round"] { write("apps/mobile/android/app/src/main/res/mipmap-\(density)/\(name).png", size) }
 write("apps/mobile/android/app/src/main/res/mipmap-\(density)/ic_launcher_foreground.png", size * 9 / 4, 0.19)
}
for dir in ["apps/harmony/AppScope/resources/base/media", "apps/harmony/entry/src/main/resources/base/media"] {
 // Harmony applies its own icon mask; use the full-bleed square artwork.
 let square = root.appendingPathComponent("apps/harmony/branding/ccBuddy-square-1024.png")
 let data = try! Data(contentsOf: square)
 try! data.write(to: root.appendingPathComponent("\(dir)/foreground.png"))
 try! data.write(to: root.appendingPathComponent("\(dir)/background.png"))
}
write("apps/harmony/entry/src/main/resources/base/media/startIcon.png", 1024)
write("apps/harmony/branding/app-icon-216.png", 216)

// App Store icons must be opaque.
let iconURL = root.appendingPathComponent("apps/mobile/ios/App/App/Assets.xcassets/AppIcon.appiconset/AppIcon-512@2x.png")
let input = CGImageSourceCreateWithURL(iconURL as CFURL, nil)!
let icon = CGImageSourceCreateImageAtIndex(input, 0, nil)!
let ctx = CGContext(data: nil, width: icon.width, height: icon.height, bitsPerComponent: 8, bytesPerRow: icon.width * 4, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue)!
ctx.setFillColor(NSColor.white.cgColor); ctx.fill(CGRect(x: 0, y: 0, width: icon.width, height: icon.height)); ctx.draw(icon, in: CGRect(x: 0, y: 0, width: icon.width, height: icon.height))
let output = CGImageDestinationCreateWithURL(iconURL as CFURL, UTType.png.identifier as CFString, 1, nil)!
CGImageDestinationAddImage(output, ctx.makeImage()!, nil); precondition(CGImageDestinationFinalize(output))
