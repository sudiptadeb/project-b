# To do

What is decided but not built, and what is waiting for a discussion. Last updated 2026-11-12.
Take an item off when it lands or is dropped.

## For discussion with the owner first

- **URL policy on every request.** Each request is allowed or blocked by policy. The options so
  far: a proxy in the browser process, or rules in content's network layer, before each
  request is sent. Not discussed yet; nothing to build until it is.
- **Menus and shortcuts, what is left.** Built for macOS ([menus-shortcuts.md](menus-shortcuts.md)).
  Not discussed yet: the six points from the guidelines at its end. Decided, not built:
  catching keys at the window (the late keys after clicking a suggestion), the menu editor,
  the in-app menu for Windows and Linux. Ctrl+Cmd+F works through the driver; the VM test
  should confirm it with AppKit's own dispatch.
- **The certificate error page.** Mock and two ways to wire it (in the tab, or painted by the
  shell) in `project-b-research/morning-2026-11-10/cert-error-mock-notes.md`.
- **Certificate revocation.** Recommendation: Go fetches Google's CRLSet, with a copy in the
  bundle. `revocation.md` in the same folder.
- **Browser contexts for automation** (what Playwright needs). `browser-contexts.md` in the same
  folder.
- **A new tab page and a crash page.** A crashed page shows the frame colour today.
- **Content's own `chrome://` pages** (such as `chrome://gpu`) pass the gate but don't load,
  because the resource pack lacks their files.
- **Back after the first page in a new tab** returns to `about:blank`.
- **What the first timelines show** ([logging.md](logging.md), baselines in `tools/baseline/`, medians of 10
  starts of the development build, 2026-11-12). Nothing is changed yet; each is for a decision.
  - *Settings is laid over the window before it has drawn anything.* From the click: over the window at 3 ms
    on the Mac and 7 ms on the Android emulator; its first paint at 71 ms and 117 ms; its first frame on screen
    at 132 ms and 163 ms. On Android the window's first frame with the empty overlay is on screen at 45 ms, so
    the phone shows a blank page for about 120 ms: the flash. The Mac covers the window for as long.
  - *A first frame takes long to reach the screen after it is painted:* 64 ms on the Mac for the shell at
    start-up and 62 ms for settings, about four frames; 118 ms and 45 ms on the emulator.
  - *Mac start-up is 296 ms to the shell on screen.* The larger steps: 27 ms before `main`, 41 ms to content
    ready, 39 ms to the window, 53 ms from the window shown to the first tab's commit, and the 64 ms above.
  - *Input on this Mac is not late:* a real click reaches the window 1 to 2 ms after the OS made it and the
    shell 1 ms later. One real Cmd+T took 22 ms from the window to Go, against 0.5 ms through the driver;
    one sample, worth a look. The macOS VM's lag (clicks landing 1 to 3 s late) hasn't been read this way
    yet: `tools/timeline.py` on the VM's `timeline.log` will say which side it is on.
  - *Android start-up is 989 ms on the emulator:* 276 ms from Go started to the shell's commit, and 202 ms
    from the shell's first state to its first paint.
  - *Go sends to the shell before its script listens:* each start logs "__shellReceive is not defined" a few
    times on stderr. The snapshot that follows `ready` makes up for it.
- **Slowness, fixed at the cause, never masked:** a stale picture after a resize, and a brief
  flash at launch. The rejected masking attempts are on branch `parked-window`.
- **Android:** the colour before Go starts. See [android.md](android.md#the-shell-on-a-phone).
- **Extensions: the checks left out, and what is open.** Releases carry extensions since 0.2.25,
  with their flag off by default; turned on, they install from the Chrome Web Store. Every
  check skipped on purpose (hashes, publisher proof, blocklist, policy, content verification,
  host scrubbing) and every open behaviour is listed in
  [extensions.md](extensions.md#checks-to-discuss), to decide before the flag is on by default,
  along with the URL policy above covering what extensions request.
- **Checking store packages against Google's signature.** Today a Web Store package is held to
  the plain CRX3 check only, not to Google's publisher proof. No checks for now, by the owner's
  call; to discuss later.
- **The window's drag region.** Whether it can be simplified. The current fix is in 0.2.23.

## Decided, not started

- Move focus decisions into Go (fixes Cmd+W pressed twice quickly).
- A single-instance lock.
- An engine stress test against other engines, once the owner calls the engine solid.
- A comparison of JS frameworks for the shell, and the final language decision.

## Small fixes

- A page's own full screen (`requestFullscreen()`, such as a video's full-screen button) isn't
  supported: no tab hook takes the window to full screen, so the page's promise never settles.
  Supporting it is the fix; how it behaves is for the owner.
- The first real click after leaving full screen doesn't land; keys work.
- `tools/vm/check.py` times clicks by pixels, so it measures the VM's display; time inside the
  app instead.
