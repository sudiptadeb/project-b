# Tracking web features over time

A proposal. Nothing here is wired into tests yet.

## The problem

A web feature's state in Project B is decided in four places that never meet: Blink's runtime
features, content's binder maps, our engine's hooks and delegates, and Go's managers. Chromium
moves all four on every version bump. Today nothing tells us when a bump adds a Blink API whose
browser side is chrome's, and the first we hear of it is a site crashing, as DuckDuckGo did.

## Recommendation: one registry, read by the tests, diffed on every bump

Shaped like `core/flags/registry.json`, and for the same reason: one file, read by everything
that needs it, so a feature can't change state in one place and not the others.

### The three pieces

1. **The registry**: `engine/web_features.json`, one entry per feature, as in the inventory.
   It lives in `engine/` because the engine owns what it answers for; Go doesn't read it at run
   time.
2. **The tests read it.**
   - The Android probe (`tools/drive_android.py`, branch `android-api-probe`) builds its page
     from the entries that have a `probe` call, instead of its own `WEB_APIS` table. It then
     checks each result against the entry's `status` for that platform, not only that the
     renderer lived.
   - A Mac probe does the same, through the driver (`tools/drive_first_app.py`). The script used
     for the inventory (`docs/research/web-features/scripts/mac_probe.py`) is the start of it. It goes in a new `tools/check_web_features.py`,
     run by `tools/test.sh web-features`, on Mac and, with `--android`, through the probe.
   - `tools/check_basics.py` keeps its areas, and each check names the registry ids it covers
     (`fullscreen`, `downloads`…). When a basics area turns ok, the test fails until the registry
     says "works" too.
   - `kChromeOnlyBlinkFeatures` (`engine/common/main_delegate.cc`) is generated from the entries
     whose `needs` is `stay-off`, so the off list and the registry can't drift. GN reads the JSON
     as it already does for flags (`engine/flags.gni`).
3. **The bump check**: `tools/check_web_features_bump.py`, run with `tools/gen_hooks.py` after
   every Chromium upgrade. It extracts chrome's binders, content's binders and the delegate
   interfaces from the new tree, diffs them against the registry, and lists what is new. It
   exits non-zero on a new interface that Blink can ask for and that content doesn't bind.

### Draft format

Each entry:

| Key | Meaning |
|---|---|
| `id` | The inventory's id |
| `api` | What a page calls |
| `blink_features` | Blink runtime features that gate it; empty when nothing does |
| `interface` | The Mojo interface, with its scope (`frame`, `service_worker`, `dedicated_worker`) |
| `delegate` | The delegate method that decides it, when there is one |
| `permission` | The `blink::PermissionType`, when there is one |
| `status` | Per platform: `works`, `quiet`, `absent`, `kills`, `unknown` |
| `needs` | `stay-off`, `binder`, `delegate`, `go-manager`, `prompt`, `build`, or a list |
| `owner` | Our file and function that answers for it, or `null` |
| `probe` | The JS the probes call; the result a status expects |
| `test` | The check that proves the status: a basics area, a probe id |

Six real entries, with today's state:

```json
[
  {
    "id": "badge",
    "api": "navigator.setAppBadge, navigator.clearAppBadge",
    "blink_features": [],
    "interface": {"name": "blink.mojom.BadgeService", "scope": ["frame", "service_worker"]},
    "delegate": null,
    "permission": null,
    "status": {"mac": "kills", "android": "works"},
    "needs": ["binder"],
    "owner": null,
    "probe": {"call": "navigator.setAppBadge(3)", "expect": {"mac": "resolved", "android": "resolved"}},
    "test": ["probe:badge"]
  },
  {
    "id": "installed-apps",
    "api": "navigator.getInstalledRelatedApps",
    "blink_features": ["InstalledApp"],
    "interface": {"name": "blink.mojom.InstalledAppProvider", "scope": ["frame"]},
    "delegate": null,
    "permission": null,
    "status": {"mac": "absent", "android": "absent"},
    "needs": ["stay-off"],
    "owner": "engine/common/main_delegate.cc:DisableChromeOnlyBlinkFeatures",
    "probe": {"call": "navigator.getInstalledRelatedApps()", "expect": "TypeError"},
    "test": ["probe:installedApps"]
  },
  {
    "id": "file-input",
    "api": "<input type=file>",
    "blink_features": [],
    "interface": {"name": "blink.mojom.FileChooser", "scope": ["frame"], "bound_by": "content"},
    "delegate": "WebContentsDelegate::RunFileChooser",
    "permission": null,
    "status": {"mac": "quiet", "android": "unknown"},
    "needs": ["delegate", "prompt"],
    "owner": null,
    "probe": null,
    "test": ["basics:14"]
  },
  {
    "id": "perm-geolocation",
    "api": "navigator.geolocation.getCurrentPosition",
    "blink_features": [],
    "interface": {"name": "blink.mojom.GeolocationService", "scope": ["frame"], "bound_by": "content"},
    "delegate": "BrowserContext::GetPermissionControllerDelegate",
    "permission": "GEOLOCATION",
    "status": {"mac": "quiet", "android": "quiet"},
    "needs": ["delegate", "go-manager", "prompt"],
    "owner": "engine/browser/browser_context.cc:BrowserContext::GetPermissionControllerDelegate",
    "probe": {"call": "new Promise((ok, no) => navigator.geolocation.getCurrentPosition(ok, no, {timeout: 1500}))",
              "expect": "PositionError 1"},
    "test": ["probe:geolocation"]
  },
  {
    "id": "speech-on-device",
    "api": "SpeechRecognition.available({processLocally: true}), SpeechRecognition.install()",
    "blink_features": ["OnDeviceWebSpeechAvailable", "InstallOnDeviceSpeechRecognition"],
    "interface": {"name": "media.mojom.OnDeviceSpeechRecognition", "scope": ["frame"]},
    "delegate": null,
    "permission": null,
    "status": {"mac": "kills", "android": "kills"},
    "needs": ["stay-off"],
    "owner": null,
    "probe": {"call": "SpeechRecognition.available({langs: ['en-US'], processLocally: true})",
              "expect": "TypeError"},
    "test": ["probe:speechLocal"]
  },
  {
    "id": "fullscreen",
    "api": "Element.requestFullscreen",
    "blink_features": [],
    "interface": null,
    "delegate": "WebContentsDelegate::EnterFullscreenModeForTab",
    "permission": null,
    "status": {"mac": "quiet", "android": "quiet"},
    "needs": ["delegate", "go-manager"],
    "owner": "core/content/wire.go (not subscribed)",
    "probe": null,
    "test": ["basics:18"]
  }
]
```

`expect` for `speech-on-device` is what it will be once it is off; until the fix lands, the
registry says `kills` and the probe skips calls whose status is `kills`, so the run doesn't die.
That rule matters: a `kills` entry is listed, counted and reported, never called.

A key the loader doesn't know fails a Go test, as `TestTheRegistry` does for flags. A `status`
of `kills` on any platform fails `tools/test.sh web-features` unless the entry also names an
issue, so a known kill is always on someone's list.

### How the bump check extracts chrome's binders

The binder lists are C++ calls, `map->Add<ns::mojom::Name>(...)`, inside a handful of functions,
under `#if BUILDFLAG(...)` guards. A 60-line Python reader is enough; the one used for the
inventory is `scripts/binders.py` here. Outline:

```
for each (file, function) in SOURCES:
    find the function's body by brace depth
    keep a stack of #if / #elif / #else / #endif conditions (also "# endif")
    on each map->Add<X>(binder) or registry->AddInterface<X>(...):
        record X, the conditions, the line, and whether the binder is
        EmptyBinderForFrame or base::DoNothing
reduce conditions to platforms: mac, android (treat other BUILDFLAGs as on unless
    named in a small table: ENABLE_SPELLCHECK on, ENABLE_PRINT_PREVIEW off on Android, ...)
```

`SOURCES`:

| Scope | Chrome | Content |
|---|---|---|
| frame | `chrome/browser/chrome_browser_interface_binders.cc:PopulateChromeFrameBinders` | `content/browser/browser_interface_binders.cc:PopulateBinderMapWithContext(RenderFrameHostImpl*)` |
| service worker | `chrome/browser/chrome_content_browser_client_receiver_bindings.cc:RegisterBrowserInterfaceBindersForServiceWorker` | `PopulateServiceWorkerBinders`, `PopulateBinderMapWithContext(ServiceWorkerHost*)` |
| dedicated, shared worker | (none today) | `PopulateDedicatedWorkerBinders`, `PopulateSharedWorkerBinders` |
| ours | – | `engine/browser/content_browser_client.cc`, `engine/extensions/browser/content_hooks.cc` |

Then the check answers three questions:

1. **Which interfaces does chrome bind that content and we don't?** Set difference per platform.
2. **Can Blink ask for each one?** Grep `third_party/blink/renderer` for the interface's Blink
   type next to `GetBrowserInterfaceBroker().GetInterface(`, and note the file's IDL
   `RuntimeEnabled=` and the `#if BUILDFLAG(IS_ANDROID)` around the call. The feature's status
   comes from `runtime_enabled_features.json5` (the `status` object per platform) and from
   `content/child/runtime_features.cc` (base features that turn it on). If no Blink code asks, the
   interface is chrome's renderer's business and is ignored.
3. **Is it in the registry?** If not, it's new: print it with its Blink caller and status.
   If it is, and the platform status changed (a feature went stable on Android, say), print that.

For delegates, the check reuses `tools/gen_hooks.py`'s parse of `WebContentsDelegate`,
`ContentBrowserClient` and `BrowserContext`: any new virtual method, and any method whose
default body changed, is listed. A person decides whether it matters for the web; the check only
makes sure nobody misses it. The permission list comes from the `PermissionType` enum: a new
value is a new entry.

A bump then reads:

```
new interface, Blink can ask:  blink.mojom.FooService  (frame)
    caller: third_party/blink/renderer/modules/foo/foo.cc:88, RuntimeEnabled=Foo, status stable
    chrome binds it: chrome/browser/chrome_browser_interface_binders.cc:530, all platforms
    content: no binder  -> a page calling navigator.foo() will be killed
status moved: OnDeviceWebSpeechAvailable  experimental -> stable
new delegate method: WebContentsDelegate::RequestFoo (default: does nothing)
new permission type: FOO = 47
```

### Where each piece lives

| Piece | Path |
|---|---|
| The registry | `engine/web_features.json` |
| Its loader and schema test | `core/webfeatures/registry.go`, `registry_test.go` (the probes are Python, but Go tests own schemas here) |
| The generated off list | `engine/web_features.gni` writes `kChromeOnlyBlinkFeatures` into a generated header for `engine/common/main_delegate.cc` |
| Empty binders the registry asks for | `engine/browser/content_browser_client.cc`, a new `RegisterBrowserInterfaceBindersForFrame` and `…ForServiceWorker` on the plain client, which the extensions client calls first |
| The Mac probe | `tools/check_web_features.py`, `tools/test.sh web-features` |
| The Android probe | `tools/drive_android.py`, reading the registry |
| The bump check | `tools/check_web_features_bump.py`, next to `tools/gen_hooks.py`, and a line in `docs/content-hooks.md`'s "When Chromium is upgraded" |
| The human summary | `docs/research/web-features/README.md` today; it moves to `docs/web-features.md` when the registry lands |

## Alternatives, in brief

- **Keep the inventory as a document only.** Cheapest, and stale after the next bump. The
  DuckDuckGo crash happened because nothing checked; a document doesn't check either.
- **Bind everything chrome binds to an empty binder, and stop there.** It removes every renderer
  kill in one change and needs no registry. But a page then waits forever on a promise instead of
  being told the API isn't there, which is worse for a site that would have fallen back. Use it
  as the safety net for unknown interfaces, not as the policy. Worth doing anyway for `BadgeService`
  and `PersistentRendererPrefsService`, which have no runtime feature to turn off.
- **Turn on `--enable-experimental-web-platform-features` style allow-listing: ship with a fixed
  list of Blink features on, everything else off.** Strong, but it fights Chromium on every bump
  and breaks web-platform behaviour nobody listed. Too blunt.
- **Use the generated hooks table (`docs/content-hooks.md`) as the registry.** It already lists
  delegate methods per Chromium version. But it knows nothing of binders, runtime features or
  permissions, and its unit is a C++ method, not something a page does. Better to link the two:
  each registry entry names its delegate, and the hooks table gets a column of registry ids.
