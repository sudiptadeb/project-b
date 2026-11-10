#!/usr/bin/env python3
"""Drives the real app with Google's ChromeDriver, as a test suite drives Chrome.

Plain WebDriver over HTTP, no Selenium. ChromeDriver starts the app with its own
--user-data-dir and --remote-debugging-port=0, so this also proves the app looks like
Chrome to it. What Go thinks the tabs are is read from the shell through --pb-driver,
which ChromeDriver passes along. Then Chrome's defaults, which come with content: off
without the switch, 127.0.0.1 only, foreign origins refused. Run it from the repo root:

    python3 tools/drive_chromedriver.py

PB_BIN picks another binary, CHROMEDRIVER another ChromeDriver. Results go to
build/cdp/. Every profile is a fresh temporary one.
"""

import base64
import http.server
import json
import os
import re
import shutil
import socket
import struct
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
BIN = os.environ.get("PB_BIN") or os.path.join(ROOT, "chromium/src/out/pb/project_b")
CHROMEDRIVER = os.environ.get("CHROMEDRIVER") or os.path.join(
    ROOT, ".tools/chromedriver/chromedriver-mac-arm64/chromedriver")
OUT = os.path.join(ROOT, "build/cdp")
VERSION = "155.0.8059.41"

# What Go thinks the tabs are, as the shell shows them.
ROWS = "document.querySelectorAll('[role=tab]').length"
ACTIVE_ROW_TITLE = "document.querySelector('[role=tab][aria-selected=true] .title').textContent"

PAGES = {
    "/": b"""<!doctype html><title>Fixture</title>
<h1>Fixture</h1><button id=go onclick="out.textContent='clicked'">Go</button><p id=out>waiting</p>""",
    "/two": b"<!doctype html><title>Second</title><h1>Second</h1>",
}

failures = 0


def check(name, ok, detail=""):
    global failures
    print(("ok   " if ok else "FAIL ") + name + ("" if ok or detail == "" else " -> " + str(detail)), flush=True)
    if not ok:
        failures += 1


class Fixture(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = PAGES.get(self.path)
        self.send_response(200 if body else 404)
        self.send_header("Content-Type", "text/html")
        self.end_headers()
        self.wfile.write(body or b"")

    def log_message(self, *args):
        pass


def free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def http_json(method, url, body=None, headers=None, timeout=60):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(url, data=data, method=method, headers={"Content-Type": "application/json", **(headers or {})})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, json.loads(r.read() or b"null")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read() or b"null")


class Lines:
    """Reads a process's output into a log, and keeps what --pb-driver replies."""

    def __init__(self, proc, log_path):
        self.proc = proc
        self.text = []
        self.replies = {}
        self.cond = threading.Condition()
        self.log = open(log_path, "a")
        threading.Thread(target=self._read, daemon=True).start()

    def _read(self):
        for line in self.proc.stdout:
            self.log.write(line)
            self.log.flush()
            with self.cond:
                self.text.append(line)
                m = re.match(r"PBDRIVER (\S+) (.*)$", line.rstrip("\n"))
                if m:
                    self.replies[m.group(1)] = m.group(2)
                self.cond.notify_all()

    def wait(self, test, timeout=30):
        deadline = time.time() + timeout
        with self.cond:
            while not test():
                left = deadline - time.time()
                if left <= 0:
                    return False
                self.cond.wait(left)
            return True

    def has(self, s):
        with self.cond:
            return any(s in line for line in self.text)


class Driver:
    """The app's development driver: evaluates in the shell."""

    def __init__(self, lines, fifo):
        self.lines, self.fifo, self.next, self.writer = lines, fifo, 0, None

    def start(self, timeout=60):
        if not self.lines.wait(lambda: "start" in self.lines.replies, timeout):
            raise TimeoutError("the app's driver never started")
        self.writer = open(self.fifo, "w", buffering=1)

    def cmd(self, *parts, timeout=20):
        self.next += 1
        key = str(self.next)
        self.writer.write(" ".join([key, *map(str, parts)]) + "\n")
        if not self.lines.wait(lambda: key in self.lines.replies, timeout):
            raise TimeoutError(parts)
        return self.lines.replies.pop(key)

    def js(self, expr):
        r = self.cmd("eval", expr)
        return json.loads(r) if r != "" else None

    def wait(self, expr, want, timeout=15):
        deadline = time.time() + timeout
        got = None
        while time.time() < deadline:
            got = self.js(expr)
            if got == want:
                return got
            time.sleep(0.1)
        return got


def make_fifo():
    fifo = os.path.join(tempfile.mkdtemp(prefix="pb-cdp-driver-"), "fifo")
    os.mkfifo(fifo)
    return fifo


def listening_ports(pid):
    r = subprocess.run(["lsof", "-nP", "-a", "-p", str(pid), "-iTCP", "-sTCP:LISTEN"], capture_output=True, text=True)
    return re.findall(r":(\d+) \(LISTEN\)", r.stdout)


def descendants(pid):
    ps = subprocess.run(["ps", "-axo", "pid=,ppid="], capture_output=True, text=True).stdout
    pairs = [tuple(map(int, line.split())) for line in ps.splitlines()]
    seen, changed = {pid}, True
    while changed:
        changed = False
        for p, pp in pairs:
            if pp in seen and p not in seen:
                seen.add(p)
                changed = True
    return seen


def alive(pids):
    out = []
    for p in pids:
        try:
            os.kill(p, 0)
            out.append(p)
        except OSError:
            pass
    return out


# A WebSocket client small enough to read: enough for a few DevTools commands.
def ws_open(url, origin=None):
    u = urllib.parse.urlparse(url)
    s = socket.create_connection((u.hostname, u.port), timeout=10)
    key = base64.b64encode(os.urandom(16)).decode()
    req = (f"GET {u.path} HTTP/1.1\r\nHost: {u.hostname}:{u.port}\r\nUpgrade: websocket\r\n"
           f"Connection: Upgrade\r\nSec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\n")
    if origin:
        req += f"Origin: {origin}\r\n"
    s.sendall((req + "\r\n").encode())
    head = b""
    while b"\r\n\r\n" not in head:
        chunk = s.recv(1)
        if not chunk:
            break
        head += chunk
    return s, int(head.split(b" ")[1])


def ws_send(s, text):
    data = text.encode()
    mask = os.urandom(4)
    n = len(data)
    head = bytes([0x81]) + (bytes([0x80 | n]) if n < 126 else bytes([0x80 | 126]) + struct.pack(">H", n) if n < 65536 else bytes([0x80 | 127]) + struct.pack(">Q", n))
    s.sendall(head + mask + bytes(b ^ mask[i % 4] for i, b in enumerate(data)))


def ws_recv(s):
    def read(n):
        buf = b""
        while len(buf) < n:
            chunk = s.recv(n - len(buf))
            if not chunk:
                raise ConnectionError("closed")
            buf += chunk
        return buf
    b0, b1 = read(2)
    n = b1 & 0x7F
    if n == 126:
        n = struct.unpack(">H", read(2))[0]
    elif n == 127:
        n = struct.unpack(">Q", read(8))[0]
    return read(n).decode()


def cdp(s, msg_id, method, params=None):
    ws_send(s, json.dumps({"id": msg_id, "method": method, "params": params or {}}))
    while True:
        m = json.loads(ws_recv(s))
        if m.get("id") == msg_id:
            return m


def run_app(args, wait_for=None, log="app.log"):
    """Starts the app directly, waits until its driver answers, and returns it."""
    fifo = make_fifo()
    proc = subprocess.Popen([BIN, f"--pb-driver={fifo}", *args], stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                            text=True, bufsize=1)
    lines = Lines(proc, os.path.join(OUT, log))
    driver = Driver(lines, fifo)
    driver.start()
    driver.wait("!!document.querySelector('input')", True, 30)
    if wait_for:
        lines.wait(lambda: lines.has(wait_for), 10)
    return proc, lines, driver


def stop_app(proc, driver):
    try:
        driver.cmd("close")
    except Exception:
        pass
    try:
        proc.wait(timeout=30)
    except subprocess.TimeoutExpired:
        proc.kill()


def defaults(scratch):
    # Off unless asked: no listening socket, no port file.
    profile = os.path.join(scratch, "plain")
    proc, lines, driver = run_app([f"--user-data-dir={profile}"])
    time.sleep(1)
    check("without the switch nothing listens", listening_ports(proc.pid) == [], listening_ports(proc.pid))
    check("without the switch there is no DevToolsActivePort", not os.path.exists(os.path.join(profile, "DevToolsActivePort")))
    stop_app(proc, driver)

    # With the switch: Chrome's line on stderr, the port file, this machine only.
    other = os.path.join(scratch, "debug")
    proc, lines, driver = run_app(["--remote-debugging-port=0", f"--user-data-dir={other}"], wait_for="DevTools listening on")
    check("with the switch it prints Chrome's line", lines.has("DevTools listening on ws://127.0.0.1:"))
    port_file = os.path.join(other, "DevToolsActivePort")
    ok = os.path.exists(port_file)
    check("and writes DevToolsActivePort into the profile", ok)
    if ok:
        port = open(port_file).read().split("\n")[0]
        r = subprocess.run(["lsof", "-nP", "-a", "-p", str(proc.pid), "-iTCP", "-sTCP:LISTEN"], capture_output=True, text=True).stdout
        check("listening on 127.0.0.1 only", f"127.0.0.1:{port} (LISTEN)" in r and "*:" + port not in r, r)
    stop_app(proc, driver)


def session(fixture, extra_args=(), full=True):
    cd_port = free_port()
    fifo = make_fifo()
    cd = subprocess.Popen([CHROMEDRIVER, f"--port={cd_port}"], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, bufsize=1)
    lines = Lines(cd, os.path.join(OUT, "chromedriver.log"))
    lines.wait(lambda: lines.has("started successfully"), 15)
    wd = f"http://127.0.0.1:{cd_port}"
    mode = "pipe" if "--remote-debugging-pipe" in extra_args else "port"
    caps = {"capabilities": {"alwaysMatch": {"browserName": "chrome", "goog:chromeOptions": {
        "binary": BIN, "args": [f"--pb-driver={fifo}", *extra_args]}}}}
    status, r = http_json("POST", wd + "/session", caps, timeout=90)
    value = (r or {}).get("value", {})
    check(f"[{mode}] session created", status == 200 and "sessionId" in value, value.get("message", status))
    if status != 200:
        cd.kill()
        return
    sid, caps = value["sessionId"], value["capabilities"]
    base = f"{wd}/session/{sid}"
    check(f"[{mode}] browserName and browserVersion", caps.get("browserName") == "chrome" and caps.get("browserVersion") == VERSION,
          (caps.get("browserName"), caps.get("browserVersion")))
    driver = Driver(lines, fifo)
    driver.start()

    def wd_call(method, path, body=None):
        return http_json(method, base + path, body)[1]["value"]

    wd_call("POST", "/url", {"url": fixture + "/"})
    check(f"[{mode}] navigate, then the title", wd_call("GET", "/title") == "Fixture")
    if not full:
        http_json("DELETE", base)
        cd.kill()
        return

    user_data_dir = caps.get("chrome", {}).get("userDataDir", "")
    check("ChromeDriver's own profile has DevToolsActivePort", os.path.exists(os.path.join(user_data_dir, "DevToolsActivePort")), user_data_dir)
    debugger = caps.get("goog:chromeOptions", {}).get("debuggerAddress", "")
    devtools = "http://" + debugger.replace("localhost", "127.0.0.1")
    app_pid = None
    port = debugger.rsplit(":", 1)[-1]
    found = subprocess.run(["lsof", "-nP", "-t", f"-iTCP:{port}", "-sTCP:LISTEN"], capture_output=True, text=True).stdout.split()
    if found:
        app_pid = int(found[0])

    button = wd_call("POST", "/element", {"using": "css selector", "value": "#go"})
    element = next(iter(button.values()), None) if isinstance(button, dict) else None
    check("find element", element is not None, button)
    wd_call("POST", f"/element/{element}/click", {})
    out = wd_call("POST", "/element", {"using": "css selector", "value": "#out"})
    check("click changes the page", wd_call("GET", f"/element/{next(iter(out.values()))}/text") == "clicked")
    check("execute script", wd_call("POST", "/execute/sync", {"script": "return document.title + ':' + arguments[0]", "args": [7]}) == "Fixture:7")
    png = base64.b64decode(wd_call("GET", "/screenshot"))
    with open(os.path.join(OUT, "screenshot.png"), "wb") as f:
        f.write(png)
    check("screenshot is a PNG", png[:8] == b"\x89PNG\r\n\x1a\n", len(png))

    rect = wd_call("GET", "/window/rect")
    check("get window rect", all(k in rect for k in ("x", "y", "width", "height")), rect)
    want = {"x": 60, "y": 80, "width": 1000, "height": 640}
    got = wd_call("POST", "/window/rect", want)
    check("set window rect", all(got.get(k) == v for k, v in want.items()), got)

    first = wd_call("GET", "/window")
    check("Go starts with one tab", driver.js(ROWS) == 1)
    new = wd_call("POST", "/window/new", {"type": "window"})
    check("new window gives a new handle", isinstance(new, dict) and new.get("handle") not in (None, first), new)
    check("new window is a new tab in Go", driver.wait(ROWS, 2) == 2)
    handles = wd_call("GET", "/window/handles")
    check("two window handles", len(handles) == 2, handles)
    wd_call("POST", "/window", {"handle": new["handle"]})
    wd_call("POST", "/url", {"url": fixture + "/two"})
    check("the new tab navigates", wd_call("GET", "/title") == "Second")
    check("Go shows the new tab", driver.wait(ACTIVE_ROW_TITLE, "Second") == "Second")
    wd_call("POST", "/window", {"handle": first})
    check("switch to the first window", wd_call("GET", "/title") == "Fixture")
    check("Go shows the first tab again", driver.wait(ACTIVE_ROW_TITLE, "Fixture") == "Fixture")
    wd_call("POST", "/window", {"handle": new["handle"]})
    left = wd_call("DELETE", "/window")
    check("close window leaves one handle", left == [first], left)
    check("Go closed the tab", driver.wait(ROWS, 1) == 1)
    wd_call("POST", "/window", {"handle": first})

    # The shell, seen from a DevTools client of our own.
    status, listed = http_json("GET", devtools + "/json/list")
    check("/json/list has the page", any(t.get("url", "").startswith(fixture) for t in listed), listed)
    shell = [t for t in listed if t.get("url", "").startswith("chrome://shell")]
    print("shell in /json/list:", json.dumps([t["type"] for t in shell]), flush=True)
    check("the shell is no page target", not any(t["type"] == "page" for t in shell), shell)
    status, tabs = http_json("GET", devtools + "/json/list?for_tab")
    check("the shell is no tab target", not any(t["type"] == "tab" and t.get("url", "").startswith("chrome://shell") for t in tabs), tabs)
    status, version = http_json("GET", devtools + "/json/version")
    check("/json/version says Chrome", version.get("Browser") == "Chrome/" + VERSION, version.get("Browser"))
    browser_ws = version["webSocketDebuggerUrl"].replace("localhost", "127.0.0.1")

    # DevTools gives no new way into the shell's bridge: a navigation to chrome://shell/,
    # and a new target opened on it, both end without chrome.send.
    has_bridge = "return [location.href, typeof (window.chrome && chrome.send)]"
    wd_call("POST", "/url", {"url": "chrome://shell/"})
    landed = wd_call("POST", "/execute/sync", {"script": has_bridge, "args": []})
    check("navigating to chrome://shell/ gets no bridge", landed[0] != "chrome://shell/" and landed[1] == "undefined", landed)
    s, code = ws_open(browser_ws)
    r = cdp(s, 1, "Target.createTarget", {"url": "chrome://shell/"})
    s.close()
    handles = wd_call("GET", "/window/handles")
    stray = [h for h in handles if h != first]
    landed = None
    if stray:
        wd_call("POST", "/window", {"handle": stray[0]})
        time.sleep(1)
        landed = wd_call("POST", "/execute/sync", {"script": has_bridge, "args": []})
        wd_call("DELETE", "/window")
        wd_call("POST", "/window", {"handle": first})
    check("a target opened on chrome://shell/ gets no bridge", landed is None or (landed[0] != "chrome://shell/" or landed[1] == "undefined"), (r, landed))
    wd_call("POST", "/url", {"url": fixture + "/"})
    s, code = ws_open(browser_ws)
    check("a client with no Origin connects", code == 101, code)
    r = cdp(s, 1, "Browser.getVersion")
    check("Browser.getVersion says Chrome", r.get("result", {}).get("product") == "Chrome/" + VERSION, r)
    s.close()
    s, code = ws_open(browser_ws, origin="https://evil.test")
    check("a WebSocket from a foreign Origin is refused", code == 403, code)
    s.close()

    pids = descendants(app_pid) if app_pid else set()
    status, _ = http_json("DELETE", base)
    check("quit", status == 200, status)
    deadline = time.time() + 15
    while alive(pids) and time.time() < deadline:
        time.sleep(0.2)
    check("quit leaves no process", pids != set() and alive(pids) == [], (len(pids), alive(pids)))
    cd.kill()


def main():
    if os.path.exists(OUT):
        shutil.rmtree(OUT)
    os.makedirs(OUT)
    scratch = tempfile.mkdtemp(prefix="pb-cdp-")
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Fixture)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    fixture = f"http://127.0.0.1:{server.server_port}"

    session(fixture)
    session(fixture, ["--remote-debugging-pipe"], full=False)
    defaults(scratch)

    shutil.rmtree(scratch, ignore_errors=True)
    print("\nall passed" if failures == 0 else f"\n{failures} FAILED", flush=True)
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
