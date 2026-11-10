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

**2. The repo.** The build scripts expect this exact path for now:

    git clone https://github.com/sudiptadeb/project-b ~/Official/workspace/ulaa/project-b
    cd ~/Official/workspace/ulaa/project-b
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

## Test it

The test tools live outside the repo for now:

    mkdir -p ~/.local/playwright && (cd ~/.local/playwright && npm install playwright && npx playwright install chromium)
    mkdir -p ~/PW_TOOLS/chromedriver && cd ~/PW_TOOLS/chromedriver
    curl -LO https://storage.googleapis.com/chrome-for-testing-public/155.0.8059.41/mac-arm64/chromedriver-mac-arm64.zip
    unzip chromedriver-mac-arm64.zip && cd -

Then, from the repo:

    (cd core && go test ./...)                     # Go managers
    node shell/e2e/first-app.cjs                   # the shell on the mock engine
    node shell/e2e/settings.cjs                    # the settings panel on the mock
    python3 tools/drive_first_app.py               # the real app; PB_BIN picks another binary
    python3 tools/drive_first_app.py settings      # the settings panel in the real app
    python3 tools/drive_chromedriver.py            # Google's ChromeDriver drives the real app

Each prints `ok` or `FAIL` per check and ends with `all passed` or a count of failures.

`tools/check_first_paint.py` captures the real window. It needs a person logged in with
the screen unlocked and a screen-capture tool, and skips itself otherwise. Real clicks,
drags and full screen are covered by a macOS VM: [docs/vm-testing.md](docs/vm-testing.md).

Packaging a signed DMG and publishing a release need the signing key, which stays on the
build Mac: [docs/updates.md](docs/updates.md).
