# The first app

One box, an address bar on top, opening empty. The user types, and a suggestion box
fed by real history extends below the bar, over the page.

## Pieces

| Folder | What it is | Needs Chromium |
|---|---|---|
| `core/` | Go managers and the engine contract | No |
| `core/mockengine`, `core/cmd/dev` | Mock engine and dev server | No |
| `shell/` | Vue 3 shell | No |
| `core/cmd/lib` | The cgo layer: the Go core as a c-archive | Links into the main binary |
| `engine/` | C++ adapter on Chromium's content layer | Yes |

## Two binaries

- `project_b` is the main process: content plus Go, linked statically.
- `project_b_helper` runs every child: the same content, with no Go.

Go's runtime starts by itself when a binary linked with it loads, so "linked but
unused" isn't possible in a child. Content asks `GetChildProcessPath` which program to
launch, and we answer with the helper.

## The boundary

`engine/abi/pb_abi.h` is plain C, ids and strings. Go calls C++ through a function table
handed over at start, and C++ calls Go through exported functions
(`engine/abi/pb_exports.h`). Every call is on the main thread, except `post_wakeup`.
Mojo and content's types stay inside the adapter.

## The window

Two stacked views: the page at the bottom, a full-window transparent shell on top.
Shell components mark themselves with `v-shell` and report their rectangles, and
`<PageView>` reports where the page goes. `PBContainerView hitTest:` asks one question:
is this point in a shell rectangle? Content's own views ask the same `hitTest:` before
accepting a mouse event, so nothing routes events by hand.

The page view keeps its margins through autoresizing, so it follows a window resize in
the same frame. A page position measured for an older window size is ignored, because a
fresher one follows.

## Not in the first app

- Gates cover the top-level navigation only: not iframes, subresources, WebSockets or
  WebTransport. `isAllowedUrl` doesn't see those yet.
- Gates answer immediately; holding a navigation for a prompt isn't built.
- One tab. `target=_blank` and `window.open` don't open anything.
- No downloads, permissions, dialogs, autofill or extensions.
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
| `project_b` / `project_b_helper` | 399 MB / 396 MB, `symbol_level = 0` |
| Go symbols in main / helper | 1,889 / 0 |
| cgo call, C++ to Go | 34 to 51 ns |
| Forced full GC blocking the main thread | median 181 us, max 743 us, 20 runs |
| First paint, process start to shell paint | 1.05 to 1.5 s at best; up to 10.6 s from the OS issue below |
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
- **A Go nil dereference kills the whole app by default.** Chromium's
  `EnableInProcessStackDumping` replaces Go's SIGSEGV handler, so it never becomes a Go
  panic. With `--disable-in-process-stack-traces` it recovers normally. Content only
  enables that dumper in non-official builds, and what Crashpad does in an official build
  isn't tested. A C++ crash on the main thread is still caught and dumped by Chromium.
- **Two binaries cost about 400 MB of duplicated code.** Sharing it would take a shared
  library, which we chose not to use for Go but could use for content alone.
- **Content has a native screen requirement** the reference example skips: on Mac without
  `display::ScopedNativeScreen`, creating the first view hits a CHECK.

### Not tested

- Anything on a real display: compositing of the transparent shell, resize tearing as seen
  by eye, drag and drop, real GPU and battery cost, a real video, screen readers.
- AppKit's own event dispatch: real mouse, real keyboard, input methods.
- Crashpad and official builds. Crashpad isn't in this build.
- Windows and Linux.

## Commands

    tools/fetch-chromium.sh                # fresh Chromium 155, once
    tools/build.sh                         # both binaries, out/pb
    python3 tools/drive_first_app.py       # real app, results in build/real
    python3 tools/measure_first_app.py     # the numbers above
    (cd core && go test ./...)             # managers, no C++
    node shell/e2e/first-app.cjs           # shell on the mock engine, in a browser
    (cd core && go run ./cmd/dev)          # try it on the mock: http://127.0.0.1:8765

Run the real app:

    chromium/src/out/pb/project_b --block=blocked.test
