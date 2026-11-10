# Project B

A proof of concept for a clean-sheet enterprise browser: Go managers, a Vue shell and
Chromium's content layer (155.0.8059.41, unpatched) behind one engine interface. Apple
silicon Macs only for now.

Releases with a DMG and self-update: https://github.com/sudiptadeb/project-b/releases.
Where the project stands: [docs/status.md](docs/status.md). How it is built:
[docs/first-app.md](docs/first-app.md).

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
                                                          # settings: http://127.0.0.1:8765/settings/

## Test it

Test tools go in `.tools/` inside the repo, which git ignores. `npm install` in step 2
already brought Playwright; it needs its browser once:

    (cd shell && npx playwright install chromium)

ChromeDriver, pinned to the Chromium we build:

    mkdir -p .tools/chromedriver && cd .tools/chromedriver
    curl -LO https://storage.googleapis.com/chrome-for-testing-public/155.0.8059.41/mac-arm64/chromedriver-mac-arm64.zip
    unzip chromedriver-mac-arm64.zip && cd ../..

Then:

    (cd core && go test ./...)                     # Go managers
    node shell/e2e/first-app.cjs                   # the shell on the mock engine
    node shell/e2e/settings.cjs                    # the settings page on the mock
    python3 tools/drive_first_app.py               # the real app; PB_BIN picks another binary
    python3 tools/drive_first_app.py settings      # menus, keys and the settings page in the real app
    python3 tools/drive_chromedriver.py            # Google's ChromeDriver drives the real app

Each prints `ok` or `FAIL` per check and ends with `all passed` or a count of failures.

### Checks on the real screen

`tools/check_first_paint.py` looks at the real window, through `deskctl`, a small tool in
`tools/deskctl/` that takes window captures and gives real clicks and keys, even from an
agent or ssh session with no display:

    tools/deskctl/install.sh          # builds .tools/desk/desk, signed with a local key

Then allow `.tools/desk/deskctl` once in System Settings, Privacy & Security, under both
Screen & System Audio Recording and Accessibility, and keep someone logged in with the
screen unlocked. `.tools/desk/desk status` says what's missing. Without it the check skips
itself. `DESK` and `CHROMEDRIVER` point the tests at other copies; a machine that already
has these tools elsewhere can also link `.tools/desk` and `.tools/chromedriver` to them.

Real clicks, drags and full screen at scale run in a macOS VM:
[docs/vm-testing.md](docs/vm-testing.md).

Packaging a signed DMG and publishing a release need the signing key, which stays on the
build Mac: [docs/updates.md](docs/updates.md).
