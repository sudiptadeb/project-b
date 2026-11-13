#!/usr/bin/env python3
"""Copy and paste in the real app, through Go's keys and the Mac's clipboard.

NEEDS A PERSON LOGGED IN at this Mac's screen: the clipboard belongs to their desktop
session, and an app started from a background job can't reach it. It replaces what they
had copied with the words "copied text". Start it through on_screen.py:

    tools/test.sh screen                                   # with the other real-screen checks
    tools/on_screen.py python3 tools/check_clipboard.py    # alone

PB_BIN picks another binary.
"""

import os
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import drive_first_app as drive  # noqa: E402
from drive_first_app import INPUT, VALUE, check  # noqa: E402


def clipboard_checks():
    """Copy and paste go through Go to whichever page has the keyboard: here, the shell."""
    app = drive.App(tempfile.mkdtemp(prefix="pb-profile-"))
    drive.focus_bar(app)
    app.cmd("type", "copied text")
    app.wait(f"{VALUE}==='copied text'")
    app.cmd("key", "cmd+a")
    app.cmd("key", "cmd+c")
    app.js(f"{INPUT}.value=''")
    app.cmd("key", "cmd+v")
    try:
        app.wait(f"{VALUE}==='copied text'", 5)
        check("Cmd+A, Cmd+C and Cmd+V work in the address bar", True)
    except TimeoutError as e:
        check("Cmd+A, Cmd+C and Cmd+V work in the address bar", False, e)
    app.close()


if __name__ == "__main__":
    os.makedirs(drive.OUT, exist_ok=True)
    drive.run_parts([clipboard_checks])
