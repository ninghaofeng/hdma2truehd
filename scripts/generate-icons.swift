// Run on macOS: swift scripts/generate-icons.swift
// The original SVG is the only artwork source; normal builds use the committed
// PNG/ICO/ICNS assets and do not require Swift or an image-conversion package.
import AppKit
import Foundation

func word(_ value: UInt16) -> Data {
    Data([UInt8(value & 255), UInt8(value >> 8)])
}

func dword(_ value: UInt32) -> Data {
    word(UInt16(truncatingIfNeeded: value)) + word(UInt16(value >> 16))
}

func generate() throws {
    let manager = FileManager.default
    let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent()
    let app = root.appendingPathComponent("cmd/hdma2truehd-ui")
    let svg = app.appendingPathComponent("frontend/icon.svg")
    guard let image = NSImage(contentsOf: svg) else {
        throw NSError(domain: "HDMAIcons", code: 1, userInfo: [NSLocalizedDescriptionKey: "Cannot decode \(svg.path)"])
    }
    image.cacheMode = .never
    let assets = app.appendingPathComponent("assets", isDirectory: true)
    try manager.createDirectory(at: assets, withIntermediateDirectories: true)
    let temporary = manager.temporaryDirectory.appendingPathComponent("hdma-icons-\(UUID().uuidString)", isDirectory: true)
    let iconset = temporary.appendingPathComponent("HDMA.iconset", isDirectory: true)
    try manager.createDirectory(at: iconset, withIntermediateDirectories: true)
    defer {
        do { try manager.removeItem(at: temporary) }
        catch { fputs("Cannot clean up \(temporary.path): \(error)\n", stderr); exit(1) }
    }

    var pngs: [Int: Data] = [:]
    // Render each size from the vector source rather than scaling a cached
    // small bitmap, preserving sharp strokes in Dock and taskbar variants.
    for size in [16, 24, 32, 48, 64, 128, 256, 512, 1024] {
        guard let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size,
                bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0),
              let context = NSGraphicsContext(bitmapImageRep: bitmap) else {
            throw NSError(domain: "HDMAIcons", code: 2, userInfo: [NSLocalizedDescriptionKey: "Cannot allocate \(size)-pixel image"])
        }
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = context
        context.imageInterpolation = .high
        image.draw(in: NSRect(x: 0, y: 0, width: CGFloat(size), height: CGFloat(size)),
                   from: .zero, operation: .copy, fraction: 1)
        NSGraphicsContext.restoreGraphicsState()
        guard let data = bitmap.representation(using: .png, properties: [:]) else {
            throw NSError(domain: "HDMAIcons", code: 3, userInfo: [NSLocalizedDescriptionKey: "Cannot encode PNG"])
        }
        pngs[size] = data
    }
    try pngs[1024]!.write(to: assets.appendingPathComponent("app.png"), options: .atomic)

    // iconutil uses the filename's point size/@2x suffix to distinguish retina
    // representations, even when two entries share the same pixel dimensions.
    for size in [16, 32, 128, 256, 512] {
        try pngs[size]!.write(to: iconset.appendingPathComponent("icon_\(size)x\(size).png"))
        try pngs[size * 2]!.write(to: iconset.appendingPathComponent("icon_\(size)x\(size)@2x.png"))
    }
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/iconutil")
    process.arguments = ["-c", "icns", iconset.path, "-o", assets.appendingPathComponent("app.icns").path]
    try process.run()
    process.waitUntilExit()
    guard process.terminationStatus == 0 else {
        throw NSError(domain: "HDMAIcons", code: 4, userInfo: [NSLocalizedDescriptionKey: "iconutil failed"])
    }

    // Modern Windows ICO entries contain PNG payloads. Include small native
    // sizes as well as 256px so Explorer and the taskbar need not upscale.
    let sizes = [16, 24, 32, 48, 64, 128, 256]
    var ico = Data([0, 0, 1, 0]) + word(UInt16(sizes.count))
    // ICO directory offsets are absolute byte positions. Dimensions use one
    // byte, where zero denotes 256px; word/dword serialize little-endian fields.
    var offset = 6 + sizes.count * 16
    for size in sizes {
        let data = pngs[size]!
        let dimension = size == 256 ? UInt8(0) : UInt8(size)
        ico.append(contentsOf: [dimension, dimension, 0, 0])
        ico.append(word(1))
        ico.append(word(32))
        ico.append(dword(UInt32(data.count)))
        ico.append(dword(UInt32(offset)))
        offset += data.count
    }
    for size in sizes { ico.append(pngs[size]!) }
    try ico.write(to: assets.appendingPathComponent("app.ico"), options: .atomic)
    print("Generated app.png, app.ico and app.icns from frontend/icon.svg")
}

do { try generate() }
catch { fputs("Icon generation failed: \(error)\n", stderr); exit(1) }
