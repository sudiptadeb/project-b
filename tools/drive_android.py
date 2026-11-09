#!/usr/bin/env python3
"""Drives the real Android app on an emulator: real touches, real keys, real screenshots.

A small local web server is the oracle for what the page received, and the app's own history
file for what the managers kept. Results and screenshots go to build/android-drive.

    ADB_SERIAL=emulator-5600 python3 tools/drive_android.py            # the WebView build
    ENGINE=content python3 tools/drive_android.py                      # the content build
"""
import http.server
import json
import os
import subprocess
import threading
import time
import zipfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SDK = os.path.expanduser("~/Library/Android/sdk")
ADB = [SDK + "/platform-tools/adb", "-s", os.environ.get("ADB_SERIAL", "emulator-5600")]
ENGINE = os.environ.get("ENGINE", "webview")
APK = ROOT + "/build/android/project-b-%s.apk" % ENGINE
PACKAGE = {"webview": "dev.projectb.webview", "content": "dev.projectb.browser"}[ENGINE]
ACTIVITY = ".BrowserActivity"
PROFILE = {"webview": "files/profile", "content": "app_project_b/profile"}[ENGINE]
OUT = os.environ.get("OUT", ROOT + "/build/android-drive-" + ENGINE)
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
    "/input": "<title>Input</title><form action=/typed><input name=q autofocus "
              "style='width:100%;height:200px;font-size:40px'></form>",
    "/typed": "<title>Typed</title>typed",
    "/words": "<title>Words</title><p style='font-size:40px'>Some words to hold a finger on, " + "words " * 200,
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
    # Android takes the app zygote down a few seconds after a force stop. A start inside that window
    # reused the dying zygote once and hung for good, so wait it out. Killing it ourselves is worse:
    # the system never makes another, and no renderer starts.
    for _ in range(30):
        if PACKAGE + "_zygote" not in adb("shell", "ps", "-A", "-o", "NAME"):
            break
        time.sleep(0.5)
    args = ["shell", "am", "start", "-W", "-n", PACKAGE + "/" + ACTIVITY]
    if block:
        args += ["--es", "block", block]
    adb(*args)
    # Content starts in the background, so the window shows before the shell does. The shell is
    # up once its renderer is.
    for _ in range(30):
        if any("sandboxed" in name for _, name in children()):
            break
        time.sleep(0.5)
    time.sleep(2)


def history_file():
    return adb("shell", "cat", "/data/data/%s/%s/history.jsonl" % (PACKAGE, PROFILE))


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


def go_library_mappings(pid):
    """How many executable mappings of libcore.so a process has. Android either unpacks a library
    to a file or maps it straight from the APK, where it shows as a stretch of the APK."""
    start, end = go_library_span()
    count = 0
    for line in adb("shell", "cat", "/proc/%s/maps" % pid).splitlines():
        parts = line.split()
        if len(parts) < 6 or "r-xp" not in parts[1] or PACKAGE + "-" not in parts[5]:
            continue
        if parts[5].endswith("/libcore.so"):
            count += 1
        elif parts[5].endswith("base.apk") and start <= int(parts[2], 16) < end:
            count += 1
    return count


def go_library_span():
    """Where lib/arm64-v8a/libcore.so sits in the APK, from the zip's own directory."""
    with zipfile.ZipFile(APK) as z:
        info = z.getinfo("lib/arm64-v8a/libcore.so")
    # The stored data begins just after the local header, a few hundred bytes at most.
    return info.header_offset, info.header_offset + 4096 + info.file_size


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

    # Typing into a page goes through the keyboard to the page, not the shell.
    go(BASE + "/input")
    served.clear()
    tap(PAGE_TOP, 1.0)
    type_text("hello")
    key(66, 2.0)
    shot("10b-typed-into-page")
    # Android 16's keyboard capitalises the first letter, as it would for a person.
    check("the keyboard types into a page", any("q=hello" in s.lower() for s in served), str(served))

    # Holding a finger on text starts a selection, and the app lives through it.
    go(BASE + "/words")
    adb("shell", "input", "swipe", "400", "600", "400", "600", "900")
    time.sleep(1.5)
    shot("10c-text-selected")
    check("selecting text in a page doesn't kill the app", PACKAGE in adb("shell", "ps", "-A", "-o", "NAME"))
    key(4)

    # The phone turns: the page follows, and a tap in the new layout still lands.
    go(BASE + "/tap")
    adb("shell", "settings", "put", "system", "accelerometer_rotation", "0")
    adb("shell", "settings", "put", "system", "user_rotation", "1")
    time.sleep(3)
    shot("10d-landscape")
    served.clear()
    tap((1200, 300), 1.5)
    check("a tap lands in the page in landscape", "/tapped" in served, str(served))
    adb("shell", "settings", "put", "system", "user_rotation", "0")
    time.sleep(3)

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

    # Where Go is: the app's process, and none of its children. The page needs a renderer first.
    go(BASE + "/a")
    main_pid = None
    for line in adb("shell", "ps", "-A", "-o", "PID,NAME").splitlines():
        parts = line.split()
        if len(parts) == 2 and parts[1] == PACKAGE:
            main_pid = parts[0]
    kids = children()
    go_size = go_library_mappings(main_pid)
    check("Go's library is mapped in the main process", go_size > 0, "%d executable mapping(s)" % go_size)
    for line in adb("shell", "ps", "-A", "-o", "PID,NAME").splitlines():
        parts = line.split()
        if len(parts) == 2 and parts[1] == PACKAGE + "_zygote":
            kids.append((parts[0], "app:zygote"))
    for pid, name in kids:
        rmaps = adb("shell", "cat", "/proc/%s/maps" % pid)
        check("Go's library is not in " + name.split(":")[1], len(rmaps) > 0 and go_library_mappings(pid) == 0, "pid " + pid)
    renderers = sorted((int(pid) for pid, name in kids if "sandboxed" in name))
    if ENGINE == "webview":
        check("exactly one renderer holds shell and page", len(renderers) == 1, str(kids))
    else:
        check("the shell and the page have separate renderers", len(renderers) >= 2, str(kids))

    # The page's renderer dies, which is the last one to start.
    adb("shell", "kill", "-9", str(renderers[-1]))
    time.sleep(3)
    shot("16-after-page-renderer-kill")
    check("the app survives a renderer crash", PACKAGE in adb("shell", "ps", "-A", "-o", "NAME"))
    served.clear()
    tap(RELOAD, 3.0)
    shot("17-reloaded-after-crash")
    check("reload recovers after a crash", "/a" in served, str(served))

    if ENGINE == "content":
        # The shell's renderer dies alone: it comes back from a snapshot, and the page stays.
        adb("shell", "kill", "-9", str(renderers[0]))
        time.sleep(4)
        shot("17b-after-shell-renderer-kill")
        served.clear()
        tap(RELOAD, 3.0)
        check("the shell works again after its renderer died", "/a" in served, str(served))
        # The GPU process dies.
        for pid, name in children():
            if "privileged" in name:
                adb("shell", "kill", "-9", pid)
        time.sleep(5)
        shot("17c-after-gpu-kill")
        served.clear()
        tap(RELOAD, 3.0)
        shot("17d-reload-after-gpu-kill")
        check("the app works after the GPU process died", "/a" in served, str(served))

    # On WebView one renderer holds both views, so a page that spins freezes the shell too. On
    # content they're separate, so the shell keeps working.
    launch()
    go(BASE + "/busy")
    tap(PAGE_TOP, 1.0)
    tap(FIELD, 1.0)
    type_text("xyz")
    time.sleep(2)
    shot("18-page-spinning")
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
