# A native shell: Swift on the Mac, Kotlin on Android

A design note, 2026-11-14, with a spike on the branch `native-shell` (not for merge). The
owner: "we would however should have one native shell too... meaning a compiles swift in
mac... kotlin one for android".

Every shell today is a web page, Vue or Svelte, in a WebContents of its own over the
window ([shells.md](shells.md)). A native shell is the same shell drawn by the platform's
own toolkit: AppKit, from Swift, on the Mac; Android's views, from Kotlin, on a phone. It
is held to the same messages and the same rules as every other shell. Standard stays the
default and the reference: every Go feature shows there first, and the native shell
follows it.

## The one idea

**The shell's channel is already plain JSON across the C boundary.** Go sends a shell a
message with `send_to_shell(window, json)`, and a shell's message reaches Go as
`PB_EVENT_SHELL_MESSAGE` with the window and the JSON (`engine/abi/pb_abi.h`). A web
shell's page is only what sits at the two ends: `chrome.send('shell')` on one side,
`window.__shellReceive` on the other. A native shell puts its own view at the same two
ends. Go doesn't change, and doesn't know: "Go doesn't know which shell is on the other
end" stays true.

```
              Go core (unchanged)
   send_to_shell(window, json)  ^  PB_EVENT_SHELL_MESSAGE(window, json)
                |               |
          Engine::SendToShell   hooks::Fire
                |               |
      +---------+---------------+----------+
      | web shell:  ShellPage (WebContents) |  chrome.send / __shellReceive
      | native:     NativeShell (an NSView  |  receive(json) / send block
      |             or an Android View)     |
      +-------------------------------------+
```

## How each side receives the snapshot and sends the messages

**Mac.** `NativeShell` (`engine/browser/native_shell_mac.{h,mm}`) owns a Swift view,
`PBNativeShellView`, exposed to Objective-C++ through the header swiftc writes.

- In: `Engine::SendToShell` calls `[view receive:json]`. The view parses the message with
  `JSONSerialization` and keeps Go's state exactly as the SDK's rule 1 says: the snapshot
  has everything, a state message sets the fields it carries, `tab` replaces one tab.
- Out: the view calls the block the engine gave it, with JSON. The block posts a task that
  fires `PB_EVENT_SHELL_MESSAGE`, so a message sent while Go is calling the view (`shown`,
  in answer to the snapshot) never re-enters Go, and the order is kept, as `chrome.send`'s
  are.
- `focus_shell` makes the view's address field first responder; a tab's page takes the
  keyboard as it does today (`FocusTab`).

**Android.** The same at the JNI line. `BrowserWindow.java` already holds the shell's
`ContentView` in `ShellHolder`, the view that refuses a touch where the layout says page.
A native shell puts a Kotlin `View` there instead:

- In: `Engine::SendToShell` calls the Kotlin view's `receive(json: String)` through JNI
  (`@CalledByNative`), on the UI thread, which is the browser's main thread on Android.
- Out: the view calls a `@NativeMethods` function, `send(window, json)`, which posts the
  same task and fires the same event.
- The touch rule doesn't change: `ShellHolder` asks the layout, then the native view.

In both, the native code is a shell without the SDK, so it does the seven things
"A shell without the SDK" lists itself. The only ones with any weight are measuring the
layout and the colours (below).

## Page views under native chrome

Nothing new. The window already puts the shell's view on top of the pages and decides
every press from the layout's regions (`PBContainerView hitTest:` on the Mac,
`ShellHolder` on Android). A native shell's view takes the web shell's place in that
stack, transparent wherever it draws nothing, and reports the same `layout`:

- `shell`: its bars; `drag`: the strip that moves the window; `controls`: the controls in
  that strip; `tabs`: each shown tab and its rectangle; `viewW`, `viewH`; `frame`.
- Units are the web shell's: points on the Mac (a flipped `NSView` measures y down from
  the top, as the rectangles do), dp on Android (px divided by the density).
- It measures its own subviews' frames and sends `layout` only when it changed, as the
  SDK does once a frame: after a resize, and as soon as it has drawn a message from Go,
  since a message alone starts no display pass in AppKit.
- A press in a `shell` region reaches `[shellView hitTest:]`, so AppKit's own hit testing
  picks the button. A press in a page region never reaches the native view.

## Build and packaging

**Swift is built beside GN, as Go is.** Chromium's Mac toolchain has no `swift` tool on
purpose: "Swift is only used on iOS, not macOS... we intentionally disallow future uses
of Swift on macOS" (`build/toolchain/apple/toolchain.gni`, crbug.com/965663). Using
`swift_source_set` would mean patching upstream. So:

- `action("native_shell_swift")` runs `engine/build_swift.py`: Xcode's `swiftc`, one
  call, `-emit-library -static` and `-emit-objc-header-path`, for the deployment target
  and SDK GN already uses. Out come `libnative_shell.a` and `ProjectBNative-Swift.h`.
- `source_set("native_shell_lib")` links the archive. The same action writes a linker
  response file with two library folders, the SDK's `usr/lib/swift` and the toolchain's
  `usr/lib/swift/macosx`, which every target linking it reads (`all_dependent_configs`),
  since Chromium's `.gn` allows no new `exec_script`. The archive asks for the Swift
  libraries it needs itself (autolink), and lld follows.
- **No Swift runtime ships.** macOS has carried it since 10.14.4 and the deployment target
  is 13.0. The one static piece the toolchain adds is `libswiftCompatibilityPacks.a`,
  linked in.
- It links into `libproject_b_content.dylib`, which only the browser process uses for
  windows; helpers load it but never touch the view.

**Kotlin goes in the APK through Chromium's own rules.** `android_library` takes `.kt`
sources beside `.java` (`build/config/android/internal_rules.gni`, with
`//third_party/kotlinc` and `//third_party/kotlin_stdlib`), so the Kotlin shell is
another library in `engine/android/BUILD.gn`. R8 already shrinks the APK; the Kotlin
standard library adds what the shell uses of it.

**Plain views, not SwiftUI or Compose, to start.** AppKit and Android views give the
frame of every control directly, which the layout needs every pass, and they cost
little at start (the spike's view: 17 ms to make). SwiftUI would need geometry readers to report frames and adds its own
start-up; Compose isn't in Chromium's tree and is a large dependency. (Question 3.)

## Choosing it like any other shell

- **One line in the registry:** `{ "id": "native", "name": "Native", "native": true }`.
  The id is the same on both platforms; each platform's build carries its own
  implementation. `Listed` offers a native shell on every platform; it is the phone's
  `phone` rule's only exception, since on a phone the Kotlin shell is a phone shell.
- **Go names it as any shell:** the page is `shell/native`. The engine is the only one
  that knows it is native: where it would make a `ShellPage` for that page it makes a
  `NativeShell`. View, Shell, the pref, a template and the live switch all work as they
  do (the switch drops the old view, the new one says `ready`, Go sends the snapshot).
- No folder in `shell/src/apps/shell/` and nothing for vite to build. The registry test
  that every entry has a folder learns that native entries have none.
- The mock can't load it (the mock is a browser). See "Testing while writing it" below.

## The contract with no DOM

The rules in "What every shell does" are about behaviour; only the way the check finds
things is DOM. Split it the same way:

1. **What Go sees** (rules 1, 2, 3, 7, 8, 9, 10, 13): `ready`, `shown`, the layout's pages
   inside the band and not overlapping, the active tab shown, `focusTab` only after a
   layout that shows it, no page under the window buttons, a reload and a switch coming
   back from Go's data, no errors on the timeline. All of it is in the layout and the
   messages, which the real app's driver and DevTools (`ProjectB.*`) can already read.
   None of it needs a DOM.
2. **What a screen reader sees** (rules 4, 5, 6, 11, 12): every control **by role and
   name in the platform's accessibility tree**, which is how the check already says it
   finds them. The roles map one to one:

   | Rule | Web (ARIA) | Mac (NSAccessibility) | Android (AccessibilityNodeInfo) |
   |---|---|---|---|
   | A tab | `role="tab"`, `aria-selected` | `AXRadioButton`, subrole `AXTabButton`, `AXSelected` and `AXValue` 1 while active | `className` `android.widget.RadioButton` or a `CollectionItemInfo` with `isSelected` |
   | Its id | `data-tab="<id>"` | `AXIdentifier` `tab-<id>` | `uniqueId` `tab-<id>` (API 33), else the view's tag read through the driver |
   | Address | text box or combo box named Address | `AXTextField` or `AXComboBox`, label Address | `EditText`, contentDescription Address |
   | Back, Forward, Reload, New Tab, Close Tab, Settings | buttons by name | `AXButton` by its label | `Button` by contentDescription |
   | Rule 6, every control is the shell's | the control's box inside a `shell` region | the element's `AXFrame` inside a `shell` region | `boundsInScreen` inside a `shell` region |

   Chromium maps ARIA to the same Mac roles (`role="tab"` is `AXRadioButton` with
   `AXTabButton`), so **one check can read the OS tree for every shell, web or native**,
   on the real app. That is the recommendation: the contract check grows a second front
   end that reads the OS accessibility tree (AXUIElement on the Mac, UiAutomator on
   Android, which `tools/drive_android.py` already reads by label), and the web shells
   keep the DOM front end on the mock for speed.

**Testing while writing it.** A native shell is still written against the mock, as the
rule "Building a shell is friendly to its developer" asks: the Swift view takes a
transport, the engine's block in the app, or the mock's HTTP channel (`/shell/send` and
`/shell/events`, what `core/cmd/dev/webui.js` uses) in a small harness app that draws a
placeholder where each page goes. Same code, no Chromium build. The Kotlin shell the same,
with the emulator talking to the mock through `adb reverse`.

## Cost against a web shell

Measured on the spike, the same binary for both shells, five starts each after a
warm-up, headless through the driver, on a Mac Studio with a load average of 19 from
other builds, so the times are higher than [benchmarks.md](benchmarks.md)'s 0.2.50
numbers and only the comparison counts. Memory is macOS's footprint 5 s after start.

| Median of 5 | Standard (Vue) | Native (spike) |
|---|---|---|
| `window.shown`, ms from start | 191 | 229 |
| `shell.ready` (Go sent the snapshot) | 378 | 370 |
| `app.ready` (the shell said `shown`) | 404 | 407 |
| The shell's own first paint, then on screen | 390, 465 | drawn when `shown` is sent; no renderer to wait for |
| Browser process, MiB | 33.9 | 37.1 |
| GPU process | 30.2 | 12.6 |
| Renderers | 3, 62.9 | 2, 34.5 |
| Network | 7.9 | 7.6 |
| **Total** | **134.7** | **91.8** |
| What ships | 96 KB of JS | a 148 KB static archive before the link strips it; no runtime |

- **Memory: 43 MiB less, a third.** The shell's renderer goes (about 28 MiB), and the GPU
  process halves, since it no longer composites a page the size of the window over the
  pages. The browser process grows by 3 MiB of AppKit views and the Swift runtime.
- **Start-up: level.** The native shell skips the renderer and the navigation, but its
  view is made inside Go's start (`window.created` to `shell.created`: 17 ms, against
  3 ms for a WebContents), and its `ready` waits behind macOS's launch step like
  everything else. The web shell's renderer boots in parallel with that same wait, which
  is why it loses nothing. A native shell would win only once the launch step stops
  holding the main thread ([benchmarks.md](benchmarks.md), "Ideas").
- **SF Symbols cost 170 ms.** The spike's first build drew its buttons with SF Symbols, and
  the first use of the symbol font held the main thread for about 170 ms at start. The
  spike draws text glyphs; a real shell bundles its icons as images.
- A crash in a native shell is a crash of the browser process. A web shell's renderer can
  die and reload with the pages untouched.
- Every Go feature a web shell can use (groups, workspaces, extensions, suggestions,
  context menus, the update, the crash notice) is drawn again in Swift and again in
  Kotlin.

The size of the app hardly moves either way: the pages need the renderer and Blink
regardless, and the settings page stays a web page.

The real cost is the last row. Every message a web shell can use (groups, workspaces,
extensions, suggestions, context menus, the update, the crash notice) a native shell
draws twice, in two languages, with a check that only proves it matches by role and
name.

## What the spike proves

Branch `native-shell`, `engine/native/mac/NativeShell.swift`, about 330 lines, run
headless through the driver on `build/pb/project_b` with a profile whose pref is
`{"shell": "native"}`:

- **The shell starts from Go's snapshot alone.** Its view says `ready` once it is in the
  window, draws the snapshot, reports the layout and says `shown`; Go marks `shell.ready`
  and `app.ready` as for any shell, and the window shows the first tab's page under it.
- **Every message the spike uses works both ways.** New Tab, typing an address and Enter,
  a press on a tab, and Cmd+T from the menu each reach Go, and Go's `tabs` and `tab`
  answers redraw the native tabs (the title follows the page's `<title>`) and the field.
  After each, the window shows the active tab's page, and only it.
- **The window manager's rule holds with no change.** The driver's hit test answers
  shell for the toolbar, frame (it drags) for the strip's empty part, page for the page.
- **The check by role and name works on the native tree.** The driver reads the shell's
  own accessibility tree in the app (`ax`) and presses and types by role and name
  (`axpress`, `axset`). Each tab is `AXRadioButton` with the `AXTabButton` subrole, its
  id `tab-<id>`, selected while active; Address is an `AXTextField`; Back, Forward,
  Reload, New Tab and Settings are `AXButton`s by name.
- **Swift builds in `tools/build.sh`'s ninja run** with no upstream change, and links into
  the content library.
- The timeline has no warning or error over the run.

What it taught, each fixed on the branch:

1. **AppKit's accessibility element for a control is its cell.** A role or name set on an
   `NSButton` or `NSTextField` doesn't reach the tree; the shell's controls answer from
   cell subclasses (`TabCell`, `NamedCell`, `AddressCell`).
2. **Report the layout when Go's message is drawn, not in AppKit's display pass.** A
   message that starts no display pass, a new tab from Cmd+T, otherwise left the old
   tab's page showing.
3. **The driver sent keys straight to the first responder.** With a native field there,
   Cmd+T never reached the menu; it now tries the menu bar after the responder, as
   AppKit does.
4. **The engine's code assumed a shell page** in a handful of places (`devtools.cc`,
   `popup.cc`, `shell_page.cc`, the driver); each now checks.

Not done: a picture of the real window with its page, which needs a desktop (the
driver's pictures are the shell's views alone); the live switch to and from another shell
(the engine code is there, not run); groups, workspaces, extensions, suggestions, the
notices, colours from the frame colour; the mock harness; Android.

The registry test that every shell has a folder fails on the branch, by design: it is
the rule this doc proposes to change.

## Open questions for the owner

1. **One shell, two implementations, or two shells?** Recommendation: one registry id,
   `native`, drawn the platform's way on each (a Safari-like toolbar on the Mac, the
   Mobile shell's thumb bar on Android), with the same name, so a profile that syncs
   keeps it.
2. **Where it sits in the lineup.** Standard stays the default and the reference.
   Recommendation: Native is a style to choose, as Glide and Fox are, and follows Standard
   one release behind; it is never where a Go feature shows first.
3. **AppKit and views, or SwiftUI and Compose?** Recommendation: AppKit and plain Android
   views now, for frames the layout reads directly and no start-up cost; revisit SwiftUI
   if the shell grows sheets and animations that are tedious in AppKit.
4. **The colours.** The web shells make their palette from the frame colour in JS
   (`@/tones.js`). Recommendation: move `tones` into Go and send the palette in the
   snapshot and `windowColor`, so every shell, web or native, paints the same tones from
   one place. The alternative is a copy of `tones` in Swift and in Kotlin.
5. **The check.** Recommendation: read the OS accessibility tree in the real-app check for
   every shell (above), keep the DOM check on the mock for the web shells, and require the
   native shell to pass the OS one on each platform before it is offered.
6. **Right-click and long-press menus.** Decided for web shells: drawn by the shell, Go
   supplies the context. For a native shell, drawing them is NSMenu and Android's popup
   menu, still from Go's `contextMenu` message. Recommendation: allow it, since there the
   platform menu is the shell's own drawing.
7. **Is the cost worth it now?** It saves 43 MiB of 135 at start, a third, and no start-up
   time, and it doubles the drawing of every feature. Recommendation: keep
   it to this spike and the doc until the daily-driver gaps close, then build the Mac one
   first, against the OS-tree check.
