#!/usr/bin/env python3
"""Drives the real Android app on an emulator: real touches, real keys, real screenshots.

A small local web server is the oracle for what the page received, and the app's own history
file for what the managers kept. Results and screenshots go to build/android-drive.

    ADB_SERIAL=emulator-5600 APK=build/android/project-b-webview.apk python3 tools/drive_android.py
"""
import http.server
import json
import os
import subprocess
import threading
import time

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SDK = os.path.expanduser("~/Library/Android/sdk")
ADB = [SDK + "/platform-tools/adb", "-s", os.environ.get("ADB_SERIAL", "emulator-5600")]
APK = os.environ.get("APK", ROOT + "/build/android/project-b-webview.apk")
PACKAGE = os.environ.get("PACKAGE", "dev.projectb.webview")
ACTIVITY = os.environ.get("ACTIVITY", ".BrowserActivity")
OUT = os.environ.get("OUT", ROOT + "/build/android-drive")
PORT = 8765
BASE = "localhost:%d" % PORT

# Pixel 6 screen, 1080x2400 at density 2.625, status bar 53dp tall. All in device pixels.
FIELD = (680, 190)
BACK = (57, 190)
FORWARD = (152, 190)
RELOAD = (246, 190)
FIRST_SUGGESTION = (680, 320)
PAGE_TOP = (540, 300)
PAGE_MIDDLE = (540, 1300)

PAGES = {
    "/a": "<title>Page A</title><a href=/b style='display:block;height:200px;font-size:40px'>to b</a>",
    "/b": "<title>Page B \U0001F642</title>b",
    "/c": "<title>Page C</title>c",
    "/tap": "<title>Tap</title><button style='position:fixed;left:0;top:0;width:100%;height:200px'"
            " onclick=\"fetch('/tapped')\">tap</button>",
    "/tall": "<title>Tall</title><div style='height:5000px'>tall</div>"
             "<script>onscroll=()=>fetch('/scrolled?y='+Math.round(scrollY))</script>",
    "/busy": "<title>Busy</title><button style='width:100%;height:200px' onclick='for(;;);'>busy</button>",
    "/form": "<title>Form</title><form method=post action=http://blocked.test/>"
             "<button style='width:100%;height:200px'>post</button></form>",
    "/to-blocked": None,
}

served = []
results = []


class Server(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        served.append(self.path)
        path = self.path.split("?")[0]
        if path == "/to-blocked":
            self.send_response(302)
            self.send_header("Location", "http://blocked.test/")
            self.end_headers()
            return
        body = PAGES.get(path)
        head = "<!doctype html><meta charset=utf-8><meta name=viewport content='width=device-width'>"
        data = (head + (body or "")).encode()
        self.send_response(200 if body else 404)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        # So that back and forward show up here instead of being answered from the cache.
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args):
        pass


def adb(*args, check=False):
    return subprocess.run(ADB + list(args), capture_output=True, text=True, check=check).stdout


def shot(name):
    with open("%s/%s.png" % (OUT, name), "wb") as f:
        subprocess.run(ADB + ["exec-out", "screencap", "-p"], stdout=f)


def tap(point, wait=0.8):
    adb("shell", "input", "tap", str(point[0]), str(point[1]))
    time.sleep(wait)


def type_text(text):
    adb("shell", "input", "text", text.replace(" ", "%s"))
    time.sleep(0.8)


def key(code, wait=0.8):
    adb("shell", "input", "keyevent", str(code))
    time.sleep(wait)


def go(text, wait=2.0):
    tap(FIELD)
    type_text(text)
    key(66, wait)


def launch(block=""):
    adb("shell", "am", "force-stop", PACKAGE)
    args = ["shell", "am", "start", "-W", "-n", PACKAGE + "/" + ACTIVITY]
    if block:
        args += ["--es", "block", block]
    adb(*args)
    time.sleep(3)


def history_file():
    return adb("shell", "cat", "/data/data/%s/files/profile/history.jsonl" % PACKAGE)


def visited(url):
    return any(json.loads(line)["url"] == url for line in history_file().splitlines() if line.strip())


def history_title(url):
    title = None
    for line in history_file().splitlines():
        if line.strip() and json.loads(line)["url"] == url:
            title = json.loads(line)["title"]
    return title


def children():
    """The app's child processes as (pid, process name), from what the system knows it bound."""
    out = []
    pending = False
    for line in adb("shell", "dumpsys", "activity", "services", PACKAGE).splitlines():
        if "ServiceRecord{" in line:
            pending = (PACKAGE + "/" in line) and "ProcessService" in line
        elif pending and "app=ProcessRecord{" in line:
            pid, name = line.split("ProcessRecord{")[1].split()[1].split(":", 1)
            out.append((pid, name.split("/")[0]))
            pending = False
    return out


def check(name, ok, detail=""):
    results.append((name, bool(ok), detail))
    print("%s  %s  %s" % ("ok  " if ok else "FAIL", name, detail), flush=True)


def main():
    os.makedirs(OUT, exist_ok=True)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", PORT), Server)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    adb("root")
    time.sleep(2)
    adb("wait-for-device")
    adb("reverse", "tcp:%d" % PORT, "tcp:%d" % PORT)
    adb("install", "-r", APK)
    adb("shell", "pm", "clear", PACKAGE)

    launch(block="blocked.test")
    shot("01-opens-empty")

    # Type a URL.
    go(BASE + "/a")
    shot("02-page-a")
    check("typing a URL loads it", "/a" in served)
    check("the visit reached Go's history file", visited("http://%s/a" % BASE))

    # A tap on the page, below the bar, belongs to the page.
    tap((540, 300), 2.0)
    shot("03-link-tapped")
    check("a tap on a link in the page reaches the page", "/b" in served)

    # Back, forward, reload through the shell's own buttons.
    served.clear()
    tap(BACK, 2.0)
    shot("04-back")
    check("back returns to the page before", "/a" in served, str(served))
    served.clear()
    tap(FORWARD, 2.0)
    shot("05-forward")
    check("forward returns to the page after", "/b" in served, str(served))
    served.clear()
    tap(RELOAD, 2.0)
    shot("06-reload")
    check("reload asks the server again", "/b" in served, str(served))
    check("an emoji in a title reaches Go whole", history_title("http://%s/b" % BASE) == "Page B \U0001F642",
          repr(history_title("http://%s/b" % BASE)))

    go(BASE + "/c")
    # Suggestions come from history, shown over the page.
    tap(FIELD)
    type_text("page")
    shot("07-suggestions")
    served.clear()
    tap(FIRST_SUGGESTION, 2.0)
    shot("08-suggestion-tapped")
    check("a tap on a suggestion loads it", len(served) == 1, str(served))

    # The phone's back key follows the same path as the shell's button.
    served.clear()
    key(4, 2.0)
    shot("09-back-key")
    check("the phone's back key goes back", len(served) == 1, str(served))

    # Touch routing: a tap and a scroll in the page area go to the page, not the shell above it.
    go(BASE + "/tap")
    served.clear()
    tap(PAGE_TOP, 1.5)
    check("a tap just below the bar reaches the page", "/tapped" in served, str(served))
    go(BASE + "/tall")
    served.clear()
    adb("shell", "input", "swipe", "540", "1800", "540", "600", "300")
    time.sleep(1.5)
    shot("10-scrolled")
    check("a swipe scrolls the page under the shell", any(s.startswith("/scrolled") for s in served), str(served[:3]))

    # A gate says no: for a typed URL and for a redirect.
    go("blocked.test")
    shot("11-blocked")
    check("a blocked host never joins history", not visited("https://blocked.test/"))
    go(BASE + "/to-blocked")
    shot("12-redirect-blocked")
    check("a redirect to a blocked host is stopped", not visited("http://blocked.test/"))

    # A POST is not offered to shouldOverrideUrlLoading. This one documents the gap.
    go(BASE + "/form")
    tap(PAGE_TOP, 3.0)
    shot("13-post-to-blocked-host")

    # A load that fails stays out of history.
    go("https://nothing.invalid/", 3.0)
    shot("14-failed")
    check("a failed load never joins history", not visited("https://nothing.invalid/"))
    go("https://expired.badssl.com/", 4.0)
    shot("14b-bad-certificate")

    # History survives the process.
    adb("shell", "am", "force-stop", PACKAGE)
    time.sleep(1)
    launch()
    tap(FIELD)
    type_text("page")
    shot("15-suggestions-after-restart")
    check("history file still holds Page A after a restart", visited("http://%s/a" % BASE))
    launch()

    # Where Go is: the app's process, and none of its children.
    main_pid = None
    for line in adb("shell", "ps", "-A", "-o", "PID,NAME").splitlines():
        parts = line.split()
        if len(parts) == 2 and parts[1] == PACKAGE:
            main_pid = parts[0]
    renderers = children()
    # Libraries are mapped straight from the APK, so the mapping's name is the APK's.
    ours = PACKAGE + "-"
    maps = adb("shell", "cat", "/proc/%s/maps" % main_pid)
    check("our code is mapped in the app's process", any(ours in l and "r-xp" in l for l in maps.splitlines()),
          "pid " + str(main_pid))
    for pid, name in renderers:
        rmaps = adb("shell", "cat", "/proc/%s/maps" % pid)
        check("none of our code in " + name.split(":")[-1], ours not in rmaps and len(rmaps) > 0, "pid " + pid)
    check("exactly one renderer holds shell and page", len(renderers) == 1, str(renderers))

    # Renderer crash: the app lives, the shell comes back from a snapshot, reload recovers.
    go(BASE + "/a")
    for pid, _ in children():
        adb("shell", "kill", "-9", pid)
    time.sleep(3)
    shot("16-after-renderer-kill")
    alive = PACKAGE in adb("shell", "ps", "-A", "-o", "NAME")
    check("the app survives a renderer crash", alive)
    served.clear()
    tap(RELOAD, 3.0)
    shot("17-reloaded-after-crash")
    check("reload recovers after a crash", "/a" in served, str(served))

    # One renderer holds both views, so a page that spins freezes the shell too.
    go(BASE + "/busy")
    tap(PAGE_TOP, 1.0)
    tap(FIELD, 1.0)
    type_text("xyz")
    time.sleep(2)
    shot("18-page-spinning-shell-frozen")
    adb("shell", "am", "force-stop", PACKAGE)

    # The real web, over the office's 443-only egress.
    launch()
    go("example.com", 4.0)
    shot("19-real-web-example")
    check("a real https page joins history", visited("https://example.com/"))
    go("en.wikipedia.org", 6.0)
    shot("20-real-web-wikipedia")
    adb("shell", "input", "swipe", "540", "1800", "540", "600", "300")
    time.sleep(1.5)
    shot("21-real-web-scrolled")
    tap(FIELD)
    type_text("wiki")
    shot("22-real-web-suggestions")
    adb("shell", "am", "force-stop", PACKAGE)

    failed = [r for r in results if not r[1]]
    print("\n%d checks, %d failed" % (len(results), len(failed)))
    with open(OUT + "/results.json", "w") as f:
        json.dump(results, f, indent=1)


if __name__ == "__main__":
    main()
