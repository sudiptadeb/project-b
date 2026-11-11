#!/usr/bin/env python3
"""Readies a running VM for check.py, once per VM. run.sh runs it after start.sh; it changes
nothing on a VM that is ready.

check.py moves and sizes the app's window through System Events, by tart exec, so
tart-guest-agent needs macOS's permission to control System Events. A VM made fresh from the
image doesn't have it: the first such call puts up a prompt, and while nobody answers, every
call waits and times out. Writing the grant into the VM's TCC database doesn't hold, because
tccd keeps its own. So this answers the prompt as a person would: it asks System Events
something, finds the prompt's blue Allow button on the VM's screen, and clicks it.

Run it with the VM's own Python, with PB_VM set for a VM other than pb-test.
"""

import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import rfb  # Stops here, saying so, under a Python without numpy.

import numpy as np

TART = rfb.VM_HOME + "/bin/tart.app/Contents/MacOS/tart"
os.environ["TART_HOME"] = rfb.VM_HOME + "/tart-home"
ASK = ['with timeout of 10 seconds', 'tell application "System Events" to count processes', 'end timeout']
BLUE = (0, 122, 255)  # macOS's accent blue, the filled button of a prompt.
BUTTON = (100, 22)  # Smaller than the Allow button, in points.


def guest(*cmd, timeout=60):
    return subprocess.run([TART, "exec", rfb.VM, *cmd], capture_output=True, text=True, timeout=timeout)


def answers():
    """Whether System Events answers tart-guest-agent within 10 s."""
    args = [a for line in ASK for a in ("-e", line)]
    try:
        return guest("/usr/bin/osascript", *args, timeout=30).returncode == 0
    except subprocess.TimeoutExpired:
        return False


def allow_button(img, scale):
    """The middle of the densest patch of accent blue the size of a button, or None. The blue
    shows elsewhere too, in the Dock's icons, but not as a filled rectangle that big."""
    mask = np.all(np.abs(img.astype(np.int16) - np.array(BLUE, dtype=np.int16)) <= 12, axis=2).astype(np.int32)
    w, h = int(BUTTON[0] * scale), int(BUTTON[1] * scale)
    sums = np.pad(mask, ((1, 0), (1, 0))).cumsum(0).cumsum(1)
    area = sums[h:, w:] - sums[:-h, w:] - sums[h:, :-w] + sums[:-h, :-w]
    y, x = np.unravel_index(np.argmax(area), area.shape)
    if area[y, x] < 0.9 * w * h:
        return None
    return (x + w / 2) / scale, (y + h / 2) / scale


def main():
    # Clicking the wallpaper would hide every window (Show Desktop), and the checks click near it.
    guest("/usr/bin/defaults", "write", "com.apple.WindowManager", "EnableStandardClickToShowDesktop", "-bool", "false")
    if answers():
        print("System Events: tart-guest-agent may control it")
        return
    vnc = rfb.Vnc()
    for _ in range(3):
        # Ask in the background, so the prompt is up while this looks for it.
        args = [a for line in ASK for a in ("-e", line)]
        asking = subprocess.Popen([TART, "exec", rfb.VM, "/usr/bin/osascript", *args],
                                  stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        where = None
        for _ in range(16):
            time.sleep(0.5)
            where = allow_button(vnc.image(), vnc.scale)
            if where:
                break
        if where:
            print(f"clicking Allow at {where[0]:.0f}, {where[1]:.0f}")
            vnc.click(*where)
        asking.wait(timeout=60)
        if answers():
            print("System Events: tart-guest-agent may control it now")
            return
    vnc.shot(rfb.VM_HOME + f"/prepare-{rfb.VM}.png")
    sys.exit(f"System Events still doesn't answer tart-guest-agent in {rfb.VM}. The screen is in "
             f"{rfb.VM_HOME}/prepare-{rfb.VM}.png: if a prompt shows, click Allow by hand (docs/vm-testing.md).")


main()
