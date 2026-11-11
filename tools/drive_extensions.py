#!/usr/bin/env python3
"""Tests extensions in the real app, built with EXTENSIONS=1, through its driver.

It starts the mock store on the folder tools/fetch-extensions.sh and
tools/pack-test-extension.py fill, and in the app's own UI installs our test extension and
uBlock Origin Lite from it, loads the test extension unpacked, pins and unpins, opens the
popup, and starts the app again. It needs the network: uBlock Origin Lite is seen blocking
a real ad host. Run it from the repo root:

    PB_BIN=chromium/src/out/ext/project_b python3 tools/drive_extensions.py

Results go to build/real/, as tools/drive_first_app.py's do.
"""

import http.server
import json
import os
import subprocess
import sys
import tempfile
import threading
import time

import drive_first_app
from drive_first_app import App, check, click, run_parts, visit

ROOT = drive_first_app.ROOT
STORE_DIR = os.path.join(ROOT, ".tools/extensions")
STORE = "http://127.0.0.1:8767"
TEST_ID = "oepjjojhhaclfhmopocgmdfbeiffpheh"
TEST_NAME = "Project B test extension"
UBOL_ID = "ddkjiahejlhfcafbddmgiahcphecmpfh"
UBOL_NAME = "uBlock Origin Lite"
# An address every ad blocker's default lists block outright.
AD_URL = "https://ad.doubleclick.net/ddm/ad/pb"


def fetch_ad(app):
    """Whether a request from the shown page for the ad host gets there: "reached", or
    "blocked" when it fails or is answered by something else, as an ad blocker does."""
    app.js(f"window.pbAd='';fetch({json.dumps(AD_URL)},{{mode:'no-cors'}}).then(r=>window.pbAd=r.type==='opaque'?'reached':'blocked',()=>window.pbAd='blocked');0", page=True)
    return app.wait("window.pbAd", 30, page=True)


class Pages(http.server.BaseHTTPRequestHandler):
    """A page that asks for the ad script, as a page with ads does."""

    def do_GET(self):
        body = f'<!doctype html><title>Ads</title><script src="{AD_URL}"></script>'.encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


def settings(app, expr, timeout=20):
    """Evaluates in the settings page."""
    r = app.cmd("evalsettings", expr.replace("\n", " "), timeout=timeout)
    # Nothing to evaluate in while the settings page is closed.
    return json.loads(r) if r != "" and not r.startswith("ERR") else None


def settings_wait(app, expr, timeout=20):
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        last = settings(app, expr)
        if last:
            return last
        time.sleep(0.05)
    raise TimeoutError(f"settings: {expr} last={last}")


def settings_click(app, selector):
    x, y = settings(app, f"(()=>{{const r=document.querySelector({json.dumps(selector)}).getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]}})()")
    app.cmd("click", x, y)


def menu_click(app, selector):
    """Clicks something in the extensions menu, once the window knows the menu is the
    shell's: its rectangle reaches the window a frame or two after it is drawn."""
    x, y = drive_first_app.center(app, selector)
    deadline = time.time() + 5
    while app.cmd("hit", x, y) != "shell":
        if time.time() > deadline:
            raise TimeoutError(f"{selector} never became the shell's: a press there lands on {app.cmd('hit', x, y)}")
        time.sleep(0.02)
    app.cmd("click", x, y)


def row(name):
    """An installed extension's row, or a store item's, by name."""
    return f"[aria-label={json.dumps(name)}]"


INSTALLED = "[...document.querySelectorAll('section')[0].querySelectorAll('.pbx-extension')].map(e=>e.getAttribute('aria-label'))"
STORE_BUTTON = "[...document.querySelectorAll('section')[1].querySelectorAll('.pbx-extension')].find(e=>e.getAttribute('aria-label')===%s).querySelector('button')"


def open_extension_settings(app):
    click(app, "[aria-label='Extensions']")
    app.wait("!!document.querySelector('.pbx-menu')")
    menu_click(app, ".pbx-manage")
    settings_wait(app, "!!document.querySelector('.pbx-url')", 30)


def close_settings(app):
    """Escape closes the settings page, which is gone when a press in the middle of the
    window no longer lands on it."""
    app.cmd("key", "escape")
    deadline = time.time() + 10
    while app.cmd("hit", 500, 300) == "settings":
        if time.time() > deadline:
            raise TimeoutError("the settings page stayed open")
        time.sleep(0.05)


def install_from_store(app, name):
    button = STORE_BUTTON % json.dumps(name)
    settings_wait(app, f"!!({button})")
    x, y = settings(app, f"(()=>{{const r=({button}).getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]}})()")
    app.cmd("click", x, y)
    settings_wait(app, "!!document.querySelector('.pbx-prompt')", 60)
    prompt = settings(app, "document.querySelector('.pbx-prompt').innerText")
    settings_click(app, ".pbx-prompt .pbx-primary")
    settings_wait(app, f"({INSTALLED}).includes({json.dumps(name)})", 60)
    return prompt


def buttons(app):
    """The pinned buttons' labels."""
    return app.js("[...document.querySelectorAll('.pbx-action')].map(e=>e.getAttribute('aria-label'))")


def pin(app, name, on):
    click(app, "[aria-label='Extensions']")
    app.wait("!!document.querySelector('.pbx-menu')")
    menu_click(app, f"[aria-label={json.dumps(('Pin ' if on else 'Unpin ') + name)}]")
    app.wait(f"document.querySelectorAll('.pbx-action').length>=0&&!!document.querySelector('[aria-label={json.dumps(('Unpin ' if on else 'Pin ') + name)}]')")
    app.cmd("key", "escape")
    app.wait("!document.querySelector('.pbx-menu')")


def popup_box(app):
    return [float(v) for v in app.cmd("popup").split()]


def open_popup(app):
    """Presses the pinned button and returns the popup's box once it has its size."""
    click(app, ".pbx-action")
    deadline = time.time() + 20
    while popup_box(app)[2] <= 25 and time.time() < deadline:
        time.sleep(0.1)
    return popup_box(app)


def extension_checks():
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = App(profile)
    app.wait("!!document.querySelector('input')", 30)
    check("the toolbar has the extensions button", app.js("!!document.querySelector('[aria-label=Extensions]')"))

    # Before any extension, the page gets its ad script.
    visit(app, PAGE)
    check("with no extension the ad host answers", fetch_ad(app) == "reached")

    open_extension_settings(app)
    check("the extension manager opens from the toolbar's menu", True)
    check("nothing is installed in a new profile", settings(app, INSTALLED) == [])

    # The store.
    settings_click(app, ".pbx-url")
    app.cmd("type", STORE)
    app.cmd("key", "enter")
    settings_wait(app, "document.querySelectorAll('section')[1].querySelectorAll('.pbx-extension').length>=6", 30)
    check("the store lists the fetched extensions and ours", True)

    prompt = install_from_store(app, TEST_NAME)
    check("the install prompt names the extension and its permissions", TEST_NAME in prompt and "tabs" in prompt and "storage" in prompt, prompt)
    check("the test extension installs from the store", True)
    prompt = install_from_store(app, UBOL_NAME)
    check("uBlock Origin Lite installs from the store", "declarativeNetRequest" in prompt, prompt)
    check("the store says both are installed", settings(app, f"({STORE_BUTTON % json.dumps(UBOL_NAME)}).textContent.trim()") == "Installed")

    # Load unpacked, through the chooser the driver answers for the user.
    app.cmd("choose", os.path.join(ROOT, "tools/test-extension"))
    settings_click(app, ".pbx-heading .text-button")
    settings_wait(app, f"({INSTALLED}).filter(n=>n==={json.dumps(TEST_NAME)}).length===2", 30)
    check("Load unpacked adds the folder's extension beside the packaged one", True)

    close_settings(app)

    # The content script runs in a page, talks to its service worker, and the worker
    # counts in storage and sets the badge through Go.
    visit(app, drive_first_app.URL_A)
    app.wait("!!document.documentElement.dataset.pbTestExtension", 30, page=True)
    answer = json.loads(app.js("document.documentElement.dataset.pbTestExtension", page=True))
    check("a content script runs and its message is answered by the service worker", answer["pages"] >= 1 and answer["tab"] > 0, answer)

    # uBlock Origin Lite blocks the ad host.
    visit(app, PAGE)
    check("uBlock Origin Lite blocks a known ad host", fetch_ad(app) == "blocked")

    # Pinning.
    check("no button is pinned at first", buttons(app) == [])
    pin(app, TEST_NAME, True)
    check("pinning puts the extension's button in the toolbar", buttons(app) == [TEST_NAME], buttons(app))
    app.wait("!!document.querySelector('.pbx-action .pbx-badge')", 20)
    check("the button shows the badge its service worker set", int(app.js("document.querySelector('.pbx-action .pbx-badge').textContent")) >= 1)

    # The popup: a view of its own under the button, running the extension's page.
    x, y, w, h = open_popup(app)
    bx, by, bw, bh = app.js("(()=>{const r=document.querySelector('.pbx-action').getBoundingClientRect();return [r.x,r.y,r.width,r.height]})()")
    check("the popup opens under its button", w >= 240 and h > 25 and abs((x + w) - (bx + bw)) <= 1 and y >= by + bh, (x, y, w, h, bx, by, bw, bh))
    deadline = time.time() + 20
    pages = ""
    while not pages and time.time() < deadline:
        pages = json.loads(app.cmd("evalpopup", "document.getElementById('pages').textContent") or '""')
        time.sleep(0.1)
    tabs = json.loads(app.cmd("evalpopup", "[...document.querySelectorAll('li')].map(e=>e.textContent)"))
    check("the popup's page reads storage and chrome.tabs.query", int(pages) >= 1 and any(PAGE in t for t in tabs), (pages, tabs))
    check("the shell marks the button whose popup is open", app.js("!!document.querySelector('.pbx-action.pbx-open')"))

    # A second press on the button closes the popup, and it stays closed.
    click(app, ".pbx-action")
    deadline = time.time() + 10
    while popup_box(app)[2] != 0 and time.time() < deadline:
        time.sleep(0.05)
    time.sleep(0.5)
    check("a second press on the button closes the popup", popup_box(app)[2] == 0 and not app.js("!!document.querySelector('.pbx-action.pbx-open')"), popup_box(app))
    x, y, w, h = open_popup(app)
    check("and a third opens it again", w > 25, (x, y, w, h))

    # Its button calls chrome.tabs.create.
    rows = app.js(drive_first_app.ROWS)
    px, py = json.loads(app.cmd("evalpopup", "(()=>{const r=document.getElementById('open').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()"))
    app.cmd("click", x + px, y + py)
    app.wait(f"{drive_first_app.ROWS}==={rows + 1}", 20)
    check("a click in the popup reaches it, and chrome.tabs.create opens a tab", True)
    deadline = time.time() + 10
    while popup_box(app)[2] != 0 and time.time() < deadline:
        time.sleep(0.1)
    check("the popup closes when the keyboard goes elsewhere", popup_box(app)[2] == 0 and not app.js("!!document.querySelector('.pbx-action.pbx-open')"))
    # The new tab takes the keyboard for its page once it has loaded.
    app.wait(drive_first_app.TAB_READY, 30)
    app.wait("document.hasFocus()", 15, page=True)

    pin(app, TEST_NAME, False)
    check("unpinning takes the button away", buttons(app) == [])
    pin(app, UBOL_NAME, True)
    app.close()

    # Everything survives a restart: what is installed is the engine's to remember, the
    # pin is Go's.
    app = App(profile)
    app.wait("!!document.querySelector('input')", 30)
    app.wait("document.querySelectorAll('.pbx-action').length===1", 30)
    check("the pin survives a restart", buttons(app) == [UBOL_NAME], buttons(app))
    visit(app, PAGE)
    check("uBlock Origin Lite still blocks after a restart", fetch_ad(app) == "blocked")
    visit(app, drive_first_app.URL_A)
    app.wait("!!document.documentElement.dataset.pbTestExtension", 30, page=True)
    check("the test extension still runs after a restart", True)

    # Disable and remove.
    open_extension_settings(app)
    check("the extension manager lists all three after a restart", sorted(settings(app, INSTALLED)) == sorted([TEST_NAME, TEST_NAME, UBOL_NAME]), settings(app, INSTALLED))
    settings_click(app, f"{row(UBOL_NAME)} .switch")
    settings_wait(app, f"!document.querySelector({json.dumps(row(UBOL_NAME) + ' .switch')}).checked")
    close_settings(app)
    app.wait("document.querySelectorAll('.pbx-action').length===0", 20)
    check("turning an extension off takes its button away", True)
    visit(app, PAGE)
    check("and it stops blocking", fetch_ad(app) == "reached")

    open_extension_settings(app)
    settings_click(app, f"[aria-label={json.dumps('Remove ' + UBOL_NAME)}]")
    settings_wait(app, f"!({INSTALLED}).includes({json.dumps(UBOL_NAME)})", 30)
    check("Remove uninstalls it", True)

    app.close()


if __name__ == "__main__":
    os.makedirs(drive_first_app.OUT, exist_ok=True)
    if not os.path.exists(os.path.join(STORE_DIR, "catalogue.json")):
        print("no mock store in .tools/extensions: run tools/fetch-extensions.sh and tools/pack-test-extension.py", file=sys.stderr)
        sys.exit(2)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Pages)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    PAGE = f"http://127.0.0.1:{server.server_port}/"
    # Built first, so the process started is the store itself and stops when told to.
    store_bin = os.path.join(ROOT, "build/e2e/extstore")
    subprocess.check_call(["go", "build", "-o", store_bin, "./cmd/extstore"], cwd=os.path.join(ROOT, "core"))
    store = subprocess.Popen([store_bin, "-dir", STORE_DIR, "-addr", STORE.split("//")[1]], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        run_parts([extension_checks])
    finally:
        store.terminate()
