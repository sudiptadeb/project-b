#!/usr/bin/env python3
"""Runs a command inside the logged-in desktop session and passes on its output and exit code.

NEEDS A PERSON LOGGED IN at this Mac's screen, with the screen unlocked.

Agent, ssh and amarit sessions on a Mac live in a Background session. An app started from
one can't enter full screen and its window may never reach the screen, so the tests that
look at the real window start through here: the command runs as a one-shot launchd job in
the desktop's gui domain.

    tools/on_screen.py                              # says who is at the screen, exit 0 if someone is
    tools/on_screen.py python3 tools/check_full_screen.py

The job gets this shell's folder and environment.
"""

import getpass
import os
import plistlib
import shutil
import subprocess
import sys
import tempfile
import time

DOMAIN = "gui/%d" % os.getuid()
LABEL = "dev.projectb.on-screen.%d" % os.getpid()


def refusal():
    """Why nothing can run on the screen now, or None when this user is logged in and unlocked."""
    root = plistlib.loads(subprocess.run(["ioreg", "-n", "Root", "-d1", "-a"], capture_output=True).stdout)
    me = getpass.getuser()
    mine = [u for u in root.get("IOConsoleUsers", []) if u.get("kCGSSessionUserNameKey") == me]
    if not any(u.get("kCGSSessionOnConsoleKey") for u in mine):
        return f"nobody is logged in as {me} at this Mac's screen"
    if any(u.get("CGSSessionScreenIsLocked") for u in mine):
        return "the screen is locked"
    return None


def launchctl(*args):
    return subprocess.run(["launchctl", *args], capture_output=True, text=True)


def run(command):
    with tempfile.TemporaryDirectory() as work:
        out = os.path.join(work, "out")
        open(out, "w").close()
        plist = os.path.join(work, "job.plist")
        with open(plist, "wb") as f:
            plistlib.dump({
                "Label": LABEL,
                # launchd doesn't look a program up in this shell's PATH.
                "ProgramArguments": [shutil.which(command[0]) or command[0], *command[1:]],
                "WorkingDirectory": os.getcwd(),
                "EnvironmentVariables": dict(os.environ),
                "RunAtLoad": True,
                "LimitLoadToSessionType": "Aqua",
                "StandardOutPath": out,
                "StandardErrorPath": out,
            }, f)
        started = launchctl("bootstrap", DOMAIN, plist)
        if started.returncode != 0:
            sys.exit("can't start in the desktop session: " + started.stderr.strip())
        code = None
        try:
            with open(out, errors="replace") as shown:
                while code is None:
                    state = launchctl("print", f"{DOMAIN}/{LABEL}").stdout
                    if "state = running" not in state:
                        if "last terminating signal" in state:
                            code = 1
                        elif "last exit code = " in state:
                            value = state.split("last exit code = ", 1)[1].split("\n", 1)[0].strip()
                            if value.isdigit():
                                code = int(value)
                    sys.stdout.write(shown.read())
                    sys.stdout.flush()
                    if code is None:
                        time.sleep(0.2)
        finally:
            launchctl("bootout", f"{DOMAIN}/{LABEL}")
        return code


if __name__ == "__main__":
    why = refusal()
    if why:
        sys.exit(f"NEEDS A PERSON AT THE SCREEN: {why}. Log in at the Mac's desktop (Screen Sharing works), "
                 "leave it unlocked, and run this again.")
    if len(sys.argv) == 1:
        print("someone is logged in at the screen and it is unlocked")
        sys.exit(0)
    sys.exit(run(sys.argv[1:]))
