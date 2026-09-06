// ezlol-ocr: capture the main display with ScreenCaptureKit, run Vision OCR,
// print recognized lines as JSON. Used by the backend on macOS to detect the
// ARAM Mayhem augment selection screen. Needs Screen Recording permission.
import Foundation
import CoreGraphics
import ScreenCaptureKit
import Vision
import AppKit

// Window enumeration needs a CoreGraphics session; initialise AppKit headlessly.
_ = NSApplication.shared

func fail(_ msg: String) -> Never {
    FileHandle.standardError.write((msg + "\n").data(using: .utf8)!)
    exit(2)
}

let sem = DispatchSemaphore(value: 0)
var captured: CGImage?
var captureErr: String?

Task {
    do {
        let content = try await SCShareableContent.excludingDesktopWindows(false, onScreenWindowsOnly: true)
        let cfg = SCStreamConfiguration()
        cfg.showsCursor = false
        // Find the League game window and capture the display it lives on. Capturing
        // the window itself yields a blank frame (Metal), so we take the whole display
        // and rely on the game being fullscreen there.
        let game = content.windows.first { w in
            (w.owningApplication?.bundleIdentifier ?? "").lowercased().contains("gameclient") && w.frame.width > 300
        }
        var display = content.displays.first
        if let w = game {
            let center = CGPoint(x: w.frame.midX, y: w.frame.midY)
            if let d = content.displays.first(where: { $0.frame.contains(center) }) { display = d }
        }
        guard let d = display else { throw NSError(domain: "ezlol", code: 1, userInfo: [NSLocalizedDescriptionKey: "no display"]) }
        let filter = SCContentFilter(display: d, excludingWindows: [])
        let scale = 0.5
        cfg.width = Int(Double(d.width) * scale)
        cfg.height = Int(Double(d.height) * scale)
        captured = try await SCScreenshotManager.captureImage(contentFilter: filter, configuration: cfg)
    } catch {
        captureErr = error.localizedDescription
    }
    sem.signal()
}
sem.wait()
if let e = captureErr { fail("capture failed: \(e) (grant Screen Recording to ezlol)") }
guard let image = captured else { fail("capture failed") }

// Debug: EZLOL_OCR_DUMP=/path.png writes the captured frame.
if let dump = ProcessInfo.processInfo.environment["EZLOL_OCR_DUMP"] {
    let rep = NSBitmapImageRep(cgImage: image)
    if let png = rep.representation(using: .png, properties: [:]) { try? png.write(to: URL(fileURLWithPath: dump)) }
}

var lines: [[String: Any]] = []
let req = VNRecognizeTextRequest { request, _ in
    for obs in (request.results as? [VNRecognizedTextObservation]) ?? [] {
        guard let top = obs.topCandidates(1).first else { continue }
        let b = obs.boundingBox
        lines.append(["text": top.string, "conf": top.confidence, "x": b.midX, "y": 1 - b.midY, "w": b.width])
    }
}
req.recognitionLevel = ProcessInfo.processInfo.environment["EZLOL_OCR_FAST"] != nil ? .fast : .accurate
req.usesLanguageCorrection = false
try? VNImageRequestHandler(cgImage: image, options: [:]).perform([req])

let out: [String: Any] = ["width": image.width, "height": image.height, "lines": lines]
let data = try! JSONSerialization.data(withJSONObject: out)
FileHandle.standardOutput.write(data)
FileHandle.standardOutput.write("\n".data(using: .utf8)!)
