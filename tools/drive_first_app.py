#!/usr/bin/env python3
"""Runs the first app with real Chromium and tests it through real NSEvents.

The driver asks the window's own hitTest:, sets first responder and hands the event to
the view, because [window sendEvent:] delivers nothing without a GUI session. Content's
view checks the same hitTest: before accepting the event. AppKit's own dispatch isn't
exercised, and neither are full screen and the clipboard, which need a person's desktop
session (tools/check_full_screen.py, tools/check_clipboard.py). This needs no display. Run it from the repo root:

    python3 tools/drive_first_app.py
    python3 tools/drive_first_app.py settings   # only menus, keys and the settings page
    python3 tools/drive_first_app.py timeline   # only the timeline
    python3 tools/drive_first_app.py cookies    # only cookies kept across a restart
    python3 tools/drive_first_app.py visible    # only the tabs a window shows
    python3 tools/drive_first_app.py windows    # only several windows

Results go to build/real/. Each screenshot is what content rendered, drawn shell over
page by the driver. It is not read back from the window.
"""

import http.server
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import traceback

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import timeline  # noqa: E402

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
BIN = os.environ.get("PB_BIN") or os.path.join(ROOT, "chromium/src/out/pb/project_b")
OUT = os.path.join(ROOT, "build/real")
BAR_Y = 26  # The middle of the top row, where the window buttons centre.
BLOCKED = "blocked.test"
URL_A = "https://example.com/"
URL_B = "https://www.iana.org/help/example-domains"

failures = 0


# Pages served on this machine for the tests that need a server's answer.
class Pages(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/redirect":
            self.send_response(302)
            self.send_header("Location", "chrome://shell/")
            self.end_headers()
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
        }.get(self.path, b'<!doctype html><title>Links</title><a href="chrome://shell/">The shell</a>')
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


pages = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Pages)
threading.Thread(target=pages.serve_forever, daemon=True).start()
PAGES = f"http://127.0.0.1:{pages.server_port}/"


def check(name, ok, detail=""):
    global failures
    print(("ok   " if ok else "FAIL ") + name + ("" if ok or detail == "" else " -> " + str(detail)), flush=True)
    if not ok:
        failures += 1


class App:
    running = []  # Every app started and not yet closed.

    def __init__(self, profile, extra=()):
        self.fifo = os.path.join(tempfile.mkdtemp(prefix="pb-driver-"), "fifo")
        os.mkfifo(self.fifo)
        self.log = open(os.path.join(OUT, "app.log"), "a")
        self.replies = {}
        self.cond = threading.Condition()
        self.next = 0
        self.started = time.time()
        self.proc = subprocess.Popen(
            [BIN, f"--profile={profile}", f"--block={BLOCKED}", f"--pb-driver={self.fifo}", "--disable-backgrounding-occluded-windows", *extra],
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, errors="replace", bufsize=1,
        )
        App.running.append(self)
        threading.Thread(target=self._read, daemon=True).start()
        self.wait_reply("start", 60)
        self.writer = open(self.fifo, "w", buffering=1)

    def _read(self):
        for line in self.proc.stdout:
            self.log.write(line)
            self.log.flush()
            m = re.match(r"PBDRIVER (\S+) (.*)$", line.rstrip("\n"))
            if m:
                with self.cond:
                    self.replies[m.group(1)] = m.group(2)
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

    def cmd(self, *parts, timeout=20):
        self.next += 1
        key = str(self.next)
        self.writer.write(" ".join([key, *map(str, parts)]) + "\n")
        self.writer.flush()
        return self.wait_reply(key, timeout)

    def js(self, expr, page=False, timeout=20):
        r = self.cmd("evalpage" if page else "eval", expr.replace("\n", " "), timeout=timeout)
        return json.loads(r) if r != "" else None

    def wait(self, expr, timeout=15, page=False):
        """Polls until expr is truthy. A navigation that swaps the page's frame drops the
        answer to an eval sent just before, so a lost answer is tried again."""
        deadline = time.time() + timeout
        last = None
        while time.time() < deadline:
            try:
                last = self.js(expr, page, timeout=5)
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
    x, y = center(app, selector)
    return app.cmd("click", x, y)


def box(app):
    return app.js("(()=>{const r=document.querySelector('#suggestions').getBoundingClientRect();return [r.x,r.y,r.width,r.height]})()")


def focus_bar(app):
    """Clicks the bar and waits until it really has focus and everything is selected."""
    click(app, "input")
    app.wait(f"document.hasFocus()&&document.activeElement==={INPUT}&&{INPUT}.selectionStart===0&&{INPUT}.selectionEnd==={VALUE}.length")


def leave_bar(app):
    """Escape until the page has focus. focusPage is asynchronous, so wait for it."""
    deadline = time.time() + 5
    while not app.js("document.hasFocus()", page=True):
        if time.time() > deadline:
            raise TimeoutError("page never got focus")
        app.cmd("key", "escape")
        time.sleep(0.1)


def visit(app, url):
    focus_bar(app)
    app.cmd("type", url)
    app.wait(f"{VALUE}==={json.dumps(url)}")
    app.cmd("key", "enter")
    app.wait(f"{VALUE}.startsWith({json.dumps(url)})&&document.activeElement!=={INPUT}", 30)
    app.wait(TAB_READY, 30)


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
        try:
            part()
        except (RuntimeError, TimeoutError, OSError) as e:
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
    r = subprocess.run([BIN, "--version"], capture_output=True, text=True, timeout=10)
    want = bundle_version() or "development build"
    check("--version prints the version and exits", r.returncode == 0 and re.fullmatch(rf"Project B {re.escape(want)}, Chromium [0-9.]+\n", r.stdout) is not None, (r.returncode, r.stdout))


# The start-up milestones, in the order they must come. The shell's first paint and the
# first tab's commit race the rest, so they are checked apart.
STARTUP = ["process.start", "main.entered", "content.ready", "go.starting", "window.created", "shell.created",
           "shell.navigation_start", "window.shown", "go.started", "shell.committed",
           "shell.script", "shell.mounted", "shell.ready", "shell.snapshot"]


def timeline_checks():
    """The app's own timeline (docs/logging.md): the start-up milestones from C++, Go and
    the shell's JavaScript on one clock, each interaction as a span, and the same lines in
    timeline.log."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    log = os.path.join(OUT, "app.log")
    log_start = os.path.getsize(log) if os.path.exists(log) else 0
    app = App(profile)
    app.wait("!!document.querySelector('input')", 30)

    def read():
        return json.loads(app.cmd("timeline", 0))["result"]["entries"]

    def wait_for(test, timeout=20):
        deadline = time.time() + timeout
        while time.time() < deadline:
            if test(read()):
                return True
            time.sleep(0.1)
        return False

    wait_for(lambda es: {"shell.on_screen", "tab.committed"} <= set(timeline.marks(es)))
    m = timeline.marks(read())
    ms = lambda name: m[name] / 1e6
    missing = [n for n in STARTUP + ["shell.first_paint", "shell.on_screen", "tab.committed"] if n not in m]
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
    check("the first tab commits after Go starts it", ms("go.starting") <= ms("tab.committed") < ms("go.starting") + 15000, (ms("go.starting"), ms("tab.committed")))

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

    opened = span_after(lambda: click(app, "button.settings"), "settings.open")
    check("opening settings is a span from the click to its first frame on screen, through Go, the engine and the page",
          in_order(opened, ["settings.open.go", "settings.open.created", "settings.open.shown", "settings.navigation_start",
                            "settings.committed", "settings.script", "settings.mounted", "settings.snapshot",
                            "settings.first_paint", "settings.on_screen"]), opened)
    app.cmd("key", "cmd+w")
    time.sleep(0.5)
    new = span_after(lambda: click(app, ".new-tab"), "tab.new")
    check("a new tab is a span that ends when the shell has drawn it", in_order(new, ["tab.new.go", "tab.new.done"]), new)
    closed = span_after(lambda: app.cmd("key", "cmd+w"), "tab.close")
    check("closing a tab from the keyboard is a span too, begun in Go", in_order(closed, ["tab.close.go", "tab.close.done"]), closed)
    moved = span_after(lambda: click(app, "button[aria-label^='Tabs on']"), "tabs.layout")
    check("moving the tabs is a span that covers the page moving", in_order(moved, ["tabs.layout.go", "tabs.layout.done", "tabs.layout.page_moved"]), moved)

    # Each click and key press is a span of its own, from the OS's time for the event to
    # our code having it, with the window getting it between.
    inputs = [s for s in timeline.spans(read()) if s["name"] in ("input.click", "input.key")]
    check("a click and a key press are each a span from the OS's event, through the window, to the shell or Go",
          {"input.click", "input.key"} <= {s["name"] for s in inputs} and
          all(0 <= s["stages"].get(s["name"] + ".window", -1) <= s["dur"] < 5e9 for s in inputs), inputs[:4])

    # A LOG in the engine's C++ is an entry.
    app.cmd("strayshell")
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
    through Go, and the settings page, a page of its own over the whole window."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = App(profile)
    app.wait("!!document.querySelector('input')", 30)
    time.sleep(0.5)

    def find(items, title):
        return next((i for i in items if isinstance(i, dict) and i["title"] == title), None)

    menu = json.loads(app.cmd("menu"))
    view, edit, tab = find(menu, "View"), find(menu, "Edit"), find(menu, "Tab")
    full = find(view["items"], "Full Screen") if view else None
    check("the menu bar is Go's tree: Edit, View, Tab", bool(view and edit and tab), [m["title"] for m in menu if isinstance(m, dict)])
    check("View has Full Screen on Ctrl+Cmd+F", bool(full) and full["key"] == "^@f", full)
    check("Edit has Redo", bool(edit) and find(edit["items"], "Redo") is not None)
    check("the hidden Select Tab submenu isn't drawn", bool(tab) and find(tab["items"], "Select Tab") is None)
    check("AppKit added no Full Screen item of its own", sum(1 for m in menu for i in m.get("items", []) if isinstance(i, dict) and "Full Screen" in i["title"]) == 1)

    # Cmd+, opens the settings page over the whole window.
    settings = lambda expr: json.loads(app.cmd("evalsettings", expr))
    app.cmd("key", "cmd+,")
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
    check("it covers the window: the middle and the top row are the settings page's", app.cmd("hit", 600, 300) == "settings" and app.cmd("hit", 300, BAR_Y) == "settings", (app.cmd("hit", 600, 300), app.cmd("hit", 300, BAR_Y)))
    check("it shows the general settings", settings("!!document.querySelector('.version')"))
    print(app.cmd("capture", os.path.join(OUT, "settings-general.png")), flush=True)

    # A key recorded on the settings page reaches Go, and runs its action.
    settings("[...document.querySelectorAll('.nav-item')].find(b=>b.textContent==='Keyboard').click()")
    time.sleep(0.3)
    settings("[...document.querySelectorAll('button.action')].find(b=>b.querySelector('.name').textContent.trim()==='New Tab').click()")
    time.sleep(0.3)
    settings("[...document.querySelectorAll('button')].find(b=>b.textContent.trim()==='Add key').click()")
    time.sleep(0.3)
    app.cmd("key", "cmd+shift+k")
    keymap = os.path.join(profile, "keymap.json")
    deadline = time.time() + 5
    while time.time() < deadline and not (os.path.exists(keymap) and "Shift+Cmd+K" in open(keymap).read()):
        time.sleep(0.1)
    check("a key pressed while recording is added to keymap.json", os.path.exists(keymap) and "Shift+Cmd+K" in open(keymap).read(),
          (os.listdir(profile), app.cmd("evalsettings", "[document.activeElement.className, document.querySelector('[aria-label=File]').textContent.slice(0,80)]")))
    print(app.cmd("capture", os.path.join(OUT, "settings-keyboard.png")), flush=True)

    # Cmd+W closes the settings page, not the tab under it.
    rows = app.js(ROWS)
    app.cmd("key", "cmd+w")
    time.sleep(0.5)
    check("Cmd+W closes the settings page, not the tab", app.cmd("evalsettings", "1").startswith("ERR") and app.js(ROWS) == rows)
    app.cmd("key", "cmd+shift+k")
    try:
        app.wait(f"{ROWS}==={rows + 1}", 5)
        check("the new key opens a tab", True)
    except TimeoutError as e:
        check("the new key opens a tab", False, e)

    # Ctrl+Tab goes back through recent tabs while Ctrl is held; letting go settles.
    while app.js(ROWS) < 3:
        app.cmd("key", "cmd+t")
        time.sleep(0.3)
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

    # After a restart the keymap is still the user's.
    app = App(profile)
    app.wait("!!document.querySelector('input')", 30)
    rows = app.js(ROWS)
    app.cmd("key", "cmd+shift+k")
    try:
        app.wait(f"{ROWS}==={rows + 1}", 5)
        check("the added key survives a restart", True)
    except TimeoutError as e:
        check("the added key survives a restart", False, e)
    app.close()


def browsing_checks():
    """The window, the address bar, pages, tabs and crashes in the real app."""
    if os.path.exists(OUT):
        shutil.rmtree(OUT)
    os.makedirs(OUT)
    profile = tempfile.mkdtemp(prefix="pb-profile-")

    t0 = time.time()
    app = App(profile)
    app.wait("!!document.querySelector('input')", 30)
    print(f"shell page answered {time.time() - t0:.2f}s after launch (includes driver polling)", flush=True)
    stats = json.loads(app.cmd("stats", timeout=60))
    print("stats", json.dumps(stats), flush=True)

    check("opens empty", app.js(VALUE) == "")
    check("snapshot says the real engine", app.js("document.querySelector('iframe')===null"))
    app.wait("document.querySelector('form').getBoundingClientRect().height>0")
    time.sleep(0.3)
    print("bounds", app.cmd("bounds"), flush=True)
    check("one tab to start with", app.js(ROWS) == 1 and app.js(ACTIVE_ROW_TITLE) == "New Tab")
    check("opens with the address bar focused, as a new tab does", app.js(f"document.hasFocus()&&document.activeElement==={INPUT}"))

    # Hit tests start from the window's frame view, as AppKit's do, so the title bar and
    # its buttons take part in the answer.
    bar_x, bar_y = center(app, "input")
    check("the address bar, at title bar height, belongs to the shell and not the title bar", app.cmd("hit", bar_x, bar_y) == "shell", app.cmd("hit", bar_x, bar_y))
    check("the page belongs to the page", app.cmd("hit", 600, 300) == "page")
    check("the sidebar belongs to the shell", app.cmd("hit", 100, 60) == "shell", app.cmd("hit", 100, 60))
    window = json.loads(app.cmd("bounds"))
    print("title bar", window["titlebar"], "buttons", window["buttonsStart"], window["buttonsEnd"], "middle", window["buttonsMiddleY"], "size", window["buttonSize"], "screen", app.js("[screen.width,screen.height]"), flush=True)
    toggle_left = app.js("document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x")
    check("the shell leaves exactly the window buttons' space, as the engine reported it", toggle_left == window["buttonsEnd"] + 8, (toggle_left, window))
    gap = app.cmd("hit", (window["buttonsEnd"] + toggle_left) / 2, BAR_Y)
    check("the gap after the window buttons falls through to the window frame, which drags", gap == "frame", gap)
    right_end = app.cmd("hit", app.js("window.innerWidth") - 6, BAR_Y)
    check("the right end of the top row falls through to the frame too", right_end == "frame", right_end)
    below_tabs = app.cmd("hit", 100, 600)
    check("the empty sidebar is shell, so it never drags the window", below_tabs == "shell", below_tabs)
    over_buttons = app.cmd("hit", window["buttonsStart"] + window["buttonSize"] / 2, BAR_Y)
    check("the window buttons still get their own clicks", over_buttons.startswith("native:"), over_buttons)
    print("hit over window buttons:", over_buttons, flush=True)
    # The window server moves the window by itself from whatever part of the title bar
    # AppKit gives it, without asking the app. Given the address bar, it moved the window
    # on a drag there instead of selecting text; given nothing, no drag moved it.
    region = app.cmd("dragregion")
    served = [tuple(map(float, r)) for r in re.findall(r"\{\{([\d.]+), ([\d.]+)\}, \{([\d.]+), ([\d.]+)\}\}", region)]
    def given(x, y):
        return any(r[0] <= x < r[0] + r[2] and r[1] <= window["content"][1] - y < r[1] + r[3] for r in served)
    check("the window server isn't given the address bar to drag by itself", region != "unknown" and not given(bar_x, bar_y), region)
    gap_x = window["buttonsEnd"] + 4
    check("the window server is given the gap right of the window buttons", given(gap_x, BAR_Y), (gap_x, region))
    # An empty tab holds the blank page, so its page area is a page in content's own
    # colours, never the frame colour showing through. The frame colour used to take
    # over a few seconds in, so this looks after ten.
    time.sleep(max(0, 10 - (time.time() - app.started)))
    color = app.cmd("pagecolor")
    check("ten seconds on, an empty tab's page area is the page's colour, not the frame colour", color not in ("none", "#4f9e9e"), color)
    print(app.cmd("capture", os.path.join(OUT, "1-empty.png")), flush=True)

    # Real pages through the address bar.
    visit(app, URL_A)
    check("loads a real page", app.js("location.href", page=True) == URL_A, app.js("location.href", page=True))
    title = app.wait("document.title", page=True)
    check("the page's own title reaches the page", "Example" in title, title)
    time.sleep(0.5)
    check("window title follows the tab", "Example" in app.cmd("title"), app.cmd("title"))
    visit(app, URL_B)
    app.wait(f"{VALUE}.includes('iana.org')", 30)
    # A hook added in Go only: content's load progress reaches the shell's bar.
    progress = json.loads(app.cmd("stats", timeout=60))["go"]["hooks"].get("WebContentsObserver.LoadProgressChanged", 0)
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
    check("the suggestion has the real page's title", "Example" in app.js("document.querySelector('[role=option]').innerText"))
    x, y, w, h = box(app)
    check("the box belongs to the shell", app.cmd("hit", x + 30, y + h - 6) == "shell")
    check("below the box belongs to the page", app.cmd("hit", x + 30, y + h + 40) == "page")

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
    x, y, w, h = box(app)
    app.cmd("click", x + 40, y + 20)
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
    check("a blocked URL stays out of history", app.js(OPTIONS) == 0, app.js(OPTIONS))
    # With no suggestions open, Escape leaves the bar and the shell asks for the page's
    # keyboard. A click on the bar before that request lands loses the keyboard to it, so
    # wait for the page to have it.
    leave_bar(app)
    focus_bar(app)
    app.cmd("type", "blank")
    app.wait(f"{VALUE}==='blank'")
    time.sleep(0.4)
    check("the blank page a new tab holds stays out of history", app.js(OPTIONS) == 0, app.js(OPTIONS))
    leave_bar(app)

    # A page restored from the back-forward cache never reports a start, so its commit
    # has to clear the Blocked chip.
    click(app, "[aria-label=Back]")
    try:
        app.wait("document.querySelector('.status')===null", 30)
        check("going back clears the blocked chip", True)
    except TimeoutError as e:
        check("going back clears the blocked chip", False, e)

    # The shell's own page never loads in a tab, however it's asked for: a copy there
    # would talk to Go as the shell. Content's other internal pages are let through.
    def typed(url):
        focus_bar(app)
        app.cmd("type", url)
        app.wait(f"{VALUE}==={json.dumps(url)}")
        app.cmd("key", "enter")

    status = "(document.querySelector('.status')||{}).innerText||''"
    visit(app, PAGES + "links")
    typed("chrome://shell/")
    try:
        app.wait(f"{status}==='Blocked'", 10)
        check("typing chrome://shell/ is refused by the gate", True)
    except TimeoutError as e:
        check("typing chrome://shell/ is refused by the gate", False, e)
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
    check("a page's link to chrome://shell/ is refused, and the tab has no bridge", landed[0] != "chrome://shell/" and landed[1] == "undefined", (landed, app.js(status)))
    typed(PAGES + "redirect")
    time.sleep(2)
    landed = app.js("[location.href, typeof (window.chrome && chrome.send)]", page=True)
    check("a redirect to chrome://shell/ is refused, and the tab has no bridge", landed[0] != "chrome://shell/" and landed[1] == "undefined", landed)
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
    stray = app.cmd("strayshell", timeout=30)
    time.sleep(1)
    check("a chrome://shell/ page outside the shell gets no bridge", stray == "chrome://shell/ undefined", stray)
    check("and Go receives no shell message from it", shell_messages() == before, (before, shell_messages()))
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
    time.sleep(0.3)
    settled = json.loads(app.cmd("bounds"))
    r = settled["reported"]
    check("a burst of resizes ends in the right place", settled["frame"] == [r[0], settled["content"][1] - r[1] - r[3], r[2], r[3]] and r[0] == 224 and r[2] == settled["content"][0] - 228, settled)
    print("stale bounds ignored", json.loads(app.cmd("stats", timeout=60))["ignoredStaleBounds"] - before, flush=True)

    # The window controls stay put through resizes, so the shell's space for them does too.
    check("the controls' space survives resizing", app.js("document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x") == window["buttonsEnd"] + 8)

    # The frame colour has one owner: Go gave it to the window and to the shell.
    frame = app.js("getComputedStyle(document.documentElement).getPropertyValue('--frame').trim()")
    check("the window and the shell paint the frame colour Go owns", frame == "#4f9e9e" and app.cmd("color") == frame, (frame, app.cmd("color")))
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
    check("the window draws the frame's curve with its own radius and Go's width and colour", windowed > 0 and native() == {"radius": windowed, "width": 4, "color": "#4f9e9e"}, (windowed, native()))

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
        check("the shell's zoom request zooms the window", False, f"no display? {e}")

    # Tabs. Tab 1 is on example.com here.
    app.cmd("key", "cmd+t")
    app.wait(f"{ROWS}===2", 10)
    check("Cmd+T opens a tab and makes it active", app.js(ACTIVE_ROW_TITLE) == "New Tab", app.js(ACTIVE_ROW_TITLE))
    app.wait(f"document.hasFocus()&&document.activeElement==={INPUT}", 5)
    check("the new tab focuses the address bar", True)
    check("back and reload follow the new, empty tab", disabled("Back") and disabled("Reload"))
    color = "none"
    for _ in range(50):
        color = app.cmd("pagecolor")
        if color != "none":
            break
        time.sleep(0.1)
    check("the new tab's page area is the page's colour too", color not in ("none", "#4f9e9e"), color)
    app.cmd("type", URL_B)
    app.wait(f"{VALUE}==={json.dumps(URL_B)}")
    app.cmd("key", "enter")
    app.wait(f"{VALUE}.includes('iana.org')&&document.activeElement!=={INPUT}", 30)
    app.wait(TAB_READY, 30)
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
    check("the strip leaves exactly the window buttons' space", first_tab_x == window["buttonsEnd"] + 8, (first_tab_x, window))
    check("a strip tab belongs to the shell", app.cmd("hit", first_tab_x + 20, BAR_Y) == "shell")
    strip_empty = app.cmd("hit", 1000, BAR_Y)
    check("the empty strip falls through to the frame, which drags", strip_empty == "frame", strip_empty)
    joint = app.js("(()=>{const a=document.querySelector('.strip .shape').getBoundingClientRect();const b=document.querySelector('.strip').getBoundingClientRect();return [a.bottom,b.bottom]})()")
    check("the active tab's shape runs under the bar, so no line shows between them", joint[0] > joint[1], joint)
    n = app.js(ROWS)
    app.cmd("key", "cmd+t")
    app.wait(f"{ROWS}==={n + 1}", 10)
    check("Cmd+T works with the tabs on top", app.js(ACTIVE_ROW_TITLE) == "New Tab")
    # The shell re-reports its drag rectangles a frame after the strip changed. A click
    # inside that frame would still hit the old empty space.
    time.sleep(0.3)
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
    check("every child process is the helper", children and all(r["exe"] == "project_b_helper" for r in children), children)

    # Binaries: Go only in the main one, content only in the library.
    macos = os.path.dirname(BIN)
    helper = os.path.join(macos, "project_b_helper")
    library = os.path.join(macos, "../Frameworks/libproject_b_content.dylib")
    if not os.path.exists(library):
        library = os.path.join(macos, "libproject_b_content.dylib")
    nm = lambda *args: subprocess.run(["nm", *args], capture_output=True, text=True).stdout
    check("the helper has no Go", "_cgo" not in nm(helper) and "runtime." not in nm(helper))
    check("the library has no Go", "_cgo" not in nm(library) and "runtime." not in nm(library))
    check("the main binary has no content", "__ZN7content" not in nm(BIN) and "__ZN5blink" not in nm(BIN))
    exports = nm("-gU", library).split()
    check("the library exports only ProjectBMain", [e for e in exports if e.startswith("_")] == ["_ProjectBMain"], exports[:12])
    final = json.loads(app.cmd("stats", timeout=60))
    print("stats", json.dumps(final), flush=True)

    # Leave the tabs on top, so the restart below shows the choice was kept.
    click(app, "[aria-label='Tabs on top']")
    app.wait("!!document.querySelector('.strip')", 5)

    # Closing the last tab closes the window, which ends the app. After each close the
    # shell puts focus back in the next tab, so the next key waits for that.
    while app.js(ROWS) > 1:
        n = app.js(ROWS)
        app.cmd("key", "cmd+w")
        app.wait(f"{ROWS}==={n - 1}", 10)
        deadline = time.time() + 10
        while not (app.js("document.hasFocus()") or app.js("document.hasFocus()", page=True)):
            if time.time() > deadline:
                raise TimeoutError("nothing has focus after closing a tab")
            time.sleep(0.1)
    app.cmd("key", "cmd+w")
    try:
        app.proc.wait(timeout=30)
        check("closing the last tab closes the window and ends the app", True)
    except subprocess.TimeoutExpired:
        check("closing the last tab closes the window and ends the app", False)
        app.close()

    # History survives a restart.
    app = App(profile)
    app.wait("!!document.querySelector('input')", 30)
    time.sleep(1)
    focus_bar(app)
    app.cmd("type", "exam")
    try:
        app.wait(OPTIONS, 10)
        check("history survives a restart", True)
        check("titles survive a restart", "Example" in app.js("document.querySelector('[role=option]').innerText"))
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
    app.wait("!!document.querySelector('input')", 30)
    app.cmd("key", "cmd+t")
    app.wait(f"{ROWS}===2")
    deadline = time.time() + 10
    while json.loads(app.cmd("visible"))[0]["tab"] != 2 and time.time() < deadline:
        time.sleep(0.05)
    check("the shell's layout lists the new tab, the one it shows", [v["tab"] for v in json.loads(app.cmd("visible"))] == [2])
    app.cmd("showtabs", 1, 224, 48, 400, 600, 2, 624, 48, 400, 600)
    visible = json.loads(app.cmd("visible"))
    check("a layout of two tabs puts both views in the window, each at its rectangle",
          [(v["tab"], v["frame"], v["hidden"]) for v in visible] == [(1, [224, 48, 400, 600], False), (2, [624, 48, 400, 600], False)], visible)
    check("and tells both they are shown", all(v["shown"] for v in visible), visible)
    check("both pages are visible to themselves",
          [json.loads(app.cmd("evaltab", t, "document.visibilityState")) for t in (1, 2)] == ["visible", "visible"])
    check("a press goes to the page under it", app.cmd("hit", 400, 300) == "page" and app.cmd("hit", 800, 300) == "page")
    app.cmd("showtabs", 2, 224, 48, 800, 600)
    deadline = time.time() + 10
    while json.loads(app.cmd("evaltab", 1, "document.visibilityState")) != "hidden" and time.time() < deadline:
        time.sleep(0.05)
    check("a tab that leaves the list is told it is hidden", json.loads(app.cmd("evaltab", 1, "document.visibilityState")) == "hidden")
    app.close()


def window_checks():
    """Several windows: Cmd+N makes a second window with one new tab and its own shell, a
    little down and right of the first; a command acts on the window in front; a second
    start's URL opens in the window used last; closing a window's last tab closes it, and
    closing the last window quits. The driver makes a window the key window, as a click
    on it would on a real desktop."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = App(profile)
    app.wait("!!document.querySelector('input')", 30)
    windows = lambda: json.loads(app.cmd("windows"))
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
    check("the File menu has New Window on Cmd+N", {"title": "New Window", "key": "@n"} in
          [{"title": i["title"], "key": i["key"]} for i in file["items"] if i != "-"], file["items"])

    app.cmd("window", 2)
    app.wait("!!document.querySelector('input')", 30)
    app.wait(f"{ROWS}===1")
    check("the second window has a shell of its own, with one new tab", app.js(f"{ROWS}===1&&{VALUE}===''"))
    deadline = time.time() + 10
    while not windows()[1]["tabs"] and time.time() < deadline:
        time.sleep(0.05)
    check("each window shows its own tab", [w["tabs"] for w in windows()] == [[1], [2]], windows())

    # A command acts on the window in front.
    app.cmd("key", "cmd+t")
    app.wait(f"{ROWS}===2")
    app.cmd("window", 1)
    check("Cmd+T in the second window added a tab there, not in the first", app.js(f"{ROWS}") == 1)

    # The first window is the one used last now: a second start's URL opens there.
    started = subprocess.run([BIN, f"--profile={profile}", PAGES + "clicks"], timeout=60, capture_output=True)
    check("a second start on the profile hands over and exits", started.returncode == 0, started.returncode)
    app.wait(f"{ROWS}===2&&{VALUE}==={json.dumps(PAGES + 'clicks')}", 20)
    check("its URL opens in a new tab in the window used last", True)
    app.cmd("window", 2)
    check("and not in the other window", app.js(f"{ROWS}") == 2)

    # Closing the second window's last tab closes it; the app goes on.
    app.cmd("key", "cmd+w")
    app.wait(f"{ROWS}===1")
    app.cmd("key", "cmd+w")
    deadline = time.time() + 10
    while len(windows()) > 1 and time.time() < deadline:
        time.sleep(0.05)
    check("closing a window's last tab closes the window", [w["id"] for w in windows()] == [1], windows())
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


def cookie_checks():
    """A cookie a page sets is saved in the profile and sent back after a restart."""
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = App(profile)
    app.wait("!!document.querySelector('input')", 30)
    visit(app, PAGES + "cookie")
    check("the first visit sends no cookie", app.js("document.title", page=True) == "none")
    app.close()
    check("the cookie is saved in the profile", os.path.exists(os.path.join(profile, "chromium/Network/Cookies")))
    app = App(profile)
    app.wait("!!document.querySelector('input')", 30)
    visit(app, PAGES + "cookie")
    check("the cookie is sent back after a restart", app.js("document.title", page=True) == "a=1", app.js("document.title", page=True))
    app.close()


if __name__ == "__main__":
    os.makedirs(OUT, exist_ok=True)
    parts = {"settings": [settings_checks], "timeline": [timeline_checks], "cookies": [cookie_checks], "visible": [visible_tabs_checks], "windows": [window_checks]}
    run_parts(parts.get(" ".join(sys.argv[1:]), [version_checks, timeline_checks, browsing_checks, settings_checks, visible_tabs_checks, window_checks, cookie_checks]))
