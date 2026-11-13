# Status

Where the project stands, for the next person or agent. Last updated 2026-11-10.

## Docs

- [design-system.md](design-system.md): how our UI looks: colours, type, sizes and the shared controls. Read it before drawing anything.
- [first-app.md](first-app.md): pieces, the window, measurements, packaging.
- [logging.md](logging.md): **follow it in every feature.** The one timeline for logs and timing: how to log in Go, C++ and JS, names, levels, what never goes in, and how start-up and each interaction are measured.
- [window-manager.md](window-manager.md): one Go owner for how the window behaves, built for macOS, and how Windows and Linux map to it.
- [profiles.md](profiles.md) and [profiles-build.md](profiles-build.md): profiles, each in a copy of the app of its own, the list beside their folders, templates, the Profiles menu, the picker, the Profile section in settings, and updates shared by every copy.
- [windows.md](windows.md): several windows, each with its own shell and settings, and the list of tabs each one shows.
- [workspaces.md](workspaces.md): workspaces, sets of tabs inside a profile with cookies and site storage of their own.
- [architecture-review.md](architecture-review.md): an independent verdict from building tabs.
- [content-hooks.md](content-hooks.md): every content hook, carried or left, and what to do about the rest.
- [updates.md](updates.md): how an installed copy updates itself, and how to publish a release.
- [devtools.md](devtools.md): the DevTools endpoint, as Chrome opens it, for ChromeDriver and CDP.
- [menus-shortcuts.md](menus-shortcuts.md): the design for actions, menus and key bindings, not built yet; Chrome's full list is in [chrome-shortcuts.md](chrome-shortcuts.md).
- [remote-build.md](remote-build.md): compiles on a build Mac's Buildbarn, which `tools/build.sh` uses whenever it answers, and why its config is as it is.
- [vm-testing.md](vm-testing.md): a macOS VM with real clicks and keys, and what it found.
- [android.md](android.md): the same shell and managers on Android's content layer, built in a Linux VM on this Mac.
- [extensions.md](extensions.md): Chrome extensions, in every build and run only with their flag on: the layers, the mock store, the build flag, and the checks left out on purpose.
- [flags.md](flags.md): every flag in one file, `core/flags/registry.json`: the six types, build flags and `WITHOUT`, and how a flag reaches the engine.

## Daily commands

    tools/build-and-wait.sh                    # incremental build of out/pb
    tools/test.sh mock                         # Go's tests, and the shell on the mock engine in Playwright
    tools/test.sh app                          # the real app, by its driver and by ChromeDriver; no display
    tools/test.sh basics                       # one check per checklist area; most fail today, a map of gaps
    tools/test.sh screen                       # full screen, the clipboard, first paint; NEEDS A PERSON logged in at the screen
    tools/test.sh vm                           # real clicks in the macOS VM; needs a desktop login once per boot
    tools/test.sh android                      # the Android app on the emulator (amarit start project-b-emu)
    python3 tools/measure.py --compare         # start-up and responsiveness from the app's timeline, against the baseline

`PB_BIN` picks another Mac binary and `PB_APK` another APK. The README's "Test it" says what
each group needs.

## Extensions

    tools/fetch-extensions.sh && tools/pack-test-extension.py    # the mock store's folder, once (drive_extensions packs ours again each run)
    tools/test.sh mock app                                       # the extension checks are in both groups

Every build has them; they run only with the Extensions flag on (Settings, Flags), which is
off by default. [extensions.md](extensions.md) and [flags.md](flags.md) have the rest.

## Shipping a build to try

    amarit restart projectb-release            # RELEASE=1 OUT=out/rel tools/build.sh
    OUT=out/rel tools/package.sh               # signed build/project-b.dmg
    hdiutil attach -nobrowse -readonly -mountpoint <dir> build/project-b.dmg
    PB_BIN="<dir>/Project B.app/Contents/MacOS/project_b" tools/test.sh app
    ~/my-claude-setup/dropd/drop build/project-b.dmg     # prints the link

A Mac installs `project-b-trust.mobileconfig` once, from `tools/signing-setup.sh`. The key
stays on the build Mac. `tools/gen_hooks.py` reruns only on a Chromium upgrade.

## Publishing a release

    tools/release.sh                           # VERSION, committed, becomes a GitHub release

Installed copies pick it up within a minute and offer a relaunch; see [updates.md](updates.md).

## Limits of the tests

The build Mac's agent sessions have no display, so the `app` group's `drive_first_app.py`
calls views directly and its pictures are composites without the native window buttons.
Real full screen, copy and paste, and the real window's first paint are the `screen`
group, which starts the app inside the desktop session of a person logged in at the Mac
and refuses to run without one. Dragging, zoom and tearing need a person's hand, or the `vm` group's macOS
VM, which needs someone logged in to the Mac's desktop once after each boot. An app
started from a background job such as amarit can't enter full screen or reach the
clipboard at all, which is why those checks aren't in the `app` group.

## Open decisions and to do

See [todo.md](todo.md).
