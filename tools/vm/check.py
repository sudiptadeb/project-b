#!/usr/bin/env python3
"""Checks the installed app in the VM with real clicks and keys.

Start the VM and install a DMG first (docs/vm-testing.md). Prints pass or FAIL for each check
with what it measured, and saves a shot per step. Everything the app receives comes through
VNC; window positions are read from macOS's window list inside the VM. Positions in the
window come from the shell's source and are in points.
"""

import json
import os
import subprocess
import sys
import time

import numpy as np

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import rfb

VM_HOME = os.path.expanduser("~/PW_TOOLS/macos-vm")
TART = VM_HOME + "/bin/tart.app/Contents/MacOS/tart"
VM = "pb-test"
SHOTS = time.strftime(VM_HOME + "/shots/%Y%m%d-%H%M%S")
BUNDLE_ID = "dev.projectb.browser"
APP = "/Applications/Project B.app"
SCREEN = (1440, 900)
FRAME = (15, 170, 170)  # --frame and --accent
BAR = (226, 242, 242)  # --bar
WHITE = (255, 255, 255)  # --surface
BUTTONS_X = (12, 72)  # Where the window buttons belong, left of close to right of zoom.
BUTTONS_Y = 20  # Their middle, which is also the middle of the top row.
PAGE_LOAD = 3.0
ANIMATION = 2.0
MOVE = (60, 40)

os.environ["TART_HOME"] = VM_HOME + "/tart-home"
os.makedirs(SHOTS)
passed = failed = 0


def check(name, ok, detail=""):
    global passed, failed
    passed, failed = passed + bool(ok), failed + (not ok)
    print(f"{'pass' if ok else 'FAIL'}  {name}: {detail}", flush=True)


def guest(*cmd):
    return subprocess.run([TART, "exec", VM, *cmd], capture_output=True, text=True, timeout=60).stdout.strip()


WINDOWS = """
ObjC.import("CoreGraphics");
const all = ObjC.deepUnwrap(ObjC.castRefToObject($.CGWindowListCopyWindowInfo(1, 0)));
JSON.stringify(all.filter(w => w.kCGWindowLayer == 0 && ["Project B", "project_b"].includes(w.kCGWindowOwnerName))
    .map(w => w.kCGWindowBounds));
"""


def window():
    """The app's largest window as (x, y, w, h), or None."""
    found = json.loads(guest("/usr/bin/osascript", "-l", "JavaScript", "-e", WINDOWS) or "[]")
    if not found:
        return None
    w = max(found, key=lambda b: b["Width"] * b["Height"])
    return (w["X"], w["Y"], w["Width"], w["Height"])


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
    s = vnc.scale
    return img[int(y * s):int((y + h) * s), int(x * s):int((x + w) * s)]


def near(img, colour, tolerance=16):
    return np.all(np.abs(img.astype(np.int16) - np.array(colour, dtype=np.int16)) <= tolerance, axis=2)


def changed(a, b):
    """The share of pixels that differ between two crops of the same size."""
    if a.shape != b.shape or a.size == 0:
        return 1.0
    return float(np.mean(np.any(np.abs(a.astype(np.int16) - b.astype(np.int16)) > 16, axis=2)))


def shot(name):
    return vnc.shot(f"{SHOTS}/{name}.png")


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
        found[name] = ((xs.min() + xs.max() + 1) / 2 / vnc.scale, (ys.min() + ys.max() + 1) / 2 / vnc.scale,
                       (xs.max() - xs.min() + 1) / vnc.scale)
    return found


def field_middle(img, win, x):
    """The address field's vertical middle in window points: the first run of white at column x.

    Only the first run, because a white page can start right below the field.
    """
    ys = np.nonzero(near(crop(img, win[0] + x, win[1], 1, 90), WHITE, 6)[:, 0])[0]
    if len(ys) == 0:
        return None
    last = ys[np.argmax(np.diff(ys, append=ys[-1] + 2) > 1)]
    return (ys[0] + last + 1) / 2 / vnc.scale


def launch():
    """Opens the app the way Finder does. Returns seconds to the shell's first paint, or None."""
    start = time.time()
    guest("/usr/bin/open", APP)
    need = 80000 * vnc.scale * vnc.scale  # About half the window's pale shell.
    while time.time() - start < 60:
        if near(vnc.image(), BAR, 8).sum() > need:
            return time.time() - start
        time.sleep(0.01)
    return None


def quit_app():
    vnc.key("cmd+q")
    for _ in range(50):
        if guest("/usr/bin/pgrep", "-x", "project_b") == "":
            return True
        time.sleep(0.2)
    guest("/usr/bin/killall", "project_b")
    return False


def drags(name, x, y, home, expect="moves the window"):
    """expect is "moves the window", or "resizes the window" for the edges AppKit keeps for itself."""
    win = window()
    vnc.drag(win[0] + x, win[1] + y, win[0] + x + MOVE[0], win[1] + y + MOVE[1])
    time.sleep(0.6)
    now = window()
    moved, resized = (now[0] - win[0], now[1] - win[1]), (now[2] - win[2], now[3] - win[3])
    if expect == "moves the window":
        ok = abs(moved[0] - MOVE[0]) <= 3 and abs(moved[1] - MOVE[1]) <= 3 and resized == (0, 0)
    else:
        ok = resized != (0, 0)
    check(f"dragging {name} {expect}", ok, f"moved {moved}, resized {resized}")
    set_window(home)


def zooms(name, x, y, home):
    """Zoom grows both ways. Double-clicking a window edge grows just one, so it doesn't count."""
    win = window()
    vnc.dbl(win[0] + x, win[1] + y)
    time.sleep(ANIMATION)
    now = window()
    check(f"double-clicking {name} zooms", now[2] > win[2] and now[3] > win[3], f"size {win[2:]} to {now[2:]}")
    set_window(home)


def selects_text(home):
    """Dragging across text in the address field selects it and leaves the window alone."""
    vnc.click(PX + 500, PY + TOP)
    time.sleep(0.4)
    vnc.type("select these words")
    time.sleep(0.3)
    win = window()
    vnc.drag(win[0] + AFTER_BUTTONS + 130, win[1] + TOP, win[0] + AFTER_BUTTONS + 230, win[1] + TOP)
    time.sleep(0.6)
    now = window()
    shot("address-field-drag")
    check("dragging across the address field selects text, not the window", now == win, f"window moved from {win} to {now}")
    vnc.key("esc")
    vnc.key("esc")
    time.sleep(0.3)
    set_window(home)


def shown_green_button(name):
    """In full screen, shows the hidden title bar with the pointer, as a person would.

    Returns where its green button is, or None.
    """
    vnc.move(SCREEN[0] / 2, 40, 0.2)
    vnc.move(SCREEN[0] / 2, 0, 2.0)
    img = vnc.image()
    shot("fullscreen-reveal-" + name)
    box = crop(img, 0, 0, 160, 120).astype(np.int16)
    ys, xs = np.nonzero((box[..., 1] > 150) & (box[..., 0] < 120) & (box[..., 2] < 120))
    if len(xs) < 20:
        return None
    return (xs.min() + xs.max() + 1) / 2 / vnc.scale, (ys.min() + ys.max() + 1) / 2 / vnc.scale


def leave_full_screen(name):
    """Clicks the green button in the shown title bar. Returns whether there was one."""
    green = shown_green_button(name)
    if green is None:
        return False
    vnc.click(*green)
    time.sleep(ANIMATION)
    # Resting on the green button opens macOS's Move & Resize menu, which takes the next click.
    vnc.move(SCREEN[0] / 2, SCREEN[1] / 2, 1.0)
    return True


def full_screen(name, enter, home, first_control):
    enter()
    time.sleep(ANIMATION)
    win = window()
    shot("fullscreen-" + name)
    img = vnc.image()
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
    img = vnc.image()
    shot("fullscreen-exit-" + name)
    # macOS puts the window back where it last placed it itself, which can be the zoomed
    # frame rather than where these checks moved it, so any window frame counts.
    check(f"{name}: the green button in the shown title bar brings the window back",
          clicked and win is not None and win[2:] != SCREEN and lights(img, win) is not None, f"window {win}")
    if win is not None and win[2:] == SCREEN:
        # Leave without the green button, so the checks after this still mean something.
        guest("/usr/bin/osascript", "-e",
              f'tell application "System Events" to tell (first process whose bundle identifier is "{BUNDLE_ID}") '
              'to set value of attribute "AXFullScreen" of window 1 to false')
        time.sleep(ANIMATION)
    if window() != home:
        set_window(home)


# Start clean: nothing running, no profile, so the tabs start on the side.
guest("/usr/bin/killall", "project_b")
guest("/bin/rm", "-rf", "/Users/admin/.project-b")
time.sleep(1)
vnc = rfb.Vnc()
print(f"display {vnc.width}x{vnc.height}, shots in {SHOTS}", flush=True)
shot("desktop")
# After a boot the image's login item leaves an "App Background Activity" notice at the top
# right, over the window's right end. Swipe it away as a person would; on a clear desktop
# this does nothing. Low on the notice, below the "screen is being controlled" one that
# connecting puts over its top.
vnc.drag(SCREEN[0] - 120, 135, SCREEN[0] - 1, 135, 15)
time.sleep(1)

# First paint: the first launch of an install, then three more.
paints = []
for i in range(4):
    paints.append(launch())
    time.sleep(1.5)
    shot(f"launch-{i}")
    if i < 3 and not quit_app():
        print(f"Cmd+Q left the app running on launch {i}", flush=True)
    time.sleep(1)
print("first paint, seconds: " + ", ".join("none" if p is None else f"{p:.2f}" for p in paints), flush=True)
check("first paint under 5 s", all(p is not None and p < 5 for p in paints), str(paints))

# The window and its buttons.
home = window()
check("window is on screen", home is not None and home[2] > 400, f"window {home}")
if home is None:
    sys.exit("no window, nothing else can run")
W, H = home[2], home[3]
PX, PY = home[0], home[1]
img = vnc.image()
sampled = img[int((PY + H / 2) * vnc.scale), int((PX + 100) * vnc.scale)]
print(f"sidebar colour on screen {sampled.tolist()}, the shell asks for {list(BAR)}", flush=True)
L = lights(img, home)
check("window buttons are visible", L is not None, str(L))
if L is None:
    sys.exit("no window buttons found: the window isn't active, or the colours need calibrating")
buttons = (L["close"][0] - L["close"][2] / 2, L["zoom"][0] + L["zoom"][2] / 2)
check(f"window buttons run from x {BUTTONS_X[0]} to {BUTTONS_X[1]}",
      abs(buttons[0] - BUTTONS_X[0]) <= 1 and abs(buttons[1] - BUTTONS_X[1]) <= 1, f"x {buttons[0]} to {buttons[1]}")
check(f"window buttons' middle is at y {BUTTONS_Y}", abs(L["close"][1] - BUTTONS_Y) <= 1, f"y {L['close'][1]}")
TOP = field_middle(img, home, 500)
if TOP is None:
    sys.exit("no address field found, so nothing below knows where to click")
check("window buttons centre on the address field", abs(TOP - L["close"][1]) <= 1, f"buttons y {L['close'][1]}, field y {TOP}")
AFTER_BUTTONS = L["zoom"][0] + L["zoom"][2] / 2 + 8  # Where the shell's first control starts.
PAGE_V = (260, 60, W - 280, H - 80)  # An inside part of the page, tabs on the side.
NEUTRAL = (PX + W / 2, PY + H / 2)


def page(box=PAGE_V, limit=15):
    """The page once it has stopped changing for half a second, or as it is after the limit."""
    vnc.move(*NEUTRAL, 0.3)
    last, since, end = None, time.time(), time.time() + limit
    while time.time() < end:
        now = crop(vnc.image(), PX + box[0], PY + box[1], box[2], box[3])
        if last is None or changed(last, now) > 0.001:
            last, since = now, time.time()
        elif time.time() - since >= 0.5:
            break
        time.sleep(0.1)
    return last


def still():
    return window() == home


# Real clicks at title-bar height, tabs on the side.
blank = page()
vnc.click(PX + 500, PY + TOP)
time.sleep(0.4)
vnc.type("example.com")
vnc.key("return")
time.sleep(PAGE_LOAD)
example = page()
shot("vertical-example")
check("a click reaches the address bar", changed(blank, example) > 0.001 and still(), f"page changed {changed(blank, example):.3f}")

img = vnc.image()
sidebar_bar = float(np.mean(near(crop(img, PX + 8, PY + 300, 180, 200), BAR)))
edge = float(np.mean(near(crop(img, PX + 1, PY + 100, 2, H - 200), FRAME)))
page_bar = float(np.mean(near(crop(img, PX + PAGE_V[0], PY + PAGE_V[1], PAGE_V[2], PAGE_V[3]), BAR, 4)))
check("tabs on the side: shell draws over the page", sidebar_bar > 0.9 and edge > 0.9 and page_bar < 0.2,
      f"sidebar {sidebar_bar:.2f}, frame {edge:.2f}, shell colour in page {page_bar:.2f}")

vnc.click(PX + 500, PY + TOP)
time.sleep(0.4)
vnc.type("iana.org/help/example-domains")
vnc.key("return")
time.sleep(PAGE_LOAD)
iana = page()
check("a second page loads", changed(example, iana) > 0.001, f"page changed {changed(example, iana):.3f}")

vnc.click(PX + AFTER_BUTTONS + 40, PY + TOP)
time.sleep(PAGE_LOAD)
back = page()
shot("after-back")
check("a click reaches the back button", changed(back, example) < 0.01 and still(), f"differs from first page {changed(back, example):.3f}")
vnc.click(PX + AFTER_BUTTONS + 68, PY + TOP)
time.sleep(PAGE_LOAD)
forward = page()
check("a click reaches the forward button", changed(forward, iana) < 0.01 and still(), f"differs from the second page {changed(forward, iana):.3f}")

# Suggestions: Cmd+L from the page, type, pick one with a real click over the page.
vnc.click(*NEUTRAL)
time.sleep(0.3)
before = crop(vnc.image(), PX + 300, PY + 34, 400, 60)
vnc.key("cmd+l")
time.sleep(0.3)
vnc.type("exam")
time.sleep(0.8)
listed = crop(vnc.image(), PX + 300, PY + 34, 400, 60)
shot("suggestions")
check("Cmd+L and typing shows suggestions over the page", changed(before, listed) > 0.05, f"changed {changed(before, listed):.3f}")
vnc.click(PX + 500, PY + TOP + 38)
time.sleep(PAGE_LOAD)
check("a click picks a suggestion", changed(page(), example) < 0.01, f"differs from example.com {changed(page(), example):.3f}")

# Tabs by key, compared by what the sidebar shows.
SIDE = (8, 36, 180, 300)


def side():
    vnc.move(*NEUTRAL, 0.3)
    return crop(vnc.image(), PX + SIDE[0], PY + SIDE[1], SIDE[2], SIDE[3])


one = side()
vnc.key("cmd+t")
start = time.time()
two = one
while changed(one, two) <= 0.005 and time.time() - start < 5:
    time.sleep(0.05)
    two = crop(vnc.image(), PX + SIDE[0], PY + SIDE[1], SIDE[2], SIDE[3])
took = time.time() - start
check("Cmd+T adds a tab within 0.5 s", changed(one, two) > 0.005 and took <= 0.5,
      f"the tab showed after {took:.2f} s" if changed(one, two) > 0.005 else "no tab after 5 s")
if changed(one, two) <= 0.005:
    # So the checks below still have their two tabs.
    vnc.key("cmd+t")
time.sleep(0.5)
two = side()
shot("two-tabs")
vnc.click(PX + 100, PY + 78)
time.sleep(1)
check("a click reaches a tab in the sidebar", changed(page(), example) < 0.01)
vnc.click(PX + 100, PY + 110)
time.sleep(1)
vnc.key("cmd+t")
time.sleep(1)
vnc.key("cmd+w")
time.sleep(1)
check("Cmd+W closes a tab", changed(side(), two) < 0.005, f"differs from two tabs {changed(side(), two):.3f}")
vnc.key("cmd+t")
time.sleep(1)
vnc.key("cmd+w")
vnc.key("cmd+w")
time.sleep(1)
now = side()
shot("cmd-w-twice")
check("Cmd+W twice fast closes two tabs", changed(now, one) < 0.005,
      f"differs from one tab {changed(now, one):.3f}, from two tabs {changed(now, two):.3f}")
if changed(now, one) >= 0.005:
    vnc.key("cmd+w")
    time.sleep(1)

# Dragging and double-clicking, tabs on the side. The top row is in the title bar; the empty
# sidebar is below it. The outer 3 points of the left and right edges are AppKit's resize zone.
gap = L["zoom"][0] + L["zoom"][2] / 2 + 4
TOP_EDGE = ("the top frame edge", W / 2, 2)
RESIZE_EDGES = (("the left edge", 1, H / 2), ("the right edge", W - 1, H / 2))
SIDE_PLACES = (("the gap right of the buttons", gap, 14), ("the end of the top row", W - 6, TOP),
               ("the empty sidebar", 100, H - 40))
for place in SIDE_PLACES + (TOP_EDGE,):
    drags(*place, home)
for place in RESIZE_EDGES:
    drags(*place, home, "resizes the window")
selects_text(home)
for place in SIDE_PLACES:
    zooms(*place, home)

# Full screen, both ways in.
full_screen("green button", lambda: vnc.click(PX + L["zoom"][0], PY + L["zoom"][1]), home, FRAME)
full_screen("Ctrl+Cmd+F", lambda: vnc.key("ctrl+cmd+f"), home, FRAME)

# Resizing, on a page of one colour so its edges can be measured in every frame.
PINK = (255, 0, 255)
vnc.click(PX + 500, PY + TOP)
time.sleep(0.4)
vnc.type("data:text/html,<body bgcolor=fuchsia>")
vnc.key("return")
time.sleep(PAGE_LOAD)
vnc.move(*NEUTRAL, 0.3)


def page_box():
    """The page's bounds on screen in points, from its colour, or None."""
    ys, xs = np.nonzero(near(vnc.image(), PINK, 10))
    if len(xs) == 0:
        return None
    s = vnc.scale
    return (xs.min() / s, ys.min() / s, (xs.max() + 1) / s, (ys.max() + 1) / s)




def slot(win):
    """Where the page belongs in a window."""
    return (win[0] + INSET[0], win[1] + INSET[1], win[0] + win[2] - INSET[2], win[1] + win[3] - INSET[3])


def page_timeline(action, seconds):
    """Runs action and records the page's bounds for that long. Returns them and the window after."""
    start = time.time()
    action()
    boxes = []
    while time.time() - start < seconds:
        vnc.wait_frame(0.2)
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
    vnc.move(x1, y1, 0.4)
    vnc.press()
    time.sleep(0.2)
    torn = []
    for i in range(1, 31):
        vnc.move(x1 + 8 * i, y1 + 3 * i, 0.0)
        vnc.wait_frame(1.0)
        box = page_box()
        # The frame's right end on one row, looking no further right than the pointer: the
        # desktop picture can have the frame's colour in it.
        row = np.nonzero(near(crop(vnc.image(), PX, PY + 300, x1 + 8 * i + 10 - PX, 1), FRAME, 12)[0])[0]
        frame_right = PX + (row.max() + 1) / vnc.scale if len(row) else None
        if i == 15:
            shot("live-resize")
        if box is None or frame_right is None or not 0 <= frame_right - box[2] <= INSET[2]:
            torn.append((i, box, frame_right))
        time.sleep(0.03)
    vnc.release()
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
    boxes, win = page_timeline(lambda: vnc.dbl(PX + gap, PY + 14), 4)
    at = in_place_at(boxes, win)
    check("after zoom, the page fills its new place within 2 s", at is not None and at <= 2, f"in place after {at} s, window {win}")
    boxes, win = page_timeline(lambda: vnc.dbl(win[0] + gap, win[1] + 14), 4)
    at = in_place_at(boxes, win)
    check("after zooming back, the page fills its place within 2 s", at is not None and at <= 2, f"in place after {at} s, window {win}")
    set_window(home)

    # Leaving full screen.
    vnc.click(PX + L["zoom"][0], PY + L["zoom"][1])
    time.sleep(ANIMATION)
    target = shown_green_button("resize")
    if target is not None:
        boxes, win = page_timeline(lambda: vnc.click(*target), 5)
        vnc.move(SCREEN[0] / 2, SCREEN[1] / 2, 1.0)  # Off the green button, as in leave_full_screen.
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


# Tabs on top.
vnc.click(PX + W - 20, PY + TOP)
time.sleep(1)
shot("horizontal")
img = vnc.image()
strip = float(np.mean(near(crop(img, PX + W - 200, PY + 6, 150, 20), FRAME)))
gone = float(np.mean(near(crop(img, PX + 8, PY + 300, 180, 200), BAR)))
check("the layout button puts the tabs on top", strip > 0.9 and gone < 0.2, f"strip {strip:.2f}, shell colour where the sidebar was {gone:.2f}")
TOP_H = field_middle(img, home, 500)
if TOP_H is None:
    sys.exit("no address field found with the tabs on top")
PAGE_H = (60, 80, W - 120, H - 100)
content = float(np.mean(near(crop(img, PX + PAGE_H[0], PY + PAGE_H[1], PAGE_H[2], PAGE_H[3]), BAR, 4)))
edge = float(np.mean(near(crop(img, PX + 1, PY + 100, 2, H - 200), FRAME)))
check("tabs on top: shell draws over the page", strip > 0.9 and edge > 0.9 and content < 0.2, f"frame {edge:.2f}, shell colour in page {content:.2f}")
print(f"tabs on top: window buttons {lights(img, home)}", flush=True)

on_top = page(PAGE_H)
vnc.key("cmd+t")
time.sleep(1)
blank_tab = page(PAGE_H)
check("Cmd+T adds a tab, tabs on top", changed(on_top, blank_tab) > 0.001)
vnc.click(PX + AFTER_BUTTONS + 76, PY + TOP)
time.sleep(1)
check("a click reaches a tab at title-bar height", changed(page(PAGE_H), on_top) < 0.01 and still())
vnc.click(PX + AFTER_BUTTONS + 76 + 152, PY + TOP)
time.sleep(1)
vnc.key("cmd+w")
time.sleep(1)
vnc.click(PX + 500, PY + TOP_H)
time.sleep(0.4)
vnc.type("iana.org/help/example-domains")
vnc.key("return")
time.sleep(PAGE_LOAD)
check("a click reaches the address bar, tabs on top", changed(page(PAGE_H), on_top) > 0.001 and still())
vnc.click(PX + 20, PY + TOP_H)
time.sleep(PAGE_LOAD)
check("a click reaches the back button, tabs on top", changed(page(PAGE_H), on_top) < 0.01 and still())

STRIP = ("the empty strip", W - 100, 14)
for place in (STRIP, TOP_EDGE):
    drags(*place, home)
for place in RESIZE_EDGES:
    drags(*place, home, "resizes the window")
zooms(*STRIP, home)
full_screen("tabs on top, green button", lambda: vnc.click(PX + L["zoom"][0], PY + L["zoom"][1]), home, BAR)

vnc.click(PX + W - 20, PY + TOP_H)
time.sleep(1)
shot("back-to-vertical")
img = vnc.image()
check("the layout button brings the tabs back to the side", float(np.mean(near(crop(img, PX + 8, PY + 300, 180, 200), BAR))) > 0.9)

check("Cmd+Q ends the app", quit_app())
print(f"{passed} passed, {failed} failed, shots in {SHOTS}", flush=True)
sys.exit(1 if failed else 0)
