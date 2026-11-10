# To do

What is decided but not built, and what is waiting for a discussion. Last updated 2026-11-10.
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
- **Slowness, fixed at the cause, never masked:** a stale picture after a resize, and a brief
  flash at launch. The rejected masking attempts are on branch `parked-window`.
- **Android:** the colour before Go starts. See [android.md](android.md#the-shell-on-a-phone).

## Decided, not started

- Move focus decisions into Go (fixes Cmd+W pressed twice quickly).
- A single-instance lock.
- An engine stress test against other engines, once the owner calls the engine solid.
- A comparison of JS frameworks for the shell, and the final language decision.

## Small fixes

- `--version` prints the version and exits. Do it with the next feature.
- A page's own full screen (`requestFullscreen()`, such as a video's full-screen button) isn't
  supported: no tab hook takes the window to full screen, so the page's promise never settles.
  Supporting it is the fix; how it behaves is for the owner.
- The first real click after leaving full screen doesn't land; keys work.
- `tools/drive_first_app.py`: the check for typing `chrome://shell/` is flaky.
- `tools/vm/check.py` times clicks by pixels, so it measures the VM's display; time inside the
  app instead.
- The ChromeDriver suite looks for a window named `project_b`; the packaged app's is "Project B".
