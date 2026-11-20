#!/usr/bin/env python3
"""The guest's half of tools/linux/check_real_window.py, run in a Linux desktop session: starts
the app with our driver, opens a page, saves what the driver composes (the shell's surface over
the page's), writes the app's pid, and holds the window open until the Mac's half has taken its
shot of the real window (or a minute has gone).

    PB_BIN=<project_b> python3 tools/linux/real_window_guest.py <out folder>
"""

import json
import os
import sys
import tempfile
import time

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import drive_first_app as d  # noqa: E402


def main():
    out = os.path.abspath(sys.argv[1])
    os.makedirs(out, exist_ok=True)
    os.makedirs(d.OUT, exist_ok=True)
    for name in ("ready", "done", "composed.png", "pid", "error"):
        try:
            os.remove(os.path.join(out, name))
        except OSError:
            pass
    app = None
    try:
        app = d.App(tempfile.mkdtemp(prefix="pb-profile-"))
        d.from_box(app, d.PAGES)
        time.sleep(1)
        d.page_click(app, "body")  # A page with the keyboard, as once hid the shell.
        time.sleep(0.5)
        reply = app.cmd("capture", os.path.join(out, "composed.png"))
        with open(os.path.join(out, "pid"), "w") as f:
            f.write(str(app.proc.pid))
        with open(os.path.join(out, "bounds.json"), "w") as f:
            f.write(app.cmd("bounds"))
        open(os.path.join(out, "ready"), "w").write(reply)
        deadline = time.time() + 60
        while time.time() < deadline and not os.path.exists(os.path.join(out, "done")):
            time.sleep(0.2)
    except Exception as e:  # noqa: BLE001
        open(os.path.join(out, "error"), "w").write(repr(e))
    finally:
        if app:
            app.close()


if __name__ == "__main__":
    main()
