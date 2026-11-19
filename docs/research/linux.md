# Linux

The spec for Linux, the first platform after the Mac and Android ([platforms.md](platforms.md)).
It builds on the three layers ([layers.md](layers.md)): Linux is a new wrapper, a new
`core/system` and a new window. Code starts once the layers work has landed and this spec is
agreed.

## Decided

- **Linux first, aiming at Ubuntu 24.04** (2026-11-19: "yes and yes").
- **Distro-agnostic: Ubuntu, Debian, Arch and KDE must work** (2026-11-19: "arc and debian
  and kde should work").
- **arm64 built and tested here first, amd64 later on bigboy** (2026-11-19).
- **Flatpak is the only Linux package** (2026-11-19: "flatpak itis"). No tarball, no `.deb`.
- **Pure Go wherever possible**: the system layer talks D-Bus through `godbus/dbus`.
- **The shell draws the window buttons at the right; the menu strip is off by default**
  (2026-11-19, on ux's mocks: "looks good").

## What runs where

| | Today on the Mac | On Linux |
|---|---|---|
| Wrapper | `main.cc`, a Mac app bundle | the same `main.cc`, an ELF binary |
| Window | `window_mac.mm`, an `NSWindow` | an Aura window tree on Ozone (Wayland, or X11) |
| Window buttons | macOS's own | the shell's, top right |
| OS calls | `core/system` darwin (after layers) | `core/system` linux: portals over D-Bus |
| Child processes | the helper app | the same helper binary, started through zypak |
| Package | `.app` in a DMG | a Flatpak, from our own repository |
| Updates | Go swaps the bundle | Flatpak updates it; Go asks through the portal |

## 1. The build

- **Host.** Chromium's clang exists only for Linux on x64, so every Linux build happens in
  the amd64 container `pb` under Rosetta, as Android's does. arm64 is Chromium's supported
  cross-build from there, with the arm64 sysroot it downloads
  (`build/linux/sysroot_scripts/install-sysroot.py --arch=arm64`).
- **Proved on 2026-11-19.** Stock content_shell for Linux x64 built in `pb` with no new sync,
  no new deps and the sysroot that was already there: 37,961 steps in 3 hours 34 minutes.
  It ran headless and loaded a page.
- **Our output.** `out/linux-arm64`, then `out/linux-x64`, beside the Android tree in the
  container's `/work`. About 15 GB each.
- **Go.** `libcore` as a static archive with `GOOS=linux`, cgo through Chromium's clang and
  sysroot, as Android does. `engine/build_go.py` hard-codes darwin today; the layers work's
  step 6 (the three-piece build) makes it take the platform.
- **Priority.** A build reniced on the host and run in a slot from the tester. A build on
  12 cores at full priority failed 0.2.109's Android run on 2026-11-19.
- **amd64 later, on bigboy.** A native Linux x64 host builds with no Rosetta: the same
  scripts and `out/linux-x64`.

## 2. The window

The engine's one big new piece, and its first exception ([layers.md](layers.md#the-exceptions-where-the-engine-still-touches-the-os)):
`window_linux.cc` beside `window_mac.mm`, behind the same calls from Go.

- **The tree.** One Aura window tree per top-level window, on Ozone. The shell's view on top,
  the pages' views below, and Go's regions (`WindowLayout.At`) decide who gets each press, as
  on the Mac. Content shell's Aura path (`content/shell/browser/shell_platform_delegate_aura.cc`)
  is the smallest example. Not Views: we don't use them on the Mac either.
- **Wayland and X11.** Ozone picks at start (`--ozone-platform-hint=auto`): Wayland on Ubuntu
  24.04, Debian 13 and KDE Plasma 6; X11 when that's the session. One code path; Ozone hides
  the difference.
- **No title bar of the OS's.** We ask for client-side decorations: on KDE through
  `xdg-decoration`; GNOME never draws them. The shell draws the top row with minimise,
  maximise and close at the right.
- **Move and resize.** A drag on the shell's empty top row asks the compositor to move the
  window (Ozone's `StartWindowMove`, `xdg_toplevel.move` on Wayland). An invisible border of
  a few pixels around the window starts a resize (`xdg_toplevel.resize`). Double-click on the
  top row maximises. The compositor does the rest, so snapping and tiling work as for every
  other app.
- **What Wayland won't do.** An app can't place its own window or read where it is. A restored
  window gets its size back but not its place; on X11 it gets both.
- **The menu strip.** Off by default, with a "Show menu bar" setting. Alt shows it for a
  moment, and F10 opens the first menu. The shell draws it from Go's menus, so nothing new in
  the engine.
- **Input.** Keys, the input method (Wayland's `text-input-v3`, IBus or Fcitx on X11), the
  cursor, drag and drop and the clipboard are all Chromium's, through Ozone.
- **Size.** 1,500 to 2,000 lines, written once and reused by Windows, which also uses Aura.

## 3. Processes and the sandbox

- **The same split as on the Mac.** The main program holds Go. A helper binary with no Go runs
  every child process. Linux's zygote starts from the helper, so Go never reaches a renderer.
- **Inside Flatpak, Chromium's sandbox can't make its own namespaces**, because bubblewrap
  already did. zypak fixes that: it makes Chromium believe its setuid sandbox is there, and
  starts each sandboxed child in a new Flatpak sandbox through the Flatpak portal. Chrome,
  Chromium, Brave, Edge and most Electron apps on Flathub use it, and it ships in the
  Electron and Chromium base apps. We start as `zypak-wrapper project-b`.
- **This removes Ubuntu's AppArmor problem.** Ubuntu 23.10 and later block unprivileged user
  namespaces; a Flatpak never needs them, so no AppArmor profile and no setuid helper.
- **Proven first.** zypak expects one binary that re-runs itself for children; ours has a
  separate helper. Its `ZYPAK_SANDBOX_FILENAME` and Chromium's `--browser-subprocess-path`
  should cover that. The first spike shows it working, with `chrome://sandbox` all green.
- **Headless tests outside a Flatpak**, in a container, run with `--no-sandbox`. That's for
  tests only; what users run is always the Flatpak.

## 4. `core/system` on Linux

Every call is a desktop portal over D-Bus, from pure Go (`github.com/godbus/dbus/v5`).
Portals are the same on GNOME, KDE and the rest, so one implementation covers all four
targets. The same calls work outside a Flatpak, which keeps headless tests simple.

| Call | On Linux |
|---|---|
| `OpenFile` | OpenURI portal, `OpenFile` |
| `RevealFile` | OpenURI portal, `OpenDirectory` (the file manager, the file selected) |
| `IsDefaultBrowser` | reads `x-scheme-handler/https` from `~/.config/mimeapps.list`, with read-only access to that one file |
| `AskDefaultBrowser` | no portal can set it: the shell explains where (GNOME: Settings > Apps > Default Apps; KDE: System Settings > Default Applications) |
| Notifications | Notification portal: title, body, icon and buttons; a click comes back as `ActionInvoked` |
| `NotificationsAllowed`, `AskNotifications` | the portal has no ask; the user turns them off in the desktop's settings. Always allowed from our side |
| `SetDockColor`, `OpenFullDiskAccess` | Mac only; nothing on Linux |
| Dark mode, accent colour | Settings portal, `org.freedesktop.appearance` (`color-scheme`, `accent-color`), and its change signal |
| Single instance (`core/instance`) | a Unix socket in `$XDG_RUNTIME_DIR/app/<app id>/`, which every copy of our Flatpak shares |
| Links and files from the OS | the `.desktop` file declares `http`, `https` and HTML files, and the desktop launches us with them on the command line. A copy already running gets them through `core/instance`. So on Linux this needs no OS events and isn't an exception |
| Relaunch (`core/relaunch`) | the Flatpak portal's `Spawn`, with the latest-version flag after an update |
| Updates | see 5 |

Through Chromium, not `core/system`:

- **Save as and the folder chooser.** `ui::SelectFileDialog` uses the FileChooser portal, so
  each desktop shows its own chooser. A Flatpak gets access to what the user picks.
- **Import.** Other browsers' profiles are hidden folders. The Flatpak gets read-only access to
  the known ones (`~/.mozilla`, `~/.config/google-chrome`, `~/.config/chromium`,
  `~/.config/BraveSoftware`, and Flatpak Firefox's `~/.var/app/org.mozilla.firefox`), so
  import finds them with no chooser. The chooser stays as the fallback.
- **Print.** Chromium's Linux print dialog is GTK's, and we don't load GTK. Either we load it
  for printing only, or the shell draws the print settings and Chromium sends the job to CUPS
  through the Flatpak's `cups` socket. A spike decides; until then Linux doesn't print.
- **Share.** Linux has no share sheet, so the shell offers Copy link instead.

Tests: the fake `system.System` from the layers work. For the D-Bus side, a session bus in the
container with a stub portal.

## 5. The Flatpak

- **How it's made.** A manifest names the runtime (`org.freedesktop.Platform`, the current
  release), the base app that carries zypak, our files and the permissions. `flatpak-builder`
  doesn't build Chromium: it copies in our finished `out/linux-<cpu>` files, so a Flatpak
  takes minutes.
- **Permissions** (`finish-args`): Wayland, with X11 as the fallback; the GPU (`dri`);
  PulseAudio, which also covers PipeWire; the network; IPC; CUPS; read-only `mimeapps.list`
  and the browser profiles in 4; `xdg-download` for downloads. Nothing else: the portals
  cover the rest.
- **Our own repository first, Flathub later.**
  - The repository is a folder of static files (OSTree), signed with our own GPG key, sealed
    in the kosh. Any static HTTPS host serves it.
  - Users install with one click from a `.flatpakref` file, in GNOME Software or KDE Discover,
    or with `flatpak install --from <url>`.
  - Ubuntu doesn't ship Flatpak, so there is one `sudo apt install flatpak` first; the install
    page says so. Debian, Arch and KDE's own distros have it or one command away.
- **Updates.**
  - The app's own: Flatpak's update portal. Go makes an update monitor and hears
    `UpdateAvailable`. The shell shows "Update ready", Go calls `Update` and shows its
    progress, and a relaunch starts the new version through `Spawn`.
  - `core/update/bundle.go` (the Mac's folder swap) isn't used on Linux, and our own signature
    check isn't needed, since Flatpak checks the repository's signature.
  - The runtime updates by itself, with every other Flatpak.
- **Size.** About 150 to 200 MB for us, plus the shared runtime, which most Linux desktops
  already have.

## 6. Testing

| Where | What | Runs |
|---|---|---|
| Container `pb`, amd64 | the x64 build, headless | `tools/test.sh app` with our driver, Ozone headless |
| An arm64 container | the arm64 build, headless, at native speed | the same |
| Tart VM: Ubuntu 24.04, GNOME, Wayland | the Flatpak, a real desktop | install, first run, driver, look |
| Tart VM: Debian 13, GNOME | the same | the same |
| Tart VM: Arch Linux ARM, KDE Plasma 6 | the same, and KDE's decorations | the same |
| bigboy, later | amd64 builds, and an x64 Arch for real Arch | the same |

- Arch itself is x64 only; on arm64 the nearest is Arch Linux ARM, a separate project. Real
  Arch is tested when amd64 comes, on bigboy.
- X11: Ubuntu's login screen still offers an Xorg session; one run there covers X11.
- ChromeDriver: Chrome for Testing has none for Linux arm64, so we build it from the same
  tree, or use only our own driver.
- **deskrig** needs a Linux target: a small agent in the VM for screenshots, clicks and keys.
  That's deskrig's own repo, for the tester.
- The tester hands out the container, the slots and the VMs.

## 7. Order of work

After the layers work lands, each step leaves the Mac and Android green:

1. **The spike** (2 to 3 days): stock content_shell for arm64, as a Flatpak through zypak, in
   the Ubuntu VM. Shows the cross-build, zypak with a separate helper, Wayland and the sandbox.
2. **Our engine and `libcore` for Linux arm64**, headless in the arm64 container: the app
   starts, our driver runs.
3. **The window**: the tree, the top row, move and resize, the menu strip.
4. **`core/system` on Linux**, and the import permissions.
5. **The Flatpak and our repository**, updates through the portal.
6. **The three VMs**, the deskrig Linux target, then the release run gains Linux.
7. **amd64 on bigboy.**

About 5 weeks for one session, like the earlier estimate: the window is still the big piece,
and Flatpak takes the place of the `.deb` and AppArmor work.

## Open

1. **The app id.** Flatpak names an app by reverse DNS, and changing it later loses users'
   updates. A domain we own, or `io.github.sudiptadeb.<Name>`.
2. **Where the repository lives.** Static files, a few hundred MB per release with deltas.
   GitHub Pages is capped at 1 GB, so our own host is likelier.
3. **Printing**: GTK only for print, or our own print settings (the spike in 4).

## Sources

- [zypak](https://github.com/refi64/zypak): how it works, its spawn strategy, its settings.
- [Flatpak portal update monitor (ashpd)](https://docs.rs/crate/ashpd/latest/source/src/flatpak/update_monitor.rs),
  [An update on Flatpak updates](https://blogs.gnome.org/mclasen/2018/11/26/an-update-on-flatpak-updates/).
- Chromium 155: `content/shell/browser/shell_platform_delegate_aura.cc`,
  `build/linux/sysroot_scripts/install-sysroot.py`, `tools/clang/scripts/update.py`.
