# Android

Can we build Android the same way as the Mac app? Mostly yes, and this page says what changes and why.
All Chromium paths below are in the 155.0.8059.41 tree (`chromium/src`).

## The short answer

- **Yes, on real content.** The same Go managers, the same `PBEngineCalls` table, the same C++ adapter files and
  the same Vue shell, transparent over the page, run on Chromium 155's content layer on Android. Tested on two
  emulators. Only the window and the start-up are new code.
- **It can't be built on macOS itself.** Chromium only builds Android on Linux x86-64. An amd64 container under
  Rosetta in a Colima VM on the Mac Studio does it: about 2.5 hours for the first build, and about 2 minutes after a
  change to our code.
- **The two binaries become three libraries.** Android has no `main` of ours. Content goes in one library that
  every process loads. Go goes in another that only the main window loads, so renderers, the GPU process and the
  app zygote never start Go. Checked on the emulator, in `/proc/<pid>/maps`.
- **The shell on `main` runs on it.** Tabs, favicons, the content hooks, the settings panel with the frame colour,
  and the window manager's one layout, on `main` at 0.2.13. A touch screen gets bigger targets and no hover, and a
  phone shows its tabs as favicons under the toolbar with the full list as a drawer.
- **System WebView was tried first and dropped.** It ran the same managers and shell in a 2.5 MB APK, but it can't
  gate every request, and the shell shares a renderer with the page. What it showed is in
  [the alternatives](#6-the-alternatives-behind-the-same-engine-interface).

## 1. Can a content-based app be built for Android?

Yes. `content_shell_apk` is one, and it's small enough to copy from.

- The target is in `content/shell/android/BUILD.gn`: an `android_apk` with `shared_libraries` set to one library,
  and `content/public/android:content_java` for the Java side.
- Java starts everything. `ContentShellApplication` tells the browser process from a child by whether the process
  name has a colon. `ContentShellActivity` calls `LibraryLoader.ensureInitialized()`, then
  `BrowserStartupController.startBrowserProcessesAsync()`.
- The native side is two functions in `content/shell/android/shell_library_loader.cc`: `JNI_OnLoad`, and a hook
  that calls `content::SetContentMainDelegate`. `content/shell/app/shell_main_delegate.cc` creates a
  `BrowserMainRunner` and leaks it, because Android's looper is the message loop and content never shuts down there.
- A page gets on screen with `ContentView.createContentView()` and `webContents.setDelegates()`
  (`content/shell/android/java/.../Shell.java`), and a `ContentViewRenderView` that owns a `SurfaceView` and one
  compositor (`components/embedder_support/android/view/content_view_render_view.cc`).

The minimum Android version for a content build is 10 (API 29): `default_min_sdk_version` in
`build/config/android/config.gni`.

## 2. What host does it need?

**Linux on x86-64.** `docs/android_build_instructions.md` says "Building the Android client on Windows or Mac is
not supported and doesn't work", and `build/config/BUILDCONFIG.gn` asserts it:
`assert(host_os == "linux", "Android builds are only supported on Linux.")`. The comment above that assert calls
Mac-hosted Android "best-effort", but much of what `DEPS` pulls for Android is `linux-amd64` only, like the JDK
and `aosp_dalvik`.

Chromium's stated needs (`docs/android_build_instructions.md`): at least 100 GB of free disk, 8 GB of RAM, and
"more than 16GB is highly recommended".

The hosts we have, measured:

| Host | Fits? | What it took |
|---|---|---|
| The Mac Studio, as macOS | No | `gn gen` refuses: the assert above |
| The Mac Studio, an amd64 container in a Colima VM with Rosetta: 24 vCPU, 96 GB RAM, 230 GB disk | **Yes, used here** | Fetch, sync, build dependencies and hooks: 16 minutes. First build of `content_shell_apk`: 2 h 31 min, 47,280 steps. Then our APK: about 2 minutes. 57 GB used in the VM, 66 GB on the Mac's disk |
| `bigboy`: 4 vCPU (EPYC 7571), 15 GB RAM, 64 GB disk, 59 GB free | **No, by disk.** 15 GB must stay free, which leaves 44 GB against the 57 GB measured | Not tried. A rough estimate is 40 to 50 hours per full build: a single-threaded Python loop ran 3 times slower there than in the Rosetta container, and it has 6 times fewer cores |

**Recommendation: keep building in the Mac VM.** It's the fastest machine we have, even through Rosetta: the same
Python loop took 3.1 s in the container against 2.2 s natively on the Mac. Growing `bigboy`'s disk to 128 GB would
make the checkout fit, but at about two days per full build it isn't worth it, and it would load a box that serves
the portal and the RBI fleet. A native Linux x86-64 machine with many cores would beat both, but we don't have one.

Three traps, all from Rosetta:

- **Go tools hang.** `cipd` sat for 20 minutes with every thread in a kernel futex wait. Go's async preemption
  signal and Rosetta's own lock deadlock. `GODEBUG=asyncpreemptoff=1` fixes it, for `cipd` and for `siso`, which is
  Go too. Both scripts set it.
- **About 1 compile in 15,000 dies** with `exit=-1` and no message (3 of 47,280). Run again, they pass. The build
  script retries once.
- **Rosetta's future.** Apple has said full Rosetta ends after macOS 27. Reports say a subset stays for Linux VMs
  ([OSnews](https://www.osnews.com/story/145279/macos-27-drops-intel-support-will-be-last-release-with-rosetta-2/),
  [Apple developer forums](https://developer.apple.com/forums/thread/787530)). That's not confirmed from Apple's
  own documentation, so check it before macOS 28.

## 3. How do Android's processes change the two-binary model?

On Mac, content's `GetChildProcessPath` names a program, and that program is the helper with no Go in it. Android
has no programs to choose between.

- **Every process comes from a zygote and starts in Java.** `ActivityThread` runs first, in every process, and
  our native code only arrives when Java loads it. There is no `main` to link Go into.
- **Children are Android services.** `content/browser/child_process_launcher_helper_android.cc` calls Java's
  `ChildProcessLauncherHelperImpl`, which binds a service declared in the manifest:
  `SandboxedProcessService0..N` for renderers with `android:isolatedProcess="true"`, and `PrivilegedProcessService`
  for GPU and network (`content/shell/android/shell_apk/AndroidManifest.xml.jinja2`). They're the same APK
  and the same libraries, only a different process name.
- **Renderers can fork from an app zygote.** `android:useAppZygote="true"` with `ZygotePreload` loads the content
  library once, then forks. Its own comment in `content/public/android/.../ZygotePreload.java` says code there
  "should not create any thread". Go's runtime creates threads when it loads, so Go must never be preloaded there.
- **The library list is explicit.** `LibraryLoader` loads what `NativeLibraries.LIBRARIES` names, in every process
  (`ContentChildProcessServiceDelegate`). What isn't on that list never loads in a child.

So the same idea survives in a different shape:

| | Mac | Android |
|---|---|---|
| Content and the adapter | `libproject_b_content.dylib`, linked at load time by both binaries | `libproject_b_content.so`, on `LibraryLoader`'s list, in every process |
| Go | Static in `project_b`, the main binary | `libcore.so`, not on the list |
| What keeps Go out of children | They run a different binary | They never load that library |
| How the two meet | `main` hands the table to `ProjectBMain` | Java passes the table's address across, as a `long` |

**Library loading.** The owner dislikes `dlopen`, and on Android it can't be avoided: `System.loadLibrary` is the
only way native code gets into a process that started in Java, and Chromium's own `LibraryLoader` does the same
through `android_dlopen_ext` (`docs/android_native_libraries.md`). What we keep is the part that matters: one
explicit load in one place, and `DT_NEEDED` for anything below it. The library still has no link-time reference
to Go, because the table crosses at runtime, as it does on Mac.

## 4. How does Go fit so renderers never start it?

Go's runtime starts by itself when the library holding it loads, so the rule is the same as on Mac: the only thing
that may load `libcore.so` is code that runs only in the main process. That code is `BrowserActivity`, because an
activity with no `android:process` only ever runs in the main process. A child process runs only its service and
`BrowserApplication`, and neither touches Go.

How it's laid out (`engine/android/BUILD.gn`):

| Library | What's in it | Who loads it |
|---|---|---|
| `libproject_b_content.so` | Content, Blink, V8, the network stack and the C++ adapter | Every process, through `LibraryLoader`: it's the APK's `shared_libraries` |
| `libcore.so` | The Go core, built by `engine/android/build_go.py` with Chromium's own clang | Nobody directly |
| `libproject_b_go.so` | One C function, `GoCore.table()`, that returns Go's `PBGoCalls`. Links `libcore.so` | `GoCore`, from `BrowserActivity` |

`libcore.so` and `libproject_b_go.so` are `loadable_modules`, so they're in the APK but on nobody's list. Java hands
the table's address to the content library as a `long`, and `SetGo` takes it from there, the same table `app/main.cc`
fills on the Mac. Neither content library depends on the Go ones (`llvm-readelf -d`).

**Checked on the emulator:** `/proc/<pid>/maps` shows `libcore.so` mapped in the main process,
and in none of the two renderers, the GPU process or the app zygote.

Things that are different on Android:

- Every Go engine call lands on Android's main thread, which is content's UI thread, as on the Mac, and
  `post_wakeup` posts to it as before.
- `onDestroy` ends the process. Go's state can't outlive the views it describes, and a new window in the same
  process would otherwise find managers describing a page that no longer exists.
- Go's own output goes nowhere on Android, because stderr is `/dev/null`. `log.Printf` isn't visible in `logcat`.

## 5. Does the transparent shell over the page work on Android?

**Yes, tested.** Touches need nothing new. Android offers a finger down to views from the top, and the one that
accepts it keeps the whole gesture, so the shell's view refuses when the point is outside the shell's rectangles and
the page gets it. That's the same question as the Mac's `hitTest:`.

Two details:

- `setMotionEventSplittingEnabled(false)` on the parent, or a second finger landing on the bar during a pinch would
  go to the shell.
- `navigator.maxTouchPoints` marks a phone, not `pointer: coarse`. Chromium calls any device with a stylus a fine
  pointer (`ui/android/java/.../TouchDevice.java`), and the emulator's touch screen says it has one, as an S Pen
  phone would.

**With one compositor for both, not two views.** `ContentViewRenderView` owns one
`SurfaceView` and shows one `WebContents`, and two stacked `SurfaceView`s won't blend: Android draws each as its own
hardware layer. So the window (`engine/android/window_android.cc`, `BrowserWindow.java`) is:

    FrameLayout
      SurfaceView      one compositor. Root layer -> shown page's layer, then shell's layer on top
      ContentView      one per tab, at the page's rectangle. Only the shown tab's is visible. Draws
                       nothing, takes touches and the keyboard
      ShellHolder      full window, refuses a finger down where the layout says page
        ContentView    shell. Draws nothing

Why it works, from the source:

- `components/thin_webview/internal/compositor_view_impl.cc` already builds a compositor over its own root layer
  with `web_contents->GetNativeView()->GetLayer()` as a child, so the layers aren't tied to one view.
- `ui/android/delegated_frame_host_android.cc` creates each `WebContents`' surface layer with
  `SetContentsOpaque(false)`, so a transparent shell can blend over the layer below.
- `ContentView.onTouchEvent` forwards through `EventForwarder` to its own `WebContents`, so the same
  `dispatchTouchEvent` refusal routes touches.

Only the shown tab's layer is in the compositor's tree, so a hidden tab costs no drawing; content is told it's
hidden, as on the Mac. The root layer is the frame colour, so it shows before the shell's first frame and under a
crashed page.

On the emulator: the bar and the suggestions draw over the page, a tap or a swipe just below the bar reaches the
page, the keyboard types into whichever was touched, and the page follows a rotation. The page's position uses the
same stale-size rule as the Mac, and each move sets both the layer's position and the view's margins.

## 6. The alternatives behind the same engine interface

| | System WebView | GeckoView | Our own content |
|---|---|---|---|
| Size of the APK | 2.5 MB | 91.5 MB AAR for arm64 alone, [version 159](https://maven.mozilla.org/maven2/org/mozilla/geckoview/geckoview-arm64-v8a/) | 122 MB, from a 199 MB unstripped, non-official library. An official build would be smaller; not measured |
| Engine version | Whatever the phone has: 113 on the Android 14 image, 133 on the Android 16 image | Ours, shipped with the app | Ours, pinned to the desktop build |
| Same engine as desktop | Blink, but not our version | No. Gecko on phones and Blink on desktop means testing every site twice | Yes |
| Renderers | One, for all views. `GetMaxRendererProcessCountOverride` returns 1 and `ShouldDisableSiteIsolation` returns true (`android_webview/browser/aw_content_browser_client.cc`) | Its own multi-process | Separate shell and page, with site isolation |
| A page spinning | Freezes the shell too. Tested | Separate, if the shell isn't Gecko | The shell keeps working. Tested |
| Network stack | Theirs. No hook below the request level | Theirs (Necko) | Ours, including certificates, proxy and DNS |

**URL policy on every request is what decides it.**

| Request | WebView | GeckoView | Content |
|---|---|---|---|
| Top-level GET a page starts | `shouldOverrideUrlLoading` | `NavigationDelegate.onLoadRequest` | `NavigationThrottle` |
| Top-level the app starts (`loadUrl`) | Not offered: "[this is not called for navigations which the app initiated](https://developer.android.com/reference/android/webkit/WebViewClient#shouldOverrideUrlLoading(android.webkit.WebView,%20android.webkit.WebResourceRequest))". We ask the managers first ourselves | Yes | `NavigationThrottle` |
| Back and forward | Not offered. We ask before moving | Not checked here | `NavigationThrottle` |
| Redirect | Offered, GET only | Not checked here | `NavigationThrottle` |
| Top-level POST | **Not offered**, in `AwContentBrowserClient::ShouldOverrideUrlLoading` ("Only GETs can be overridden"). Tested: a form reached the network past the gate | Not checked here | `NavigationThrottle`. Tested: the same form is blocked |
| Iframes and subresources | `shouldInterceptRequest`, on a background thread, and it can only answer with a substitute response. Doesn't see the POST body | Through a built-in WebExtension's [`webRequest`](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/webRequest/onBeforeRequest), which can cancel | `URLLoaderThrottle` and `WillCreateURLLoaderFactory` |
| WebSocket, WebTransport, WebRTC | **No hook.** `shouldInterceptRequest` isn't called for these | Not checked here | `CreateWebSocket` and friends on `ContentBrowserClient` |
| Service worker fetches | A separate `ServiceWorkerController` client | Not checked here | Same path as other requests |
| Downloads | `DownloadListener`, after the fact | `onExternalResponse` | Our `DownloadManagerDelegate` |
| Proxy or SWG steering | [`ProxyController`](https://developer.android.com/reference/androidx/webkit/ProxyController), one setting for all WebViews in the app | Runtime and extension proxy settings | Per-context `NetworkContext` |

Other things content gives us that WebView can't: the same native code as the desktop build, so a feature only
gets written once; client certificates and device trust below the WebView API; session and cookie control; the
extension system if we ever want it; and no `; wv` in the user agent. Google's sign-in refuses embedded WebViews.

**Recommendation: our own content on Android, on the same interface as the Mac.** It's built and it works. WebView was
built first, as a stopgap, and dropped: the holes above are in the platform and we can't close them. GeckoView would close most of them, but puts a second engine
under every site we support.

## 7. How the existing Ulaa browser does it

Ulaa is a different animal: it builds Chromium's whole `chrome` layer for Android, `chrome_public_apk` or
`chrome_public_bundle`, with its overlay on top (`src/scripts/utils/build_utils.py`), and its Android code is
Java inside `org.chromium.chrome.browser` (`docs/android/Guide.md`). It runs on Chromium's own Android host rule,
so the same Linux requirement applies, and `target_devices = arm64 x86` in `src/scripts/config/config.ini` picks the CPU.
Project B has none of that layer, so nothing there is reusable except the signing and the knowledge that the
pinned version builds.

## The content build

| Folder or file | What it is |
|---|---|
| `engine/android/BUILD.gn` | The APK: the content library, the Go libraries, the shell and the pak as assets |
| `engine/android/window_android.cc` | `Window` on Android: one compositor, the shell's layer over the shown page's |
| `engine/android/java/.../BrowserWindow.java` | The views: the surface, a `ContentView` per tab and the shell's, the touch rule, the frame colour |
| `engine/android/java/.../BrowserActivity.java` | Starts content, then the window, then Go |
| `engine/android/java/.../BrowserApplication.java` | Runs first in every process, as content needs |
| `engine/android/java/.../GoCore.java`, `go_table.c` | The one load of Go, and its table |
| `engine/android/library_loader.cc` | `JNI_OnLoad` for every process |
| `engine/android/build_go.py` | `core/` as `libcore.so`, with Chromium's clang and the NDK's sysroot |
| `tools/android-content-fetch.sh` | Chromium 155 for Android and its Linux build dependencies, once |
| `tools/android-content-build.sh` | `gn gen` and the build, inside the container |
| `tools/android-content-package.sh` | Copies the APK out, compresses its libraries, signs it on the Mac |
| `tools/android-release.sh` | For `tools/release.sh`: copies a checkout's tracked files into the container, builds, signs, and stops the VM |

The rest of the adapter is shared with the Mac: `engine/browser/` and `engine/common/`, with `BUILDFLAG` guards. The
ones that change behaviour on Android: the pak comes from the APK and goes to each child as an open file, the shell's
files come from the APK's assets, pages are handed over as `WebContents` rather than native views, `RunProcess`
starts the browser and returns, because Android's looper is the message loop, and the shell's delegate opens
Android's colour picker.

To build, with the shell built first on the Mac (`(cd shell && npm run build)`), because the APK takes `shell/dist`:

    colima start -p pb-android                    # the VM, if it's stopped
    DOCKER_HOST=unix://$HOME/.colima/pb-android/docker.sock docker start pb
    amarit restart pb-android-build               # tools/android-content-build.sh in the container
    tools/android-content-package.sh              # build/android/project-b-content.apk, signed

The build copies the tree it's given (`REPO`, `/repo` by default) into `/work/tree` in the container, rewriting a
file only when its content changes, and GN builds that copy. A release builds another checkout than the one the
container mounts, and ninja goes by file times, so building either one in place could keep a stale output.

The VM and container were made with `colima start -p pb-android --vz-rosetta --arch aarch64 --cpu 24 --memory 96
--disk 230` and `docker run -d --name pb --platform linux/amd64 -v pb-work:/work -v <this worktree>:/repo ubuntu:24.04
sleep infinity`; the checkout lives in the `pb-work` volume.

Minimum Android 10 (API 29), arm64 only. Libraries are compressed in the APK to halve the download, so Android
unpacks them at install, which costs about 200 MB of phone storage. The version is `VERSION`'s, for the APK's
`versionName` and for Go's updater, and the `versionCode` grows with it: 0.2.13 is 2013.

### What was tested

Before tabs, `tools/drive_android.py`, on an Android 14 emulator and an Android 16 emulator with 16 KB memory pages,
both headless, 29 checks, all passing on both. For this shell, see [The shell on a phone](#the-shell-on-a-phone). Results and screenshots land in `build/android-drive-content`.

- Typing a URL, a tap on a link, back, forward, reload, the phone's back key.
- Suggestions from history, over the page, with the keyboard up. A tap on one loads it.
- History surviving a force stop. An emoji in a title reaching Go intact.
- A tap and a swipe just below the bar going to the page. Typing into a form in the page. Holding a finger on text.
  A tap in landscape.
- A blocked host, typed, through a redirect, and as a form POST. A failed load staying out of history.
- Go in the main process only: not in either renderer, the GPU process or the app zygote.
- Separate renderers for the shell and the page. A page in an endless loop while the bar takes typing.
- The page's renderer killed: the bar says Crashed, and reload recovers. The shell's renderer killed: it comes back
  and works. The GPU process killed: the app keeps working.
- Real sites over the office network: `example.com`, Wikipedia.

Measured on the Android 16 emulator, before tabs, both builds showing `example.com`, one run each:

| | Content | WebView |
|---|---|---|
| Window on screen, `am start -W`, cold then warm | 1.05 s, then 0.46 to 0.48 s | 0.48 s, then 0.24 to 0.25 s |
| Main process RSS, threads | 244 MB, 50 | 211 MB, 60 |
| GPU process | 195 MB | Inside WebView's own processes |
| Renderers | Two, 190 MB each | One, 228 MB, owned by the WebView package |
| App zygote | 98 MB | Shared `webview_zygote`, 59 MB |

The window being on screen isn't the shell being painted, which wasn't measured. The content build is non-official,
with DCHECKs off, so an official build should be a little faster and smaller.

Content-only things not tested: a real phone and its GPU, battery, video, file upload, downloads, permission
prompts (none are wired), `target=_blank`, and Android 10 to 13. Hover and mouse wheel aren't routed by the hit
test, only touches.

One driver trap: Android takes the app zygote down a few seconds after a force stop. A start inside that window
reused the dying zygote once and hung until it was killed, after which no renderer would start. The driver waits for
it to go. A user won't force-stop and relaunch inside a few seconds, but it's worth knowing.

## Signing

The APK is signed with `~/PW_TOOLS/project-b-signing/android.keystore`, made once by `tools/android-signing-setup.sh`,
password in `~/PW_TOOLS/lab-secrets.env` as `PROJECT_B_ANDROID_KEYSTORE_PASSWORD`. Losing the key forces every phone to
uninstall before it can update.

## The shell on a phone

The branch sits on `main` at 0.2.13. Everything the Mac app's shell does goes through the same managers and the same
adapter here, with `Window` as the one Android file (`engine/android/window_android.cc`).

| On `main` | On Android |
|---|---|
| Tabs: create, show, close, favicons, a page opening a tab | The same managers. Each tab has its own `ContentView`; `ShowTab` swaps which page layer the compositor draws. The shell shows them differently: see below |
| Content's hooks, carried to Go by the generated classes | The same classes. Eight hooks content only declares off Android (five for window controls from the web, three others) are left out by the same platform condition content uses: `tools/gen_hooks.py` copies it into `hooks_gen.h` and `hooks_gen.cc` |
| Settings: tab layout, frame colour, version, updates | The same panel, without the tab layout. The custom colour opens Android's own colour picker, because Blink on Android has none inside the page; the shell's delegate answers `OpenColorChooser` with the one Chromium's embedders share (`components/embedder_support/android/delegate`) |
| The frame colour | Paints the root layer, the window's background and the status bar, and changes live with the shell |
| The window manager's one layout | `SetWindowLayout` places every tab's view and gives `BrowserWindow.java` the regions; the first region holding a finger decides |
| `WindowState` | The window's size in CSS pixels, no buttons, square corners, and `full_screen` true |
| Updates | The same manager and the same `amarit.Client`, every minute, on the `android/arm64` asset. The APK is downloaded into the profile, size and sha256 checked by amarit, and the bubble and the settings panel offer to install it: see [Updates on a phone](#updates-on-a-phone) |

**Tabs on a phone.** A sidebar would take half a phone's width, so on a touch screen (`phone` in `shell/src/state.js`)
the tabs are favicons in a row of their own under the toolbar (`TabIcons.vue`), the shown one marked as in the sidebar;
a tap shows that tab, and a tab with no favicon yet shows its title's first letter. The sidebar button opens the
sidebar's own list as a drawer over the toolbar and the page: choosing a tab or New Tab closes it, a tap outside closes
it, and a tab's close button leaves it open. Every tab's close button shows, since there's no hover. The tab layout
setting and the toolbar's layout button aren't shown, because neither layout applies. Desktop is unchanged.

### Updates on a phone

`core/update` does on the phone what it does on the Mac, up to the download: the same check every minute, the
same switch, the same Check for updates, against the same `releases.json`, for the `android/arm64` asset. What
changes is what happens to the download (`Config.Install`, set in `core/cmd/lib/update_android.go`):

- The APK is staged as it was downloaded, in `profile/updates/stage`. There is no bundle to unpack or version to
  read from it, and no writable-location check.
- The bubble says "Install to update", the settings panel says the release is ready to install, and its button
  becomes Install update. Either one hands the APK to Android's package installer: Go calls the engine's
  `install_update`, and `BrowserWindow.installPackage` opens it with `ACTION_VIEW` through a `FileProvider` that may
  read that one folder (`res/xml/updates.xml`). The app declares `REQUEST_INSTALL_PACKAGES`.
- Android does the rest. The first time, it asks the user to let Project B install apps; then it shows its own
  install dialog, refuses an APK signed with another key, and replaces the app once the user agrees. Nothing is
  installed silently. A user who cancels can tap again: the release stays staged until the next start.
- Trust is the signature: the phone only accepts an update signed with the key the installed app has.

The real app always checks the published manifest. It was checked once on the emulator against a local manifest
through a launch extra that has since been removed: 0.2.13 found 0.2.14, offered it, and Android's installer
replaced it.

What doesn't apply on a phone, and what the app does instead:

- **Dragging the window.** It fills the screen and never moves, which is what full screen means to the window manager,
  so it sends no drag areas.
- **Window buttons.** There are none, so the shell leaves its 8px ends. Minimise, which nothing on a phone sends, moves
  the app to the background; zoom and full screen do nothing, because the window already fills the screen. Close
  finishes the activity, which ends the process, as closing the last tab does.
- **The frame's band.** The shell's own edge strips draw it. With square corners there is no curve for the window to
  draw over the page, so `frame_width` isn't used.
- **The window title.** A phone doesn't show one.
- **DevTools and the driver.** The remote debugging endpoint and its window commands, and `driver.mm`, stay on the Mac.
- **Keyboard shortcuts.** Go reserves Command combinations, plus Control+Tab; a phone's keyboard was not tried.

Open:

- **The toolbar's middle doesn't paint while settings is open.** On Android only, in either layout: back, forward,
  reload and the address field stay laid out and hit-testable, but Blink doesn't draw them until the panel closes.
  The shell's own screenshot (DevTools' `Page.captureScreenshot`) shows the same, and the compositor holds no extra
  layers, so it's in Blink's painting, not our window. Lowering the panel's `z-index`, or raising the header's above
  it, makes it paint; the cause wasn't found, so neither is applied. Desktop Chromium, on the mock engine at phone width
  with touch, paints it; the Mac app wasn't checked.
- **The colour before Go starts.** The window shows the default frame colour from `styles.xml` until Go's first
  layout, so a phone set to another colour sees the default for about a second at a cold start.

`tools/drive_android.py` taps fixed points of the shell from before tabs, so it needs new coordinates before it
checks this shell. This round was checked by hand on the Android 14 emulator: the app starts, a typed address loads
`example.com`, New Tab opens a second tab that loads Wikipedia with its favicon, tapping the first tab shows its page
again, settings opens in both layouts, a preset frame colour changes the frame, the shell and the status bar at
once and survives a restart, the custom colour opens the picker and applies what it sets, and the layout switches
to tabs on top.
