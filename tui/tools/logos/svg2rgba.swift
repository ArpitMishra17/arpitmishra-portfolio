// Renders an SVG onto a transparent canvas with AppKit and writes its raw
// RGBA bytes, for trace.mjs. Prints the pixel size.
//
//   swiftc -O svg2rgba.swift -o svg2rgba && ./svg2rgba logo.svg logo.rgba 512
import AppKit

let a = CommandLine.arguments
guard a.count == 4, let img = NSImage(contentsOf: URL(fileURLWithPath: a[1])), let size = Int(a[3]) else {
	FileHandle.standardError.write("usage: svg2rgba in.svg out.rgba size\n".data(using: .utf8)!)
	exit(1)
}
let scale = CGFloat(size) / max(img.size.width, img.size.height)
let w = Int(img.size.width * scale), h = Int(img.size.height * scale)
let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: w, pixelsHigh: h, bitsPerSample: 8,
                           samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB,
                           bytesPerRow: w * 4, bitsPerPixel: 32)!
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
img.draw(in: NSRect(x: 0, y: 0, width: w, height: h))
NSGraphicsContext.restoreGraphicsState()
// AppKit keeps colour premultiplied by alpha; undo it so edges keep their hue
let p = rep.bitmapData!
for i in stride(from: 0, to: w * h * 4, by: 4) where p[i + 3] > 0 && p[i + 3] < 255 {
	let al = Double(p[i + 3]) / 255
	for c in 0..<3 { p[i + c] = UInt8(min(255, Double(p[i + c]) / al)) }
}
try! Data(bytes: p, count: w * h * 4).write(to: URL(fileURLWithPath: a[2]))
print(w, h)
