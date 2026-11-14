# Web features whose browser side is above content

One row per web-platform feature (a JS API or a page-visible behaviour) that Chrome makes work
from `//chrome` or `//components`, and Project B either lacks or only half has. Read from
Chromium 155.0.8059.41 and this repo at 0.2.61 (HEAD 9555f69), 2026-11-14.

## How to read it

**What a page sees** is one of:

- **works**: does what Chrome does.
- **quiet**: fails quietly. The call rejects, returns empty, resolves to "denied", or never
  answers. The page carries on.
- **absent**: the API isn't there (`typeof` is `undefined`), or it throws at once.
- **KILLS**: content kills the page's renderer for a bad Mojo message. Every tab in that
  renderer process shows as crashed.
- **unknown**: not run, and the code doesn't settle it.

**Mac** and **Android** are given apart where they differ.

**Needs** is one of: a Go manager; an engine hook (named); a native panel or prompt; or
**stay off** (turn the Blink runtime feature off, as 7025aec does).

**Size**: **S** under a day. **M** a few days: a hook, a Go manager and one prompt. **L** weeks,
or a service we don't have.

**Evidence**: `C:` is a path under `chromium/src`. `E:` is a path in this repo. "Probe" is the
Android 17 probe (`~/PW_TOOLS/project-b-suites-0.2.57/probe-0.2.57.txt` before 7025aec,
`probe-fix.txt` after). "Basics N" is check N in `~/PW_TOOLS/project-b-suites-0.2.59/basics.txt`
(macOS 0.2.59). "Mac probe" is a run made for this research on the packaged 0.2.59 Mac app,
through the driver; its script and output are listed under [Mac probe](#the-mac-probe).

The **ids** in the first column are the ones a registry would use ([tracking.md](tracking.md)).

## Why things break: the three mechanisms

1. **A missing binder.** Blink asks the browser for a Mojo interface. Content looks it up in
   the frame's binder map: content's own list (`C:content/browser/browser_interface_binders.cc:883`,
   `PopulateBinderMapWithContext`), then the embedder's
   (`ContentBrowserClient::RegisterBrowserInterfaceBindersForFrame`, called last at
   `C:content/browser/browser_interface_binders.cc:1483`). Chrome fills its part in
   `PopulateChromeFrameBinders` (`C:chrome/browser/chrome_browser_interface_binders.cc:447`).
   Our plain `ContentBrowserClient` doesn't override it at all (`E:engine/browser/content_browser_client.h:16-49`);
   the extensions one adds binders only for `chrome-extension:` frames
   (`E:engine/extensions/browser/content_hooks.cc:242-253`). With no binder, content calls
   `ReportNoBinderForInterface` (`C:content/browser/renderer_host/render_frame_host_impl.cc:8073`)
   and the renderer dies. Workers do the same (`C:content/browser/service_worker/service_worker_host.cc:288`).
   Content registers **empty binders** for a few interfaces it knows Blink asks for
   (`C:content/browser/browser_interface_binders.cc:889-902`); those never kill, the call just
   gets no answer.
2. **A missing delegate.** Content binds the interface, then asks a delegate
   (`WebContentsDelegate`, `ContentBrowserClient`, or one of `BrowserContext`'s) what to do. The
   default is "no", "not supported", or nothing. Our `BrowserContext` returns `nullptr` for every
   delegate it can (`E:engine/browser/browser_context.cc:28-85`). The generated hook classes
   (`E:engine/browser/hooks_gen.h`) carry many delegate methods to Go, but Go subscribes to only
   fifteen of them (`E:core/content/wire.go:71-165`); the rest run content's default.
3. **A missing permission answer.** `BrowserContext::GetPermissionControllerDelegate` returns
   `nullptr` (`E:engine/browser/browser_context.cc:58-61`). Content then answers **DENIED** to every
   status check and every request (`C:content/browser/permissions/permission_controller_impl.cc:557-561`,
   `:623-625`, `:648-650`). No prompt is ever shown.

## A. Renderer kills and Mojo binders

| id | Feature | Mac | Android | Why | Needs | Size | Evidence |
|---|---|---|---|---|---|---|---|
| `badge` | `navigator.setAppBadge`, `clearAppBadge` in a page | **KILLS** (Mac probe: the promise resolves, then the renderer dies) | works (Blink skips the call on Android) | No `blink.mojom.BadgeService` binder. Chrome binds it on desktop only. The API has no runtime feature: it can't be turned off | Engine hook: an empty binder in `RegisterBrowserInterfaceBindersForFrame`, on the plain `ContentBrowserClient`. Later a Go manager to set the Dock badge | S (M for the Dock) | `C:chrome/browser/chrome_browser_interface_binders.cc:522`; `C:third_party/blink/renderer/modules/badging/navigator_badge.cc:115-118,159,173-176`; `navigator_badge.idl` has no `RuntimeEnabled`; Mac probe |
| `badge-sw` | `navigator.setAppBadge` in a service worker (push handlers) | **KILLS** the worker's process (from code) | works (skipped) | No `BadgeService` in the service-worker map. Chrome adds it in `RegisterBrowserInterfaceBindersForServiceWorker`; we don't override that | Engine hook: `ContentBrowserClient::RegisterBrowserInterfaceBindersForServiceWorker`, an empty binder | S | `C:chrome/browser/chrome_content_browser_client_receiver_bindings.cc:381`; `C:third_party/blink/renderer/modules/badging/worker_navigator_badge.idl:10-13`; `C:content/browser/browser_interface_binders.cc:1816-1980` (no BadgeService) |
| `speech-on-device` | `SpeechRecognition.available({processLocally: true})`, `SpeechRecognition.install()` | **KILLS** (Mac probe) | **KILLS** (from code; same path, not in the Android probe) | No `media.mojom.OnDeviceSpeechRecognition` binder. Chrome binds it on every platform. Runtime features OnDeviceWebSpeechAvailable and InstallOnDeviceSpeechRecognition are stable | Stay off: add both features to 7025aec's list. Or an empty binder (then `available()` never answers) | S | `C:chrome/browser/chrome_browser_interface_binders.cc:552`; `C:third_party/blink/renderer/modules/speech/speech_recognition_controller.cc:160-168`; `speech_recognition.idl:78-87`; `runtime_enabled_features.json5:4464` and `:3763`; Mac probe |
| `payment-link` | `<link rel="facilitated-payment" href=…>` in a main frame | no call (Android only) | **KILLS** (from code) | No `payments.facilitated.mojom.PaymentLinkHandler` binder. Blink asks as soon as the link is in the document. PaymentLinkDetection is stable | Stay off: add PaymentLinkDetection to 7025aec's list | S | `C:third_party/blink/renderer/core/html/html_link_element.cc:633-641`; `C:third_party/blink/renderer/core/dom/document.cc:10207-10223`; `C:chrome/browser/chrome_browser_interface_binders.cc:602`; `runtime_enabled_features.json5:4705` |
| `view-source-wrap` | The "Line wrap" box on a `view-source:` page | **KILLS** if `view-source:` loads at all (unknown whether a tab can reach it) | no call | No `blink.mojom.PersistentRendererPrefsService` binder; chrome binds it on desktop | Engine hook: empty binder (or keep `view-source:` closed) | S | `C:third_party/blink/renderer/core/html/html_view_source_document.cc:77-88`; `C:chrome/browser/chrome_browser_interface_binders.cc:524` |
| `installed-apps` | `navigator.getInstalledRelatedApps` | absent (since 7025aec; before it, content's own desktop binder answered empty) | absent (**killed** before 7025aec: DuckDuckGo) | Content binds `InstalledAppProvider` on desktop only; chrome binds it on Android | Stay off (done) | S, done | `C:content/browser/browser_interface_binders.cc:1304`; `C:chrome/browser/chrome_browser_interface_binders.cc:500`; `runtime_enabled_features.json5:3747`; probe 0.2.57 DEAD, probe-fix ok |
| `digital-goods` | `getDigitalGoodsService` | absent (status "test" off Android) | absent (**killed** before 7025aec) | No `DigitalGoodsFactory` binder; chrome forwards it to Play Billing in Java | Stay off (done) | S, done | `C:chrome/browser/chrome_browser_interface_binders.cc:502`; `C:third_party/blink/renderer/modules/payments/goods/dom_window_digital_goods.cc:110`; `runtime_enabled_features.json5:2309`; probe 0.2.57 DEAD |
| `sub-apps` | `navigator.subApps` | absent | absent | Isolated Web Apps only; `navigator.subApps` was already undefined before 7025aec (probe 0.2.57) | Stay off (done; harmless) | S | `C:chrome/browser/chrome_browser_interface_binders.cc:575`; `runtime_enabled_features.json5:6056` |
| `web-share` | `navigator.share`, `canShare` | absent (Mac probe) | absent (probe, before and after 7025aec) | WebShare's status is "test"; only chrome's renderer client turns it on. It was never on in Project B | Native share sheet (`NSSharingServicePicker`, Android `Intent.ACTION_SEND`) behind a `blink.mojom.ShareService` binder; until then stay off | M | `runtime_enabled_features.json5:7181`; `C:chrome/renderer/chrome_content_renderer_client.cc:1484-1490`; `C:chrome/browser/chrome_browser_interface_binders.cc:544,547` |
| `payment-request` | `PaymentRequest` | quiet: constructor exists, `canMakePayment()` rejects UnknownError (Mac probe: `function`) | quiet: same reject (probe, before and after 7025aec) | Content has an empty binder, so it never kills. **7025aec doesn't turn it off**: the WebPayments feature sets it again through a web preference after the command line | Stay off properly with `--disable-features=WebPayments`, or leave it | S | `C:content/browser/browser_interface_binders.cc:891-892`; `C:content/browser/web_contents/web_contents_impl.cc:4094-4095`; `C:third_party/blink/renderer/core/exported/web_view_impl.cc:1972-1973`; probe-fix |
| `web-printing` | `navigator.printing` | absent | absent | Status "test" (ChromeOS only) | Stay off | S | `C:chrome/browser/chrome_browser_interface_binders.cc:596`; `runtime_enabled_features.json5:7158` |
| `credentials-password` | `navigator.credentials.get/store` with passwords or federated | quiet: rejects NotSupportedError | quiet: same (probe) | Content's empty `CredentialManager` binder; chrome's password manager answers | Go password manager plus a save prompt | L | `C:content/browser/browser_interface_binders.cc:895-896`; `C:chrome/browser/chrome_browser_interface_binders.cc:493`; probe `passwordCredential` |
| `web-install` | `navigator.install()`, `<install>` | unknown: empty binder, so likely never answers | same | Content's empty `WebInstallService` binder | Stay off, or a Go web-app manager | S / L | `C:content/browser/browser_interface_binders.cc:901-902` |
| `nostate-prefetch` | `<link rel=prerender>` | quiet: no-op | quiet: no-op | Content's empty binder; chrome does NoState Prefetch | Nothing (speculation rules prerender is content's and still works) | – | `C:content/browser/browser_interface_binders.cc:889-890` |
| `ai-prompt` | `LanguageModel`, `Summarizer`, `Writer`, `Rewriter`, `Proofreader` | **wrong**, counted as quiet: present, and `LanguageModel.availability()` says "downloadable" (Mac probe). Behind it is content's test echo, which answers prompts by echoing them | absent (status off on Android; probe) | Content's default `BindAIManager` is `EchoAIManagerImpl`, written for web tests | Stay off (AIPromptAPI, AISummarizationAPI, AIWriterAPI, AIRewriterAPI, AIProofreaderAPI), or override `ContentBrowserClient::BindAIManager` to say "unavailable" | S | `C:content/public/browser/content_browser_client.cc:1879-1885`; `C:content/browser/ai/echo_ai_manager_impl.cc:140-160`; `runtime_enabled_features.json5:409,514`; Mac probe |
| `ai-translator` | `Translator`, `LanguageDetector` | quiet: `Translator.availability()` never answers (Mac probe: nothing in 4 s) | absent | Content's default `BindTranslationManager` drops the receiver | Stay off (TranslationAPI, LanguageDetectionAPI) | S | `C:content/public/browser/content_browser_client.cc:1887-1892`; `runtime_enabled_features.json5:6504,3835`; Mac probe |

Checked and safe: the rest of chrome's frame binders are asked for only by chrome's own renderer
code, which we don't run (translate, spellcheck, network hints, distiller, image annotation,
OpenSearch, offline pages, screen AI, `glic`, `pwc`, `geic`, PDF help bubble), or content has an
empty binder for them (`AnchorElementMetricsHost`, `LCPCriticalPathPredictorHost`,
`ScriptToolHost`, `ModelBroker`, `UnhandledTapNotifier`, the speech-service ones on desktop).
`MediaControlsMenuHost` (Android) has no caller in Blink 155. Associated interfaces and
process-level receivers that nobody binds are dropped, never fatal.

## B. Delegates: dialogs, files, downloads, prompts, devices

| id | Feature | Mac | Android | Why | Needs | Size | Evidence |
|---|---|---|---|---|---|---|---|
| `js-dialogs` | `alert`, `confirm`, `prompt` | quiet: return at once (basics 13) | quiet (same code, not run) | No `JavaScriptDialogManager` | Engine hook: `WebContentsDelegate::GetJavaScriptDialogManager`; a shell or native dialog | M | `C:content/public/browser/web_contents_delegate.cc:172-175`; `E:engine/browser/page_tab.h:57-75` (not overridden); basics 13 |
| `beforeunload` | "Leave this page?" on close | quiet: tab closes without asking (basics 1) | quiet (not run) | Same manager's `RunBeforeUnloadDialog` | With `js-dialogs` | M | basics 1 |
| `file-input` | `<input type=file>` | quiet: cancels at once (basics 14) | unknown: the probe marked it DEAD with no kill in the log and no report at all | Default `RunFileChooser` cancels | Engine hook: `WebContentsDelegate::RunFileChooser` and `EnumerateDirectory`; `NSOpenPanel`, Android `ACTION_OPEN_DOCUMENT` | M | `C:content/public/browser/web_contents_delegate.cc:275-280`; basics 14; probe `fileInput` |
| `fs-access` | `showOpenFilePicker`, `showSaveFilePicker`, `showDirectoryPicker` | unknown (needs a gesture) | unknown (needs a gesture; probe only saw the gesture error) | Needs `BrowserContext::GetFileSystemAccessPermissionContext`, which we don't provide | Engine hook plus the file panel above; or stay off | M | probe `showOpenFilePicker` |
| `downloads` | `<a download>`, `Content-Disposition: attachment` | quiet: no file arrives (basics 15) | quiet (not run) | `GetDownloadManagerDelegate` returns `nullptr` | Go downloads manager; engine hook `DownloadManagerDelegate::DetermineDownloadTarget`; a downloads UI | L | `E:engine/browser/browser_context.cc:28-30`; basics 15 |
| `http-auth` | 401 and proxy auth | quiet: the 401 page shows (basics 12) | quiet (not run) | Default `CreateLoginDelegate` returns `nullptr` | Engine hook `ContentBrowserClient::CreateLoginDelegate`; a password prompt | M | `C:content/public/browser/content_browser_client.cc:1309`; basics 12 |
| `cert-error` | A bad certificate | quiet: an empty page (basics 10) | quiet (not run) | Default `AllowCertificateError` denies; no interstitial; `SSLHostStateDelegate` is `nullptr` | Engine hook `ContentBrowserClient::AllowCertificateError`; an error page; `BrowserContext::GetSSLHostStateDelegate` for "proceed anyway" | M | `C:content/public/browser/content_browser_client.cc:722`; `E:engine/browser/browser_context.cc:54-56`; basics 10 |
| `client-cert` | TLS client certificates | unknown (basics 11 not written) | unknown | Default `SelectClientCertificate` goes on without one | Engine hook `ContentBrowserClient::SelectClientCertificate`; a picker | M | `C:content/public/browser/content_browser_client.cc:740` |
| `popup-block` | `window.open` without a click | quiet the wrong way: opens (basics 6) | same (not run) | Default `CanCreateWindow` says yes | Engine hook `ContentBrowserClient::CanCreateWindow` with the user-gesture check; a "blocked" note | S-M | `C:content/public/browser/content_browser_client.cc:801-816`; basics 6 |
| `fullscreen` | `element.requestFullscreen()`, video full screen | quiet: stays pending (basics 18) | quiet (probe only checked without a gesture) | `EnterFullscreenModeForTab` is carried to Go but nobody subscribes; the default does nothing | Go subscription in `E:core/content/wire.go`; window full screen on Mac, immersive mode on Android | M | `C:content/public/browser/web_contents_delegate.h:542-544`; `E:engine/browser/hooks_gen.h:553`; basics 18 |
| `orientation-lock` | `screen.orientation.lock()` | n/a | quiet: SecurityError, needs full screen (probe) | Depends on `fullscreen` | With `fullscreen`, then the Activity's orientation | S after `fullscreen` | probe `orientationLock` |
| `pip-video` | `video.requestPictureInPicture()` | quiet: NotSupportedError (basics 36) | quiet (probe: rejects) | Default `EnterPictureInPicture` says not supported | Engine hooks `WebContentsDelegate::EnterPictureInPicture` and `ContentBrowserClient::CreateWindowForVideoPictureInPicture`; a native floating window | L | `C:content/public/browser/web_contents_delegate.cc:418-421`; basics 36; probe `pip` |
| `pip-document` | `documentPictureInPicture.requestWindow()` | unknown (stable on Mac) | absent (off on Android) | Chrome opens a browser window for it | Stay off until `pip-video` | S | `runtime_enabled_features.json5:2467` |
| `camera-mic` | `getUserMedia` | quiet: NotSupportedError, no prompt (basics 17) | quiet: NotSupportedError (probe) | Default `RequestMediaAccessPermission` says not supported; `CheckMediaAccessPermission` says no | Engine hook `WebContentsDelegate::RequestMediaAccessPermission` (needs a `MediaStreamRequest` copier and the callback mechanism); a prompt; the OS permission (TCC on Mac, runtime permission on Android) | L | `C:content/public/browser/web_contents_delegate.cc:295-322`; basics 17; probe `getUserMedia` |
| `screen-capture` | `getDisplayMedia` | quiet: NotSupportedError (from code) | absent (off on Android) | Same as `camera-mic`, plus chrome's picker | With `camera-mic`, plus a source picker | L | as above |
| `media-labels` | `enumerateDevices()` | works, without labels (from code) | works (probe) | Labels only after a grant | With `camera-mic` | – | probe |
| `context-menu` | Right-click or long press on a page | partly: Go answers for links and images (`wire.go`); Standard has no menu yet; basics 22 fails | works for links and images: the Mobile shell's menu (probe-37 0.2.59) | `HandleContextMenu` is carried and subscribed; a plain right-click falls through to content | The shell menu on desktop (branch `right-click`) | M | `E:core/content/wire.go:108-118`; basics 22; `probe-37-0.2.59.txt` |
| `color-input` | `<input type=color>` | quiet: nothing opens (from code) | quiet (from code) | Default `OpenColorChooser` returns `nullptr` | Engine hook `WebContentsDelegate::OpenColorChooser`; `NSColorPanel` or a shell picker | S-M | `C:content/public/browser/web_contents_delegate.cc:261-266` |
| `eye-dropper` | `new EyeDropper().open()` | quiet: exists (Mac probe), `open()` gets no dropper (from code) | absent | Default `OpenEyeDropper` returns `nullptr` | Stay off, or `WebContentsDelegate::OpenEyeDropper` | S | `C:content/public/browser/web_contents_delegate.cc:269-273`; Mac probe |
| `pointer-lock` | `element.requestPointerLock()` | quiet: rejects (from code) | quiet (from code) | `RequestPointerLock` is carried, not subscribed; the default answers kUnknownError | Go subscription; Esc-to-exit note | S-M | `C:content/public/browser/web_contents_delegate.cc:239-244` |
| `keyboard-lock` | `navigator.keyboard.lock()` | works (from code: default accepts) | works (probe) | – | Decide whether to keep Cmd+W and friends reserved (they are, in Go) | – | `C:content/public/browser/web_contents_delegate.cc:251-256`; probe |
| `protocol-handler` | `navigator.registerProtocolHandler` | quiet: no-op | quiet: no-op | `RegisterProtocolHandler` is carried, not subscribed | Go manager for handlers, a prompt, and a URL rewrite in the navigation gate | M | `E:engine/browser/hooks_gen.h:600`; `C:content/public/browser/web_contents_delegate.h:580` |
| `external-protocol` | Links to `mailto:`, `tel:`, `zoommtg:`, `intent:` | quiet: nothing happens (from code) | quiet: nothing happens (from code). Hurts more on a phone | Default `HandleExternalProtocol` says "handled" and does nothing | Engine hook `ContentBrowserClient::HandleExternalProtocol` (callback group); `NSWorkspace openURL`, Android `Intent` with a confirm | M | `C:content/public/browser/content_browser_client.cc:1324-1338` |
| `print` | `window.print()`, Cmd+P | quiet: returns at once (daily-use 13) | quiet (not run) | Printing is chrome's: a renderer helper plus print preview | `//components/printing` renderer and browser parts, or a native print of a snapshot | L | `docs/daily-use.md` on `daily-use`, row 13 |
| `webauthn` | `navigator.credentials.create/get({publicKey})` | unknown: the Mac probe on 127.0.0.1 failed earlier, on the RP ID | quiet: "Error connecting to Web Authentication service" (probe-fix) | No authenticator UI or request delegate; on Mac, Touch ID needs a keychain access group | Engine hooks `ContentBrowserClient::GetWebAuthenticationRequestDelegate` and `GetWebAuthenticationDelegate`; prompts; Android needs Play Services | L | `C:content/public/browser/content_browser_client.cc:1298`; probe-fix `webauthn` |
| `fedcm` | `navigator.credentials.get({identity})` | unknown | quiet: NetworkError or timeout (probe) | No `IdentityRequestDialogController` | Stay off (FedCm), or engine hook `CreateIdentityRequestDialogController` and an account picker | S / L | `C:content/public/browser/content_browser_client.cc:1643`; probe `fedcm` |
| `web-otp` | `navigator.credentials.get({otp})` | n/a | quiet: times out (probe) | `CreateSmsPrompt` does nothing; the SMS retriever is Play Services | Stay off (WebOTP) | S | `C:content/public/browser/web_contents_delegate.cc:177-182`; probe `otp` |
| `contacts` | `navigator.contacts.select()` | absent (Android only) | quiet: "Unable to open a contact selector" (probe) | No contacts picker in Java | Stay off (ContactsManager) or a Java picker | S / M | probe `contacts` |
| `bluetooth` | `navigator.bluetooth` | unknown (stable on Mac; `BluetoothDelegate` is `nullptr`) | quiet: "not supported on this platform" (probe) | No `BluetoothDelegate`, no chooser | Stay off (WebBluetooth) | S | `C:content/public/browser/content_browser_client.cc:1254`; probe |
| `usb` | `navigator.usb` | quiet (from code) | quiet: `getDevices()` is empty (probe) | No `UsbDelegate`, no chooser | Stay off (WebUSB) or a chooser | S / L | `C:content/public/browser/content_browser_client.cc:1258`; probe |
| `serial` | `navigator.serial` | quiet (from code) | quiet: empty (probe) | No `SerialDelegate` | Stay off (Serial) or a chooser | S / L | `C:content/public/browser/content_browser_client.cc:1246`; probe |
| `hid` | `navigator.hid` | quiet: `getDevices()` gives 0 (Mac probe) | absent (experimental on Android) | No `HidDelegate`: content never creates the service | Stay off (WebHID) or a chooser | S / L | `C:content/browser/hid/hid_service.cc:168-171`; Mac probe |
| `local-fonts` | `queryLocalFonts()` | quiet: 0 fonts (Mac probe) | absent | No `FontAccessDelegate`; LOCAL_FONTS denied | Stay off, or a permission answer | S | `C:content/public/browser/content_browser_client.cc:1266`; Mac probe |
| `presentation` | `PresentationRequest` | quiet (from code) | quiet: `getAvailability()` answers (probe) | No `ControllerPresentationServiceDelegate`; Cast is chrome's | Stay off (Presentation) | S | `C:content/public/browser/content_browser_client.cc:928`; probe |
| `speech-cloud` | `new SpeechRecognition().start()` | unknown | unknown | No `SpeechRecognitionManagerDelegate`; Chrome's server recognizer needs Google API keys | Stay off (ScriptedSpeechRecognition, UnprefixedSpeechRecognition) or a platform recognizer | S / L | `C:content/public/browser/content_browser_client.cc:820` |
| `speech-synthesis` | `speechSynthesis` | unknown | works: `getVoices()` answers (probe) | Content's own platform TTS | – | – | probe |
| `notifications` | `Notification`, `showNotification` | quiet: `requestPermission()` gives "denied" (basics 16) | quiet: same (probe) | NOTIFICATIONS denied; `GetPlatformNotificationService` is `nullptr` | Go notifications manager; `BrowserContext::GetPlatformNotificationService`; a prompt; OS notifications | M | `E:engine/browser/browser_context.cc:40-43`; basics 16 |
| `push` | `PushManager.subscribe` | quiet (from code) | quiet (probe rejected on the test key, which says nothing) | `GetPushMessagingService` is `nullptr`; Chrome's push rides Google's GCM | A push service of our own | L | `E:engine/browser/browser_context.cc:45-47` |
| `background-sync` | `SyncManager`, `periodicSync` | quiet (from code) | quiet: UnknownError (probe) | `GetBackgroundSyncController` is `nullptr` | Stay off (PeriodicBackgroundSync) | S | `E:engine/browser/browser_context.cc:77-80`; probe `periodicSync` |
| `background-fetch` | `BackgroundFetchManager` | unknown | unknown | `GetBackgroundFetchDelegate` is `nullptr` | Stay off (BackgroundFetch) | S | `E:engine/browser/browser_context.cc:73-75` |
| `client-hints` | `Accept-CH` remembered across loads | quiet: high-entropy hints never sent on later loads (from code) | same | `GetClientHintsControllerDelegate` is `nullptr` | Engine hook: a small `ClientHintsControllerDelegate` | M | `E:engine/browser/browser_context.cc:68-71` |
| `local-network` | A public page fetching `localhost` or a LAN address | quiet: blocked (from code) | same | Local Network Access checks are on; the LOCAL_NETWORK and LOOPBACK_NETWORK permissions are denied | A permission answer and a prompt | M | `C:services/network/public/cpp/features.cc:242`; `C:content/browser/storage_partition_impl.cc:2215-2304` |
| `sensors` | `deviceorientation`, `devicemotion`, `Accelerometer` | quiet (no sensors on most Macs) | quiet: events never fire (from code) | SENSORS denied | A permission answer (Chrome grants it by default) | S | `C:content/browser/generic_sensor/frame_sensor_provider_proxy.cc:127-136` |
| `clipboard-async` | `navigator.clipboard.writeText`, `readText` | quiet: NotAllowedError (Mac probe) | quiet: "Write permission denied" even after a tap (probe) | CLIPBOARD_SANITIZED_WRITE and CLIPBOARD_READ_WRITE denied. Many "Copy" buttons use this | A permission answer: grant sanitized write with a gesture, as Chrome does; prompt for read | S | probe `clipboardWrite`; Mac probe |

## C. Permissions

What our engine answers for each `blink::PermissionType`
(`C:third_party/blink/public/common/permissions/permission_utils.h:25-71`): **DENIED, always**,
because there is no `PermissionControllerDelegate`. A row is given only where the page sees
something worth knowing; the delegate fix for all of them is one class,
`BrowserContext::GetPermissionControllerDelegate`, backed by a Go permissions manager with a
prompt and per-site storage. **Size M** for the manager and prompt, then S per type.

| id | PermissionType | API | Page sees, both platforms | Chrome's default |
|---|---|---|---|---|
| `perm-geolocation` | GEOLOCATION, GEOLOCATION_APPROXIMATE | `navigator.geolocation` | quiet: error 1, "User denied" (probe, Mac probe) | prompt |
| `perm-notifications` | NOTIFICATIONS | `Notification.requestPermission` | "denied" (basics 16, probe) | prompt |
| `perm-midi` | MIDI, MIDI_SYSEX | `requestMIDIAccess` | NotAllowedError (probe) | MIDI granted, sysex prompt |
| `perm-persist` | PERSISTENT_STORAGE | `navigator.storage.persist()` | resolves false | heuristics |
| `perm-clipboard` | CLIPBOARD_READ_WRITE, CLIPBOARD_SANITIZED_WRITE | async clipboard | NotAllowedError (see `clipboard-async`) | write granted with focus; read prompts |
| `perm-wake-lock` | WAKE_LOCK_SCREEN, WAKE_LOCK_SYSTEM | `navigator.wakeLock.request` | NotAllowedError (probe) | screen granted |
| `perm-idle` | IDLE_DETECTION | `IdleDetector.requestPermission` | "denied" | prompt |
| `perm-nfc` | NFC | `NDEFReader.scan` (Android) | NotAllowedError (probe) | prompt |
| `perm-sensors` | SENSORS | device orientation and motion | events never fire (see `sensors`) | granted |
| `perm-window-mgmt` | WINDOW_MANAGEMENT | `getScreenDetails` | NotAllowedError (probe) | prompt |
| `perm-local-fonts` | LOCAL_FONTS | `queryLocalFonts` | empty (see `local-fonts`) | prompt |
| `perm-storage-access` | STORAGE_ACCESS_GRANT, TOP_LEVEL_STORAGE_ACCESS | `document.requestStorageAccess` | resolves when not needed (probe); a cross-site iframe that needs it is refused | heuristics, prompt |
| `perm-local-network` | LOCAL_NETWORK_ACCESS, LOCAL_NETWORK, LOOPBACK_NETWORK | fetch to private addresses | blocked (see `local-network`) | prompt |
| `perm-protected-media` | PROTECTED_MEDIA_IDENTIFIER | EME on Android | refused; moot without a CDM (see `eme`) | granted |
| `perm-background` | BACKGROUND_SYNC, PERIODIC_BACKGROUND_SYNC, BACKGROUND_FETCH | see section B | refused | granted |
| `perm-xr` | VR, AR, HAND_TRACKING | WebXR sessions | refused; `isSessionSupported` answers | prompt |
| `perm-other` | PAYMENT_HANDLER, DISPLAY_CAPTURE, CAPTURED_SURFACE_CONTROL, SMART_CARD, WEB_PRINTING, SPEAKER_SELECTION, CAMERA_PAN_TILT_ZOOM, AUTOMATIC_FULLSCREEN, WEB_APP_INSTALLATION, KEYBOARD_LOCK, POINTER_LOCK | various | refused | various |

Not decided by the permission controller: camera and microphone (the `WebContentsDelegate`, see
`camera-mic`), page full screen (`fullscreen`), keyboard lock and pointer lock (their delegate
methods; the permission types are only for the "always allow" path).

## D. Components above content

Features a user sees that live in `//components` or `//chrome`, or in a build setting Chrome sets.

| id | Feature | Mac | Android | Why | Needs | Size | Evidence |
|---|---|---|---|---|---|---|---|
| `codecs-h264` | H.264, AAC and MP4 playback (`<video src=x.mp4>`, `canPlayType`) | absent: `canPlayType` gives "" (from code) | absent (from code) | `proprietary_codecs` defaults to true only for Chrome-branded builds; our `args.gn` doesn't set it | Build setting `proprietary_codecs = true` (and `ffmpeg_branding = "Chrome"`), after a licensing decision | S to build, needs discussion | `C:build/config/features.gni:31-32`; `C:media/base/mime_util_internal.cc:329-365`; `E:tools/build.sh:156-178`; `E:tools/android-content-build.sh:65-81` |
| `eme` | Encrypted Media (Netflix, Spotify, Prime Video) | absent: no Widevine (from code) | absent: no key systems from our renderer (from code) | Widevine is off in non-Chrome Mac builds, and key systems come from `ContentRendererClient::GetSupportedKeySystems`, which we don't override (no renderer client outside extensions) | Widevine needs Google's licence and a CDM component; then a renderer client | L, needs discussion | `C:third_party/widevine/cdm/widevine.gni:14-20`; `C:content/public/renderer/content_renderer_client.h:294-295` |
| `pdf` | Viewing a PDF in a tab | unknown, most likely downloads (which fail) or shows nothing | same | The PDF viewer is a chrome extension plus `//pdf` | `//components/pdf` and the viewer, or hand PDFs to the OS | L | – |
| `spellcheck` | Red squiggles in text fields | absent | absent | `SpellCheckHost` and its renderer side are chrome's | `//components/spellcheck` renderer and browser parts; on Mac the platform checker | M | `C:chrome/browser/chrome_browser_interface_binders.cc:608-612` |
| `autofill` | Address and card autofill, saved passwords | absent (the macOS AutoFill menu items are the system's) | absent | `AutofillDriver` and the password manager are chrome's | Go password store first; autofill later | L | `C:chrome/browser/chrome_content_browser_client_receiver_bindings.cc:419-421` |
| `translate` | Translate this page | absent | absent | chrome's translate driver | Needs discussion | L | `C:chrome/browser/chrome_browser_interface_binders.cc:488` |
| `safe-browsing` | Malware and phishing warnings | absent (basics 28) | absent | chrome's Safe Browsing service | Needs discussion; a URL policy hook exists for top-level loads | L | basics 28 |

## E. What 7025aec turned off, and how well

7025aec (`E:engine/common/main_delegate.cc`, `kChromeOnlyBlinkFeatures`) passes
`--disable-blink-features=InstalledApp,WebShare,DigitalGoods,PaymentRequest,SubApps`.

| Feature | Before | After | Verdict |
|---|---|---|---|
| InstalledApp | Android **killed**; Mac answered empty through content's own binder | absent on both | Needed on Android. On Mac it removed a working API; harmless |
| DigitalGoods | Android **killed**; Mac already off | absent | Needed |
| WebShare | already absent (status "test"; chrome's renderer turns it on) | absent | Redundant |
| SubApps | already absent (isolated apps only) | absent | Redundant |
| PaymentRequest | present, rejects (empty binder) | **still present** (probe-fix), rejects | Doesn't work: the WebPayments web preference turns it back on per page. Harmless, since content has an empty binder |

Missing from the list, and default-on: OnDeviceWebSpeechAvailable and
InstallOnDeviceSpeechRecognition (both platforms), PaymentLinkDetection (Android), and the AI
APIs on Mac. `navigator.setAppBadge` on Mac can't be fixed this way: it has no runtime feature,
so it needs a binder.

## F. Where the ground truth and this reading disagree

- **The probe's "ok" means "alive", not "works".** Most "ok" rows are rejections or
  "denied". Read the `load` and `tap` values, not the first column.
- **The probe never calls `setAppBadge` on Mac** (it runs on Android, where Blink skips the
  call), and never calls `SpeechRecognition.available({processLocally: true})`. Both kill on Mac;
  the second kills on Android too by the code.
- **`fileInput` is DEAD in both Android probes with no kill in the log** and no report at all,
  not even "alive". That looks like the probe's own page, not a renderer kill. Unknown until the
  probe reports it the way it reports the others.
- **`paymentRequest` stays present after 7025aec.** The commit's message says it was turned off.
- **Basics 22's message is stale.** It says `HandleContextMenu` isn't carried. It has been carried
  and subscribed since 019e4b1, which is in 0.2.59 (`E:core/content/wire.go:108-118`). Go returns
  "not handled" for a plain right-click with no link or image, so the check, which right-clicks on
  text, still fails, but for that reason.
- **`docs/daily-use.md` (branch `daily-use`)** agrees with every basics row used here. It has no
  row for the renderer kills, the clipboard, the codecs or EME; the README here gives them.

## G. Corrections to docs/content-hooks.md

- **"Carried" doesn't mean "handled".** Of the 359 carried hooks, Go subscribes to fifteen
  (`E:core/content/wire.go:71-165`). `EnterFullscreenModeForTab`, `RequestPointerLock`,
  `RegisterProtocolHandler`, `CheckMediaAccessPermission`, `EnterPictureInPicture` and
  `CanEnterFullscreenModeForTab` are carried and unused, so content's default runs. The table
  needs a "subscribed" column.
- **`RegisterBrowserInterfaceBindersForFrame` is listed as "left: Mojo"** and the prose says
  "leave them". It is the one hook a renderer kill is fixed in. Our plain `ContentBrowserClient`
  doesn't override it; only `ExtensionsContentBrowserClient` does, for extension frames.
  `RegisterBrowserInterfaceBindersForServiceWorker` isn't overridden by either.
- **The BrowserContext delegates aren't in the doc.** Downloads, permissions, notifications,
  push, SSL decisions, client hints, background sync and fetch are all `nullptr` in
  `E:engine/browser/browser_context.cc`, and they decide more web features than most
  `ContentBrowserClient` hooks. The doc mentions only `DownloadManagerDelegate`.
- **`RunFileChooser` doesn't receive a `unique_ptr`.** It receives a
  `scoped_refptr<FileSelectListener>`. "Ownership" is still the right group; the reason is the
  listener, not a `unique_ptr`.
- **`BindAIManager` is listed as left for want of a copier.** Its default isn't empty: it binds
  content's test echo, so leaving it is not neutral.

## H. What couldn't be determined

- Android behaviour of every delegate row marked "not run": the Android probe checks only
  frame APIs, and there is no Android basics run.
- `fileInput` on Android (see F).
- WebAuthn on Mac: the Mac probe's test page used 127.0.0.1, which isn't a valid RP ID.
- Whether a tab can load `view-source:` (it decides `view-source-wrap`).
- What a PDF link does today.
- `speech-cloud`, `background-fetch`, `pip-document`, `bluetooth` and `client-cert` were not run.

## The Mac probe

Run for this research on 2026-11-14 against `~/PW_TOOLS/project-b-suites-0.2.59/Project B.app`,
one fresh profile per API, typing the URL into the address bar through the driver
(`tools/drive_first_app.py`). The script is `scripts/mac_probe.py` here (run: `python3 scripts/mac_probe.py <project_b binary> <repo>/tools`); it
belongs in the probe once the registry exists ([tracking.md](tracking.md)).

```
badge           resolved           Terminating render process for bad Mojo message: ... No binder found for interface blink.mojom.BadgeService for the frame/document scope
badgeClear      resolved           (same)
speechLocal     (no report)        ... No binder found for interface media.mojom.OnDeviceSpeechRecognition for the frame/document scope
languageModel   rej:NotAllowedError: Requires a user gesture when availability is "downloading" or "downloadable".
translator      no answer in 4s
share           undefined
installedApps   undefined
paymentRequest  function
clipboardWrite  rej:NotAllowedError
geolocation     err:1
eyeDropper      function
hid             devices:0
queryLocalFonts fonts:0
webauthn        rej:SecurityError: This is an invalid domain.
```

## Counts

76 features: 16 in A, 44 in B, 7 in D, and 9 in C that have no row of their own elsewhere
(`perm-geolocation`, `perm-midi`, `perm-persist`, `perm-wake-lock`, `perm-idle`, `perm-nfc`,
`perm-window-mgmt`, `perm-storage-access`, `perm-xr`). The other C rows repeat a B row and
aren't counted twice; `perm-other` is a group and isn't counted.

By what a page sees, taking the worse platform where they differ:

| What a page sees | Count | Which |
|---|---:|---|
| KILLS | 5 | `badge`, `badge-sw`, `speech-on-device`, `payment-link`, `view-source-wrap` |
| quiet | 48 | 4 in A (one, `ai-prompt`, answers wrongly), 35 in B, the 9 C rows |
| absent | 11 | 5 in A, 6 in D |
| works or partly | 4 | `keyboard-lock`, `media-labels`, `context-menu`, `speech-synthesis` |
| unknown | 7 | `web-install`, `fs-access`, `client-cert`, `pip-document`, `speech-cloud`, `background-fetch`, `pdf` |
| nothing to do | 1 | `nostate-prefetch` |
