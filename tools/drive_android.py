#!/usr/bin/env python3
"""Drives the real Android app on an emulator: real touches, real keys, real screenshots.

Where to touch comes from the app itself: Android's accessibility tree, which content
fills with the shell's controls by their labels, the settings page and the shown page,
each with its place on the screen. Nothing here is a fixed position, so a change to the
shell's layout needs no new numbers. A small local web server is the oracle for what the
page received, and the app's own history file for what the managers kept. Results and
screenshots go to build/android-drive-content.

    amarit start project-b-emu          # the emulator; docs/android.md says how it was made
    tools/test.sh android               # or: python3 tools/drive_android.py

ADB_SERIAL picks another device than emulator-5600, and PB_APK another APK than
build/android/project-b-content.apk. The APK is installed over what is there and the
app's data is cleared.
"""
import http.server
import json
import os
import re
import subprocess
import sys
import threading
import time
import xml.etree.ElementTree as ET
import zipfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SDK = os.path.expanduser("~/Library/Android/sdk")
ADB = [SDK + "/platform-tools/adb", "-s", os.environ.get("ADB_SERIAL", "emulator-5600")]
APK = os.environ.get("PB_APK") or ROOT + "/build/android/project-b-content.apk"
PACKAGE = "dev.projectb.browser"
ACTIVITY = ".BrowserActivity"
PROFILE = "app_project_b/profile"
OUT = os.environ.get("OUT", ROOT + "/build/android-drive-content")
FIELD = "android.widget.EditText"  # The address field is the shell's only text field.

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

# Web APIs whose browser side is a frame interface: Chrome binds them, content doesn't. When the
# browser has no binder, content kills the asking renderer for a bad message, as DuckDuckGo's
# search did on Android with getInstalledRelatedApps. The ones that open a system picker over
# the app are left out, since the picker would take the touches meant for the next part.
WEB_APIS = {
    "installedApps": "navigator.getInstalledRelatedApps()",
    "share": "navigator.share({url: location.href})",
    "passwordCredential": "navigator.credentials.store(new PasswordCredential({id: 'a', password: 'b'}))",
    "credentialsGet": "navigator.credentials.get({password: true, mediation: 'silent'})",
    "otp": "navigator.credentials.get({otp: {transport: ['sms']}, signal: AbortSignal.timeout(1500)})",
    "webauthn": "navigator.credentials.create({publicKey: {challenge: new Uint8Array(16), rp: {name: 'x'},"
                " user: {id: new Uint8Array(4), name: 'u', displayName: 'u'}, pubKeyCredParams: [{type: 'public-key', alg: -7}], timeout: 1500}})",
    "fedcm": "navigator.credentials.get({identity: {providers: [{configURL: 'https://example.com/c', clientId: 'x'}]},"
             " signal: AbortSignal.timeout(1500)})",
    "setAppBadge": "navigator.setAppBadge(3)",
    "paymentRequest": "new PaymentRequest([{supportedMethods: 'https://example.com/pay'}],"
                      " {total: {label: 't', amount: {currency: 'USD', value: '1'}}}).canMakePayment()",
    "digitalGoods": "getDigitalGoodsService('https://play.google.com/billing')",
    "subApps": "navigator.subApps.list()",
    "contacts": "navigator.contacts.select(['name'])",
    "idle": "IdleDetector.requestPermission()",
    "wakeLock": "navigator.wakeLock.request('screen')",
    "nfc": "new NDEFReader().scan()",
    "battery": "navigator.getBattery()",
    "geolocation": "new Promise((ok, no) => navigator.geolocation.getCurrentPosition(ok, no, {timeout: 1500}))",
    "notification": "Notification.requestPermission()",
    "persist": "navigator.storage.persist()",
    "clipboard": "navigator.clipboard.writeText('x')",
    "enumerateDevices": "navigator.mediaDevices.enumerateDevices()",
    "orientationLock": "screen.orientation.lock('landscape')",
    "bluetooth": "navigator.bluetooth.getAvailability()",
    "usb": "navigator.usb.getDevices()",
    "serial": "navigator.serial.getPorts()",
    "hid": "navigator.hid.getDevices()",
    "presentation": "new PresentationRequest(['https://example.com']).getAvailability()",
    "keyboardLock": "navigator.keyboard.lock()",
    "speechSynthesis": "speechSynthesis.getVoices()",
    "eyeDropper": "new EyeDropper().open()",
    "windowManagement": "getScreenDetails()",
    "localFonts": "queryLocalFonts()",
    "storageAccess": "document.requestStorageAccess()",
    "topics": "document.browsingTopics()",
    "sharedStorage": "sharedStorage.set('a', 'b')",
    "adAuction": "navigator.runAdAuction({})",
    "interestGroup": "navigator.joinAdInterestGroup({})",
    "privateToken": "document.hasPrivateToken('https://example.com')",
    "launchQueue": "launchQueue.setConsumer(() => {})",
    "xr": "navigator.xr.isSessionSupported('immersive-vr')",
    "midi": "navigator.requestMIDIAccess()",
    "webgpu": "navigator.gpu.requestAdapter()",
    "translator": "Translator.availability({sourceLanguage: 'en', targetLanguage: 'fr'})",
    "summarizer": "Summarizer.availability()",
    "languageModel": "LanguageModel.availability()",
    "pressure": "new PressureObserver(() => {}).observe('cpu')",
    "handwriting": "navigator.createHandwritingRecognizer({languages: ['en']})",
    "barcode": "new BarcodeDetector().detect(new ImageData(1, 1))",
    "face": "new FaceDetector().detect(new ImageData(1, 1))",
    "text": "new TextDetector().detect(new ImageData(1, 1))",
}

# Calls every one of them when the page loads and again on a tap, which some need, waiting at most
# 2 s for each, then says so; and says again a moment later, so a renderer killed after it
# answered still shows.
PAGES["/apis"] = ("<title>APIs</title><button style='position:fixed;inset:0;width:100%;height:100%'>apis</button><script>"
                  "const calls = {" + ",".join("%s: () => %s" % kv for kv in WEB_APIS.items()) + "};"
                  "function run(when) {"
                  " Promise.all(Object.values(calls).map(call => { try { return Promise.race([Promise.resolve(call()).catch(() => 0),"
                  " new Promise(done => setTimeout(done, 2000))]) } catch (e) { return 0 } }))"
                  " .then(() => fetch('/apis-ran?' + when)).then(() => setTimeout(() => fetch('/apis-alive?' + when), 3000)) }"
                  "run('load'); document.querySelector('button').onclick = () => run('tap')</script>")

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


server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Server)
threading.Thread(target=server.serve_forever, daemon=True).start()
PORT = server.server_port
BASE = "localhost:%d" % PORT


def adb(*args):
    return subprocess.run(ADB + list(args), capture_output=True, text=True, errors="replace").stdout


def shot(name):
    with open("%s/%s.png" % (OUT, name), "wb") as f:
        subprocess.run(ADB + ["exec-out", "screencap", "-p"], stdout=f)


def until(condition, timeout=15):
    """Waits for condition to give something other than None or False, and returns it. When
    time runs out it returns None."""
    deadline = time.time() + timeout
    while True:
        value = condition()
        if value is not None and value is not False:
            return value
        if time.time() > deadline:
            return None
        time.sleep(0.1)


def asked(path, timeout=15):
    """Waits until the page has asked the local server for a path that starts this way."""
    return until(lambda: any(s.startswith(path) for s in served), timeout)


# What is on the screen, asked from the app. Content gives Android's accessibility tree one
# WebView per page it shows: the shell, the settings page while it is open, and the shown
# tab's page. A node's attributes are Android's: class, text, content-desc (an aria-label),
# focused, selected, and bounds in device pixels.
PARTS = {
    "shell": lambda labels: "Menu" in labels,
    "settings": lambda labels: "Close settings" in labels,
    "page": lambda labels: "Menu" not in labels and "Close settings" not in labels,
}


def screen_parts():
    """The parts on screen now, by name. Empty while Android can't read the screen, as during
    an animation, or before content has built its tree."""
    out = adb("exec-out", "uiautomator", "dump", "/dev/tty")
    if "</hierarchy>" not in out:
        return {}
    root = ET.fromstring(out[:out.rindex("</hierarchy>") + len("</hierarchy>")])
    parts = {}
    for view in root.iter("node"):
        if view.get("class") == "android.webkit.WebView":
            labels = {n.get("content-desc") for n in view.iter("node")}
            parts[next(name for name, test in PARTS.items() if test(labels))] = view
    return parts


def find(part, **want):
    """The first node in a part of the screen whose attributes are all as wanted, or None.
    A wanted value is the attribute's text, or a function of it. label is content-desc,
    kind is class."""
    names = {"label": "content-desc", "kind": "class"}
    view = screen_parts().get(part)
    for node in view.iter("node") if view is not None else ():
        if all(v(node.get(names.get(k, k))) if callable(v) else node.get(names.get(k, k)) == v for k, v in want.items()):
            return node
    return None


def see(part, timeout=20, **want):
    """Waits for such a node to show, and returns it."""
    node = until(lambda: find(part, **want), timeout)
    if node is None:
        raise TimeoutError("never showed in the %s: %s" % (part, want))
    return node


def box(node):
    """A node's place on the screen: left, top, right, bottom."""
    return tuple(map(int, re.findall(r"\d+", node.get("bounds"))))


def middle(node):
    left, top, right, bottom = box(node)
    return (left + right) // 2, (top + bottom) // 2


def tap(point):
    adb("shell", "input", "tap", str(point[0]), str(point[1]))


def press(label):
    """Taps one of the shell's controls, found by its label."""
    tap(middle(see("shell", label=label)))


def menu(label):
    """Taps an item of the Mobile shell's menu sheet, which the ⋯ button opens. A tap the
    shell missed, as it can while it draws again after the GPU process died, is tried once
    more."""
    for _ in range(3):
        press("Menu")
        if until(lambda: find("shell", label=label), 5) is not None:
            break
    press(label)


def host(address):
    """What the Mobile shell's address pill shows of an address at rest: its host."""
    return re.sub(r"^https?://(www\.)?", "", address).split("/")[0]


def key(code):
    adb("shell", "input", "keyevent", str(code))


def type_text(text):
    adb("shell", "input", "text", text.replace(" ", "%s"))


def type_in_bar(text):
    """Taps the address field and types, waiting for the field to have the keyboard and
    then to show what was typed."""
    tap(middle(see("shell", kind=FIELD)))
    see("shell", kind=FIELD, focused="true")
    type_text(text)
    see("shell", kind=FIELD, text=lambda shown: shown.lower().startswith(text.lower()))


def go(text):
    type_in_bar(text)
    key(66)


def page_top():
    """A point just inside the top of the shown page, right under the shell's rows."""
    left, top, right, _ = box(see("page"))
    return (left + right) // 2, top + 10


def launch(block=""):
    adb("shell", "am", "force-stop", PACKAGE)
    # Android takes the app zygote down a few seconds after a force stop. A start inside that window
    # reused the dying zygote once and hung for good, so wait it out. Killing it ourselves is worse:
    # the system never makes another, and no renderer starts.
    until(lambda: PACKAGE + "_zygote" not in adb("shell", "ps", "-A", "-o", "NAME"))
    args = ["shell", "am", "start", "-W", "-n", PACKAGE + "/" + ACTIVITY]
    if block:
        args += ["--es", "block", block]
    adb(*args)
    # Content starts in the background, so the window shows before the shell does.
    see("shell", timeout=60, label="Menu")
    # A run that ended in a kill leaves a notice over the top of the page. It is shell, so
    # it would take the touches meant for the page.
    # The Mobile shell's has Dismiss.
    dismiss = lambda: find("shell", label="Close") or find("shell", text="Dismiss")
    notice = dismiss()
    if notice is not None:
        tap(middle(notice))
        until(lambda: dismiss() is None)


def processes():
    """The device's processes as (pid, name)."""
    return [tuple(line.split()) for line in adb("shell", "ps", "-A", "-o", "PID,NAME").splitlines() if len(line.split()) == 2]


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


def status(url, word):
    """Waits for the address pill to show url's host, then says whether the shell marks it
    with this word: Blocked, Failed or Crashed."""
    see("shell", kind=FIELD, text=host(url))
    return until(lambda: find("shell", text=word) is not None)


def alive():
    return PACKAGE in [name for _, name in processes()]


def turn(rotation):
    """Turns the phone and waits for the page to have the new shape."""
    adb("shell", "settings", "put", "system", "accelerometer_rotation", "0")
    adb("shell", "settings", "put", "system", "user_rotation", str(rotation))

    def turned():
        page = find("page")
        if page is None:
            return False
        left, top, right, bottom = box(page)
        return (right - left > bottom - top) == (rotation == 1)

    return until(turned)


def url(path):
    return "http://%s%s" % (BASE, path)


def browsing():
    """Pages, tabs, settings, touches and the gate, in one sitting."""
    launch(block="blocked.test")
    shot("01-opens-empty")

    # Type a URL.
    go(BASE + "/a")
    check("typing a URL loads it", asked("/a"))
    check("the visit reached Go's history file", until(lambda: visited(url("/a"))))
    shot("02-page-a")

    # A tap on the page belongs to the page.
    tap(middle(see("page", label="to b")))
    check("a tap on a link in the page reaches the page", asked("/b"))
    shot("03-link-tapped")

    # Back, forward, reload through the shell's menu sheet.
    served.clear()
    menu("Back")
    check("back returns to the page before", asked("/a"), str(served))
    served.clear()
    menu("Forward")
    check("forward returns to the page after", asked("/b"), str(served))
    served.clear()
    menu("Reload")
    check("reload asks the server again", asked("/b"), str(served))
    check("an emoji in a title reaches Go whole", until(lambda: history_title(url("/b")) == "Page B \U0001F642"),
          repr(history_title(url("/b"))))

    # Tabs: New Tab is in the menu, the count opens the switcher, a card per tab.
    menu("New Tab")
    see("shell", kind=FIELD, text="", focused="true")
    go(BASE + "/c")
    check("New Tab in the menu opens a tab", asked("/c"))
    press("Tabs")
    check("the count opens the switcher, a card per tab", until(lambda: find("shell", label="Close Page C")) is not None and
          find("shell", label="Close Page B \U0001F642") is not None)
    shot("06b-switcher")
    served.clear()
    tap(middle(see("shell", text="Page B \U0001F642")))
    check("a tap on a card shows that tab", until(lambda: find("page", text="b")) is not None and find("shell", label="Close Page C") is None)

    # Settings is a page of its own over the window.
    menu("Settings")
    check("the settings button opens settings over the window", until(lambda: find("settings", label="Close settings")) is not None)
    shot("06c-settings")
    key(4)
    check("the phone's back key closes settings", until(lambda: "settings" not in screen_parts()))

    # Suggestions come from history, shown over the page.
    go(BASE + "/c")
    asked("/c")
    type_in_bar("page")
    shot("07-suggestions")
    served.clear()
    tap(middle(see("shell", text=lambda shown: shown.startswith("Page A"))))
    check("a tap on a suggestion loads it", asked("/a"), str(served))
    shot("08-suggestion-tapped")

    # The phone's back key follows the same path as the shell's button.
    served.clear()
    key(4)
    check("the phone's back key goes back", asked("/c"), str(served))
    shot("09-back-key")

    # Touch routing: a tap and a scroll in the page area go to the page, not the shell above it.
    go(BASE + "/tap")
    asked("/tap")
    served.clear()
    tap(page_top())
    check("a tap just below the shell's rows reaches the page", asked("/tapped"), str(served))
    go(BASE + "/tall")
    asked("/tall")
    served.clear()
    left, top, right, bottom = box(see("page"))
    across, high = (left + right) // 2, bottom - top
    adb("shell", "input", "swipe", str(across), str(top + high * 3 // 4), str(across), str(top + high // 4), "300")
    check("a swipe scrolls the page under the shell", asked("/scrolled"), str(served[:3]))
    shot("10-scrolled")
    check("scrolling down, the shell's bars give the page their room", until(lambda: box(see("page"))[3] > bottom + 100) is not None,
          "%d then %d" % (bottom, box(see("page"))[3]))
    shot("10a-bars-away")
    adb("shell", "input", "swipe", str(across), str(top + high // 4), str(across), str(top + high * 3 // 4), "300")
    check("scrolling back up brings them back", until(lambda: box(see("page"))[3] == bottom) is not None, "%d then %d" % (bottom, box(see("page"))[3]))

    # Typing into a page goes through the keyboard to the page, not the shell.
    go(BASE + "/input")
    asked("/input")
    served.clear()
    tap(middle(see("page", kind=FIELD)))
    see("page", kind=FIELD, focused="true")
    type_text("hello")
    key(66)
    # Android 16's keyboard capitalises the first letter, as it would for a person.
    check("the keyboard types into a page", until(lambda: any("q=hello" in s.lower() for s in served)), str(served))
    shot("10b-typed-into-page")

    # Holding a finger on text starts a selection, and the app lives through it.
    go(BASE + "/words")
    asked("/words")
    left, top, _, _ = box(see("page", text=lambda shown: shown.startswith("Some words")))
    adb("shell", "input", "swipe", str(left + 100), str(top + 30), str(left + 100), str(top + 30), "900")
    check("selecting text in a page doesn't kill the app", alive())
    shot("10c-text-selected")
    key(4)

    # The phone turns: the page follows, and a tap in the new layout still lands.
    go(BASE + "/tap")
    asked("/tap")
    check("the page follows the phone into landscape", turn(1))
    shot("10d-landscape")
    served.clear()
    tap(page_top())
    check("a tap lands in the page in landscape", asked("/tapped"), str(served))
    turn(0)

    # A gate says no: for a typed URL, for a redirect and for a form's POST.
    go("blocked.test")
    check("a blocked host is marked Blocked and never joins history",
          status("https://blocked.test/", "Blocked") and not visited("https://blocked.test/"))
    shot("11-blocked")
    go(BASE + "/to-blocked")
    check("a redirect to a blocked host is stopped", status("http://blocked.test/", "Blocked") and not visited("http://blocked.test/"))
    shot("12-redirect-blocked")
    go(BASE + "/form")
    asked("/form")
    tap(page_top())
    check("a form posted to a blocked host is stopped", status("http://blocked.test/", "Blocked"))
    shot("13-post-to-blocked-host")

    # A load that fails stays out of history.
    go("https://nothing.invalid/")
    check("a failed load is marked Failed and never joins history",
          status("https://nothing.invalid/", "Failed") and not visited("https://nothing.invalid/"))
    shot("14-failed")


def long_press(point):
    adb("shell", "input", "swipe", str(point[0]), str(point[1]), str(point[0]), str(point[1]), "1000")


def mobile_shell():
    """The Mobile shell's own: the back key closes each of its sheets; a long press on a link
    opens its menu, whose Open in new tab in group gathers the tabs in the strip behind the
    page being read; and each screen in light and dark, for the owner."""
    launch()
    go(BASE + "/a")
    asked("/a")
    press("Tabs")
    see("shell", label="Close Page A")
    key(4)
    check("the back key closes the switcher", bool(until(lambda: find("shell", label="Close Page A") is None)))
    press("Menu")
    see("shell", label="Settings")
    key(4)
    check("the back key closes the menu", bool(until(lambda: find("shell", label="Settings") is None)))
    tap(middle(see("shell", kind=FIELD)))
    see("shell", kind=FIELD, focused="true")
    key(4)  # The keyboard goes first.
    key(4)
    check("the back key ends editing the address", bool(until(lambda: find("shell", kind=FIELD, focused="true") is None)))

    long_press(middle(see("page", label="to b")))
    item = see("shell", text="Open in new tab in group")
    check("a long press on a link opens the shell's menu, Open in new tab in group first", item is not None)
    shot("20-link-menu")
    key(4)
    check("the back key closes it", bool(until(lambda: find("shell", text="Open in new tab in group") is None)))
    served.clear()
    long_press(middle(see("page", label="to b")))
    tap(middle(see("shell", text="Open in new tab in group")))
    check("Open in new tab in group opens the link behind the page, and the strip shows the group",
          asked("/b") and until(lambda: find("shell", label=lambda l: (l or "").startswith("Group "))) is not None
          and find("page", label="to b") is not None)
    shot("21-strip")
    tap(middle(see("shell", label="Go to Page B \U0001F642")))
    check("a tap on a favicon in the strip shows that tab", until(lambda: find("page", text="b")) is not None)

    # Each screen in light and dark, the theme chosen in settings.
    for scheme in ("light", "dark"):
        menu("Settings")
        tap(middle(see("settings", text=scheme.capitalize())))
        key(4)
        until(lambda: "settings" not in screen_parts())
        time.sleep(1)
        shot("30-page-" + scheme)
        press("Tabs")
        see("shell", label="New Tab")
        shot("31-switcher-" + scheme)
        key(4)
        press("Menu")
        see("shell", label="Settings")
        shot("32-menu-" + scheme)
        key(4)
        tap(middle(see("shell", kind="android.widget.Button", label=lambda l: (l or "").startswith("Group "))))
        see("shell", text="Ungroup")
        shot("33-group-" + scheme)
        key(4)
        type_in_bar("page")
        see("shell", text=lambda shown: (shown or "").startswith("Page"))
        shot("34-suggestions-" + scheme)
        key(4)
        key(4)
    menu("Settings")
    tap(middle(see("settings", text="System")))
    key(4)


def history_after_a_restart():
    launch()
    type_in_bar("page")
    check("history still suggests Page A after a restart",
          until(lambda: find("shell", text=lambda shown: shown.startswith("Page A"))) is not None and visited(url("/a")))
    shot("15-suggestions-after-restart")


def renderers():
    """The pids of the app's renderers, the shell's first: it starts first."""
    return sorted(int(pid) for pid, name in children() if "sandboxed" in name)


def where_go_runs():
    """Go is in the app's process and in none of its children. The page needs a renderer first."""
    launch()
    go(BASE + "/a")
    asked("/a")
    main_pid = next(pid for pid, name in processes() if name == PACKAGE)
    kids = children()
    go_size = go_library_mappings(main_pid)
    check("Go's library is mapped in the main process", go_size > 0, "%d executable mapping(s)" % go_size)
    kids += [(pid, "app:zygote") for pid, name in processes() if name == PACKAGE + "_zygote"]
    for pid, name in kids:
        rmaps = adb("shell", "cat", "/proc/%s/maps" % pid)
        check("Go's library is not in " + name.split(":")[1], len(rmaps) > 0 and go_library_mappings(pid) == 0, "pid " + pid)
    check("the shell and the page have separate renderers", len(renderers()) >= 2, str(kids))


def the_page_renderer_dies():
    launch()
    go(BASE + "/a")
    asked("/a")
    adb("shell", "kill", "-9", str(renderers()[-1]))
    check("a page whose renderer died is marked Crashed, and the app lives", status(url("/a"), "Crashed") and alive())
    shot("16-after-page-renderer-kill")
    served.clear()
    menu("Reload")
    check("reload recovers after a crash", asked("/a"), str(served))
    shot("17-reloaded-after-crash")


def the_shell_renderer_dies():
    """It dies alone: the shell comes back, and the page stays."""
    launch()
    go(BASE + "/a")
    asked("/a")
    before = renderers()
    adb("shell", "kill", "-9", str(before[0]))
    again = until(lambda: len(set(renderers()) - set(before)) > 0)
    check("the shell gets a new renderer, and the app lives", again and alive())
    shot("17b-after-shell-renderer-kill")
    if not alive():
        return
    served.clear()
    menu("Reload")
    check("the shell works again after its renderer died", asked("/a"), str(served))


def the_gpu_process_dies():
    launch()
    go(BASE + "/a")
    asked("/a")
    privileged = lambda: {pid for pid, name in children() if "privileged" in name}
    before = privileged()
    for pid in before:
        adb("shell", "kill", "-9", pid)
    until(lambda: privileged() - before)
    shot("17c-after-gpu-kill")
    served.clear()
    menu("Reload")
    check("the app works after the GPU process died", asked("/a"), str(served))
    shot("17d-reload-after-gpu-kill")


def a_page_spins():
    """The shell and the page have separate renderers, so a page that spins leaves the shell working."""
    launch()
    go(BASE + "/busy")
    asked("/busy")
    tap(page_top())
    try:
        type_in_bar("xyz")
        check("the address field takes typing while a page spins", True)
    except TimeoutError as e:
        check("the address field takes typing while a page spins", False, e)
    shot("18-page-spinning")


def web_apis():
    """A page that calls each web API whose browser side is Chrome's keeps its renderer."""
    launch()
    adb("logcat", "-c")
    go(BASE + "/apis")
    for when in ("load", "tap"):
        if when == "tap":
            tap(middle(see("page")))
        lived = asked("/apis-alive?" + when, 30)
        # Content says which interface it found no binder for.
        killed = [line.split("Received bad user message: ", 1)[-1] for line in adb("logcat", "-d").splitlines()
                  if "bad Mojo message" in line]
        check("a page that calls %d web APIs on %s keeps its renderer" % (len(WEB_APIS), "a tap" if when == "tap" else "load"),
              lived and not killed, "; ".join(killed) or ("" if lived else "no word from the page after the calls"))
        if not lived:
            return
    shot("18b-web-apis")


def the_real_web():
    """Over the office's 443-only egress."""
    launch()
    go("example.com")
    check("a real https page joins history", until(lambda: visited("https://example.com/"), 30))
    shot("19-real-web-example")


def main():
    os.makedirs(OUT, exist_ok=True)
    adb("root")
    # adbd restarts as root a moment after saying so, and drops what is sent meanwhile.
    if not until(lambda: adb("shell", "id", "-u").strip() == "0", 30):
        sys.exit("no rooted device answers as %s: start the emulator with `amarit start project-b-emu`" % ADB[-1])
    adb("reverse", "tcp:%d" % PORT, "tcp:%d" % PORT)
    if "Success" not in adb("install", "-r", APK):
        sys.exit("can't install %s" % APK)
    adb("shell", "pm", "clear", PACKAGE)

    # Each part starts the app again, so one that the app doesn't survive costs only itself.
    for part in (browsing, mobile_shell, history_after_a_restart, where_go_runs, the_page_renderer_dies, the_shell_renderer_dies,
                 the_gpu_process_dies, a_page_spins, web_apis, the_real_web):
        try:
            part()
        except TimeoutError as e:
            # Something the next step needed never came on screen.
            shot("stopped-" + part.__name__)
            check("%s ran to its end" % part.__name__.replace("_", " "), False, "%s; the app is %s" % (e, "running" if alive() else "gone"))
    adb("shell", "am", "force-stop", PACKAGE)

    failed = [r for r in results if not r[1]]
    print("\n%d checks, %d failed" % (len(results), len(failed)))
    with open(OUT + "/results.json", "w") as f:
        json.dump([(name, ok, str(detail)) for name, ok, detail in results], f, indent=1)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
