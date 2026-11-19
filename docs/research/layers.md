# Three layers: wrapper, Go core, engine

Agreed with Sudipta on 2026-11-19, while discussing other platforms
([platforms.md](platforms.md)). This is the brief for the work; the session building it
turns it into its own feature doc and keeps that up to date.

## The model

Three pieces on every platform, read the same way on each:

- **The wrapper** starts the app and nothing else: `main.cc` on the desktops, Java on
  Android, an `main.mm` on iOS one day. In his words: "it should just be start gocore start
  content and thats it".
- **The Go core**: the managers and the shell. Some managers talk to the engine, through
  `engine.Engine`; some talk to the OS, through a new Go interface, `core/system`, with a
  version per OS. Go decides everything.
- **The engine**: our adapter and Chromium content, one library. "engine only talks to go
  core": it never calls an OS manager, and it touches the OS only where Chromium's objects
  force it (the exceptions below).

Not Rust ("ok no rust for now"), and not a Go `main`: with a wrapper this thin, `main.cc`
stays and Chromium's build keeps making and packaging the app.

## 1. The thin wrapper

Today `ProjectBMain` (`engine/project_b_main.cc`) decides in C++, before `ContentMain`:
`--version`, claiming the profile (`PBClaimProfile`), the saved flags
(`PBLaunchSwitches`), and the switch that leaves crash signals to Go
(`--disable-in-process-stack-traces`). All of that moves into one Go call:

```c
int main(int argc, const char** argv) {
  char** args = PBInit(argc, argv);  // the Go core starts
  if (!args) return 0;               // it printed the version, or handed over and quits
  return ProjectBMain(args, &go);    // content starts
}
```

- `PBInit` returns the finished command line for content, or nothing when this copy should
  quit. Chromium's version for `--version` is stamped into Go at build time.
- `ProjectBMain` keeps only: init the command line, store Go's table and the copy folder,
  start the timeline, `ContentMain`. The helper's path is unchanged.
- Order to read: `main` → `PBInit` → content → `PBStart` → `app.Start`.
- Android: Java calls the same `PBInit` before content starts, in place of
  `browser_main_parts_android.cc` calling `claim_profile`, so both read the same from there.

## 2. `core/system`: the OS managers' door

`engine.Engine` (113 methods) carries calls that are really the OS's. They move to a
`system.System` interface with `_darwin`, `_android` (and later `_linux`, `_windows`)
implementations, and their C++ goes:

| Today on `engine.Engine` | C++ today | Moves to |
|---|---|---|
| `OpenFile`, `RevealFile` | `downloads_mac.mm` | `core/system` |
| `AskDefaultBrowser`, `IsDefaultBrowser` | `mac_app.mm` | `core/system` |
| `SetDockColor` | `mac_app.mm` / dock | `core/system` |
| `OpenFullDiskAccess` | none, already Go | `core/system` |
| `AskNotifications`, `NotificationsAllowed` | none, already Go | `core/system` |
| Posting a page's notification, and its click | `notifications_mac.mm` | adapter hands it to Go; Go's notifications go through `core/system`; a click comes back to Go, which tells the engine |
| Links and files the OS hands the app | `mac_app.mm` delegate | `core/system` registers for macOS's open-URL and open-document events itself (prove first; if it can't, this stays a fourth exception) |

Also behind `core/system`: what is per OS in Go today, `core/instance`,
`core/relaunch`, `core/update/bundle.go` and `core/cmd/lib/*_darwin.go` /
`*_android.go`, so a reader finds every OS call in one place.

- On the Mac some of it is Objective-C, in `.m` files beside the Go, built by cgo.
- macOS allows its UI only on the main thread: `core/system` hops there itself (as
  `nativeEngine.Post` does), so no caller can get it wrong.
- Tests: a fake `system.System` beside `core/mockengine`, so the core still runs with
  neither Chromium nor the OS.

## The exceptions: where the engine still touches the OS

Written down so OS code doesn't creep back into the adapter. Only these:

1. **The window** (`window_mac.mm`). It holds Chromium's page views and shares input with
   them. On Linux and Windows the OS calls are inside Chromium's own Aura and Ozone.
2. **Panels attached to a window**: print and Share. Save as and the folder chooser move
   to Chromium's own file dialog (`ui::SelectFileDialog`), as a page's file chooser
   already uses, so `downloads_mac.mm`'s panel and `choose_path_mac.mm` go and Linux and
   Windows get both for free (2026-11-19: "yes lets do that").
3. **The Mac menu bar, and quitting from the Dock or at logout**: shortcuts go through
   Chromium's key handling, and a quit has to end Chromium's loop. Mac only.

The clipboard stays in the engine: Chromium has it for every platform.

## 3. The shell inside the Go core

Today the built shell is loose files beside the app (`Contents/Resources/shell/` on the
Mac, `assets/shell/` in the APK), read by `engine/browser/shell_page.cc`. Instead:

- Go embeds the built shell (`go:embed`), and the engine asks Go for a file through one
  new call in `PBGoCalls`, where `shell_page.cc` reads the disk today.
- `tools/package.sh` and the APK stop copying `shell/`.
- The dev server (`core/cmd/dev`) keeps serving from disk for live reload.

The core is then one piece; the shell and the managers can't drift apart after an update.
Costs: a few MB in the binary, and a shell change relinks Go (seconds).

## 4. The build in three pieces

`engine/BUILD.gn` and the scripts name the three: the wrapper, `libcore` (Go and the
embedded shell), and the engine library. One packaging entry point takes the platform's
name. Mac and Android stay working throughout; Linux and Windows come later and plug in.

## Order of work

Each a commit or a few, each leaving the app working, the release run green:

1. The thin wrapper (`PBInit`) on the Mac, then Android.
2. `core/system` with the moved methods, Mac and Android, and its fake.
3. Notifications through Go.
4. The open-link events into `core/system` on the Mac, or the fourth exception, written
   down.
5. The shell embedded in the Go core.
6. The three-piece build and the one packaging entry point.

## Not in this work

Linux and Windows themselves: their system layers and the Aura window come after, from
[platforms.md](platforms.md).
