#!/usr/bin/env python3
"""Runs the first app with real Chromium and tests it through real NSEvents.

The driver asks the window's own hitTest:, sets first responder and hands the event to
the view, because [window sendEvent:] delivers nothing without a GUI session. Content's
view checks the same hitTest: before accepting the event. AppKit's own dispatch isn't
exercised. Run it from the repo root:

    python3 tools/drive_first_app.py

Results go to build/real/. Each screenshot is what content rendered, drawn shell over
page by the driver. It is not read back from the window.
"""

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import threading
import time

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
BIN = os.environ.get("PB_BIN") or os.path.join(ROOT, "chromium/src/out/pb/project_b")
OUT = os.path.join(ROOT, "build/real")
BAR_Y = 20  # The middle of the top row: the title bar's height, shared with the window buttons.
BLOCKED = "blocked.test"
URL_A = "https://example.com/"
URL_B = "https://www.iana.org/help/example-domains"

failures = 0


def check(name, ok, detail=""):
    global failures
    print(("ok   " if ok else "FAIL ") + name + ("" if ok or detail == "" else " -> " + str(detail)), flush=True)
    if not ok:
        failures += 1


class App:
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
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, bufsize=1,
        )
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


def main():
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
    right_end = app.cmd("hit", 1096, BAR_Y)
    check("the right end of the top row falls through to the frame too", right_end == "frame", right_end)
    below_tabs = app.cmd("hit", 100, 600)
    check("the empty sidebar falls through to the frame, so it drags the window", below_tabs == "frame", below_tabs)
    over_buttons = app.cmd("hit", 40, BAR_Y)
    check("the window buttons still get their own clicks", over_buttons.startswith("native:"), over_buttons)
    print("hit over window buttons:", over_buttons, flush=True)
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
    leave_bar(app)

    # A page restored from the back-forward cache never reports a start, so its commit
    # has to clear the Blocked chip.
    click(app, "[aria-label=Back]")
    try:
        app.wait("document.querySelector('.status')===null", 30)
        check("going back clears the blocked chip", True)
    except TimeoutError as e:
        check("going back clears the blocked chip", False, e)

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
    check("a burst of resizes ends in the right place", settled["frame"] == [r[0], settled["content"][1] - r[1] - r[3], r[2], r[3]] and r[0] == 200 and r[2] == settled["content"][0] - 204, settled)
    print("stale bounds ignored", json.loads(app.cmd("stats", timeout=60))["ignoredStaleBounds"] - before, flush=True)

    # The window controls stay put through resizes, so the shell's space for them does too.
    check("the controls' space survives resizing", app.js("document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x") == window["buttonsEnd"] + 8)

    # The engine reads the window's corner radius from macOS and the shell's frame
    # follows it, one frame width inside. Real full screen, which needs a display, has
    # square corners and no controls; the radius comes back with the window.
    radius = lambda: float(app.js("getComputedStyle(document.documentElement).getPropertyValue('--window-radius')").replace("px", ""))
    inner = lambda: float(app.js("getComputedStyle(document.querySelector('header')).borderTopLeftRadius").replace("px", ""))
    windowed = radius()
    print("window corner radius", windowed, "inner", inner(), flush=True)
    check("the frame takes the window's corner radius from the engine", windowed > 0 and inner() == windowed - 4, (windowed, inner()))
    app.cmd("fullscreen")
    try:
        app.wait("getComputedStyle(document.documentElement).getPropertyValue('--window-radius').trim()==='0px'", 10)
        check("real full screen reports square corners and no controls", inner() == 0 and app.js("document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x") == 12, (inner(), radius()))
        app.cmd("fullscreen")
        app.wait(f"getComputedStyle(document.documentElement).getPropertyValue('--window-radius').trim()==='{windowed:g}px'", 10)
        app.wait(f"document.querySelector('[aria-label$=sidebar]').getBoundingClientRect().x==={window['buttonsEnd'] + 8}", 10)
        check("leaving full screen brings the curve and the controls back", True)
    except TimeoutError as e:
        check("real full screen reports square corners and no controls", False, f"no display? {e}")

    # Tabs. Tab 1 is on example.com here.
    app.cmd("key", "cmd+t")
    app.wait(f"{ROWS}===2", 10)
    check("Cmd+T opens a tab and makes it active", app.js(ACTIVE_ROW_TITLE) == "New Tab", app.js(ACTIVE_ROW_TITLE))
    app.wait(f"document.hasFocus()&&document.activeElement==={INPUT}", 5)
    check("the new tab focuses the address bar", True)
    check("back and reload follow the new, empty tab", disabled("Back") and disabled("Reload"))
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
    app.wait(f"{VALUE}.includes('iana.org')", 10)
    check("Ctrl+Tab picks the next tab", True)
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
    app.wait("document.querySelector('nav').getBoundingClientRect().width===196", 5)
    time.sleep(0.3)
    check("expanding it takes the width back", json.loads(app.cmd("bounds"))["frame"][0] == 200, app.cmd("bounds"))

    # The same tabs on top. The strip is the title bar then; the toolbar row is below it.
    click(app, "[aria-label='Tabs on top']")
    app.wait("!!document.querySelector('.strip')", 5)
    time.sleep(0.4)
    horizontal = json.loads(app.cmd("bounds"))
    check("with the tabs on top the page spans the width below two rows", horizontal["reported"][0] == 4 and horizontal["reported"][1] == 68 and horizontal["frame"][0] == 4, horizontal)
    first_tab_x = app.js("document.querySelector('[role=tab]').getBoundingClientRect().x")
    check("the strip leaves exactly the window buttons' space", first_tab_x == window["buttonsEnd"] + 8, (first_tab_x, window))
    check("a strip tab belongs to the shell", app.cmd("hit", first_tab_x + 20, BAR_Y) == "shell")
    strip_empty = app.cmd("hit", 1000, BAR_Y)
    check("the empty strip falls through to the frame, which drags", strip_empty == "frame", strip_empty)
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
    check("switching back brings the sidebar and the page follows", json.loads(app.cmd("bounds"))["frame"][0] == 200, app.cmd("bounds"))
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
    visit(app, URL_A)
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

    print("\nall passed" if failures == 0 else f"\n{failures} FAILED", flush=True)
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
