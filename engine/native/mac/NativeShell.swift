// The spike's native shell: AppKit views in place of the shell's web page. It speaks
// the same messages on channel `shell` (docs/shells.md): Go's JSON arrives through
// receive(_:), and what the user does leaves through the send block the engine gives it,
// which fires PB_EVENT_SHELL_MESSAGE exactly as chrome.send('shell') does.
//
// It draws a tab strip (the title bar, which drags), a toolbar under it with Back,
// Forward, Reload, the Address field, New Tab and Settings, and leaves the rest to the
// active tab's page. It measures its own views and sends `layout`, as the SDK does.

import AppKit

@objc(PBNativeShellView)
public final class NativeShellView: NSView, NSTextFieldDelegate {
  private let send: (String) -> Void

  // Go's state, as the snapshot and the messages after it leave it.
  private var tabs: [[String: Any]] = []
  private var active: Int64 = 0
  private var controls = NSRect.zero  // The window buttons, shell coordinates.
  private var snapshotted = false
  private var shown = false

  private let strip = NSView()
  private let toolbar = NSView()
  private let tabRow = NSStackView()
  private let back = NativeShellView.button("chevron.left", "Back")
  private let forward = NativeShellView.button("chevron.right", "Forward")
  private let reload = NativeShellView.button("arrow.clockwise", "Reload")
  private let newTab = NativeShellView.button("plus", "New Tab")
  private let settings = NativeShellView.button("gearshape", "Settings")
  private let address = AddressField()
  private var tabButtons: [Int64: NSButton] = [:]
  private var lastLayout = ""

  private static let stripH: CGFloat = 38
  private static let toolbarH: CGFloat = 38

  @objc public init(send: @escaping (String) -> Void) {
    self.send = send
    super.init(frame: .zero)
    wantsLayer = true
    for bar in [strip, toolbar] {
      bar.wantsLayer = true
      addSubview(bar)
    }
    toolbar.layer?.backgroundColor = NSColor.windowBackgroundColor.cgColor
    strip.layer?.backgroundColor = NSColor.underPageBackgroundColor.cgColor

    tabRow.orientation = .horizontal
    tabRow.spacing = 2
    strip.addSubview(tabRow)
    strip.addSubview(newTab)

    address.placeholderString = "Search or enter address"
    address.setAccessibilityLabel("Address")
    address.delegate = self
    address.target = self
    address.action = #selector(navigate)
    address.lineBreakMode = .byTruncatingTail
    address.cell?.sendsActionOnEndEditing = false
    for v in [back, forward, reload, address, settings] as [NSView] {
      toolbar.addSubview(v)
    }
    back.target = self; back.action = #selector(goBack)
    forward.target = self; forward.action = #selector(goForward)
    reload.target = self; reload.action = #selector(doReload)
    newTab.target = self; newTab.action = #selector(openTab)
    settings.target = self; settings.action = #selector(openSettings)
  }

  required init?(coder: NSCoder) { fatalError() }

  // Shell coordinates measure y down from the top, as the layout's rectangles do.
  public override var isFlipped: Bool { true }

  // The view covers the window; it is asked only for the points the window manager says
  // are the shell's, so a press anywhere else never reaches it.
  public override func viewDidMoveToWindow() {
    super.viewDidMoveToWindow()
    if window != nil && !snapshotted {
      post(["t": "ready"])
    }
  }

  // MARK: Go to the shell

  @objc public func receive(_ json: String) {
    guard let data = json.data(using: .utf8),
          let msg = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
          let t = msg["t"] as? String else { return }
    switch t {
    case "snapshot":
      take(msg)
      snapshotted = true
      draw()
      if !shown {
        shown = true
        post(["t": "shown"])
      }
    case "tabs", "controls":
      take(msg)
      draw()
    case "tab":
      if let tab = msg["tab"] as? [String: Any], let id = int(tab["id"]),
         let i = tabs.firstIndex(where: { int($0["id"]) == id }) {
        tabs[i] = tab
        draw()
      }
    case "focusAddressBar":
      window?.makeFirstResponder(address)
    default:
      break
    }
  }

  // The keyboard for the shell, as focus_shell asks: the address field, its one field.
  @objc public func focusShell() {
    window?.makeFirstResponder(address)
  }

  private func take(_ msg: [String: Any]) {
    if let t = msg["tabs"] as? [[String: Any]] { tabs = t }
    if let a = int(msg["active"]) { active = a }
    if let c = msg["controls"] as? [String: Any] {
      controls = NSRect(x: dbl(c["x"]), y: dbl(c["y"]), width: dbl(c["w"]), height: dbl(c["h"]))
    }
  }

  // MARK: Drawing

  private func draw() {
    var seen = Set<Int64>()
    for (i, tab) in tabs.enumerated() {
      guard let id = int(tab["id"]) else { continue }
      seen.insert(id)
      let b = tabButtons[id] ?? makeTab(id)
      let title = label(tab)
      b.title = title
        b.state = id == active ? .on : .off
      if tabRow.arrangedSubviews.firstIndex(of: b) != i {
        tabRow.removeArrangedSubview(b)
        tabRow.insertArrangedSubview(b, at: min(i, tabRow.arrangedSubviews.count))
      }
    }
    for (id, b) in tabButtons where !seen.contains(id) {
      tabRow.removeArrangedSubview(b)
      b.removeFromSuperview()
      tabButtons[id] = nil
    }
    let a = tabs.first { int($0["id"]) == active }
    back.isEnabled = (a?["canGoBack"] as? Bool) ?? false
    forward.isEnabled = (a?["canGoForward"] as? Bool) ?? false
    if window?.firstResponder != address.currentEditor() {
      address.stringValue = (a?["url"] as? String) ?? ""
    }
    // Laid out and reported now, not in AppKit's next display pass: a message from Go
    // that changes nothing on screen yet (a new tab from Cmd+T) starts no display pass,
    // and the window would go on showing the old tab's page.
    needsLayout = true
    layoutSubtreeIfNeeded()
  }

  private func makeTab(_ id: Int64) -> NSButton {
    let b = TabButton(title: "", target: self, action: #selector(pickTab(_:)))
    b.setButtonType(.pushOnPushOff)
    b.bezelStyle = .accessoryBarAction
    b.tag = Int(id)
    b.lineBreakMode = .byTruncatingTail
    b.widthAnchor.constraint(lessThanOrEqualToConstant: 180).isActive = true
    // A tab is a tab to a screen reader, as role="tab" is to the web shells' check, and
    // carries its id as data-tab does.
    b.setAccessibilityIdentifier("tab-\(id)")
    tabButtons[id] = b
    tabRow.addArrangedSubview(b)
    return b
  }

  public override func layout() {
    super.layout()
    let w = bounds.width
    let lead = max(controls.maxX + 8, 8)
    strip.frame = NSRect(x: 0, y: 0, width: w, height: Self.stripH)
    toolbar.frame = NSRect(x: 0, y: Self.stripH, width: w, height: Self.toolbarH)
    let rowW = min(tabRow.fittingSize.width, w - lead - 40)
    tabRow.frame = NSRect(x: lead, y: 6, width: max(rowW, 0), height: 26)
    newTab.frame = NSRect(x: lead + rowW + 4, y: 6, width: 28, height: 26)
    var x: CGFloat = 8
    for b in [back, forward, reload] {
      b.frame = NSRect(x: x, y: 6, width: 28, height: 26)
      x += 30
    }
    settings.frame = NSRect(x: w - 36, y: 6, width: 28, height: 26)
    address.frame = NSRect(x: x + 4, y: 8, width: max(w - x - 48, 40), height: 22)
    report()
  }

  // MARK: The layout, as the SDK sends it

  private func report() {
    guard window != nil, snapshotted else { return }
    let w = bounds.width, h = bounds.height
    let top = Self.stripH + Self.toolbarH
    let stripControls = tabRow.arrangedSubviews.map { $0.convert($0.bounds, to: self) } + [newTab.frame]
    var tabsShown: [[String: Any]] = []
    if active != 0 {
      tabsShown.append(["tab": active, "rect": rect(NSRect(x: 0, y: top, width: w, height: h - top))])
    }
    let msg: [String: Any] = [
      "t": "layout",
      "shell": [rect(toolbar.frame)],
      "controls": stripControls.map(rect),
      "drag": [rect(strip.frame)],
      "tabs": tabsShown,
      "viewW": w, "viewH": h, "frame": 0,
    ]
    guard let data = try? JSONSerialization.data(withJSONObject: msg, options: [.sortedKeys]),
          let json = String(data: data, encoding: .utf8), json != lastLayout else { return }
    lastLayout = json
    send(json)
  }

  // MARK: The shell to Go

  @objc private func pickTab(_ b: NSButton) { post(["t": "activateTab", "id": b.tag]) ; draw() }
  @objc private func openTab() { post(["t": "newTab"]) }
  @objc private func goBack() { post(["t": "back"]) }
  @objc private func goForward() { post(["t": "forward"]) }
  @objc private func doReload() { post(["t": "reload"]) }
  @objc private func openSettings() { post(["t": "run", "text": "app.settings"]) }
  @objc private func navigate() {
    let text = address.stringValue.trimmingCharacters(in: .whitespaces)
    if !text.isEmpty {
      post(["t": "navigate", "text": text])
      if active != 0 { post(["t": "focusTab", "id": active]) }
    }
  }

  public func controlTextDidBeginEditing(_ obj: Notification) {
    post(["t": "focusShell"])
  }

  private func post(_ msg: [String: Any]) {
    if let data = try? JSONSerialization.data(withJSONObject: msg),
       let json = String(data: data, encoding: .utf8) {
      send(json)
    }
  }

  // MARK: Helpers

  private static func button(_ symbol: String, _ name: String) -> NSButton {
    let glyphs = ["chevron.left": "‹", "chevron.right": "›", "arrow.clockwise": "↻",
                  "plus": "+", "gearshape": "⚙"]
    let b = NamedButton(title: glyphs[symbol] ?? name, target: nil, action: nil)
    b.bezelStyle = .texturedRounded
    b.isBordered = false
    b.setAccessibilityLabel(name)
    b.toolTip = name
    return b
  }

  private func label(_ tab: [String: Any]) -> String {
    if let t = tab["title"] as? String, !t.isEmpty { return t }
    if let u = tab["url"] as? String, !u.isEmpty { return u }
    return "New Tab"
  }

  private func rect(_ r: NSRect) -> [String: Any] {
    ["x": r.minX, "y": r.minY, "w": r.width, "h": r.height]
  }
}

// A tab, to a screen reader, as role="tab" is in a web shell: AXRadioButton with the
// AXTabButton subrole, selected while it is the active tab, named by its title. AppKit's
// accessibility element for a control is its cell, so the cell says it.
final class TabButton: NSButton {
  override class var cellClass: AnyClass? {
    get { TabCell.self }
    set {}
  }
}

final class TabCell: NSButtonCell {
  override func accessibilityRole() -> NSAccessibility.Role? { .radioButton }
  override func accessibilitySubrole() -> NSAccessibility.Subrole? {
    NSAccessibility.Subrole(rawValue: "AXTabButton")
  }
  override func accessibilityLabel() -> String? { title }
  override func accessibilityIdentifier() -> String { controlView?.accessibilityIdentifier() ?? "" }
  override func isAccessibilitySelected() -> Bool { state == .on }
  override func accessibilityValue() -> Any? { NSNumber(value: state == .on ? 1 : 0) }
}

// A button that shows a glyph and is named for what it does, Back or Settings, as the
// check finds it. (SF Symbols would look better, but their first use costs about 170 ms
// at start-up: docs/native-shell.md.)
final class NamedButton: NSButton {
  override class var cellClass: AnyClass? {
    get { NamedCell.self }
    set {}
  }
}

final class NamedCell: NSButtonCell {
  override func accessibilityLabel() -> String? { controlView?.toolTip ?? title }
}

// The address field, named Address.
final class AddressField: NSTextField {
  override class var cellClass: AnyClass? {
    get { AddressCell.self }
    set {}
  }
}

final class AddressCell: NSTextFieldCell {
  override func accessibilityLabel() -> String? { "Address" }
}

private func int(_ v: Any?) -> Int64? {
  if let n = v as? NSNumber { return n.int64Value }
  return nil
}

private func dbl(_ v: Any?) -> CGFloat {
  CGFloat((v as? NSNumber)?.doubleValue ?? 0)
}
