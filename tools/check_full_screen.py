#!/usr/bin/env python3
"""Real full screen in the real app: by AppKit's own toggle, by the shell's request and by key.

NEEDS A PERSON LOGGED IN at this Mac's screen. An app started from an agent, ssh or amarit
session can't enter full screen, so start it through on_screen.py:

    tools/test.sh screen                                     # with the other real-screen checks
    tools/on_screen.py python3 tools/check_full_screen.py    # alone

The window takes the whole screen for a few seconds. PB_BIN picks another binary.
"""

import json
import os
import sys
import tempfile
import time

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import drive_first_app as drive  # noqa: E402
from drive_first_app import BAR_Y, check  # noqa: E402

RADIUS = "getComputedStyle(document.documentElement).getPropertyValue('--window-radius').trim()"
SIDEBAR_X = "document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x"
# macOS drops a toggle sent while its own transition still runs, and nothing the app
# reports says when that has ended.
TRANSITION = 1.0


def full_screen_checks():
    """Into full screen and out again, three ways."""
    app = drive.App(tempfile.mkdtemp(prefix="pb-profile-"))
    state = lambda: json.loads(app.cmd("state"))
    native = lambda: json.loads(app.cmd("frame"))  # The border the window itself draws.
    windowed = app.js(RADIUS)
    controls_end = json.loads(app.cmd("bounds"))["buttonsEnd"] + 8

    def enter(toggle):
        toggle()
        app.wait(f"{RADIUS}==='0px'", 10)
        time.sleep(TRANSITION)

    def leave(toggle):
        toggle()
        app.wait(f"{RADIUS}==='{windowed}'&&{SIDEBAR_X}==={controls_end}", 20)
        time.sleep(TRANSITION)

    # AppKit's toggle, which the green button sends. The corners go square and the shell
    # takes the window buttons' space; both come back with the window.
    toggle = lambda: app.cmd("fullscreen")
    enter(toggle)
    check("full screen has square corners and no window buttons", native()["radius"] == 0 and app.js(SIDEBAR_X) == 12, (native(), app.js(SIDEBAR_X)))
    leave(toggle)
    check("leaving full screen brings the curve and the buttons back", not state()["fullScreen"] and f"{native()['radius']}px" == windowed, (state(), native()))

    # The shell asks Go's window manager, the manager tells the engine, and the engine
    # reports the result. In full screen nothing drags.
    toggle = lambda: app.js("(chrome.send('shell',[JSON.stringify({t:'window',text:'fullScreen'})]),1)")
    enter(toggle)
    check("the shell's request goes through the manager into full screen", state()["fullScreen"], state())
    right_end = app.cmd("hit", app.js("window.innerWidth") - 6, BAR_Y)
    check("in full screen the empty top row is shell, not drag", right_end == "shell", right_end)
    leave(toggle)
    # macOS may put the window back at another size, so the end of the row is measured.
    right_end = app.cmd("hit", app.js("window.innerWidth") - 6, BAR_Y)
    check("and out again, where the empty top row drags", not state()["fullScreen"] and right_end == "frame", (state(), right_end))

    # The View menu's key, through Go's keymap.
    toggle = lambda: app.cmd("key", "ctrl+cmd+f")
    enter(toggle)
    check("Ctrl+Cmd+F enters full screen", state()["fullScreen"], state())
    leave(toggle)
    check("and leaves it", not state()["fullScreen"], state())
    app.close()


if __name__ == "__main__":
    os.makedirs(drive.OUT, exist_ok=True)
    drive.run_parts([full_screen_checks])
