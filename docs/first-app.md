# The first app

Windows with tabs. A 44px top row holds the sidebar toggle, back, forward, reload and
the address bar after the space for the native window controls; tabs sit in a sidebar
on the left, or in a strip on top when the user switches, and the page sits beside or
below them. The address bar's suggestion box, fed by real history, extends over the page.

## Pieces

| Folder | What it is | Needs Chromium |
|---|---|---|
| `core/` | Go managers (tabs, history, suggest, policy, window, prefs, update, shell), the timeline that all logging goes through, and the engine contract | No |
| `core/mockengine`, `core/cmd/dev` | Mock engine and dev server | No |
| `shell/` | Vue 3 shell | No |
| `core/cmd/lib` | The cgo layer: the Go core as a c-archive | Links into the main binary |
| `engine/` | C++ adapter on Chromium's content layer, built into the library | Yes |

## One library, two binaries

- `libproject_b_content.dylib` holds content, Blink, V8, the network stack and the C++
  adapter. It exports one function, `ProjectBMain`.
- `project_b` is the main process: Go, and a `main` that hands Go's functions to
  `ProjectBMain`.
- `project_b_helper` runs every child: a `main` that calls `ProjectBMain` with no Go.

Both binaries link the library when they load, through `@rpath`, so our code never calls
`dlopen`. Chromium's own `content_shell` and Chrome do load theirs by hand. The adapter
has to live in the library, because outside a component build content exports nothing a
binary could link against.

Go's runtime starts by itself when a binary linked with it loads, so "linked but
unused" isn't possible in a child. Content asks `GetChildProcessPath` which program to
launch, and we answer with the helper.

## The boundary

`engine/abi/pb_abi.h` is plain C, ids and values. Each side holds a table of the other's
functions: `PBEngineCalls` for Go calling C++, handed over at start, and `PBGoCalls` for
C++ calling Go, which `app/main.cc` fills from Go's exports (`engine/abi/pb_exports.h`).
So the library has no link-time reference to Go, and the helper has nothing to resolve.
Every call is on the main thread, except `post_wakeup`. Mojo and content's types stay
inside the adapter.

Everything the adapter tells Go goes through one function, `hook`: a number and a flat
list of values. Content's own hooks are carried by generated classes that check one bit
each and cross only when Go subscribed; the adapter's four own events (a shell message,
a tab a page opened, a favicon, the window controls) use the same call. Adding a use of a
content hook is a Go change only; see [content-hooks.md](content-hooks.md).

## The window

Stacked views: the page views of the tabs the window shows at the bottom, inside one
holder, each where the layout puts it, with every other page view the window holds
hidden; a full-window transparent shell on top. Shell components mark themselves with
`v-shell`, the top row's bars that move the window wherever they have no control mark
themselves with `v-drag`, and `<PageView>` says where the active tab's page goes; the shell
sends all of it as one `layout` message, with the controls the SDK found in the drag bars,
and `tabs`, each visible tab with its rectangle. The shell shows one tab, so the list has
one entry. Go's window manager (`core/window`) owns the window: it takes that report and
the engine's `WindowState` (size, buttons, corner radius, full screen), decides what each
point does, and gives the engine one `WindowLayout` through `SetWindowLayout`: the drag
bars' controls first, as shell areas, then the drag areas, then the other shell areas, the
visible tabs, and the frame. The engine puts each listed tab's view at its rectangle,
tells a tab that joins the list it is shown and one that leaves it that it is hidden,
and titles the window after the visible tab that has the keyboard. In full screen it sends no drag areas. Close, minimise, zoom and full screen
requests go through it too. The design: [window-manager.md](window-manager.md).

`PBContainerView hitTest:` walks the layout's regions in order: a drag area answers nil,
so the press falls through to AppKit's frame, which zooms on a double-click; a
shell area answers the shell's view; anything else the page under it. Content's own views ask
the same `hitTest:` before accepting a mouse event, so nothing routes events by hand.
That is how Chrome does it too (`components/remote_cocoa/app_shim/bridged_content_view.mm`).
The sidebar and the frame's left, right and bottom edges never drag; the resize edges
are AppKit's.

Each page view keeps its margins through autoresizing, so it follows a window resize
in the same frame. Page positions measured for an older window size are ignored,
because fresher ones follow.

Focus is its own call: the shell asks for a tab's page with `focusTab`, after sending
its layout, so the tab is already showing, and Go calls `FocusTab`. When a click or Tab
moves the keyboard into a visible page, content's `OnWebContentsFocused` hook tells Go
which tab, and it becomes the active one.

The shell's top row is 44px, centred on the window buttons of a unified toolbar, and
the window uses `NSWindowStyleMaskFullSizeContentView` with a transparent title bar, so
the shell draws under the native window buttons. The engine reports where those buttons are
(`WindowChanged`: after start, on resize, on entering or leaving full screen, empty
when there are none) and the shell leaves exactly that space on whichever side
they are. Hit-testing from the window's frame view, as AppKit does, shows the title bar
does not swallow clicks over the shell: a point on a control returns the shell, a point
on the empty strip returns the frame, a point on a window button returns the button.
The window server, though, moves a window by itself, without asking the app, from any
part of the title bar that AppKit says no view needs (`_opaqueRectForWindowMoveWhenInTitlebar`),
and a transparent view needs none. So the container covers everything but the drag areas
with empty views that each claim all of themselves: the window server moves the window
from the drag areas, as it does any app's title bar, and every other press goes through
the hit test, so a drag across the address field selects text. A drag area gives up the
part an earlier region holds, as the hit test does, so a control in a bar that drags stays
the app's. The window server only looks at the title bar, 66 points with the unified
toolbar, so a drag area below it, such as the toolbar row under the tab strip, moves the
window because the window hands a press there to the server itself
(`performWindowDragWithEvent:` in `PBNSWindow`'s `sendEvent:`).

The window goes on screen once Go has given it its first layout, frame included, so even its
first frame has them. Until the shell's first frame, content paints the shell's view
white over all of it, as it does any view it has no frame for yet. A new tab loads
`about:blank`, so every tab has a page view that paints in content's default colour from
the start. The tab still reads as new: no address, titled New Tab, and `about:blank`
stays out of history. A crashed page has no view, so the frame colour shows there.

## Several windows

Go owns which windows exist and makes them: `CreateWindow` with Go's id for it, 1 for the
first, the window in front to open near, and its first layout. Each window has its own
shell page and its own settings page, and every call and event about a window names it.
Go keeps each window's tabs in order with its own active tab, and the window in front:
the last one the engine reported as the key window (`WindowFocused`), or the one Go made
last. Keys, menu items and commands act on the active tab of the window in front, and
the menu shows its state. File, New Window (Cmd+N) opens a window with one new tab, a
little down and right of the window in front, the same size. Closing a window's last tab
closes the window; its own close button closes it and its tabs; closing the last window
quits, and Quit ends the app with every window at once. A tab a page opens goes in its
opener's window, and one nothing opened, from DevTools or an extension, in the window in
front. A minimised window's tabs are told they are hidden. On the mock engine each window
is an OS window page of its own, `/?window=<id>` on the dev server, window 1 at `/`, and
the id goes on to its shell, its settings page, its mock page and its feeds. The design
and what is still to come: [windows.md](windows.md).

## Our own page

The engine knows exactly one page of ours, `chrome://shell/`, built from `shell/` and served
from the app's `Resources/shell` (`out/pb/shell` in the build folder, which ninja builds with
vite through `engine/build_shell.py`). It gets its bridge only in a window's own shell WebContents;
anywhere else its WebUI controller gives it no bindings and drops whatever it sends, and
the policy gate refuses it in every tab, however it is reached. Our pages load in a context
of their own, off the record and with no folder, made before the profile's
(`engine/browser/browser_context.h`), and are registered only there: in the profile's
context, where the tabs, extension pages and popups are, their addresses are no page at
all. On Android the two are one context. No other page has
`chrome.send`, and Chromium itself refuses a web page's link to a `chrome://` page or a
frame of one.

## Settings

Settings are part of the shell, not a page. The toolbar's gear opens a panel the shell
draws over the page's top-right corner, under the gear, in either layout; the gear again,
Escape or its close button closes it, and there is only ever one. Like any shell component
it marks itself `v-shell`, so its rectangle is a shell region and clicks inside it never
reach the page. It shows the tab layout, the frame colour, the running version with Check
for updates, and the automatic updates switch. Go owns all of it: the snapshot carries it,
the panel sends the user's changes, and Go says when they change.

The frame colour is a pref, `frameColor`, by default a dusty teal, `#4f9e9e`. The panel
offers it and the frames of the Ulaa design's modes (Personal's teal, Work's blue, Kids'
amber, Open Season's red, Incognito's slate), and a colour input for any other. The
window manager paints the window's frame with it through the next `SetWindowLayout` and tells the
shell, so both change together, and Go derives the shell's other tones from it
(`core/tones`, sent with the colour): the bar and surfaces toward white, the icons' ink toward black, the accent's soft
fills the colour itself, faint.

## One copy per profile

Go chooses the profile before content starts (`core/profiles`, through `PBClaimProfile`):
the listed profile `--profile=<id>` names, else the one used last. The list is
`profiles.json` in the root, `~/.project-b` unless `--root=<dir>` names another, beside the
profile folders; a first start makes it holding `default`, named Personal, and leaves a
`default` that already holds a profile as it is. `--user-data-dir=<dir>` is a folder used
as the profile as it is, and never enters the list: ChromeDriver and the tests use it.

The Profiles menu lists the profiles, read as it opens, and choosing one starts a copy of
the app on it; each copy has its own Dock icon, with a dot in its profile's colour.
"Profiles…" there, `--picker`, or a list with no profile used yet starts a copy that shows
the picker page instead, which opens the profile chosen, or a new one, in a copy of its
own. See [profiles-build.md](profiles-build.md).

A profile has workspaces: sets of tabs whose cookies and site storage are their own, one at
least. A new profile has Main, the profile's own storage; every other is a storage
partition of the profile's context. They are listed in `workspaces.json` in the profile
folder. Settings, Workspaces makes, edits, moves and deletes them; Tab, New Tab in
Workspace opens a tab in one, and every tab that tab opens is in it too, as is New Tab
(Cmd+T) from it. A tab nothing places opens in the first. See [workspaces.md](workspaces.md).

One copy of the app owns a profile folder while it runs (`core/instance`). A second copy
started on the same profile writes nothing there: it hands the URLs
on its command line to the running copy, whose window used last comes to the front and
opens each in a new tab, and exits 0.

## Logs and timing

The app keeps one timeline of what it did and when: start-up as marks, each interaction as
a span, and every line of log, from C++, Go and the pages on one clock. It prints it,
writes it to `timeline.log` in the profile and keeps it in memory for
`ProjectB.getTimeline`. Every feature logs through it: [logging.md](logging.md).

## Crashes

While the app runs, `crash.log` in the profile (`~/.project-b/default/` unless
`--user-data-dir` names another) starts with the process id (`core/crash`). A clean stop
deletes it, so one found at the next start means the last run crashed, whatever crashed;
a force quit counts too. Go renames it `crash-<yyyy-mm-dd-hhmmss>.log`, the time it was
last written, and the shell shows a notice over the page's top-left corner, wrapped so
both paths show whole: "Project B crashed last time. The log is at" that file, "macOS's
report at" macOS's report of that process, with a close button, which sends
`dismissCrash`. The snapshot carries both, as `crashLog` and `crashReport`.

- **Go**, from a panic or a fatal error in any goroutine, cgo callbacks and nil
  dereferences included. Go recovers none: a panic ends the app, since nothing would
  bring back its tabs yet. `ProjectB.panic` over DevTools forces one in a hook
  (`docs/devtools.md`): the runtime writes its report, with every goroutine's stack,
  into `crash.log` (`debug.SetCrashOutput`). A fault in C++ called from Go is reported
  there too, as "signal arrived during cgo execution".
- **Native, in the main process**: macOS writes a report with every thread's stack,
  symbolized, to `~/Library/Logs/DiagnosticReports/project_b-<time>.ips` (a run outside
  the desktop session gets `/Library/Logs/DiagnosticReports`). Go finds the one with the
  last run's pid. macOS writes it a few seconds after the crash and sometimes not at all
  (it stopped after about a dozen crashes in an hour here), so when there is none the
  notice names the folder. Nothing waits for it.
- **Child processes** (renderers, GPU, utility): `engine/browser/child_processes.cc` sends
  each one that ends other than normally to Go, which adds a line to `crashes.log` in the
  profile: time, type, how it ended, exit code, pid, and a renderer's page. It is kept,
  and brings up no notice. macOS writes a `project_b_helper` report for these as well.

Go's handlers for SIGSEGV, SIGBUS and SIGFPE are installed when the binary loads, before
`ContentMain`, which would reset them and, outside official builds, put Chromium's stack
dumper in their place. So the main process gives itself content's
`--disable-in-process-stack-traces` (`engine/project_b_main.cc`). Go then passes a fault
that isn't Go's on, as documented for c-archive, to the default action, and macOS reports
it. Content copies the switch to child processes, so a dev build's children no longer
print their stack to stderr; macOS's reports have it.

The driver forces each: `gocrash 0` to `3` (nil dereference on the calling thread, panic
there, nil dereference in a goroutine, nil dereference in C called from Go), `segv` (C++
on the main thread), and typing `chrome://crash`, `chrome://gpucrash` or
`chrome://inducebrowsercrashforrealz`. Dev mode shows the notice too: `kill -QUIT` the
dev server and start it again on the same `--profile`.

## Not in the first app

- Gates cover the top-level navigation only: not iframes, subresources, WebSockets or
  WebTransport. `isAllowedUrl` doesn't see those yet.
- Gates answer immediately; holding a navigation for a prompt isn't built.
- A back or forward restored from the back-forward cache skips the gates, because content
  runs no navigation start for it. A page allowed earlier stays reachable that way.
- No popup blocking: every `window.open` becomes a tab, in the opener's window. No window
  features (size, position) are honoured.
- No spaces, groups, pinned tabs, tab search, drag to reorder, or session restore. Tabs
  are not saved; the prefs, the tab layout, the frame colour and the theme, are, in
  `prefs.json` in the profile. Automatic updates are the app's: [updates.md](updates.md).
- No downloads, permissions, dialogs, autofill or extensions. An installed copy does update
  itself: [updates.md](updates.md).
- Content's own `chrome://` pages, such as `chrome://gpu`, pass the gate but fail to
  load: `project_b.pak` doesn't carry their resources.
- Mac only. Windows needs checking, because Go's cgo there wants mingw.

## What the first run showed (2026-11-09)

Measured on the dev Mac Studio (M3 Ultra, 32 cores), Chromium 155.0.8059.41, build `out/pb`:
non-official with `dcheck_always_on = true`, so runtime numbers are pessimistic.

The Claude session that ran this has no GUI session (`CGSessionCopyCurrentDictionary`
returns nothing), so the window exists but is never on screen. `tools/drive_first_app.py`
therefore calls the window's own `hitTest:`, sets first responder and hands events to the
view directly, because `[window sendEvent:]` delivers nothing there. It also forces the
window to report as key. Screenshots are real content surfaces drawn together by the
driver, not read back from the window.

| What | Result |
|---|---|
| Full build | 3,192 s (53 min), about 37,800 steps, 32 cores |
| Adapter change to running | 5 to 31 s |
| Library / `project_b` / `project_b_helper`, unstripped, decimal MB | 415 / 2.6 / 0.03, `symbol_level = 0` |
| Go symbols in main / helper / library | 1,911 / 0 / 0 |
| Content symbols in main / helper | 0 / 0 |
| cgo call, C++ to Go | 34 to 54 ns (bare); a `hook` crossing with no values is 120 ns |
| Forced full GC blocking the main thread | median 181 us, max 743 us, 20 runs |
| First paint, process start to shell paint | 1.05 to 1.5 s at best; up to 10.6 s from the OS issue below. Measured since from the timeline: `tools/measure.py` |
| Key press to next frames, inside the shell | median 8.6 to 12.1 ms, max about 33 ms, 26 keys |
| Threads / RSS, main | 27 to 31 / 140 to 149 MB |
| Threads / RSS, GPU | 14 to 19 / 97 to 106 MB |
| Threads / RSS, network | 11 / 66 to 71 MB |
| Threads / RSS, shell renderer | 25 / 153 MB |
| Threads / RSS, page renderer (example.com) | 25 / 152 MB |
| Threads / RSS, spare renderer | 10 / 90 MB |
| Page frame right immediately after resize | Yes, 3 of 3; stale shell reports ignored in a burst of 30: 2 to 3 |
| GPU process CPU over 10 s, shell shown / hidden | 2.05, 2.12 s / 1.77, 2.20 s, animated page, window off screen |

### Findings

- **Slow first paint here is the OS.** A sample shows the main thread in
  `+[NSTextInputContext initialize]`, in a synchronous XPC call to macOS's ViewBridge
  service, which doesn't answer without a GUI session. It should be measured again on a
  real desktop.
- **A Go nil dereference killed the whole app.** Chromium's
  `EnableInProcessStackDumping` replaced Go's SIGSEGV handler, so it never became a Go
  panic. The main process now turns that dumper off; see "Crashes" above.
- **Sharing content took about 270 MB out of the app.** With two full binaries, the
  installed app was 572 MB and the DMG 153 MB. With the library they're 305 MB and 82 MB.
- **Sharing doesn't change what each process reports.** RSS and dirty footprint are the
  same either way, because RSS counts shared pages in every process. The main process keeps
  39 MB of the app's code resident, and that used to be a separate copy, so about 39 MB is
  the most physical memory this can save. The real saving wasn't measured.
- **Content has a native screen requirement** the reference example skips: on Mac without
  `display::ScopedNativeScreen`, creating the first view hits a CHECK.

### Not tested

- Anything on a real display: compositing of the transparent shell, resize tearing as seen
  by eye, drag and drop, real GPU and battery cost, a real video, screen readers.
- AppKit's own event dispatch: real mouse, real keyboard, input methods.
- Crashpad and official builds. Crashpad isn't in this build.
- Double-clicking the app. `open` fails from a session with no display, and fails the same
  way for Calculator. The 39 checks start the binary inside the mounted DMG directly.
- Windows and Linux.

## What the tabs run showed (2026-11-09)

Same machine and build. 84 real-app checks and 67 mock checks pass; the packaged app in
the DMG passes the same 84.

- **Closing a tab from its own keyboard event crashes** if the `WebContents` dies right
  there: Cmd+W arrives through the tab's `PreHandleKeyboardEvent`, and content keeps using
  the `RenderWidgetHost` after we return. The adapter takes the tab out of its map at once
  and deletes it on a later task (`DeleteSoon`), and posts the window close the same way.
- **A tab must stop observing its `WebContents` before destroying it.** Teardown finishes
  pending navigations, which calls `DidFinishNavigation` on the half-destroyed observer.
- **A popup's first navigation goes through the gate.** Content calls `AddNewContents`
  before it loads the new `WebContents`, so Go knows the tab when the throttle asks.
- **Focus has a gap after a tab closes**: nothing has the keyboard until the shell hears
  the new tab list and sends `focusTab` or `focusShell` back, one round trip. A second
  Cmd+W inside that gap does nothing.
- **Shell rectangles are a frame or two stale after a layout change**, plus the hop to the
  main process. A click 30 ms after Cmd+T hit the strip's old empty space and dragged
  instead. A person can't click that fast; the driver can, and waits.
- **The mock could not show compositing bugs.** An opaque background on the shell's root
  painted over the page hole and looked fine over the mock's iframe. Only the real
  driver's composite caught it. The mock's window now holds the shell in a frame over the
  page's, as the real window holds its views, so an opaque shell hides the page there too.
- Favicons: content hands over the candidates and downloads one; the adapter picks the
  plain icon declared smallest, encodes a PNG and sends a data URL. iana.org shows its
  icon; example.com, whose `/favicon.ico` is a 404, shows none.

## Packaging

`tools/package.sh` makes `build/project-b.dmg`: `Project B.app` with the two binaries in
`Contents/MacOS`, the library in `Contents/Frameworks`, its data files inside, and a link to
Applications. Inside an app, Chromium looks under the "framework bundle". We have no
framework, so `MainDelegate` points it at `Contents`, which gives `Contents/Resources` and
`Contents/Libraries`.

The app is signed with our own key (`tools/signing-setup.sh`), not notarized. Gatekeeper
blocks a copy downloaded by a browser until its quarantine flag is removed; `curl` doesn't
set the flag. The script also makes a profile a Mac can install to allow that key, and
whether it gets a browser download past Gatekeeper on a real Mac isn't confirmed yet.

Sizes in decimal MB, for arm64:

| Binary | File | Code (`__TEXT`) | Symbols (`__LINKEDIT`) |
|---|---:|---:|---:|
| Library, `out/pb` | 415 | 258 | 149 |
| Library, stripped, in the DMG | 270 | | |
| `project_b`, stripped, in the DMG | 2.3 | | |
| `project_b_helper`, in the DMG | 0.04 | | |
| Ulaa Framework 155, official, a whole browser | 249 | 234 | 2 |
| Chrome for Testing 153 framework | 246 | | |

`out/pb` isn't a debug build, but a non-official build keeps its symbols, has DCHECKs on and
skips link-time optimisation. The DMG is 82 MB. The installed app is 305 MB against 376 MB
for Chrome for Testing.

## Commands

    tools/fetch-chromium.sh                # fresh Chromium 155, once
    tools/build.sh                         # the library and both binaries, out/pb
    RELEASE=1 OUT=out/rel tools/build.sh   # official-style build, takes hours
    tools/package.sh                       # build/project-b.dmg; OUT=out/rel for the release
    python3 tools/drive_first_app.py       # real app, results in build/real (7 screenshots); PB_BIN picks another binary
    python3 tools/measure_first_app.py     # memory, boundary cost and overlay cost above
    python3 tools/measure.py               # start-up and responsiveness, from the app's timeline (logging.md)
    (cd core && go test ./...)             # managers, no C++
    node shell/e2e/first-app.cjs           # shell on the mock engine, in a browser
    (cd core && go run ./cmd/dev)          # try it on the mock: http://127.0.0.1:8765

Run the real app:

    build/pb/project_b --block=blocked.test
