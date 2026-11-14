#!/usr/bin/env python3
"""The first page a ChromeDriver session loads shows in the real window.

ChromeDriver navigates within a second of the app starting. The shell's view, layered
over the page, used to stay white there until the shell next painted, so a static page
stayed hidden though it had rendered. This loads a static page in a solid colour that
way and checks a capture of the real window, taken with deskrig, for that colour. The
app's own captures read the page's surface and would pass anyway.

NEEDS A PERSON LOGGED IN at this Mac's screen, and deskrig allowed to record it
(github.com/sudiptadeb/deskrig; `deskrig setup permissions`). The app has to start inside the desktop session for its
window to be on the screen, so start it through on_screen.py:

    tools/test.sh screen                                     # with the other real-screen checks
    tools/on_screen.py python3 tools/check_first_paint.py    # alone

PB_BIN picks another binary, CHROMEDRIVER another ChromeDriver, DESKRIG another deskrig. The
capture goes to build/first-paint/.
"""

import http.server
import os
import shutil
import subprocess
import sys
import threading
import time

from PIL import Image

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
BIN = os.environ.get("PB_BIN") or os.path.join(ROOT, "build/pb/project_b")
CHROMEDRIVER = os.environ.get("CHROMEDRIVER") or os.path.join(
    ROOT, ".tools/chromedriver/chromedriver-mac-arm64/chromedriver")
DESKRIG = [os.environ.get("DESKRIG") or "deskrig", "--target", "local"]
OUT = os.path.join(ROOT, "build/first-paint")

COLOUR = (31, 122, 77)
PAGE = b"""<!doctype html><title>Green</title>
<style>html, body { margin: 0; height: 100%; background: rgb(31, 122, 77); }</style>"""

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
from drive_chromedriver import http_json, start_chromedriver  # noqa: E402


class Fixture(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.end_headers()
        self.wfile.write(PAGE)

    def log_message(self, *args):
        pass


def app_started_by(pid):
    """The app ChromeDriver started: its one child process. A person's own copy of the
    app may be open on the same screen, so the capture is of this process's window."""
    ps = subprocess.run(["ps", "-axo", "pid=,ppid="], capture_output=True, text=True).stdout
    return next(int(line.split()[0]) for line in ps.splitlines() if int(line.split()[1]) == pid)


def page_colour_share(shot, app):
    """How much of the app's window, captured now, is the page's colour."""
    if os.path.exists(shot):
        os.remove(shot)
    taken = subprocess.run(DESKRIG + ["shot", shot, str(app)], capture_output=True, text=True)
    if not os.path.exists(shot):
        return 0, taken.stderr.strip()
    image = Image.open(shot).convert("RGB")
    total = image.width * image.height
    return sum(n for n, c in image.getcolors(total) if all(abs(a - b) <= 3 for a, b in zip(c, COLOUR))) / total, ""


def main():
    try:
        status = subprocess.run(DESKRIG + ["status"], capture_output=True, text=True).stdout
    except FileNotFoundError:
        status = ""
    if "on console: true" not in status or "screen locked: false" not in status or "screen recording: true" not in status:
        print("NEEDS A PERSON AT THE SCREEN: deskrig has to capture the real window, which takes someone logged in, "
              "the screen unlocked, and Screen Recording allowed for deskrig (deskrig setup permissions). deskrig status says:")
        print(status.strip() or "nothing: it isn't installed")
        return 2

    if os.path.exists(OUT):
        shutil.rmtree(OUT)
    os.makedirs(OUT)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Fixture)
    threading.Thread(target=server.serve_forever, daemon=True).start()

    cd, _, wd = start_chromedriver(CHROMEDRIVER, os.path.join(OUT, "chromedriver.log"))
    base = None
    try:
        caps = {"capabilities": {"alwaysMatch": {"goog:chromeOptions": {"binary": BIN}}}}
        status, r = http_json("POST", wd + "/session", caps, timeout=90)
        if status != 200:
            print("FAIL session ->", r)
            return 1
        base = f"{wd}/session/{r['value']['sessionId']}"
        app = app_started_by(cd.pid)
        http_json("POST", base + "/url", {"url": f"http://127.0.0.1:{server.server_port}/"})

        # The page has loaded. Its colour reaches the window with the next frame; a page
        # that stays hidden never gets there. The page area is well over half the window,
        # and a white page area has none of the colour.
        shot = os.path.join(OUT, "window.png")
        deadline = time.time() + 10
        while True:
            share, error = page_colour_share(shot, app)
            if share > 0.4 or time.time() > deadline:
                break
        ok = share > 0.4
        print(("ok   " if ok else "FAIL ") + f"the first page shows in the window: {share:.0%} of it is the page's colour ({shot})"
              + (f" -> {error}" if error else ""))
        return 0 if ok else 1
    finally:
        if base:
            http_json("DELETE", base)
        cd.kill()
        server.shutdown()


if __name__ == "__main__":
    sys.exit(main())
