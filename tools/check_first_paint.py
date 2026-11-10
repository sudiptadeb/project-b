#!/usr/bin/env python3
"""The first page a ChromeDriver session loads shows in the real window.

ChromeDriver navigates within a second of the app starting. The shell's view, layered
over the page, used to stay white there until the shell next painted, so a static page
stayed hidden though it had rendered. This loads a static page in a solid colour that
way and checks a capture of the real window, taken with desk, for that colour. The
app's own captures read the page's surface and would pass anyway. Run it from the repo
root on a Mac with someone logged in at the console:

    python3 tools/check_first_paint.py

PB_BIN picks another binary, CHROMEDRIVER another ChromeDriver. The capture goes to
build/first-paint/.
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
BIN = os.environ.get("PB_BIN") or os.path.join(ROOT, "chromium/src/out/pb/project_b")
CHROMEDRIVER = os.environ.get("CHROMEDRIVER") or os.path.join(
    ROOT, ".tools/chromedriver/chromedriver-mac-arm64/chromedriver")
DESK = os.environ.get("DESK") or os.path.join(ROOT, ".tools/desk/desk")
OUT = os.path.join(ROOT, "build/first-paint")

COLOUR = (31, 122, 77)
PAGE = b"""<!doctype html><title>Green</title>
<style>html, body { margin: 0; height: 100%; background: rgb(31, 122, 77); }</style>"""

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
from drive_chromedriver import free_port, http_json  # noqa: E402


class Fixture(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.end_headers()
        self.wfile.write(PAGE)

    def log_message(self, *args):
        pass


def window_name():
    # desk names a window by its app: the bundle's name for a packaged app.
    parts = BIN.split(os.sep)
    bundles = [p for p in parts if p.endswith(".app")]
    return bundles[-1][:-len(".app")] if bundles else os.path.basename(BIN)


def main():
    status = subprocess.run([DESK, "status"], capture_output=True, text=True).stdout if os.path.exists(DESK) else ""
    if "on console: 1" not in status or "screen locked: no" not in status:
        print("skipped: needs desk and an unlocked console session to capture the real window")
        print(status.strip())
        return 0

    if os.path.exists(OUT):
        shutil.rmtree(OUT)
    os.makedirs(OUT)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Fixture)
    threading.Thread(target=server.serve_forever, daemon=True).start()

    port = free_port()
    cd = subprocess.Popen([CHROMEDRIVER, f"--port={port}"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    wd = f"http://127.0.0.1:{port}"
    base = None
    try:
        deadline = time.time() + 15
        while time.time() < deadline:
            try:
                if http_json("GET", wd + "/status", timeout=2)[0] == 200:
                    break
            except OSError:
                time.sleep(0.1)
        caps = {"capabilities": {"alwaysMatch": {"goog:chromeOptions": {"binary": BIN}}}}
        status, r = http_json("POST", wd + "/session", caps, timeout=90)
        if status != 200:
            print("FAIL session ->", r)
            return 1
        base = f"{wd}/session/{r['value']['sessionId']}"
        http_json("POST", base + "/url", {"url": f"http://127.0.0.1:{server.server_port}/"})
        time.sleep(2)

        shot = os.path.join(OUT, "window.png")
        subprocess.run([DESK, "shot", shot, window_name()], capture_output=True)
        if not os.path.exists(shot):
            print(f"FAIL no capture of the {window_name()} window")
            return 1
        image = Image.open(shot).convert("RGB")
        total = image.width * image.height
        share = sum(n for n, c in image.getcolors(total) if all(abs(a - b) <= 3 for a, b in zip(c, COLOUR))) / total
        # The page area is well over half the window; a white page area has none of it.
        ok = share > 0.4
        print(("ok   " if ok else "FAIL ") + f"the first page shows in the window: {share:.0%} of it is the page's colour ({shot})")
        return 0 if ok else 1
    finally:
        if base:
            http_json("DELETE", base)
        cd.kill()
        server.shutdown()


if __name__ == "__main__":
    sys.exit(main())
