#!/usr/bin/env python3
"""Checks the installed app in the VM with real clicks and keys.

Start the VM and install a DMG first (docs/vm-testing.md); it says which of those is missing.
Run it with the VM's own Python, as tools/test.sh vm does. Prints pass or FAIL for each check
with what it measured, and saves a shot per step. Everything the app receives is real input
sent inside the VM by deskrig (screen.py); window positions are read from macOS's window list inside the VM. Where the shell draws
its controls is asked from the installed app itself, through its driver, in a launch of its
own before the checks (layout()); positions are in points.
"""

import atexit
import html
import json
import os
import subprocess
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import screen  # Stops here, saying so, under a Python without numpy.

import numpy as np

VM_HOME = screen.VM_HOME
TART = VM_HOME + "/bin/tart.app/Contents/MacOS/tart"
VM = screen.VM
SHOTS = os.environ.get("PB_VM_SHOTS") or time.strftime(VM_HOME + "/shots/%Y%m%d-%H%M%S")
BUNDLE_ID = "dev.projectb.browser"
APP = "/Applications/Project B.app"
SCREEN = (1440, 900)
# The default frame colour, #4f9e9e (core/prefs/prefs.go), and the light tones the shell
# makes from it (shell/src/tones.js). A fresh profile has these.
FRAME = (79, 158, 158)  # --frame and --accent
BAR = (232, 242, 242)  # --bar, the frame mixed 0.87 toward white
WHITE = (255, 255, 255)  # --surface
ICON = (66, 133, 133)  # --icon, the toolbar's buttons: the frame mixed 0.16 toward black
PAGE_LOAD = 3.0
ANIMATION = 2.0
MOVE = (60, 40)

os.environ["TART_HOME"] = VM_HOME + "/tart-home"

# Checks that fail because of a bug we know about, by the check's name, with the bug. Such a
# failure is reported as "known" and doesn't fail the run; a known check that passes says so,
# so the entry can go once the bug is fixed. A new failure anywhere else fails the run.
KNOWN = {
    "a live resize keeps the page against the frame in every frame":
        "during a live resize the page runs past the frame in some frames (docs/vm-testing.md, bug 7)",
}

# The test pages, served inside the VM so no check waits on the internet. Each is one colour,
# so which page shows is plain from any part of it. Plain http, because history keeps only
# http and https pages, and the suggestions come from history.
PAGES = "/Users/admin/pb-pages"
PORT = 8765
ALPHA = f"http://127.0.0.1:{PORT}/alpha.html"
BRAVO = f"http://127.0.0.1:{PORT}/bravo.html"
CHARLIE = f"http://127.0.0.1:{PORT}/charlie.html"
PAGE_HTML = {
    "alpha.html": "<title>Alpha</title><body style='margin:0;background:#2b6cb0;color:#fff;font:64px sans-serif'><p>Alpha</p>",
    "bravo.html": "<title>Bravo</title><body style='margin:0;background:#dd6b20;color:#fff;font:64px sans-serif'><p>Bravo</p>",
    "charlie.html": "<title>Charlie</title><body style='margin:0;background:#2f855a;color:#fff;font:64px sans-serif'><p>Charlie</p>",
}
PROFILE = "/Users/admin/.project-b/default"
# Every launch starts from a fresh profile with automatic update checks off, so no update
# notice comes up over the page in the middle of a check.
PREFS = '{"autoUpdate": false}'

results = []
finished = False
about = ""


def check(name, ok, detail=""):
    known = KNOWN.get(name)
    status = "pass" if ok else "known" if known else "FAIL"
    note = (f"  [known: {known}]" if not ok else f"  [a known bug didn't show: {known}]") if known else ""
    results.append({"name": name, "status": status, "detail": detail, "known": known})
    print(f"{status:5}  {name}: {detail}{note}", flush=True)


def counts():
    return {k: sum(r["status"] == k for r in results) for k in ("pass", "known", "FAIL")}


def report():
    """Writes summary.txt, results.json and index.html next to the shots, however the run ended."""
    if not os.path.isdir(SHOTS):
        return
    c = counts()
    verdict = ("PASSED" if finished and not c["FAIL"] else "FAILED") + \
        f": {c['pass']} passed, {c['known']} failed as known bugs, {c['FAIL']} failed" + \
        ("" if finished else ", and the checks stopped early")
    with open(SHOTS + "/summary.txt", "w") as f:
        f.write(about)
        for r in results:
            f.write(f"{r['status']:5}  {r['name']}: {r['detail']}" + (f"  [{r['known']}]" if r["known"] else "") + "\n")
        f.write(verdict + "\n")
    with open(SHOTS + "/results.json", "w") as f:
        json.dump({"verdict": verdict, "finished": finished, "counts": c, "checks": results}, f, indent=1)
    rows = "".join(f"<tr class={r['status']}><td>{r['status']}</td><td>{html.escape(r['name'])}"
                   f"<div>{html.escape(str(r['detail']))}</div>"
                   + (f"<div>known: {html.escape(r['known'])}</div>" if r["known"] else "") + "</td></tr>"
                   for r in results)
    shots = "".join(f"<figure><img src='{html.escape(n)}' loading=lazy><figcaption>{html.escape(n[:-4])}</figcaption></figure>"
                    for n in sorted(os.listdir(SHOTS)) if n.endswith(".png"))
    with open(SHOTS + "/index.html", "w") as f:
        f.write("<!doctype html><meta charset=utf-8><meta name=viewport content='width=device-width'>"
                "<title>VM checks</title><style>body{font:14px system-ui;margin:16px}td{padding:4px 8px;vertical-align:top}"
                "td div{color:#666;font-size:12px}.FAIL td:first-child{color:#c00;font-weight:bold}"
                ".known td:first-child{color:#b70}.pass td:first-child{color:#080}img{max-width:100%}"
                "figure{margin:16px 0}</style>"
                f"<h1>{html.escape(verdict)}</h1><p>{html.escape(about)}</p><table>{rows}</table>{shots}")
    print(verdict + f", shots in {SHOTS}", flush=True)


def guest(*cmd):
    return subprocess.run([TART, "exec", VM, *cmd], capture_output=True, text=True, timeout=60).stdout.strip()


def fresh_profile(path=PROFILE, wipe="/Users/admin/.project-b"):
    guest("/bin/sh", "-c", f"rm -rf '{wipe}' && mkdir -p '{path}' && echo '{PREFS}' > '{path}/prefs.json'")


def serve_pages():
    """Writes the test pages into the VM and serves them there, on its own loopback."""
    files = " ".join(f"&& printf '%s' \"{page}\" > {PAGES}/{name}" for name, page in PAGE_HTML.items())
    guest("/bin/sh", "-c", f"mkdir -p {PAGES} {files}")
    guest("/usr/bin/pkill", "-f", f"http.server {PORT}")
    # tart exec waits for everything it started, so the server is a child of this run instead,
    # for as long as it lasts.
    server = subprocess.Popen([TART, "exec", VM, "/usr/bin/python3", "-m", "http.server", str(PORT),
                               "--bind", "127.0.0.1", "--directory", PAGES],
                              stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    atexit.register(server.terminate)
    for _ in range(50):
        if guest("/usr/bin/curl", "-fs", ALPHA):
            return
        time.sleep(0.2)
    sys.exit(f"the test pages aren't served at {ALPHA} in the VM")


WINDOWS = """
ObjC.import("CoreGraphics");
const all = ObjC.deepUnwrap(ObjC.castRefToObject($.CGWindowListCopyWindowInfo(1, 0)));
JSON.stringify(all.filter(w => w.kCGWindowLayer == 0 && ["Project B", "project_b"].includes(w.kCGWindowOwnerName))
    .map(w => w.kCGWindowBounds));
"""


# A left-button drag from (x1, y1) by (dx, dy) in screen points, in 20 steps, posted inside
# the VM with each event's movement set. The window server moves a window by those
# movements; Screen Sharing's pointer events carry movements that lose part of a drag, so
# over it even Terminal's title bar moved only some of the way.
DRAG = """
ObjC.import("CoreGraphics");
ObjC.import("Foundation");
function run(argv) {
  const [x1, y1, dx, dy] = argv.map(Number);
  const post = (type, x, y, mx, my) => {
    const e = $.CGEventCreateMouseEvent($(), type, $.CGPointMake(x, y), $.kCGMouseButtonLeft);
    $.CGEventSetIntegerValueField(e, $.kCGMouseEventDeltaX, mx);
    $.CGEventSetIntegerValueField(e, $.kCGMouseEventDeltaY, my);
    $.CGEventPost($.kCGHIDEventTap, e);
  };
  post($.kCGEventMouseMoved, x1, y1, 0, 0);
  $.NSThread.sleepForTimeInterval(0.3);
  post($.kCGEventLeftMouseDown, x1, y1, 0, 0);
  $.NSThread.sleepForTimeInterval(0.2);
  let x = x1, y = y1;
  for (let i = 1; i <= 20; i++) {
    const nx = Math.round(x1 + dx * i / 20), ny = Math.round(y1 + dy * i / 20);
    post($.kCGEventLeftMouseDragged, nx, ny, nx - x, ny - y);
    x = nx;
    y = ny;
    $.NSThread.sleepForTimeInterval(0.02);
  }
  $.NSThread.sleepForTimeInterval(0.2);
  post($.kCGEventLeftMouseUp, x, y, 0, 0);
}
"""


def drag(x1, y1, x2, y2):
    guest("/usr/bin/osascript", "-l", "JavaScript", "-e", DRAG, *map(str, (x1, y1, x2 - x1, y2 - y1)))


def window():
    """The app's largest window as (x, y, w, h), or None."""
    found = json.loads(guest("/usr/bin/osascript", "-l", "JavaScript", "-e", WINDOWS) or "[]")
    if not found:
        return None
    w = max(found, key=lambda b: b["Width"] * b["Height"])
    return (w["X"], w["Y"], w["Width"], w["Height"])


def window_or_stop(name):
    """The app's window, for a check that needs one. With none, the app has quit or crashed
    and nothing after this can run: the check fails and the run ends with its count."""
    win = window()
    if win is None:
        check(name, False, "no window: the app is gone, so the checks after this can't run")
        sys.exit(1)
    return win


def set_window(box):
    """Puts the window back between checks. Done inside the VM: it isn't what's being tested."""
    x, y, w, h = box
    guest("/usr/bin/osascript", "-e",
          f'tell application "System Events" to tell (first process whose bundle identifier is "{BUNDLE_ID}") to tell window 1\n'
          f"set position to {{{x}, {y}}}\n"
          f"set size to {{{w}, {h}}}\n"
          "end tell")
    time.sleep(0.4)
    if window() != box:
        sys.exit(f"couldn't put the window back at {box}: it's at {window()}, so nothing after this would mean anything")


def crop(img, x, y, w, h):
    s = desk.scale
    return img[int(y * s):int((y + h) * s), int(x * s):int((x + w) * s)]


def near(img, colour, tolerance=16):
    return np.all(np.abs(img.astype(np.int16) - np.array(colour, dtype=np.int16)) <= tolerance, axis=2)


def changed(a, b):
    """The share of pixels that differ between two crops of the same size."""
    if a.shape != b.shape or a.size == 0:
        return 1.0
    return float(np.mean(np.any(np.abs(a.astype(np.int16) - b.astype(np.int16)) > 16, axis=2)))


def shot(name):
    return desk.shot(f"{SHOTS}/{name}.png")


def lights(img, win):
    """The window buttons' (middle x, middle y, width) in window points, from their colours.

    Dimmed buttons don't match, so this also says the window is the active one.
    """
    box = crop(img, win[0], win[1], 140, 48).astype(np.int16)
    r, g, b = box[..., 0], box[..., 1], box[..., 2]
    found = {}
    for name, mask in (("close", (r > 200) & (g < 130) & (b < 130)),
                       ("min", (r > 200) & (g > 150) & (b < 110)),
                       ("zoom", (g > 150) & (r < 120) & (b < 120))):
        ys, xs = np.nonzero(mask)
        if len(xs) < 20:
            return None
        found[name] = ((xs.min() + xs.max() + 1) / 2 / desk.scale, (ys.min() + ys.max() + 1) / 2 / desk.scale,
                       (xs.max() - xs.min() + 1) / desk.scale)
    return found


def field_middle(img, win, x):
    """The address field's vertical middle in window points: the first run of white at column x.

    Only the first run, because a white page can start right below the field.
    """
    ys = np.nonzero(near(crop(img, win[0] + x, win[1], 1, 90), WHITE, 6)[:, 0])[0]
    if len(ys) == 0:
        return None
    last = ys[np.argmax(np.diff(ys, append=ys[-1] + 2) > 1)]
    return (ys[0] + last + 1) / 2 / desk.scale


CAL = "/tmp/pb-layout"  # In the guest: the driver's pipe, its output and a throwaway profile.


def driven(where, profile):
    """Opens the installed app with its driver (--pb-driver, as tools/drive_first_app.py uses
    it), on a profile of its own, keeping the driver's pipe and the app's output in where in
    the guest. Returns ask(*parts), which sends one driver command and returns its answer,
    once the driver has started."""
    guest("/bin/sh", "-c", f"rm -rf {where}; mkdir -p {where} && mkfifo {where}/fifo")
    fresh_profile(profile, profile)
    guest("/usr/bin/open", "-n", "--stdout", f"{where}/out", "--stderr", f"{where}/err", APP, "--args",
          f"--pb-driver={where}/fifo", f"--user-data-dir={profile}")
    asked = [0]

    def reply(key, timeout=30):
        end = time.time() + timeout
        while time.time() < end:
            for line in guest("/bin/cat", f"{where}/out").splitlines():
                if line.startswith(f"PBDRIVER {key} "):
                    return line[len(f"PBDRIVER {key} "):]
            time.sleep(0.3)
        sys.exit(f"the app's driver didn't answer {key}; its output: {guest('/bin/cat', f'{where}/out', f'{where}/err')[-2000:]}")

    def ask(*parts):
        asked[0] += 1
        guest("/bin/sh", "-c", 'printf "%s\\n" "$1" > "$2"', "sh", " ".join([str(asked[0]), *parts]), f"{where}/fifo")
        return reply(asked[0])

    reply("start")
    return ask


def layout():
    """Where the shell draws everything the checks click, in both tab layouts, asked from the
    installed app through its driver.

    A launch of its own, with its own profile, so the checks after it still start from a fresh
    app. Returns the shell's rectangles as (x, y, w, h) in window points from the top left for
    the window size it opened at, with two tabs open, and AppKit's own report of the window
    buttons and the page ("bounds").
    """
    ask = driven(CAL, f"{CAL}/profile")

    def js(expr):
        return json.loads(ask("eval", expr) or "null")

    def until(expr):
        end = time.time() + 15
        while not js(expr):
            if time.time() > end:
                sys.exit(f"the shell never showed {expr}")
            time.sleep(0.3)
        time.sleep(0.5)  # The shell's regions follow a frame later.

    def measure():
        found = js("(()=>{const r=e=>{if(!e)return null;const b=e.getBoundingClientRect();return [b.x,b.y,b.width,b.height]};"
                   "const q=s=>r(document.querySelector(s));"
                   "return {w:innerWidth,h:innerHeight,field:q('form.field'),back:q('[aria-label=Back]'),"
                   "forward:q('[aria-label=Forward]'),toggle:q('[aria-label$=sidebar]'),layout:q('button.layout'),"
                   "settings:q('button.settings'),sidebar:q('nav.sidebar'),rest:q('.strip .rest'),"
                   "tabs:[...document.querySelectorAll('[role=tab]')].map(r)}})()")
        found["bounds"] = json.loads(ask("bounds"))
        return found

    until("!!document.querySelector('input')")
    js("document.querySelector('.new-tab').click()")
    until("document.querySelectorAll('[role=tab]').length===2")
    side = measure()
    js("document.querySelector('button.layout').click()")
    until("!!document.querySelector('.strip')")
    top = measure()
    ask("close")
    for _ in range(50):
        if guest("/usr/bin/pgrep", "-x", "project_b") == "":
            break
        time.sleep(0.2)
    guest("/usr/bin/killall", "project_b")
    return side, top


def middle(r):
    return r[0] + r[2] / 2, r[1] + r[3] / 2


def click_on(r, win=None, right=False):
    """Clicks the middle of a shell rectangle in a window, home by default. right keeps its
    distance from the window's right edge instead of its left, for controls that sit there."""
    win = win or home
    x, y = middle(r)
    desk.click(win[0] + (win[2] - (V["w"] - x) if right else x), win[1] + y)


def launch():
    """Opens the app the way Finder does. Returns seconds to the shell's first paint, or None."""
    start = time.time()
    guest("/usr/bin/open", APP)
    need = 80000 * desk.scale * desk.scale  # About half the window's pale shell.
    while time.time() - start < 60:
        if near(desk.image(), BAR, 8).sum() > need:
            return time.time() - start
        time.sleep(0.01)
    return None


def quit_app():
    desk.key("cmd+q")
    for _ in range(50):
        if guest("/usr/bin/pgrep", "-x", "project_b") == "":
            return True
        time.sleep(0.2)
    guest("/usr/bin/killall", "project_b")
    return False


def drags(name, x, y, home, expect="moves the window"):
    """expect is "moves the window", "resizes the window" for the edges AppKit keeps for
    itself, or "leaves the window alone"."""
    win = window_or_stop(f"dragging {name} {expect}")
    drag(win[0] + x, win[1] + y, win[0] + x + MOVE[0], win[1] + y + MOVE[1])
    time.sleep(0.6)
    now = window()
    moved, resized = (now[0] - win[0], now[1] - win[1]), (now[2] - win[2], now[3] - win[3])
    if expect == "moves the window":
        ok = abs(moved[0] - MOVE[0]) <= 3 and abs(moved[1] - MOVE[1]) <= 3 and resized == (0, 0)
    elif expect == "leaves the window alone":
        ok = moved == (0, 0) and resized == (0, 0)
    else:
        ok = resized != (0, 0)
    check(f"dragging {name} {expect}", ok, f"moved {moved}, resized {resized}")
    set_window(home)


def zooms(name, x, y, home):
    """Zoom grows both ways. Double-clicking a window edge grows just one, so it doesn't count."""
    win = window_or_stop(f"double-clicking {name} zooms")
    desk.dbl(win[0] + x, win[1] + y)
    time.sleep(ANIMATION)
    now = window()
    check(f"double-clicking {name} zooms", now[2] > win[2] and now[3] > win[3], f"size {win[2:]} to {now[2:]}")
    set_window(home)


def selects_text(home):
    """Dragging across text in the address field selects it and leaves the window alone."""
    click_on(V["field"])
    time.sleep(0.4)
    desk.type("select these words")
    time.sleep(0.3)
    win = window_or_stop("dragging across the address field selects text, not the window")
    text = V["field"][0] + 12  # The input's padding.
    drag(win[0] + text + 10, win[1] + TOP, win[0] + text + 90, win[1] + TOP)
    time.sleep(0.6)
    now = window()
    shot("address-field-drag")
    check("dragging across the address field selects text, not the window", now == win, f"window moved from {win} to {now}")
    desk.key("esc")
    desk.key("esc")
    time.sleep(0.3)
    set_window(home)


def shown_green_button(name):
    """In full screen, shows the hidden title bar with the pointer, as a person would.

    Returns where its green button is, or None.
    """
    desk.move(SCREEN[0] / 2, 40, 0.2)
    desk.move(SCREEN[0] / 2, 0, 2.0)
    img = desk.image()
    shot("fullscreen-reveal-" + name)
    box = crop(img, 0, 0, 160, 120).astype(np.int16)
    ys, xs = np.nonzero((box[..., 1] > 150) & (box[..., 0] < 120) & (box[..., 2] < 120))
    if len(xs) < 20:
        return None
    return (xs.min() + xs.max() + 1) / 2 / desk.scale, (ys.min() + ys.max() + 1) / 2 / desk.scale


def leave_full_screen(name):
    """Clicks the green button in the shown title bar. Returns whether there was one."""
    green = shown_green_button(name)
    if green is None:
        return False
    desk.click(*green)
    time.sleep(ANIMATION)
    # Resting on the green button opens macOS's Move & Resize menu, which takes the next click.
    desk.move(SCREEN[0] / 2, SCREEN[1] / 2, 1.0)
    return True


def layout_click_works(win, on_top):
    """Clicks the layout button and, once the tabs move, clicks it again to put them back.

    Returns how many seconds the tabs took to move, or None if they didn't within 5 s.
    """
    def strip():
        return float(np.mean(near(crop(desk.image(), win[0] + win[2] - 200, win[1] + 6, 150, 20), FRAME)))

    first, then = (T, V) if on_top else (V, T)
    before = strip()
    click_on(first["layout"], win, right=True)
    start = time.time()
    while time.time() - start < 5:
        if abs(strip() - before) > 0.5:
            took = time.time() - start
            time.sleep(0.5)
            click_on(then["layout"], win, right=True)
            time.sleep(1)
            return took
        time.sleep(0.1)
    return None


def full_screen(name, enter, home, first_control, on_top=False):
    enter()
    time.sleep(ANIMATION)
    win = window()
    shot("fullscreen-" + name)
    img = desk.image()
    entered = win is not None and win[2:] == SCREEN
    check(f"{name} enters full screen", entered, f"window {win}")
    if not entered:
        if win != home:
            set_window(home)
        return
    check(f"{name}: window buttons hide", lights(img, win) is None)
    moved = near(crop(img, 8, 6, 24, 24), first_control).sum()
    check(f"{name}: controls move into the buttons' space", moved > 10, f"{moved} pixels of the first control at the far left")
    clicked = leave_full_screen(name)
    win = window()
    img = desk.image()
    shot("fullscreen-exit-" + name)
    # macOS puts the window back where it last placed it itself, which can be the zoomed
    # frame rather than where these checks moved it, so any window frame counts.
    check(f"{name}: the green button in the shown title bar brings the window back",
          clicked and win is not None and win[2:] != SCREEN and lights(img, win) is not None, f"window {win}")
    if win is not None and win[2:] != SCREEN:
        # Coming back to a frame other than the one it left from has left the shell deaf to
        # clicks, or slow to take them.
        took = layout_click_works(win, on_top)
        check(f"{name}: after leaving, the shell takes a click within 1 s", took is not None and took <= 1,
              f"the tabs moved {took:.1f} s after the click" if took is not None
              else f"the layout button did nothing in 5 s, window {win}")
        if took is None:
            # Start the app again from a fresh profile, so the checks after this still mean
            # something: a click that lands even later would move the tabs for the next launch.
            quit_app()
            fresh_profile()
            launch()
            time.sleep(1.5)
            if on_top:
                click_on(V["layout"], right=True)
                time.sleep(1)
    if window() is not None and window()[2:] == SCREEN:
        # Leave without the green button, so the checks after this still mean something.
        guest("/usr/bin/osascript", "-e",
              f'tell application "System Events" to tell (first process whose bundle identifier is "{BUNDLE_ID}") '
              'to set value of attribute "AXFullScreen" of window 1 to false')
        time.sleep(ANIMATION)
    if window() != home:
        set_window(home)


def missing():
    """What the checks still need before they can run, or None."""
    if not os.path.exists(TART):
        return "Tart and the VM aren't set up: run tools/vm/setup.sh once"
    if subprocess.run(["pgrep", "-f", f"tart run .* {VM}$"], capture_output=True).returncode != 0:
        return ("the VM isn't running: run tools/vm/start.sh. macOS only starts it after someone has logged in "
                "at this Mac's desktop since the last boot")
    if not guest("/bin/ls", "-d", APP):
        return "Project B isn't installed in the VM: run tools/vm/install-dmg.sh <path or URL of a DMG>"
    return None


def until(get, ok, limit=PAGE_LOAD + 5):
    """get() once ok() holds of it, or as it is after the limit. Checks of whether something
    works wait for their result this way, so a slow step fails only its own "within" check:
    only the checks named "within" time things.
    Waiting also keeps a late input from landing on the step after its own."""
    end = time.time() + limit
    now = get()
    while not ok(now) and time.time() < end:
        time.sleep(0.2)
        now = get()
    return now


def windows():
    """Every window of the app, as (x, y, w, h), in the window server's order, front first."""
    found = json.loads(guest("/usr/bin/osascript", "-l", "JavaScript", "-e", WINDOWS) or "[]")
    return [(w["X"], w["Y"], w["Width"], w["Height"]) for w in found]


def titles():
    """The windows' titles, front first, as System Events reads them: each is its shown tab's."""
    names = guest("/usr/bin/osascript", "-e",
                  f'tell application "System Events" to tell (first process whose bundle identifier is "{BUNDLE_ID}") '
                  "to get name of every window")
    return [n.strip() for n in names.split(",")] if names else []


def window_menu():
    """The items of the app's Window menu, by title."""
    names = guest("/usr/bin/osascript", "-e",
                  f'tell application "System Events" to tell (first process whose bundle identifier is "{BUNDLE_ID}") '
                  'to get name of every menu item of menu "Window" of menu bar 1')
    return [n.strip() for n in names.split(",")] if names else []


def go_to(url):
    """Cmd+L in the window in front, then the address."""
    desk.key("cmd+l")
    time.sleep(0.4)
    desk.type(url)
    desk.key("return")


def window_checks():
    """Several windows, with the real keyboard and mouse: Cmd+N, keys and commands going to the
    window clicked, a second start's URL in the window used last, the Window menu's list and
    Cmd+`, a window closing with its last tab, and the app quitting with its last window. Which window shows what is read from
    the windows' titles, each its shown tab's."""
    guest("/usr/bin/killall", "project_b")
    fresh_profile()
    time.sleep(1)
    launch()
    time.sleep(1.5)
    first = window_or_stop("the app starts with one window")
    desk.key("cmd+n")
    both = until(windows, lambda w: len(w) == 2)
    check("Cmd+N makes a second window", len(both) == 2, str(both))
    if len(both) != 2:
        return
    second = next(w for w in both if w != first)
    check("the second window opens a little down and right of the first, the same size",
          0 < second[0] - first[0] <= 60 and 0 < second[1] - first[1] <= 60 and second[2:] == first[2:],
          f"first {first}, second {second}")
    # Where each window shows whichever is in front: the left edge of the first, which the
    # second never covers, and the right edge of the second, which the first never does.
    step = second[0] - first[0]
    in_first = (first[0] + step / 2, first[1] + first[3] / 2)
    in_second = (second[0] + second[2] - step / 2, second[1] + second[3] / 2)

    desk.click(*in_first)
    time.sleep(0.4)
    go_to(ALPHA)
    until(titles, lambda t: "Alpha" in t)
    desk.click(*in_second)
    time.sleep(0.4)
    go_to(BRAVO)
    now = until(titles, lambda t: sorted(t) == ["Alpha", "Bravo"])
    check("Cmd+L and typing go to the window clicked", sorted(now) == ["Alpha", "Bravo"], str(now))
    shot("two-windows")

    desk.click(*in_first)
    until(titles, lambda t: t[:1] == ["Alpha"])
    guest("/usr/bin/open", "-n", APP, "--args", CHARLIE)
    now = until(titles, lambda t: sorted(t) == ["Bravo", "Charlie"], 15)
    check("a second start's URL opens in a new tab in the window used last, in front", now[:1] == ["Charlie"] and
          sorted(now) == ["Bravo", "Charlie"], str(now))
    shot("handed-over")

    # What macOS gives an app with several windows: the Window menu's list, and Cmd+`.
    menu = window_menu()
    check("the Window menu lists both windows", "Charlie" in menu and "Bravo" in menu, str(menu))
    desk.key("cmd+`")
    now = until(titles, lambda t: t[:1] == ["Bravo"])
    check("Cmd+` brings the other window to the front", now[:1] == ["Bravo"], str(now))

    desk.click(*in_second)
    until(titles, lambda t: t[:1] == ["Bravo"])
    desk.key("cmd+w")
    now = until(windows, lambda w: len(w) == 1)
    check("closing a window's last tab closes the window", len(now) == 1 and until(titles, lambda t: t == ["Charlie"]) == ["Charlie"],
          f"{now} {titles()}")
    desk.key("cmd+shift+w")
    gone = until(lambda: guest("/usr/bin/pgrep", "-x", "project_b"), lambda p: p == "", 10) == ""
    check("closing the last window quits", gone)
    if not gone:
        guest("/usr/bin/killall", "project_b")


MOVE = "/tmp/pb-move"  # In the guest: the driver's pipe and output, and a profile, for move_checks.
ALPHA_RGB = (43, 108, 176)  # alpha.html's colour.


def move_checks():
    """A tab moved to the other window arrives there still showing its page. The move is
    Go's, asked for through the driver's own DevTools command, ProjectB.moveTab."""
    guest("/usr/bin/killall", "project_b")
    ask = driven(MOVE, f"{MOVE}/profile")
    window_or_stop("the app starts with one window")
    go_to(ALPHA)
    until(titles, lambda t: t == ["Alpha"])
    desk.key("cmd+t")
    go_to(BRAVO)
    until(titles, lambda t: t == ["Bravo"])
    desk.key("cmd+n")
    until(windows, lambda w: len(w) == 2)
    shot("move-before")
    ask("projectb", "moveTab", json.dumps({"tab": 1, "window": 2, "index": 0}))
    now = until(titles, lambda t: sorted(t) == ["Alpha", "Bravo"])
    front = windows()[0]
    middle_of_page = crop(desk.image(), front[0] + front[2] / 2, front[1] + front[3] / 2, 40, 40)
    shot("move-after")
    check("a tab moved to the other window arrives there showing its page",
          now[:1] == ["Alpha"] and float(np.mean(near(middle_of_page, ALPHA_RGB))) > 0.9, str(now))
    desk.key("cmd+q")
    until(lambda: guest("/usr/bin/pgrep", "-x", "project_b"), lambda p: p == "", 10)


STATE = "/tmp/pb-state"  # In the guest: the driver's pipe and output, and a profile, for state_checks.


def state_checks():
    """A window's frame and state on a real screen, as Go asks for and reads them through
    the driver's own DevTools commands: bounds set and read back, and Cmd+M minimising it,
    which its tab is told as hidden."""
    guest("/usr/bin/killall", "project_b")
    ask = driven(STATE, f"{STATE}/profile")
    window_or_stop("the app starts with one window")
    windows_of_go = lambda: json.loads(ask("projectb", "getWindows", "{}"))["result"]["windows"]
    ask("projectb", "setWindowBounds", json.dumps({"window": 1, "bounds": {"x": 200, "y": 150, "w": 900, "h": 600}}))
    now = until(windows, lambda w: w == [(200, 150, 900, 600)])
    check("a window's frame goes on screen where Go asks", now == [(200, 150, 900, 600)], str(now))
    seen = until(windows_of_go, lambda w: w[0]["bounds"] == {"x": 200, "y": 150, "w": 900, "h": 600})
    check("and Go reads back where it is", seen[0]["bounds"] == {"x": 200, "y": 150, "w": 900, "h": 600}, str(seen))
    desk.key("cmd+m")
    seen = until(windows_of_go, lambda w: w[0]["state"] == "minimised")
    hidden = until(lambda: json.loads(ask("evaltab", "1", "document.visibilityState")), lambda v: v == "hidden")
    check("Cmd+M minimises the window, Go reads it so, and its tab is told it is hidden",
          seen[0]["state"] == "minimised" and hidden == "hidden", f"{seen} {hidden}")
    guest("/usr/bin/killall", "project_b")


if why := missing():
    screen.refuse(f"can't run the VM checks: {why}. See docs/vm-testing.md.")
os.makedirs(SHOTS, exist_ok=True)

# Start clean: nothing running, no profile, so the tabs start on the side.
guest("/usr/bin/killall", "project_b")
fresh_profile()
time.sleep(1)
desk = screen.Screen()
print(f"display {desk.width}x{desk.height}, shots in {SHOTS}", flush=True)
about = (f"Real pixels: the macOS {guest('/usr/bin/sw_vers', '-productVersion')} VM ({VM})'s screen, from its window\n"
         f"on this Mac by deskrig, {time.strftime('%Y-%m-%d %H:%M')}, not composites. Project B "
         f"{guest('/usr/bin/defaults', 'read', APP + '/Contents/Info', 'CFBundleVersion')} from /Applications.\n")
with open(SHOTS + "/ABOUT.txt", "w") as f:
    f.write(about)
atexit.register(report)
serve_pages()
shot("desktop")
# After a boot the image's login item leaves an "App Background Activity" notice at the top
# right, over the window's right end. deskrig closes such banners.
desk.dismiss_notifications()
time.sleep(1)

# "check.py windows" runs only the checks of several windows.
if sys.argv[1:] == ["windows"]:
    window_checks()
    move_checks()
    state_checks()
    finished = True
    sys.exit(1 if counts()["FAIL"] else 0)

# First paint: the first launch of an install, then three more.
paints = []
for i in range(4):
    paints.append(launch())
    time.sleep(1.5)
    shot(f"launch-{i}")
    if not quit_app():
        print(f"Cmd+Q left the app running on launch {i}", flush=True)
    time.sleep(1)
print("first paint, seconds: " + ", ".join("none" if p is None else f"{p:.2f}" for p in paints), flush=True)
check("first paint under 5 s", all(p is not None and p < 5 for p in paints), str(paints))

# Where everything is, from the app itself, then the app the checks drive.
V, T = layout()
print(f"from the driver, tabs on the side: {json.dumps(V)}", flush=True)
print(f"from the driver, tabs on top: {json.dumps(T)}", flush=True)
launch()
time.sleep(1.5)

# The window and its buttons.
home = window()
check("window is on screen", home is not None and home[2] > 400, f"window {home}")
if home is None:
    sys.exit("no window, nothing else can run")
W, H = home[2], home[3]
PX, PY = home[0], home[1]
if (W, H) != (V["w"], V["h"]):
    sys.exit(f"the window opened at {W}x{H}, but the driver measured {V['w']}x{V['h']}")
img = desk.image()
sampled = img[int((PY + H / 2) * desk.scale), int((PX + 100) * desk.scale)]
print(f"sidebar colour on screen {sampled.tolist()}, the shell asks for {list(BAR)}", flush=True)
L = lights(img, home)
check("window buttons are visible", L is not None, str(L))
if L is None:
    sys.exit("no window buttons found: the window isn't active, or the colours need calibrating")
buttons = (L["close"][0] - L["close"][2] / 2, L["zoom"][0] + L["zoom"][2] / 2)
B = V["bounds"]
check(f"window buttons are drawn where AppKit says, x {B['buttonsStart']} to {B['buttonsEnd']}, middle y {B['buttonsMiddleY']}",
      abs(buttons[0] - B["buttonsStart"]) <= 1 and abs(buttons[1] - B["buttonsEnd"]) <= 1
      and abs(L["close"][1] - B["buttonsMiddleY"]) <= 1, f"x {buttons[0]} to {buttons[1]}, y {L['close'][1]}")
TOP = middle(V["field"])[1]
check("window buttons' middle is the top row's", abs(B["buttonsMiddleY"] - TOP) <= 1,
      f"buttons y {B['buttonsMiddleY']}, the address field's middle {TOP}")
FIELD_Y = field_middle(img, home, middle(V["field"])[0])
check("window buttons centre on the address field", FIELD_Y is not None and abs(FIELD_Y - L["close"][1]) <= 1,
      f"buttons y {L['close'][1]}, field y {FIELD_Y}")
check("the shell's first control starts 8 points after the buttons", V["toggle"][0] == B["buttonsEnd"] + 8,
      f"the sidebar button at x {V['toggle'][0]}, buttons end at {B['buttonsEnd']}")
PX0, PY0, PW, PH = B["reported"]
PAGE_V = (PX0 + 36, PY0 + 12, PW - 56, PH - 32)  # An inside part of the page, tabs on the side.
NEUTRAL = (PX + W / 2, PY + H / 2)


def page(box=PAGE_V, limit=15):
    """The page once it has stopped changing for half a second, or as it is after the limit."""
    desk.move(*NEUTRAL, 0.3)
    last, since, end = None, time.time(), time.time() + limit
    while time.time() < end:
        now = crop(desk.image(), PX + box[0], PY + box[1], box[2], box[3])
        if last is None or changed(last, now) > 0.001:
            last, since = now, time.time()
        elif time.time() - since >= 0.5:
            break
        time.sleep(0.1)
    return last


def still():
    return window() == home


def address():
    """The address field's text, as pixels. Which page is showing, without depending on how far
    the page has drawn: a first visit can wait seconds on fonts for some of its languages."""
    desk.move(*NEUTRAL, 0.3)
    return crop(desk.image(), PX + V["field"][0] + 8, PY + TOP - 7, 400, 14)


def field_focused():
    """Whether the address field shows its focus ring, in the frame colour."""
    f = V["field"]
    return int(near(crop(desk.image(), PX + f[0], PY + f[1], f[2], f[3]), FRAME, 40).sum()) > 300


def type_address(text):
    """Clicks the address field, waits for it to take focus, and goes to text."""
    click_on(V["field"])
    until(field_focused, bool, 5)
    desk.type(text)
    desk.key("return")


# Real clicks at title-bar height, tabs on the side.
blank = page()
type_address(ALPHA)
example = until(page, lambda p: changed(blank, p) > 0.001)
example_at = address()
shot("vertical-example")
check("a click reaches the address bar", changed(blank, example) > 0.001 and still(), f"page changed {changed(blank, example):.3f}")

img = desk.image()
sidebar_bar = float(np.mean(near(crop(img, PX + 8, PY + 300, 180, 200), BAR)))
edge = float(np.mean(near(crop(img, PX + 1, PY + 100, 2, H - 200), FRAME)))
page_bar = float(np.mean(near(crop(img, PX + PAGE_V[0], PY + PAGE_V[1], PAGE_V[2], PAGE_V[3]), BAR, 4)))
check("tabs on the side: shell draws over the page", sidebar_bar > 0.9 and edge > 0.9 and page_bar < 0.2,
      f"sidebar {sidebar_bar:.2f}, frame {edge:.2f}, shell colour in page {page_bar:.2f}")

type_address(BRAVO)
iana = until(page, lambda p: changed(example, p) > 0.001)
iana_at = address()
check("a second page loads", changed(example, iana) > 0.001 and changed(example_at, iana_at) > 0.01,
      f"page changed {changed(example, iana):.3f}, address {changed(example_at, iana_at):.3f}")

click_on(V["back"])
back = until(address, lambda a: changed(a, example_at) < 0.01)
shot("after-back")
check("a click reaches the back button", changed(back, example_at) < 0.01 and still(), f"address differs from the first page's by {changed(back, example_at):.3f}")
click_on(V["forward"])
forward = until(address, lambda a: changed(a, iana_at) < 0.01)
check("a click reaches the forward button", changed(forward, iana_at) < 0.01 and still(), f"address differs from the second page's by {changed(forward, iana_at):.3f}")

# Suggestions: Cmd+L from the page, type, pick one with a real click over the page.
desk.click(*NEUTRAL)
time.sleep(0.3)
SUGGESTED = (V["field"][0] + 70, V["field"][1] + V["field"][3] - 8, 400, 60)  # Where the list opens.


def suggested():
    return crop(desk.image(), PX + SUGGESTED[0], PY + SUGGESTED[1], SUGGESTED[2], SUGGESTED[3])


before = suggested()
desk.key("cmd+l")
until(field_focused, bool, 5)
desk.type("alph")
listed = until(suggested, lambda l: changed(before, l) > 0.05, 5)
time.sleep(0.5)  # The list can still be growing.
shot("suggestions")
check("Cmd+L and typing shows suggestions over the page", changed(before, listed) > 0.05, f"changed {changed(before, listed):.3f}")
# The first item: 6 below the field, inside the list's 1 border and 4 padding, 32 high.
desk.click(PX + middle(V["field"])[0], PY + V["field"][1] + V["field"][3] + 6 + 1 + 4 + 16)
picked = until(address, lambda a: changed(a, example_at) < 0.01)
check("a click picks a suggestion", changed(picked, example_at) < 0.01, f"address differs from the first page's by {changed(picked, example_at):.3f}")

# Tabs by key, compared by what the sidebar shows.
SIDE = (V["sidebar"][0] + 4, V["sidebar"][1] + 4, 180, 300)


def side():
    desk.move(*NEUTRAL, 0.3)
    return crop(desk.image(), PX + SIDE[0], PY + SIDE[1], SIDE[2], SIDE[3])


one = side()
desk.key("cmd+t")
start = time.time()
two = one
while changed(one, two) <= 0.005 and time.time() - start < 5:
    time.sleep(0.05)
    two = crop(desk.image(), PX + SIDE[0], PY + SIDE[1], SIDE[2], SIDE[3])
took = time.time() - start
check("Cmd+T adds a tab within 0.5 s", changed(one, two) > 0.005 and took <= 0.5,
      f"the tab showed after {took:.2f} s" if changed(one, two) > 0.005 else "no tab after 5 s")
if changed(one, two) <= 0.005:
    # So the checks below still have their two tabs.
    desk.key("cmd+t")
    until(side, lambda n: changed(one, n) > 0.005, 5)
time.sleep(0.5)
two = side()
shot("two-tabs")
click_on(V["tabs"][0])
shown = until(address, lambda a: changed(a, example_at) < 0.01)
check("a click reaches a tab in the sidebar", changed(shown, example_at) < 0.01, f"address differs from the first page's by {changed(shown, example_at):.3f}")
click_on(V["tabs"][1])
until(address, lambda a: changed(a, example_at) >= 0.01)
desk.key("cmd+t")
until(side, lambda n: changed(two, n) > 0.005, 5)
desk.key("cmd+w")
check("Cmd+W closes a tab", changed(until(side, lambda n: changed(n, two) < 0.05, 5), two) < 0.05,
      f"differs from two tabs {changed(side(), two):.3f}")
desk.key("cmd+t")
until(side, lambda n: changed(two, n) > 0.005, 5)
desk.key("cmd+w")
desk.key("cmd+w")
# Which of the two the sidebar looks like: a fixed threshold read a sidebar with one tab, drawn
# a pixel differently, as not one tab.
now = until(side, lambda n: changed(n, one) < 0.05 and changed(n, one) < changed(n, two) / 4, 5)
shot("cmd-w-twice")
closed_two = changed(now, one) < 0.05 and changed(now, one) < changed(now, two) / 4
check("Cmd+W twice fast closes two tabs", closed_two,
      f"differs from one tab {changed(now, one):.3f}, from two tabs {changed(now, two):.3f}")
if not closed_two and changed(now, two) < changed(now, one):
    # Two tabs left: close one, so the checks below start from one. Never otherwise, because
    # Cmd+W on the last tab ends the app.
    desk.key("cmd+w")
    time.sleep(1)

# Dragging and double-clicking, tabs on the side. Only the empty spaces in the top row drag;
# the sidebar never does. The outer 3 points of the left and right edges are AppKit's resize
# zone.
gap = L["zoom"][0] + L["zoom"][2] / 2 + 4
TOP_EDGE = ("the top frame edge", W / 2, 2)
RESIZE_EDGES = (("the left edge", 1, H / 2), ("the right edge", W - 1, H / 2))
SIDE_PLACES = (("the gap right of the buttons", gap, 14), ("the end of the top row", W - 6, TOP))
for place in SIDE_PLACES + (TOP_EDGE,):
    drags(*place, home)
drags("the empty sidebar", 100, H - 40, home, "leaves the window alone")
for place in RESIZE_EDGES:
    drags(*place, home, "resizes the window")
selects_text(home)
for place in SIDE_PLACES:
    zooms(*place, home)

# Full screen, both ways in.
full_screen("green button", lambda: desk.click(PX + L["zoom"][0], PY + L["zoom"][1]), home, ICON)
full_screen("Ctrl+Cmd+F", lambda: desk.key("ctrl+cmd+f"), home, ICON)

# Resizing, on a page of one colour so its edges can be measured in every frame.
PINK = (255, 0, 255)
click_on(V["field"])
time.sleep(0.4)
desk.type("data:text/html,<body bgcolor=fuchsia>")
desk.key("return")
time.sleep(PAGE_LOAD)
desk.move(*NEUTRAL, 0.3)


def page_box():
    """The page's bounds on screen in points, from its colour, or None."""
    ys, xs = np.nonzero(near(desk.image(), PINK, 10))
    if len(xs) == 0:
        return None
    s = desk.scale
    return (xs.min() / s, ys.min() / s, (xs.max() + 1) / s, (ys.max() + 1) / s)




def slot(win):
    """Where the page belongs in a window."""
    return (win[0] + INSET[0], win[1] + INSET[1], win[0] + win[2] - INSET[2], win[1] + win[3] - INSET[3])


def page_timeline(action, seconds):
    """Runs action and records the page's bounds for that long, timed from when action
    returns. Returns them and the window after.

    A resize from outside goes through tart exec and System Events, which take 0.25 to 0.8 s
    to answer, and answer once the window has its new size, so the clock starts then."""
    action()
    start = time.time()
    boxes = []
    while time.time() - start < seconds:
        desk.wait_frame(0.2)
        boxes.append((time.time() - start, page_box()))
    return boxes, window()


def in_place_at(boxes, win):
    """When the page took its place in win for good, or None."""
    want, at = slot(win), None
    for t, box in boxes:
        if box is not None and all(abs(a - b) <= 1 for a, b in zip(box, want)):
            at = t if at is None else at
        else:
            at = None
    return at


def resize_from_outside(w, h):
    guest("/usr/bin/osascript", "-e",
          f'tell application "System Events" to tell (first process whose bundle identifier is "{BUNDLE_ID}") '
          f"to set size of window 1 to {{{w}, {h}}}")


# Typed keys can wait seconds behind the page (see the Cmd+T check), so give it time.
HOME_PAGE, end = None, time.time() + 10
while HOME_PAGE is None and time.time() < end:
    time.sleep(0.2)
    HOME_PAGE = page_box()
check("the one-colour test page loads", HOME_PAGE is not None)


def resizing():
    # A live resize by the corner: in every frame the page should end where the frame starts.
    x1, y1 = PX + W - 2, PY + H - 2
    desk.move(x1, y1, 0.4)
    desk.press()
    time.sleep(0.2)
    torn = []
    for i in range(1, 31):
        desk.move(x1 + 8 * i, y1 + 3 * i, 0.0)
        desk.wait_frame(1.0)
        box = page_box()
        # The frame's right end on one row, looking no further right than the pointer: the
        # desktop picture can have the frame's colour in it.
        row = np.nonzero(near(crop(desk.image(), PX, PY + 300, x1 + 8 * i + 10 - PX, 1), FRAME, 12)[0])[0]
        frame_right = PX + (row.max() + 1) / desk.scale if len(row) else None
        if i == 15:
            shot("live-resize")
        if box is None or frame_right is None or not 0 <= frame_right - box[2] <= INSET[2]:
            torn.append((i, box, frame_right))
        time.sleep(0.03)
    desk.release()
    time.sleep(0.6)
    check("a live resize keeps the page against the frame in every frame", not torn, f"{len(torn)} of 30 frames off: {torn[:3]}")
    set_window(home)

    # A resize from outside the app, as window managers and tiling do: there's no animation.
    set_window((PX, PY, W - 200, H - 150))
    time.sleep(1.5)
    boxes, win = page_timeline(lambda: resize_from_outside(W, H), 4)
    at = in_place_at(boxes, win)
    shot("resized-from-outside")
    check("after a resize from outside the app, the page fills its new place within 0.5 s",
          at is not None and at <= 0.5, f"in place after {at} s, window {win}, page at 0.2 s {next((b for t, b in boxes if t >= 0.2), None)}")
    set_window(home)

    # Zoom, and back.
    boxes, win = page_timeline(lambda: desk.dbl(PX + gap, PY + 14), 4)
    at = in_place_at(boxes, win)
    check("after zoom, the page fills its new place within 2 s", at is not None and at <= 2, f"in place after {at} s, window {win}")
    boxes, win = page_timeline(lambda: desk.dbl(win[0] + gap, win[1] + 14), 4)
    at = in_place_at(boxes, win)
    check("after zooming back, the page fills its place within 2 s", at is not None and at <= 2, f"in place after {at} s, window {win}")
    set_window(home)

    # Leaving full screen.
    desk.click(PX + L["zoom"][0], PY + L["zoom"][1])
    time.sleep(ANIMATION)
    target = shown_green_button("resize")
    if target is not None:
        boxes, win = page_timeline(lambda: desk.click(*target), 5)
        desk.move(SCREEN[0] / 2, SCREEN[1] / 2, 1.0)  # Off the green button, as in leave_full_screen.
        at = in_place_at(boxes, win)
        shot("after-full-screen")
        check("after leaving full screen, the page fills its place within 2 s", at is not None and at <= 2,
              f"in place after {at} s, window {win}, page at 1 s {next((b for t, b in boxes if t >= 1), None)}")
    else:
        check("after leaving full screen, the page fills its place within 2 s", False, "no green button to leave by")
    if window() != home:
        guest("/usr/bin/osascript", "-e",
              f'tell application "System Events" to tell (first process whose bundle identifier is "{BUNDLE_ID}") '
              'to set value of attribute "AXFullScreen" of window 1 to false')
        time.sleep(ANIMATION)
    set_window(home)


if HOME_PAGE is not None:
    INSET = (HOME_PAGE[0] - PX, HOME_PAGE[1] - PY, PX + W - HOME_PAGE[2], PY + H - HOME_PAGE[3])
    print(f"the page sits at {HOME_PAGE}, inset {INSET} from the window", flush=True)
    resizing()
set_window(home)


# Tabs on top, from a new launch: leaving full screen above has left the shell deaf to
# clicks (docs/vm-testing.md, bug 2), and nothing below should fail for that.
quit_app()
fresh_profile()
time.sleep(1)
launch()
time.sleep(1.5)
if window() != home:
    set_window(home)
type_address(ALPHA)
until(address, lambda a: changed(a, example_at) < 0.01)


def tabs_on_top(img):
    """Whether the tabs are on top: the strip along the window's top right is the frame colour."""
    return float(np.mean(near(crop(img, PX + W - 200, PY + 6, 150, 20), FRAME))) > 0.9


def switch_layout(rect, to_top, name):
    """Clicks the layout button and waits up to 5 s for the tabs to move, timing it, so a
    click the app takes late fails only this check and not the ones after it."""
    click_on(rect, right=True)
    start = time.time()
    while time.time() - start < 5 and tabs_on_top(desk.image()) != to_top:
        time.sleep(0.1)
    took = time.time() - start
    moved = tabs_on_top(desk.image()) == to_top
    check(f"the layout button {name} within 1 s", moved and took <= 1,
          f"the tabs moved {took:.1f} s after the click" if moved else "the tabs didn't move in 5 s")
    time.sleep(1)


switch_layout(V["layout"], True, "puts the tabs on top")
start = time.time()
while time.time() - start < 5 and float(np.mean(near(crop(desk.image(), PX + 8, PY + 300, 180, 200), BAR))) >= 0.2:
    time.sleep(0.1)
took = time.time() - start
check("after the tabs move to the top, the sidebar is gone within 1 s", took <= 1, f"gone after {took:.1f} s")
shot("horizontal")
img = desk.image()
strip = float(np.mean(near(crop(img, PX + W - 200, PY + 6, 150, 20), FRAME)))
gone = float(np.mean(near(crop(img, PX + 8, PY + 300, 180, 200), BAR)))
check("the layout button puts the tabs on top", strip > 0.9 and gone < 0.2, f"strip {strip:.2f}, shell colour where the sidebar was {gone:.2f}")
PX0, PY0, PW, PH = T["bounds"]["reported"]
PAGE_H = (PX0 + 56, PY0 + 12, PW - 112, PH - 32)  # An inside part of the page, tabs on top.
content = float(np.mean(near(crop(img, PX + PAGE_H[0], PY + PAGE_H[1], PAGE_H[2], PAGE_H[3]), BAR, 4)))
edge = float(np.mean(near(crop(img, PX + 1, PY + 100, 2, H - 200), FRAME)))
check("tabs on top: shell draws over the page", strip > 0.9 and edge > 0.9 and content < 0.2, f"frame {edge:.2f}, shell colour in page {content:.2f}")
print(f"tabs on top: window buttons {lights(img, home)}", flush=True)

def page_until(want, limit=PAGE_LOAD + 5):
    """The page, tabs on top, once want(page) holds, or as it is after the limit. A click or key
    that lands late (timed by the layout checks) mustn't land on a later step, so each step waits
    for its own result before the next, and a late one can't land on the step after it."""
    end = time.time() + limit
    now = page(PAGE_H)
    while not want(now) and time.time() < end:
        time.sleep(0.3)
        now = page(PAGE_H)
    return now


on_top = page(PAGE_H)
desk.key("cmd+t")
blank_tab = page_until(lambda p: changed(on_top, p) > 0.001)
check("Cmd+T adds a tab, tabs on top", changed(on_top, blank_tab) > 0.001)
click_on(T["tabs"][0])
check("a click reaches a tab at title-bar height", changed(page_until(lambda p: changed(p, on_top) < 0.01), on_top) < 0.01 and still())
click_on(T["tabs"][1])
page_until(lambda p: changed(p, blank_tab) < 0.01)
desk.key("cmd+w")
page_until(lambda p: changed(p, on_top) < 0.01)
click_on(T["field"])
time.sleep(0.4)
desk.type(BRAVO)
desk.key("return")
check("a click reaches the address bar, tabs on top", changed(page_until(lambda p: changed(p, on_top) > 0.001), on_top) > 0.001 and still())
click_on(T["back"])
check("a click reaches the back button, tabs on top", changed(page_until(lambda p: changed(p, on_top) < 0.01), on_top) < 0.01 and still())

STRIP = ("the empty strip", W - (T["w"] - T["rest"][0] - T["rest"][2]) - 60, middle(T["rest"])[1])
for place in (STRIP, TOP_EDGE):
    drags(*place, home)
for place in RESIZE_EDGES:
    drags(*place, home, "resizes the window")
zooms(*STRIP, home)
full_screen("tabs on top, green button", lambda: desk.click(PX + L["zoom"][0], PY + L["zoom"][1]), home, BAR, on_top=True)

switch_layout(T["layout"], False, "brings the tabs back to the side")
start = time.time()
while time.time() - start < 5 and float(np.mean(near(crop(desk.image(), PX + 8, PY + 300, 180, 200), BAR))) <= 0.9:
    time.sleep(0.1)
took = time.time() - start
check("after the tabs move back to the side, the sidebar shows within 1 s", took <= 1, f"shown after {took:.1f} s")
shot("back-to-vertical")
img = desk.image()
check("the layout button brings the tabs back to the side", float(np.mean(near(crop(img, PX + 8, PY + 300, 180, 200), BAR))) > 0.9)

check("Cmd+Q ends the app", quit_app())
window_checks()
move_checks()
state_checks()
finished = True
sys.exit(1 if counts()["FAIL"] else 0)
