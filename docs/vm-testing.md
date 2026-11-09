# Testing on a real macOS desktop

`tools/drive_first_app.py` runs with no display, so it can't exercise AppKit's event dispatch,
the window buttons, dragging or real full screen. `tools/vm/` is for those: a macOS VM with a
real desktop, driven from outside through its display, mouse and keyboard.

## Status (2026-11-09)

**The macOS VM hasn't booted on the Mac Studio yet, so the app hasn't been tested in it.** It
needs one desktop login first (next section).

| Piece | How far it has run |
|---|---|
| Tart under amarit | A Linux VM ran. The macOS VM stops with the error below |
| `setup.sh`, `start.sh`'s failure path, `stop.sh` | Ran for real |
| `rfb.py` frames and keys | Ran against Virtualization's own VNC server, on a Linux VM's console |
| `rfb.py` Apple login | Ran only against a stand-in server written for the test |
| `install-dmg.sh` | Never run: it needs the VM |
| `check.py` | Ran only against a stand-in desktop, which found script errors and says nothing about the app |

Expect the first real run of `check.py` to need calibrating: colours, the key for Command, and
positions in the window.

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
in. Screen Sharing answers on port 5900 and should reach the login window. Then run `start.sh`.
This is the expected fix, not a tested one. It isn't known whether the key also needs the screen
unlocked when the VM starts, so keep it unlocked for the first try. The Mac is managed and
reboots on its own, so this will come up again.

Not tried, on purpose: `sudo`, the keychain, or anything that goes around the enclave.

## Run it

    tools/vm/setup.sh                 # once: Tart, Python, the image, the VM
    tools/vm/start.sh                 # boots it and waits for the desktop
    tools/vm/install-dmg.sh <path or URL>
    "$HOME/PW_TOOLS/macos-vm/venv/bin/python" tools/vm/check.py
    tools/vm/stop.sh

`check.py` prints pass or FAIL for each check with what it measured, and saves a shot per step
under `~/PW_TOOLS/macos-vm/shots/`. Look at the shots; a check only compares pixels.

To drive it by hand, with coordinates in points:

    rfb.py shot out.png | click X Y | dbl X Y | move X Y | drag X1 Y1 X2 Y2 | key cmd+t | type "text"

Run those with the same Python. `TART_HOME=~/PW_TOOLS/macos-vm/tart-home tart exec pb-test <command>`
runs a command inside the VM with no password and no network.

## Decisions

- **Tart**, because it needs no sudo, ships signed with the Virtualization entitlement, and has
  maintained images. It moved to `github.com/openai/tart`; the images stay on `ghcr.io/cirruslabs`.
- **`macos-tahoe-base`, not `-vanilla`**: three months newer, and it has Tart's guest agent, so
  `tart exec` works. It's macOS 26.6.2, with auto-login. Its login, `admin` / `admin`, is a public
  default. It holds none of the owner's credentials.
- **The VM's own Screen Sharing, not Tart's `--vnc-experimental`.** Tart prints `127.0.0.1` for that
  server, but it listens on every interface (`lsof`: `TCP *:59307`), and a sandbox profile can only
  block the bind, not limit who connects. A firewall rule would need root. Screen Sharing is on in
  the image, on the VM's private address, so nothing new listens on this Mac's network. The same goes
  for the VM's SSH, which these scripts don't use. Frames and input then go through macOS inside
  the VM instead of the virtual display and keyboard. The image already grants Screen Sharing the
  permissions, so there should be no prompts. If this doesn't work out, `rfb.py` also has the login
  for the other server, but using that one needs the owner's OK.
- **1440x900 points at 2x**, so half-point offsets show in shots.
- **The DMG goes in through a read-only shared folder**, so it has no quarantine flag. Gatekeeper
  and the signing profile aren't tested here.
- **Window positions come from macOS's window list**, read inside the VM. Only the checks read
  it; the app gets its input through VNC.

## Remove it

    tools/vm/stop.sh
    rm -rf ~/PW_TOOLS/macos-vm

That frees about 33 GB.
