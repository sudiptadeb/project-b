# Chrome extensions

Project B can run Chrome's Manifest V3 extensions. Every build has them, and they run only while
the Extensions flag is on (Settings, Flags; [flags.md](flags.md)). It is off by default.
This is the design as built. The research behind it is in
`project-b-research/chrome-extensions.md` and `chrome-extensions-reassessment.md`.

    tools/fetch-extensions.sh                     # six real packages into .tools/extensions/
    tools/pack-test-extension.py                  # ours, packed into the same folder
    (cd core && go run ./cmd/extstore -dir ../.tools/extensions)    # the mock store, on 127.0.0.1:8766
    tools/build.sh                                # extensions are in the build by default
    chromium/src/out/pb/project_b                 # then Settings, Flags, Extensions, and Relaunch

The store is a website. Started like that, the store is the Chrome Web Store: visit
`chromewebstore.google.com` and press "Add to Chrome". To make the mock store the store:

    chromium/src/out/pb/project_b --apps-gallery-url=http://127.0.0.1:8766 \
        --apps-gallery-download-url=http://127.0.0.1:8766/download/%s

and visit `http://127.0.0.1:8766/`.

What works today, in the real app: uBlock Origin Lite installs from the real Chrome Web Store's
page, and it and the test extension from the mock store's page, each with the prompt; uBlock Origin Lite blocks, and shows its count on its button;
content scripts, service workers, messaging both ways and `chrome.storage` run; Fonts Ninja opens its panel; buttons pin and unpin;
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
| `OpenExtensionPopup(window, id, url, anchor)`, `CloseExtensionPopup()` | Shows a page of the extension in a view of its own under a rectangle of the window whose button was pressed |
| `ChoosePath(window, kind)` | Asks the user for a folder or a package file, in the system's panel on the window that asked |

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
| `browser/content_hooks.cc` | The content hooks: an extension's pages in a process of their own, the `chrome-extension:` scheme's loader factories, the Mojo interfaces, and the place in front of the network that `declarativeNetRequest` and `webRequest` need. Each is for the profile's context only: our pages' context has no extensions |
| `browser/go_function.cc` | The one extension function, below |
| `browser/api_client.cc` | Which tab a message came from, and the count of blocked requests |
| `browser/web_contents_observer.cc` | Per page: which extension a frame belongs to, and `chrome.scripting` in a tab |
| `browser/popup.cc` | The popup: a page of the extension that sizes itself, in a view over the shell |

The tab and window ids `//extensions` sees are the engine's own, which are Go's.
`chrome.windows` answers from Go's windows (`core/window`): each window's frame, state and
whether it is in front, its tabs, each with its index there, and one active tab per window.
"The current window" is the window of the caller's tab, or for a popup or a service worker
the window in front, which is also the last focused. Go's word for a window's state maps to
Chrome's: zoomed is `maximized`. Every window is a normal one: one created as a popup is a
normal window. Nothing is fired when the whole app loses focus, so `onFocusChanged` never
reports `WINDOW_ID_NONE`. A frame dragged across the screen fires `onBoundsChanged` each time
the engine reports it.

**What `//extensions` insisted on.** It would not start, or its installer
would not run, without each of these, so each exists with nothing behind it:

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

**Messages to a tab.** `tabs.sendMessage` and `tabs.connect` open a message channel, as
`runtime.sendMessage` does the other way, and never reach Go as calls. Chrome answers them in
its renderer code (`//chrome`); ours are `engine/extensions/renderer/tabs_hooks.cc`, and the
browser finds the tab by Go's id in `api_client.cc`.

**One event path.** Go fires `tabs.onCreated`, `onUpdated`, `onActivated`, `onRemoved` and
`action.onClicked` with `DispatchExtensionEvent`, one extension at a time, because what a tab
object holds depends on who reads it.

What Go answers today:

| API | Functions |
|---|---|
| `tabs` | `get`, `getCurrent`, `query` (`url` by match pattern, `title` by `*` and `?`), `create` (into `windowId`), `duplicate`, `update` (`url`, `active`), `reload`, `remove`, `captureVisibleTab` |
| `windows` | `get`, `getCurrent`, `getLastFocused`, `getAll`, `create` (`url`, one or a list, or `tabId`, with `left`, `top`, `width`, `height`, `state`), `update` (`focused`, the frame, `state`), `remove`; the events `onCreated`, `onRemoved`, `onFocusChanged`, `onBoundsChanged`, and `tabs.onDetached` and `onAttached` for a tab moved between windows |
| `action` | `setTitle`, `getTitle`, `setBadgeText`, `getBadgeText`, `setBadgeBackgroundColor`, `getBadgeBackgroundColor`, `setBadgeTextColor`, `getBadgeTextColor`, `setPopup`, `getPopup`, `setIcon` (by `path` or `imageData`), `enable`, `disable`, `isEnabled`, `getUserSettings` |
| `commands` | `getAll` |

Two things cross differently, because only C++ can do them. `action.setIcon` names a file inside
the extension, so the engine reads it and gives Go the picture as a data: URL (an engine
function, below); or it passes a picture drawn on a canvas, which only the renderer can read,
so `engine/extensions/renderer/action_hooks.cc` makes it a PNG there. `tabs.captureVisibleTab`
asks Go which tab is showing, and the engine takes its picture if the extension may see the
page (all hosts, as `activeTab` is never granted). And the tab a
message's sender is in (`sender.tab`) is asked of Go by the engine itself, as `tabs.get` with no
extension.

## Adding an API

An API of the browser itself, one Chrome implements in `//chrome`, is answered in Go. Adding
one, say `bookmarks`, is three places, each holding only what is new:

1. **Its name** in `project_b_go_apis` in `engine/BUILD.gn`. That pulls in Chrome's schema,
   `chrome/common/extensions/api/bookmarks.json`, and registers each of its functions.
2. **Where it appears**, a row in `engine/extensions/common/api_features.json`, copied from
   Chrome's `_api_features.json`; and its permission in `permission_features.json` and
   `extensions_client.cc`, if it has one.
3. **Its functions**, in `core/extensions/chromeapi/bookmarks.go`: one
   `functions["bookmarks.get"] = func(a *API, c call) (any, error)` each, with `c.arg(i, &v)`
   for the arguments. A function left out answers "… is not available in Project B".

Adding a function to an API that is there is only step 3.

**A function that needs the engine.** Some functions need C++ for a part: reading a file of
the extension, taking a picture of a tab. Such a function is a subclass of `GoFunction`, in
`engine/extensions/browser/<api>_functions.cc`, declared in `engine_functions.h` and listed
in `kEngineFunctions` in `go_function.cc`. It does the engine's part and then calls `Ask`
with the arguments Go should see, so the decision stays in Go. `action.setIcon` and `tabs.captureVisibleTab` are two.

**A function that runs in the page.** A few never reach the browser as a function call:
Chrome answers them in the renderer, with a hooks delegate. Ours are in
`engine/extensions/renderer/`, registered in `api_provider.cc`; `tabs.sendMessage` is one.

## Installing: the store is a website

An extension is installed from a store's own page, as in Chrome. The page calls
`chrome.webstorePrivate`, an API `//extensions` implements and gives only to the store's origin:

1. "Add to Chrome" on the page calls `beginInstallWithManifest3` with the extension's id,
   manifest and icon. `//extensions` asks its embedder to show a prompt. Ours goes to Go, and
   the shell shows it over the page, with the manifest's permission names as written.
2. On yes the page calls `completeInstall`. `//extensions`' `WebstoreInstaller` downloads the
   package and hands it to `CrxInstaller`, which unpacks it in a sandboxed process, records it
   and loads it. The registry tells Go, which tells the shell.

**Which origins are a store.** `chromewebstore.google.com` has the API by `//extensions`' own
rule. One more origin can be named on the command line with `--apps-gallery-url`, which is
Chromium's own switch for testing a store (`extensions/common/webstore_override.cc`); we turn
its check on and copy the switch to renderers. That is how the mock store's page gets the API.
It names one origin, so there are at most two stores at once: Google's and that one.

**Where packages come from.** `WebstoreInstaller` builds one download address for every
install: Google's update endpoint, or the template in `--apps-gallery-download-url`. So with the
mock store's two switches, an install begun on Google's page would also be fetched from the mock
store. The app as it ships passes neither switch, and everything is Google's.

**The download** goes through content's `DownloadManager`, as in Chrome. It needed nothing from
us: with no download manager delegate, content saves to the path the installer forces (a
temporary file under the profile's `Webstore Downloads`), and the installer deletes it. No
other download behaves differently, and there is still no general download feature.

**What the store's page made us add**, each because the real page calls it or the installer
dereferences it: `chrome.management` with a delegate of ours (`management_delegate.cc`; the
page listens for installs and has a Remove button), an image decoder for the icon, the
browser's version in the download address (Google answers nothing without it), an install
verifier that is never started, a shared module service, and the prefs of an enterprise
banner the page asks about.

A local `.crx` (Install package… in the extension manager) goes straight to `CrxInstaller`, which
asks with the same prompt, shown in the settings page. "Load unpacked" loads a folder, with no
prompt, as Chrome's does. The extension manager has a link that opens the Chrome Web Store in a
tab.

A store page's Remove button asks first, as Chrome's does: `//extensions` hands the question to
our management delegate, which asks through the install prompt marked as a removal, and removes
only on a yes. Remove in the extension manager removes at once.

**The Chrome Web Store's words.** The page says "Add to Chrome", and tells a browser that isn't
Chrome to switch. A style sheet the renderer adds to that page, and no other
(`content_renderer_client.cc`), makes the button read "Add to Project B" and "Remove from
Project B" and hides the banner. It is CSS only: it finds the button by the names the store's
markup gives it (a class and `jsname`), so it stops working, harmlessly, when Google renames
them. The extensions test checks it.

Removing an extension removes what it stored too: everything kept under its origin, its
`chrome.storage`, and for a packaged one its files.

**The mock store** is `core/cmd/extstore`, a separate small Go program on `127.0.0.1`. It serves
the folder `tools/fetch-extensions.sh` fills, `.tools/extensions/`, which git ignores:

- `GET /`: the store's page: the catalogue, with an "Add to Project B" button for each, which
  makes the same two `chrome.webstorePrivate` calls the Chrome Web Store's page makes.
- `GET /catalogue.json`: one entry per extension: id, name, version, description, size, package, icon.
- `GET /manifest/<id>`: an extension's manifest, which the page hands the browser.
- `GET /download/<id>`: its package, as `application/x-chrome-extension`, where the browser
  fetches it to install.
- `GET /crx/<id>_<version>.crx`: the package, byte for byte as fetched.
- `GET /icon/<id>.png`.
- `GET /update?x=id%3D<id>%26v%3D<version>%26uc`: gupdate XML in the Web Store's shape. Nothing reads it yet.

It serves uBlock Origin Lite, Bitwarden, Okta, 1Password, Grammarly and Fonts Ninja, fetched from the Web
Store's public update endpoint, and one of ours: `tools/test-extension/`, a small Manifest V3
extension with an action popup, a content script, storage, tabs and messaging.
`tools/pack-test-extension.py [version]` packs it as a CRX3 into the store's folder, signed with
`tools/test-extension.pem`, a key that exists only so it keeps one id. Packing 1.0 and then 1.1
gives the store an update to offer. `tools/drive_extensions.py` packs 1.0 again before it
runs, so the store's copy is never older than the checks.

To keep the store up, build it and hand it to amarit:

    (cd core && go build -o ~/PW_TOOLS/project-b-ext/extstore ./cmd/extstore)
    amarit run -name pb-extstore ~/PW_TOOLS/project-b-ext/extstore -dir "$PWD/.tools/extensions"

## The build flag

Extensions is a build flag (`"build": true` in `core/flags/registry.json`), built in unless
`WITHOUT=extensions tools/build.sh`. What runs is then the Extensions flag's to say.
[flags.md](flags.md) describes both.

| Layer | Built in | Left out |
|---|---|---|
| GN: `project_b_extensions` (`engine/flags.gni`) | `engine/extensions/` is compiled and `//extensions` is linked; `BUILDFLAG(PB_EXTENSIONS)` guards the few calls into it from existing files | No dependency on `//extensions`, so nothing of it links |
| Go: build tag `extensions` | `core/extensions/`, and one tagged file each in `engine`, `mockengine`, `content`, `app`, `cmd/lib`, `cmd/dev`, `flags` | None of them is compiled |
| Shell: the `@extensions` alias | `shell/src/extensions/` | `shell/src/extensions-off/`, the same names with nothing in them |

Keep a build without extensions in its own folder (`WITHOUT=extensions OUT=out/noext`). It is
Mac only; the Android build leaves it out.

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
  Store: a link that opens the Chrome Web Store in a tab.
- **The install prompt** is one dialog, shown by the shell over the page for an install a
  store's page began, and by the settings page for a package picked there. A store page's
  Remove asks with the same dialog, as "Remove?".

## Tests

The extension checks are part of two of the existing groups, unless `WITHOUT=extensions`.

- `tools/test.sh mock`: Go's tests with the tag (`core/app/extensions_test.go`, 8
  tests: the prompt in both pages, a package, load unpacked, pins, the popup, and the `tabs` and
  `action` APIs, on the mock engine's fake runtime), then `shell/e2e/extensions.cjs`, 42 checks: the same
  through the real UI in a browser. Its last check builds the shell without the flag and finds
  no extension UI in it.
- `tools/test.sh app`: `tools/drive_extensions.py`, 54 checks on the real app
  (`PB_BIN`, or `chromium/src/out/pb/project_b`), in profiles with the Extensions flag on, and
  one with it off, where no button shows, the store's page has no store API, and the Flags
  section's switch and Relaunch turn it on. Started with the mock store as
  its store, it installs the test extension and uBlock Origin Lite from the mock store's page,
  sees the content script run and an ad host blocked, pins and unpins, opens the popup and
  clicks in it, has its service worker message the tab, restarts, turns one off and removes it. The popup's opening has the worker draw
  its icon on a canvas, and the button shows it; and it takes a picture of the tab.
  Fonts Ninja, installed the same way, opens its panel on the page when its button is pressed. Then, started as it ships, it opens the
  Chrome Web Store from the extension manager, installs uBlock Origin Lite with the real page's
  "Add to Chrome", and sees it block. It needs the network, and Google's page as it is today.
  Its first wait, 30 s for the store's tab, has run out once with the store answering; the
  rerun passed. Rerun once before taking that as a failure.
  `tools/drive_extensions.py fonts_ninja` runs one part. A real extension's known limit is
  written with `limit()`: reported on a `known` line until it works, and then a failure, so
  it becomes a check.
- `WITHOUT=extensions tools/test.sh app` runs `tools/check_no_extensions.py`: the library
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

- A package from the mock store is not checked against anything the store said: no hash, no
  size. The fetch script that fills the mock store checks neither a hash nor that a file is a CRX.
- Any origin named with `--apps-gallery-url` is a store, over plain http too.
- The supervised-user question (a child needing a parent's yes) is always answered no.

**In the installer**

- The package is verified as a plain `CRX3`: its own signature and the id that follows from its
  key. Google's publisher proof, which a Web Store package also carries, is not required.
- No blocklist: `GetBlocklist` answers nothing, so Chrome's malware list is never consulted.
- No management policy: nothing can forbid an extension, a permission or a host.
- No off-store rule: any CRX3 from any file installs. A package downloaded for a store's page
  is held to the same plain `CRX3`, not to the publisher proof Chrome demands of its store.
- The install verifier, which in Chrome checks that an installed extension came from the store,
  exists and is never started.
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

- **Extension commands have no keys.** `commands.getAll` lists them without shortcuts, and
  nothing fires `commands.onCommand`. They belong in the keymap, with the menus work.
- **`action.openPopup`, `tabs.move`, zoom, groups** and the other functions not in the table above answer "not available".
- **Updates.** The store answers `/update`, and the installer accepts a newer package, but
  nothing checks on a timer. Chromium's own updater stays off: it would follow each manifest's
  `update_url` to Google.
- **Extension pages in tabs** (an options page, `chrome-extension://…` typed in the bar) load,
  but the URL gate treats them as any page.
- **Bitwarden, Okta, 1Password and Grammarly** are in the store and install, and were not
  tested beyond that. 1Password needs `nativeMessaging` and Grammarly `identity`.
- **What the first launch after a link costs.** Twice, the first start of a freshly linked
  extensions build printed nothing for a minute, and the next start was normal. Not looked into.
- **The store** is decided: it is a website, the Chrome Web Store by default, and the mock
  store when the two switches name it. Left open by how Chromium's installer works: there is
  one download address for all installs, so the two stores can't both install in one run.
