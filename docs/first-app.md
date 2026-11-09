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

`engine/abi/pb_abi.h` is plain C, ids and strings. Each side holds a table of the other's
functions: `PBEngineCalls` for Go calling C++, handed over at start, and `PBGoCalls` for
C++ calling Go, which `app/main.cc` fills from Go's exports (`engine/abi/pb_exports.h`).
So the library has no link-time reference to Go, and the helper has nothing to resolve.
Every call is on the main thread, except `post_wakeup`. Mojo and content's types stay
inside the adapter.

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
| Library / `project_b` / `project_b_helper`, unstripped, decimal MB | 415 / 2.6 / 0.03, `symbol_level = 0` |
| Go symbols in main / helper / library | 1,911 / 0 / 0 |
| Content symbols in main / helper | 0 / 0 |
| cgo call, C++ to Go | 34 to 54 ns |
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
    python3 tools/drive_first_app.py       # real app, results in build/real; PB_BIN picks another binary
    python3 tools/measure_first_app.py     # the numbers above
    (cd core && go test ./...)             # managers, no C++
    node shell/e2e/first-app.cjs           # shell on the mock engine, in a browser
    (cd core && go run ./cmd/dev)          # try it on the mock: http://127.0.0.1:8765

Run the real app:

    chromium/src/out/pb/project_b --block=blocked.test
