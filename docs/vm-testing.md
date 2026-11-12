# Testing on a real macOS desktop

`tools/drive_first_app.py` runs with no display, so it can't exercise AppKit's event dispatch,
the window buttons or dragging. `tools/vm/` is for those: a macOS VM with a
real desktop, driven by [deskrig](https://github.com/sudiptadeb/deskrig): real mouse and keys
inside the VM, frames from its window. It is the `vm` group of `tools/test.sh`.

## Status (2026-11-09)

**It runs.** After the owner logged in to the Mac Studio's desktop at 21:19, the VM booted to its
desktop in under a minute, and `stop.sh`, `start.sh`, `install-dmg.sh` and `check.py` then ran
back to back from a stopped VM with no hands. Every piece has now run for real, against the
macOS 26.6.2 guest. If the Mac reboots, or its keys lock again, `start.sh` stops with the host key
error below until someone logs in again.

The DMG tested is `155.0.8059.41` built at 19:33, before main's `8a90a1a` moved the window buttons,
so the three button-position checks fail on it by design: buttons at x 9 to 69, middle y 16, the
address field's middle at 18.

### What the real window showed

Works: the buttons are drawn and active; shell colours are exact; clicks reach the address bar,
back, forward, suggestions and tabs; Cmd+W twice fast closes two tabs; dragging the top row and the
top frame edge moves the window; the left and right edges resize; double-clicking the top row or
the empty sidebar zooms; the green button enters full screen, the buttons hide and the shell gives
their space to its controls. The last full run: 40 passed, 17 failed. The failures are the three
old button positions, first paint over 5 s once while the host was busy (1 to 2 s otherwise), and
the bugs below.

Project B bugs, worst first:

1. **A crash, once.** `EXC_BREAKPOINT` (a `CHECK`) on the browser main thread, in a posted task,
   during a run of drags and outside resizes. It didn't come back on the same steps. The report is
   `~/PW_TOOLS/macos-vm/crashes/project_b-2026-11-09-164114.ips`; the library is stripped and the
   release build on this Mac has been rebuilt since, so it can't be symbolized.
2. **After leaving full screen onto another frame, the shell ignores clicks.** When macOS puts
   the window back on a frame other than the one it left from, such as the zoomed frame it
   remembers, the address field and the layout button stop answering clicks while keys still
   work. Once it lasted over a minute and through a live resize. Steps: zoom, resize from outside,
   full screen, leave by the green button.
3. **Dragging across the address field moves the window** instead of selecting text. Any drag that
   starts in the title bar strip moves the window, controls included.
4. **Ctrl+Cmd+F does nothing**, in or out of full screen: the app's menus are only the app menu
   and Edit, with no View or Window menu.
5. **Shortcuts can wait behind the page.** Right after picking a suggestion by click and moving the
   pointer over the new page, the first Cmd+T or Cmd+L landed 1.1 to 2.4 s late in most tries, and
   in 0.1 to 0.2 s in the last two full runs. Terminal under the same input was instant. The browser main thread sat
   idle in its event wait the whole time, so the key is queued behind the page's input, not stuck
   on a busy thread. Clicks wait the same way: on 2026-11-11, in 0.2.22 and in 0.2.23 alike, a
   click on the layout button right after the tabs moved, or right after a page loaded, took
   effect 1.5 to 3 s later or was lost, and keys typed after it were partly lost.

   **On 2026-11-12 this turned out to be the pictures, not the app.** The app's timeline on
   0.2.27 in the VM had every click in the window within 3 ms and drawn within 30 ms. Typing into
   Terminal in the VM, each letter was in Terminal within 0.2 to 1.1 s (as read through
   accessibility), in the VM's window on this Mac within one capture, and in Screen Sharing's
   picture 0.2 to 14 s later, or not within 15 s. Asking Screen Sharing for whole frames every
   0.1 s, or giving the VM a display, didn't change that. So the checks now take their pictures
   from the VM's window ([Pictures](#pictures)); with them every letter showed in 0.17 to 0.33 s,
   and every timed click and shortcut check passed. The late clicks of bug 2 and the stale
   pictures of bug 7 after a resize or a layout change didn't show either, so at least part of
   those was the same lag; the live resize of bug 7 still runs past the frame in 2 to 4 of 30
   frames.
6. **The empty sidebar doesn't drag the window**, though double-clicking it zooms. Since then
   this is the design: only the empty spaces in the top row drag, and the sidebar is shell, so it
   neither drags nor zooms ([window-manager.md](window-manager.md)). `check.py` expects that.
7. **Stale pictures after resizes.** A resize from outside the app, as window managers and tiling
   do it, shows the old window for 0.8 to 2 s: the whole old frame stays on screen, and after
   shrinking, the old page still shows outside the new frame
   (`shots/real-first/seq05-set-home.png`). Once, after the resize checks, switching the tabs to
   the top left the lower part of the old sidebar painted over the page
   (`shots/20261109-230458/horizontal.png`). A live corner drag kept the page against the frame
   in 30 of 30 frames in one run and 26 of 30 in another, where the page ran up to 12 px past it.
8. **In full screen, showing the title bar covers the shell's top row** with an empty white bar,
   so the address field is hidden while it shows.

`check.py` checks 2 to 7; the crash and 8 were found by hand. Pictures are under
`~/PW_TOOLS/macos-vm/shots/`: one folder per run, and `real-first/` for the probes behind the list
above. They are real pixels from
the guest's screen, not composites; each folder has an `ABOUT.txt` that says so.

### Calibration, for whoever changes the scripts

- Where the shell draws what `check.py` clicks comes from the installed app: before the checks it
  launches the app once more with `--pb-driver` and a throwaway profile under `/tmp/pb-layout`,
  asks for the rectangles in both tab layouts with two tabs open, and AppKit's own place for the
  window buttons and the page. A change to the shell's layout needs no new numbers in the script.
- Drags are posted inside the VM as the guest's own mouse events, with each step's movement
  set. The window server moves a window by those movements. This dates from input over Screen
  Sharing, whose pointer events lost part of a drag; deskrig's own `drag` posts movements too.
- The guest draws at 1x, 1440x900 pixels for 1440x900 points.
- Clicking the wallpaper hides every window (Show Desktop), so the app is opened with `open`.
  `prepare.py` also turns `EnableStandardClickToShowDesktop` off.
- Resting the pointer on the green button opens macOS's Move & Resize menu, which takes the next
  click. After leaving full screen, the pointer moves away.
- After a boot, an "App Background Activity" notice sits over the window's top right; `deskrig vm
  start` and `check.py` close it (`deskrig dismiss-notifications`).
- Leaving full screen puts the window back where macOS last placed it itself, which can be the
  zoomed frame.
- Once per VM: the first window move asks "tart-guest-agent wants to control System Events",
  and until someone answers, every such call waits and times out. `prepare.py`, which `run.sh`
  runs after starting the VM, clicks the prompt's Allow and checks that System
  Events then answers; the grant stays on the VM's disk. Writing it into the VM's TCC database
  doesn't hold, because tccd keeps its own. On a VM that already allows it, `prepare.py` changes
  nothing. It also turns off `EnableStandardClickToShowDesktop`.

## Why the VM needs a desktop login

`tart run` stops within a second:

    VZErrorDomain Code=-9 "The virtual machine encountered a security error."
    Failed to get current host key. Failed to create new HostKey.

The system log shows why. `com.apple.Virtualization.VirtualMachine` asks the Secure Enclave
to sign with, and then to make, its host key, and both answer `errSecInteractionNotAllowed`,
with `AKSError=-536870174` (`0xE00002E2`, not permitted). Only macOS guests need this key,
which is why Linux VMs and colima run fine. Any tool built on Virtualization should hit the
same wall, because the failure is inside Apple's service; Tart is the only one tried.

A small test program made Secure Enclave keys from the same account, with nothing stored:

| Key's protection class | Result |
|---|---|
| `AlwaysThisDeviceOnly` | Made, and signed |
| `AfterFirstUnlockThisDeviceOnly` | Refused the same way |
| `WhenUnlockedThisDeviceOnly` | Refused the same way |

So the enclave works, and this account's keys have stayed locked since the Mac booted. FileVault is
on, the console is at the login window, and the failure is the same from a tool shell and from
amarit. Only a real login with the account's password unlocks them, and only for that boot.

**What's needed:** log in as `debmin` at the Mac's desktop once after each boot, and stay logged
in. Screen Sharing answers on port 5900 and reaches the login window. Then run `start.sh`. Tested
on 2026-11-09: after that login the VM started at once. Whether it still starts with the screen
locked is untested. The Mac is managed and reboots on its own, so this will come up again.

Not tried, on purpose: `sudo`, the keychain, or anything that goes around the enclave.

## Run it

With nobody watching, one command from a DMG to a verdict:

    tools/vm/setup.sh                 # once: Tart, Python, the image, the VM
    tools/vm/run.sh <DMG path or URL> # or tools/test.sh vm, on build/project-b.dmg or PB_DMG

`run.sh` takes the VM's lock, starts the VM if it was stopped, readies it, installs the DMG, runs
`check.py`, copies the app's crash reports from the VM, and stops the VM again if it started
it. It writes everything to one folder, `~/PW_TOOLS/macos-vm/shots/<time>-<VM>`, which
`shots/latest-<VM>` points to:

- `summary.txt`: one line per check, `pass`, `known` or `FAIL`, and the verdict last.
- `results.json`: the same, for a script.
- `index.html`: the checks and every shot on one page.
- `run.txt`: all it printed. `project_b-*.ips`: any crash reports.

Exit 0 is a pass: every check passed, or failed only as a known bug. A check that fails
because of a bug we know about is listed in `KNOWN` at the top of `check.py`, with the bug;
it's reported as `known` and doesn't fail the run. When it passes, the line says the known bug
didn't show, so the entry can go once the bug is fixed. Exit 1 is a new failure, or the checks
stopped early (the app quit or crashed). Exit 2 is that it couldn't run: no DMG, another run
holds the lock, or the VM didn't start.

Nothing the checks open comes from the internet: `check.py` writes two one-colour pages into
the VM and serves them there on `127.0.0.1:8765`. Every launch starts from a fresh profile with
automatic update checks off, so no update notice comes up over a page.

A check of whether something works waits up to several seconds for its result, and fails only
if it never comes. Only the checks named "within" time things. On 2026-11-11, with Screen
Sharing's pictures, five runs in a row on 0.2.24 all passed with three or four known failures
each, from bugs 2, 5 and 7. On 2026-11-12, with the pictures from the VM's window, three runs in
a row on 0.2.27 passed 64 of 65, every "within" check in time; the one known failure is the live
resize (bug 7).

By hand, a step at a time:

    tools/vm/start.sh                 # boots it and waits for the desktop
    tools/vm/install-dmg.sh <path or URL>
    ~/PW_TOOLS/macos-vm/venv/bin/python tools/vm/check.py
    tools/vm/stop.sh

### A second VM

`PB_VM` names the VM every script here works on, `pb-test` by default. Each VM is a deskrig
target of the same name, with its own run lock, so two run side by side, for example one for
the release checks and one for a feature's:

    PB_VM=pb-build tools/vm/setup.sh  # once: a VM of its own, from the image
    PB_VM=pb-build tools/vm/run.sh <DMG>

Each VM is made from the image, not from another VM, so they share nothing. Tart clones the
image with APFS's copy-on-write, so a new VM takes little disk until it changes. macOS runs at
most two macOS VMs at once. `run.sh` readies a new VM by itself (`prepare.py`, see Calibration).

What the checks need, and what they say when it's missing:

- **deskrig**, in `~/.local/bin`: `curl -fsSL https://raw.githubusercontent.com/sudiptadeb/deskrig/main/install.sh | sh`.
  `tools/vm/deskrig.py` is a copy of its Python client.
- **The VM's own Python**, `~/PW_TOOLS/macos-vm/venv/bin/python`, which `setup.sh` makes with
  numpy and pillow. The system's Python has neither, and `check.py` says so under it.
  `tools/test.sh vm` uses the right one.
- **The VM running**, by `start.sh`, which needs the desktop login below once per boot.
- **This Mac's desktop unlocked, and deskrig allowed to record the screen**, for the VM's frames
  ([Pictures](#pictures)). `deskrig status` and `deskrig setup permissions` say what's missing.
- **The app installed in the VM**, by `install-dmg.sh`.

`check.py` prints pass or FAIL for each check with what it measured, and saves a shot per step
under `~/PW_TOOLS/macos-vm/shots/`. Look at the shots; a check only compares pixels.

To drive it by hand, with coordinates in points, use deskrig on the VM's target:

    deskrig --target pb-test shot out.png | click X Y | dblclick X Y | drag X1 Y1 X2 Y2 | key cmd+t | type "text"
    deskrig --target pb-test run -- <command>   # inside the VM, in its desktop session

## Pictures

The VM runs in a window on this Mac's desktop, and `deskrig frames` keeps that window's latest
frame, cropped to the VM's screen, in a mapped file, through ScreenCaptureKit, which sends a
frame whenever the window changes. `screen.py` reads it; `wait_frame` waits for the next one.
The mouse and keys go in inside the VM, through Tart's guest agent.

The window shows the VM's screen when it changes; Screen Sharing's picture of it, which these
checks used until 2026-11-12, came 0.2 to 14 s late, which looked like the app taking clicks
late (bug 5). The cost: the window sits on the desktop while the VM runs, and the desktop has to
stay logged in and unlocked. The capture works with other windows over it, but not with it
minimised.

## Decisions

- **Tart**, because it needs no sudo, ships signed with the Virtualization entitlement, and has
  maintained images. It moved to `github.com/openai/tart`; the images stay on `ghcr.io/cirruslabs`.
- **`macos-tahoe-base`, not `-vanilla`**: three months newer, and it has Tart's guest agent, so
  `tart exec` works. It's macOS 26.6.2, with auto-login. Its login, `admin` / `admin`, is a public
  default. It holds none of the owner's credentials.
- **No VNC.** Tart's `--vnc-experimental` listens on every interface (`lsof`: `TCP *:59307`)
  though it prints `127.0.0.1`, and using it needs the owner's OK. The VM's own Screen Sharing
  sent its pictures seconds late. deskrig needs neither: input through Tart's guest agent,
  frames from the window ([Pictures](#pictures)). Nothing listens on this Mac's network.
- **1440x900 points.** The guest draws them at 1x, so a shot has one pixel per point.
- **The DMG goes in through deskrig's read-only shared folder** (`deskrig install`), so it has no
  quarantine flag. Gatekeeper
  and the signing profile aren't tested here.
- **Window positions come from macOS's window list**, read inside the VM. Only the checks read
  it; the app gets its input from deskrig.

## Remove it

    tools/vm/stop.sh
    rm -rf ~/PW_TOOLS/macos-vm

That frees about 33 GB.
