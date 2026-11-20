# Linux

The spec for Linux, the first platform after the Mac and Android ([platforms.md](platforms.md)).
It builds on the three layers ([layers.md](layers.md)): Linux is a new wrapper, a new
`core/system` and a new window. Code starts once the layers work has landed and this spec is
agreed.

## Decided

- **Linux first, aiming at Ubuntu 24.04** (2026-11-19: "yes and yes").
- **Distro-agnostic: Ubuntu, Debian, Arch and KDE must work** (2026-11-19: "arc and debian
  and kde should work").
- **Every Linux build, arm64 and amd64, stays in container `pb` on this Mac** (2026-11-20:
  "no lets keep using the vm here"). Not bigboy: it has 4 cores, 16 GB and a 64 GB disk, and
  runs the shared services.
- **Only arm64 for now; amd64 builds and tests come later** (2026-11-20: "we can run amd64
  tests later... for now lets only focus on arm64 builds"). Nothing amd64 happens before
  step 7 of the order of work.
- **Flatpak is the only Linux package** (2026-11-19: "flatpak itis"). No tarball, no `.deb`.
- **The app id is `io.github.sudiptadeb.project-b`** (2026-11-19: "yea lets go with this").
  Flathub reads it as github.com/sudiptadeb/project-b, which makes its GitHub check possible;
  a dash is allowed in the last part. It never changes, or users stop getting updates.
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
- **Our output.** `out/linux-arm64` beside the Android tree in the container's `/work`,
  about 15 GB. `out/linux-x64` comes with amd64, in step 7.
- **Go.** `libcore` as a static archive with `GOOS=linux`, cgo through Chromium's clang and
  sysroot, as Android does. `engine/build_go.py` hard-codes darwin today; the layers work's
  step 6 (the three-piece build) makes it take the platform.
- **Priority.** A build reniced on the host, run in a slot from the tester as a one-time
  `amarit job` (never `amarit run`, which an amarit update restarts). A build on
  12 cores at full priority failed 0.2.109's Android run on 2026-11-19.
- **Rosetta.** Clang under Rosetta crashes now and then; `build-content-shell.sh` retries
  those steps.
- **amd64 in step 7, in `pb` too.** The same scripts, into `out/linux-x64`.

## 2. The window

The engine's one big new piece, and its first exception ([layers.md](layers.md#the-exceptions-where-the-engine-still-touches-the-os)):
`window_linux.cc` beside `window_mac.mm`, behind the same calls from Go.

- **The tree.** One Aura window tree per top-level window, on Ozone. The shell's view on top,
  the pages' views below, and Go's regions (`WindowLayout.At`) decide who gets each press, as
  on the Mac. Content shell's Aura path (`content/shell/browser/shell_platform_delegate_aura.cc`)
  is the smallest example. Not Views: we don't use them on the Mac either.
- **Wayland and X11.** Go picks in `PBInit` and puts `--ozone-platform=wayland` or `=x11` on
  content's command line: Wayland when `WAYLAND_DISPLAY` is set (or `XDG_SESSION_TYPE` is
  `wayland`), X11 otherwise. That's Wayland on Ubuntu 24.04, Debian 13 and KDE Plasma 6, and
  X11 when that's the session. Content ignores `--ozone-platform-hint=auto`: the spike's
  content_shell took X11 under Wayland and quit, since the hint is read only in Chrome's own
  code. After the pick, one code path; Ozone hides the difference.
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
  every child process, the zygote included, so Go never reaches a renderer. On Linux that takes
  one line of ours: the zygote always starts from `base::FILE_EXE`, never from
  `--browser-subprocess-path`, and the renderers, the sandboxed utilities and the GPU process
  are all forked from a zygote. So `ProjectBMain`, in the browser process before
  `ContentMain`, points `FILE_EXE` at the helper beside it, with
  `PathService::OverrideAndCreateIfNeeded(base::FILE_EXE, helper, false, false)` (plain
  `Override` treats the path as a folder and refuses a file). The spike proved it: every
  child runs and maps only the helper. Nothing else of ours or content's on Linux reads
  `FILE_EXE` for anything but the folder, which is the same; Go's relaunch uses
  `os.Executable`.
- **Inside Flatpak, Chromium's sandbox can't make its own namespaces**, because bubblewrap
  already did. zypak fixes that: it makes Chromium believe its setuid sandbox is there, and
  starts each sandboxed child in a new Flatpak sandbox through the Flatpak portal. Chrome,
  Chromium, Brave, Edge and most Electron apps on Flathub use it, and it ships in the
  Electron and Chromium base apps. We start as `zypak-wrapper project-b`.
- **This removes Ubuntu's AppArmor problem.** Ubuntu 23.10 and later block unprivileged user
  namespaces; a Flatpak never needs them, so no AppArmor profile and no setuid helper.
- **zypak needs nothing for the separate helper**: no `ZYPAK_SANDBOX_FILENAME`, no other
  setting. Its spawn strategy starts each sandboxed child in a nested Flatpak sandbox (its own
  PID namespace) whatever binary the child is. The renderers carry Chromium's seccomp-bpf
  policy on top of Flatpak's filter. See the spike.
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
| Single instance (`core/instance`) | a Unix socket in `$XDG_RUNTIME_DIR/app/io.github.sudiptadeb.project-b/`, which every copy of our Flatpak shares |
| Links and files from the OS | the `.desktop` file declares `http`, `https` and HTML files, and the desktop launches us with them on the command line. A copy already running gets them through `core/instance`. So on Linux this needs no OS events and isn't an exception |
| Relaunch (`core/relaunch`) | the Flatpak portal's `Spawn`; after an update, amarit's, with the latest-version flag (see 5) |
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
  through the Flatpak's `cups` socket. The spike shows the socket works from inside the
  sandbox (CUPS answers on `/run/cups/cups.sock`); content_shell has no print dialog, so the
  dialog itself is still to decide. Until then Linux doesn't print.
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
- **On GitHub, no server of ours** (2026-11-19: "lets use github as the source of updates").
  The Mac already updates from GitHub Releases ([updates.md](../updates.md)).
  - **First: our repository on GitHub Pages.** The repository is a folder of static files
    (OSTree), signed with our own GPG key, sealed in the kosh. Release builds and signs it
    here and attaches it to the GitHub release; a small workflow publishes it to Pages.
    Pages through a workflow, not a branch, because git refuses files over 100 MB and the
    main binary is one.
  - Pages caps a site at 1 GB and asks for under 100 GB of traffic a month. The repository
    keeps the last three releases, and updates download only what changed, so that covers
    the early users.
  - **Later: Flathub, with our files on GitHub Releases.** Flathub's entry holds only the
    manifest; at install, Flatpak downloads our files from the GitHub release, as Chrome and
    Brave do on Flathub. Flathub carries the traffic, Releases has no cap, and the Pages
    repository can go.
  - Users install with one click from a `.flatpakref` file, in GNOME Software or KDE Discover,
    or with `flatpak install --from <url>`.
  - Ubuntu doesn't ship Flatpak, so there is one `sudo apt install flatpak` first; the install
    page says so. Debian, Arch and KDE's own distros have it or one command away.
- **Updates.**
  - **amarit owns them** (agreed with amarit, 2026-11-20): appupdate gets a Flatpak source,
    the module `amarit/flatpak`, with `godbus` in a module of its own, driving Flatpak's
    update portal. The plan is in amarit's `docs/design.md`, "Flatpak source for appupdate".
    The shell sees the same states and commands as on the Mac:
    - `UpdateAvailable` is ready to download; `Update` with its progress is downloading.
    - An update GNOME Software or `flatpak update` made while we run goes straight to ready.
    - An update that asks for new permissions fails with kind `needs-user`: it goes through
      the software centre.
    - The relaunch after an update is the Flatpak portal's `Spawn` with the latest-version
      flag, inside `amarit/flatpak`; the new copy waits for our single-instance socket.
    - A notification through the Notification portal offers Relaunch.
  - **The split.** amarit owns updates and the relaunch after an update; `core/relaunch`'s
    `After` and the kqueue wait (`wait_darwin.go`) leave once project-b switches to it.
    project-b keeps `core/instance`, and `core/relaunch`'s `Copy`, `Self` and `Copies`.
  - amarit builds it at step 5 of 7, when our repository can raise `UpdateAvailable`, and
    tests it with us in the VM slot.
  - `core/update/bundle.go` (the Mac's folder swap) isn't used on Linux, and our own signature
    check isn't needed, since Flatpak checks the repository's signature.
  - The runtime updates by itself, with every other Flatpak.
- **Fonts: the host's.** Flatpak shares the system's fonts with every app at
  `/run/host/fonts`, and the spike's pages drew Chinese from the host's Noto CJK once the
  host had it. We ship no fonts. A desktop install of Ubuntu brings Noto CJK; where a distro
  leaves it out (Arch installs no fonts at all), CJK shows as boxes in every app there, and
  the fix is the distro's package (`fonts-noto-cjk`, Arch's `noto-fonts-cjk`).
- **Size.** About 150 to 200 MB for us, plus the shared runtime, which most Linux desktops
  already have.

## 6. Testing

| Where | What | Runs |
|---|---|---|
| An arm64 container | the arm64 build, headless, at native speed | `tools/test.sh app` with our driver, Ozone headless |
| `deskrig-ub`: Ubuntu 24.04, GNOME, Wayland | the Flatpak, a real desktop | install, first run, driver, look |
| `deskrig-ubx`: the same on Xorg | the same, on X11 | the same |
| `deskrig-deb`: Debian 13, GNOME | the same | the same |
| `deskrig-kde`: Arch Linux ARM, KDE Plasma 6.8, Wayland | the same, and KDE's decorations | the same |
| Step 7: container `pb`, amd64 | the x64 build, headless | as the arm64 container |
| Step 7: the same Tart VMs | the amd64 Flatpak, its programs through Rosetta | install, first run, driver, look |

- Arch itself is x64 only; on arm64 the nearest is Arch Linux ARM, a separate project, and
  it stands in for Arch.
- amd64 is tested only from step 7: the x64 Flatpak in the arm64 VMs, its programs through
  Rosetta, or headless in `pb`.
- X11: `deskrig-ubx` is Ubuntu's Xorg session; runs there cover X11.
- ChromeDriver: Chrome for Testing has none for Linux arm64, so we build it from the same
  tree, or use only our own driver.
- **deskrig** has Linux targets since v0.2.0: the four `deskrig-*` VMs above, in the tester's
  Tart home. A test script changes only `--target`. What differs is in deskrig's
  `docs/linux.md` ("As built"): no fling on X11, `cmd` means Super, `type` knows only the US
  keyboard, and there's no record on Linux yet, only frames.
- The tester hands out the container, the slots and the VMs. The Ubuntu VM is `pb-ubuntu`
  in the tester's Tart home: 6 CPUs, 8 GB, 60 GB disk, made by `tools/linux/vm-setup-guest.sh`
  and started and stopped with `tools/linux/vm.sh`. It's the Linux session's slot while
  Linux is being built; stop it when not in use.
- No GPU in the VMs: Apple's virtualisation gives a Linux guest a 2D virtio-gpu only (no
  virgl), so Chromium blocklists it and draws in software. A real GPU test needs real
  hardware, such as a laptop.

## 7. Order of work

After the layers work lands, each step leaves the Mac and Android green:

1. **The spike** (done 2026-11-20, see Spike): stock content_shell for arm64, as a Flatpak
   through zypak, in the Ubuntu VM. Shows the cross-build, zypak with a separate helper,
   Wayland and the sandbox.
2. **Our engine and `libcore` for Linux arm64**, headless in the arm64 container: the app
   starts, our driver runs. It needs the window's tree, so that comes here (see the plan below).
3. **The window** on a real desktop: the top row, move and resize, the menu strip.
4. **`core/system` on Linux**, and the import permissions.
5. **The Flatpak and our repository**, updates through the portal.
6. **The four deskrig VMs** (they and deskrig's Linux target are ready), then the release run
   gains Linux.
7. **amd64**, the first amd64 work: built in `pb`, tested headless there and through
   Rosetta in the VMs. Steps 1 to 6 are arm64 only.

About 5 weeks for one session, like the earlier estimate: the window is still the big piece,
and Flatpak takes the place of the `.deb` and AppArmor work.

### Step 2, the plan

Our engine and `libcore` for Linux arm64, built in `pb`, started headless, our driver running.
Nothing here changes the Mac or Android; each piece is its own commit.

**The build.**
- `engine/BUILD.gn` serves the Mac and Linux, the desktops: the `assert(is_mac)` goes, and
  what is the Mac's moves under `if (is_mac)`: the `.mm` files, the frameworks, `sqlite3`,
  `//sandbox/mac:seatbelt`, and the `-exported_symbol`/`install_name` and `@executable_path`
  link flags. Linux's are `$ORIGIN` for the rpath; `ProjectBMain` is already exported by its
  visibility attribute. Android keeps its own `engine/android/BUILD.gn`.
- The shell's pages: Linux builds no shell in `pb` (no Node there), as Android doesn't.
  `tools/build.sh linux arm64` builds `shell/dist` on the Mac first, and `build_go.py` copies
  all of it, the desktop's shells, into `core/shellfiles/dist`.
- `build_go.py` gains `linux`: `GOOS=linux`, `GOARCH=arm64`, `CGO_ENABLED=1`, a c-archive as on
  the Mac, `CC` Chromium's clang with `--target=aarch64-linux-gnu` and `--sysroot` the arm64
  sysroot GN already uses (GN passes both, as Android's passes clang and the NDK). Chromium's
  version goes in with `-X main.chromiumVersion`, as on the Mac. The Go is `pb`'s own
  `/usr/local/go`, x64, cross-compiling.
- `tools/build.sh linux arm64` does what `android arm64` does: the Mac's lock, the tree
  copied into `pb`'s `/work/tree` under `/work/.android-build.lock` (one build at a time, as the
  tree and `//project_b` links are shared), then `gn gen out/linux-arm64` with
  `root_extra_deps = [ "//project_b/engine" ]` and `autoninja`. Run as an `amarit job`,
  reniced, in a slot from the tester. The spike's `out/linux-arm64` is reused: content is
  already built there.

**The engine's Mac-only C++, and what Linux gets.**

| Mac | Linux in step 2 |
|---|---|
| `window_mac.mm` | `window_linux.cc`: the Aura tree, the shell's view over the pages' views, Go's regions for each press. No top row, move or resize yet (step 3) |
| `browser_main_parts.mm` | `browser_main_parts_linux.cc`: Aura's screen on Ozone, then the shared startup |
| `driver.mm` | its commands and screenshots move to `driver.cc`, shared; `driver_mac.mm` keeps the NSEvents; `driver_linux.cc` sends `ui::Event`s into the Aura tree the same way |
| `mac_app.mm` | nothing: no menu bar (the strip is step 3), no Dock |
| `printing_mac.mm` | a stub that says printing isn't there yet (open item 1) |
| `ProjectBMain`'s Seatbelt | nothing: the zygote's sandbox is content's own |

- `ProjectBMain`, the main binary's branch, on Linux: points `base::FILE_EXE` at
  `project_b_helper` beside it with `OverrideAndCreateIfNeeded(..., false, false)` before
  `ContentMain`, as the spike proved, so no child maps the binary with Go in it.
- Headless needs no window of the OS's: Ozone's headless platform gives Aura a surface, and
  the driver's screenshots are what content rendered, as on the Mac.

**The Go core.** Built for Linux with cgo off, everything but `core/cmd/lib` compiles today
except two gaps (`instance.Quit`, `relaunch`'s wait). Linux gets:
- `core/cmd/lib`: `switches_linux.go` adds the Ozone pick: `--ozone-platform=wayland` when
  `WAYLAND_DISPLAY` is set, `x11` when `DISPLAY` is, else `headless`; with the Mac's
  `--disable-in-process-stack-traces` (Go owns the crash signals there too) and the saved
  flags. `keys_linux.go`: no keychain, so a profile keeps its own key in its folder (the
  Secret portal is step 4). `choose`, `exit`, `print`, `download_files` as the Mac's;
  `update_linux.go`: no updates yet (amarit's Flatpak source comes at step 5).
- `core/instance`: the Mac's flock and Unix socket work on Linux as they are, so
  `instance_darwin.go` becomes `instance_unix.go`; the socket goes in
  `$XDG_RUNTIME_DIR/app/io.github.sudiptadeb.project-b/` inside the Flatpak (3), the user's
  temporary folder outside it.
- `core/relaunch`: a `wait_linux.go` beside the kqueue one, waiting with `pidfd_open` and
  `poll`. After and the kqueue wait stay as they are.
- `core/system/platform`: `platform_linux.go`, every call answering as "not on this system",
  so the managers run; the portals are step 4. Import finds no browsers yet (`browserdata`'s
  `_other.go`), also step 4.

**Where it runs.** An arm64 Ubuntu 24.04 container in `pb`'s colima VM, native speed, no
Rosetta: `pb`'s `/work` mounted read-only, Chromium's runtime libraries and fonts installed,
`--no-sandbox` (a container gives no user namespaces; the Flatpak's sandbox is the VMs').
`tools/test.sh app` there runs `drive_first_app.py`'s parts that aren't the Mac's own
(driver, browsing, settings, timeline, cookies, visible, windows, empty, move, shells,
internal, links). ChromeDriver for Linux arm64 doesn't exist as a download, so
`drive_chromedriver.py` waits for one built from the tree, after the milestone.

**Done when** `tools/build.sh linux arm64` builds the app, it starts headless in the arm64
container, shows the shell and a page, and those driver parts pass, with the Mac's app suite
and Android's still green.

## Spike (2026-11-20)

Step 1 of 7: stock content_shell for Linux arm64, none of our code, as a Flatpak through
zypak, in `pb-ubuntu` (Ubuntu 24.04 arm64, GNOME on Wayland). The scripts are in
`tools/linux/`; the pictures are in [linux-spike/](linux-spike/).

### What worked

- **The arm64 cross-build** in container `pb`, with the arm64 sysroot (Debian bullseye) and the
  `out/linux-x64` args plus `target_cpu="arm64"` (`build-content-shell.sh arm64`).
- **The Flatpak**: runtime `org.freedesktop.Platform` 26.08, base `org.electronjs.Electron2.BaseApp`
  26.08 with its zypak, started as `zypak-wrapper content_shell` from `/app/bin/project-b`
  (`io.github.sudiptadeb.project-b.yml`, `stage.sh`, `project-b.sh`). The section 5
  permissions are all in, and the dock shows our icon from the `.desktop` file.
- **Wayland**, with `--ozone-platform=wayland` (see 2: content needs the pick made for it).
- **A page loads** ([wayland-page](linux-spike/wayland-page.png)).
- **The sandbox.** Every child is in Chromium's sandbox as zypak arranges it:

  | Process | Runs | Seccomp filters | PID namespace |
  |---|---|---|---|
  | browser | main binary | 1 (Flatpak's) | the app's |
  | zygote (unsandboxed), GPU process, network service | helper | 1 | the app's |
  | zygote (sandboxed), renderers, storage service | helper | 2 (Flatpak's and Chromium's) | its own, nested |

  `Seccomp: 2` and `NoNewPrivs: 1` everywhere. The renderers sit in a PID namespace of their
  own, started through `bwrap` by zypak's spawn strategy. The network service is unsandboxed
  by Chromium's design on Linux (`--service-sandbox-type=none`), as in Chrome.
- **The separate helper**, with one line of ours (see 3). Without it, `--browser-subprocess-path`
  moves only the directly launched children (the network service) to the helper; the zygote,
  and so the renderers, the sandboxed utilities and the GPU process, run the main binary. With
  `FILE_EXE` pointed at the helper (`spike-helper-exe.patch`, a spike-only change to
  content_shell), every child's `/proc/<pid>/exe` is the helper and no child maps the main
  binary. zypak needs nothing for it.
- **Move, resize, maximise**: a drag on the top row moved the window, a drag on its corner
  resized it, a double-click maximised it, all done by the compositor
  ([moved](linux-spike/moved.png), [resized](linux-spike/resized.png),
  [maximised](linux-spike/maximised.png)).
- **CUPS from the sandbox**: `--socket=cups` works; CUPS answers on `/run/cups/cups.sock`.
- **The host's fonts**: with Noto CJK on the host, the sandbox draws Chinese
  ([host-cjk-font](linux-spike/host-cjk-font.png)).

### What didn't

- **The GPU, in the VM.** Apple's virtualisation gives Linux a 2D virtio-gpu with no virgl.
  Chromium blocklists it (WebGL 1 and 2 blocklisted). With `--ignore-gpu-blocklist` the GPU
  process loses its context and restarts in a loop. With `--enable-unsafe-swiftshader`, WebGL 2
  runs on ANGLE, Vulkan and SwiftShader inside the sandbox
  ([webgl-swiftshader](linux-spike/webgl-swiftshader.png)). A real GPU needs real hardware.
- **The GPU process has no Chromium seccomp under zypak**: it logs "InitializeSandbox() called
  with multiple threads" and keeps only Flatpak's filter. In the VM it ran on software, so
  this is to check on real hardware (Open, 2).
- **Printing can't be tested with content_shell**: `window.print()` returns at once with no
  dialog ([print](linux-spike/print.png)). Only the CUPS socket is shown.
- **`--ozone-platform-hint=auto`** is ignored by content: X11 was picked under Wayland, and
  with no X server it quit.
- **Rosetta**: Chromium's x64 clang died from a signal twice (exit -1, a few seconds into a
  step) in 77,544 steps, about once every 39,000. `build-content-shell.sh` builds on with
  `-k 0` and retries when every failure is such a crash.

### Numbers

| | |
|---|---|
| arm64 build, from nothing | 77,544 steps, 6 h 26 min over three runs (2 h 20, 2 h 16, 1 h 50), reniced to 15 beside release's Android runs; x64 took 3 h 34 |
| `out/linux-arm64` | 14 GB |
| A one-file change and relink | about 1 min |
| `flatpak-builder` | 11.6 s, copying only |
| The Flatpak | 754 MB of files, 411 MB installed: content_shell (370 MB, test code linked in) and its copy as the helper are one object in OSTree |
| The Ubuntu VM | 12 GB on the host's disk after setup |

### What changes in the spec

Done above: Go picks Wayland or X11 in `PBInit` (2); `ProjectBMain` points `FILE_EXE` at the
helper, and zypak needs nothing (3); the CUPS socket works (4); we rely on the host's fonts
(5); the VM slot and no GPU in VMs (6); every Linux build stays in `pb`, reniced in the
tester's slots, with Rosetta's clang crashes retried by `build-content-shell.sh` (1).

### Setting up the Ubuntu VM

`tools/linux/vm-setup-guest.sh` on `ghcr.io/cirruslabs/ubuntu:24.04`. Two things it fixes: our
network lets only 443 out with no IPv6, so apt uses https over IPv4; and
`ubuntu-desktop-minimal` without recommends leaves a netplan file that gives the network to a
NetworkManager that isn't installed, so the VM boots with no network until it's masked.
Pictures come from Tart's VNC server (`vm.sh vnc capture`), so no window opens on the Mac;
`tart run --vnc` opens Screen Sharing by itself, and `vm.sh start` closes only that one.

## Open

1. **Printing**: GTK only for print, or our own print settings (4). The CUPS socket works
   from the sandbox; the choice needs our engine, since content_shell has no dialog.
2. **The GPU process's sandbox under zypak**: no Chromium seccomp in the spike's VM, on
   software GL. Check on real hardware whether Chrome's Flatpak is the same.
3. **Dev and Stable on Linux**, for Sudipta. The proposal: two branches of the app,
   `//stable` and `//dev`, in the same repository; switching opens the other branch's
   `.flatpakref`.
4. **Release notes**, for Sudipta: whether the shell reads them from `releases.json`, as on
   the Mac, with no `min_version` on Linux.

## Sources

- [zypak](https://github.com/refi64/zypak): how it works, its spawn strategy, its settings.
- [Flatpak portal update monitor (ashpd)](https://docs.rs/crate/ashpd/latest/source/src/flatpak/update_monitor.rs),
  [An update on Flatpak updates](https://blogs.gnome.org/mclasen/2018/11/26/an-update-on-flatpak-updates/).
- Chromium 155: `content/shell/browser/shell_platform_delegate_aura.cc`,
  `build/linux/sysroot_scripts/install-sysroot.py`, `tools/clang/scripts/update.py`.
