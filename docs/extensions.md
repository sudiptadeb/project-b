# Chrome extensions

Project B can run Chrome's Manifest V3 extensions. It is a build-time feature, off by default.
This is the design as built. The research behind it is in
`project-b-research/chrome-extensions.md` and `chrome-extensions-reassessment.md`.

    tools/fetch-extensions.sh                     # five real packages into .tools/extensions/
    tools/pack-test-extension.py                  # ours, packed into the same folder
    (cd core && go run ./cmd/extstore -dir ../.tools/extensions)    # the mock store, on 127.0.0.1:8766
    EXTENSIONS=1 OUT=out/ext tools/build.sh       # a build with extensions, in a folder of its own
    chromium/src/out/ext/project_b

Then press the puzzle piece in the toolbar, Manage extensions, set the store's address to
`http://127.0.0.1:8766`, and install.

What works today, in the real app: the test extension and uBlock Origin Lite install from the
mock store, with the prompt; uBlock Origin Lite blocks, and shows its count on its button;
content scripts, service workers, messaging and `chrome.storage` run; buttons pin and unpin;
popups open, size themselves and take clicks; Load unpacked and a local `.crx` install; and all
of it is there again after a restart.

## The layers

Three layers, and each knows only the one below it.

1. **The runtime is Chromium's `//extensions`, unmodified, inside `engine/`.** It loads
   extensions, runs their service workers and content scripts, and implements the APIs that are
   about the platform: `runtime`, messaging, `storage`, `scripting`, `declarativeNetRequest`,
   `webRequest`, `alarms`, `offscreen`, `permissions`, `i18n` and the rest of
   `extensions/browser/api`. Our embedder for it is `engine/extensions/`.
2. **The APIs about the browser itself are answered in Go**: `tabs`, `windows`, `action` and
   `commands`. Chrome implements these in `//chrome`, against its own `Browser` and tab strip,
   which we don't have. Ours are in `core/extensions/chromeapi`, against Go's tabs and window.
3. **The shell draws all extension UI from Go's state**: the toolbar buttons, the puzzle menu,
   the extension manager in settings, the install prompt. The one thing it doesn't draw is an
   extension's popup, which is a page of the extension in a view the engine owns.

Go never sees a Chromium type. Everything shaped by Chrome's API is in one Go package,
`chromeapi`. A Gecko engine could not run these packages (Firefox has no extension service
workers), so nothing is built for it; what would carry over is the store client, the install
decision, the buttons' state and the shell UI.

## The engine interface

`engine.Extensions` in `core/engine/extensions.go`, beside `engine.Engine`. It is engine-neutral:

| Go asks | What it does |
|---|---|
| `InstallExtension(path)` | Installs a package file. The engine asks first, with `ExtensionInstallPrompt`, and Go answers with `AnswerInstallPrompt`. `ExtensionInstallDone` says how it ended |
| `LoadUnpackedExtension(dir)` | Loads an extension from its folder, with no prompt |
| `UninstallExtension`, `SetExtensionEnabled`, `Extensions()` | Remove, turn on or off, list |
| `DispatchExtensionEvent(id, event, args)` | Fires an event such as `tabs.onUpdated` in one extension |
| `OpenExtensionPopup(id, url, anchor)`, `CloseExtensionPopup()` | Shows a page of the extension in a view of its own under a rectangle of the window |
| `ChoosePath(kind)` | Asks the user for a folder or a package file, in the system's panel |

| The engine tells Go | When |
|---|---|
| `ExtensionsChanged` | The installed set, or what the engine says about one extension, changed |
| `ExtensionInstallPrompt`, `ExtensionInstallDone` | An install wants an answer; an install or a load ended |
| `ExtensionCall(extension, name, args, tab)` | An extension called one of the browser's own APIs. Go answers with a JSON result or an error |
| `ExtensionBlockedCount(id, tab, count)` | An extension's rules blocked requests in a tab, and it asked to show the count |
| `ExtensionPopupClosed`, `PathChosen` | The popup went away; the user picked a path, or nothing |

**The engine remembers what is installed.** `//extensions` keeps its own state in the profile,
under `chromium/`: `extension-prefs.json` (what is installed, each manifest, granted
permissions), the `Extensions/` folder (the unpacked packages, by id and version), and its
LevelDB stores for `chrome.storage`, rules and scripts. Go keeps no second list: it asks.
Go's own part is `extensions.json` in the profile: the store's address and which buttons are
pinned.

Across the C boundary this is `engine/abi/pb_extensions_abi.h`: a second table of calls that
`PBEngineCalls` points at, and seven events on the one `hook` call. Only a build with
extensions includes it.

## The embedder, `engine/extensions/`

`//extensions` asks its embedder about 75 questions through a handful of interfaces. For one
profile with no incognito, kiosk, guest or policy side, most answers are one line. It is about
3,400 lines of C++; Go's side is about 2,500 lines with its tests, and the shell's 670.

| File | What it is |
|---|---|
| `common/extensions_client.cc` | In every process: which APIs exist. The core ones, plus ours: the features files beside it, and Chrome's schema files for `tabs`, `windows`, `action` and `commands`, used where they are in the checkout |
| `renderer/content_renderer_client.cc` | In a renderer: hands content's hooks to `//extensions`' dispatcher, which makes the `chrome.*` bindings, runs content scripts and starts extension service workers. Project B had no renderer client before |
| `utility/content_utility_client.cc` | In a utility process: the unzip service that unpacking a package uses |
| `browser/extensions.cc` | The runtime as Go sees it: every call of `PBExtensionCalls` lands here. It makes `//extensions`' prefs and services for the profile, loads what is installed at start, installs through `CrxInstaller` and `UnpackedInstaller`, and is the registrar's delegate |
| `browser/extensions_browser_client.cc` | The main-process answers, and the small delegates: background pages, `chrome.runtime`, an extension's own pages |
| `browser/extension_system.cc` | The services `//extensions` expects one object to own: stores, the service worker and user script managers, quotas |
| `browser/content_hooks.cc` | The content hooks: an extension's pages in a process of their own, the `chrome-extension:` scheme's loader factories, the Mojo interfaces, and the place in front of the network that `declarativeNetRequest` and `webRequest` need |
| `browser/go_function.cc` | The one extension function, below |
| `browser/api_client.cc` | Which tab a message came from, and the count of blocked requests |
| `browser/web_contents_observer.cc` | Per page: which extension a frame belongs to, and `chrome.scripting` in a tab |
| `browser/popup.cc` | The popup: a page of the extension that sizes itself, in a view over the shell |

The tab ids `//extensions` sees are the engine's own, which are Go's. There is one window, id 1.

**What `//extensions` insisted on that we don't use.** It would not start, or its installer
would not run, without each of these, so each exists with nothing behind it:

- a delegate for the web store's private API (`api_client.cc`). No page can reach that API here;
- a registry for the schemes an extension handles, which tells macOS nothing;
- a blocklist with no list, an install tracker and an install stage tracker (`extensions.cc`);
- a policy client that allows everything, and a hold on the profile that holds nothing
  (`extensions_browser_client.cc`).

**One change outside the flag.** A crashed shell is now reloaded on a later task, not from
inside the notice that its process died. `//extensions` tracks renderers by process, and lost
the one started in between.

## The bridge to Go

**One function.** Every function of `tabs`, `windows`, `action` and `commands` is registered as
the same C++ class, `GoFunction`. It has no logic: the function's name, its arguments as a JSON
list, the calling extension and the tab the caller runs in go to Go in one `hook` crossing, and
Go's answer, a result or an error, goes back. The names come from Chrome's schemas at start, so
nothing lists them twice. A function Go doesn't have answers "… is not available in Project B",
as a call Chrome refuses does.

**One event path.** Go fires `tabs.onCreated`, `onUpdated`, `onActivated`, `onRemoved` and
`action.onClicked` with `DispatchExtensionEvent`, one extension at a time, because what a tab
object holds depends on who reads it.

What Go answers today:

| API | Functions |
|---|---|
| `tabs` | `get`, `getCurrent`, `query` (not by `url` or `title`), `create`, `duplicate`, `update` (`url`, `active`), `reload`, `remove` |
| `windows` | `get`, `getCurrent`, `getLastFocused`, `getAll`, `create` (its pages open as tabs), `update` (answers, changes nothing) |
| `action` | `setTitle`, `getTitle`, `setBadgeText`, `getBadgeText`, `setBadgeBackgroundColor`, `getBadgeBackgroundColor`, `setBadgeTextColor`, `getBadgeTextColor`, `setPopup`, `getPopup`, `setIcon` (by path), `enable`, `disable`, `isEnabled`, `getUserSettings` |
| `commands` | `getAll` |

Two things cross differently, because only C++ can do them. `action.setIcon` names a file inside
the extension, so the engine reads it and gives Go the picture as a data: URL. And the tab a
message's sender is in (`sender.tab`) is asked of Go by the engine itself, as `tabs.get` with no
extension.

## Installing, and the mock store

Installs come from a store the browser knows by one setting, its address. There is no default,
and nothing points at Google. Not `webstorePrivate`, and no policy install of any kind.

1. The settings page shows the store's catalogue, which Go fetched (`core/extensions/store.go`).
2. Install: Go downloads the package into the profile and hands the file to the engine.
3. `CrxInstaller` unpacks it in a sandboxed process and asks. The prompt goes to Go and on to
   the settings page, which shows the manifest's permission names as written.
4. On yes, `//extensions` records it and loads it, and tells Go, which tells the shell.

A local `.crx` takes the same path from step 2. "Load unpacked" loads a folder, as Chrome's does.

Removing an extension removes what it stored too: everything kept under its origin, its
`chrome.storage`, and for a packaged one its files.

**The mock store** is `core/cmd/extstore`, a separate small Go program on `127.0.0.1`. It serves
the folder `tools/fetch-extensions.sh` fills, `.tools/extensions/`, which git ignores:

- `GET /catalogue.json`: one entry per extension: id, name, version, description, size, package, icon.
- `GET /crx/<id>_<version>.crx`: the package, byte for byte as fetched.
- `GET /icon/<id>.png`.
- `GET /update?x=id%3D<id>%26v%3D<version>%26uc`: gupdate XML in the Web Store's shape. Nothing reads it yet.

It serves uBlock Origin Lite, Bitwarden, Okta, 1Password and Grammarly, fetched from the Web
Store's public update endpoint, and one of ours: `tools/test-extension/`, a small Manifest V3
extension with an action popup, a content script, storage, tabs and messaging.
`tools/pack-test-extension.py [version]` packs it as a CRX3 into the store's folder, signed with
`tools/test-extension.pem`, a key that exists only so it keeps one id. Packing 1.0 and then 1.1
gives the store an update to offer.

To keep the store up, build it and hand it to amarit:

    (cd core && go build -o ~/PW_TOOLS/project-b-ext/extstore ./cmd/extstore)
    amarit run -name pb-extstore ~/PW_TOOLS/project-b-ext/extstore -dir "$PWD/.tools/extensions"

## The build flag

One switch: `EXTENSIONS=1 tools/build.sh`. Default off. It sets three things from one variable,
and nothing checks anything at run time.

| Layer | On | Off |
|---|---|---|
| GN: `project_b_extensions` (`engine/extensions.gni`) | `engine/extensions/` is compiled and `//extensions` is linked; `BUILDFLAG(PB_EXTENSIONS)` guards the few calls into it from existing files | No dependency on `//extensions`, so nothing of it links |
| Go: build tag `extensions` | `core/extensions/`, and one tagged file each in `engine`, `mockengine`, `content`, `app`, `cmd/lib`, `cmd/dev` | None of them is compiled |
| Shell: the `@extensions` alias | `shell/src/extensions/` | `shell/src/extensions-off/`, the same names with nothing in them |

Keep an extensions build in its own folder (`OUT=out/ext`). The flag is Mac only; the Android
build refuses it.

Measured on the build Mac, same arguments otherwise (not official, `symbol_level = 0`):

| | Off | On |
|---|---:|---:|
| `libproject_b_content.dylib`, unstripped, bytes | 415,852,800 | 427,483,248 |
| Build steps for a full build | about 37,800 | about 42,500 |
| Full build, 32 cores | 53 min | 57 min |

So the flag adds 11.6 MB to the unstripped library. With the flag off, `nm` finds no symbol of
namespace `extensions` in the library, and the shell's files hold no extension UI. Switching the
flag in one folder rebuilds our own files and links again, under a minute, because the rest of
`//extensions` stays compiled.

## The UI

- **Toolbar.** A puzzle-piece button opens a menu of the enabled extensions, each with a pin.
  A pinned extension has its own button: icon, badge, and its popup under it. Pressing an
  unpinned one in the menu opens its popup under the puzzle button. "Manage extensions" opens
  settings at Extensions.
- **The popup** is a `WebContents` of its own in a native view over the shell, never a frame
  inside the shell. It takes the size its page lays out to (25 by 25 up to 800 by 600, as
  Chrome's), and closes on Escape, when the page closes itself, on a second press of its
  button, and on a press anywhere else. The engine closes it when the keyboard goes to a
  page. When the keyboard goes to the shell the engine leaves it, because only the shell knows
  whether the press was on the popup's own button: that press goes to Go, which closes the
  popup it finds open, and any other press in the shell asks Go to close it.
- **Settings, Extensions.** Installed: enable, pin, remove, and details (id, version, source,
  permissions, site access). "Load unpacked" and "Install package…" open the system's panel.
  Store: the address, and the catalogue with Install. The install prompt is a dialog there.

## Tests

`EXTENSIONS=1` adds to two of the existing groups.

- `EXTENSIONS=1 tools/test.sh mock`: Go's tests with the tag (`core/app/extensions_test.go`, 8
  tests: the store, the prompt, load unpacked, pins, the popup, and the `tabs` and `action` APIs,
  on the mock engine's fake runtime), then `shell/e2e/extensions.cjs`, 40 checks: the same
  through the real UI in a browser. Its last check builds the shell without the flag and finds
  no extension UI in it.
- `EXTENSIONS=1 tools/test.sh app`: `tools/drive_extensions.py`, 28 checks on the real app built
  with the flag (`PB_BIN`, or `chromium/src/out/ext/project_b`): it installs the test extension
  and uBlock Origin Lite from the mock store, sees the content script run and an ad host
  blocked, pins and unpins, opens the popup and clicks in it, restarts, turns one off and
  removes it. It needs the network, for the ad host.
- `tools/test.sh app`, without the flag, also runs `tools/check_no_extensions.py`: the library
  has no symbol of namespace `extensions`, and the main binary none of Go's extension packages.

The mock engine's fake runtime (`core/mockengine/extensions.go`) reads a package's manifest as
the real engine does and keeps the list, but runs no JavaScript: a test calls an API as an
extension would (`POST /mock/extensions/call` on the dev server) and reads back the events and
the popup the app asked for.

The existing app checks (`tools/drive_first_app.py`, 131) pass on a build with the flag and on
one without.

## Checks to discuss

Every check below was left out on purpose, to be discussed and then built or dropped.

**At the store and before install**

- The package's SHA-256 is not checked against anything: not by the fetch script, not by the
  store, not by Go after the download.
- The fetch script does not check that what it downloaded is a CRX.
- Go does not check that the installed id is the one the catalogue named.
- The store's address is taken as typed: no scheme rule, no allow-list, no TLS requirement.
- No size limit on a download.

**In the installer**

- The package is verified as a plain `CRX3`: its own signature and the id that follows from its
  key. Google's publisher proof, which a Web Store package also carries, is not required.
- No blocklist: `GetBlocklist` answers nothing, so Chrome's malware list is never consulted.
- No management policy: nothing can forbid an extension, a permission or a host.
- No off-store rule: any CRX3 from any file installs.
- An update that asks for more permissions installs like any other. Chrome disables such an
  update until the user agrees.

**While running**

- No content verification: an installed extension's files are not checked against
  `verified_contents.json`.
- No Safe Browsing checks on extensions or on what they load.
- Requests extensions make, and redirects they cause, are not put to our URL policy, because
  the policy covers top-level navigations only. See "URL policy on every request" in
  [todo.md](todo.md): `declarativeNetRequest` and `webRequest` sit in the same hook that a
  proxy would.
- A tab's address and title go to an extension that has the `tabs` permission or asks for any
  host at all. Chrome checks the host against each tab's address.
- `activeTab` is never granted by a click on the button.
- The install prompt shows raw permission names, not Chrome's warning sentences.
- User-level host controls ("on click", "on specific sites") don't exist.

## Open for the owner

- **`action.setIcon` with a canvas bitmap** (`imageData`) isn't supported, only `path`. So are
  `tabs.sendMessage` and `tabs.connect` from the extension to a tab: Chrome implements both in
  `//chrome`'s renderer code. Messages from a content script to its extension work.
- **Extension commands have no keys.** `commands.getAll` lists them without shortcuts, and
  nothing fires `commands.onCommand`. They belong in the keymap, with the menus work.
- **`action.openPopup`, `tabs.query` by `url` or `title`, `tabs.move`, zoom, `captureVisibleTab`,
  groups** and the other functions not in the table above answer "not available".
- **Updates.** The store answers `/update`, and the installer accepts a newer package, but
  nothing checks on a timer. Chromium's own updater stays off: it would follow each manifest's
  `update_url` to Google.
- **Extension pages in tabs** (an options page, `chrome-extension://…` typed in the bar) load,
  but the URL gate treats them as any page.
- **Bitwarden, Okta, 1Password and Grammarly** are in the store and install, and were not
  tested beyond that. 1Password needs `nativeMessaging` and Grammarly `identity`.
- **What the first launch after a link costs.** Twice, the first start of a freshly linked
  extensions build printed nothing for a minute, and the next start was normal. Not looked into.
- **The store's default address is undecided.** Today there is none, and a new profile shows
  an empty store until one is set. The owner's word so far: "Store defaults to chrome store for
  now... we can change it to mock or just visit mock store for testing". What "chrome store"
  means for a store that is a catalogue and package files, which Google's isn't, is a question
  still to put to him. Nothing is built for it.
