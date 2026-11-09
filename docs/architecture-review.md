# Architecture review, from building tabs

Written 2026-11-09 after adding window controls, drag regions, vertical and horizontal
tabs, favicons and the layout switch on branch `tabs` (five commits, about 2,300 lines
changed in 40 files). Everything below is from that work, with the file and line it
rests on. Ratings: **good** (keep), **fine** (keep, with a known cost), **weak** (change
when it next hurts), **mistake** (change now).

The short version: the shape is right. Go managers with hooks, one engine interface, a
thin C++ edge and a transparent web shell gave a first-week codebase that a new person
can read in an hour, and adding a whole feature (horizontal tabs with a persisted
preference) never touched C++ or C. The costs are concentrated in two places: the C
boundary is glue that must be typed six times per event, and the two-layer window makes
hit-testing and focus eventually consistent instead of synchronous. Neither is a
mistake; both are worth a deliberate decision now that the pattern is clear.

## 1. The engine interface: would it survive a second engine? Good

`core/engine/engine.go` is 115 lines: an `Engine` with 16 functions (line 42) and a
`Listener` with 13 events (line 86). Everything crossing it is an integer id, a string,
a rectangle or an enum. Nothing content-specific leaked while tabs were added:

- Tabs needed `CreateTab` (now hidden), `CloseTab`, `ShowTab`, `CloseWindow`,
  `FocusShell` and the events `TabOpened` and `Shortcut`. Each is what any engine does.
- Favicons crossed as a `data:` URL string (`FaviconChanged`), so the interface knows
  nothing about bitmaps, mojom or Skia. `engine/browser/page_tab.cc:102` holds all of
  that.
- The mock implements the whole thing in `core/mockengine/mockengine.go` (355 lines)
  with per-tab history, which is the test that the interface is engine-neutral.

Two places a second engine would push back. `Shortcut` assumes the engine lets the app
see reserved keys before the page does; content does (`PreHandleKeyboardEvent`), WebView2
does, WKWebView does not cleanly. And the shell side of the interface (`SendToShell`,
`SetShellRegions`, `SetPageBounds`) assumes the shell is a second web view the engine
hosts, which is a design choice the whole app rests on, not an accident.

What was designed for one tab and had to change: `CreateTab` made the tab active and
`OpenURLFromTab` loaded everything into the current tab (`page_tab.cc`, old lines 78 to
90). Expected in a first app; the change was contained to the adapter and the manager.

## 2. Managers and hooks: good

`core/tabs/tabs.go` (340 lines) is the best argument for the model. It owns the list, the
order, the active tab and each tab's state; every change is a plain function; the
engine is told what to show (`Activate` at line 88 calls `eng.ShowTab`), and subscribers
are informed after. A reader can follow `Close` (line 123) top to bottom and see what
happens to the active tab and what fires when the last one goes.

All wiring is in `core/app/app.go:45-52`, eight lines. Adding the horizontal layout meant
one new manager (`core/prefs/prefs.go`, 69 lines) and two of those lines. The hooks
only inform: `a.Tabs.OnEmpty(e.CloseWindow)` is the whole "closing the last tab closes
the window" feature.

Two judgement calls worth knowing about. `App.Shortcut` (`app.go:97`) is a switch that
maps each reserved key to one manager call; it is dispatch, not logic, but it is the only
branching in the wiring file. And `core/shell/shell.go` is a protocol translator with 15
commands; at this size it is one screen, at fifty it would want splitting by manager.

## 3. The C boundary with two function tables: fine, with a tax

The design holds: `engine/abi/pb_abi.h` (94 lines) is readable C, the library has no
link-time reference to Go, the helper has no Go at all, and the main-thread rule is
checked in one place (`engine/browser/engine.cc:34`). The driver confirms 0 Go symbols in
the helper and library and only `_ProjectBMain` exported.

The tax is mechanical and measurable. One engine-to-Go event touches six files before
any real code: `engine.go`, `app.go`, `pb_abi.h`, `pb_exports.h`, `app/main.cc`,
`cmd/lib/main.go`. One Go-to-engine call touches five, including a `static inline` C shim
per function in `cmd/lib/main.go:12-26` because cgo cannot call a function pointer. This
session added five events and six calls: roughly sixty edits of pure glue. It also
produced the one confusing failure of the day: the Go export `PBShortcut` collided with
the C enum `PBShortcut`, because cgo puts exports in the header's namespace
(`pb_exports.h:29` is now `PBShortcutPressed`).

Verdict: keep the two tables, generate the glue. One table in one file (the Go interface
is already that table) can produce the C header, the shims, the exports and the
`main.cc` initialiser. About a hundred lines of Go, run from `engine/build_go.py`. Until
then every feature pays the six-file toll and a junior will miss one of the six.

## 4. One shared library and two thin binaries: good

Untouched this session and it never got in the way. Incremental builds of an adapter
change ran 6 to 32 s (`tools/build-and-wait.sh`); the DMG from `tools/package.sh` is 80 MB
and the packaged app passes the same checks as the build-folder binary. `first-app.md`'s
sizes still hold. Leave it alone.

## 5. Go for the managers, including cgo: good

The app's logic is 1,900 lines of Go (mock included) that compile and test in a second with no Chromium.
The two most readable files in the repo are `tabs.go` and `prefs.go`. The C++ adapter is
2,200 lines plus a 490-line driver, nearly all content plumbing, with no decisions in it.

cgo's costs are the boundary tax above plus two facts from `first-app.md` that still
stand: a Go nil dereference is fatal once Chromium's signal handler is installed, and
the Go runtime is in the main process only. Neither bit this session. 34 to 38 ns per
call to Go (`build/drive.out`) is not a number anyone will notice.

## 6. The two-layer window: good, with one property to know

The shell is a transparent page over the page views and reports rectangles; the
container's `hitTest:` answers from those rectangles
(`engine/browser/window_mac.mm:43`). Three things this session rested on it, all small:

- Drag regions: a second rectangle list, `nil` from `hitTest:` inside it so AppKit's
  frame drags and zooms (the way Chrome does it, `bridged_content_view.mm:298`), and a
  `sendEvent:` for double-clicks below the title bar (`window_mac.mm:89`). About sixty
  lines across four files.
- Window controls: the engine measures the buttons (`window_mac.mm:162`) and the shell
  leaves the space; the full-screen guess from window size is gone. Hit-testing from
  the frame view showed the title bar does not swallow clicks over the shell.
- Many page views: one holder view, autoresizing kept, `ShowPageView` hides the rest
  (`window_mac.mm:248-270`).

The property: rectangles are pushed from the renderer after layout, so they are one or
two frames stale plus an IPC hop after any change. The driver clicked 30 ms after Cmd+T
and hit the strip's old empty space, which dragged instead of closing. Chrome asks its
view tree synchronously on each event; we cache. A person cannot click that fast. If a
real display ever shows mis-clicks, the fix is a synchronous ask over the same channel,
not a different window model.

Second property: focus policy is a round trip. After Cmd+W, nothing holds the keyboard
until the shell hears the new tab list and sends `focusPage` or `focusShell` back
(`shell/src/AddressBar.vue:146`). A second Cmd+W inside that gap does nothing. See the
first item under "change first".

Third: the mock cannot see compositing. An opaque background on the shell's root
painted over the page hole and looked right over the mock's iframe; only the real
driver's composite showed a blank page.

## 7. Vue for the shell: good

Eleven files, 1,100 lines. `state.js` is a mirror of what Go sends and computes nothing but
`activeTab` and a label. The horizontal layout was a `v-if` in `App.vue` and one new
component; both tab views read the same state and send the same three commands. The
regions directive (`shell/src/regions.js`) absorbed `v-drag` as a second set with no
change to the batcher.

Weak spot: `AddressBar.vue` is 285 lines and now holds focus policy (which tab wants the
address bar) as well as suggestions and selection. The CSP in `shell_page.cc` with
trusted types held without change. The design's Zoho Puvi font is not bundled, so the
shell renders in the system font.

## 8. The mock engine: fine

It carried per-tab history, tab opening, shortcuts and full screen without the
interface changing, and the Go tests on it run in a second. Its costs: it is a
hand-written parallel of content's behaviour (loads, commits, history) that can drift;
the dev server and mock page are a second protocol to maintain; and it cannot show the
two kinds of bug this session actually hit, compositing and focus. Keep it for manager
tests and the shell's logic; do not expect it to find window bugs.

## 9. The three test layers: good

`go test` (1 s) caught nothing this session, which is right: the managers were not where
the risk was. The Playwright run on the mock (30 s, 67 checks) caught protocol and
layout mistakes in the shell. The real-app driver (3 min, 84 checks) found every real
bug: the opaque root, both crashes in the adapter, the stale drag rectangle, and the
focus gap. It is the most valuable layer and the most expensive to keep honest: 600
lines of Python and 490 of Objective-C, one flake fixed this session (an eval during a
frame swap never answers), and a session with no display, so dragging, double-click
zoom and real full screen are still unverified.

Change made this session: the driver's hit test now starts from the window's frame view
(`engine/browser/driver.mm:60`), so the title bar and its buttons take part. The old
test started from the content view and could not have seen the risk it was meant to
cover.

## 10. Build and packaging: good

`tools/build.sh` is flat, re-runnable and honest about what it does; GN sees the repo
through three symlinks and `root_extra_deps`. The Go archive is a GN action with a
depfile, so a Go edit rebuilds exactly that. Packaging signs with one key, strips, and
the DMG mounts and runs. The one GN edit this session was a dependency for a mojom
header (`engine/BUILD.gn`). Leave it alone.

## What was wrong before this work

- `main`'s own driver run ended `1 FAILED` (`build/drive.out` at the start: "at screen
  size the tab takes the window buttons' place"), while `first-app.md` said all 39
  checks pass. The full-screen guess compared window size to screen size and never
  fired. Not an architecture fault, but the docs claimed what was not measured.
- The driver's "shell" hit tests never went through the title bar, so the stated risk
  (the title bar swallowing clicks) was untested either way. It turned out not to
  swallow them; the measurement was ten lines.
- `Engine::FocusAddressBar` in C++ both moved native focus and told Go. Focus is now a
  plain engine call (`FocusShell`) that Go decides to make. Small, but it was the one
  piece of policy in the adapter.

## Change first

1. **Generate the C glue** from the Go interface: header, shims, exports and the
   `main.cc` table. Removes the six-file toll and the namespace clash class of error.
2. **Move focus policy into Go.** Let the tabs manager call `FocusPage` or
   `FocusShell` when the active tab changes, so Cmd+W followed by Cmd+W works and the
   shell stops deciding where the keyboard goes.
3. **Run the driver on a display** for the list in the final report: drag by the empty
   strip, double-click zoom, real full screen hiding the buttons, and compositing by
   eye. Everything else about the window is measured.

## Leave alone

The engine interface, the managers and their wiring, the shared library and two
binaries, Go, the Vue shell and the build. They did what they promised when a second
feature pushed on them.
