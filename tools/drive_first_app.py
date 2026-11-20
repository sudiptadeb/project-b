#!/usr/bin/env python3
"""Runs the first app with real Chromium and tests it through real NSEvents.

The driver asks the window's own hitTest:, sets first responder and hands the event to
the view, because [window sendEvent:] delivers nothing without a GUI session. Content's
view checks the same hitTest: before accepting the event. AppKit's own dispatch isn't
exercised, and neither are full screen and the clipboard, which need a person's desktop
session (tools/check_full_screen.py, tools/check_clipboard.py). This needs no display. Run it from the repo root:

    python3 tools/drive_first_app.py
    python3 tools/drive_first_app.py driver     # only the driver itself
    python3 tools/drive_first_app.py settings   # only menus, keys and the settings page
    python3 tools/drive_first_app.py timeline   # only the timeline
    python3 tools/drive_first_app.py cookies    # only cookies kept across a restart
    python3 tools/drive_first_app.py visible    # only the tabs a window shows
    python3 tools/drive_first_app.py windows    # only several windows
    python3 tools/drive_first_app.py empty      # only the new tab a window opens on, and its last tab
    python3 tools/drive_first_app.py move       # only moving a tab between windows
    python3 tools/drive_first_app.py bounds     # only a window's frame, set and read
    python3 tools/drive_first_app.py profiles   # only the start choosing a profile from the list
    python3 tools/drive_first_app.py copies     # only two profiles' copies, the Profiles menu and the Dock
    python3 tools/drive_first_app.py picker     # only the picker
    python3 tools/drive_first_app.py workspaces # only workspaces: their storage, New Tab in Workspace, and deleting one
    python3 tools/drive_first_app.py sitedata   # only Settings' Sites and Clear browsing data on the engine's storage
    python3 tools/drive_first_app.py quiterase  # only Clear cookies when the app quits, but the sites kept
    python3 tools/drive_first_app.py shells     # only every shell: starting on it, and switching to it live
    python3 tools/drive_first_app.py webapis    # only the web APIs in core/webfeatures/registry.json
    python3 tools/drive_first_app.py sparks     # only sparks: in the page itself, styled and writing HTML on strict sites
    python3 tools/drive_first_app.py passkeys   # only a page's passkey request, with a virtual security key
    python3 tools/drive_first_app.py internal   # only our own pages as tabs: settings, and the navigation rule
    python3 tools/drive_first_app.py links      # only Links to other sites: the ten ways a page leaves
    python3 tools/drive_first_app.py applink    # only a page's project-b://sync link: pressed, by script, in a frame, by a redirect

Results go to build/real/. Each screenshot is what content rendered, drawn shell over
page by the driver. It is not read back from the window.
"""

import http.server
import json
import os
import re
import shlex
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
import traceback
import urllib.parse
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import links_page  # noqa: E402
import timeline  # noqa: E402
import web_features  # noqa: E402
from drive_chromedriver import cdp, ws_open  # noqa: E402

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
BIN = os.environ.get("PB_BIN") or os.path.join(ROOT, "build/pb/project_b")
# More switches for every start, as Linux's headless container needs --no-sandbox
# (tools/linux/headless.sh).
ARGS = shlex.split(os.environ.get("PB_ARGS", ""))
OUT = os.path.join(ROOT, "build/real")
BAR_Y = 26  # The middle of the top row, where the window buttons centre.
BLOCKED = "blocked.test"
# A blocked host that resolves, to this machine as every *.localhost does, so a request
# the engine failed to refuse would reach the test server.
BLOCKED_LOCAL = "blocked.localhost"
URL_A = "https://example.com/"
URL_B = "https://www.iana.org/help/example-domains"

failures = 0


# Pages served on this machine for the tests that need a server's answer.
class Pages(http.server.BaseHTTPRequestHandler):
    # Every request's host and path, in order, for the checks that something never came.
    seen = []

    def do_GET(self):
        Pages.seen.append(f"{self.headers.get('Host', '').split(':')[0]}{self.path}")
        if self.path == "/subresources":
            # A page on an allowed host asking a blocked one for an image, a fetch and a
            # frame, and this host for an image, which says when the page has loaded.
            port = self.server.server_port
            blocked = f"http://{BLOCKED_LOCAL}:{port}"
            body = (f'<!doctype html><title>Subresources</title><img src="{blocked}/img-blocked">'
                    f'<iframe src="{blocked}/frame-blocked"></iframe>'
                    f'<script>fetch("{blocked}/fetch-blocked").catch(() => {{}})</script>'
                    f'<img src="/img-allowed" onload="document.title=\'loaded\'" onerror="document.title=\'loaded\'">').encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if self.path in ("/sync-loads", "/sync-worker.js", "/sync-import.js", "/sync-data"):
            # A page that loads synchronously, as WebContainer-style pages do: a synchronous
            # XMLHttpRequest, and a worker's importScripts. Content moves a synchronous load's
            # throttles to another thread first, which killed the page when one of ours said
            # it couldn't move. Cross-origin isolated, as such pages are.
            body = {
                "/sync-loads": b'<!doctype html><title>Sync</title><script>'
                               b'const x=new XMLHttpRequest();x.open("GET","/sync-data",false);x.send();'
                               b'const w=new Worker("/sync-worker.js");'
                               b'w.onmessage=e=>document.title=x.responseText+" "+e.data+" "+crossOriginIsolated</script>',
                "/sync-worker.js": b'importScripts("/sync-import.js");postMessage(imported)',
                "/sync-import.js": b'var imported="imported"',
                "/sync-data": b"xhr",
            }[self.path]
            self.send_response(200)
            self.send_header("Content-Type", "text/javascript" if self.path.endswith(".js") else "text/html" if self.path == "/sync-loads" else "text/plain")
            self.send_header("Cross-Origin-Opener-Policy", "same-origin")
            self.send_header("Cross-Origin-Embedder-Policy", "require-corp")
            self.send_header("Cross-Origin-Resource-Policy", "same-origin")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if self.path.startswith("/applink"):
            # The app's own links on a page (docs/sync.md, The link): the sync link to press, one
            # to a server that counts connections, another project-b: link, a link the server
            # redirects to the sync link, a frame holding it, and a page whose script goes there.
            if self.path == "/applink-redirect":
                self.send_response(302)
                self.send_header("Location", APP_LINK)
                self.end_headers()
                return
            a = lambda id, href, words: f'<a id={id} href="{href}" style="display:block;padding:12px">{words}</a>'
            body = {
                "/applink": "<!doctype html><title>App links</title>" + a("sync", APP_LINK, "Sync") + a("counted", Pages.counted_link, "Sync, counted")
                            + a("other", "project-b://elsewhere?address=https%3A%2F%2Fsync.example.test%2Fsync%2Fabc", "Other") + a("redirect", "/applink-redirect", "Redirected"),
                "/applink-frame": '<!doctype html><title>Frame</title><iframe id=frame src="/applink-inner" style="width:400px;height:200px;border:0"></iframe>',
                "/applink-inner": f'<!doctype html><a href="{APP_LINK}" onclick="parent.document.title=\'pressed\'" style="display:block;height:200px">Sync</a>',
                "/applink-script": f'<!doctype html><title>Script</title><script>setTimeout(() => {{ location.href = "{APP_LINK}"; document.title = "went" }}, 300)</script>',
            }.get(self.path, "").encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if self.path == "/leave-redirect":
            # Links to other sites' ninth way: a same-site link the server sends elsewhere.
            self.send_response(302)
            self.send_header("Location", f"{OTHER}/r9")
            self.end_headers()
            return
        if self.path == "/leave":
            body = links_page.page(OTHER).encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if self.path == "/redirect":
            self.send_response(302)
            self.send_header("Location", "chrome://shell/standard/")
            self.end_headers()
            return
        if self.path in ("/strict", "/strict.js", "/trusted"):
            # A page whose policy allows only its own styles and scripts, as GitHub's does,
            # which blocks a style a spark adds unless the policy's style rules give way; its
            # script says whether window.open is still the browser's own. And one that wants
            # Trusted Types for its HTML, as YouTube's does.
            body = {
                "/strict": b'<!doctype html><title>Strict</title><p>strict</p><script src="/strict.js"></script>',
                "/strict.js": b'document.title = "open " + (/native code/.test(String(window.open)) ? "native" : "replaced")',
                "/trusted": b'<!doctype html><title>Trusted</title><p>trusted</p>',
            }[self.path]
            self.send_response(200)
            self.send_header("Content-Type", "text/javascript" if self.path.endswith(".js") else "text/html")
            if self.path == "/strict":
                self.send_header("Content-Security-Policy", "default-src 'self'; style-src 'self'")
            if self.path == "/trusted":
                self.send_header("Content-Security-Policy", "require-trusted-types-for 'script'")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if self.path == "/signals":
            # What a site reads to tell Chrome from an embedded browser: the client hints it
            # asks for, kept for browser_signal_checks.
            Pages.signals = {k.lower(): v for k, v in self.headers.items() if k.lower().startswith("sec-ch-ua")}
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.send_header("Accept-CH", "Sec-CH-UA-Full-Version-List, Sec-CH-UA-Platform-Version")
            self.end_headers()
            self.wfile.write(b"<!doctype html><title>Signals</title>")
            return
        if self.path == "/cookie":
            # Sets a cookie that lasts an hour, and shows in its title the cookies it got.
            body = f"<!doctype html><title>{self.headers.get('Cookie', 'none')}</title>".encode()
            self.send_response(200)
            self.send_header("Set-Cookie", "a=1; Max-Age=3600")
            self.send_header("Content-Type", "text/html")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        body = {
            # Counts the clicks that reach the page, in its title.
            "/clicks": b'<!doctype html><title>0</title><body style="height:100vh;margin:0" onclick="document.title=+document.title+1">',
            # Links to itself: in this tab, in a new tab without its opener, as target=_blank
            # is by default and rel=noopener says outright.
            "/store": b'<!doctype html><title>Store</title><a id=link href=/store>Here</a> <a id=blank target=_blank href=/store>Blank</a> <a id=noopener target=_blank rel=noopener href=/store>No opener</a>',
            # A copy button, as sites make them: a click writes to the clipboard.
            "/copy": b'<!doctype html><title>Copy</title><button id=copy style="width:300px;height:100px" onclick="navigator.clipboard.writeText(\'copied\').then(()=>document.title=\'copied\',e=>document.title=\'refused \'+e.name)">Copy</button>',
            # Passkeys (docs/passkeys.md): make(name, exclude, attachment) saves one for a
            # user, on a key that remembers it, or only on the kind of key attachment names;
            # use() signs in with whichever the person chooses; canKeep() asks whether this
            # device can keep one. The title says how each ended, with the passkey's id or
            # the account's name.
            "/passkey": b"""<!doctype html><title>Passkey</title><script>
const b64 = (b) => btoa(String.fromCharCode(...new Uint8Array(b)));
const bytes = (s) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0));
const say = (p) => p.then((s) => document.title = s, (e) => document.title = 'refused ' + e.name);
function make(name, exclude, attachment) {
  document.title = 'asking';
  say(navigator.credentials.create({publicKey: {challenge: new Uint8Array(16), rp: {name: 'Passkeys'},
    user: {id: new TextEncoder().encode(name), name: name + '@example.com', displayName: name},
    pubKeyCredParams: [{type: 'public-key', alg: -7}], authenticatorSelection: {residentKey: 'required', authenticatorAttachment: attachment || undefined},
    excludeCredentials: exclude ? [{type: 'public-key', id: bytes(exclude)}] : []}}).then((c) => 'made ' + b64(c.rawId)));
}
function canKeep() {
  say(PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable().then((yes) => 'keeps ' + yes));
}
function use() {
  document.title = 'asking';
  say(navigator.credentials.get({publicKey: {challenge: new Uint8Array(16)}}).then((c) => 'used ' + new TextDecoder().decode(c.response.userHandle)));
}
</script>""",
            # A service worker that sets the app's badge as it starts, as push handlers do.
            "/badge-worker": b'<!doctype html><title>Worker</title><script>navigator.serviceWorker.register("/badge-sw.js")</script>',
            "/badge-sw.js": b'self.oninstall = () => navigator.setAppBadge(1)',
        }.get(self.path, b'<!doctype html><title>Links</title><a href="chrome://shell/standard/">The shell</a>')
        self.send_response(200)
        self.send_header("Content-Type", "text/javascript" if self.path.endswith(".js") else "text/html")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


# A sync link as the memd page offers one (docs/sync.md).
APP_LINK = "project-b://sync?address=https%3A%2F%2Fsync.example.test%2Fsync%2Fabc"

# A server that only counts the connections it gets, for a sync link's address: nothing may
# reach it before Connect.
counter = socket.create_server(("127.0.0.1", 0))
counted = []


def count_connections():
    while True:
        c, _ = counter.accept()
        counted.append(1)
        c.close()


threading.Thread(target=count_connections, daemon=True).start()
Pages.counted_link = "project-b://sync?address=" + urllib.parse.quote(f"https://127.0.0.1:{counter.getsockname()[1]}/sync/abc", safe="")

pages = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Pages)
threading.Thread(target=pages.serve_forever, daemon=True).start()
PAGES = f"http://127.0.0.1:{pages.server_port}/"
# Another site on the same server, for Links to other sites: localhost isn't 127.0.0.1's site.
OTHER = f"http://localhost:{pages.server_port}"


def check(name, ok, detail=""):
    global failures
    print(("ok   " if ok else "FAIL ") + name + ("" if ok or detail == "" else " -> " + str(detail)), flush=True)
    if not ok:
        failures += 1


# Linux runs these checks headless in a container (tools/linux/headless.sh): what only the
# Mac's window, menu bar and Dock have, or what needs a display, is skipped there, saying so.
MAC = sys.platform == "darwin"
LINUX = sys.platform.startswith("linux")


# The app's own modifier in a menu item's key, as the Mac's menu writes it: Command there,
# Ctrl on Linux, whose keymap has Ctrl where the Mac's has Command.
CMD = "@" if MAC else "^"


def press_settings(app):
    """The key that opens Settings: Cmd+, on the Mac, Ctrl+, on Linux, which the driver's cmd
    is there."""
    app.cmd("key", "cmd+,")


def press_quit(app):
    """The key that quits: Cmd+Q on the Mac; Ctrl+Shift+Q on Linux, whose Shift guards
    against a quit by accident, as in Chrome."""
    app.cmd("key", "cmd+q" if MAC else "cmd+shift+q")


def check_mac(name, ok, detail=""):
    """A check of the Mac's own window parts (its buttons, title bar, frame, menu bar): Linux
    draws them in the shell, or will in its step 3 (docs/research/linux.md)."""
    if MAC:
        check(name, ok, detail)
    else:
        print("skip " + name + " (the Mac's own)", flush=True)


# The line the app prints once it's ready to be used: docs/logging.md.
READY = re.compile(r" mark +go +app\.ready$")


class App:
    running = []  # Every app started and not yet closed.

    def __init__(self, profile, extra=(), bin=None):
        """Starts the app on the folder profile, as it is, or with profile None on whatever
        the start chooses from extra. The root is a new one unless extra names one, so the
        app never reads the real list. bin is another binary than BIN to start."""
        if not any(x.startswith("--root=") for x in extra):
            extra = [f"--root={tempfile.mkdtemp(prefix='pb-root-')}", *extra]
        self.fifo = os.path.join(tempfile.mkdtemp(prefix="pb-driver-"), "fifo")
        os.mkfifo(self.fifo)
        self.log = open(os.path.join(OUT, "app.log"), "a")
        self.replies = {}
        self.cond = threading.Condition()
        self.next = 0
        self.started = time.time()
        self.proc = subprocess.Popen(
            [bin or BIN, *([f"--user-data-dir={profile}"] if profile else []), f"--block={BLOCKED},{BLOCKED_LOCAL}", f"--pb-driver={self.fifo}", "--disable-backgrounding-occluded-windows",
             # A first start would open the welcome over the window (docs/onboarding.md):
             # these checks start past it, as its Skip setup does.
             "--skip-welcome", *ARGS, *extra],
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, errors="replace", bufsize=1,
        )
        App.running.append(self)
        threading.Thread(target=self._read, daemon=True).start()
        self.wait_reply("start", 60)
        self.writer = open(self.fifo, "w", buffering=1)
        # The app prints app.ready once the shell shows Go's state: tests start there, once
        # the shell has also reached the screen. Blink drops a page's input until its first
        # paint, which headless Linux, drawing in software, can reach after app.ready.
        self.wait_reply("app.ready", 60)
        self.wait_painted()

    def wait_painted(self, timeout=10):
        """Waits for the timeline's shell.on_screen, the shell's first frame on screen."""
        deadline = time.time() + timeout
        while time.time() < deadline:
            reply = self.cmd("timeline", 0)
            try:
                entries = json.loads(reply)["result"]["entries"]
            except (ValueError, KeyError, TypeError):
                return  # No timeline to wait on, as the profile picker's copy has.
            if "shell.on_screen" in timeline.marks(entries):
                return
            time.sleep(0.05)

    def _read(self):
        for line in self.proc.stdout:
            self.log.write(line)
            self.log.flush()
            m = re.match(r"PBDRIVER (\S+) (.*)$", line.rstrip("\n"))
            if m:
                with self.cond:
                    self.replies[m.group(1)] = m.group(2)
                    self.cond.notify_all()
            elif READY.search(line):
                with self.cond:
                    self.replies["app.ready"] = ""
                    self.cond.notify_all()
        with self.cond:
            self.replies["exit"] = "exit"
            self.cond.notify_all()

    def wait_reply(self, key, timeout=20):
        deadline = time.time() + timeout
        with self.cond:
            while key not in self.replies:
                if "exit" in self.replies and key != "exit":
                    raise RuntimeError(f"app exited waiting for {key}")
                left = deadline - time.time()
                if left <= 0:
                    raise TimeoutError(key)
                self.cond.wait(left)
            return self.replies.pop(key)

    def send(self, *parts):
        """Sends a command without waiting for its reply, and returns the key wait_reply
        takes for it."""
        self.next += 1
        key = str(self.next)
        self.writer.write(" ".join([key, *map(str, parts)]) + "\n")
        self.writer.flush()
        return key

    def cmd(self, *parts, timeout=20):
        return self.wait_reply(self.send(*parts), timeout)

    def js(self, expr, page=False, timeout=20, tab=None):
        """Runs expr in the shell, the focused tab's page with page, or tab's page, by Go's
        id for it. The driver's ERR, for a tab that isn't there or a page that went before
        it answered, is a failure."""
        where = ["evaltab", tab] if tab else ["evalpage" if page else "eval"]
        r = self.cmd(*where, expr.replace("\n", " "), timeout=timeout)
        if r.startswith("ERR"):
            raise RuntimeError(f"{' '.join(map(str, where))}: {r}")
        return json.loads(r) if r != "" else None

    def wait(self, expr, timeout=15, page=False, tab=None):
        """Polls until expr is truthy. A navigation that swaps the page's frame drops the
        answer to an eval sent just before, so a lost answer is tried again."""
        deadline = time.time() + timeout
        last = None
        while time.time() < deadline:
            try:
                last = self.js(expr, page, timeout=5, tab=tab)
            except TimeoutError:
                last = "no answer"
                continue
            if last:
                return last
            time.sleep(0.05)
        raise TimeoutError(f"{expr} last={last}")

    def close(self):
        self.cmd("close")
        self.proc.wait(timeout=30)
        App.running.remove(self)


INPUT = "document.querySelector('input')"
VALUE = INPUT + ".value"
OPTIONS = "document.querySelectorAll('[role=option]').length"
# History's rows: every typed word also gets a search row, and may get an address or action row.
FOUND = "document.querySelectorAll('[role=option][data-kind=history]').length"
ACTIVE = "document.querySelectorAll('li.active').length"
TAB_READY = "!document.querySelector('.progress.on')"
ROWS = "document.querySelectorAll('[role=tab]').length"
ACTIVE_ROW_TITLE = "document.querySelector('[role=tab][aria-selected=true] .title').textContent"
ROW_TITLES = "[...document.querySelectorAll('[role=tab] .title')].map(e=>e.textContent)"


def center(app, selector):
    """The middle of an element, in shell coordinates."""
    x, y = app.js(f"(()=>{{const r=document.querySelector({json.dumps(selector)}).getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]}})()")
    return x, y


def click(app, selector):
    """Clicks a shell element once the window agrees the point is the shell's. The shell draws a
    change a frame before its new regions reach the window manager, and a click in between goes
    to whatever was there before: the page, after the tabs move."""
    x, y = center(app, selector)
    return click_at(app, x, y, selector)


def click_menu_item(app, title):
    """Clicks the item of an open menu of the shell's whose text starts with title, once it
    shows: the standard shell's ☰ menu fills as Go answers."""
    item = "[...document.querySelectorAll('[role=menuitem]')].find(e=>e.textContent.trim().startsWith(%s))" % json.dumps(title)
    app.wait(f"!!{item}", 10)
    x, y = app.js(f"(()=>{{const r={item}.getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]}})()")
    return click_at(app, x, y, title)


def open_settings(app):
    """Opens settings as a person does in the standard shell: its ☰ menu, the menu bar's
    menus (docs/menus-shortcuts.md, "The shell's menu"), then Settings…."""
    click(app, "[aria-label=Menu]")
    return click_menu_item(app, "Settings…")


def click_at(app, x, y, what):
    """Clicks the shell at x, y once the window agrees the point is the shell's, as click does."""
    end = time.time() + 5
    waited = 0
    while app.cmd("hit", x, y) != "shell" and time.time() < end:
        waited += 1
        time.sleep(0.05)
    if waited:
        print(f"note click on {what} waited {waited} polls for the shell's region", flush=True)
    return app.cmd("click", x, y)


def toolbar_gap(app):
    """The middle of the gap between the address field and the button after it, in shell
    coordinates: empty toolbar, which drags the window."""
    return app.js("(()=>{const f=document.querySelector('form.field').getBoundingClientRect();"
                  "const n=document.querySelector('form.field').nextElementSibling.getBoundingClientRect();"
                  "return [(f.right+n.left)/2,f.y+f.height/2]})()")


def served(app):
    """What AppKit last told the window server it may drag by itself: (x, y, w, h)
    rectangles in window points, y from the bottom."""
    return [tuple(map(float, r)) for r in re.findall(r"\{\{([\d.]+), ([\d.]+)\}, \{([\d.]+), ([\d.]+)\}\}", app.cmd("dragregion"))]


def moves_window(app, x, y):
    """What a real drag from this point in shell coordinates does: whether the window
    moves. Inside the title bar the window server moves it by itself from what AppKit
    gives it; below, only the window can hand the press to the server."""
    press = app.cmd("press", x, y)
    if press == "titlebar":
        height = json.loads(app.cmd("bounds"))["content"][1]
        return any(r[0] <= x < r[0] + r[2] and r[1] <= height - y < r[1] + r[3] for r in served(app))
    return press == "moves"


def box(app):
    return app.js("(()=>{const r=document.querySelector('#suggestions').getBoundingClientRect();return [r.x,r.y,r.width,r.height]})()")


def focus_bar(app):
    """Clicks the bar and waits until it really has focus and everything is selected. At the
    start, a new tab's command box takes the keyboard once the shell has drawn it, which can
    come after a click, so the click is tried again."""
    focused = f"document.hasFocus()&&document.activeElement==={INPUT}&&{INPUT}.selectionStart===0&&{INPUT}.selectionEnd==={VALUE}.length"
    for _ in range(2):
        click(app, "input")
        if holds(app, focused, 3):
            return
    click(app, "input")
    app.wait(focused)


def leave_bar(app):
    """Escape until the page has focus. focusPage is asynchronous, so wait for it."""
    deadline = time.time() + 5
    while not app.js("document.hasFocus()", page=True):
        if time.time() > deadline:
            raise TimeoutError("page never got focus")
        app.cmd("key", "escape")
        time.sleep(0.1)


def visit(app, url):
    """Types url in the bar and returns once the tab has its page: the shell shows it, and the
    engine has committed it and finished loading. The shell alone can show the address and no
    progress a moment before the navigation begins, and the page asked about then is the one
    being replaced, whose answer never comes."""
    focus_bar(app)
    app.cmd("type", url)
    app.wait(f"{VALUE}==={json.dumps(url)}")
    app.cmd("key", "enter")
    app.wait(f"{VALUE}.startsWith({json.dumps(url)})&&document.activeElement!=={INPUT}", 30)
    loaded(app, url)


def loaded(app, url):
    """Returns once the window shows a tab that has url's page, committed and done loading.
    In a window with no tabs, as Standard's are at the start, the address typed opens one,
    and until the window shows it there is no page for the driver to ask."""
    app.wait(TAB_READY, 30)
    deadline = time.time() + 30
    while not json.loads(app.cmd("visible")):
        if time.time() > deadline:
            raise TimeoutError(f"the window never showed a tab for {url}")
        time.sleep(0.05)
    deadline = time.time() + 30
    while not ((page := json.loads(app.cmd("committed")))["url"].startswith(url) and not page["loading"]):
        if time.time() > deadline:
            raise TimeoutError(f"the tab never committed {url}: {page}")
        time.sleep(0.05)


# Standard's command box (shell/src/apps/shell/standard/CommandBox.vue), which shows on a new
# tab. Standard takes New Tab: Cmd+T, File > New Tab and its + buttons go to the window's new
# tab, opening one when there is none, and a row chosen there, or an address typed with
# Enter, loads in that tab. A window opens on a new tab (docs/windows.md).
BOX = "document.querySelector('.command-box input')"
BOX_FOCUSED = f"!!{BOX}&&document.hasFocus()&&document.activeElement==={BOX}"


def box_open(app, timeout=10):
    """Waits until the command box shows and has the keyboard."""
    app.wait(BOX_FOCUSED, timeout)


def holds(app, expr, timeout=10, page=False):
    """Whether expr comes true in the shell, or the focused tab's page with page, within
    timeout."""
    try:
        return bool(app.wait(expr, timeout, page=page))
    except TimeoutError:
        return False


def from_box(app, url):
    """Types url in the command box, which has the keyboard on a new tab, and presses Enter,
    the user's way to a page on Standard: the new tab loads it. Returns Go's id for that tab
    once the window shows it with url's page, as visit does."""
    box_open(app)
    rows = app.js(ROWS)
    app.cmd("type", url)
    app.wait(f"{BOX}.value==={json.dumps(url)}")
    app.cmd("key", "enter")
    app.wait(f"{ROWS}==={rows}&&!{BOX}", 30)
    loaded(app, url)
    return tabs_of(app)["active"]


def new_tab(app, url):
    """New Tab, the user's way, on Standard: Cmd+T goes to a new tab, with the box on it, and
    url typed there with Enter loads it. Returns Go's id for the tab."""
    app.cmd("key", "cmd+t")
    return from_box(app, url)


def mac_binary_checks(children):
    """The Mac's three pieces: every child is the helper; Go only in the main binary, content
    only in the library."""
    check("every child process is the helper", children and all(r["exe"] == "project_b_helper" for r in children), children)
    macos = os.path.dirname(BIN)
    helper = os.path.join(macos, "project_b_helper")
    if not os.path.exists(helper):  # the packed app's, in an app of its own
        helper = os.path.join(macos, "../Frameworks/Project B Helper.app/Contents/MacOS/project_b_helper")
    library = os.path.join(macos, "../Frameworks/libproject_b_content.dylib")
    if not os.path.exists(library):
        library = os.path.join(macos, "libproject_b_content.dylib")
    nm = lambda *args: subprocess.run(["nm", *args], capture_output=True, text=True).stdout
    check("the helper has no Go", "_cgo" not in nm(helper) and "runtime." not in nm(helper))
    check("the library has no Go", "_cgo" not in nm(library) and "runtime." not in nm(library))
    check("the main binary has no content", "__ZN7content" not in nm(BIN) and "__ZN5blink" not in nm(BIN))
    exports = nm("-gU", library).split()
    check("the library exports only ProjectBMain", [e for e in exports if e.startswith("_")] == ["_ProjectBMain"], exports[:12])


def linux_binary_checks(app, children):
    """Linux's one binary (docs/research/linux.md, 3): only the browser process maps the Go
    core, libcore.so; no child does, whether started fresh or forked from a zygote."""
    def maps(pid):
        try:
            with open(f"/proc/{pid}/maps") as f:
                return f.read()
        except OSError:
            return ""
    check("the browser process maps the Go core", "/libcore.so" in maps(app.proc.pid))
    live = [r for r in children if "<defunct>" not in r["exe"]]
    mapping = [r for r in live if "/libcore.so" in maps(r["pid"])]
    check("no child process maps the Go core", live and not mapping, mapping or live)
    check("every child is the one binary", all(r["exe"] == "project_b" for r in live), live)
    nm = lambda *args: subprocess.run(["nm", *args], capture_output=True, text=True).stdout
    core = os.path.join(os.path.dirname(BIN), "libcore.so")
    check("the binary has no Go", "runtime.main" not in nm("-D", BIN), BIN)
    check("the Go core exports PBInit and has no content", " T PBInit" in nm("-D", core) and "_ZN7content" not in nm("-D", core))


def process_table(pid):
    """Our process and its descendants, with thread counts and memory."""
    ps = subprocess.run(["ps", "-axo", "pid=,ppid=,rss=,command="], capture_output=True, text=True).stdout
    rows = []
    for line in ps.splitlines():
        p, ppid, rss, command = line.split(None, 3)
        rows.append((int(p), int(ppid), int(rss), command))
    keep, seen = [], {pid}
    changed = True
    while changed:
        changed = False
        for row in rows:
            if row[1] in seen and row[0] not in seen:
                seen.add(row[0])
                changed = True
    for row in rows:
        if row[0] in seen:
            threads = len(subprocess.run(["ps", "-M", "-p", str(row[0])], capture_output=True, text=True).stdout.splitlines()) - 1
            exe = row[3].split(" --")[0].split("/")[-1]  # The path can hold spaces.
            kind = re.search(r"--type=(\S+)", row[3])
            sub = re.search(r"--utility-sub-type=(\S+)", row[3])
            keep.append({"pid": row[0], "exe": exe, "type": kind.group(1) if kind else "main", "sub": sub.group(1) if sub else "", "threads": threads, "rssMB": round(row[2] / 1024, 1)})
    return keep


def page_click(app, selector):
    """A real click on an element of the shown page: its middle, moved into the window by
    where the page view sits."""
    x, y = app.js(f"(()=>{{const r=document.querySelector({json.dumps(selector)}).getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]}})()", page=True)
    px, py = json.loads(app.cmd("bounds"))["reported"][:2]
    return app.cmd("click", px + x, py + y)


def bundle_version():
    """The version in the bundle PB_BIN runs from, or None for a build directory."""
    plist = os.path.join(os.path.dirname(BIN), "../Info.plist")
    if not os.path.exists(plist):
        return None
    m = re.search(r"CFBundleShortVersionString</key>\s*<string>([^<]+)", open(plist).read())
    return m.group(1) if m else None


def run_parts(parts):
    """Runs each part and exits with the result. A part that stops early, because the app
    exited or a wait ran out, is one failure, and the app it left open is stopped."""
    for part in parts:
        # tools/results.py splits a run's memory, CPU and checks by these lines.
        print(f"--- part {part.__name__}", flush=True)
        try:
            part()
        except (RuntimeError, TimeoutError, OSError, subprocess.TimeoutExpired) as e:
            traceback.print_exc()
            check(f"{part.__name__} ran to its end", False, e)
        for app in App.running[:]:
            app.proc.kill()
            App.running.remove(app)
    print("\nall passed" if failures == 0 else f"\n{failures} FAILED", flush=True)
    sys.exit(1 if failures else 0)


def version_checks():
    """--version prints the release, or that it's a development build, and exits without
    starting the app."""
    # 30 s: a package's first launch is checked by macOS first, which took over 10 s once
    # while an APK build loaded the Mac (0.2.100).
    r = subprocess.run([BIN, "--version"], capture_output=True, text=True, timeout=30)
    want = bundle_version() or "development build"
    check("--version prints the version and exits", r.returncode == 0 and re.fullmatch(rf"Project B {re.escape(want)}, Chromium [0-9.]+\n", r.stdout) is not None, (r.returncode, r.stdout))


# The start-up milestones, in the order they must come. The shell's first paint and the
# first tab's commit race the rest, so they are checked apart.
STARTUP = ["process.start", "main.entered", "content.ready", "go.starting", "window.created", "shell.created",
           "shell.navigation_start", "window.shown", "go.started", "shell.committed",
           "shell.script", "shell.mounted", "shell.ready", "shell.snapshot", "app.ready"]


def driver_checks():
    """The driver itself: every command answers. committed is the focused tab's page as
    content has it, which visit waits on. An eval in a page that is destroyed before its
    result comes is answered ERR gone: here a page held by a server that takes the
    connection and never answers, whose tab then closes."""
    app = App(tempfile.mkdtemp(prefix="pb-profile-"))
    visit(app, PAGES + "links")
    page = json.loads(app.cmd("committed"))
    check("committed is the URL the focused tab's page last committed, done loading", page == {"url": app.js("location.href", page=True), "loading": False}, page)
    silent = socket.create_server(("127.0.0.1", 0))
    tab = open_in(app, "", PAGES)
    held = app.send("evaltab", tab, f"(()=>{{const r=new XMLHttpRequest();r.open('GET','http://127.0.0.1:{silent.getsockname()[1]}/',false);r.send();return 1}})()")
    app.cmd("choosemenu", "File", "Close Tab")
    try:
        check("an eval whose page closes before its result comes is answered: ERR gone", app.wait_reply(held) == "ERR gone")
    except TimeoutError as e:
        check("an eval whose page closes before its result comes is answered: ERR gone", False, e)
    silent.close()
    app.close()


def timeline_checks():
    """The app's own timeline (docs/logging.md): the start-up milestones from C++, Go and
    the shell's JavaScript on one clock, each interaction as a span, and the same lines in
    timeline.log."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    log = os.path.join(OUT, "app.log")
    log_start = os.path.getsize(log) if os.path.exists(log) else 0
    app = App(profile)

    def read():
        return json.loads(app.cmd("timeline", 0))["result"]["entries"]

    def wait_for(test, timeout=20):
        deadline = time.time() + timeout
        while time.time() < deadline:
            if test(read()):
                return True
            time.sleep(0.1)
        return False

    # Standard starts on a new tab, with its command box: an address typed there loads the
    # first page, and the window shows pages from then.
    wait_for(lambda es: "shell.on_screen" in timeline.marks(es))
    from_box(app, PAGES)
    wait_for(lambda es: {"shell.on_screen", "window.pages_shown", "tab.committed"} <= set(timeline.marks(es)))
    m = timeline.marks(read())
    ms = lambda name: m[name] / 1e6
    missing = [n for n in STARTUP + ["shell.first_paint", "shell.on_screen", "window.pages_shown", "tab.committed"] if n not in m]
    check("every start-up milestone is on the timeline", not missing, missing)
    if missing:
        app.close()
        return
    check("process start is time zero", m["process.start"] == 0)
    steps = [(a, b, ms(b) - ms(a)) for a, b in zip(STARTUP, STARTUP[1:])]
    check("the milestones come in order, C++, Go and JavaScript alike", all(step >= 0 for _, _, step in steps), [s for s in steps if s[2] < 0])
    # Generous: without a screen AppKit alone can take seconds before main.
    check("no step takes over 15 s and the shell's state arrives within 30 s", all(step < 15000 for _, _, step in steps) and ms("shell.snapshot") < 30000, steps)
    check("the shell paints after its script runs, and that frame reaches the screen after it is painted",
          ms("shell.script") <= ms("shell.first_paint") <= ms("shell.on_screen") < ms("shell.first_paint") + 5000,
          (ms("shell.script"), ms("shell.first_paint"), ms("shell.on_screen")))
    check("the window shows its pages only after the shell has drawn the layout that places them, sent before it says shown",
          ms("shell.snapshot") <= ms("app.ready") <= ms("window.pages_shown"),
          (ms("shell.snapshot"), ms("app.ready"), ms("window.pages_shown")))
    check("the first tab, made from the command box, commits after Go starts it", ms("go.starting") <= ms("tab.committed") < ms("go.starting") + 15000, (ms("go.starting"), ms("tab.committed")))

    def span_after(action, name):
        """Does the action and returns the span it makes, or None."""
        before = len([s for s in timeline.spans(read()) if s["name"] == name])
        action()
        wait_for(lambda es: len([s for s in timeline.spans(es) if s["name"] == name]) > before, 10)
        found = [s for s in timeline.spans(read()) if s["name"] == name]
        return found[-1] if len(found) > before else None

    def in_order(span, stages):
        return span is not None and all(s in span["stages"] for s in stages) and \
            [span["stages"][s] for s in stages] == sorted(span["stages"][s] for s in stages) and 0 < span["dur"] < 10e9

    click(app, "[aria-label=Menu]")
    opened = span_after(lambda: click_menu_item(app, "Settings…"), "settings.open")
    check("opening settings is a span from the click to its first frame on screen, through Go, the engine and the page",
          in_order(opened, ["settings.open.go", "settings.navigation_start", "settings.open.done",
                            "settings.committed", "settings.script", "settings.mounted", "settings.snapshot",
                            "settings.first_paint", "settings.on_screen"]), opened)
    app.cmd("key", "cmd+w")
    time.sleep(0.5)
    # Standard takes New Tab: its span ends when the shell has drawn what it does, a new tab
    # with its command box.
    rows = app.js(ROWS)
    # Cmd+T runs Go's tab.new; Standard's own + goes to the new tab without it.
    new = span_after(lambda: app.cmd("key", "cmd+t"), "tab.new")
    check("New Tab is a span that ends when the shell has drawn what it does, Standard's command box", in_order(new, ["tab.new.go", "tab.new.done"]), new)
    check("on a new tab", holds(app, BOX_FOCUSED) and app.js(ROWS) == rows + 1, (app.js(ROWS), rows))
    closed = span_after(lambda: app.cmd("key", "cmd+w"), "tab.close")
    check("closing a tab from the keyboard is a span too, begun in Go", in_order(closed, ["tab.close.go", "tab.close.done"]), closed)
    moved = span_after(lambda: click(app, "button[aria-label^='Tabs on']"), "tabs.layout")
    check("moving the tabs is a span that covers the page moving", in_order(moved, ["tabs.layout.go", "tabs.layout.done", "tabs.layout.page_moved"]), moved)

    # Each click and key press is a span of its own, from the OS's time for the event to
    # our code having it, with the window getting it between.
    inputs = [s for s in timeline.spans(read()) if s["name"] in ("input.click", "input.key")]
    through = lambda s: 0 <= s["stages"].get(s["name"] + ".window", -1) <= s["dur"] < 5e9
    check("a click and a key press are each a span from the OS's event, through the window, to the shell or Go",
          {"input.click", "input.key"} <= {s["name"] for s in inputs} and all(through(s) for s in inputs),
          ({s["name"] for s in inputs}, [s for s in inputs if not through(s)][:4]))

    # A LOG in the engine's C++ is an entry.
    app.cmd("strayshell pages")
    logged = lambda es, src, kind: [e for e in es if e["src"] == src and e["kind"] == kind]
    wait_for(lambda es: logged(es, "engine", "warn"), 5)
    entries = read()
    check("LOG(WARNING) in the engine is a warn entry", any("outside its page" in e.get("text", "") for e in logged(entries, "engine", "warn")), logged(entries, "engine", "warn"))
    check("no entry carries a URL", not [e for e in entries if "://" in e.get("text", "") + e.get("name", "") and "chrome://" not in e.get("text", "")])

    app.close()
    with open(os.path.join(profile, "timeline.log")) as f:
        written = timeline.parse(f)
    in_file = timeline.marks(written)
    check("timeline.log in the profile has the same marks at the same times", all(in_file.get(n) == m[n] for n in STARTUP), [(n, in_file.get(n), m[n]) for n in STARTUP if in_file.get(n) != m[n]])
    with open(log) as f:
        f.seek(log_start)
        printed = timeline.marks(timeline.parse(f))
    check("and so does what the app printed", all(printed.get(n) == m[n] for n in STARTUP))
    check("and the spans", {"settings.open", "tab.new", "tab.close", "tabs.layout"} <= {s["name"] for s in timeline.spans(written)})


def settings_checks():
    """Menus, keys and the settings page in the real app: the menu bar Go builds, keys
    through Go, and the settings page, in a tab of ours."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = App(profile)
    time.sleep(0.5)
    # Standard starts on a new tab; the checks below want a page under the settings page.
    from_box(app, PAGES)

    def find(items, title):
        return next((i for i in items if isinstance(i, dict) and i["title"] == title), None)

    menu = json.loads(app.cmd("menu"))
    view, edit, tab = find(menu, "View"), find(menu, "Edit"), find(menu, "Tab")
    full = find(view["items"], "Full Screen") if view else None
    check("the menu bar is Go's tree: Edit, View, Tab", bool(view and edit and tab), [m["title"] for m in menu if isinstance(m, dict)])
    # Linux's keymap has F11 for it, as Chrome's on Linux.
    check("View has Full Screen on Ctrl+Cmd+F" if MAC else "View has Full Screen on F11", bool(full) and full["key"] == ("^@f" if MAC else "F11"), full)
    check("Edit has Redo", bool(edit) and find(edit["items"], "Redo") is not None)
    check("the hidden Select Tab submenu isn't drawn", bool(tab) and find(tab["items"], "Select Tab") is None)
    check("AppKit added no Full Screen item of its own", sum(1 for m in menu for i in m.get("items", []) if isinstance(i, dict) and "Full Screen" in i["title"]) == 1)

    # Cmd+, opens the settings tab, beside the active tab, and shows it.
    settings = lambda expr: json.loads(app.cmd("evalsettings", expr))
    rows = app.js(ROWS)
    press_settings(app)
    try:
        deadline = time.time() + 10
        while True:
            r = app.cmd("evalsettings", "!!document.querySelector('h1')")
            if r == "true":
                break
            if time.time() > deadline:
                raise TimeoutError(r)
            time.sleep(0.1)
        check("Cmd+, opens the settings page", True)
    except TimeoutError as e:
        check("Cmd+, opens the settings page", False, e)
        app.close()
        return
    time.sleep(0.5)
    check("it is a tab, its page where the layout puts a page, under the shell's top row",
          app.js(ROWS) == rows + 1 and app.cmd("hit", 600, 300) == "page" and app.cmd("hit", 300, BAR_Y) == "shell", (app.js(ROWS), app.cmd("hit", 600, 300), app.cmd("hit", 300, BAR_Y)))
    check("it shows the settings, in sections", settings("!!document.querySelector('.sections-nav')"))
    print(app.cmd("capture", os.path.join(OUT, "settings-general.png")), flush=True)

    # A key recorded on the settings page reaches Go, and runs its action.
    settings("document.querySelector('[data-section=keyboard]').click()")
    time.sleep(0.3)
    settings("[...document.querySelectorAll('button.action')].find(b=>b.querySelector('.name').textContent.trim()==='New Tab').click()")
    time.sleep(0.3)
    settings("[...document.querySelectorAll('button')].find(b=>b.textContent.trim()==='Add key').click()")
    time.sleep(0.3)
    app.cmd("key", "cmd+shift+k")
    # How the keymap writes that key: the Mac's order and Command; Ctrl first on Linux.
    RECORDED = "Shift+Cmd+K" if MAC else "Ctrl+Shift+K"
    keymap = os.path.join(profile, "keymap.json")
    deadline = time.time() + 5
    while time.time() < deadline and not (os.path.exists(keymap) and RECORDED in open(keymap).read()):
        time.sleep(0.1)
    check("a key pressed while recording is added to keymap.json", os.path.exists(keymap) and RECORDED in open(keymap).read(),
          (os.listdir(profile), app.cmd("evalsettings", "[document.activeElement.className, document.querySelector('[aria-label=File]').textContent.slice(0,80)]")))
    print(app.cmd("capture", os.path.join(OUT, "settings-keyboard.png")), flush=True)

    # Cmd+W closes the settings tab, as it closes any tab.
    app.cmd("key", "cmd+w")
    time.sleep(0.5)
    check("Cmd+W closes the settings tab", app.cmd("evalsettings", "1").startswith("ERR") and app.js(ROWS) == rows)
    # The key runs New Tab, which Standard takes: it goes to a new tab, with its command box.
    app.cmd("key", "cmd+shift+k")
    check("the new key runs New Tab: a new tab, with Standard's command box", holds(app, BOX_FOCUSED, 5) and app.js(ROWS) == rows + 1, app.js(ROWS))
    app.cmd("key", "cmd+w")
    app.wait(f"{ROWS}==={rows}", 5)

    # Ctrl+Tab goes back through recent tabs while Ctrl is held; letting go settles.
    while app.js(ROWS) < 3:
        open_in(app, "", PAGES)
    selected = "[...document.querySelectorAll('[role=tab]')].findIndex(e=>e.getAttribute('aria-selected')==='true')"
    last = app.js(ROWS) - 1
    app.wait(f"{selected}==={last}", 5)
    app.cmd("key", "ctrl+tab")
    one_back = app.js(selected)
    app.cmd("key", "ctrl+tab")
    two_back = app.js(selected)
    app.cmd("mods", 0)
    app.cmd("key", "ctrl+tab")
    time.sleep(0.2)
    back = app.js(selected)
    check("held Ctrl+Tab goes back through three tabs, and after letting go one press returns to where it started",
          len({last, one_back, two_back}) == 3 and back == last, (last, one_back, two_back, back))
    app.cmd("mods", 0)

    app.close()

    # After a restart the keymap is still the user's: the key opens the box over a tab. Tabs come
    # back at start by default, so start fresh, on the box, as this check expects.
    if os.path.exists(os.path.join(profile, "session.json")):
        os.remove(os.path.join(profile, "session.json"))
    app = App(profile)
    from_box(app, PAGES)
    rows = app.js(ROWS)
    app.cmd("key", "cmd+shift+k")
    check("the added key survives a restart: it goes to a new tab", holds(app, BOX_FOCUSED, 5) and app.js(ROWS) == rows + 1, app.js(ROWS))
    app.close()


def browsing_checks():
    """The window, the address bar, pages, tabs and crashes in the real app."""
    if os.path.exists(OUT):
        shutil.rmtree(OUT)
    os.makedirs(OUT)
    profile = tempfile.mkdtemp(prefix="pb-profile-")

    t0 = time.time()
    app = App(profile)
    print(f"shell page answered {time.time() - t0:.2f}s after launch (includes driver polling)", flush=True)
    check("opens empty", app.js(VALUE) == "")
    check("snapshot says the real engine", app.js("document.querySelector('iframe')===null"))
    app.wait("document.querySelector('form').getBoundingClientRect().height>0")
    time.sleep(0.3)
    # The window opens on a new tab, which loads nothing: the page area is Standard's
    # command box.
    check("one new tab to start with: the page area is Standard's command box", app.js(ROWS) == 1 and app.js(f"!!{BOX}"), app.js(ROWS))
    check("opens with the command box focused", holds(app, BOX_FOCUSED, 5))
    check("on the new tab the page area is the shell's, its command box", app.cmd("hit", 600, 300) == "shell", app.cmd("hit", 600, 300))
    # The page area is the command box, the shell's own, never the frame colour showing
    # through a missing page. The frame colour used to take over a few seconds in, so this
    # looks after ten.
    time.sleep(max(0, 10 - (time.time() - app.started)))
    check("ten seconds on, the page area of the new tab is still the command box, the shell's",
          app.js(f"!!{BOX}") and app.cmd("hit", 600, 300) == "shell", app.cmd("hit", 600, 300))
    print(app.cmd("capture", os.path.join(OUT, "1-empty.png")), flush=True)

    # Real pages through the address bar: the new tab loads what is typed there.
    visit(app, URL_A)
    check("an address typed in the address field of the new tab loads it there", app.js(ROWS) == 1 and not app.js(f"!!{BOX}"), app.js(ROWS))
    # The driver's stats and bounds speak of the focused tab's page, so they wait for one.
    stats = json.loads(app.cmd("stats", timeout=60))
    print("stats", json.dumps(stats), flush=True)
    print("bounds", app.cmd("bounds"), flush=True)

    # Hit tests start from the window's frame view, as AppKit's do, so the title bar and
    # its buttons take part in the answer.
    bar_x, bar_y = center(app, "input")
    check("the address bar, at title bar height, belongs to the shell and not the title bar", app.cmd("hit", bar_x, bar_y) == "shell", app.cmd("hit", bar_x, bar_y))
    check("the page belongs to the page", app.cmd("hit", 600, 300) == "page")
    check("the sidebar belongs to the shell", app.cmd("hit", 100, 60) == "shell", app.cmd("hit", 100, 60))
    window = json.loads(app.cmd("bounds"))
    print("title bar", window["titlebar"], "buttons", window["buttonsStart"], window["buttonsEnd"], "middle", window["buttonsMiddleY"], "size", window["buttonSize"], "screen", app.js("[screen.width,screen.height]"), flush=True)
    toggle_left = app.js("document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x")
    check_mac("the shell leaves exactly the window buttons' space, as the engine reported it", toggle_left == window["buttonsEnd"] + 8, (toggle_left, window))
    gap = app.cmd("hit", (window["buttonsEnd"] + toggle_left) / 2, BAR_Y)
    check("the gap after the window buttons falls through to the window frame, which drags", gap == "frame", gap)
    right_end = app.cmd("hit", app.js("window.innerWidth") - 6, BAR_Y)
    check("the right end of the top row falls through to the frame too", right_end == "frame", right_end)
    below_tabs = app.cmd("hit", 100, 600)
    check("the empty sidebar is shell, so it never drags the window", below_tabs == "shell", below_tabs)
    over_buttons = app.cmd("hit", window["buttonsStart"] + window["buttonSize"] / 2, BAR_Y)
    check_mac("the window buttons still get their own clicks", over_buttons.startswith("native:"), over_buttons)
    print("hit over window buttons:", over_buttons, flush=True)
    # The window server moves the window by itself from whatever part of the title bar
    # AppKit gives it, without asking the app. Given the address bar, it moved the window
    # on a drag there instead of selecting text; given nothing, no drag moved it.
    region = app.cmd("dragregion")
    rects = served(app)
    def given(x, y):
        return any(r[0] <= x < r[0] + r[2] and r[1] <= window["content"][1] - y < r[1] + r[3] for r in rects)
    check_mac("the window server isn't given the address bar to drag by itself", region != "unknown" and not given(bar_x, bar_y), region)
    gap_x = window["buttonsEnd"] + 4
    check_mac("the window server is given the gap right of the window buttons", given(gap_x, BAR_Y), (gap_x, region))
    # The toolbar drags wherever it has no control: its gaps are the frame's, and the
    # window server's, and the controls in it stay the shell's.
    tool_x, tool_y = toolbar_gap(app)
    tool = app.cmd("hit", tool_x, tool_y)
    check("the toolbar's gap between the address field and the buttons falls through to the frame", tool == "frame", (tool_x, tool_y, tool))
    check("a drag from that gap moves the window", moves_window(app, tool_x, tool_y), (tool_x, tool_y, app.cmd("press", tool_x, tool_y), region))
    back_x, back_y = center(app, "[aria-label=Back]")
    check("the back button in the toolbar belongs to the shell", app.cmd("hit", back_x, back_y) == "shell", app.cmd("hit", back_x, back_y))

    check("loads a real page", app.js("location.href", page=True) == URL_A, app.js("location.href", page=True))
    title = app.wait("document.title", page=True)
    check("the page's own title reaches the page", "Example" in title, title)
    time.sleep(0.5)
    check("window title follows the tab", "Example" in app.cmd("title"), app.cmd("title"))
    visit(app, URL_B)
    app.wait(f"{VALUE}.includes('iana.org')", 30)
    # A hook added in Go only: content's load progress reaches the shell's bar.
    progress = json.loads(app.cmd("stats", timeout=60))["go"]["hooks"].get("engine.LoadProgressChanged", 0)
    check("the load progress hook, carried by Go alone, fired in the real app", progress > 0, progress)
    check("the progress bar shows the page's real load progress", app.js("document.querySelector('.progress').style.width") == "100%", app.js("document.querySelector('.progress').style.width"))
    print(app.cmd("capture", os.path.join(OUT, "2-page.png")), flush=True)

    # Back, forward and reload, through the real buttons.
    disabled = lambda name: app.js(f"document.querySelector('[aria-label={name}]').disabled")
    check("back is on and forward is off after new pages", not disabled("Back") and disabled("Forward"))
    click(app, "[aria-label=Back]")
    app.wait(f"location.href==={json.dumps(URL_A)}", 30, page=True)
    app.wait(f"{VALUE}==={json.dumps(URL_A)}", 10)
    check("back returns to the previous page", True)
    app.wait("!document.querySelector('[aria-label=Forward]').disabled", 10)
    check("forward is on after going back", True)
    app.wait(f"{ACTIVE_ROW_TITLE}.includes('Example Domain')", 10)
    check("the tab's title comes back with the page", True)
    click(app, "[aria-label=Forward]")
    app.wait("location.href.includes('iana.org')", 30, page=True)
    app.wait(f"{VALUE}.includes('iana.org')", 10)
    check("forward returns to the next page", True)
    app.js("(()=>{window.__before_reload=1;return 1})()", page=True)
    click(app, "[aria-label=Reload]")
    app.wait("document.readyState==='complete'&&!window.__before_reload", 30, page=True)
    check("reload loads the page again", True)

    # Suggestions over the page, from real history.
    focus_bar(app)
    app.cmd("type", "exam")
    app.wait(OPTIONS)
    time.sleep(0.3)
    print(app.cmd("capture", os.path.join(OUT, "3-suggestions.png")), flush=True)
    check("typing shows suggestions from history", app.js(OPTIONS) >= 1, app.js(OPTIONS))
    # The first row searches what was typed; history's rows follow it.
    check("a suggestion has the real page's title", app.js("[...document.querySelectorAll('[role=option]')].some(o => o.innerText.includes('Example'))"))
    x, y, w, h = box(app)
    check("the box belongs to the shell", app.cmd("hit", x + 30, y + h - 6) == "shell")
    # Below the box over the page: where the box starts depends on the window buttons, at the
    # left on the Mac and the right on Linux, so the point is inside the page's span too.
    page_x = json.loads(app.cmd("bounds"))["reported"][0]
    check("below the box belongs to the page", app.cmd("hit", max(x, page_x) + 30, y + h + 40) == "page")

    # Bar and list behave as one UI.
    app.cmd("key", "down")
    app.wait(ACTIVE)
    check("down moves into the list", app.js(VALUE).startswith("https://"), app.js(VALUE))
    app.cmd("key", "up")
    app.wait(f"{ACTIVE}===0")
    check("up from the first item returns to what was typed", app.js(VALUE) == "exam", app.js(VALUE))
    check("focus never left the address bar", app.js(f"document.activeElement==={INPUT}"))

    # Clicking the page closes the box and focus moves to it.
    app.js("window.__pageClicks=0", page=True)
    app.js("(()=>{window.__pageClicks=0;addEventListener('mousedown',()=>window.__pageClicks++);return 1})()", page=True)
    app.cmd("click", 900, 600)
    app.wait(f"{OPTIONS}===0")
    check("clicking the page closes the box", True)
    check("the click reached the page", app.wait("window.__pageClicks", page=True) == 1)
    check("page has focus after clicking it", app.js("document.hasFocus()", page=True))

    # A click on the box must not reach the page behind it, and it navigates.
    focus_bar(app)
    app.cmd("type", "exam")
    app.wait(OPTIONS)
    time.sleep(0.2)
    # History's row for the page, not the search row first.
    x, y = app.js("(r => [r.x + 40, r.y + r.height / 2])([...document.querySelectorAll('[role=option]')].find(o => o.innerText.includes('example.com')).getBoundingClientRect())")
    app.cmd("click", x, y)
    app.wait(f"{VALUE}.startsWith('https://example.com')", 30)
    app.wait(TAB_READY, 30)
    check("clicking a suggestion navigates, so the shell got the click and not the page", True)

    # Shortcut from the page.
    app.cmd("click", 900, 600)
    app.wait(f"document.activeElement!=={INPUT}")
    app.cmd("key", "cmd+l")
    try:
        app.wait(f"document.activeElement==={INPUT}&&document.hasFocus()", 5)
        check("Cmd+L focuses the address bar from the page", True)
    except TimeoutError as e:
        check("Cmd+L focuses the address bar from the page", False, e)
    leave_bar(app)

    # Tab off the end of the shell moves into the page. The sidebar comes after the bar.
    focus_bar(app)
    for _ in range(8):
        app.cmd("key", "tab")
        time.sleep(0.15)
        if app.js("document.hasFocus()", page=True):
            break
    check("tab past the end of the shell moves into the page", app.js("document.hasFocus()", page=True))

    # Policy answered synchronously by Go.
    focus_bar(app)
    app.cmd("type", "https://www.blocked.test/secret")
    app.wait(f"{VALUE}==='https://www.blocked.test/secret'")
    app.cmd("key", "enter")
    try:
        app.wait("document.querySelector('.status')&&document.querySelector('.status').innerText==='Blocked'", 10)
        check("a blocked host is stopped by Go's gate", True)
    except TimeoutError as e:
        check("a blocked host is stopped by Go's gate", False, e)
    check("the page never left the host it was on", "blocked.test" not in app.js("location.host", page=True), app.js("location.host", page=True))
    time.sleep(0.3)
    print(app.cmd("capture", os.path.join(OUT, "4-blocked.png")), flush=True)
    focus_bar(app)
    app.cmd("type", "secret")
    app.wait(f"{VALUE}==='secret'")
    time.sleep(0.4)
    check("a blocked URL stays out of history", app.js(FOUND) == 0, app.js(FOUND))
    # With no suggestions open, Escape leaves the bar and the shell asks for the page's
    # keyboard. A click on the bar before that request lands loses the keyboard to it, so
    # wait for the page to have it.
    leave_bar(app)
    focus_bar(app)
    app.cmd("type", "blank")
    app.wait(f"{VALUE}==='blank'")
    time.sleep(0.4)
    check("the blank page stays out of history", app.js(FOUND) == 0, app.js(FOUND))
    leave_bar(app)

    # A page restored from the back-forward cache never reports a start, so its commit
    # has to clear the Blocked chip.
    click(app, "[aria-label=Back]")
    try:
        app.wait("document.querySelector('.status')===null", 30)
        check("going back clears the blocked chip", True)
    except TimeoutError as e:
        check("going back clears the blocked chip", False, e)

    # A page on an allowed host can't reach a blocked one by its own requests either: its
    # images, fetches and frames there never leave the engine.
    visit(app, PAGES + "subresources")
    try:
        app.wait("document.title==='loaded'", 20, page=True)
        time.sleep(0.5)
        check("the page's request to an allowed host arrives", "127.0.0.1/img-allowed" in Pages.seen, Pages.seen[-5:])
        reached = [s for s in Pages.seen if s.startswith(BLOCKED_LOCAL)]
        check("its image, fetch and frame on a blocked host never arrive", reached == [], reached)
    except TimeoutError as e:
        check("the page with subresources on a blocked host loads", False, e)

    # Synchronous loads go through the same throttle, which content moves to another
    # thread for them; the page must live through it, as on StackBlitz's WebContainers.
    visit(app, PAGES + "sync-loads")
    try:
        app.wait("document.title==='xhr imported true'", 20, page=True)
        check("a cross-origin isolated page's synchronous XMLHttpRequest and worker importScripts load, and the page lives", True)
    except TimeoutError as e:
        check("a cross-origin isolated page's synchronous XMLHttpRequest and worker importScripts load, and the page lives", False, e)

    # The shell's own page never loads in a tab, however it's asked for: a copy there
    # would talk to Go as the shell. Content's other internal pages are let through.
    def typed(url):
        focus_bar(app)
        app.cmd("type", url)
        app.wait(f"{VALUE}==={json.dumps(url)}")
        app.cmd("key", "enter")

    status = "(document.querySelector('.status')||{}).innerText||''"
    visit(app, PAGES + "links")
    typed("chrome://shell/standard/")
    try:
        app.wait(f"{status}==='Blocked'", 10)
        check("typing chrome://shell/standard/ is refused by the gate", True)
    except TimeoutError as e:
        check("typing chrome://shell/standard/ is refused by the gate", False, e)
    # A refused load leaves the tab on content's error page, a moment after the chip shows.
    try:
        app.wait("location.href==='chrome-error://chromewebdata/'", 10, page=True)
        check("and the tab ends on the error page a refused load leaves, not on the shell's page", True)
    except TimeoutError as e:
        check("and the tab ends on the error page a refused load leaves, not on the shell's page", False, e)
    print(app.cmd("capture", os.path.join(OUT, "4-shell-refused.png")), flush=True)
    app.js("document.querySelector('a').click(), 1", page=True)
    time.sleep(1)
    landed = app.js("[location.href, typeof (window.chrome && chrome.send)]", page=True)
    check("a page's link to chrome://shell/standard/ is refused, and the tab has no bridge", landed[0] != "chrome://shell/standard/" and landed[1] == "undefined", (landed, app.js(status)))
    typed(PAGES + "redirect")
    time.sleep(2)
    landed = app.js("[location.href, typeof (window.chrome && chrome.send)]", page=True)
    check("a redirect to chrome://shell/standard/ is refused, and the tab has no bridge", landed[0] != "chrome://shell/standard/" and landed[1] == "undefined", landed)
    # Content serves chrome://gpu/ itself. The gate lets it through; whether it renders
    # depends on the resources the app bundles.
    typed("chrome://gpu/")
    time.sleep(2)
    check("typing an internal page content serves, chrome://gpu/, is not refused", app.js(status) != "Blocked" and app.js(VALUE) == "chrome://gpu/", (app.js(status), app.js(VALUE), app.js("location.href", page=True)))
    # Behind the gate, the engine gives the shell's bridge to the shell's own WebContents
    # only. A copy of the shell anywhere else, here in a WebContents no gate sees, gets no
    # chrome.send, and Go hears nothing from it.
    shell_messages = lambda: json.loads(app.cmd("stats", timeout=60))["go"]["hooks"].get("engine.ShellMessage", 0)
    before = shell_messages()
    stray = app.cmd("strayshell pages", timeout=30)
    time.sleep(1)
    check("a chrome://shell/standard/ page outside the shell gets no bridge", stray == "chrome://shell/standard/ undefined", stray)
    check("and Go receives no shell message from it", shell_messages() == before, (before, shell_messages()))
    # Our pages load only in their own context. In the profile's, where every tab is, the
    # shell's address is no page at all, even with no gate in the way.
    stray = app.cmd("strayshell tabs", timeout=30)
    check("chrome://shell/standard/ in the tabs' context doesn't load the shell: it ends on an error page", stray == "chrome-error://chromewebdata/ undefined", stray)
    click(app, "[aria-label='Tabs on top']")
    try:
        app.wait("!!document.querySelector('.strip')", 5)
        click(app, "[aria-label='Tabs on the side']")
        app.wait("!!document.querySelector('nav')", 5)
        check("after those attempts the shell still takes clicks: the layout toggle works", True)
    except TimeoutError as e:
        check("after those attempts the shell still takes clicks: the layout toggle works", False, e)
    time.sleep(0.3)
    visit(app, URL_A)

    # Key press to paint, measured inside the shell. Not a display measurement.
    focus_bar(app)
    app.js("""(()=>{window.__k=[];const i=document.querySelector('input');
      i.addEventListener('input',e=>{const t=e.timeStamp;requestAnimationFrame(()=>requestAnimationFrame(()=>window.__k.push(performance.now()-t)))});return 1})()""")
    for ch in "abcdefghijklmnopqrstuvwxyz":
        app.cmd("type", ch)
        time.sleep(0.06)
    time.sleep(0.2)
    samples = sorted(app.js("window.__k") or [])
    if samples:
        print(f"key to next frames: n={len(samples)} median={samples[len(samples)//2]:.1f}ms max={samples[-1]:.1f}ms", flush=True)
    leave_bar(app)

    # Resize: does the page follow in step with the shell?
    before = json.loads(app.cmd("stats", timeout=60))["ignoredStaleBounds"]
    results = []
    for w, h in ((1300, 800), (900, 560), (1100, 700)):
        app.cmd("resize", w, h)
        immediately = json.loads(app.cmd("bounds"))
        time.sleep(0.25)
        settled = json.loads(app.cmd("bounds"))
        results.append((w, h, immediately["frame"], settled["frame"], settled["reported"]))
        check(f"page frame is right straight after resize to {w}x{h}", immediately["frame"] == settled["frame"], (immediately, settled))
    print("resize", results, flush=True)
    for i in range(30):
        app.cmd("resize", 1000 + i * 7, 650 + i * 5)
    final_frame = json.loads(app.cmd("bounds"))
    # Where it ends, not how soon: headless Linux, drawing in software, catches up in about
    # half a second where the Mac takes a frame or two.
    right = lambda b: b["frame"] == [b["reported"][0], b["content"][1] - b["reported"][1] - b["reported"][3], b["reported"][2], b["reported"][3]] \
        and b["reported"][0] == 224 and b["reported"][2] == b["content"][0] - 228
    deadline = time.time() + 3
    time.sleep(0.3)
    settled = json.loads(app.cmd("bounds"))
    while not right(settled) and time.time() < deadline:
        time.sleep(0.1)
        settled = json.loads(app.cmd("bounds"))
    check("a burst of resizes ends in the right place", right(settled), settled)
    print("stale bounds ignored", json.loads(app.cmd("stats", timeout=60))["ignoredStaleBounds"] - before, flush=True)

    # The window controls stay put through resizes, so the shell's space for them does too.
    check_mac("the controls' space survives resizing", app.js("document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x") == window["buttonsEnd"] + 8)

    # The frame colour has one owner: Go gave it to the window and to the shell. A fresh
    # profile has none picked, so both paint None's grey for the theme they show
    # (core/tones).
    frame = app.js("getComputedStyle(document.documentElement).getPropertyValue('--frame').trim()")
    check("the window and the shell paint the frame colour Go owns", frame in ("#dcdee2", "#3a3f4a") and app.cmd("color") == frame, (frame, app.cmd("color")))
    check("the window already had the frame colour when it went on screen", app.cmd("showncolor") == frame, app.cmd("showncolor"))

    # The shell is not a page: nothing scrolls, and a trackpad swipe must not rubber-band it.
    root = app.js("(()=>{const s=getComputedStyle(document.documentElement);return [s.overflow,s.overscrollBehaviorY,document.scrollingElement.scrollHeight-window.innerHeight]})()")
    check("the shell document neither scrolls nor bounces", root == ["hidden", "none", 0], root)

    # The engine reads the window's corner radius from macOS and the shell's frame
    # follows it, one frame width inside.
    radius = lambda: float(app.js("getComputedStyle(document.documentElement).getPropertyValue('--window-radius')").replace("px", ""))
    native = lambda: json.loads(app.cmd("frame"))  # The border the window itself draws.
    windowed = radius()
    print("window corner radius", windowed, "native frame", native(), flush=True)
    check_mac("the window draws the frame's curve with its own radius and Go's width and colour", windowed > 0 and native() == {"radius": windowed, "width": 4, "color": frame}, (windowed, native()))

    # The shell asks Go's window manager, the manager tells the engine, and the engine
    # reports the result. Full screen is checked on a real screen: tools/check_full_screen.py.
    request = lambda r: app.js(f"(chrome.send('shell',[JSON.stringify({{t:'window',text:'{r}'}})]),1)")
    state = lambda: json.loads(app.cmd("state"))
    width = app.js("window.innerWidth")
    request("zoom")
    try:
        app.wait(f"window.innerWidth>{width}", 10)
        check("the shell's zoom request zooms the window", state()["zoomed"], state())
        request("zoom")
        app.wait(f"window.innerWidth==={width}", 10)
        check("and zooms it back", not state()["zoomed"], state())
    except TimeoutError as e:
        if LINUX and not os.environ.get("WAYLAND_DISPLAY") and not os.environ.get("DISPLAY"):
            print("skip the shell's zoom request zooms the window (headless: no window manager)", flush=True)
        else:
            check("the shell's zoom request zooms the window", False, f"no display? {e}")

    # Tabs. Tab 1 is on example.com here. Standard takes New Tab: Cmd+T goes to a new tab,
    # with its command box, which is the shell's; an address typed in the box with Enter
    # loads it there. Cmd+T again stays on that new tab.
    app.cmd("key", "cmd+t")
    check("Cmd+T opens a new tab with the command box, focused", holds(app, BOX_FOCUSED, 5) and app.js(ROWS) == 2, app.js(ROWS))
    bx, by = center(app, ".command-box input")
    deadline = time.time() + 2
    while (app.cmd("hit", bx, by), app.cmd("hit", 600, 300)) != ("shell", "shell") and time.time() < deadline:
        time.sleep(0.05)
    check("the box and the page area of the new tab are the shell's", app.cmd("hit", bx, by) == "shell" and app.cmd("hit", 600, 300) == "shell", (app.cmd("hit", bx, by), app.cmd("hit", 600, 300)))
    check("back and reload follow the new, empty tab", disabled("Back") and disabled("Reload"))
    app.cmd("key", "cmd+t")
    check("Cmd+T again stays on that new tab, and makes no other", holds(app, BOX_FOCUSED, 5) and app.js(ROWS) == 2, app.js(ROWS))
    from_box(app, URL_B)
    check("an address typed in the box with Enter loads it in the new tab", app.js(ROWS) == 2 and "iana.org" in app.js(VALUE), (app.js(ROWS), app.js(VALUE)))
    app.wait(f"{ACTIVE_ROW_TITLE}.includes('Example Domains')", 10)
    titles = app.js(ROW_TITLES)
    check("each tab keeps its own title", titles[0] == "Example Domain" and titles[1] == "Example Domains", titles)
    try:
        app.wait("document.querySelector('[role=tab]:nth-child(2) img')&&document.querySelector('[role=tab]:nth-child(2) img').naturalWidth>0", 15)
        check("a real favicon from the page shows on its row", True)
    except TimeoutError as e:
        check("a real favicon from the page shows on its row", False, e)
    check("a page without a favicon shows none", app.js("document.querySelector('[role=tab]:nth-child(1) img')===null"))
    app.js("(()=>{window.__vis=[];document.addEventListener('visibilitychange',()=>window.__vis.push(document.visibilityState));return 1})()", page=True)
    click(app, "[role=tab]")
    app.wait(f"{VALUE}==={json.dumps(URL_A)}", 10)
    app.wait("document.title==='Example Domain'", 10, page=True)
    check("clicking a row switches tabs: the address bar and the page follow", True)
    check("back follows the tab that has history", not disabled("Back"))
    click(app, "[role=tab]:nth-child(2)")
    app.wait("document.title==='Example Domains'", 10, page=True)
    vis = app.wait("window.__vis.length>=2&&window.__vis", 10, page=True)
    check("a hidden tab is told it is hidden, and shown again later", vis == ["hidden", "visible"], vis)
    time.sleep(0.3)
    print(app.cmd("capture", os.path.join(OUT, "5-tabs.png")), flush=True)

    # Pages opening tabs.
    app.js("window.open('https://example.com/?popup')", page=True)
    app.wait(f"{ROWS}===3", 15)
    app.wait(f"{VALUE}==='https://example.com/?popup'", 30)
    app.wait(TAB_READY, 30)
    check("window.open opens a new, active tab", app.js(ACTIVE_ROW_TITLE) == "Example Domain", app.js(ACTIVE_ROW_TITLE))
    app.js("(()=>{const a=document.createElement('a');a.href='https://example.com/?blank';a.target='_blank';a.textContent='x';document.body.appendChild(a);a.click();return 1})()", page=True)
    app.wait(f"{ROWS}===4", 15)
    app.wait(f"{VALUE}==='https://example.com/?blank'", 30)
    check("target=_blank opens a new, active tab", True)
    app.js("window.open('https://www.blocked.test/popup')", page=True)
    app.wait(f"{ROWS}===5", 15)
    try:
        app.wait("document.querySelector('.status')&&document.querySelector('.status').innerText==='Blocked'", 10)
        check("a tab a page opens goes through the gate", True)
    except TimeoutError as e:
        check("a tab a page opens goes through the gate", False, e)

    # Keys.
    app.cmd("key", "cmd+w")
    app.wait(f"{ROWS}===4", 10)
    check("Cmd+W closes the active tab and its left neighbour takes over at the end", app.js(VALUE) == "https://example.com/?blank", app.js(VALUE))
    app.cmd("key", "cmd+1")
    app.wait(f"{VALUE}==={json.dumps(URL_A)}", 10)
    check("Cmd+1 picks the first tab", app.js("document.querySelector('[role=tab]').getAttribute('aria-selected')") == "true")
    app.cmd("key", "ctrl+tab")
    app.wait(f"{VALUE}==='https://example.com/?blank'", 10)
    check("Ctrl+Tab picks the tab shown before this one", True)
    app.cmd("mods", 0)
    app.cmd("key", "ctrl+tab")
    app.wait(f"{VALUE}==={json.dumps(URL_A)}", 10)
    check("and, after letting go, Ctrl+Tab again returns", True)
    app.cmd("mods", 0)
    app.cmd("key", "cmd+9")
    app.wait(f"{VALUE}==='https://example.com/?blank'", 10)
    check("Cmd+9 picks the last tab", True)
    click(app, "[role=tab][aria-selected=true] .close")
    app.wait(f"{ROWS}===3", 10)
    check("the close button on a row closes that tab", "blank" not in app.js(VALUE), app.js(VALUE))

    # The sidebar collapses and the page follows.
    click(app, "[aria-label$=sidebar]")
    app.wait("document.querySelector('nav').getBoundingClientRect().width===0", 5)
    time.sleep(0.3)
    collapsed = json.loads(app.cmd("bounds"))
    check("collapsing the sidebar gives its width to the page", collapsed["reported"][0] == 4 and collapsed["frame"][0] == 4, collapsed)
    print(app.cmd("capture", os.path.join(OUT, "6-collapsed.png")), flush=True)
    click(app, "[aria-label$=sidebar]")
    app.wait("document.querySelector('nav').getBoundingClientRect().width===220", 5)
    time.sleep(0.3)
    check("expanding it takes the width back", json.loads(app.cmd("bounds"))["frame"][0] == 224, app.cmd("bounds"))

    # The same tabs on top. The strip is the title bar then; the toolbar row is below it.
    click(app, "[aria-label='Tabs on top']")
    app.wait("!!document.querySelector('.strip')", 5)
    time.sleep(0.4)
    horizontal = json.loads(app.cmd("bounds"))
    check("with the tabs on top the page spans the width below two rows", horizontal["reported"][0] == 4 and horizontal["reported"][1] == 92 and horizontal["frame"][0] == 4, horizontal)
    first_tab_x = app.js("document.querySelector('[role=tab]').getBoundingClientRect().x")
    check_mac("the strip leaves exactly the window buttons' space", first_tab_x == window["buttonsEnd"] + 8, (first_tab_x, window))
    check("a strip tab belongs to the shell", app.cmd("hit", first_tab_x + 20, BAR_Y) == "shell")
    strip_empty = app.cmd("hit", 1000, BAR_Y)
    check("the empty strip falls through to the frame, which drags", strip_empty == "frame", strip_empty)
    tool_x, tool_y = toolbar_gap(app)
    tool = app.cmd("hit", tool_x, tool_y)
    check("under the strip, the toolbar's gap falls through to the frame too", tool == "frame", (tool_x, tool_y, tool))
    check("a drag from the empty strip moves the window", moves_window(app, 1000, BAR_Y), app.cmd("press", 1000, BAR_Y))
    check("and a drag from the toolbar's gap under the strip moves it too, below the title bar", moves_window(app, tool_x, tool_y), (tool_x, tool_y, app.cmd("press", tool_x, tool_y)))
    left_x = app.js("(()=>{const r=document.querySelector('[aria-label=Reload]').getBoundingClientRect();return r.right+(document.querySelector('form.field').getBoundingClientRect().x-r.right)/2})()")
    check("and from the gap before the address field", moves_window(app, left_x, tool_y), (left_x, tool_y, app.cmd("press", left_x, tool_y)))
    field_x, field_y = center(app, "input")
    check("and the address field under the strip belongs to the shell", app.cmd("hit", field_x, field_y) == "shell", app.cmd("hit", field_x, field_y))
    look = app.js("(t => t && [getComputedStyle(t).backgroundColor, getComputedStyle(t).borderRadius])(document.querySelector('.strip .tab.active'))")
    check("the active tab is a flat row in the selected colour", bool(look) and look[1] == "6px" and look[0] != "rgba(0, 0, 0, 0)", look)
    n = app.js(ROWS)
    app.cmd("key", "cmd+t")
    check("Cmd+T works with the tabs on top: a new tab, with the command box", holds(app, BOX_FOCUSED, 5) and app.js(ROWS) == n + 1, app.js(ROWS))
    from_box(app, PAGES)
    # The shell re-reports its drag rectangles a frame after the strip changed. A click
    # inside that frame would still hit the old empty space.
    time.sleep(0.3)
    added_x, added_y = center(app, "[role=tab][aria-selected=true]")
    check("the tab the box added to the strip belongs to the shell", app.cmd("hit", added_x, added_y) == "shell", (added_x, added_y, app.cmd("hit", added_x, added_y)))
    click(app, "[role=tab][aria-selected=true] .close")
    app.wait(f"{ROWS}==={n}", 10)
    check("the close button on a strip tab closes it", True)
    click(app, "[role=tab]")
    app.wait(f"{VALUE}==={json.dumps(URL_A)}", 10)
    check("clicking a strip tab activates it", True)
    time.sleep(0.3)
    print(app.cmd("capture", os.path.join(OUT, "7-horizontal.png")), flush=True)
    click(app, "[aria-label='Tabs on the side']")
    app.wait("!!document.querySelector('nav')", 5)
    time.sleep(0.4)
    check("switching back brings the sidebar and the page follows", json.loads(app.cmd("bounds"))["frame"][0] == 224, app.cmd("bounds"))
    app.cmd("key", "cmd+1")
    app.wait(f"{VALUE}==={json.dumps(URL_A)}", 10)

    # Crash handling, from a healthy page: a blocked tab correctly keeps saying Blocked.
    visit(app, URL_A)
    app.cmd("crashpage")
    try:
        app.wait("document.querySelector('.status')&&document.querySelector('.status').innerText==='Crashed'", 10)
        check("a crashed page shows", True)
    except TimeoutError as e:
        check("a crashed page shows", False, e)
    try:
        visit(app, URL_A)
        check("a tab whose page crashed loads a page again", True)
    except RuntimeError as e:
        check("a tab whose page crashed loads a page again", False, e)
        raise
    app.cmd("crashshell")
    time.sleep(0.2)
    deadline = time.time() + 15
    recovered = False
    while time.time() < deadline:
        try:
            if app.js(f"{VALUE}.startsWith('https://example.com')"):
                recovered = True
                break
        except Exception:
            pass
        time.sleep(0.1)
    check("a crashed shell reloads and gets its state back", recovered)
    check("the page survived the shell crash", "Example" in app.js("document.title", page=True))

    # Processes.
    table = process_table(app.proc.pid)
    for row in table:
        print("process", json.dumps(row), flush=True)
    check("the main process is project_b", any(r["type"] == "main" and r["exe"] == "project_b" for r in table))
    children = [r for r in table if r["type"] != "main"]
    if LINUX:
        linux_binary_checks(app, children)
    else:
        mac_binary_checks(children)
    final = json.loads(app.cmd("stats", timeout=60))
    print("stats", json.dumps(final), flush=True)

    # Leave the tabs on top, so the restart below shows the choice was kept.
    click(app, "[aria-label='Tabs on top']")
    app.wait("!!document.querySelector('.strip')", 5)

    # Closing the tabs down to the last. After each close the shell puts focus back in the
    # next tab, so the next key waits for that.
    while app.js(ROWS) > 1:
        n = app.js(ROWS)
        app.cmd("key", "cmd+w")
        app.wait(f"{ROWS}==={n - 1}", 10)
        deadline = time.time() + 10
        while not (app.js("document.hasFocus()") or app.js("document.hasFocus()", page=True)):
            if time.time() > deadline:
                raise TimeoutError("nothing has focus after closing a tab")
            time.sleep(0.1)
    app.close()

    # History survives a restart.
    app = App(profile)
    time.sleep(1)
    focus_bar(app)
    app.cmd("type", "exam")
    try:
        app.wait(OPTIONS, 10)
        check("history survives a restart", True)
        check("titles survive a restart", app.js("[...document.querySelectorAll('[role=option][data-kind=history]')].some(o => o.innerText.includes('Example'))"))
    except TimeoutError as e:
        check("history survives a restart", False, e)
    app.cmd("key", "escape")
    check("the tab layout survives a restart", app.js("!!document.querySelector('.strip')"))
    click(app, "[aria-label='Tabs on the side']")
    app.wait("!!document.querySelector('nav')", 5)
    check("and can be switched back", app.js("!document.querySelector('.strip')"))
    app.close()


def visible_tabs_checks():
    """A window shows the tabs its layout lists, side by side if the layout says so: each
    view where its rectangle is, each told it is shown, and one that leaves the list told
    it is hidden. The shell lists one tab, so the driver gives the window the layout."""
    app = App(tempfile.mkdtemp(prefix="pb-profile-"))
    # The driver opens the two, as a page opens one, beside the window's new tab.
    first = open_in(app, "", PAGES)
    second = open_in(app, "", PAGES + "?second")
    app.wait(f"{ROWS}===3")
    deadline = time.time() + 10
    while [v["tab"] for v in json.loads(app.cmd("visible"))] != [second] and time.time() < deadline:
        time.sleep(0.05)
    check("the shell's layout lists the new tab, the one it shows", [v["tab"] for v in json.loads(app.cmd("visible"))] == [second])
    app.cmd("showtabs", first, 224, 48, 400, 600, second, 624, 48, 400, 600)
    visible = json.loads(app.cmd("visible"))
    check("a layout of two tabs puts both views in the window, each at its rectangle",
          [(v["tab"], v["frame"], v["hidden"]) for v in visible] == [(first, [224, 48, 400, 600], False), (second, [624, 48, 400, 600], False)], visible)
    check("and tells both they are shown", all(v["shown"] for v in visible), visible)
    check("both pages are visible to themselves",
          [json.loads(app.cmd("evaltab", t, "document.visibilityState")) for t in (first, second)] == ["visible", "visible"])
    check("a press goes to the page under it", app.cmd("hit", 400, 300) == "page" and app.cmd("hit", 800, 300) == "page")
    app.cmd("showtabs", second, 224, 48, 800, 600)
    deadline = time.time() + 10
    while json.loads(app.cmd("evaltab", first, "document.visibilityState")) != "hidden" and time.time() < deadline:
        time.sleep(0.05)
    check("a tab that leaves the list is told it is hidden", json.loads(app.cmd("evaltab", first, "document.visibilityState")) == "hidden")
    app.close()


def window_checks():
    """Several windows: Cmd+N makes a second window with its own shell, a little down and
    right of the first, on a new tab with Standard's command box; a command acts on the
    window in front; a second start's URL opens in the window used last; closing a window's
    last tab closes it, and closing the last window quits. The driver
    makes a window the key window, as a click on it would on a real desktop."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = App(profile)
    windows = lambda: json.loads(app.cmd("windows"))
    # The first window starts on a new tab, which loads what its command box takes.
    tab = from_box(app, PAGES)
    first = windows()[0]
    app.cmd("key", "cmd+n")
    deadline = time.time() + 10
    while len(windows()) < 2 and time.time() < deadline:
        time.sleep(0.05)
    both = windows()
    check("Cmd+N makes a second window", [w["id"] for w in both] == [1, 2], both)
    second = both[-1]
    fx, fy, fw, fh = first["frame"]
    sx, sy, sw, sh = second["frame"]
    check("it opens a little down and right of the first, the same size",
          0 < sx - fx <= 60 and 0 < sy - fy <= 60 and (sw, sh) == (fw, fh), (first["frame"], second["frame"]))
    file = next(m for m in json.loads(app.cmd("menu")) if m["title"] == "File")
    check("the File menu has New Window on Cmd+N", {"title": "New Window", "key": CMD + "n"} in
          [{"title": i["title"], "key": i["key"]} for i in file["items"] if i != "-"], file["items"])

    app.cmd("window", 2)
    app.wait("!!document.querySelector('input')", 30)
    check("the second window has a shell of its own, on a new tab with its command box focused", holds(app, f"{ROWS}===1&&{VALUE}===''&&{BOX_FOCUSED}"))
    check("each window shows its own tabs, the second no page, on its new tab", [w["tabs"] for w in windows()] == [[tab], []], windows())

    # A command acts on the window in front: New Tab there, and an address in its box.
    new_tab(app, PAGES + "second")
    app.cmd("window", 1)
    check("Cmd+T in the second window went to its new tab, the first keeping its one", app.js(f"{ROWS}") == 1)

    # The first window is the one used last now: a second start's URL opens there.
    started = subprocess.run([BIN, f"--user-data-dir={profile}", PAGES + "clicks"], timeout=60, capture_output=True)
    check("a second start on the profile hands over and exits", started.returncode == 0, started.returncode)
    app.wait(f"{ROWS}===2&&{VALUE}==={json.dumps(PAGES + 'clicks')}", 20)
    check("its URL opens in a new tab in the window used last", True)
    app.cmd("window", 2)
    check("and not in the other window", app.js(f"{ROWS}") == 1)

    # Closing the second window's last tab closes it, and the app goes on.
    app.cmd("key", "cmd+w")
    deadline = time.time() + 10
    while len(windows()) > 1 and time.time() < deadline:
        time.sleep(0.05)
    check("closing a window's last tab closes it", [w["id"] for w in windows()] == [1], windows())
    app.cmd("window", 1)
    check("the other window goes on with its tabs", app.js(f"{ROWS}") == 2)

    # Closing the last window quits.
    app.cmd("key", "cmd+shift+w")
    try:
        code = app.proc.wait(timeout=20)
    except subprocess.TimeoutExpired:
        code = None
    check("closing the last window quits, cleanly", code == 0, code)
    App.running.remove(app)


def last_tab_checks():
    """A window always has a tab: it opens on a new tab, which loads nothing, with Standard's
    command box where the page goes; an address typed in the field loads in it. Closing a
    window's last tab closes it, which for the last window ends the app."""
    app = App(tempfile.mkdtemp(prefix="pb-profile-"))
    # An address typed in the field loads in the new tab, whose page shows, and the box goes.
    holds(app, BOX_FOCUSED, 10)
    focus_bar(app)
    app.cmd("type", PAGES)
    app.wait(f"{VALUE}==={json.dumps(PAGES)}")
    app.cmd("key", "enter")
    deadline = time.time() + 10
    while not (json.loads(app.cmd("visible")) and not app.js(f"!!{BOX}")) and time.time() < deadline:
        time.sleep(0.05)
    check("an address typed in the field of the new tab loads it there, and the box goes",
          bool(json.loads(app.cmd("visible"))) and not app.js(f"!!{BOX}") and app.js(ROWS) == 1, (app.cmd("visible"), app.js(f"!!{BOX}"), app.js(ROWS)))
    print(app.cmd("capture", os.path.join(OUT, "empty-field-opens.png")), flush=True)
    app.cmd("key", "cmd+w")
    try:
        code = app.proc.wait(timeout=20)
    except subprocess.TimeoutExpired:
        code = None
    check("Cmd+W on the last tab closes the window, which ends the app, cleanly", code == 0, code)
    if code is None:
        app.close()
    else:
        App.running.remove(app)


def move_checks():
    """A tab moving to another window keeps its id and its page: nothing reloads, a form
    keeps what was typed, a timer goes on, and the window it arrives in shows it. A tab can
    move into a new window of its own, and a window whose last tab leaves closes. The move
    is Go's, asked for through our own DevTools command."""
    app = App(tempfile.mkdtemp(prefix="pb-profile-"))
    windows = lambda: json.loads(app.cmd("windows"))
    move = lambda tab, window: json.loads(app.cmd("projectb", "moveTab", json.dumps({"tab": tab, "window": window, "index": 0})))
    # A window opens on a new tab: the first window's loads what is typed in its address
    # field, the driver opens another, and the second window's new tab loads what its
    # command box takes.
    visit(app, "data:text/html,<title>Kept</title><input id=f><script>n=0;setInterval(()=>n++,20)</script>")
    moved = tabs_of(app)["active"]
    app.js("document.getElementById('f').value='typed';window.kept=42", page=True)
    stays = open_in(app, "", PAGES)
    app.wait(f"{ROWS}===2")
    app.cmd("key", "cmd+n")
    deadline = time.time() + 10
    while len(windows()) < 2 and time.time() < deadline:
        time.sleep(0.05)
    app.cmd("window", 2)
    own = from_box(app, PAGES + "own")
    app.cmd("window", 1)

    move(moved, 2)
    deadline = time.time() + 10
    while moved not in windows()[1]["tabs"] and time.time() < deadline:
        time.sleep(0.05)
    check("a tab moved to another window shows there", [w["tabs"] for w in windows()] == [[stays], [moved]], windows())
    kept = json.loads(app.cmd("evaltab", moved, "[window.kept, document.getElementById('f').value, n]"))
    check("its page didn't reload: what the page and the form held is there", kept[:2] == [42, "typed"], kept)
    deadline = time.time() + 5
    later = kept[2]
    while later <= kept[2] and time.time() < deadline:
        later = json.loads(app.cmd("evaltab", moved, "n"))
    check("and it goes on running", later > kept[2], (kept[2], later))
    app.cmd("window", 2)
    visible = json.loads(app.cmd("visible"))
    check("the window it arrived in shows its view and tells it it is shown",
          [(v["tab"], v["hidden"], v["shown"]) for v in visible] == [(moved, False, True)], visible)
    check("both shells list their tabs: two in the window it arrived in, the moved one active",
          app.js(f"{ROWS}") == 2 and app.js(ACTIVE_ROW_TITLE) == "Kept", app.js(ACTIVE_ROW_TITLE))
    app.cmd("window", 1)
    check("and one in the window it left", app.js(f"{ROWS}") == 1)

    # Into a new window of its own, which starts with it and no tab of its own.
    move(own, 0)
    deadline = time.time() + 10
    while [w["tabs"] for w in windows()][2:] != [[own]] and time.time() < deadline:
        time.sleep(0.05)
    check("a tab moved into a new window is that window's only tab", [w["tabs"] for w in windows()][2:] == [[own]] and
          [w["id"] for w in windows()] == [1, 2, 3], windows())

    # The first window's last tab leaves, and it closes.
    move(stays, 3)
    deadline = time.time() + 10
    while len(windows()) > 2 and time.time() < deadline:
        time.sleep(0.05)
    check("a window whose last tab moves away closes", [w["id"] for w in windows()] == [2, 3], windows())
    app.cmd("window", 2)
    press_quit(app)
    check("Cmd+Q ends the app with both windows, cleanly", app.proc.wait(timeout=20) == 0)
    App.running.remove(app)


def bounds_checks():
    """Go puts a window's frame where it is asked, and reads back where it is, as the engine
    reports it."""
    app = App(tempfile.mkdtemp(prefix="pb-profile-"))
    want = {"x": 200, "y": 150, "w": 900, "h": 600}
    app.cmd("projectb", "setWindowBounds", json.dumps({"window": 1, "bounds": want}))
    frame = json.loads(app.cmd("windows"))[0]["frame"]
    check("a window's frame goes where Go asks", frame == [200, 150, 900, 600], frame)
    deadline = time.time() + 10
    seen = None
    while seen != want and time.time() < deadline:
        seen = json.loads(app.cmd("projectb", "getWindows", "{}"))["result"]["windows"][0]["bounds"]
    check("and Go reads back where it is", seen == want, seen)
    app.close()


def profile_checks():
    """The start chooses the profile in Go, from the list in a root of its own here: a first
    start makes the list holding Personal in default, from the Personal template, a listed profile is named by its id,
    and --user-data-dir is a folder used as it is, which never enters the list."""
    root = tempfile.mkdtemp(prefix="pb-root-")
    listed = lambda: json.load(open(os.path.join(root, "profiles.json")))["profiles"]
    app = App(None, [f"--root={root}"])
    app.wait("!!document.querySelector('input')", 30)
    first = listed()
    check("a first start makes the list holding Personal, in default",
          [(p["id"], p["name"]) for p in first] == [("default", "Personal")], first)
    check("and runs on that folder", os.path.exists(os.path.join(root, "default", "timeline.log")), os.listdir(root))
    made = json.load(open(os.path.join(root, "default", "prefs.json")))
    check("made from the Personal template: its prefs are Personal's, and the list says so",
          made == {"tabLayout": "vertical", "frameColor": "", "theme": "system", "shell": "standard"} and first[0]["template"] == "Personal", (made, first))
    check("the list records when the profile was used last", first[0].get("lastUsed", "") != "", first)
    started = subprocess.run([BIN, f"--root={root}", "--profile=default", PAGES + "clicks"], timeout=60, capture_output=True)
    check("a start naming the running profile by its id hands over and exits", started.returncode == 0, started.returncode)
    # The URL's tab opens beside the new tab the window started on.
    app.wait(f"{ROWS}===2&&{VALUE}==={json.dumps(PAGES + 'clicks')}", 20)
    check("and its URL opens in the running copy", True)
    app.close()
    started = subprocess.run([BIN, f"--root={root}", "--profile=nobody"], timeout=60, capture_output=True, text=True)
    check("a profile that isn't listed is refused", started.returncode == 1 and "no profile \"nobody\"" in started.stderr, (started.returncode, started.stderr[-300:]))
    other = tempfile.mkdtemp(prefix="pb-root-")
    app = App(tempfile.mkdtemp(prefix="pb-profile-"), [f"--root={other}"])
    app.wait("!!document.querySelector('input')", 30)
    check("a folder named with --user-data-dir leaves the root alone", os.listdir(other) == [], os.listdir(other))
    app.close()


def add_work_profile(root):
    """Lists a second profile, work, made from the Work template, as New Profile makes one."""
    path = os.path.join(root, "profiles.json")
    listed = json.load(open(path))
    listed["profiles"].append({"id": "work", "name": "Work", "color": "#5b7fd6", "template": "Work"})
    with open(path, "w") as f:
        json.dump(listed, f)
    os.makedirs(os.path.join(root, "work"))
    with open(os.path.join(root, "work", "prefs.json"), "w") as f:
        json.dump({"tabLayout": "horizontal", "frameColor": "#5b7fd6", "theme": "system"}, f)


def copies_on(root, profile):
    """The pids of the copies running on a listed profile of root that the app started,
    not the driver."""
    out = subprocess.run(["pgrep", "-f", "--", f"--root={root} --profile={profile}"], capture_output=True, text=True).stdout
    return [p for p in out.split() if int(p) not in {a.proc.pid for a in App.running}]


def copies_checks():
    """Two copies on two profiles of one root: each is an app of its own, with its profile's
    colour in a dot on its Dock icon. The Profiles menu in each lists both, with a tick on its
    own. Choosing a running profile starts a copy that hands over to it; choosing a stopped
    one starts it. Quit ends one profile's copy only."""
    root = tempfile.mkdtemp(prefix="pb-root-")
    a = App(None, [f"--root={root}"])
    a.wait("!!document.querySelector('input')", 30)
    add_work_profile(root)
    b = App(None, [f"--root={root}", "--profile=work"])
    b.wait("!!document.querySelector('input')", 30)
    check("two profiles run at once, each in a copy of its own", a.proc.poll() is None and b.proc.poll() is None and a.proc.pid != b.proc.pid)
    # On a desktop the copy started second came to the front, and the OS took the key
    # window from the first. The driver holds the first's key, as a click on it makes it,
    # so a click in its address bar still gives its shell the keyboard.
    try:
        focus_bar(a)
        check("a click in the address bar of the copy behind gives it the keyboard", True)
    except TimeoutError as e:
        check("a click in the address bar of the copy behind gives it the keyboard", False, e)
    # Personal has no colour picked, so its icon has no dot: where it would be shows the
    # icon's own artwork, as the picker's does.
    check("each copy's Dock icon has a dot in its profile's colour, and none with no colour", b.cmd("dockdot") == "#5b7fd6" and a.cmd("dockdot") != "#5b7fd6", (a.cmd("dockdot"), b.cmd("dockdot")))
    shown = lambda app: [(i["title"], i["checked"]) for m in json.loads(app.cmd("menu")) if m["title"] == "Profiles" for i in m["items"] if i != "-"]
    check("the Profiles menu lists both, with a tick on this one", shown(a) == [("Personal", True), ("Work", False), ("Profiles…", False)] and shown(b) == [("Personal", False), ("Work", True), ("Profiles…", False)], (shown(a), shown(b)))
    handed = lambda: [e for e in json.loads(b.cmd("timeline"))["result"]["entries"] if "handed over" in e.get("text", "")]
    before = len(handed())
    check("choosing Work in the other copy's menu works", a.cmd("choosemenu", "Profiles", "Work") == "ok")
    deadline = time.time() + 30
    while len(handed()) == before and time.time() < deadline:
        time.sleep(0.1)
    check("it starts a copy on Work, which hands over to the copy running there", len(handed()) == before + 1, handed())
    while len(copies_on(root, "work")) > 0 and time.time() < deadline:
        time.sleep(0.1)
    check("and that copy quits", copies_on(root, "work") == [], copies_on(root, "work"))
    b.close()
    a.cmd("choosemenu", "Profiles", "Work")
    deadline = time.time() + 30
    while not copies_on(root, "work") and time.time() < deadline:
        time.sleep(0.1)
    started = copies_on(root, "work")
    check("choosing a profile that isn't running starts a copy on it", len(started) == 1, started)
    for pid in started:
        os.kill(int(pid), signal.SIGTERM)
    while copies_on(root, "work") and time.time() < deadline:
        time.sleep(0.1)
    b = App(None, [f"--root={root}", "--profile=work"])
    b.wait("!!document.querySelector('input')", 30)
    press_quit(a)
    check("Quit ends this profile's copy only", a.proc.wait(timeout=20) == 0 and b.proc.poll() is None)
    App.running.remove(a)
    b.close()


def picker_checks():
    """A copy started to pick shows the picker page in one window, with the plain Dock icon.
    Choosing a profile opens it in a copy of its own, which hands over to the copy already
    running there, if any, and the picker copy quits. New Profile makes one from a template
    and opens it the same way. Profiles… in the menu starts a picker copy, and the start
    shows the picker by itself when no profile was used yet."""
    root = tempfile.mkdtemp(prefix="pb-root-")
    first = App(None, [f"--root={root}"])
    first.wait("!!document.querySelector('input')", 30)
    first.close()
    listed = lambda: json.load(open(os.path.join(root, "profiles.json")))["profiles"]
    rows = "[...document.querySelectorAll('.row')].map(r => r.textContent.trim())"

    def started_on(profile):
        """Waits for the copy a picker started on profile, and gives its pids."""
        deadline = time.time() + 30
        while not copies_on(root, profile) and time.time() < deadline:
            time.sleep(0.1)
        return copies_on(root, profile)

    def stop(pids, profile):
        for pid in pids:
            os.kill(int(pid), signal.SIGTERM)
        deadline = time.time() + 30
        while copies_on(root, profile) and time.time() < deadline:
            time.sleep(0.1)

    picker = App(None, [f"--root={root}", "--picker"])
    picker.wait(f"{rows}.length===1", 30)
    check("a copy started to pick shows the picker page", picker.js("location.href") == "chrome://picker/" and picker.js(rows) == ["Personal"], picker.js("location.href"))
    plain = picker.cmd("dockdot")
    check("its Dock icon has no profile's dot", plain != "#5b7fd6", plain)
    used = listed()[0]["lastUsed"]
    picker.js("document.querySelector('.row').click(), 1")
    check("choosing a profile quits the picker copy", picker.proc.wait(timeout=30) == 0)
    App.running.remove(picker)
    opened = started_on("default")
    check("and opens the profile in a copy of its own", len(opened) == 1, opened)
    deadline = time.time() + 30
    while listed()[0]["lastUsed"] == used and time.time() < deadline:
        time.sleep(0.1)
    check("which starts on it: the list says it was used now", listed()[0]["lastUsed"] != used, (used, listed()[0]["lastUsed"]))
    stop(opened, "default")

    app = App(None, [f"--root={root}", "--profile=default"])
    app.wait("!!document.querySelector('input')", 30)
    profiles_menu = [(i["title"], i["checked"]) for m in json.loads(app.cmd("menu")) if m["title"] == "Profiles" for i in m["items"] if i != "-"]
    check("a profile's Profiles menu has Profiles… under the list", profiles_menu == [("Personal", True), ("Profiles…", False)], profiles_menu)
    handed = lambda: [e for e in json.loads(app.cmd("timeline"))["result"]["entries"] if "handed over" in e.get("text", "")]
    before = len(handed())
    other = App(None, [f"--root={root}", "--picker"])
    other.wait(f"{rows}.length===1", 30)
    other.js("document.querySelector('.row').click(), 1")
    check("choosing a profile a copy runs on quits the picker copy", other.proc.wait(timeout=30) == 0)
    App.running.remove(other)
    deadline = time.time() + 30
    while len(handed()) == before and time.time() < deadline:
        time.sleep(0.1)
    check("and the copy it started hands over to the running one", len(handed()) == before + 1, handed())

    made = App(None, [f"--root={root}", "--picker"])
    made.wait(f"{rows}.length===1", 30)
    made.js("[...document.querySelectorAll('button')].find(b => b.textContent.trim()==='New Profile').click(), 1")
    made.wait("!!document.querySelector('select')")
    made.js("(() => { const i = document.querySelector('input[aria-label=Name]'); i.value = 'Office'; i.dispatchEvent(new Event('input')); const s = document.querySelector('select'); s.value = 'Work'; s.dispatchEvent(new Event('change')); return 1 })()")
    made.js("document.querySelector('form').requestSubmit(), 1")
    check("New Profile quits the picker copy", made.proc.wait(timeout=30) == 0)
    App.running.remove(made)
    office = [p for p in listed() if p["name"] == "Office"]
    check("having made the profile from the template", len(office) == 1 and office[0]["template"] == "Work" and office[0]["color"] == "#5b7fd6", listed())
    opened = started_on(office[0]["id"]) if office else []
    check("and opened it in a copy of its own", len(opened) == 1, opened)
    stop(opened, office[0]["id"] if office else "")

    check("Profiles… in the menu works", app.cmd("choosemenu", "Profiles", "Profiles…") == "ok")
    deadline = time.time() + 30
    while not copies_picking(root) and time.time() < deadline:
        time.sleep(0.1)
    picking = copies_picking(root)
    check("and starts a copy that picks", len(picking) == 1, picking)
    for pid in picking:
        os.kill(int(pid), signal.SIGTERM)
    while copies_picking(root) and time.time() < deadline:
        time.sleep(0.1)
    app.close()

    fresh = tempfile.mkdtemp(prefix="pb-root-")
    with open(os.path.join(fresh, "profiles.json"), "w") as f:
        json.dump({"profiles": [{"id": "default", "name": "Personal", "color": "#4f9e9e", "template": "Personal"}]}, f)
    app = App(None, [f"--root={fresh}"])
    app.wait(f"{rows}.length===1", 30)
    check("with no profile named and none used yet, the start shows the picker", app.js("location.href") == "chrome://picker/")
    press_quit(app)
    check("and Quit there ends it, cleanly", app.proc.wait(timeout=20) == 0)
    App.running.remove(app)


def copies_picking(root):
    """The pids of the picker copies on root that the app started, not the driver."""
    out = subprocess.run(["pgrep", "-f", "--", f"--root={root} --picker"], capture_output=True, text=True).stdout
    return [p for p in out.split() if int(p) not in {a.proc.pid for a in App.running}]


def relaunch_checks():
    """Relaunch to update, with copies on two profiles of one root and a picker copy: every
    copy reads the release staged in the root's update folder and offers it; relaunching
    one tells the others to quit, the picker among them, swaps the installed app once they
    have ended, and starts each profile that was running again on the new release. The
    installed app here is a clone of the one under test in a folder of the run's own, and
    the staged release is a bundle like the update tests' fixtures (core/update): an
    Info.plist and an executable that records how it was started. Automatic checks are off,
    so no copy asks the release manifest."""
    if bundle_version() is None:
        check("relaunch to update needs the app in its bundle (PB_BIN)", False, BIN)
        return
    bundle = os.path.dirname(os.path.dirname(os.path.dirname(BIN)))
    installed = os.path.join(tempfile.mkdtemp(prefix="pb-installed-"), os.path.basename(bundle))
    subprocess.run(["cp", "-cR", bundle, installed], check=True)
    clone = os.path.join(installed, "Contents/MacOS", os.path.basename(BIN))
    root = tempfile.mkdtemp(prefix="pb-root-")
    started = os.path.join(root, "started")
    staged = os.path.join(root, "update/stage", os.path.basename(bundle))
    os.makedirs(os.path.join(staged, "Contents/MacOS"))
    with open(os.path.join(staged, "Contents/Info.plist"), "w") as f:
        f.write('<?xml version="1.0" encoding="UTF-8"?>\n<plist version="1.0"><dict>'
                "<key>CFBundleExecutable</key><string>project_b</string>"
                "<key>CFBundleShortVersionString</key><string>99.0.0</string></dict></plist>\n")
    with open(os.path.join(staged, "Contents/MacOS/project_b"), "w") as f:
        f.write(f"#!/bin/sh\necho \"$*\" >> '{started}'\n")
    os.chmod(os.path.join(staged, "Contents/MacOS/project_b"), 0o755)
    os.makedirs(os.path.join(root, "update/amarit"), exist_ok=True)
    with open(os.path.join(root, "update/amarit/settings.json"), "w") as f:
        json.dump({"auto_update": False}, f)

    a = App(None, [f"--root={root}"], clone)
    add_work_profile(root)
    b = App(None, [f"--root={root}", "--profile=work"], clone)
    picker = App(None, [f"--root={root}", "--picker"], clone)
    # A ready release's one-time notice (docs/updates.md), whose Relaunch is the bubble's.
    offer = "!!document.querySelector('.update-notice')?.textContent.includes('99.0.0 is ready')"
    check("each profile's copy offers the release staged in the root's update folder", bool(a.wait(offer, 20)) and bool(b.wait(offer, 20)))
    a.js("document.querySelector('.update-notice .primary').click(), 1")
    check("relaunching one tells the other profile's copy to quit", b.proc.wait(timeout=30) == 0)
    check("and the picker copy", picker.proc.wait(timeout=30) == 0)
    check("and the copy asked quits once they have ended", a.proc.wait(timeout=30) == 0)
    for app in (a, b, picker):
        App.running.remove(app)
    version = lambda app: re.search(r"CFBundleShortVersionString</key>\s*<string>([^<]+)", open(os.path.join(app, "Contents/Info.plist")).read()).group(1)
    check("the new release is in place, the previous one kept in the root's update folder",
          version(installed) == "99.0.0" and version(os.path.join(root, "update/previous", os.path.basename(bundle))) == bundle_version())
    deadline = time.time() + 30
    lines = lambda: sorted(open(started).read().splitlines()) if os.path.exists(started) else []
    while len(lines()) < 2 and time.time() < deadline:
        time.sleep(0.1)
    check("each profile that was running starts again on it, and the picker doesn't",
          lines() == [f"--root={root} --profile=default", f"--root={root} --profile=work"], lines())
    shutil.rmtree(os.path.dirname(installed))


STORE = PAGES + "store"


def open_in(app, workspace, url):
    """Opens url in a new tab in a workspace, by its id, as the driver can without Go's
    list, and returns Go's id for the tab once its page has loaded."""
    tab = int(app.cmd("opentab", workspace, url))
    app.wait(f"document.readyState==='complete'&&location.href==={json.dumps(url)}", tab=tab)
    return tab


def opened_by(app, opener, script):
    """Runs script in opener's page, which opens one tab on the store page in front, and
    returns Go's id for that tab once its page has loaded."""
    rows = app.js(ROWS)
    app.js(script, tab=opener)
    app.wait(f"{ROWS}==={rows + 1}")
    tab = max(t["id"] for t in json.loads(app.cmd("projectb", "getTabs", "{}"))["result"]["tabs"])
    app.wait("document.readyState==='complete'&&location.pathname==='/store'", tab=tab)
    return tab


def stored(app, tab):
    """The cookie and the local storage value the workspace checks set, as a tab's page
    sees them."""
    return app.js("[document.cookie, localStorage.getItem('w')]", tab=tab)


def databases(app, tab):
    """The names of the IndexedDB databases a tab's page sees."""
    app.js("indexedDB.databases().then(d=>document.title='databases:'+d.map(x=>x.name).join())", tab=tab)
    return app.wait("document.title.startsWith('databases:')&&document.title", tab=tab)[len("databases:"):]


def committed(app, tab, times=1):
    """Waits for a tab's page to have committed a navigation so many times, as the engine
    marks each on the timeline."""
    deadline = time.time() + 20
    while len([e for e in json.loads(app.cmd("timeline"))["result"]["entries"] if e.get("name") == "tab.committed" and e.get("text") == f"tab={tab}"]) < times:
        if time.time() > deadline:
            raise TimeoutError(f"tab {tab} never committed")
        time.sleep(0.05)


def attach(profile, url):
    """A DevTools connection to the page at url, through the port the app was started with."""
    port = open(os.path.join(profile, "DevToolsActivePort")).read().split()[0]
    target = next(t for t in json.load(urllib.request.urlopen(f"http://127.0.0.1:{port}/json/list")) if t["type"] == "page" and t["url"] == url)
    return ws_open(target["webSocketDebuggerUrl"])[0]


def workspace_checks():
    """A workspace keeps its own cookies and site storage. What a page in workspace a sets,
    a cookie, a local storage value and an IndexedDB database, a second tab of a sees, and
    workspace b and Main, the profile's own storage, don't; a restart on the same profile
    keeps it. A tab a page in a opens is in a too: by window.open, with its opener or
    without, by a link opened in a new tab, and by a target=_blank or rel=noopener link,
    which content would put in the profile's own storage. The driver opens a tab in a
    workspace by its id, as Go does."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = App(profile, ["--remote-debugging-port=0"])
    a = open_in(app, "a", STORE)
    app.js("document.cookie='w=a; max-age=3600';localStorage.setItem('w','a');indexedDB.open('w').onsuccess=()=>document.title='stored'", tab=a)
    app.wait("document.title==='stored'", tab=a)
    second = open_in(app, "a", STORE)
    check("a second tab of a workspace sees its cookie and local storage", stored(app, second) == ["w=a", "a"], stored(app, second))
    b = open_in(app, "b", STORE)
    check("a tab of another workspace sees neither", stored(app, b) == ["", None], stored(app, b))
    # Main's is opened here, in the profile's own storage.
    main = open_in(app, "", STORE)
    check("nor does a tab of Main", stored(app, main) == ["", None], stored(app, main))
    check("an IndexedDB database made in a workspace is seen there, and not in another or Main",
          (databases(app, second), databases(app, b), databases(app, main)) == ("w", "", ""))
    # The blank page in a workspace tab, reached by every way but the one Go loads it by:
    # a reload, history, and DevTools. Content would stop the app on the next navigation
    # if it left the tab without a site there. Go reloads no blank tab, so DevTools does.
    reloaded = int(app.cmd("opentab", "a", "about:blank"))
    committed(app, reloaded)
    s = attach(profile, "about:blank")
    cdp(s, 1, "Page.reload")
    s.close()
    committed(app, reloaded, 2)
    visit(app, STORE)
    check("a workspace tab reloaded on the blank page goes on in the workspace", stored(app, reloaded) == ["w=a", "a"], stored(app, reloaded))
    went_back = int(app.cmd("opentab", "a", "about:blank"))
    committed(app, went_back)
    visit(app, STORE)
    app.cmd("key", "cmd+[")
    app.wait("location.href==='about:blank'&&document.readyState==='complete'", tab=went_back)
    app.cmd("key", "cmd+]")
    app.wait("location.pathname==='/store'&&document.readyState==='complete'", tab=went_back)
    check("so does one gone back to the blank page and forward again", stored(app, went_back) == ["w=a", "a"], stored(app, went_back))
    attached = open_in(app, "a", STORE + "?devtools")
    s = attach(profile, STORE + "?devtools")
    cdp(s, 1, "Page.navigate", {"url": "about:blank"})
    app.wait("location.href==='about:blank'&&document.readyState==='complete'", tab=attached)
    cdp(s, 2, "Page.navigate", {"url": STORE})
    app.wait("location.href===" + json.dumps(STORE) + "&&document.readyState==='complete'", tab=attached)
    s.close()
    check("and one DevTools takes to the blank page and on to another, with the app still running",
          app.proc.poll() is None and stored(app, attached) == ["w=a", "a"], stored(app, attached))
    popup = opened_by(app, a, f"window.open({json.dumps(STORE)})")
    check("a tab a page opens with window.open is in its workspace", stored(app, popup) == ["w=a", "a"], stored(app, popup))
    linked = opened_by(app, a, "document.getElementById('link').dispatchEvent(new MouseEvent('click',{metaKey:true,bubbles:true,cancelable:true}))")
    check("so is a link it opens in a new tab", stored(app, linked) == ["w=a", "a"], stored(app, linked))
    blank = int(app.cmd("opentab", "a", "about:blank"))
    committed(app, blank)
    visit(app, STORE)
    check("a tab of a workspace on the blank page, as a new tab is, goes on in the workspace", stored(app, blank) == ["w=a", "a"], stored(app, blank))
    targeted = opened_by(app, a, "document.getElementById('blank').click()")
    check("and a target=_blank link's tab, which has no opener",
          stored(app, targeted) == ["w=a", "a"] and app.js("window.opener===null", tab=targeted), stored(app, targeted))
    check("whose session storage is its own, in the workspace",
          app.js("sessionStorage.setItem('s','b');sessionStorage.getItem('s')", tab=targeted) == "b" and app.js("sessionStorage.getItem('s')", tab=a) is None)
    noopener = opened_by(app, a, "document.getElementById('noopener').click()")
    check("and a rel=noopener link's", stored(app, noopener) == ["w=a", "a"], stored(app, noopener))
    severed = opened_by(app, a, f"window.open({json.dumps(STORE)},'','noopener')")
    check("and a tab window.open opens without its opener", stored(app, severed) == ["w=a", "a"], stored(app, severed))
    app.close()
    check("a workspace's storage is in the profile's folder", os.path.isdir(os.path.join(profile, "chromium/Storage/ext/workspace")))
    app = App(profile)
    check("a workspace's cookie and local storage are there after a restart", stored(app, open_in(app, "a", STORE)) == ["w=a", "a"])
    check("and still only there", stored(app, open_in(app, "b", STORE)) == ["", None])
    app.close()


def tabs_of(app):
    """Go's tabs and the active one, as ProjectB.getTabs answers."""
    return json.loads(app.cmd("projectb", "getTabs", "{}"))["result"]


def action_checks():
    """Each everyday action, run by its id or message as a menu, key or tab menu runs it, lands
    on a page with no error: Duplicate Tab once ended in ERR_BLOCKED_BY_CLIENT on the real
    engine while the mock engine passed it (docs/action-coverage.md). A tab landed is Go's
    tab, done loading, on the address wanted, not blocked, crashed or failed."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    with open(os.path.join(profile, "workspaces.json"), "w") as f:
        json.dump({"workspaces": [{"id": "0a1b2c3d", "name": "Work", "color": "#3470d0"}]}, f)
    app = App(profile)
    send = lambda msg: app.js(f"chrome.send('shell', [{json.dumps(json.dumps(msg))}]), 1")
    run = lambda action: send({"t": "run", "text": action})

    def tab(tid):
        return next((t for t in tabs_of(app)["tabs"] if t["id"] == tid), None)

    def landed(want_url, new_tab=None, before=None):
        """The active tab, or a new one, once on want_url and done, and what is wrong with it."""
        deadline = time.time() + 20
        while time.time() < deadline:
            r = tabs_of(app)
            t = tab(r["active"])
            if new_tab and before is not None and r["active"] in before:
                t = None
            if t and t["url"] == want_url and not t["loading"]:
                # Settled: Go sets a copy's address from the original before its own load, so a
                # blocked load shows only as its error, or a title that never comes
                # (Duplicate Tab's ERR_BLOCKED_BY_CLIENT, with an empty title).
                settle = time.time() + 10
                while time.time() < settle and not t.get("title") and not t.get("error") and not t.get("blocked"):
                    time.sleep(0.2)
                    t = tab(t["id"]) or t
                time.sleep(0.5)
                t = tab(t["id"]) or t
                bad = [k for k in ("blocked", "crashed") if t.get(k)] + ([t["error"] + " " + t.get("errorCode", "")] if t.get("error") else []) \
                    + ([] if t.get("title") else ["no title"])
                return t, bad
            time.sleep(0.1)
        return tab(tabs_of(app)["active"]), ["never landed on " + want_url]

    first = from_box(app, STORE)
    ids = lambda: {t["id"] for t in tabs_of(app)["tabs"]}

    before = ids()
    run("tab.duplicate")
    t, bad = landed(STORE, True, before)
    check("Duplicate Tab opens the page again in a new tab, with no error", t is not None and t["id"] not in before and not bad, (t and t["url"], bad))

    dup = tabs_of(app)["active"]
    run("tab.close")
    app.wait(f"{ROWS}==={len(before)}", 10)
    run("tab.reopenClosed")
    t, bad = landed(STORE)
    check("Reopen Closed Tab brings the closed tab back on its page, with no error", t is not None and not bad, (t and t["url"], bad))

    run("tab.pin")
    pinned = lambda: bool((tab(tabs_of(app)["active"]) or {}).get("pin"))
    deadline = time.time() + 10
    while not pinned() and time.time() < deadline:
        time.sleep(0.1)
    t, bad = landed(STORE)
    check("Pin Tab pins the tab and keeps its page, with no error", pinned() and not bad, (pinned(), bad))
    run("tab.pin")

    before = ids()
    send({"t": "newTab", "workspace": "0a1b2c3d", "text": STORE})
    t, bad = landed(STORE, True, before)
    check("a tab moved to another workspace opens there on its page, with no error",
          t is not None and t["workspace"] == "0a1b2c3d" and not bad, (t and t["workspace"], bad))

    click(app, f'[role=tab][data-tab="{first}"]')
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{first}\"]').getAttribute('aria-selected')==='true'")
    before = ids()
    page_click(app, "#blank")
    deadline = time.time() + 20
    opened = None
    while time.time() < deadline and opened is None:
        opened = next((t for t in tabs_of(app)["tabs"] if t["id"] not in before and t["url"] == STORE and not t["loading"]), None)
        time.sleep(0.1)
    bad = [] if opened is None else [k for k in ("blocked", "crashed") if opened.get(k)] + ([opened["error"]] if opened.get("error") else [])
    check("a link opened in a new tab loads there, with no error", opened is not None and not bad, bad or "no new tab")

    # The rest of the everyday actions, on a page: each lands on a page with no error.
    click(app, f'[role=tab][data-tab="{first}"]')
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{first}\"]').getAttribute('aria-selected')==='true'")
    for action, want in (("nav.reload", STORE), ("nav.reloadHard", STORE), ("view.zoomIn", STORE), ("view.zoomOut", STORE),
                         ("view.zoomReset", STORE), ("tab.next", None), ("tab.previous", None), ("tab.select1", None),
                         ("tab.selectLast", None), ("tab.recentNext", None)):
        run(action)
        time.sleep(0.5)
        t = tab(tabs_of(app)["active"])
        if want is not None:
            t, bad = landed(want)
        else:
            deadline = time.time() + 10
            while t and t["loading"] and time.time() < deadline:
                time.sleep(0.1)
                t = tab(tabs_of(app)["active"])
            bad = [] if t is None else [k for k in ("blocked", "crashed") if t.get(k)] + ([t["error"]] if t.get("error") else [])
        check(f"{action} lands on a page with no error", t is not None and not bad, (t and t["url"], bad))
    before = ids()
    run("dev.viewSource")
    deadline = time.time() + 20
    src = None
    while time.time() < deadline and src is None:
        # Go has the tab at the page's own address; the page is its source, in view-source's table.
        src = next((t for t in tabs_of(app)["tabs"] if t["id"] not in before and not t["loading"]), None)
        time.sleep(0.1)
    bad = [] if src is None else [k for k in ("blocked", "crashed") if src.get(k)] + ([src["error"]] if src.get("error") else [])
    shows = src is not None and bool(app.js("!!document.querySelector('td.line-content')", tab=src["id"]))
    check("View Source opens the page's source in a new tab, with no error", shows and not bad, bad or ("no source tab" if src is None else "not its source"))

    # Every Tab N and the rest of the tab-switching keys land on a tab of the window.
    order = [t["id"] for t in tabs_of(app)["tabs"]]
    # Named outright, so tools/action_coverage.py sees each is done.
    for n, action in enumerate(("tab.select1", "tab.select2", "tab.select3", "tab.select4",
                                "tab.select5", "tab.select6", "tab.select7", "tab.select8"), 1):
        run(action)
        want = order[min(n, len(order)) - 1]
        deadline = time.time() + 5
        while tabs_of(app)["active"] != want and time.time() < deadline:
            time.sleep(0.1)
        check(f"{action} shows tab {min(n, len(order))}", tabs_of(app)["active"] == want, tabs_of(app)["active"])
    run("tab.recentPrevious")
    time.sleep(0.5)
    t = tab(tabs_of(app)["active"])
    check("tab.recentPrevious lands on a page with no error", t is not None and not t.get("error") and not t.get("crashed"), t and t["url"])

    # Find and its steps, the address bar, and the shells' own Reload and Forward.
    click(app, f'[role=tab][data-tab="{first}"]')
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{first}\"]').getAttribute('aria-selected')==='true'")
    run("find.open")
    found = app.wait("!!document.querySelector('[role=search] input, .find-bar input')", 10)
    check("find.open opens the find bar", bool(found))
    for action in ("find.next", "find.previous"):
        run(action)
        check(f"{action} keeps the find bar and the page", bool(app.js("!!document.querySelector('[role=search] input, .find-bar input')")) and landed(STORE)[1] == [])
    app.cmd("key", "escape")
    # F6 moves the keyboard between the shell and the page, and Shift+F6 back: by the real
    # keys, as the action goes by which side the last key was pressed in, and the driver's
    # own key command has no F keys. From the address bar first, so it starts in the shell.
    def real_key(combo):
        """A key from the Mac's own keyboard, to the window in front, which is the app's."""
        subprocess.run(["deskrig", "key", combo], capture_output=True, timeout=20)
    run("address.focus")
    app.wait("document.activeElement&&document.activeElement.matches('input')", 5)
    real_key("f6")  # does: focus.nextPane
    page_has = lambda: bool(app.js("document.hasFocus()", page=True))
    deadline = time.time() + 5
    while not page_has() and time.time() < deadline:
        time.sleep(0.1)
    check("focus.nextPane moves the keyboard to the page", page_has())
    real_key("shift+f6")  # does: focus.previousPane
    deadline = time.time() + 5
    while page_has() and time.time() < deadline:
        time.sleep(0.1)
    check("focus.previousPane moves it back to the shell", not page_has() and bool(app.js("document.hasFocus()")))
    run("address.focus")
    check("address.focus gives the address bar the keyboard", bool(app.wait("document.activeElement&&document.activeElement.matches('input')", 5)))
    app.cmd("key", "escape")
    for msg in ({"t": "reload"}, {"t": "forward"}):
        send(msg)
        time.sleep(0.5)
        check(f"the shells' {msg['t']} lands on a page with no error", landed(STORE)[1] == [], landed(STORE)[1])

    # Tab groups from the tab menu: a group made, a tab added, a tab taken out.
    send({"t": "createGroup", "ids": [first]})
    deadline = time.time() + 5
    while not (tab(first) or {}).get("group") and time.time() < deadline:
        time.sleep(0.1)
    group = (tab(first) or {}).get("group")
    other = next(i for i in order if i != first)
    send({"t": "addToGroup", "id": other, "group": group})
    deadline = time.time() + 5
    while (tab(other) or {}).get("group") != group and time.time() < deadline:
        time.sleep(0.1)
    check("addToGroup puts a tab in a group", bool(group) and (tab(other) or {}).get("group") == group, tab(other))
    send({"t": "removeFromGroup", "id": other})
    deadline = time.time() + 5
    while (tab(other) or {}).get("group") and time.time() < deadline:
        time.sleep(0.1)
    check("removeFromGroup takes it out", not (tab(other) or {}).get("group"), tab(other))

    # Our pages from the menus, Stop, and a tab moved to a window of its own.
    for action, page in (("history.show", "history"), ("bookmarks.show", "bookmarks"), ("downloads.show", "downloads")):
        run(action)
        deadline = time.time() + 10
        while (tab(tabs_of(app)["active"]) or {}).get("page") != page and time.time() < deadline:
            time.sleep(0.1)
        check(f"{action} shows the {page} page", (tab(tabs_of(app)["active"]) or {}).get("page") == page)
    click(app, f'[role=tab][data-tab="{first}"]')
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{first}\"]').getAttribute('aria-selected')==='true'")
    run("nav.stop")
    check("nav.stop leaves the page as it was, with no error", landed(STORE)[1] == [])
    windows_before = len(json.loads(app.cmd("windows")))
    run("tab.moveToNewWindow")
    deadline = time.time() + 10
    while len(json.loads(app.cmd("windows"))) == windows_before and time.time() < deadline:
        time.sleep(0.1)
    check("tab.moveToNewWindow opens a window with the tab, on its page", len(json.loads(app.cmd("windows"))) == windows_before + 1 and landed(STORE)[1] == [])

    # Close Window, last: the window goes, with the app still running.
    windows = len(json.loads(app.cmd("windows")))
    run("window.close")
    deadline = time.time() + 10
    while len(json.loads(app.cmd("windows"))) == windows and time.time() < deadline:
        time.sleep(0.1)
    check("window.close closes the window, the app still running", app.proc.poll() is None and len(json.loads(app.cmd("windows"))) == windows - 1)
    app.close()


def workspace_menu_checks():
    """Tab, New Tab in Workspace lists the profile's workspaces, read as the menu opens, and
    choosing one opens a new tab in that workspace in the window in front. A new profile's
    lists Main. A workspace made in settings is listed the next time the menu opens; it is
    the profile folder's, here a folder named outright."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = App(profile)
    item = lambda: next(i for m in json.loads(app.cmd("menu")) if m["title"] == "Tab" for i in m["items"] if i != "-" and i["title"] == "New Tab in Workspace")
    check("a new profile's Tab, New Tab in Workspace lists Main", item()["enabled"] and [i["title"] for i in item()["items"]] == ["Main"], item())

    settings = lambda expr: json.loads(app.cmd("evalsettings", expr))
    press_settings(app)
    deadline = time.time() + 10
    while app.cmd("evalsettings", "!!document.querySelector('h1')") != "true":
        if time.time() > deadline:
            raise TimeoutError("the settings page never opened")
        time.sleep(0.1)
    settings("document.querySelector('[data-section=workspaces]').click()")
    # New workspace… opens a dialog with the name and the colour; Create makes it.
    for name in ("Work", "Shopping"):
        settings("[...document.querySelectorAll('button')].find(b=>b.textContent.trim()==='New workspace…').click()")
        deadline = time.time() + 5
        while not settings("!!document.querySelector('#workspace-name')"):
            if time.time() > deadline:
                raise TimeoutError("the New workspace dialog never opened")
            time.sleep(0.1)
        settings(f"(()=>{{const i=document.querySelector('#workspace-name');i.value={json.dumps(name)};i.dispatchEvent(new Event('input'));i.form.requestSubmit()}})()")
        while settings("!!document.querySelector('#workspace-name')"):
            if time.time() > deadline:
                raise TimeoutError("the New workspace dialog never closed")
            time.sleep(0.1)
    deadline = time.time() + 10
    while [i["title"] for i in item()["items"]] != ["Main", "Work", "Shopping"] and time.time() < deadline:
        time.sleep(0.1)
    check("workspaces made in settings are listed the next time the menu opens", [i["title"] for i in item()["items"]] == ["Main", "Work", "Shopping"], item())
    app.cmd("key", "cmd+w")
    ids = {w["name"]: w["id"] for w in json.load(open(os.path.join(profile, "workspaces.json")))["workspaces"]}

    # New Tab in Workspace, which no shell takes, opens one beside the window's new tab.
    check("choosing one works", app.cmd("choosemenu", "Tab", "New Tab in Workspace > Shopping") == "ok")
    app.wait(f"{ROWS}===2")
    tabs = tabs_of(app)
    shopping = next(t for t in tabs["tabs"] if t["id"] == tabs["active"])
    check("it opens a new tab, the active one, which Go has in that workspace", len(tabs["tabs"]) == 2 and shopping.get("workspace") == ids["Shopping"], tabs)
    # The new tab loads nothing, so there is no page to script: the address goes in as a
    # person's does, typed in the box Standard shows on it, and the tab, in its workspace,
    # takes its first page.
    box_open(app)
    app.cmd("type", STORE)
    app.wait(f"{BOX}.value==={json.dumps(STORE)}")
    app.cmd("key", "enter")
    deadline = time.time() + 30
    while not (tab := next(t for t in tabs_of(app)["tabs"] if t["id"] == shopping["id"]))["url"].startswith(STORE) or tab["loading"]:
        if time.time() > deadline:
            raise TimeoutError(f"the workspace's new tab never loaded {STORE}: {tab}")
        time.sleep(0.05)
    app.wait("document.readyState==='complete'&&location.pathname==='/store'", tab=shopping["id"])
    app.js("document.cookie='w=shopping; max-age=3600'", tab=shopping["id"])
    check("and the engine has it there: its cookie is that workspace's and not another's",
          stored(app, open_in(app, ids["Shopping"], STORE))[0] == "w=shopping" and stored(app, open_in(app, ids["Work"], STORE))[0] == "", tabs_of(app))
    app.close()


def in_settings(app, expr, timeout=10):
    """Waits for expr to be truthy in the settings page, and returns it."""
    deadline = time.time() + timeout
    while True:
        r = app.cmd("evalsettings", expr.replace("\n", " "))
        if r and not r.startswith("ERR") and json.loads(r):
            return json.loads(r)
        if time.time() > deadline:
            raise TimeoutError(f"settings: {expr} last={r}")
        time.sleep(0.1)


def delete_workspace(app, workspace, close=True):
    """Deletes a workspace, by its id, as a person does in Settings, Workspaces: its row's
    menu, Delete…, then Delete in the dialog that asks first. The settings tab opens on
    Cmd+, and with close closes on Cmd+W, as it is still the active tab: it is in no
    workspace. Without, it stays, as it must when the window has no other tab left."""
    row = json.dumps(f'[data-workspace="{workspace}"]')
    click = "(b=>b&&(b.click(),1))"
    press_settings(app)
    # Workspaces is a section of its own.
    in_settings(app, click + "(document.querySelector('[data-section=\"workspaces\"]'))")
    in_settings(app, click + f"(document.querySelector({row}+' .ui-row-menu > button'))")
    in_settings(app, click + "([...document.querySelectorAll('[role=menuitem]')].find(b=>b.textContent.trim()==='Delete…'))")
    in_settings(app, click + "([...document.querySelectorAll('dialog[open] button')].find(b=>b.textContent.trim()==='Delete'))")
    in_settings(app, f"!document.querySelector({row})")
    if not close:
        return
    app.cmd("key", "cmd+w")
    deadline = time.time() + 10
    while not app.cmd("evalsettings", "1").startswith("ERR"):
        if time.time() > deadline:
            raise TimeoutError("the settings tab stayed open")
        time.sleep(0.1)


def erased(app, workspace, times=1):
    """Waits for the engine to have erased a workspace's storage, as it marks on the
    timeline, times times in all."""
    deadline = time.time() + 30
    while sum(e.get("name") == "workspace.erased" and e.get("text") == f"workspace={workspace}" for e in json.loads(app.cmd("timeline"))["result"]["entries"]) < times:
        if time.time() > deadline:
            raise TimeoutError(f"workspace {workspace!r} was never erased")
        time.sleep(0.1)


def workspace_delete_checks():
    """Deleting a workspace in settings closes its tabs and then erases its storage: a tab
    opened in its partition again finds no cookie and no local storage, and the other
    workspaces keep theirs. Deleting Main erases the profile's own site data. With Main
    deleted, a tab made over DevTools is in the first workspace. A window whose tabs were
    all in the workspace deleted gets a new tab and stays open. That an extension still works with
    Main deleted is checked in tools/drive_extensions.py."""
    work, shop, spare = "0a1b2c3d", "4e5f6a7b", "8c9d0e1f"
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    with open(os.path.join(profile, "workspaces.json"), "w") as f:
        json.dump({"version": 2, "workspaces": [{"id": "", "name": "Main", "color": "#3470d0"}, {"id": work, "name": "Work", "color": "#d03434"},
                                                {"id": shop, "name": "Shop", "color": "#4f9e9e"}, {"id": spare, "name": "Spare", "color": "#b07a2a"}]}, f)
    app = App(profile, ["--remote-debugging-port=0"])

    def keep(tab, value):
        app.js(f"document.cookie='w={value}; max-age=3600';localStorage.setItem('w','{value}')", tab=tab)

    # Main's loads in the new tab the window starts on.
    main = from_box(app, STORE)
    keep(main, "main")
    in_work = [open_in(app, work, STORE), open_in(app, work, STORE)]
    keep(in_work[0], "work")
    in_shop = open_in(app, shop, STORE)
    keep(in_shop, "shop")
    check("each workspace has its own cookie and local storage", [stored(app, t) for t in (main, in_work[1], in_shop)] == [["w=main", "main"], ["w=work", "work"], ["w=shop", "shop"]])

    delete_workspace(app, work)
    left = sorted(t["id"] for t in tabs_of(app)["tabs"])
    check("deleting a workspace in settings closes its tabs", left == sorted([main, in_shop]) and app.js(ROWS) == 2, left)
    erased(app, work)
    check("and erases its storage: a tab opened in it again has no cookie and no local storage", stored(app, open_in(app, work, STORE)) == ["", None])
    check("while the others keep theirs", stored(app, main) == ["w=main", "main"] and stored(app, in_shop) == ["w=shop", "shop"])

    delete_workspace(app, "")
    check("deleting Main closes its tabs", main not in [t["id"] for t in tabs_of(app)["tabs"]], tabs_of(app))
    erased(app, "")
    check("and erases the profile's own site data", stored(app, open_in(app, "", STORE)) == ["", None])
    check("while Shop keeps its own", stored(app, in_shop) == ["w=shop", "shop"])

    port = open(os.path.join(profile, "DevToolsActivePort")).read().split()[0]
    urllib.request.urlopen(urllib.request.Request(f"http://127.0.0.1:{port}/json/new?{STORE}", method="PUT")).read()
    app.wait(f"{ROWS}===4")
    made = max(tabs_of(app)["tabs"], key=lambda t: t["id"])
    app.wait("document.readyState==='complete'&&location.pathname==='/store'", tab=made["id"])
    check("with Main deleted, a tab made over DevTools is in the first workspace", made.get("workspace") == shop and stored(app, made["id"]) == ["w=shop", "shop"], made)

    # Shop's first tab moves into a window of its own, and Shop goes.
    windows = lambda: json.loads(app.cmd("windows"))
    app.cmd("projectb", "moveTab", json.dumps({"tab": in_shop, "window": 0, "index": 0}))
    deadline = time.time() + 10
    while len(windows()) < 2 and time.time() < deadline:
        time.sleep(0.05)
    # Deleted from the first window's settings, so the settings tab isn't the second's.
    app.cmd("window", 1)
    delete_workspace(app, shop, close=False)
    app.cmd("window", 2)
    deadline = time.time() + 10
    while in_shop in [t["id"] for t in tabs_of(app)["tabs"]] and time.time() < deadline:
        time.sleep(0.05)
    check("a window whose tabs were all in the workspace deleted gets a new tab, and stays open",
          2 in [w["id"] for w in windows()] and app.js(ROWS) == 1 and holds(app, f"!!{BOX}", 5), (windows(), app.js(ROWS)))
    # Both windows go at once, as Cmd+Q does.
    press_quit(app)
    check("Cmd+Q ends the app with both windows, cleanly", app.proc.wait(timeout=20) == 0)
    App.running.remove(app)


def site_data_checks():
    """Settings' Sites and Clear browsing data on the real engine (docs/settings.md): what
    pages in two workspaces kept is counted by site, in both; a site's Clear erases it in
    both; and Clear browsing data from the last hour erases a cookie set since."""
    work = "0a1b2c3d"
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    with open(os.path.join(profile, "workspaces.json"), "w") as f:
        json.dump({"version": 2, "workspaces": [{"id": "", "name": "Main", "color": "#3470d0"}, {"id": work, "name": "Work", "color": "#d03434"}]}, f)
    app = App(profile, ["--remote-debugging-port=0"])

    def keep(tab, value):
        app.js(f"document.cookie='w={value}; max-age=3600';localStorage.setItem('w','{value}')", tab=tab)

    keep(open_in(app, "", STORE), "main")
    keep(open_in(app, work, STORE), "work")
    click = "(b=>b&&(b.click(),1))"
    button = lambda words: click + f"([...document.querySelectorAll('button')].find(b=>b.textContent.trim()==={json.dumps(words)}))"
    press_settings(app)
    # Sites is a row of Privacy that opens it as a page of its own.
    in_settings(app, click + "(document.querySelector('[data-section=\"privacy\"]'))")
    in_settings(app, click + "(document.querySelector('[data-section=\"sites\"]'))")
    # The pages are on 127.0.0.1, a site of its own: a cookie in each workspace.
    in_settings(app, click + "(document.querySelector('.site-row[data-site=\"127.0.0.1\"] button.open'))")
    view = in_settings(app, "(t=>/2 cookies/.test(t)&&t)(document.querySelector('section[aria-label=\"127.0.0.1\"]')?.textContent||'')", timeout=20)
    check("Sites counts what pages kept, by site, in every workspace", "Cookies and site data" in view, view)
    in_settings(app, button("Clear…"))
    in_settings(app, click + "([...document.querySelectorAll('dialog[open] button')].find(b=>b.textContent.trim()==='Clear'))")
    erased(app, "")
    erased(app, work)
    check("a site's Clear erases it in every workspace", [stored(app, open_in(app, w, STORE)) for w in ("", work)] == [["", None], ["", None]])

    # Clear browsing data, from the last hour, with its other choices as they start.
    keep(open_in(app, "", STORE), "again")
    in_settings(app, click + "(document.querySelector('[aria-label=\"Back to Sites\"]'))")
    in_settings(app, click + "(document.querySelector('.list-page .list-head .back'))")
    in_settings(app, click + "([...document.querySelectorAll('.row')].find(r=>r.textContent.includes('Clear browsing data'))?.querySelector('button'))")
    in_settings(app, "(s=>s&&(s.value='hour',s.dispatchEvent(new Event('change')),1))(document.querySelector('#clear-since'))")
    in_settings(app, click + "([...document.querySelectorAll('dialog[open] button')].find(b=>b.textContent.trim()==='Clear'))")
    erased(app, "", 2)
    check("Clear browsing data from the last hour erases a cookie set since", stored(app, open_in(app, "", STORE)) == ["", None])
    app.close()


def quit_erasure_checks():
    """Clear cookies when the app quits, on the real engine (docs/settings.md): with it on and
    127.0.0.1 kept, a quit erases what localhost kept, in every workspace, before the app
    ends, and keeps what 127.0.0.1 kept."""
    work = "0a1b2c3d"
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    with open(os.path.join(profile, "workspaces.json"), "w") as f:
        json.dump({"version": 2, "workspaces": [{"id": "", "name": "Main", "color": "#3470d0"}, {"id": work, "name": "Work", "color": "#d03434"}]}, f)
    with open(os.path.join(profile, "prefs.json"), "w") as f:
        json.dump({"clearCookiesOnQuit": True, "keepCookies": ["127.0.0.1"]}, f)
    local = STORE.replace("127.0.0.1", "localhost")
    app = App(profile, ["--remote-debugging-port=0"])

    def keep(tab, value):
        app.js(f"document.cookie='w={value}; max-age=3600';localStorage.setItem('w','{value}')", tab=tab)

    keep(open_in(app, "", STORE), "kept")
    keep(open_in(app, "", local), "main")
    keep(open_in(app, work, local), "work")
    app.close()
    app = App(profile, ["--remote-debugging-port=0"])
    check("a quit erases every site's cookies and storage but the kept sites', in every workspace",
          [stored(app, open_in(app, w, local)) for w in ("", work)] == [["", None], ["", None]])
    check("and keeps the kept site's", stored(app, open_in(app, "", STORE)) == ["w=kept", "kept"])
    app.close()


def shell_checks():
    """What the real app adds to a shell's contract, for every shell in the registry
    (docs/shells.md): a profile whose pref names it starts on it and marks app.ready, and
    switching to it live, as View, Shell does, leaves the tabs loaded and shows the active
    tab. The checks speak to each shell only as Go does, on the shell channel."""
    # A native shell is drawn on its platform only: tools/drive_android.py checks it.
    registry = [s for s in json.load(open(os.path.join(ROOT, "core/shells/registry.json"))) if not s.get("native")]

    def on(shell):
        profile = tempfile.mkdtemp(prefix="pb-profile-")
        with open(os.path.join(profile, "prefs.json"), "w") as f:
            json.dump({"shell": shell}, f)
        return App(profile)

    send = lambda app, msg: app.js(f"(chrome.send('shell',[{json.dumps(json.dumps(msg))}]),1)")
    # Every shell starts on a new tab.
    for shell in (s["id"] for s in registry):
        # App waits for app.ready, which the shell's shown marks.
        app = on(shell)
        check(f"{shell}: a profile whose shell it is starts on it and marks app.ready",
              app.js("location.href") == f"chrome://shell/{shell}/" and app.js(ROWS) == 1, (app.js("location.href"), app.js(ROWS)))
        app.close()

        # navigate loads in the window's new tab, and newTab, the shell's own message, makes a
        # tab whichever shell is on.
        app = on(next(s["id"] for s in registry if s["id"] != shell))
        send(app, {"t": "navigate", "text": PAGES + "first"})
        app.wait(f"{ROWS}===1", 30)
        first = tabs_of(app)["active"]
        app.wait("location.pathname==='/first'&&document.readyState==='complete'", 30, tab=first)
        send(app, {"t": "newTab"})
        app.wait(f"{ROWS}===2")
        active = tabs_of(app)["active"]
        send(app, {"t": "navigate", "text": PAGES + "second"})
        app.wait("location.pathname==='/second'&&document.readyState==='complete'", 30, tab=active)
        for tab in (first, active):
            app.js("window.kept=1,1", tab=tab)
        send(app, {"t": "run", "text": f"shells:{shell}"})
        app.wait(f"location.href==='chrome://shell/{shell}/'&&{ROWS}===2", 30)
        check(f"{shell}: switching to it live leaves the tabs loaded", all(app.js("window.kept", tab=tab) == 1 for tab in (first, active)))
        deadline = time.time() + 10
        shown = lambda: [v["tab"] for v in json.loads(app.cmd("visible")) if v["shown"] and not v["hidden"]]
        while active not in shown() and time.time() < deadline:
            time.sleep(0.05)
        check(f"{shell}: and it shows the active tab", tabs_of(app)["active"] == active and active in shown(), (tabs_of(app), app.cmd("visible")))
        app.close()


def cookie_checks():
    """A cookie a page sets is saved in the profile and sent back after a restart."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = App(profile)
    visit(app, PAGES + "cookie")
    check("the first visit sends no cookie", app.js("document.title", page=True) == "none")
    app.close()
    check("the cookie is saved in the profile", os.path.exists(os.path.join(profile, "chromium/Network/Cookies")))
    app = App(profile)
    visit(app, PAGES + "cookie")
    check("the cookie is sent back after a restart", app.js("document.title", page=True) == "a=1", app.js("document.title", page=True))
    app.close()


# Asks one process to quit as the Dock's Quit, logging out and AppleScript do: the quit Apple
# Event, aimed at its pid so no other copy hears it.
TERMINATE = 'ObjC.import("AppKit"); $.NSRunningApplication.runningApplicationWithProcessIdentifier(Number(%d)).terminate'


def glide_checks():
    """Glide, chosen on a profile a first start makes: it comes up, keeps a
    Favorite and a pinned tab, draws a folder for a tab group, and its Command Bar finds the
    open tabs; Copy Address says it copied (check_clipboard.py reads the clipboard), New Space
    makes a Space, and Space 1 to 9 show theirs. Leaves glide.png and glide-command-bar.png, which the README shows."""
    root = tempfile.mkdtemp(prefix="pb-root-")
    app = App(None, [f"--root={root}"])
    send = lambda msg: app.js(f"chrome.send('shell', [{json.dumps(json.dumps(msg))}]), 1")
    app.wait("location.href==='chrome://shell/standard/'", 30)
    check("a first start comes up on the standard shell, the default", True)
    send({"t": "run", "text": "shells:glide"})
    app.wait("location.href==='chrome://shell/glide/'&&!!document.querySelector('.glide')", 30)
    check("choosing Glide brings it up", True)
    app.cmd("resize", 1280, 800)
    send({"t": "navigate", "text": URL_A})
    app.wait(f"{ROWS}===1&&document.querySelector('[role=tab][aria-selected=true]').title!=='New Tab'", 30)
    send({"t": "run", "text": "glide.favorite"})
    check("Add to Favorites puts a tile at the top", bool(app.wait("document.querySelectorAll('.fav').length===1", 10)))
    for url in (URL_B, PAGES + "store", PAGES):
        send({"t": "newTab", "text": url})
    app.wait(f"{ROWS}===4", 30)
    tabs = json.loads(app.js("JSON.stringify([...document.querySelectorAll('[role=tab]')].map(e => +e.dataset.tab))"))
    send({"t": "activateTab", "id": tabs[1]})
    app.wait(f"document.querySelector('[role=tab][aria-selected=true]').dataset.tab==='{tabs[1]}'", 10)
    send({"t": "run", "text": "tab.pin"})
    check("Pin Tab keeps it at the top", bool(app.wait(f"document.querySelector('.pinned [role=tab]')?.dataset.tab==='{tabs[1]}'", 10)))
    send({"t": "createGroup", "ids": tabs[2:], "name": "Reading"})
    check("a tab group is a folder", bool(app.wait("document.querySelector('.folder')?.textContent.includes('Reading')", 10)))
    send({"t": "activateTab", "id": tabs[0]})
    time.sleep(2)
    print(app.cmd("capture", os.path.join(OUT, "glide.png")), flush=True)
    send({"t": "run", "text": "glide.commandBar"})
    app.wait("!!document.querySelector('[aria-label=Address]')", 10)
    app.js("(i => { i.value = 'example'; i.dispatchEvent(new Event('input', {bubbles: true})); })(document.querySelector('[aria-label=Address]')), 1")
    check("the Command Bar finds the open tabs", bool(app.wait("[...document.querySelectorAll('[role=option]')].some(o => o.textContent.includes('Example'))", 10)))
    time.sleep(0.5)
    print(app.cmd("capture", os.path.join(OUT, "glide-command-bar.png")), flush=True)
    app.cmd("key", "escape")
    app.wait("!document.querySelector('[aria-label=Address]')", 10)

    # Copy Address, as the menu bar and its keys run it, with no click in the shell. Only the
    # desktop's own session reaches the Mac's clipboard, so check_clipboard.py reads it back.
    app.js("window.toastSeen = ''; new MutationObserver(() => { const t = document.querySelector('.toast'); if (t) window.toastSeen = t.textContent; }).observe(document.body, {subtree: true, childList: true, characterData: true}), 1")
    send({"t": "run", "text": "glide.copyAddress"})
    told = app.wait("window.toastSeen", 10)
    check("Copy Address says it copied", told == "Copied", told)

    # New Space opens its dialog, which makes the Space and shows it.
    head = "document.querySelector('.space-head .label')?.textContent.trim()"
    first = app.js(head)
    send({"t": "run", "text": "glide.newSpace"})
    check("New Space asks for the Space's name", bool(app.wait("!!document.querySelector('[role=dialog][aria-label=\"New Space\"] input[aria-label=Name]')", 10)))
    app.js("(i => { i.value = 'Two'; i.dispatchEvent(new Event('input', {bubbles: true})); })(document.querySelector('[role=dialog] input[aria-label=Name]')), 1")
    app.wait("!document.querySelector('[role=dialog] [type=submit]').disabled", 5)
    app.js("document.querySelector('[role=dialog] [type=submit]').click(), 1")
    check("and Create makes it and shows it", bool(app.wait(f"{head} === 'Two'", 15)), app.js(head))
    app.cmd("key", "escape")

    # Seven more, so Space 2 to 9 each have one to show, and each shows its own. The first
    # Space is the one the window started on.
    colors = ["#3470d0", "#d0343a", "#2e9d5b", "#9b59b6", "#e67e22", "#16a085", "#7f8c8d"]
    for n, c in zip(range(3, 10), colors):
        send({"t": "createWorkspace", "text": f"S{n}", "color": c})
    app.wait("document.querySelectorAll('.foot .space').length === 9", 15)
    names = ["Two"] + [f"S{n}" for n in range(3, 10)]
    shown = []
    for name, action in zip(names, ("glide.space2", "glide.space3", "glide.space4", "glide.space5", "glide.space6", "glide.space7", "glide.space8", "glide.space9")):
        send({"t": "run", "text": action})
        shown.append(name if app.wait(f"{head} === {json.dumps(name)}", 10) else f"{action}: {app.js(head)}")
        app.cmd("key", "escape")
    check("Space 2 to 9 each show their Space", shown == names, ", ".join(shown))
    send({"t": "run", "text": "glide.space1"})
    check("and Space 1 the first again", bool(app.wait(f"{head} === {json.dumps(first)}", 10)), app.js(head))
    app.close()


def passkey_checks():
    """A page's passkey request on the real engine (docs/passkeys.md), with the DevTools
    protocol's virtual security key in place of a real one: making a passkey waits at the
    key step with no key there, and a key plugged in and touched makes it; a key that
    already has one says so, and the page hears why once the card is closed; two accounts
    on the key bring the account step, and the one chosen signs in; Cancel and Escape
    refuse the page; and with the virtual key's UI off, as automated tests run it, no card
    shows. A virtual key has no PIN, so the PIN step is checked by hand with a real key,
    as phone as key is with a real phone.

    The touch is a key added while the request waits, as one plugged in is: a virtual key
    answers a request the moment it gets it, or, with its presence off, never, and turning
    presence on later doesn't reach a request it already has."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = App(profile, ["--remote-debugging-port=0"])
    # localhost, as a passkey's site can't be an IP address.
    url = OTHER + "/passkey"
    visit(app, url)
    s = attach(profile, url)
    ids = iter(range(1, 1000))

    def call(method, params=None):
        r = cdp(s, next(ids), method, params)
        if "error" in r:
            raise RuntimeError(f"{method}: {r['error']}")
        return r["result"]

    def keys(ui):
        """Virtual keys in place of real ones, none yet, with our card or without."""
        call("WebAuthn.disable")
        call("WebAuthn.enable", {"enableUI": ui})

    def plug():
        """Plugs in a virtual key that answers at once: CTAP2 over USB, remembering its
        accounts, its owner verified and present."""
        return call("WebAuthn.addVirtualAuthenticator", {"options": {
            "protocol": "ctap2", "transport": "usb", "hasResidentKey": True, "hasUserVerification": True,
            "isUserVerified": True, "automaticPresenceSimulation": True}})["authenticatorId"]

    CARD = "document.querySelector('[role=alertdialog]')"
    TITLE = f"{CARD}?.getAttribute('aria-label')"
    WORDS = f"{CARD}?.querySelector('p')?.textContent.trim()"
    # With no card these are empty, so a check on them fails rather than stopping the part.
    BUTTONS = f"[...({CARD}?.querySelectorAll('.buttons button')||[])].map(b=>b.textContent.trim())"
    PRIMARY = f"[...({CARD}?.querySelectorAll('.primary')||[])].map(b=>b.textContent.trim())"
    ROWS_ = f"[...({CARD}?.querySelectorAll('button.account')||[])]"

    # Each of these says how it went, so a check that fails leaves the rest to run.
    def card(title, timeout=15):
        try:
            app.wait(f"{TITLE}==={json.dumps(title)}", timeout)
            return True
        except TimeoutError:
            return False

    def gone(timeout=15):
        try:
            app.wait(f"!{CARD}", timeout)
            return True
        except TimeoutError:
            return False

    def press(find, what):
        at = app.js(f"(()=>{{const r=({find})?.getBoundingClientRect();return r?[r.x+r.width/2,r.y+r.height/2]:null}})()")
        if not at:
            print(f"note nothing to press for {what}", flush=True)
            return False
        click_at(app, *at, what)
        return True

    def button(name):
        return press(f"[...({CARD}?.querySelectorAll('button')||[])].find(b=>b.textContent.trim()==={json.dumps(name)})", name)

    def title(test, timeout=15):
        """The page's title once test, a JavaScript test of t, holds; or what it was."""
        try:
            return app.wait(f"(t=>({test})&&t)(document.title)", timeout, page=True)
        except TimeoutError:
            return app.js("document.title", page=True)

    def settle():
        """Ends whatever request an earlier check left waiting, and its card."""
        if app.js(f"!!{CARD}"):
            app.cmd("key", "escape")
            gone(5)

    # Our own passkeys (docs/passkeys.md, Our own passkeys), with content's own discoveries
    # and ours among them, before any virtual key takes their place. Saving and signing in
    # need the person at macOS's own sheet, so they are checked in the VM, where its
    # password can be typed (tools/vm/check.py); here, what comes before the sheet.
    SAVE = "Save a passkey for localhost in Project B?"
    KEY = "localhost wants to save a passkey on your security key. Plug it in and touch it."
    app.js("canKeep()", page=True)
    check("a page asking whether this device can keep a passkey hears yes", title("t.startsWith('keeps')") == "keeps true", app.js("document.title", page=True))
    app.js("make('ada')", page=True)
    check("a page making a passkey is offered Project B first", card("Create a passkey") and app.js(WORDS) == SAVE, (app.js(TITLE), app.js(WORDS)))
    check("with the way to a security key or a phone, Cancel and Save, the primary", app.js(BUTTONS) == ["Use a security key or phone", "Cancel", "Save"] and app.js(PRIMARY) == ["Save"], app.js(BUTTONS))
    button("Use a security key or phone")
    try:
        app.wait(f"{WORDS}==={json.dumps(KEY)}", 10)
    except TimeoutError:
        pass
    check("Use a security key or phone goes to the key step", app.js(WORDS) == KEY and app.js(PRIMARY) == [], (app.js(WORDS), app.js(BUTTONS)))
    time.sleep(1)
    check("where the page still waits", app.js("document.title", page=True) == "asking", app.js("document.title", page=True))
    settle()
    check("Escape there refuses the page", title("t.startsWith('refused')") == "refused NotAllowedError", app.js("document.title", page=True))
    app.js("make('ada', '', 'platform')", page=True)
    check("a page that wants this device alone gets Save with no other way", card("Create a passkey") and app.js(WORDS) == SAVE and app.js(BUTTONS) == ["Cancel", "Save"], (app.js(WORDS), app.js(BUTTONS)))
    settle()
    title("t.startsWith('refused')")
    app.js("make('ada', '', 'cross-platform')", page=True)
    check("and one that wants a key apart from the device gets the key step", card("Create a passkey") and app.js(WORDS) == KEY, (app.js(TITLE), app.js(WORDS)))
    settle()
    title("t.startsWith('refused')")
    app.js("use()", page=True)
    check("signing in with no passkey here goes to the key step", card("Sign in with a passkey") and app.js(WORDS) == "localhost wants you to sign in with your security key. Plug it in and touch it.", (app.js(TITLE), app.js(WORDS)))
    settle()
    title("t.startsWith('refused')")
    # Save: the card goes while macOS asks the person, and the page going elsewhere takes
    # the request, and macOS's sheet with it, back.
    app.js("make('ada')", page=True)
    card("Create a passkey")
    button("Save")
    check("Save takes the card away while macOS checks the person", gone(), app.js(TITLE))
    time.sleep(1)
    # macOS shows its sheet only to an app in a person's session. A run started by a system
    # daemon, as a release's is, has none: the check fails at once, and the page is refused.
    if subprocess.run(["launchctl", "managername"], capture_output=True, text=True).stdout.strip() == "System":
        print("note macOS can't ask the person in a system session: the check ends at once", flush=True)
        check("and with no session for macOS to ask in, the page is refused", app.js("document.title", page=True) == "refused NotAllowedError", app.js("document.title", page=True))
    else:
        check("and the page waits for the person", app.js("document.title", page=True) == "asking", app.js("document.title", page=True))
    visit(app, url)
    app.js("use()", page=True)
    check("a request taken back made no passkey", card("Sign in with a passkey"), app.js(TITLE))
    settle()
    title("t.startsWith('refused')")

    # Making a passkey: the key step, which waits for a key.
    keys(True)
    app.js("make('ada')", page=True)
    check("a page making a passkey brings the key step's card", card("Create a passkey"), app.js(TITLE))
    check("with Go's words for the site", app.js(WORDS) == "localhost wants to save a passkey on your security key. Plug it in and touch it.", app.js(WORDS))
    check("and Cancel, with no primary button", app.js(BUTTONS)[-1:] == ["Cancel"] and app.js(PRIMARY) == [], app.js(BUTTONS))
    time.sleep(1)
    check("the page waits while it shows, with no key there", app.js("document.title", page=True) == "asking" and app.js(TITLE) == "Create a passkey", (app.js("document.title", page=True), app.js(TITLE)))
    plug()
    made = title("t.startsWith('made ')||t.startsWith('refused')")
    check("a key plugged in and touched makes the passkey", made.startswith("made "), made)
    check("and the card goes", gone())
    ada = made.split(" ", 1)[-1] if made.startswith("made ") else ""
    settle()

    # The key already has it: an error to close, and the page hears why after.
    app.js(f"make('ada', {json.dumps(ada)})", page=True)
    check("a key that already has the passkey says so", card("Already saved") and app.js(WORDS) == "This security key already has a passkey for localhost.", (app.js(TITLE), app.js(WORDS)))
    check("with Close alone, the primary", app.js(BUTTONS) == ["Close"] and app.js(PRIMARY) == ["Close"], app.js(BUTTONS))
    check("the page waits until it is closed", app.js("document.title", page=True) == "asking")
    pressed = button("Close")
    refused = title("t.startsWith('refused')||t.startsWith('made')")
    check("Close tells the page the key had it already", pressed and refused == "refused InvalidStateError", refused)
    check("and the card goes", gone())
    settle()

    # Two accounts on the key: the account step, and the chosen one signs in.
    app.js("make('grace')", page=True)
    made = title("t.startsWith('made ')||t.startsWith('refused')")
    check("a second account's passkey goes on the same key", made.startswith("made "), made)
    gone()
    settle()
    app.js("use()", page=True)
    check("signing in with two accounts on the key brings the account step", card("Choose an account") and app.js(WORDS) == "Your security key has 2 accounts for localhost.", (app.js(TITLE), app.js(WORDS)))
    rows = app.js(f"{ROWS_}.map(b=>[...b.querySelectorAll('span')].map(e=>e.textContent))")
    check("a row for each, the display name over the name", sorted(map(tuple, rows)) == [("ada", "ada@example.com"), ("grace", "grace@example.com")], rows)
    chosen = rows[1][0] if len(rows) > 1 else "no second row"
    pressed = press(f"{ROWS_}[1]", "the second account")
    used = title("t.startsWith('used ')||t.startsWith('refused')")
    check("the account chosen is the one that signs in", pressed and used == f"used {chosen}", (used, chosen))
    check("and the card goes", gone())
    settle()

    # Cancel and Escape at the key step, with no key there to answer.
    keys(True)
    app.js("use()", page=True)
    check("signing in brings the key step's card", card("Sign in with a passkey") and app.js(WORDS) == "localhost wants you to sign in with your security key. Plug it in and touch it.", (app.js(TITLE), app.js(WORDS)))
    pressed = button("Cancel")
    refused = title("t.startsWith('refused')||t.startsWith('used')")
    check("Cancel refuses the page", pressed and refused == "refused NotAllowedError", refused)
    check("and the card goes", gone())
    settle()
    app.js("use()", page=True)
    shown = card("Sign in with a passkey")
    app.cmd("key", "escape")
    refused = title("t.startsWith('refused')||t.startsWith('used')")
    check("Escape does too", shown and refused == "refused NotAllowedError", (shown, refused))
    gone()

    # The page going takes its request, and the card, with it.
    app.js("use()", page=True)
    shown = card("Sign in with a passkey")
    visit(app, PAGES + "store")
    check("a page that navigates away takes its card with it", shown and gone(), shown)

    # A virtual key with its UI off, as automated tests use one: no card.
    visit(app, url)
    s.close()
    s = attach(profile, url)
    keys(False)
    plug()
    app.js("make('quiet')", page=True)
    made = title("t.startsWith('made ')||t.startsWith('refused')")
    check("with the virtual key's UI off, a passkey is made with no card", made.startswith("made ") and not app.js(f"!!{CARD}"), made)
    s.close()
    app.close()


# Web APIs whose browser side is Chrome's, each call from the web-features registry
# (core/webfeatures/registry.json). Content kills a renderer that asks for an interface the
# browser has no binder for, so a page calling one of those crashes.
KILLERS = web_features.probes()


def browser_signal_checks():
    """What a site reads to tell Chrome from an embedded or automated browser is Chrome's: the
    client hints, navigator.userAgentData and the Sec-CH-UA headers, name Chromium at the user
    agent's version on macOS, and navigator.webdriver is false. Empty client hints were
    content's default, and Google's sign-in refused the app as a browser that may not be
    secure."""
    app = App(tempfile.mkdtemp(prefix="pb-profile-"))
    visit(app, PAGES + "signals")
    # Again, so the request carries the hints the first answer's Accept-CH asked for.
    visit(app, PAGES + "signals")
    app.js("(() => { window.sig = null; navigator.userAgentData.getHighEntropyValues(['platform', 'fullVersionList'])"
           ".then(v => window.sig = JSON.stringify({brands: navigator.userAgentData.brands, platform: v.platform, "
           "full: v.fullVersionList, webdriver: navigator.webdriver, ua: navigator.userAgent})); return 1 })()", page=True)
    sig = None
    for _ in range(50):
        sig = app.js("window.sig", page=True)
        if sig:
            break
        time.sleep(0.1)
    sig = json.loads(sig or "{}")
    major = (re.search(r"Chrome/(\d+)", sig.get("ua", "")) or [None, ""])[1]
    brands = {b["brand"]: b["version"] for b in sig.get("brands", [])}
    check("navigator.userAgentData names Chromium at the user agent's version, on macOS",
          bool(major) and brands.get("Chromium") == major and sig.get("platform") == "macOS", sig)
    check("and its full version list has Chromium's", any(b["brand"] == "Chromium" and b["version"].startswith(major + ".") for b in sig.get("full", [])), sig.get("full"))
    check("navigator.webdriver is false", sig.get("webdriver") is False, sig.get("webdriver"))
    h = getattr(Pages, "signals", {})
    check("the Sec-CH-UA headers name Chromium and macOS", f'"Chromium";v="{major}"' in h.get("sec-ch-ua", "") and h.get("sec-ch-ua-platform") == '"macOS"', h)
    app.close()


def web_api_checks():
    """A page that calls each web API in the web-features registry keeps its renderer and
    gets what the registry says it gets on the Mac; a service worker that sets the badge
    keeps its process, and a copy button's click puts its text on the clipboard, as the
    registry says."""
    app = App(tempfile.mkdtemp(prefix="pb-profile-"))
    log = os.path.join(OUT, "app.log")
    wrong, unknown = [], []
    for row, probe in KILLERS:
        visit(app, PAGES + "store")
        start = os.path.getsize(log)
        app.js(f"(() => {{ window.seen = ''; {web_features.OUTCOME}(() => {probe['call']}).then(s => window.seen = s); return 1 }})()", page=True)
        time.sleep(2.5)
        try:
            title, seen = app.js("[document.title, window.seen]", page=True, timeout=5)
        except (RuntimeError, TimeoutError):
            title, seen = None, None
        with open(log, errors="replace") as f:
            f.seek(start)
            killed = [l.split("Received bad user message: ", 1)[-1].strip() for l in f if "bad Mojo message" in l]
        alive = title == "Store" and not killed
        check(f"a page that calls {probe['name']} keeps its renderer", alive, "; ".join(killed) or ("" if alive else "the page stopped answering"))
        seen = (seen or "pending") if alive else "killed"
        if row["status"]["mac"] == "unknown":
            unknown.append(f"{row['id']} ({probe['name']}): {seen}")
        wrong.append(web_features.mismatch(row, probe, "mac", seen))
    wrong = [w for w in wrong if w]
    check(f"the {len(KILLERS)} calls give what the web-features registry says for the Mac", not wrong, "; ".join(wrong))
    if unknown:
        print("  not yet known on the Mac, seen now: " + "; ".join(unknown), flush=True)
    status = web_features.feature("badge-sw")["status"]["mac"]
    visit(app, PAGES + "badge-worker")
    start = os.path.getsize(log)
    time.sleep(3)
    with open(log, errors="replace") as f:
        f.seek(start)
        killed = [l.split("Received bad user message: ", 1)[-1].strip() for l in f if "bad Mojo message" in l]
    check("a service worker that calls setAppBadge keeps its process", not killed, "; ".join(killed))
    check("as the registry's badge-sw says", bool(killed) == (status == "kills"), f"badge-sw is {status} on mac")
    status = web_features.feature("clipboard-async")["status"]["mac"]
    visit(app, PAGES + "copy")
    page_click(app, "#copy")
    try:
        app.wait("document.title !== 'Copy'", 5, page=True)
    except TimeoutError:
        pass
    title = app.js("document.title", page=True)
    check("a copy button's click puts its text on the clipboard", title == "copied", title)
    check("as the registry's clipboard-async says", (title == "copied") == (status in ("works", "partly")), f"clipboard-async is {status} on mac")
    app.close()


def sparks_checks():
    """Sparks run in the page's own world, as other userscript managers run a script that
    asks for no extension APIs: the page sees what a spark sets, and a spark at document
    start can replace what the page calls. On a page whose policy allows only its own
    styles, the policy's style rules give way where a spark runs, so its <style> applies,
    and the owner's sparks, as he wrote them (core/sparks/testdata/site-sparks), draw there.
    On a page that wants Trusted Types, they give way too, so a spark's innerHTML works. A
    profile is given the built-in sparks, off, and one switched on runs."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    os.makedirs(os.path.join(profile, "sparks"))
    probe = ("// ==UserScript==\n// @name Probe\n// @match <all_urls>\n// ==/UserScript==\n"
             "window.sparkProbe = 1;\n"
             "const s = document.createElement('style');\n"
             "s.textContent = 'html { --spark: on; }';\n"
             "document.head.append(s);\n"
             "try { const d = document.createElement('div'); d.innerHTML = '<b>x</b>'; document.documentElement.dataset.html = 'set'; }\n"
             "catch (e) { document.documentElement.dataset.html = e.name; }\n")
    early = ("// ==UserScript==\n// @name Early\n// @match <all_urls>\n// @run-at document-start\n// ==/UserScript==\n"
             "window.open = function () { return null; };\n")
    sources = {"0000000a": probe, "0000000d": early}
    his = os.path.join(ROOT, "core/sparks/testdata/site-sparks")
    for i, name in enumerate(["Limit Screen Time.user.js", "Page Cleanup.user.js"]):
        with open(os.path.join(his, name)) as f:
            sources[f"0000000{'bc'[i]}"] = f.read()
    for sid, source in sources.items():
        with open(os.path.join(profile, "sparks", sid + ".user.js"), "w") as f:
            f.write(source)
    with open(os.path.join(profile, "sparks", "sparks.json"), "w") as f:
        json.dump([{"id": sid, "on": True} for sid in sources], f)
    app = App(profile)
    visit(app, PAGES + "strict")
    ran = holds(app, "getComputedStyle(document.documentElement).getPropertyValue('--spark').trim() === 'on'", page=True)
    check("a spark's <style> applies on a page whose policy allows only its own styles", ran)
    check("the page sees what a spark sets, as it runs in the page's own world", app.js("typeof window.sparkProbe", page=True) == "number")
    check("a spark at document start replaces what the page then calls", holds(app, "document.title === 'open replaced'", page=True),
          app.js("document.title", page=True))
    styled = holds(app, "!!document.getElementById('pc-styles') && !!document.getElementById('pc-fab') && "
                        "getComputedStyle(document.getElementById('pc-fab-container')).position === 'fixed'", page=True)
    check("Page Cleanup, as he wrote it, draws its button there, styled", styled)
    # Limit Screen Time keeps each site's time in the site's own storage; with the day's
    # limit already spent, it covers the page as it loads.
    app.js("localStorage.setItem('screentime_date_' + location.hostname, new Date().toDateString()), "
           "localStorage.setItem('screentime_seconds_' + location.hostname, '100000'), 1", page=True)
    visit(app, PAGES + "strict")
    blocked = holds(app, "!!document.getElementById('screentime-blocker') && "
                         "getComputedStyle(document.getElementById('screentime-blocker')).position === 'fixed'", page=True)
    check("Limit Screen Time, as he wrote it, covers the page once the day's limit is spent, styled", blocked)
    # On a page that wants Trusted Types, as YouTube does, they give way where a spark runs.
    visit(app, PAGES + "trusted")
    holds(app, "!!document.documentElement.dataset.html", page=True)
    html = app.js("document.documentElement.dataset.html", page=True)
    check("a spark's innerHTML works on a page that wants Trusted Types", html == "set", html)
    app.close()
    # The profile was given the built-in sparks, off; Page cleanup switched on draws its
    # button on the strict page too.
    path = os.path.join(profile, "sparks", "sparks.json")
    with open(path) as f:
        kept = json.load(f)
    builtins = [s for s in kept["sparks"] if s["id"] in kept.get("builtins", [])]
    check("the profile was given the three built-in sparks, each off", len(builtins) == 3 and not any(s["on"] for s in builtins), kept)
    if len(builtins) == 3:
        builtins[2]["on"] = True
        with open(path, "w") as f:
            json.dump(kept, f)
        app = App(profile)
        visit(app, PAGES + "strict")
        check("Page cleanup, built in and switched on, draws its button there", holds(app, "!!document.getElementById('pb-page-cleanup')", page=True))
        app.close()


def duplicate_checks():
    """Duplicate Tab gives a copy that loads its page, as the tab has it, not one failed
    ERR_BLOCKED_BY_CLIENT: its first navigation once asked Go about it before Go had it
    (docs/fragility.md, "A duplicated tab that failed to load")."""
    app = App(tempfile.mkdtemp(prefix="pb-profile-"))
    send = lambda msg: app.js(f"(chrome.send('shell',[{json.dumps(json.dumps(msg))}]),1)")
    send({"t": "navigate", "text": PAGES + "first"})
    app.wait(f"{ROWS}===1", 30)
    first = tabs_of(app)["active"]
    app.wait("location.pathname==='/first'&&document.readyState==='complete'", 30, tab=first)
    for n in range(3):
        send({"t": "run", "text": "tab.duplicate"})
        app.wait(f"{ROWS}==={n + 2}", 30)
        copy = tabs_of(app)["active"]
        deadline = time.time() + 30
        while (t := next(x for x in tabs_of(app)["tabs"] if x["id"] == copy)).get("loading") or not t.get("title"):
            if time.time() > deadline or t.get("error"):
                break
            time.sleep(0.1)
        check(f"a duplicated tab loads its page ({n + 1})", copy != first and not t.get("error") and t.get("url", "").startswith(PAGES + "first") and t.get("title") not in ("", "127.0.0.1"),
              {k: t.get(k) for k in ("url", "title", "error", "errorCode", "loading")})
    app.close()

def view_source_checks():
    """View Source's tab has the page's source and says so: its address is view-source: and
    the page's, in the address bar, its duplicate and its history; and a blocked host's
    source is blocked as the host is."""
    app = App(tempfile.mkdtemp(prefix="pb-profile-"))
    send = lambda msg: app.js(f"(chrome.send('shell',[{json.dumps(json.dumps(msg))}]),1)")
    send({"t": "navigate", "text": PAGES + "first"})
    app.wait(f"{ROWS}===1", 30)
    app.wait("location.pathname==='/first'&&document.readyState==='complete'", 30, tab=tabs_of(app)["active"])

    def settled(tab):
        deadline = time.time() + 30
        while (t := next(x for x in tabs_of(app)["tabs"] if x["id"] == tab)).get("loading") or not (t.get("title") or t.get("error") or t.get("blocked")):
            if time.time() > deadline:
                break
            time.sleep(0.1)
        return t

    send({"t": "run", "text": "dev.viewSource"})
    app.wait(f"{ROWS}===2", 30)
    source = settled(tabs_of(app)["active"])
    check("View Source's tab has the page's address with view-source: before it", source.get("url") == "view-source:" + PAGES + "first" and not source.get("error"),
          {k: source.get(k) for k in ("url", "title", "error")})
    send({"t": "run", "text": "tab.duplicate"})
    app.wait(f"{ROWS}===3", 30)
    copy = settled(tabs_of(app)["active"])
    check("and its duplicate is the source too", copy.get("url") == "view-source:" + PAGES + "first" and not copy.get("error"),
          {k: copy.get(k) for k in ("url", "title", "error")})
    seen = len(Pages.seen)
    send({"t": "navigate", "text": f"view-source:http://{BLOCKED_LOCAL}:{pages.server_port}/first"})
    blocked = settled(tabs_of(app)["active"])
    check("a blocked host's source is blocked as the host is", blocked.get("blocked") and not [x for x in Pages.seen[seen:] if x.startswith(BLOCKED_LOCAL)],
          {k: blocked.get(k) for k in ("url", "blocked", "error")})
    app.close()


def quit_checks():
    """Quitting from the Dock is a clean stop: crash.log, which says the app is running, is
    gone, and the next start has no crash to report."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = App(profile)
    running = os.path.exists(os.path.join(profile, "crash.log"))
    subprocess.run(["/usr/bin/osascript", "-l", "JavaScript", "-e", TERMINATE % app.proc.pid], capture_output=True, timeout=20)
    try:
        app.proc.wait(timeout=30)
        ended = True
    except subprocess.TimeoutExpired:
        ended = False
    App.running.remove(app)
    left = sorted(n for n in os.listdir(profile) if n.startswith("crash"))
    check("a quit from the Dock ends the app and leaves no crash.log", running and ended and left == [], (running, ended, left))
    if not ended:
        app.proc.kill()
        return
    app = App(profile)
    kept = sorted(n for n in os.listdir(profile) if n.startswith("crash-"))
    check("and the next start has no crash to report", kept == [], kept)
    app.close()


def internal_checks():
    """Our own pages are tabs (docs/windows.md), settings first. It opens beside the active
    tab as a tab of ours, in our pages' context, one a window; it is shown, laid out, given
    the keyboard and closed as any tab is; New Tab from it opens in the workspace it was
    opened from. And the one navigation rule, however a navigation starts in it: our pages
    in place, any other address in a new web tab beside it; a web tab asked for one of ours
    shows the settings tab. That extensions don't see it is checked in
    tools/drive_extensions.py, part internal."""
    work = "0a1b2c3d"
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    with open(os.path.join(profile, "workspaces.json"), "w") as f:
        json.dump({"version": 2, "workspaces": [{"id": "", "name": "Main", "color": "#3470d0"}, {"id": work, "name": "Work", "color": "#d03434"}]}, f)
    app = App(profile, ["--remote-debugging-port=0"])

    def order():
        return [t["id"] for t in tabs_of(app)["tabs"]]

    def ours():
        return [t for t in tabs_of(app)["tabs"] if t.get("page") == "settings"]

    def settings_ready():
        deadline = time.time() + 15
        while app.cmd("evalsettings", "!!document.querySelector('h1')") != "true":
            if time.time() > deadline:
                raise TimeoutError("the settings page never came")
            time.sleep(0.1)

    # 1. A tab of ours, beside the active tab, opened from it, one a window. The window
    # starts on a new tab, which the first page loads in.
    first = from_box(app, PAGES + "links")
    in_work = open_in(app, work, PAGES + "store")
    click(app, f'[role=tab][data-tab="{first}"]')
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{first}\"]').getAttribute('aria-selected')==='true'")
    click(app, f'[role=tab][data-tab="{in_work}"]')
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{in_work}\"]').getAttribute('aria-selected')==='true'")
    press_settings(app)
    settings_ready()
    found = ours()
    settings = found[0]["id"] if found else 0
    check("Settings opens a tab of ours beside the active tab, made active, remembering the workspace it was opened from",
          len(found) == 1 and order() == [first, in_work, settings] and tabs_of(app)["active"] == settings and found[0]["workspace"] == work
          and found[0]["url"] == "chrome://settings/", tabs_of(app))
    check("its page is ours, with the settings page's channel", app.cmd("evalsettings", "typeof chrome.send") == '"function"')
    click(app, f'[role=tab][data-tab="{first}"]')
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{first}\"]').getAttribute('aria-selected')==='true'")
    press_settings(app)
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{settings}\"]').getAttribute('aria-selected')==='true'")
    check("Settings again shows the window's settings tab, and opens no other", [t["id"] for t in ours()] == [settings] and tabs_of(app)["active"] == settings, tabs_of(app))

    # 2. Shown, laid out, given the keyboard and closed as any tab is.
    visible = json.loads(app.cmd("visible"))
    check("the window shows it where the layout puts a page, told it is shown",
          [v["tab"] for v in visible] == [settings] and visible[0]["shown"] and not visible[0]["hidden"] and app.cmd("hit", 600, 300) == "page", visible)
    check("it is visible to itself", app.cmd("evalsettings", "document.visibilityState") == '"visible"')
    # Into the shell first: a tab of ours has a title, not an address field.
    click(app, "[role=tab][aria-selected=true]")
    app.cmd("click", 600, 300)
    deadline = time.time() + 5
    while app.cmd("evalsettings", "document.hasFocus()") != "true" and time.time() < deadline:
        time.sleep(0.05)
    check("a press in its page gives it the keyboard", app.cmd("evalsettings", "document.hasFocus()") == "true")
    check("the window is titled after it", app.cmd("title").startswith("Settings"), app.cmd("title"))
    # Its title and icon are its page's, as a web page's are.
    png = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAEUlEQVR42mO44GDwH4QZYAwAT/QI/dxDFCMAAAAASUVORK5CYII="
    app.cmd("evalsettings", f"(document.title='Settings › Test',document.head.append(Object.assign(document.createElement('link'),{{id:'test-icon',rel:'icon',href:{json.dumps(png)}}})),1)")
    deadline = time.time() + 10
    while time.time() < deadline and not ((t := ours()) and t[0]["title"] == "Settings › Test" and t[0].get("favicon", "").startswith("data:image/png")):
        time.sleep(0.1)
    t = ours()
    check("a title and an icon its page sets are the tab's", bool(t) and t[0]["title"] == "Settings › Test" and t[0].get("favicon", "").startswith("data:image/png"), t)
    app.cmd("evalsettings", "(document.title='Settings',document.getElementById('test-icon').remove(),1)")
    # New Tab goes to a new tab, with Standard's command box, and an address there loads in it.
    new_tab(app, PAGES + "links")
    made = tabs_of(app)
    check("New Tab from it opens in the workspace it was opened from", next(t for t in made["tabs"] if t["id"] == made["active"]).get("workspace") == work, made)
    app.cmd("key", "cmd+w")
    app.wait(f"{ROWS}===3")
    click(app, f'[role=tab][data-tab="{settings}"]')
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{settings}\"]').getAttribute('aria-selected')==='true'")

    # 3. The one navigation rule, from every way a navigation starts there.
    def opened_beside(start, what):
        before = order()
        start()
        deadline = time.time() + 15
        while len(order()) == len(before) and time.time() < deadline:
            time.sleep(0.05)
        now = tabs_of(app)
        new = [t for t in now["tabs"] if t["id"] not in before]
        ok = (len(new) == 1 and not new[0].get("page") and new[0].get("workspace") == work
              and [t["id"] for t in now["tabs"]].index(new[0]["id"]) == [t["id"] for t in now["tabs"]].index(settings) + 1
              and now["active"] == new[0]["id"])
        if ok:
            app.wait("location.pathname==='/store'&&document.readyState==='complete'", tab=new[0]["id"])
        stayed = app.cmd("evalsettings", "location.href")
        check(f"{what} opens in a new web tab beside it, in its workspace, and the settings tab stays", ok and stayed == '"chrome://settings/"', (now, stayed))
        if new:
            app.cmd("key", "cmd+w")
            app.wait(f"{ROWS}==={len(before)}")
        click(app, f'[role=tab][data-tab="{settings}"]')
        app.wait(f"document.querySelector('[role=tab][data-tab=\"{settings}\"]').getAttribute('aria-selected')==='true'")

    page = json.dumps(PAGES + "store")
    opened_beside(lambda: app.cmd("evalsettings", f"location.href={page}"), "a web address its page goes to")
    opened_beside(lambda: app.cmd("evalsettings", f"(()=>{{const a=document.createElement('a');a.href={page};a.target='_blank';document.body.append(a);a.click();a.remove()}})()"),
                  "a link with target=_blank")
    opened_beside(lambda: app.cmd("evalsettings", f"(window.open({page}),1)"), "window.open")
    port = open(os.path.join(profile, "DevToolsActivePort")).read().split()[0]
    target = next(t for t in json.load(urllib.request.urlopen(f"http://127.0.0.1:{port}/json/list")) if t["url"].startswith("chrome://settings/"))
    devtools = ws_open(target["webSocketDebuggerUrl"])[0]
    opened_beside(lambda: cdp(devtools, 1, "Page.navigate", {"url": PAGES + "store"}), "a DevTools navigation")
    devtools.close()

    # The settings tab, the focused one, is asked for its address by what content committed:
    # an eval sent to its page while that page is being replaced is never answered.
    app.cmd("evalsettings", "location.href='chrome://settings/flags'")
    deadline = time.time() + 10
    while not ((page := json.loads(app.cmd("committed")))["url"] == "chrome://settings/flags" and not page["loading"]) and time.time() < deadline:
        time.sleep(0.1)
    settings_ready()
    check("one of our pages its page goes to opens in place", app.cmd("evalsettings", "location.href") == '"chrome://settings/flags"' and len(order()) == 3, tabs_of(app))

    click(app, f'[role=tab][data-tab="{in_work}"]')
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{in_work}\"]').getAttribute('aria-selected')==='true'")
    focus_bar(app)
    app.cmd("type", "chrome://settings")
    app.cmd("key", "enter")
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{settings}\"]').getAttribute('aria-selected')==='true'")
    stayed = app.js("location.href", tab=in_work)
    check("chrome://settings typed in a web tab shows the settings tab, and the web tab stays", len(order()) == 3 and stayed == PAGES + "store", (tabs_of(app), stayed))
    print(app.cmd("capture", os.path.join(OUT, "internal-settings-tab.png")), flush=True)

    app.cmd("key", "cmd+w")
    deadline = time.time() + 10
    while not app.cmd("evalsettings", "1").startswith("ERR") and time.time() < deadline:
        time.sleep(0.05)
    check("Cmd+W closes it, as any tab", not ours() and len(order()) == 2, tabs_of(app))
    app.close()

def links_checks():
    """Links to other sites (docs/site-features.md#checking-it): under Block each of the ten
    ways a page leaves for another site goes nowhere, the page staying with its count risen, no
    history and one tab, and the address field counting the tabs it closed; the person's own
    way still goes; under Allow every way goes."""
    profile = tempfile.mkdtemp(prefix="pb-links-")
    app = App(profile)
    own = PAGES + "leave"
    origin = PAGES.rstrip("/")
    other_host = "localhost"

    def setting(value):
        """Sets the site's Links to other sites as its panel does, and waits for Go to keep it:
        sites.json holds only what isn't the default, Allow."""
        app.js(f"(chrome.send('shell',[{json.dumps(json.dumps({'t': 'setSiteSetting', 'origin': origin, 'kind': 'links', 'value': value}))}]),1)")
        deadline = time.time() + 10
        while time.time() < deadline:
            try:
                kept = json.load(open(os.path.join(profile, "sites.json"))).get(origin, {}).get("links", "allow")
            except (OSError, ValueError):
                kept = "allow"
            if kept == value:
                return
            time.sleep(0.05)
        raise TimeoutError(f"Go never kept links={value} for {origin}")

    def reached():
        return [p for p in Pages.seen if p.startswith(other_host + "/")]

    def popups():
        return app.js("document.querySelector('.popups')?.getAttribute('aria-label')||''")

    def fresh():
        """One tab, on the page, nothing from the other site yet."""
        while len(tabs_of(app)["tabs"]) > 1:
            rows = len(tabs_of(app)["tabs"])
            app.cmd("key", "cmd+w")
            app.wait(f"{ROWS}==={rows - 1}")
        visit(app, own)
        Pages.seen.clear()

    visit(app, own)
    setting("block")
    Pages.seen.clear()
    blocked = 0
    for n, words, opens, paths in links_page.WAYS:
        page_click(app, f"#l{n}")
        alive = f"127.0.0.1/alive?{n}"
        deadline = time.time() + 10
        while alive not in Pages.seen and time.time() < deadline:
            time.sleep(0.05)
        if opens:
            blocked += 1
            want = links_page.popups_word(blocked)
            deadline = time.time() + 5
            while popups() != want and time.time() < deadline:
                time.sleep(0.05)
        page = app.js("[location.href, count.textContent]", page=True)
        tabs = tabs_of(app)["tabs"]
        check(f"under Block, {words} goes nowhere: the page stays, its count risen, one tab, the other site never asked"
              + (f", and the address field says {links_page.popups_word(blocked)}" if opens else ""),
              alive in Pages.seen and page == [own, f"clicks {n}"] and len(tabs) == 1 and not reached()
              and (not opens or popups() == links_page.popups_word(blocked)),
              (page, len(tabs), reached(), popups()))
    check("under Block, nothing went into the tab's history: Back stays off",
          app.js("document.querySelector('[aria-label=Back]').disabled") is True)
    visit(app, OTHER + "/typed")
    check("under Block, an address the person types still goes", "localhost/typed" in Pages.seen, Pages.seen[-3:])

    setting("allow")
    for n, words, opens, paths in links_page.WAYS:
        fresh()
        page_click(app, f"#l{n}")
        deadline = time.time() + 10
        # A form's GET asks for its path with a query.
        went = lambda: all(other_host + p in [s.split("?")[0] for s in Pages.seen] for p in paths)
        while not went() and time.time() < deadline:
            time.sleep(0.05)
        check(f"under Allow, {words} goes to the other site", went(), reached())
    app.close()


def app_link_checks():
    """A page's link to the app's own project-b://sync (docs/sync.md, The link): pressed, it
    opens Settings at Sync with the link's Dialog, the host shown and the address filled in,
    and nothing reaches the address before Connect; a page's script going there, a press in a
    frame, a redirect there and another project-b: link each do nothing, the page staying."""
    app = App(tempfile.mkdtemp(prefix="pb-applink-"))

    def settings_tabs():
        return [t for t in tabs_of(app)["tabs"] if t.get("page") == "settings"]

    def nothing(words, url):
        time.sleep(2)
        page = json.loads(app.cmd("committed"))
        check(f"{words} does nothing: no settings tab, the page where it was", not settings_tabs() and page["url"] == url, (settings_tabs(), page))

    visit(app, PAGES + "applink-script")
    app.wait("document.title==='went'", page=True)
    nothing("a page's script going to the link, with no press,", PAGES + "applink-script")
    visit(app, PAGES + "applink-frame")
    app.wait("!!document.getElementById('frame').contentDocument?.querySelector('a')", page=True)
    page_click(app, "#frame")
    app.wait("document.title==='pressed'", page=True)
    nothing("a press on the link in a frame", PAGES + "applink-frame")
    visit(app, PAGES + "applink")
    Pages.seen.clear()
    page_click(app, "#redirect")
    deadline = time.time() + 10
    while "127.0.0.1/applink-redirect" not in Pages.seen and time.time() < deadline:
        time.sleep(0.05)
    nothing("a press on a link the server redirects to it", PAGES + "applink")
    page_click(app, "#other")
    nothing("a press on another project-b: link", PAGES + "applink")

    web = tabs_of(app)["active"]
    page_click(app, "#sync")
    dialog = "document.querySelector('dialog[open]')"
    opened = in_settings(app, f"{dialog}?.getAttribute('aria-label')==='Sync with sync.example.test?'", 15)
    check("a press on the sync link opens Settings at Sync with the link's Dialog, the host shown",
          opened and len(settings_tabs()) == 1 and in_settings(app, "(()=>{const r=document.querySelector('[data-part=sync]').getBoundingClientRect();return r.top>=0&&r.top<innerHeight})()"))
    check("with the address filled in", in_settings(app, "document.getElementById('sync-address').value") == "https://sync.example.test/sync/abc")
    check("and the host in bold in its title", in_settings(app, f"{dialog}.querySelector('.ui-dialog-title').textContent.trim()") == "Sync with sync.example.test?")
    print(app.cmd("capture", os.path.join(OUT, "applink-dialog.png")), flush=True)
    app.cmd("evalsettings", f"[...{dialog}.querySelectorAll('button')].find(b=>b.textContent==='Cancel').click()")
    in_settings(app, f"!{dialog}")

    click(app, f'[role=tab][data-tab="{web}"]')
    app.wait(f"document.querySelector('[role=tab][data-tab=\"{web}\"]').getAttribute('aria-selected')==='true'")
    page_click(app, "#counted")
    host = Pages.counted_link.split("%2F%2F")[1].split("%2F")[0].replace("%3A", ":")
    in_settings(app, f"{dialog}?.getAttribute('aria-label')==='Sync with {host}?'", 15)
    time.sleep(2)
    check("nothing reaches the link's address before Connect", not counted, len(counted))
    app.cmd("evalsettings", f"[...{dialog}.querySelectorAll('button')].find(b=>b.textContent==='Connect').click()")
    deadline = time.time() + 10
    while not counted and time.time() < deadline:
        time.sleep(0.05)
    check("and Connect reaches it", bool(counted))
    app.close()


if __name__ == "__main__":
    os.makedirs(OUT, exist_ok=True)
    parts = {"driver": [driver_checks], "browsing": [browsing_checks], "actions": [action_checks], "settings": [settings_checks], "timeline": [timeline_checks], "cookies": [cookie_checks], "visible": [visible_tabs_checks], "windows": [window_checks], "empty": [last_tab_checks], "move": [move_checks], "bounds": [bounds_checks], "profiles": [profile_checks], "copies": [copies_checks], "picker": [picker_checks], "relaunch": [relaunch_checks], "workspaces": [workspace_checks, workspace_menu_checks, workspace_delete_checks], "sitedata": [site_data_checks], "quiterase": [quit_erasure_checks], "shells": [shell_checks], "glide": [glide_checks], "webapis": [web_api_checks], "signals": [browser_signal_checks], "passkeys": [passkey_checks], "sparks": [sparks_checks], "quit": [quit_checks], "internal": [internal_checks], "links": [links_checks], "viewsource": [view_source_checks], "duplicate": [duplicate_checks], "applink": [app_link_checks]}
    run_parts(parts.get(" ".join(sys.argv[1:]), [version_checks, driver_checks, timeline_checks, browsing_checks, action_checks, settings_checks, internal_checks, visible_tabs_checks, window_checks, last_tab_checks, move_checks, bounds_checks, profile_checks, copies_checks, picker_checks, relaunch_checks, cookie_checks, workspace_checks, workspace_menu_checks, workspace_delete_checks, site_data_checks, quit_erasure_checks, shell_checks, glide_checks, browser_signal_checks, web_api_checks, passkey_checks, sparks_checks, links_checks, view_source_checks, duplicate_checks, app_link_checks, quit_checks]))
