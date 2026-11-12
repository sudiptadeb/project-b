# DevTools remote debugging

The app opens Chrome's DevTools endpoint the way Chrome 155 does, so tools built for
Chrome drive it unchanged: Google's ChromeDriver, Puppeteer, anything speaking CDP. We
invent nothing here; where we differ from Chrome it is said below, with the reason.
`tools/drive_chromedriver.py` proves it with ChromeDriver 155.0.8059.41.

## Switches

| Switch | What it does | Chrome's source |
|---|---|---|
| `--user-data-dir=<dir>` | The profile folder. `--profile` is our older name for it and still works. | `chrome_paths` |
| `--remote-debugging-port=N` | Listens on `127.0.0.1:N`, or `[::1]:N` if that fails. `0` lets the system pick and writes `DevToolsActivePort` (port, then the browser target's path) into the profile. Prints `DevTools listening on ws://...` on stderr. | `chrome/browser/devtools/remote_debugging_server.cc` |
| `--remote-debugging-pipe` | CDP over fds 3 and 4. The app closes when the pipe does. | same |
| `--remote-allow-origins=<list>` | Which `Origin` headers may open a WebSocket. Content checks it, and the `Host` header too. | `content/browser/devtools/devtools_http_handler.cc` |

Without either debugging switch nothing listens, in every build, as in Chrome. With
one, it opens on any profile, the default `~/.project-b/default` included. Chrome 136
and later refuse the default profile; we add no check of our own (Ulaa doesn't refuse
it either, below).

`GetProduct` answers `Chrome/155.0.8059.41`, as Chrome does. `/json/version` and
`Browser.getVersion` show it and ChromeDriver reads the version from it. It also reaches
`chrome://gpu` and the GPU process. It is not the user agent: pages still get
`GetUserAgent`'s.

## Targets

- **Tabs** are what Chrome's tabs are: a `tab` target over a `page` target.
- **The shell** is no tab (`ShouldReportAsTabTarget` false) and its type is `other`,
  so ChromeDriver, which drives `page` targets, never picks it. It is otherwise an
  ordinary target: listed, and attachable like any frame. Chrome treats the frame of
  its own WebUI browser the same way, under the type `browser_ui`.
- **New targets**: `Target.createTarget` and `/json/new` open a tab the way a link with
  `target=_blank` does: the adapter makes it, Go hears of it before its first navigation
  (so the URL gate applies), shows it, and it loads. It opens in the window in front,
  and `newWindow: true` also opens a tab there. `background` is ignored and the tab comes to the
  front, as in content_shell; Chrome handles `background` in its own `Target` handler.
- **Close and activate**: `Target.closeTarget` runs the page's unload and then asks the
  tab's delegate to close it; `Target.activateTarget` asks it to show the tab (Chrome's
  `ChromeDevToolsManagerDelegate::Activate`). Both cross to Go's tab manager
  (`core/content/wire.go`), which decides. A page's own `window.close()` takes the same
  path, which closes the tab where Chrome would.

## The Browser domain

Content leaves four commands to the embedder. They are generated from Blink's protocol
the way content_shell's are (`engine/browser/protocol_config.json`) and answered as
Chrome's `chrome/browser/devtools/protocol/browser_handler.cc` answers them, in
`engine/browser/devtools_session.cc`:

| Command | Answer |
|---|---|
| `Browser.getWindowForTarget` | the id Go gave the window the tab shows in, and its bounds; none for a tab no window has shown yet |
| `Browser.getWindowBounds` | the frame in screen points, y down from the top of the main screen, and `normal`, `minimized`, `maximized` (AppKit's zoomed) or `fullscreen` |
| `Browser.setWindowBounds` | Chrome's rules: a state can't be combined with a rectangle, and `normal` first leaves full screen, the Dock or zoom |
| `Browser.close` | quits the app on a later task, every window at once |

## Our own commands

Commands named `ProjectB.*` are Go's. `devtools_session.cc` hands each one's method and
params, as JSON, to Go (`PB_EVENT_DEVTOOLS_COMMAND`), and Go's answer is the reply.
They let a test see and do what only Go knows. Any client of the port sends them as it
sends content's; ChromeDriver passes them through `goog:cdp`.

| Command | Answer |
|---|---|
| `ProjectB.getTabs` | `{tabs, active}`: the tabs as Go's tab manager has them, and the shown one's id |
| `ProjectB.isAllowedURL` `{url}` | `{allowed}`: whether policy lets a tab load it |
| `ProjectB.getTimeline` `{since}` | `{entries, last}`: the app's timeline as kept in memory ([logging.md](logging.md)), after the entry numbered `since`, or all of it. Each entry has `seq`, `t` in nanoseconds since process start, `kind`, `src`, and `name`, `text` or `dur` |
| `ProjectB.moveTab` `{tab, window, index}` | none: Go moves the tab to that place in the window's list, or with `window` 0 into a new window of its own ([windows.md](windows.md)) |
| `ProjectB.getWindows` | `{windows}`: each window's `id`, `bounds` (its frame on screen, y down from the top of the main screen), `state` (`normal`, `minimised`, `zoomed` or `fullScreen`) and `front` |
| `ProjectB.setWindowBounds` `{window, bounds}` | none: Go asks for the window's frame there |
| `ProjectB.panic` | none: it panics inside Go's hook, as a bug would, and the app crashes (see "Crashes" in `first-app.md`) |

A method Go doesn't have falls through to content, which says it wasn't found. The
commands live in `core/app/devtools.go`; adding one is a Go change only. They need no
switch of their own: the port is closed unless the app is started with it, and a client
of it can already drive the whole browser and crash it (`Browser.crash`).

## Not here

- Separate browser contexts (`Target.createBrowserContext`, Playwright's normal launch),
  and a new window: `newWindow` opens a tab ([windows.md](windows.md), step 6).
- `Browser.setContentsSize`, `setDockTile`, `executeBrowserCommand`.
- A page's own full screen (`requestFullscreen`), which ChromeDriver's Fullscreen Window
  uses on a non-headless Chrome: no tab hook takes the window to full screen yet.
- Chrome's refusal of the default profile, its enterprise policy
  `RemoteDebuggingAllowed`, the `chrome://inspect` approval mode, and tethering. Chrome also ignores `--remote-debugging-address` outside headless,
  and so do we.

## Ulaa

Ulaa 155's `remote_debugging_server.cc` and `GetProduct` are Chrome's, unchanged. It
builds with Chromium branding, though, so `GOOGLE_CHROME_BRANDING` is off and Ulaa does
not refuse the default profile; neither do we.
