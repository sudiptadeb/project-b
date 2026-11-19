# Other platforms

Research for a discussion, 2026-11-19. Nothing here is built or decided. It asks, for each
platform after the Mac and Android phones: what it takes, and whether we can build and test it
on the build Mac (an M3 Ultra, 32 cores, 256 GB RAM).

## The short answer

- **Linux first.** Chromium builds it in the Linux container we already run for Android, for
  x64 and arm64. Its window code, Aura, is the same one Windows uses, so the hardest part is
  written once for both. It tests here in a native arm64 VM, and headless in the container.
- **Android tablets and desktop mode beside it.** No new build at all: the same APK, made to
  resize, take a mouse and a keyboard, and pick the Standard shell on a wide window. The
  emulators already have tablet and desktop images.
- **Windows next.** Chromium cross-compiles Windows from macOS directly, no container. A
  Windows 11 on Arm VM runs both our arm64 build and our x64 build, the second through
  Windows' own x64 emulation.
- **iOS last, and it is a different question.** Outside the EU and Japan, Apple allows only
  WebKit, so a worldwide iPhone app means a WebKit engine behind our interface: a second
  engine. Our own Chromium content is possible only for EU and Japan users, with an
  entitlement Apple grants. Chromium 155 already has that build, and it runs in the iOS
  simulator here.
- **ChromeOS comes free, untested.** Chromebooks run our Android APK, and Linux apps in their
  Linux container. There is no Chromebook image we can run on Apple silicon.
- **Disk was the first blocker, and is cleared.** This Mac had 37 GB free; each new Chromium
  target needs 20 to 40 GB more. A cleanup on 2026-11-19 left 507 GB free.

## What carries over to every platform

From [first-app.md](../first-app.md), [engine-interface.md](../engine-interface.md) and
[android.md](../android.md):

| Piece | Today | On a new platform |
|---|---|---|
| Go core (`core/`) | Pure Go, with small per-OS files: `core/cmd/lib/*_darwin.go` and `*_android.go` (access, choose, dock, notify, open, print, system, update), `core/instance`, `core/relaunch`, `core/update/bundle.go` | The managers as they are. A new set of those per-OS files, about 15, each small |
| The engine interface (`engine/abi/pb_abi.h`) | One C table each way | Unchanged. It is plain C, so it is also the seam between Go's toolchain and Chromium's |
| The adapter (`engine/browser/`) | 110 files; most are content and shared, with `BUILDFLAG` guards for Android | Shared as it is |
| Mac-only adapter | About 2,900 lines: `window_mac.mm` (985), `driver.mm` (995), `mac_app.mm` (531), and small ones for start-up, notifications, printing, downloads, the folder chooser | Rewritten per platform. The window and the test driver are most of it |
| Android-only adapter | `window_android.cc` (461), about 1,200 lines of Java | Reused by Android tablets |
| Shells (`shell/`) | Web pages | Unchanged. Standard on a desktop, Mobile on a phone |
| Updates (`core/update`, amarit) | One manifest, an asset per OS and CPU | New assets (`linux/amd64`, `windows/arm64`, ...). The swap and the check are per OS |
| `engine/BUILD.gn` | `assert(is_mac, ...)`, with a separate `engine/android/BUILD.gn` | Opened up per platform |
| `engine/build_go.py` | `GOOS=darwin`, `GOARCH=arm64` hard-coded | Takes the target. Android's `engine/android/build_go.py` already builds Go with Chromium's clang and a sysroot; Linux works the same way |

The rule from 2026-11-15 holds everywhere: the engine gives us page views, the window is ours.
On Linux and Windows the page view is an Aura window that we place inside our own window, as
an `NSView` is on the Mac.

## Linux (x64 and arm64)

**Chromium.** Linux is Chromium's main platform. Its build host must be Linux on x64: the
clang that Chromium downloads exists for `Linux_x64`, `Mac`, `Mac_arm64` and `Win`, and not
for Linux on arm64 (`tools/clang/scripts/update.py`). So the build goes where Android's does,
in the amd64 container `pb` under Rosetta in Colima ([android.md](../android.md#2-what-host-does-it-need)).
x64 is a native build there; arm64 is Chromium's supported cross-build, with the arm64 sysroot
it downloads.

- **Checkout.** Add `linux` to `target_os` in the container's `.gclient` and sync. The tree
  is already there for Android; the additions are the two sysroots and a few Linux-only
  dependencies, a few GB.
- **Time.** Android's first content build took 2 hours 31 minutes in that container. Expect
  about the same for each Linux CPU, then a few minutes after a change of ours.
- **Disk.** 10 to 15 GB of output per CPU, like `out/pb` on the Mac (10 GB). It lands in the
  Colima VM's disk, which lives on this Mac's disk.

**Engine and adapter.**

- **The window, the big piece.** Content on Linux draws a page into an `aura::Window`. We
  need our own window: an Aura window tree per top-level window, with the shell's view over
  the pages' views, presses routed by Go's regions (`WindowLayout.At`), focus, the cursor,
  drag and drop, and the input method. Content shell's Aura path
  (`content/shell/browser/shell_platform_delegate_aura.cc`) is the smallest example; Chrome's
  is Views, which we don't use. Window frames: we draw our own, as on the Mac, using
  Wayland's and X11's client-side move and resize. Estimate 1,500 to 2,000 lines, written
  once for Linux and Windows.
- **Processes.** As on the Mac: the main program holds Go, a helper with no Go runs every
  child, through content's child process path. Linux's zygote starts from the helper, so Go
  never reaches a renderer.
- **The sandbox.** Chromium's Linux sandbox needs user namespaces. Ubuntu 23.10 and later block
  them for unprivileged apps unless an AppArmor profile allows the app. So the package has to
  install a profile, or a setuid sandbox helper. Running without the sandbox is not an option.
- **Go.** `libcore` as a static archive, built with Chromium's clang and its sysroot, as
  Android does. The per-OS files: the single-instance socket (as on the Mac), `xdg-open`, the
  desktop's notifications over D-Bus, the file chooser through the desktop portal, printing
  through content's own dialog.

**Shell.** The Standard shell as it is. The window buttons go on the right, as on Windows,
and the title bar follows the desktop's dark mode.

**Packaging, signing, updates.**

- A tarball that installs into the user's home, and a `.deb` for Ubuntu and Debian that
  carries the AppArmor profile. Flatpak later, if at all: its sandbox fights Chromium's.
- Linux has no code signing. The update check verifies our own signature over the download,
  with our public key in the binary, as the Mac checks our certificate.
- Updates: the tarball updates itself the way the Mac app does, by swapping its folder. A
  `.deb` installed by root can't, so it updates through an apt repository, or tells the user.
  One of the decisions below.

**Testing here.**

- **Headless, in a container, cheap.** The real app on Linux runs with no display (Chromium's
  headless Ozone, or Xvfb). `tools/test.sh app` with our own driver runs there. ChromeDriver
  for `linux64` comes from Chrome for Testing; for arm64 there is none, so we build it from
  the same tree, or use only our own driver there.
- **x64 headless:** in the amd64 container that builds it.
- **arm64 headless:** in an arm64 container, native speed. Colima already runs those.
- **A real desktop:** a Linux VM in Tart, which we already use for macOS
  (`ghcr.io/cirruslabs/ubuntu`, arm64, native speed, about 5 GB). Tart can also run x64 Linux
  programs inside it through Rosetta. deskrig has no Linux target yet; it needs one, as a
  small agent inside the VM for captures, clicks and keys. That is deskrig's own repo.
- The tester hands out the container and the VM. Neither exists for Linux yet.

**Effort.** About 4 to 5 weeks for one session: the build 2 days, the window 2 weeks, Go's
files 3 to 4 days, packaging 3 days, tests and the deskrig target a week.

## Windows 11 (arm64 and x64)

**Chromium.** Chromium supports cross-compiling Windows from a Mac host
(`docs/win_cross.md`): "It's possible to build most parts of the codebase on a Linux or Mac
host while targeting Windows". Only 32-bit builds fail on a Mac host. So this builds on the
Mac itself, natively, on all 32 cores, from the checkout we already have.

- **The Windows SDK.** The one catch. It has to be packaged once on a Windows machine with
  Visual Studio installed (`depot_tools/win_toolchain/package_from_installed.py`), into a zip
  the Mac build then reads. The Windows VM below can make it.
- **Checkout.** `target_os = ['win']` in the Mac's `.gclient`: the SDK zip, several GB
  unpacked, and Windows-only dependencies.
- **Time.** About an hour per CPU from clean, like the Mac build.
- **Our remote build server** doesn't help: Chromium's remote builds for Windows targets work
  only from a Linux host.
- **Crash reports.** Cross builds stub out `.asm` files, so Crashpad can't report crashes.
  Our own crash logs in Go don't depend on it.

**Engine and adapter.**

- **The window:** the same Aura code as Linux, with Windows' own frame handling (hit testing
  the non-client area so our drawn title bar drags and snaps, and Windows 11's snap layouts).
- **Go:** Go builds C libraries for Windows only with a GNU toolchain, and Chromium builds
  with clang in Microsoft's ABI. Our engine interface is plain C, so Go goes in a DLL of its
  own (`-buildmode=c-shared`, built with llvm-mingw) that only the main program loads, and
  the helper never does. This is the first thing to prove, in a day.
- **The sandbox:** Windows' sandbox starts in the main program's `wWinMain`, so our start-up
  follows content shell's Windows start-up.
- **Per-OS files:** the single-instance pipe, `ShellExecute`, toast notifications,
  the file dialog, default browser registration.

**Shell.** Standard as it is, buttons on the right, Windows 11's corners and its Mica
background as a later touch.

**Packaging, signing, updates.**

- A per-user install in `%LOCALAPPDATA%`, as Chrome does, needing no administrator. A zip
  with a small installer to start; MSIX later if we want the Store.
- **Signing.** Unsigned or self-signed, SmartScreen warns on every download until the file
  gains reputation, as the Mac's Gatekeeper does today. A trusted certificate costs money
  each year (Microsoft's signing service on Azure, or an OV certificate). Signing itself runs
  on the Mac (`osslsigncode` or `jsign`).
- **Updates:** Windows can't replace a running program, so each version installs into its
  own folder beside the last, and a small launcher starts the newest, as Chrome does.

**Testing here.**

- **A Windows 11 on Arm VM.** Microsoft gives out the arm64 ISO for virtual machines; it runs
  unactivated for testing. Tart can't run Windows. UTM can (free, QEMU underneath); it isn't
  on this Mac yet and would go in `~/my-claude-setup/tools/`.
- **Both CPUs in one VM:** Windows on Arm runs x64 programs through its own emulation, so the
  x64 build is tested there too, slower.
- **Driving it:** OpenSSH in the VM for our driver and ChromeDriver (Chrome for Testing has
  `win64`; it runs under emulation). Real clicks need a deskrig agent for Windows, as for
  Linux.
- About 64 GB of the Mac's disk for the VM.

**Effort.** About 3 to 4 weeks after Linux, since the window is shared: the Go DLL and
start-up a week, the Windows window parts a week, installer, updates and signing a week, the
VM and tests a few days, plus the deskrig agent.

## Android tablets, foldables and desktop mode

**Chromium.** Nothing new. The same APK.

**Engine and adapter.** What a phone doesn't need today:

- **Resizing.** The activity has to take a change of size without being recreated, because
  `onDestroy` ends our process ([android.md](../android.md#4-how-does-go-fit-so-renderers-never-start-it)).
  So the manifest declares the size changes it handles, and the window relays them.
- **The mouse and the touchpad.** Today only touches are routed by Go's regions; hover and
  the wheel aren't. Desktop mode needs both.
- **The keyboard.** Our shortcuts (`core/commands`) aren't wired on Android. A hardware
  keyboard needs them.
- **Several windows.** Android keeps one window today; desktop mode and tablets offer
  several. One activity per window, in our one process, maps onto Go's windows. This can
  come after the rest.

**Shell.** The Standard shell on a wide window with a mouse, and Mobile on a narrow one, by
the window's size and its pointer. The Mobile shell's layout at tablet width. That is shell
work for the ux session.

**Packaging, signing, updates.** As today.

**Testing here.** The emulators: tablet and foldable images, and Android 16's desktop
windowing. One AVD more for the tester. The driver (`tools/drive_android.py`) already runs
the app on four emulators; it needs mouse and keyboard steps.

**Effort.** About 1 to 2 weeks for resizing, mouse, keyboard and the shell switch; several
windows another week.

## ChromeOS

**Chromium.** ChromeOS itself is built by Google; we don't target it. A Chromebook runs our
app two ways:

- **The Android APK**, through ChromeOS's Android support. Most Chromebooks are x64, and our
  APK is arm64 only. They translate arm code, slowly. An x64 APK is one more Android build in
  the same container.
- **The Linux build**, in ChromeOS's Linux container (Crostini), as a Linux app.

**Testing here.** None. There is no ChromeOS image for Apple silicon, and ChromeOS Flex is x64
only. Android's desktop mode on the emulator is the nearest we have. A real Chromebook is
the only real test.

**Effort.** Nothing of its own once Linux and Android large screens work. An x64 APK is a
day of build work.

## iOS and iPadOS

**Apple's rule.** Apps in the App Store that browse the web must use WebKit. Two exceptions,
both with the same framework, BrowserEngineKit:

- **The EU**, from iOS 17.4.
- **Japan**, from iOS 26.2, under Japan's smartphone competition law.

In both, a browser may use its own engine only in a build distributed in that region, after
Apple grants the entitlements (`com.apple.developer.web-browser-engine.host` and others),
and with ongoing security commitments. Everywhere else the same app must use WebKit. No
browser has shipped another engine on iOS yet.

**Route 1: our own Chromium content, EU and Japan only.** Chromium 155 has it:

- `content/app/ios/appex/` builds the renderer, GPU and network processes as BrowserEngineKit
  extensions, and `content/shell` has an iOS target with the entitlements file
  (`content/app/ios/appex/app.entitlements`). `use_blink` is off by default on iOS and is
  turned on for this.
- In the simulator, content shell builds without the extensions and runs in one process. On
  a device it needs the extensions and the granted entitlements.
- Builds on this Mac with Xcode: `target_os = ['ios']` in `.gclient`, about an hour, 20 to 30
  GB.
- Go on iOS: a static archive, `GOOS=ios`, as on the Mac.
- The adapter: an iOS window (UIKit), close to Android's one-window model, and the Mobile
  shell.
- We could try this in the simulator with no Apple account at all.

**Route 2: WebKit, worldwide.** A WebKit engine behind our interface. The engine research of
2026-11-15 (`project-b-research/engines/webkit-wkwebview.md`) already ran WKWebView against
our interface on the Mac: our real shell loaded unchanged with a 3-line shim, navigations and
POSTs could be gated, workspaces mapped onto separate data stores. Its gaps: no per-request
decision for subresources (only declarative block lists), no DevTools protocol for our tests,
the engine version follows the OS. On iOS the shell's transparency is public API, where the
Mac needed a private call. It estimated 3 to 5 weeks for a first version on the Mac. On
2026-11-15 WebKit on the Mac was rejected as "just a mac webview engine swap"; on iOS it
would be the only worldwide route, which is a different question.

What our Go core and shell still give on either route: the managers (tabs, history,
bookmarks, workspaces, settings, sync once it is wired), the shells, and our pages.

**Packaging, signing, updates.** An Apple Developer Program membership ($99 a year) for any
device install, TestFlight or the App Store. Updates come from the App Store or TestFlight,
never from our own updater: Apple doesn't allow an app to update its own code.

**Testing here.** The iOS simulator, with the iOS 26.5 runtime already installed: `xcrun
simctl` installs, starts and screenshots. deskrig would need a simulator target, or we use
Apple's UI test tools. A device needs the membership, and for Route 1 the entitlements.

**Effort.** A simulator spike of content shell: 2 to 3 days. A real app on Route 1: 4 to 6
weeks, plus Apple's review of the entitlement. Route 2: 4 to 6 weeks, being a second engine.

## How the tester would run each one here

| Platform | Build | Headless tests | A real desktop | New on this Mac |
|---|---|---|---|---|
| Linux x64 | Container `pb` | Container `pb` | Tart Linux VM, Rosetta | Linux in `.gclient`, the VM |
| Linux arm64 | Container `pb`, cross | An arm64 container | Tart Linux VM | The same, deskrig Linux target |
| Windows arm64, x64 | The Mac, cross | The Windows VM over ssh | The Windows VM | UTM, the VM, the SDK zip, deskrig Windows agent |
| Android tablets, desktop | As today | Emulators | Emulators | Tablet and desktop AVDs |
| ChromeOS | As Android and Linux | None | None | Nothing we can run |
| iOS | The Mac, Xcode | Simulator | Simulator | iOS in `.gclient`; a device needs Apple |

A VM run and a desktop run still must not overlap, as for the macOS VM
([roles/tester.md](../../roles/tester.md)).

## Disk

This Mac had 37 GB free of 1.8 TB. Rough needs, before any tests:

| Target | Where | Extra disk |
|---|---|---|
| Linux, both CPUs | The Colima VM's disk, on this Mac | 25 to 35 GB |
| Windows, both CPUs | `chromium/src` on the Mac | 30 to 40 GB, with the SDK |
| Windows VM | UTM | about 64 GB |
| Linux VM | Tart | about 10 GB |
| iOS | `chromium/src` on the Mac | 20 to 30 GB |

About 170 GB for all of them. **Cleared on 2026-11-19:** unused local models, the spare macOS
VM `pb-build` (remade by `PB_VM=pb-build tools/vm/setup.sh`) and all but the five newest
release candidates came off, which left 507 GB free.

## Recommended order

1. **Free the disk.** Done on 2026-11-19.
2. **Linux x64 and arm64**, the first we can test here: built in the container we have, tested
   headless there and in a Tart VM. It writes the Aura window that Windows reuses.
3. **Android tablets and desktop mode**, alongside, by another session: no build work, tested
   on the emulators we have.
4. **Windows arm64 and x64**, cross-built on the Mac, tested in one Windows 11 on Arm VM.
5. **An iOS simulator spike** of content shell, to see Route 1 run, before deciding between
   the two routes.
6. **ChromeOS** follows from 2 and 3, with an x64 APK if wanted; a real Chromebook to test.

## Decided

- **The core stays in Go; no port to Rust for now** (2026-11-19: "ok no rust for now").
  Rust's gain is one toolchain: Chromium's build compiles Rust for every target it
  supports, which would remove the Windows Go DLL and its separate toolchain. That saves
  about a week on Windows, against two to three months of rewriting about 34,000 lines of
  Go and 14,000 of tests with features stopped. Worth asking again only if Go's runtime
  causes a real problem on a platform, such as iOS's memory limits.

- **`main` starts the Go core, then content, and nothing else** (2026-11-19: "even if we
  need to have main.cc as the app it should just be start gocore start content and thats
  it"). What `engine/project_b_main.cc` decides today before `ContentMain` moves into Go's
  start: `--version`, claiming the profile, the saved flags, and the switch that leaves
  crash signals to Go. Go hands back the command line content runs with.

## Decisions for Sudipta

1. Is the order right: Linux first, Android large screens beside it, then Windows, then iOS?
2. Linux packaging: a self-updating tarball, a `.deb` with an apt repository, or both?
3. Windows signing: ship unsigned at first, as the Mac ships unnotarized, or buy a
   certificate?
4. iOS: is an EU and Japan build interesting, or only worldwide, which means WebKit?
5. Is there an Apple Developer Program membership to use, for iOS devices and for
   notarizing the Mac app?

## Sources

- Chromium 155: `docs/win_cross.md`, `build/config/BUILDCONFIG.gn`,
  `tools/clang/scripts/update.py`, `content/app/ios/appex/`, `content/shell/BUILD.gn`,
  `build/config/features.gni`.
- [Using alternative browser engines in the EU](https://developer.apple.com/support/alternative-browser-engines/)
- [Using alternative browser engines in Japan](https://developer.apple.com/support/alternative-browser-engines-jp)
- [iOS 26.2: Alternative Browser Engines in Japan](https://mjtsai.com/blog/2026/01/06/ios-26-2-alternative-browser-engines-in-japan/)
- [Download Windows 11 for Arm-based PCs](https://www.microsoft.com/software-download/windows11arm64)
- [Windows on Arm ISOs](https://learn.microsoft.com/en-us/windows/arm/iso)
