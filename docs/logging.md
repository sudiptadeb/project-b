# Logging and timing

One timeline holds everything the app says about itself: what happened, and when. Every
feature logs through it, in C++, Go and JavaScript. This page is the guide to follow.

## The model

- **An entry** is one of three things: a **mark** (a moment, `shell.mounted`), the **begin**
  or **end** of a **span** (an interaction, `settings.open`), or a **line of log** at `info`,
  `warn` or `error`.
- **One time**: nanoseconds since the process started, on the system's monotonic clock.
- **One manager**, in Go (`core/timeline`). C++ and our pages hand it their entries.
- **Three places**, all fed by the same entry: printed (stderr on a Mac, `adb logcat -s
  ProjectB` on Android), written to `timeline.log` in the profile, and kept in memory for
  scripts.

A line, printed or in the file: milliseconds since process start to the nanosecond, the
kind, who recorded it (`go`, `engine`, `shell`, `settings`), then the name or the text.

      151.007416 mark  engine   shell.created
      165.941083 mark  go       go.started
      281.694000 mark  shell    shell.script
     1920.300000 begin shell    settings.open
     1922.372458 mark  engine   settings.open.shown
     2148.429000 end   settings settings.open 228.129000ms
     2301.118291 warn  go       prefs: open prefs.json: permission denied

## How to log, in each language

| | A moment | A span | A line of log |
|---|---|---|---|
| Go, `projectb/timeline` | `timeline.Mark("go.started")` | `timeline.Begin("settings.open")`, `timeline.End(...)` | `timeline.Infof`, `Warnf`, `Errorf` |
| C++, `engine/browser/timeline.h` | `timeline::Mark("window.created")`, `MarkAt(name, ticks)` for a time content measured | `timeline::Begin`, `timeline::End` | `LOG(INFO)`, `LOG(WARNING)`, `LOG(ERROR)`, as anywhere in Chromium |
| JS, `shell/src/timeline.js`, through the page's bridge | `mark('shell.mounted')` | `begin(...)`, `end(...)` | `info(text)`, `warn(text)`, `error(text)` |

Nothing else: no `log.Printf`, no `fmt.Fprintln(os.Stderr, ...)`, no `console.log`, no
`NSLog`, no file of a feature's own. Go's standard `log` still lands here, at `info`, for
libraries that use it.

## What to log, and at which level

- **A mark** for each stage a person would wait on: something created, loaded, committed,
  painted, on screen. If it can be slow, it gets a mark. Nothing that fires per frame, per
  key or per request.
- **A span** for each thing the user asks for that ends in something they see. It begins
  where the request is first seen, the click in the page or the action in Go, and ends when
  the result is drawn. Marks between the two ends are its stages.
- **`error`**: something failed and the user loses something by it: a file that wouldn't
  save, an update that didn't apply.
- **`warn`**: something was wrong and we carried on: a message we couldn't read, a command
  nobody knows.
- **`info`**: a decision worth knowing later, once per event, not per item: "update: 0.2.26
  is staged". If it would print on every navigation, it isn't `info`; leave it out.

## Names

- Lowercase, dots between parts, underscores within one: `area.event`. The area is the thing
  it is about: `process`, `main`, `content`, `go`, `window`, `shell`, `settings`, `tab`,
  `tabs`. `shell.first_paint`, `tab.committed`.
- A span is `area.verb`: `settings.open`, `tab.new`, `tab.close`, `tabs.layout`. A stage
  that only exists inside a span carries its name: `settings.open.shown`, `tab.new.done`.
- A mark's text is for a small detail, `tab=3`. A line of log starts with its package or
  feature: `prefs: ...`, `update: ...`. C++ lines get their file and line by themselves.
- Only one span of a name is open at a time, so no ids are passed around. A second begin
  while it is open is dropped: the shell begins `settings.open` at the click, and Go's
  begin for the same request does nothing; from the menu, Go's is the first.

## What never goes in

The file stays on disk and people send it to us. So:

- **No URLs**, with or without credentials, and no hosts. A tab is `tab=3`.
- **No page content**: titles, text, form values, console messages, cookies, headers.
- **No secrets and nothing typed**: tokens, passwords, what is in the address bar.
- **No personal data**: names, emails, account ids.
- A path inside the profile is fine. A path the user picked is not.

Chromium's own `LOG` lines are not ours to vouch for, so they stay on stderr as before
and never reach the file or memory: only lines from `engine/` do.

## The clock

The timeline's clock is Go's monotonic clock, `time.Since` a fixed origin. Go's runtime
and Chromium's `base::TimeTicks` read the same system counter, `mach_absolute_time` on a
Mac and `CLOCK_MONOTONIC` on Android, so the three languages map onto it exactly:

| Who | How its time gets on the timeline | Resolution |
|---|---|---|
| Go | Read as the entry is recorded | The counter's: 42 ns on Apple silicon |
| C++, `Mark`, `Begin`, `End`, `LOG` | Go reads the clock when the call arrives, about 100 ns later | The same |
| C++, `MarkAt(ticks)` | `ticks` minus the origin as `TimeTicks`, which the engine works out once at start by reading Go's clock between two readings of its own | 1 µs, `TimeTicks`' own |
| JS | The page sends `performance.now()` with each entry. Blink counts that from the navigation's start as a `TimeTicks`, which the engine tells Go when the page commits (`timeline::PageCommitted`). So a JS entry has the time it happened, not the time its message arrived | 100 µs, Blink's limit for a page; a frame's time on screen is given to 4 ms |

Time zero is the process's start: its creation time from the kernel on a Mac, to the
microsecond; on Android the time Android started it, to the millisecond. `main.entered`
is the first thing our code does, so the time before it is the loader's.

On the mock engine the dev server plays the engine: the page's `performance.timeOrigin`,
its navigation start in wall time, goes to Go before anything the page sends
(`core/cmd/dev/webui.js`), and Go places it on the timeline (`timeline.FromWall`).

Every window's shell is the page `shell` on the timeline, and every settings page the page
`settings`, so with several windows the one that started last sets that page's clock.

Entries are numbered in the order they were recorded. A page's entries arrive a little
after they happened, so sort by time when reading.

## Where it goes

- **Memory**: the last 16,384 entries. `ProjectB.getTimeline {since}` over DevTools
  ([devtools.md](devtools.md)) and the driver's `timeline <since>` answer
  `{entries, last}`; pass `last` back as `since` to get only what is new.
- **The file**: `timeline.log` in the profile (`~/.project-b/default/`, or
  `--user-data-dir`; on Android the app's `app_project_b/profile`). Each start begins a new
  one and keeps the one before as `timeline.previous.log`; so does a file passing 8 MB.
- **Printed**: stderr, and Android's logcat under the tag `ProjectB`.

Recording takes a lock, reads the clock, and puts the entry in memory and in a queue. One
goroutine writes the queue out, so no thread waits for a disk or a terminal. If entries
come faster than they can be written, the file says how many it missed; memory has them
all. After a crash the file can lack the last few entries; `crash.log` has the stack.

## Start-up, as marks

In the order they happen on a Mac:

| Mark | From | What has happened |
|---|---|---|
| `process.start` | | Time zero |
| `main.entered` | C++ | `ProjectBMain` runs: the binary and the library are loaded, Go's runtime is up |
| `content.ready` | C++ | Content is initialised and calls our `PreMainMessageLoopRun` |
| `go.starting` | Go | Go's managers are being built |
| `window.created` | C++ | Go asked for its first window, and the native window exists |
| `shell.created` | C++ | The window's shell `WebContents` exists |
| `shell.navigation_start` | C++ | Its load of `chrome://shell/` began: the page's time zero |
| `window.shown` | C++ | The window is on screen, in the frame colour |
| `go.started` | Go | Go's managers are built and the first tab is asked for |
| `shell.committed` | C++ | The shell's renderer has the document |
| `shell.script` | JS | The shell's script runs |
| `shell.mounted` | JS | Vue has mounted the app |
| `shell.ready` | Go | Go heard the shell and sent it the snapshot |
| `shell.snapshot` | JS | The shell has its first state from Go |
| `shell.first_paint` | JS | The renderer first painted the shell's content |
| `shell.on_screen` | JS | That frame reached the screen: the compositor's presentation time |
| `tab.committed`, `tab.first_paint` | C++ | Any tab's page committed; first painted something. `tab=1` is the first tab |

The shell's first paint and the first tab race the lines around them. Android has no
`window.shown`: its window is on screen before content starts.

## Interactions, as spans

| Span | Begins | Stages | Ends |
|---|---|---|---|
| `settings.open` | The click on the gear, or Go's action from the menu or a key | `.go` Go has it; `.created` the page's `WebContents` exists; `.shown` it is over the window; then the page's own `settings.navigation_start`, `settings.committed`, `settings.script`, `settings.mounted`, `settings.ready`, `settings.snapshot`, `settings.first_paint` | `settings.on_screen`: its first frame is on screen |
| `tab.new` | The click on New Tab, or Go's action | `.go`; `.done` the tab is made, shown, and the shell told | The shell has drawn it |
| `tab.close` | The click on a tab's close button, or Go's action | `.go`; `.done` | The shell has drawn it |
| `tab.activate` | The click on a tab | `.go`; `.done` | The shell has drawn it |
| `tabs.layout` | The click on the layout button, or Go's action | `.go`; `.done`; `.page_moved` the shell measured where the page now goes and Go placed it | The shell has drawn after that |

"The shell has drawn it" is the renderer finishing the frame (`painted` in
`shell/src/timeline.js`). That frame reaches the screen a frame or so later, and nothing
tells a page when: only a page's first paint has its time on screen. On Android,
`settings.open.overlay_on_screen` is also marked: when the window's first frame with the
settings page laid over it reached the screen, whatever that page had drawn by then.

## Input: the OS's lag or ours

Each click and key press is a span of its own, on a Mac, so a late input can be told from a
slow answer:

| Span | Begins | Stage | Ends |
|---|---|---|---|
| `input.click` | When the OS made the mouse-up: the `NSEvent`'s own timestamp, which counts on our clock | `.window`: the application's `sendEvent:` has it | The shell or the settings page has it as a DOM event |
| `input.key` | When the OS made the key-down | `.window` | Go has the key, before any page does |

The interaction's span, such as `tabs.layout`, begins where the input's ends. Read the two
together:

    5792.469 input.click        2.927 ms  .window 1.466
    5796.196 tab.new           29.100 ms  .go 0.279  .done 15.870

- **Begin to `.window`** is the OS's: the event waited that long before the app saw it. In
  a VM that lags, it is here.
- **`.window` to the end** is ours: the window, content and the renderer passing it on.
- **The interaction's span** is ours too: Go, the engine, and the shell drawing.

`python3 tools/timeline.py <profile>/timeline.log` prints exactly these lines for any run,
a VM's included; `tools/measure.py` gives their medians, though its clicks are the
driver's and are made inside the app, so its begin to `.window` is always near zero. Only
clicks the shell or the settings page gets are tracked, and not key repeats.

To add a span ended by the shell: wrap what Go does in `Shell.Interaction(name, func)`,
and add the command that starts it to `spans` in `shell/src/bridge.js`.

## Measuring

    python3 tools/measure.py                  # 10 starts: median and spread of every milestone and span
    python3 tools/measure.py --compare        # against the baseline in tools/baseline/
    python3 tools/measure.py --save           # write that baseline, for a release to be compared with
    python3 tools/measure.py --android        # the emulator, read from logcat; ADB_SERIAL picks the device
    python3 tools/measure.py --trace t.json   # the last start, for ui.perfetto.dev
    python3 tools/timeline.py timeline.log t.json   # the same from anybody's file
    python3 tools/timeline.py timeline.log          # every span in the file, with its stages

Each start is on a fresh profile; it opens settings, opens, shows and closes tabs, and
moves the tabs there and back, by clicks and keys. There are three baselines, each compared with its own
kind: `mac.json`, from the desktop session of a person logged in (`tools/on_screen.py
python3 tools/measure.py`), `mac-headless.json`, from an agent's or ssh session, which has
no screen, and `android.json`, from the emulator. On the build Mac the two Mac ones agree
to within a few milliseconds; AppKit has at times taken seconds to start without a screen,
which is why they are kept apart. Each file says the version, the commit and the machine.

`tools/test.sh app` checks the timeline itself: the milestones in order on one clock, a
span for each interaction, and the file agreeing with memory.
