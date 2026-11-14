# Web features above content: the short version

Project B runs Chromium's content layer without `//chrome`. Every web API whose browser side
Chrome provides is missing here, half-working, or dangerous. Dangerous means a page that calls
it gets its renderer killed, and every tab sharing that renderer shows as crashed. That is what
happened to DuckDuckGo's search on Android, fixed by 7025aec.

This folder:

- [inventory.md](inventory.md): 76 features, one row each, with what a page sees on Mac and
  Android, why, what it would take, and file:line evidence.
- [tracking.md](tracking.md): a proposal to keep this current, with one registry file the tests
  read and a check run on every Chromium bump.

Read on Chromium 155.0.8059.41 and Project B 0.2.61, 2026-11-14. Five renderer kills remain. Two
of them were confirmed on the 0.2.59 Mac app for this research; the rest come from the code.

## The top ten, worst first

1. **`navigator.setAppBadge` kills the renderer on Mac.** Chat and mail web apps call it to show
   an unread count. It has no runtime feature, so it can't be turned off; it needs an empty
   binder for `blink.mojom.BadgeService`. Confirmed on 0.2.59. Size S.
2. **The same call from a service worker kills that worker's process on Mac.** Push handlers do
   this. Same fix, in the service-worker binder hook. Size S.
3. **`SpeechRecognition.available({processLocally: true})` kills the renderer, on both
   platforms.** Confirmed on Mac. Turn off OnDeviceWebSpeechAvailable and
   InstallOnDeviceSpeechRecognition, next to 7025aec's list. Size S.
4. **`<link rel="facilitated-payment">` kills the renderer on Android.** Payment pages use it.
   Turn off PaymentLinkDetection. Size S.
5. **Copy buttons don't work.** `navigator.clipboard.writeText` is refused, even after a tap,
   because every permission is denied. Chrome allows it. Size S, once there is a permission
   delegate.
6. **H.264 and MP4 video doesn't play, and there's no Widevine.** The build doesn't set
   `proprietary_codecs`, and DRM needs a licence. Many sites' video, and all streaming services.
   Needs discussion.
7. **No file upload, no downloads, no dialogs.** `<input type=file>` cancels, downloads never
   arrive, `alert` returns at once, a tab closes without "leave this page?". Already rows 1 to 3
   in `docs/daily-use.md`. Sizes M, L, M.
8. **Every permission is denied with no prompt.** Notifications, location, camera and microphone
   (NotSupportedError), full screen stays pending. One permission delegate and a Go manager fix
   the shape; camera and microphone also need the media request hook. Size M, then L for media.
9. **Prompt API pages get a test stub on Mac.** `LanguageModel` is present and says it can be
   downloaded; behind it is content's echo, written for web tests. Turn the AI APIs off. Size S.
10. **Links to `mailto:`, `tel:` and other apps do nothing**, and a public site can't reach a
    `localhost` or LAN device (Local Network Access is denied). The first hurts most on a phone.
    Size M each.

Smaller, still default-on and quiet: `deviceorientation` never fires on Android, `PaymentRequest`
is still present despite 7025aec (harmless), and the "Line wrap" box on a `view-source:` page
would kill the renderer on Mac, if a tab can open one.

## How to read the inventory

- Each row has an **id** (`badge`, `file-input`…). The ids are the registry keys proposed in
  tracking.md.
- **What a page sees**: works, quiet (rejects, empty, "denied", or no answer), absent (not there,
  or throws), **KILLS**, or unknown. Mac and Android are given apart.
- **Needs**: a Go manager, an engine hook (the exact `ContentBrowserClient`, `WebContentsDelegate`
  or `BrowserContext` method), a native panel or prompt, or **stay off** (add the Blink feature to
  `kChromeOnlyBlinkFeatures` in `engine/common/main_delegate.cc`).
- **Size**: S under a day, M a few days, L weeks or a service we don't have.
- **Evidence**: `C:` paths are in `chromium/src`, `E:` paths in this repo. "Probe" is the
  Android 17 probe; "basics N" is the Mac basics run on 0.2.59; "Mac probe" is the run made for
  this research.
- Section E says what 7025aec did and didn't do. Section F lists where the probe, the basics and
  `daily-use.md` disagree with the code. Section G corrects `docs/content-hooks.md`.

A probe's "ok" means the renderer survived, not that the API worked. Read its values.
