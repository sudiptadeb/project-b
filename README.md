# Project B

An experiment in how a browser could be built with a better architecture: Go managers, a
Vue shell and Chromium's content layer (155.0.8059.41, unpatched) behind one engine interface. Apple
silicon Macs, and arm64 Android phones built on the same Mac.

Releases with a DMG and self-update: https://github.com/sudiptadeb/project-b/releases.
Where the project stands: [docs/status.md](docs/status.md). How it is built:
[docs/first-app.md](docs/first-app.md). Android: [docs/android.md](docs/android.md). How our UI
looks, for anyone drawing any of it: [docs/design-system.md](docs/design-system.md). How every
feature logs and times itself: [docs/logging.md](docs/logging.md).

## Get it working

Or hand [requirements.prompt](requirements.prompt) to a coding agent on the machine and let
it do these steps.

**1. The machine.** An Apple silicon Mac with the full Xcode app (not only the command line
tools), about 150 GB free, and:

    sudo xcode-select -s /Applications/Xcode.app
    brew install go node python

**2. The repo,** anywhere you like:

    git clone https://github.com/sudiptadeb/project-b
    cd project-b
    (cd shell && npm install)

**3. Chromium**, about 60 GB, no patches:

    tools/fetch-chromium.sh

**4. Build.** The first build is a full Chromium build: about an hour on a 32-core Mac,
several hours on a laptop. Later builds take under a minute.

    tools/build.sh

The app is `chromium/src/out/pb/project_b`. Run it directly.

**No Chromium yet?** The shell runs on a mock engine in a minute:

    (cd shell && npm run build)
    (cd core && go run ./cmd/dev -shell ../shell/dist)    # then open http://127.0.0.1:8765

The browser tab is the OS window (`core/cmd/dev/window.html`): it holds what the real window
holds in views, a mock page for each tab the layout shows, where it puts it, the shell over them in a
transparent frame, the frame band and window controls, the settings page over everything
when the gear opens it, and an extension's popup, a card naming the page. Each press goes
where the real window sends it, by the layout's regions. The shell and the settings page
are the real app's, unchanged: the server gives them `chrome.send` as the engine does
(`core/cmd/dev/webui.js`). The shell also opens alone at http://127.0.0.1:8765/shell/standard/, and
the settings page at http://127.0.0.1:8765/settings/. With `npm run dev` in `shell/`, open
the window at `/mock/window` on Vite's server. Each window Go makes is a page of its own:
window 1 at `/`, and the one New Window makes at `/?window=2`, in a tab you open.

## Test it

Test tools go in `.tools/` inside the repo, which git ignores. `npm install` in step 2
already brought Playwright; it needs its browser once:

    (cd shell && npx playwright install chromium)

ChromeDriver, pinned to the Chromium we build:

    mkdir -p .tools/chromedriver && cd .tools/chromedriver
    curl -LO https://storage.googleapis.com/chrome-for-testing-public/155.0.8059.41/mac-arm64/chromedriver-mac-arm64.zip
    unzip chromedriver-mac-arm64.zip && cd ../..

The tests come in six groups, by what each needs. One command runs a group, and a group
that lacks what it needs says so and stops before running anything:

| Group | What it runs | What it needs |
|---|---|---|
| `tools/test.sh mock` | Go's tests, and every shell, the settings page and the picker in a browser on the mock engine, with the check every shell passes ([docs/shells.md](docs/shells.md)) | Go, `npm install`, Playwright's browser. No Chromium build; runs anywhere |
| `tools/test.sh app` | The real app through its own driver, and through Google's ChromeDriver | The build, and ChromeDriver; with extensions in, Go, the mock store (`tools/fetch-extensions.sh`, then `tools/pack-test-extension.py`) and the network. No display: it runs from ssh or an agent session |
| `tools/test.sh basics` | One check per area of the whole-browser checklist on the real app; most fail today, as a map of what isn't built | As for `app`, and the network for the Safe Browsing check |
| `tools/test.sh screen` | The real app in a person's desktop session: full screen, copy and paste, and the first page's paint in the real window | **A person logged in at this Mac's screen**, with it unlocked, and `deskrig` (below) |
| `tools/test.sh vm` | Real clicks, drags and keys in a macOS VM | **A person to have logged in at this Mac's desktop once since it booted**, then the VM: [docs/vm-testing.md](docs/vm-testing.md) |
| `tools/test.sh android` | The Android app on an emulator | The emulator running, and the APK: [docs/android.md](docs/android.md) |

`PB_BIN` points the Mac groups at another binary, such as an installed app's
`Contents/MacOS/project_b`, and `PB_APK` the Android group at another APK. Each test prints
`ok` or `FAIL` per check, and the group ends with one line per test: `passed`, `FAILED` or
`DID NOT RUN`.
A test also runs alone, as the top of its file says: `python3 tools/drive_first_app.py settings`
runs only the menus, keys and settings page.

Chrome extensions are in every build (`WITHOUT=extensions tools/build.sh` leaves them out) and run
only with their flag on, in Settings, Flags: [docs/extensions.md](docs/extensions.md) and
[docs/flags.md](docs/flags.md).

### The groups that need a person

A session over ssh, an agent's session and a background job have no screen of their own.
An app a background job starts can't enter full screen or reach the clipboard, and its
window may not be on the screen at all.
So the `screen` group starts its tests inside the desktop session of whoever is logged in,
through `tools/on_screen.py`, and refuses to run when nobody is. Run alone, that says
whether someone is there:

    tools/on_screen.py

The app's window shows on that person's screen while the group runs, and takes all of it
for a few seconds; the copy and paste check replaces what they had copied. Logging in
over Screen Sharing counts.

First paint looks at the real window through [deskrig](https://github.com/sudiptadeb/deskrig),
which takes window captures and gives real clicks and keys from any session, on this Mac or
in the VM. Install it and allow it once:

    deskrig setup permissions         # after installing it, as its README says

`deskrig --target local status` says what's missing. `DESKRIG` and `CHROMEDRIVER` point the
tests at other copies; a machine that already has ChromeDriver elsewhere can link
`.tools/chromedriver` to it.

The `vm` group needs the login for another reason: macOS only starts a macOS VM after
someone has logged in at the desktop since the last boot. Nobody has to stay at the screen.

Packaging a signed DMG and publishing a release need the signing key, which stays on the
build Mac: [docs/updates.md](docs/updates.md).
