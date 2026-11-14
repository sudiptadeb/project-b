# Benchmarks

Where start-up time and memory go, per shell and against a bare native Mac app, and the
tools that measure them. `tools/measure.py` (docs/logging.md) stays the regression check
against a baseline; these are the deeper looks, run by hand when a question needs them.

| Tool | What it gives |
|---|---|
| `python3 tools/measure.py --shell <id>` | The timeline's start-up milestones for any shell |
| `python3 tools/bench/startup_profile.py trace` | Chromium's startup trace: main-thread tasks, launches, the shell's navigation, stretches with no task |
| `python3 tools/bench/startup_profile.py cpu` | Instruments' CPU samples of the main thread in 20 ms buckets, and Go's calls into the engine |
| `python3 tools/bench/first_paint.py` | The start-up marks for a reused profile (every day) and a new one (first start), side by side |
| `python3 tools/bench/memory.py <shell>...` | Footprint per process kind, the shell's JS heap and element count |
| `python3 tools/bench/native.py` | The same marks and footprint for a bare AppKit app, plain and with a WKWebView |

Point `PB_BIN` at the package (`build/package/Project B.app/Contents/MacOS/project_b`)
for what a user gets. The CPU view needs symbols, so it runs `chromium/src/out/pb/project_b`.
Run from a person's desktop session where you can (`deskrig run -- ...`): an agent's session
has no screen, and AppKit can be slower there. Measure on a quiet machine: with emulators, the
VM or a build running, `uptime`'s load runs past 20 and every mark moves later, Go's start
among them, so compare only numbers taken under the same load.

## Results, 0.2.50, 2026-11-13

`minimal-svelte` here is the shell now called Minimal (`minimal`).

On a Mac Studio (M3 Ultra), from an agent's session; ms from process start, medians of starts
after a warm-up, each on a new profile with only the shell set in its prefs.

### Start-up per shell

| | standard (Vue) | minimal-svelte | minimal-js | terminal | fox |
|---|---|---|---|---|---|
| go.started | 162 | 159 | 165 | 158 | 164 |
| shell.committed | 221 | 216 | 220 | 216 | 218 |
| shell.ready | 234 | 226 | 224 | 223 | 232 |
| shell.first_paint | 256 | 252 | 251 | 240 | 240 |
| shell.on_screen | 321 | 323 | 307 | 296 | 296 |
| Shipped JS | 96 KB | 61 KB | 20 KB | 93 KB | 95 KB |

Every shell commits at about 220 ms and spends 25 to 35 ms more to its first paint. The
framework is not where the time goes.

### Before the shell runs

| ms | Main thread |
|---|---|
| 0 to 30 | dyld loads the engine library; Go's runtime starts |
| 30 to 80 | Chromium and content initialise; NSApplication init about 13 ms |
| 80 to 125 | Go's start, synchronous: the NSWindow is created, 35 to 40 ms of AppKit (title bar and toolbar layout). Only then the shell page and its navigation, so the renderer launch starts here |
| 125 to 168 | Showing the window about 12 ms, the Dock colour, menus, the managers. The shell's request, posted at about 127, waits for Go to return |
| 168 to 220 | The run loop starts; macOS's launch step (finishLaunching, the first Core Animation commit, activation) runs no Chromium task for about 50 ms, though the shell's renderer has been up since about 170 |
| about 222 | The request is served from disk in under 1 ms; the shell commits |

Launching processes is cheap (each spawn under 1 ms; a renderer reaches its main thread
about 45 ms after); the main thread's order of work is the cost.

### Against a bare native app

| | Bare AppKit | AppKit and WKWebView | Project B |
|---|---|---|---|
| main() | 26 | 36 | 28 |
| Window created | 92 | 107 | 132 |
| Window shown | 97 | 161 | 153 |
| macOS launch step done | 134 | 202 | about 220 |
| First paint | 141 (frame out at 146) | 354 | 256 |

The NSWindow (about 37 ms) and the launch step (about 37 ms) cost a bare app the same: they
are the floor. We are about 110 ms above a bare app's first frame and about 100 ms ahead
of a WKWebView app's first paint.

### Memory, 5 s after start

| MiB | standard | minimal-svelte | minimal-js | terminal | fox |
|---|---|---|---|---|---|
| Browser | 33 | 33 | 33 | 33 | 33 |
| GPU | 28.5 | 28 | 28.5 | 29.5 | 30 |
| Renderers (3) | 43 | 43 | 41 | 41 | 42 |
| Network | 6.5 | 6.5 | 6.5 | 6.5 | 6.5 |
| **Total** | **111** | **111** | **109** | **110** | **111** |
| Shell JS heap | 1.5 | 1.2 | 0.8 | 1.5 | 1.5 |
| Shell elements | 68 | 37 | 44 | 64 | 57 |

A bare AppKit app is 15 MiB. The WKWebView app is 54: the app 23, WebContent 13, GPU 13,
Networking 5. Our extra is the three renderers at start (about 19, 12 and 10 MiB; the
third is likely Chromium's spare renderer, not yet confirmed), a GPU process twice
WebKit's, and the browser process 10 MiB over WebKit's app.

## Ideas, not yet tried

- Start the shell's navigation before creating the NSWindow, so its renderer boots while
  AppKit builds the window.
- Return from Go's start sooner and post showing the window and the rest, so macOS's
  launch step runs while the renderer boots rather than after.
- Together, an estimated first paint near 180 to 200 ms.
- Find out what the third renderer is and whether it can start later, about 10 MiB.
- Break down the GPU process's 28 MiB.
