// deskctl: look at and drive the logged-in desktop on this Mac.
// It has to run inside the desktop session; the `desk` wrapper starts it there.
//
//   deskctl status
//   deskctl windows
//   deskctl shot <file.png> [app | pid]
//   deskctl watch <pid> <file> <seconds>
//   deskctl raise <app>
//   deskctl move <x> <y>
//   deskctl click <x> <y>
//   deskctl drag <x1> <y1> <x2> <y2>
//   deskctl key <keycode> [cmd,shift,alt,ctrl]
//   deskctl type <text>
//
// Coordinates are points from the top left of the main display.
import AppKit
import ApplicationServices
import CoreMedia
import CoreGraphics
import Foundation
import ImageIO
import ScreenCaptureKit
import UniformTypeIdentifiers

func fail(_ message: String) -> Never {
    FileHandle.standardError.write((message + "\n").data(using: .utf8)!)
    exit(1)
}

func needAccessibility() {
    if !AXIsProcessTrusted() { fail("no Accessibility permission") }
}

func status() {
    let session = CGSessionCopyCurrentDictionary() as? [String: Any] ?? [:]
    let display = CGMainDisplayID()
    print("user:", session[kCGSessionUserNameKey as String] ?? "none")
    print("on console:", session[kCGSessionOnConsoleKey as String] ?? "none")
    print("screen locked:", session["CGSSessionScreenIsLocked"] ?? "no")
    print("screen recording:", CGPreflightScreenCaptureAccess())
    print("accessibility:", AXIsProcessTrusted())
    print("main display:", CGDisplayPixelsWide(display), "x", CGDisplayPixelsHigh(display))
}

// Normal app windows on screen, front to back. Titles need Screen Recording.
func appWindows() -> [[String: Any]] {
    let all = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID)
    return (all as? [[String: Any]] ?? []).filter { ($0[kCGWindowLayer as String] as? Int) == 0 }
}

func windows() {
    for window in appWindows() {
        let owner = window[kCGWindowOwnerName as String] as? String ?? "?"
        let title = window[kCGWindowName as String] as? String ?? ""
        let bounds = window[kCGWindowBounds as String] as! CFDictionary
        let rect = CGRect(dictionaryRepresentation: bounds) ?? .zero
        print("\(owner)\t\(Int(rect.minX))\t\(Int(rect.minY))\t\(Int(rect.width))\t\(Int(rect.height))\t\(title)")
    }
}

// The whole display, or with `app` only that app's largest window, with whatever
// else is on the desktop left out even if it overlaps. A number for `app` is a process
// id, which picks one of several copies of an app that are open.
func shot(_ path: String, app: String?) {
    let done = DispatchSemaphore(value: 0)
    Task {
        do {
            let content = try await SCShareableContent.current
            let config = SCStreamConfiguration()
            config.showsCursor = app == nil
            let filter: SCContentFilter
            if let app {
                let pid = pid_t(app)
                let matches = content.windows.filter {
                    (pid == nil ? $0.owningApplication?.applicationName == app : $0.owningApplication?.processID == pid)
                        && $0.isOnScreen && $0.windowLayer == 0
                }
                guard let window = matches.max(by: { $0.frame.width * $0.frame.height < $1.frame.width * $1.frame.height })
                else { fail("no window on screen for \(app)") }
                filter = SCContentFilter(desktopIndependentWindow: window)
                config.width = Int(window.frame.width * CGFloat(filter.pointPixelScale))
                config.height = Int(window.frame.height * CGFloat(filter.pointPixelScale))
            } else {
                guard let display = content.displays.first else { fail("no display") }
                filter = SCContentFilter(display: display, excludingWindows: [])
                config.width = display.width
                config.height = display.height
            }
            let image = try await SCScreenshotManager.captureImage(contentFilter: filter, configuration: config)
            let url = URL(fileURLWithPath: path) as CFURL
            guard let file = CGImageDestinationCreateWithURL(url, UTType.png.identifier as CFString, 1, nil)
            else { fail("can't write \(path)") }
            CGImageDestinationAddImage(file, image, nil)
            if !CGImageDestinationFinalize(file) { fail("can't write \(path)") }
            print("saved", path, image.width, "x", image.height)
        } catch {
            fail("capture failed: \(error)")
        }
        done.signal()
    }
    done.wait()
}

// Keeps the latest frame of a process's largest window in `path`, for a reader on the same Mac
// that needs frames faster than one `shot` per launch. The file is a 40-byte header, then the
// pixels as BGRA rows. The header: "DESKWAT1", a UInt64 frame count that is odd while a frame
// is being written, width, height and bytes per row as UInt32, 4 bytes unused, and the frame's
// capture time as seconds since 1970 in a Double. ScreenCaptureKit sends a frame only when the window changed. It stops after
// `seconds`, or when the window is gone.
final class Watcher: NSObject, SCStreamOutput, SCStreamDelegate {
    let path: String
    var map: UnsafeMutableRawPointer?
    var size = 0
    var count: UInt64 = 0

    init(path: String) { self.path = path }

    func open(_ bytes: Int) {
        if let map { munmap(map, size) }
        let fd = Darwin.open(path, O_RDWR | O_CREAT, 0o600)
        guard fd >= 0, ftruncate(fd, off_t(bytes)) == 0 else { fail("can't write \(path)") }
        guard let m = mmap(nil, bytes, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0), m != MAP_FAILED
        else { fail("can't map \(path)") }
        close(fd)
        map = m
        size = bytes
        memcpy(m, "DESKWAT1", 8)
    }

    func stream(_ stream: SCStream, didOutputSampleBuffer buffer: CMSampleBuffer, of type: SCStreamOutputType) {
        guard type == .screen, buffer.isValid,
              let info = (CMSampleBufferGetSampleAttachmentsArray(buffer, createIfNecessary: false) as? [[SCStreamFrameInfo: Any]])?.first,
              let raw = info[.status] as? Int, SCFrameStatus(rawValue: raw) == .complete,
              let pixels = CMSampleBufferGetImageBuffer(buffer) else { return }
        CVPixelBufferLockBaseAddress(pixels, .readOnly)
        defer { CVPixelBufferUnlockBaseAddress(pixels, .readOnly) }
        let width = CVPixelBufferGetWidth(pixels), height = CVPixelBufferGetHeight(pixels)
        let stride = CVPixelBufferGetBytesPerRow(pixels)
        if size != 40 + stride * height { open(40 + stride * height) }
        guard let m = map, let base = CVPixelBufferGetBaseAddress(pixels) else { return }
        let header = m.advanced(by: 8)
        count += 1
        header.storeBytes(of: count, as: UInt64.self)  // odd: writing
        OSMemoryBarrier()
        memcpy(m.advanced(by: 40), base, stride * height)
        header.advanced(by: 8).storeBytes(of: UInt32(width), as: UInt32.self)
        header.advanced(by: 12).storeBytes(of: UInt32(height), as: UInt32.self)
        header.advanced(by: 16).storeBytes(of: UInt32(stride), as: UInt32.self)
        m.advanced(by: 32).storeBytes(of: Date().timeIntervalSince1970, as: Double.self)
        OSMemoryBarrier()
        count += 1
        header.storeBytes(of: count, as: UInt64.self)  // even: whole
    }

    func stream(_ stream: SCStream, didStopWithError error: Error) { fail("the capture stopped: \(error)") }
}

func watch(pid: pid_t, path: String, seconds: Double) {
    let watcher = Watcher(path: path)
    Task {
        do {
            let content = try await SCShareableContent.current
            let matches = content.windows.filter {
                $0.owningApplication?.processID == pid && $0.isOnScreen && $0.windowLayer == 0
            }
            guard let window = matches.max(by: { $0.frame.width * $0.frame.height < $1.frame.width * $1.frame.height })
            else { fail("no window on screen for pid \(pid)") }
            let filter = SCContentFilter(desktopIndependentWindow: window)
            let config = SCStreamConfiguration()
            config.showsCursor = false
            config.width = Int(window.frame.width * CGFloat(filter.pointPixelScale))
            config.height = Int(window.frame.height * CGFloat(filter.pointPixelScale))
            config.pixelFormat = kCVPixelFormatType_32BGRA
            config.minimumFrameInterval = CMTime(value: 1, timescale: 60)
            config.queueDepth = 4
            let stream = SCStream(filter: filter, configuration: config, delegate: watcher)
            try stream.addStreamOutput(watcher, type: .screen, sampleHandlerQueue: DispatchQueue(label: "frames"))
            try await stream.startCapture()
            print("watching", window.frame.width, "x", window.frame.height)
            objc_setAssociatedObject(watcher, "stream", stream, .OBJC_ASSOCIATION_RETAIN)
        } catch {
            fail("capture failed: \(error)")
        }
    }
    DispatchQueue.global().asyncAfter(deadline: .now() + seconds) { exit(0) }
    DispatchQueue.main.async {
        Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { _ in
            if kill(pid, 0) != 0 { exit(0) }
        }
    }
    RunLoop.main.run()
}

// Input lands on whatever is on top at that point, so raise the target first.
func raise(_ app: String) {
    let match = appWindows().first { ($0[kCGWindowOwnerName as String] as? String) == app }
    guard let pid = match?[kCGWindowOwnerPID as String] as? pid_t else { fail("no window on screen for \(app)") }
    let element = AXUIElementCreateApplication(pid)
    let result = AXUIElementSetAttributeValue(element, kAXFrontmostAttribute as CFString, kCFBooleanTrue)
    if result != .success { fail("can't raise \(app): \(result.rawValue)") }
    var value: CFTypeRef?
    if AXUIElementCopyAttributeValue(element, kAXMainWindowAttribute as CFString, &value) == .success, let value {
        AXUIElementPerformAction(value as! AXUIElement, kAXRaiseAction as CFString)
    }
}

func mouse(_ type: CGEventType, _ x: Double, _ y: Double) {
    CGEvent(mouseEventSource: nil, mouseType: type, mouseCursorPosition: CGPoint(x: x, y: y), mouseButton: .left)?
        .post(tap: .cghidEventTap)
    usleep(30_000)
}

func click(_ x: Double, _ y: Double) {
    mouse(.mouseMoved, x, y)
    mouse(.leftMouseDown, x, y)
    mouse(.leftMouseUp, x, y)
}

func drag(_ x1: Double, _ y1: Double, _ x2: Double, _ y2: Double) {
    mouse(.mouseMoved, x1, y1)
    mouse(.leftMouseDown, x1, y1)
    let steps = 20
    for step in 1...steps {
        let part = Double(step) / Double(steps)
        mouse(.leftMouseDragged, x1 + (x2 - x1) * part, y1 + (y2 - y1) * part)
    }
    mouse(.leftMouseUp, x2, y2)
}

// A trackpad scroll at a point: the gesture phases a real two-finger scroll carries, then a
// short momentum tail. Synthetic wheel events without phases don't drive macOS's elastic
// overscroll; these do. dy is pixels per step, positive to scroll the content up (a swipe
// down), as on the wheel itself.
func scroll(_ x: Double, _ y: Double, _ dy: Double, _ steps: Int, holdMs: Int) {
    mouse(.mouseMoved, x, y)
    func post(_ delta: Double, phase: Int64, momentum: Int64) {
        let event = CGEvent(scrollWheelEvent2Source: nil, units: .pixel, wheelCount: 1,
                            wheel1: Int32(delta), wheel2: 0, wheel3: 0)
        event?.location = CGPoint(x: x, y: y)
        event?.setIntegerValueField(.scrollWheelEventIsContinuous, value: 1)
        event?.setIntegerValueField(.scrollWheelEventScrollPhase, value: phase)
        event?.setIntegerValueField(.scrollWheelEventMomentumPhase, value: momentum)
        event?.post(tap: .cghidEventTap)
        usleep(16_000)
    }
    post(0, phase: 1, momentum: 0)                          // began
    for _ in 0..<steps { post(dy, phase: 2, momentum: 0) }  // changed
    usleep(UInt32(holdMs) * 1000)  // fingers still down: a rubber band is at full stretch
    post(0, phase: 4, momentum: 0)                          // ended
    post(dy / 2, phase: 0, momentum: 1)                     // momentum began
    for step in 1...6 { post(dy / Double(2 + step), phase: 0, momentum: 2) }
    post(0, phase: 0, momentum: 3)                          // momentum ended
}

func key(_ code: CGKeyCode, _ modifiers: String) {
    var flags = CGEventFlags()
    for name in modifiers.split(separator: ",") {
        switch name {
        case "cmd": flags.insert(.maskCommand)
        case "shift": flags.insert(.maskShift)
        case "alt": flags.insert(.maskAlternate)
        case "ctrl": flags.insert(.maskControl)
        default: fail("unknown modifier \(name)")
        }
    }
    for down in [true, false] {
        let event = CGEvent(keyboardEventSource: nil, virtualKey: code, keyDown: down)
        event?.flags = flags
        event?.post(tap: .cghidEventTap)
        usleep(30_000)
    }
}

func type(_ text: String) {
    for unit in text.utf16 {
        var character = unit
        for down in [true, false] {
            let event = CGEvent(keyboardEventSource: nil, virtualKey: 0, keyDown: down)
            event?.keyboardSetUnicodeString(stringLength: 1, unicodeString: &character)
            event?.post(tap: .cghidEventTap)
            usleep(10_000)
        }
    }
}

let args = Array(CommandLine.arguments.dropFirst())

// A capture that can't get through must not hang the caller forever. A watch ends itself.
if args.first != "watch" {
    DispatchQueue.global().asyncAfter(deadline: .now() + 30) { fail("timed out") }
}

// Connects to the window server; capturing a single window crashes without this.
_ = NSApplication.shared

func number(_ index: Int) -> Double {
    guard index < args.count, let value = Double(args[index]) else { fail("expected a number") }
    return value
}

switch args.first {
case "status":
    status()
case "windows":
    windows()
case "shot":
    guard args.count == 2 || args.count == 3 else { fail("shot <file.png> [app | pid]") }
    shot(args[1], app: args.count == 3 ? args[2] : nil)
case "watch":
    guard args.count == 4, let pid = pid_t(args[1]) else { fail("watch <pid> <file> <seconds>") }
    watch(pid: pid, path: args[2], seconds: number(3))
case "raise":
    needAccessibility()
    guard args.count == 2 else { fail("raise <app>") }
    raise(args[1])
case "move":
    needAccessibility()
    mouse(.mouseMoved, number(1), number(2))
case "click":
    needAccessibility()
    click(number(1), number(2))
case "drag":
    needAccessibility()
    drag(number(1), number(2), number(3), number(4))
case "scroll":
    needAccessibility()
    guard (4...6).contains(args.count) else { fail("scroll x y dy [steps] [holdMs]") }
    scroll(number(1), number(2), number(3), args.count >= 5 ? Int(number(4)) : 10, holdMs: args.count == 6 ? Int(number(5)) : 0)
case "key":
    needAccessibility()
    key(CGKeyCode(number(1)), args.count > 2 ? args[2] : "")
case "type":
    needAccessibility()
    guard args.count == 2 else { fail("type <text>") }
    type(args[1])
default:
    fail("status | windows | shot <file> [app | pid] | watch pid file seconds | raise <app> | move x y | click x y | drag x1 y1 x2 y2 | scroll x y dy [steps] [holdMs] | key code [mods] | type text")
}
