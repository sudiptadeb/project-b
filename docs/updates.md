# Updates

An installed copy keeps itself current. It looks at the project's release manifest every
minute, downloads a newer release by itself, checks it, and shows a small bubble in the
page's bottom-left corner, the way a browser shows a link's address: "Downloading Project B
0.2.1…", then "Project B 0.2.1 is ready. Relaunch to update". Clicking relaunches into the
new version. Nothing shows when nothing is happening.

## Pieces

| Where | What |
|---|---|
| `core/update` | The manager: the check loop, the download, the checks, the swap, the relaunch. `CheckInterval` is the one minute, in one place. |
| [amarit](https://github.com/sudiptadeb/amarit) | The manifest (`releases.json`), its rules (channels, no downgrade, `min_version`, a rejected version is not offered again) and the download, checked against the manifest's size and sha256 as it streams. Used through `amarit.Client`, the half of amarit for a program that installs itself. |
| `shell/src/UpdateStatus.vue` | The bubble. It is inside the page's box, so it sits in the page's corner in both tab layouts and takes no room; `v-shell` makes it shell, so a click on it does not reach the page. |
| `amarit.json`, `VERSION` | The project name, the manifest URL, the asset pattern; the version being built. |
| `tools/release.sh` | Publishes a release. |

The app is a signed folder (two binaries, a library, resources), not the one binary amarit
swaps in place, so the swap is ours. `amarit.Client` was added to amarit for this; it is on
amarit's `bundle-client` branch until it is merged and released, and `core/go.mod` points at
that commit.

## What happens

1. Two seconds after start, then every `CheckInterval`, unless automatic updates are off in
   Settings, the manager fetches
   `https://github.com/sudiptadeb/project-b/releases/latest/download/releases.json`. That is
   a plain web redirect to the latest release's asset, not GitHub's REST API, so a check a
   minute is not rate limited (the response carries no rate-limit headers).
2. If the install location isn't writable (a copy still on its DMG, or in a folder the user
   doesn't own), the bubble says so and nothing is downloaded.
3. A newer `stable` version for `darwin/arm64` is downloaded into the profile
   (`~/.project-b/default/updates/download`), size and sha256 checked by amarit.
4. The zip is unpacked with `ditto` into `updates/stage`, and the bundle inside must say it
   is that version (`CFBundleShortVersionString`) and pass `codesign --verify --deep
   --strict` and `codesign --verify -R='certificate leaf = H"<sha1>"'` with the SHA-1 of our
   signing certificate, compiled into the main binary. These are the two checks
   `tools/package.sh` makes before a build ships. A bundle that fails them is refused for
   good on that install: `amarit.Client.Reject` records the version, so a bad release is
   downloaded once, not every minute. A download that fails (network, or bytes that don't
   match the manifest) is shown, and tried again next minute. A failure stays on show while
   the channel still points at that release; once it points elsewhere, the bubble goes.
5. The bubble offers relaunch. Until it is clicked, the installed app is untouched.
6. Relaunch renames the installed bundle to `updates/previous/Project B.app`, renames the
   staged one into its place, leaves a `/bin/sh` behind that waits for this process to exit
   and then starts the new one, and closes the window. Unix lets the running processes keep
   using the moved bundle, so the helpers never notice.

A failed automatic check of the manifest itself (offline) is logged, not shown: nothing was
happening for the user to see. A build running from a build directory, or with no version in its
bundle, never checks. The staged and downloaded leftovers of an earlier run are cleared at
start; a release that was ready but not relaunched into is downloaded again.

## Settings

`chrome://settings/` shows the running version, a Check for updates button and the
automatic updates switch.

- **Check for updates** runs one check at once, whatever the switch says (`CheckNow`). The
  page shows what the manager says: checking, up to date, downloading a version, ready to
  relaunch, or the error it hit, including a manifest it couldn't read. A release it finds
  goes the way an automatic one does, and the bubble stays the one place to relaunch.
  Automatic checks never set the page's check result, so the bubble is unchanged.
- **Automatic updates** is a pref, `autoUpdate` in `prefs.json`, on by default. Off, the
  manager's timer stops at once; on again, it starts as it does at launch.

## Publishing

    tools/release.sh

Bump `VERSION` and commit first. The script makes the release build (`RELEASE=1
OUT=out/rel`), packages and signs it, writes `Project-B-<v>.dmg` (for people) and
`Project-B-<v>-darwin-arm64.zip` (for the updater) into `build/dist`, downloads the last
release's `releases.json` so earlier releases stay listed, has `amarit release` add this
one, tags `v<v>` and creates the GitHub release with all of it. The first updater-enabled
version has to be installed by hand from its DMG, once; from then on the app updates itself.

## Checked

- `core/update` tests: a newer release is staged and relaunch swaps the bundle and keeps the
  previous one; a corrupted download is refused and tried again; a bundle of another
  version, or one that fails the signature check, is refused once; an unwritable location
  is reported without downloading; the same version shows nothing.
- `core/update` tests also cover a check asked for (up to date, a manifest that can't be
  read, a newer release) and the timer stopping and starting with the switch.
- `shell/e2e/settings.cjs` checks the settings page's version, Check and switch against a
  local release server, including that the switch survives a restart.
- `shell/e2e/first-app.cjs` runs the real manager against a local release server with a
  slow asset, in both layouts, through to the relaunch helper starting the new bundle.
- The signature check runs only in the real app (`core/cmd/lib`); the mock has no signing
  key and checks the version alone.

## Not done

- No automatic rollback: the previous release is kept, nothing starts it. A refused download
  never reaches the install, which is the case that matters.
- Only `stable`, only Apple silicon, no delta downloads, no progress percentage.
- A manifest signature (amarit's enterprise tier) isn't implemented in amarit; the bundle's
  code signature against the pinned key is the trust here.
