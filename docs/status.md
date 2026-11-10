# Status

Where the project stands, for the next person or agent. Last updated 2026-11-10.

## Docs

- [first-app.md](first-app.md): pieces, the window, measurements, packaging.
- [window-manager.md](window-manager.md): one Go owner for how the window behaves, built for macOS, and how Windows and Linux map to it.
- [architecture-review.md](architecture-review.md): an independent verdict from building tabs.
- [content-hooks.md](content-hooks.md): every content hook, carried or left, and what to do about the rest.
- [updates.md](updates.md): how an installed copy updates itself, and how to publish a release.
- [devtools.md](devtools.md): the DevTools endpoint, as Chrome opens it, for ChromeDriver and CDP.
- [vm-testing.md](vm-testing.md): a macOS VM with real clicks and keys, and what it found.

## Daily commands

    tools/build-and-wait.sh                    # incremental build of out/pb
    (cd core && go test ./...)
    node shell/e2e/first-app.cjs               # mock engine in Playwright
    node shell/e2e/settings.cjs                # the settings panel alone, on the mock
    python3 tools/drive_first_app.py           # real app; PB_BIN picks another binary
    python3 tools/drive_first_app.py settings  # the settings panel alone, real app
    python3 tools/drive_chromedriver.py        # ChromeDriver drives the real app; PB_BIN as above
    python3 tools/check_first_paint.py         # its first page shows in the real window (desk)

## Shipping a build to try

    amarit restart projectb-release            # RELEASE=1 OUT=out/rel tools/build.sh
    OUT=out/rel tools/package.sh               # signed build/project-b.dmg
    hdiutil attach -nobrowse -readonly -mountpoint <dir> build/project-b.dmg
    PB_BIN="<dir>/Project B.app/Contents/MacOS/project_b" python3 tools/drive_first_app.py
    ~/my-claude-setup/dropd/drop build/project-b.dmg     # prints the link

A Mac installs `project-b-trust.mobileconfig` once, from `tools/signing-setup.sh`. The key
stays on the build Mac. `tools/gen_hooks.py` reruns only on a Chromium upgrade.

## Publishing a release

    tools/release.sh                           # VERSION, committed, becomes a GitHub release

Installed copies pick it up within a minute and offer a relaunch; see [updates.md](updates.md).

## Limits of the tests

The build Mac's agent sessions have no display, so `drive_first_app.py` calls views directly
and its pictures are composites without the native window buttons. Dragging, zoom, real
full screen and tearing need a person, or the macOS VM in `tools/vm/`, which needs someone
logged in to the Mac's desktop once after each boot.

## Branches not on main

- `android`: a WebView APK and `docs/android.md`, with real content being built. Predates tabs
  and hooks, so it needs updating before merging.

## Open decisions and to do

See [todo.md](todo.md).
