#!/usr/bin/env python3
"""The basics: one check per area of the whole-browser checklist (Appendix A of the test-suite
research), on the real app, with no display needed. Each check asks for what a browser should
do, so a check fails until its area is built: the result is a map of what is missing.

Each check starts its own app with a fresh profile and pages served on this machine. A check
is ok, FAIL, or NOT WRITTEN with the reason it can't be written yet. The last line counts them.
PB_BIN picks another binary. Exit 1 if any failed.
"""
import glob
import json
import os
import re
import signal
import socket
import ssl
import subprocess
import sys
import tempfile
import threading
import time
import traceback
import http.server
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import drive_first_app as d  # noqa: E402
from drive_chromedriver import cdp, listening_ports, ws_open, ws_send  # noqa: E402

ROOT = d.ROOT
OUT = os.path.join(ROOT, "build/basics")
d.OUT = OUT
INPUT, VALUE, ROWS, TITLE, READY = d.INPUT, d.VALUE, d.ROWS, d.ACTIVE_ROW_TITLE, d.TAB_READY

results = []  # (area, name, "ok" | "FAIL" | "NOT WRITTEN", detail)
seen = []  # (Host header, path) of every request the pages' server answered


PAGE = {
    "/a": "<title>A</title><p>A",
    "/b": "<title>B</title><p>B",
    "/link": "<title>Link</title><a href=/b style='font-size:40px'>to B</a>",
    "/unload": "<title>Unload</title><body style='height:100vh' onbeforeunload='return \"stay\"'>leave?",
    "/popup": "<title>Popup</title><script>onload=()=>window.open('/b')</script>",
    "/dialog": "<title>ready</title><button id=b style='font-size:40px' onclick=\"document.title='waiting';alert('x');document.title='done'\">go</button>",
    "/file": "<title>ready</title><input id=f type=file style='font-size:40px' oncancel=\"document.title='cancelled'\" onclick=\"document.title='waiting'\">",
    "/perm": "<title>ready</title><button id=b style='font-size:40px' onclick=\"document.title='pending';Notification.requestPermission().then(r=>document.title=r)\">ask</button>",
    "/cam": "<title>ready</title><button id=b style='font-size:40px' onclick=\"document.title='pending';navigator.mediaDevices.getUserMedia({video:true}).then(()=>document.title='granted',e=>document.title=e.name)\">ask</button>",
    "/fs": "<title>ready</title><button id=b style='font-size:40px' onclick=\"document.title='pending';document.documentElement.requestFullscreen().then(()=>document.title='full',e=>document.title=e.name)\">full</button>",
    "/keys": "<title>Keys</title><body style='height:100vh' onkeydown='event.preventDefault()'>keys",
    "/steal": "<title>Steal</title><input id=i><script>setTimeout(()=>i.focus(),1500)</script>",
    "/download": "<title>Download</title><a id=a style='font-size:40px' href=/dl>file</a>",
    "/cookie": "<title>Cookie</title><script>document.cookie='basics=kept; Max-Age=3600'</script>",
    "/pip": """<title>ready</title><canvas id=c width=64 height=64></canvas><video id=v muted></video>
<button id=b style='font-size:40px' onclick="go()">pip</button><script>
async function go(){document.title='pending';const x=c.getContext('2d');x.fillRect(0,0,64,64);
v.srcObject=c.captureStream();await v.play();
v.requestPictureInPicture().then(()=>document.title='pip',e=>document.title=e.name)}</script>""",
}


class Pages(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        seen.append((self.headers.get("Host", ""), self.path))
        path = self.path.split("?")[0]
        if self.path.startswith("http://"):  # Asked as a proxy.
            path = "/a"
        if path == "/slow":
            time.sleep(4)
            return self.page("<title>Slow</title>slow")
        if path == "/auth":
            return self.page("<title>Denied</title>no", 401, [("WWW-Authenticate", 'Basic realm="basics"')])
        if path == "/dl":
            return self.page("a file", 200, [("Content-Disposition", f"attachment; filename={DL_NAME}")], "text/plain")
        if path == "/blocked-image":
            return self.page(f"<title>Image</title><img src='http://{d.BLOCKED}:{PORT}/pixel'>")
        self.page(PAGE.get(path, "<title>Pages</title>"))

    def page(self, body, status=200, headers=(), kind="text/html"):
        b = (("<!doctype html>" if kind == "text/html" else "") + body).encode()
        self.send_response(status)
        for k, v in headers:
            self.send_header(k, v)
        self.send_header("Content-Type", kind)
        self.send_header("Content-Length", str(len(b)))
        self.end_headers()
        self.wfile.write(b)

    def log_message(self, *args):
        pass


def serve(server):
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server.server_port


DL_NAME = f"pb-basics-{os.getpid()}.txt"
PORT = serve(http.server.ThreadingHTTPServer(("127.0.0.1", 0), Pages))
P = f"http://127.0.0.1:{PORT}"

# The same pages over https with a self-signed certificate, for the certificate error.
_certs = tempfile.mkdtemp(prefix="pb-basics-cert-")
subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "2", "-subj", "/CN=127.0.0.1",
                "-keyout", f"{_certs}/key.pem", "-out", f"{_certs}/cert.pem"], capture_output=True, check=True)
_https = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Pages)
_ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
_ctx.load_cert_chain(f"{_certs}/cert.pem", f"{_certs}/key.pem")
_https.socket = _ctx.wrap_socket(_https.socket, server_side=True)
HTTPS = f"https://127.0.0.1:{serve(_https)}"


def closed_port():
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def start(extra=(), profile=None):
    return d.App(profile or tempfile.mkdtemp(prefix="pb-basics-"), extra)


def ready(app):
    time.sleep(0.5)


def go(app, url, wait=True):
    """Types a URL into the address bar and presses enter; waits for the load unless told not to."""
    d.focus_bar(app)
    app.cmd("type", url)
    app.wait(f"{VALUE}==={json.dumps(url)}")
    app.cmd("key", "enter")
    if wait:
        time.sleep(0.3)
        app.wait(READY, 30)
        time.sleep(0.3)


def new_tab(app):
    n = app.js(ROWS)
    app.cmd("key", "cmd+t")
    app.wait(f"{ROWS}=={n + 1}")


def title(app):
    return app.js(TITLE)


def page_text(app):
    return app.js("document.body?document.body.innerText:''", page=True)


def devtools(profile):
    """The DevTools port the app wrote into its profile."""
    path = os.path.join(profile, "DevToolsActivePort")
    deadline = time.time() + 10
    while not os.path.exists(path):
        if time.time() > deadline:
            raise TimeoutError("no DevToolsActivePort")
        time.sleep(0.1)
    return int(open(path).read().split()[0])


def page_socket(port, url_part):
    targets = json.load(urllib.request.urlopen(f"http://127.0.0.1:{port}/json/list"))
    target = next(t for t in targets if t["type"] == "page" and url_part in t["url"])
    return ws_open(target["webSocketDebuggerUrl"])[0]


def cdp_click(s, x, y, button="left", modifiers=0):
    for i, kind in enumerate(("mousePressed", "mouseReleased")):
        cdp(s, 10 + i, "Input.dispatchMouseEvent", {"type": kind, "x": x, "y": y, "button": button, "clickCount": 1, "modifiers": modifiers})


def middle(app, selector):
    return app.js(f"(()=>{{const r=document.querySelector({json.dumps(selector)}).getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]}})()", page=True)


# One function per area: it returns (ok, detail), or NOT WRITTEN via not_written().
class NotWritten(Exception):
    pass


def not_written(reason):
    raise NotWritten(reason)


def a01(app):
    go(app, P + "/a")
    new_tab(app)
    go(app, P + "/unload")
    d.page_click(app, "body")
    app.cmd("key", "cmd+w")
    time.sleep(1.5)
    return app.js(ROWS) == 2, "the tab closed without asking"


def a02(app):
    new_tab(app)
    go(app, P + "/slow", wait=False)
    time.sleep(0.5)
    app.cmd("key", "cmd+w")
    time.sleep(1.5)
    return app.js(ROWS) == 1, app.js(ROWS)


def a03(app):
    go(app, P + "/slow", wait=False)
    time.sleep(0.5)
    go(app, P + "/b")
    time.sleep(4.5)
    return title(app) == "B" and app.js(VALUE).endswith("/b"), (title(app), app.js(VALUE))


def a04(app):
    go(app, P + "/a")
    go(app, P + "/b")
    app.cmd("key", "cmd+[")
    time.sleep(1.5)
    back = title(app)
    app.cmd("key", "cmd+]")
    time.sleep(1.5)
    return back == "A" and title(app) == "B", (back, title(app))


def a05(app):
    go(app, P + "/link")
    s = page_socket(devtools(app.profile), "/link")
    x, y = middle(app, "a")
    cdp_click(s, x, y, modifiers=4)  # Cmd.
    time.sleep(1.5)
    return app.js(ROWS) == 2 and title(app) == "Link", (app.js(ROWS), title(app))


def a06(app):
    go(app, P + "/popup")
    time.sleep(1.5)
    return app.js(ROWS) == 1, f"{app.js(ROWS)} tabs: the popup opened without a click"


def a07(app):
    go(app, f"http://127.0.0.1:{closed_port()}/")
    time.sleep(1)
    text = page_text(app)
    return bool(text and text.strip()), "the failed load shows an empty page"


def a08(app):
    go(app, f"http://user:secret@127.0.0.1:{PORT}/a")
    value = app.js(VALUE)
    return "secret" not in value, value


def a09(app):
    go(app, P + "/a")
    shown = app.js("!!document.querySelector('[aria-label*=secure i],[title*=secure i]')")
    return shown, "nothing in the shell says the http page isn't secure"


def a10(app):
    go(app, HTTPS + "/a")
    time.sleep(1)
    text = page_text(app)
    return bool(text and text.strip()), f"a self-signed certificate shows {'an empty page' if not text else repr(text[:60])}"


def a11(app):
    not_written("needs a test client certificate in the login keychain")


def a12(app):
    go(app, P + "/auth")
    time.sleep(1.5)
    return title(app) != "Denied", "the 401 page showed without asking for a password"


def a13(app):
    go(app, P + "/dialog")
    d.page_click(app, "#b")
    time.sleep(1)
    return title(app) == "waiting", f"alert returned at once (title {title(app)!r})"


def a14(app):
    go(app, P + "/file")
    d.page_click(app, "#f")
    time.sleep(1.5)
    return title(app) == "waiting", f"no file chooser (title {title(app)!r})"


def a15(app):
    # Only the profile is searched: reading ~/Downloads waits on a macOS privacy prompt that
    # no one is there to answer.
    go(app, P + "/download")
    d.page_click(app, "#a")
    deadline = time.time() + 6
    found = []
    while time.time() < deadline and not found:
        found = glob.glob(os.path.join(app.profile, "**", DL_NAME + "*"), recursive=True)
        time.sleep(0.3)
    asked = any(p.startswith("/dl") for _, p in seen)
    return bool(found), f"no file arrived in the profile{'' if asked else ', and the link was never fetched'}"


def a16(app):
    go(app, P + "/perm")
    d.page_click(app, "#b")
    time.sleep(1.5)
    return title(app) == "pending", f"no prompt, the answer was {title(app)!r}"


def a17(app):
    go(app, P + "/cam")
    d.page_click(app, "#b")
    time.sleep(1.5)
    return title(app) == "pending", f"no prompt, the answer was {title(app)!r}"


def a18(app):
    go(app, P + "/fs")
    d.page_click(app, "#b")
    time.sleep(2)
    return title(app) == "full", f"requestFullscreen gave {title(app)!r}"


def a19(app):
    go(app, P + "/keys")
    d.page_click(app, "body")
    time.sleep(0.3)
    app.cmd("key", "cmd+t")
    time.sleep(0.8)
    return app.js(ROWS) == 2, "a page that swallows keys kept Cmd+T from opening a tab"


def a20(app):
    go(app, P + "/steal")
    d.focus_bar(app)
    app.cmd("type", "x")
    time.sleep(2)
    app.cmd("type", "yz")
    time.sleep(0.3)
    value = app.js(VALUE)
    return value.endswith("xyz"), f"the page took focus mid-typing, the bar holds {value!r}"


def a21(app):
    go(app, P + "/a")
    window = json.loads(app.cmd("bounds"))
    page, top = app.cmd("hit", 600, 300), app.cmd("hit", app.js("window.innerWidth") - 6, d.BAR_Y)
    return page == "page" and top == "frame", (page, top, window.get("reported"))


def a22(app):
    # On a link: Go leaves a right-click on plain text to content, by design.
    go(app, P + "/link")
    s = page_socket(devtools(app.profile), "/link")
    x, y = app.js("(r => [r.x + r.width / 2, r.y + r.height / 2])(document.querySelector('a').getBoundingClientRect())", page=True)
    cdp_click(s, x, y, button="right")
    time.sleep(1)
    shown = app.js("!!document.querySelector('[role=menu]')")
    asked = json.loads(app.cmd("stats")).get("hooks", {}).get("HandleContextMenu", 0)
    return shown, f"no menu in the shell after a right-click on a link; Go was asked {asked} time(s)"


def a23(app):
    port = devtools(app.profile)
    version = json.load(urllib.request.urlopen(f"http://127.0.0.1:{port}/json/version"))
    s = ws_open(version["webSocketDebuggerUrl"])[0]
    r = cdp(s, 1, "Target.createBrowserContext")
    return "result" in r, r.get("error")


def a24(app):
    go(app, P + "/cookie")
    app.close()
    again = start(profile=app.profile)
    try:
        ready(again)
        go(again, P + "/a")
        cookie = again.js("document.cookie", page=True)
    finally:
        again.close()
    return "basics=kept" in cookie, repr(cookie)


def a25(app):
    go(app, P + "/a")
    app.js("location.href='chrome://shell/standard/'", page=True)
    time.sleep(1.5)
    href = app.js("location.href", page=True)
    return not href.startswith("chrome://shell"), href


def a26(app):
    seen.clear()
    go(app, "http://proxied.test/a")
    time.sleep(0.5)
    through = [p for _, p in seen if p.startswith("http://proxied.test")]
    return bool(through), "the request didn't go through --proxy-server"


def a27(app):
    seen.clear()
    go(app, P + "/blocked-image")
    time.sleep(1.5)
    leaked = [h for h, _ in seen if h.startswith(d.BLOCKED)]
    return not leaked, f"an image from the blocked host {d.BLOCKED} was fetched"


def a28(app):
    go(app, "https://testsafebrowsing.appspot.com/s/malware.html")
    time.sleep(1)
    text = page_text(app) or ""
    return "Malware Page Example" not in text, "Safe Browsing's test malware page loaded with no warning"


def a29(app):
    go(app, P + "/a")
    send = app.js("typeof (window.chrome&&window.chrome.send)", page=True)
    return send == "undefined", f"chrome.send is {send} in a page"


def a30(app):
    go(app, P + "/a")
    app.cmd("crashpage")
    time.sleep(1)
    app.cmd("key", "cmd+r")
    time.sleep(2)
    return app.js("document.title", page=True) == "A", "the crashed tab didn't come back on reload"


def a31(app):
    """A Go panic crashes the app and is in crash.log. ProjectB.panic, our own DevTools
    command, panics inside Go's hook as a bug would."""
    version = json.load(urllib.request.urlopen(f"http://127.0.0.1:{devtools(app.profile)}/json/version"))
    s = ws_open(version["webSocketDebuggerUrl"])[0]
    tabs = cdp(s, 1, "ProjectB.getTabs")
    if len(tabs.get("result", {}).get("tabs", [])) != 1:
        return False, f"ProjectB.getTabs gave {tabs}"
    ws_send(s, json.dumps({"id": 2, "method": "ProjectB.panic", "params": {}}))
    try:
        code = app.proc.wait(timeout=10)
    except subprocess.TimeoutExpired:
        return False, "the app carried on after a panic"
    d.App.running.remove(app)
    crash = open(os.path.join(app.profile, "crash.log")).read()
    return code != 0 and "panic: ProjectB.panic" in crash, f"exit code {code}, crash.log {crash[:120]!r}"


def a32(app):
    go(app, P + "/a")
    new_tab(app)
    go(app, P + "/b")
    time.sleep(1)
    app.proc.kill()
    app.proc.wait()
    d.App.running.remove(app)
    again = start(profile=app.profile)
    try:
        ready(again)
        time.sleep(1)
        rows = again.js(ROWS)
    finally:
        again.close()
    return rows == 2, f"after a crash the app came back with {rows} tab(s)"


def a33(app):
    go(app, P + "/slow", wait=False)
    time.sleep(0.5)
    app.cmd("close")
    try:
        code = app.proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        return False, "still running 5 s after closing with a load in flight"
    d.App.running.remove(app)
    return code == 0, f"exit code {code}"


def a34(app):
    r = subprocess.run(["go", "-C", os.path.join(ROOT, "core"), "test", "./update"], capture_output=True, text=True)
    return r.returncode == 0, r.stdout[-200:]


def a35(app):
    unnamed = app.js("""[...document.querySelectorAll('button,[role=button],input,[role=tab]')]
        .filter(e=>!(e.getAttribute('aria-label')||e.getAttribute('title')||e.textContent.trim()||e.getAttribute('placeholder')))
        .map(e=>e.outerHTML.slice(0,60))""")
    return not unnamed, unnamed


def a36(app):
    go(app, P + "/pip")
    d.page_click(app, "#b")
    time.sleep(2)
    return title(app) == "pip", f"picture-in-picture gave {title(app)!r}"


def a37(app):
    app.cmd("resize", 1000, 700)
    time.sleep(1)
    width = app.js("innerWidth")
    return width == 1000, width


def a38(app):
    not_written("needs a captive portal simulator; Chrome's detection is its own //chrome code")


def a39(app):
    ports = listening_ports(app.proc.pid)
    file = os.path.exists(os.path.join(app.profile, "DevToolsActivePort"))
    return not ports and not file, (ports, file)


AREAS = [
    (1, "window lifecycle: a page's beforeunload asks before its tab closes", a01, ()),
    (2, "tab lifecycle: a tab closes while it is still loading", a02, ()),
    (3, "engine ordering: a second navigation during a pending load wins", a03, ()),
    (4, "navigation: back and forward", a04, ()),
    (5, "link dispositions: Cmd+click opens a background tab", a05, ("--remote-debugging-port=0",)),
    (6, "popups: window.open without a click is blocked", a06, ()),
    (7, "load state: a failed load shows an error page", a07, ()),
    (8, "URL display: the address bar hides a URL's password", a08, ()),
    (9, "site identity: an http page is marked not secure", a09, ()),
    (10, "certificate errors: a bad certificate shows a warning page", a10, ()),
    (11, "client certificates: the user picks one", a11, ()),
    (12, "HTTP auth: a 401 asks for a password", a12, ()),
    (13, "JavaScript dialogs: alert waits for the user", a13, ()),
    (14, "file chooser: clicking a file input opens a chooser", a14, ()),
    (15, "downloads: an attachment link saves a file", a15, ()),
    (16, "permissions: a notification request asks the user", a16, ()),
    (17, "camera: getUserMedia asks the user", a17, ("--use-fake-device-for-media-stream",)),
    (18, "page fullscreen: requestFullscreen after a click works", a18, ()),
    (19, "keyboard: Cmd+T opens a tab when a page swallows keys", a19, ()),
    (20, "focus: a page can't take focus from the address bar while typing", a20, ()),
    (21, "hit-testing: the page and the top row's empty space", a21, ()),
    (22, "context menu: right-click on a link shows a menu", a22, ("--remote-debugging-port=0",)),
    (23, "profiles and contexts: a second browser context", a23, ("--remote-debugging-port=0",)),
    (24, "cookies: a persistent cookie survives a restart", a24, ()),
    (25, "internal pages: a tab can't navigate to chrome://shell/standard/", a25, ()),
    (26, "proxy: --proxy-server carries the request", a26, ("--proxy-server=PROXY",)),
    (27, "URL policy: a blocked host's image isn't fetched", a27, ("--host-resolver-rules=MAP blocked.test 127.0.0.1",)),
    (28, "Safe Browsing: a malware test page warns", a28, ()),
    (29, "shell bridge: pages have no chrome.send", a29, ()),
    (30, "renderer crash: a crashed tab comes back on reload", a30, ()),
    (31, "Go-side faults: a panic crashes the app and is logged", a31, ("--remote-debugging-port=0",)),
    (32, "session restore: the tabs come back after a crash", a32, ()),
    (33, "shutdown: closing with a load in flight exits cleanly", a33, ()),
    (34, "updates: the updater's own tests", a34, ()),
    (35, "accessibility: every shell control has a name", a35, ()),
    (36, "picture-in-picture: a video enters it", a36, ()),
    (37, "window bounds: a resize reaches the shell", a37, ()),
    (38, "captive portals", a38, ()),
    (39, "DevTools: off unless the switch is given", a39, ()),
]


STUCK = 60  # Seconds a check may take before its area is marked stuck.


def attempt(fn, extra, out):
    extra = [e.replace("PROXY", f"127.0.0.1:{PORT}") for e in extra]
    try:
        app = None
        if fn not in (a11, a34, a38):
            app = start(extra)
            app.profile = app.proc.args[1].split("=", 1)[1]
            ready(app)
        ok, detail = fn(app)
        out[:] = ["ok" if ok else "FAIL", detail]
    except NotWritten as e:
        out[:] = ["NOT WRITTEN", str(e)]
    except Exception as e:  # A check that stops early is a failure of its area.
        out[:] = ["FAIL", f"{type(e).__name__}: {e}"]
        traceback.print_exc(file=LOG)


def run(area, name, fn, extra):
    out = []
    t = threading.Thread(target=attempt, args=(fn, extra, out), daemon=True)
    t.start()
    t.join(STUCK)
    if t.is_alive() or not out:
        out = ["FAIL", f"stuck for {STUCK} s"]
    # Every app the check started and left running is closed, or killed if it won't close.
    for app in list(d.App.running):
        try:
            app.close()
        except Exception:
            app.proc.kill()
            app.proc.wait()
            if app in d.App.running:
                d.App.running.remove(app)
    status, detail = out
    results.append((area, name, status, detail))
    shown = "" if status == "ok" else f" -> {detail}"
    print(f"{status:<11} {area:>2} {name}{shown}", flush=True)


if __name__ == "__main__":
    os.makedirs(OUT, exist_ok=True)
    LOG = open(os.path.join(OUT, "basics.log"), "w")
    only = {int(a) for a in sys.argv[1:]}
    for area, name, fn, extra in AREAS:
        if not only or area in only:
            run(area, name, fn, extra)
    counts = {s: sum(1 for r in results if r[2] == s) for s in ("ok", "FAIL", "NOT WRITTEN")}
    print(f"\nbasics: {counts['ok']} of {len(results)} areas work, {counts['FAIL']} missing, {counts['NOT WRITTEN']} not written yet", flush=True)
    sys.exit(1 if counts["FAIL"] else 0)
