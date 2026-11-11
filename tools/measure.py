#!/usr/bin/env python3
"""Start-up and responsiveness, from the app's own timeline (docs/logging.md).

Starts the app several times. Each time it waits for the shell to be on screen, then
opens settings, opens, shows and closes tabs and moves the tabs to the top and back, by
clicks and keys, and reads the timeline the app kept. It prints the median and the spread
of every start-up milestone and of every span, stage by stage: each interaction's, and
each click's and key press's own (input.click, input.key), from the OS making the event
to our code having it.

    python3 tools/measure.py                     # the Mac app, 10 starts
    python3 tools/measure.py -n 20
    python3 tools/measure.py --android           # the Android app on the emulator
    python3 tools/measure.py --save              # also writes the baseline, tools/baseline/<where>.json
    python3 tools/measure.py --compare           # against that baseline; or --compare FILE
    python3 tools/measure.py --trace build/measure/trace.json   # the last start, for ui.perfetto.dev

PB_BIN picks another Mac binary; PB_APK and ADB_SERIAL another APK and device. On a Mac
an agent's or ssh session has no screen, and AppKit has at times taken seconds to start
there that it doesn't take for a person. So there are two Mac baselines,
mac-headless.json for such a session and mac.json for a person's desktop, and each run
is compared with its own. For numbers a person would see, start it in the desktop
session of whoever is logged in:

    tools/on_screen.py python3 tools/measure.py --save
"""

import argparse
import datetime
import json
import os
import platform
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import timeline  # noqa: E402

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))

# A start is over when all of these are on the timeline.
STARTED = ("shell.snapshot", "shell.on_screen", "tab.committed")


class Mac:
    """The Mac app through its driver (--pb-driver), each start on a fresh profile."""

    def __init__(self):
        import drive_first_app
        self.d = drive_first_app
        # A session without a screen has numbers of its own, and a baseline of its own.
        self.session = subprocess.run(["launchctl", "managername"], capture_output=True, text=True).stdout.strip()
        self.name = "mac" if self.session == "Aqua" else "mac-headless"
        os.makedirs(self.d.OUT, exist_ok=True)
        self.entries = []
        self.last = 0

    def start(self):
        self.entries, self.last = [], 0
        self.app = self.d.App(tempfile.mkdtemp(prefix="pb-measure-"))

    def read(self):
        reply = json.loads(self.app.cmd("timeline", self.last))["result"]
        self.entries += reply["entries"]
        self.last = reply["last"]
        return self.entries

    def interact(self):
        d, app = self.d, self.app
        d.click(app, "button.settings")
        ended(self, "settings.open")
        app.cmd("key", "cmd+w")
        until(lambda: app.cmd("evalsettings", "1").startswith("ERR"))

        d.click(app, ".new-tab")
        ended(self, "tab.new")
        # A tab's close button shows under the pointer.
        x, y = d.center(app, "[role=tab][aria-selected=true]")
        app.cmd("move", x, y)
        d.click(app, "[role=tab][aria-selected=true] .close")
        ended(self, "tab.close")

        for _ in range(2):
            d.click(app, "button[aria-label^='Tabs on']")
            ended(self, "tabs.layout")

        # The same from the keyboard, and a click on the tab behind.
        app.cmd("key", "cmd+t")
        ended(self, "tab.new")
        d.click(app, "[role=tab][aria-selected=false]")
        ended(self, "tab.activate")
        app.cmd("key", "cmd+w")
        ended(self, "tab.close")

    def stop(self):
        self.app.close()

    def describe(self):
        return {"machine": platform.machine() + ", macOS " + platform.mac_ver()[0],
                "session": "a person's desktop" if self.session == "Aqua" else "no screen (%s)" % self.session}


class Android:
    """The Android app on the emulator. It has no driver, so the timeline is read as the
    app prints it, from logcat. Starts are warm for Android: its zygote is already up."""

    name = "android"

    def __init__(self):
        import drive_android
        self.d = drive_android
        os.makedirs(self.d.OUT, exist_ok=True)
        self.d.adb("install", "-r", self.d.APK)

    def start(self):
        self.d.adb("shell", "pm", "clear", self.d.PACKAGE)
        self.d.adb("logcat", "-c")
        self.d.launch()

    def read(self):
        return timeline.parse(self.d.adb("logcat", "-d", "-s", "ProjectB").splitlines())

    def interact(self):
        d = self.d
        d.press("Settings")
        ended(self, "settings.open")
        d.key(4)  # Back.
        until(lambda: "settings" not in d.screen_parts())

        d.press("Show sidebar")
        d.tap(d.middle(d.see("shell", text="New Tab", kind="android.widget.Button")))
        ended(self, "tab.new")

    def stop(self):
        self.d.adb("shell", "am", "force-stop", self.d.PACKAGE)

    def describe(self):
        return {"machine": self.d.adb("shell", "getprop", "ro.product.model").strip()
                + ", Android " + self.d.adb("shell", "getprop", "ro.build.version.release").strip(), "session": "emulator"}


def until(test, timeout=20):
    deadline = time.time() + timeout
    while not test():
        if time.time() > deadline:
            raise TimeoutError("waited %d s" % timeout)
        time.sleep(0.05)


def ended(target, span):
    """Waits for the span the last click began to end: for one more end than was seen."""
    def count():
        return sum(1 for e in target.read() if e["kind"] == "end" and e["name"] == span)
    until(lambda: count() > target.ends.get(span, 0))
    target.ends[span] = count()


def one_start(target):
    target.ends = {}
    target.start()
    until(lambda: all(m in timeline.marks(target.read()) for m in STARTED), 60)
    time.sleep(0.5)  # Let the start settle before the first click.
    started = timeline.marks(target.read())
    done = max(started[m] for m in STARTED)
    target.interact()
    entries = target.read()
    target.stop()
    return {"startup": {name: t for name, t in started.items() if t <= done}, "spans": timeline.spans(entries), "entries": entries}


def gather(runs):
    """What every start had: each milestone's time since process start, and each span's
    length and stages since its begin."""
    names = set.intersection(*(set(r["startup"]) for r in runs))
    startup = {n: timeline.summary([r["startup"][n] for r in runs]) for n in names}
    spans = {}
    for name in sorted({s["name"] for r in runs for s in r["spans"]}):
        found = [s for r in runs for s in r["spans"] if s["name"] == name]
        stages = set.intersection(*(set(s["stages"]) for s in found))
        spans[name] = {"dur": timeline.summary([s["dur"] for s in found]),
                       "stages": {n: timeline.summary([s["stages"][n] for s in found]) for n in stages}}
    return {"startup": startup, "spans": spans}


def rows(result):
    """(section, name, summary) in the order things happen."""
    out = [("start-up, ms since process start", n, s) for n, s in sorted(result["startup"].items(), key=lambda kv: kv[1]["median"])]
    for name, span in result["spans"].items():
        title = "%s, ms since its begin (%d times)" % (name, span["dur"]["n"])
        out += [(title, n, s) for n, s in sorted(span["stages"].items(), key=lambda kv: kv[1]["median"])]
        out.append((title, "end", span["dur"]))
    return out


def show(result, baseline=None):
    old = {(sec.split(",")[0], n): s for sec, n, s in rows(baseline)} if baseline else {}
    section, before = None, 0
    for sec, name, s in rows(result):
        if sec != section:
            section, before = sec, 0
            print("\n%s" % sec)
            print("  %-32s %10s %10s %10s %10s%s" % ("", "median", "step", "min", "max", "   baseline   change" if baseline else ""))
        line = "  %-32s %10.3f %10.3f %10.3f %10.3f" % (name, s["median"], s["median"] - before, s["min"], s["max"])
        was = old.get((sec.split(",")[0], name))
        if was:
            change = s["median"] - was["median"]
            line += " %10.3f %+8.3f" % (was["median"], change) + (" (%+.0f%%)" % (100 * change / was["median"]) if was["median"] else "")
        print(line)
        before = s["median"]


def main():
    p = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    p.add_argument("-n", type=int, default=10, help="how many starts (default 10)")
    p.add_argument("--android", action="store_true")
    p.add_argument("--save", action="store_true", help="write the baseline")
    p.add_argument("--compare", nargs="?", const="", metavar="FILE", help="compare with the baseline, or with FILE")
    p.add_argument("--trace", metavar="FILE", help="write the last start as a Chrome trace")
    args = p.parse_args()

    target = Android() if args.android else Mac()
    baseline_path = os.path.join(ROOT, "tools/baseline", target.name + ".json")
    runs = []
    for i in range(args.n):
        runs.append(one_start(target))
        print("start %d: %s" % (i + 1, ", ".join("%s %.0f ms" % (m, runs[-1]["startup"][m] / 1e6) for m in STARTED)), flush=True)

    result = gather(runs)
    baseline = None
    if args.compare is not None:
        with open(args.compare or baseline_path) as f:
            baseline = json.load(f)
    show(result, baseline)

    if args.trace:
        with open(args.trace, "w") as f:
            json.dump(timeline.trace(runs[-1]["entries"]), f)
        print("\nthe last start's trace: " + args.trace)
    if args.save:
        with open(os.path.join(ROOT, "VERSION")) as f:
            version = f.read().strip()
        commit = subprocess.run(["git", "-C", ROOT, "rev-parse", "--short", "HEAD"], capture_output=True, text=True).stdout.strip()
        os.makedirs(os.path.dirname(baseline_path), exist_ok=True)
        with open(baseline_path, "w") as f:
            json.dump({"platform": target.name, "date": datetime.date.today().isoformat(), "version": version, "commit": commit,
                       "starts": args.n, **target.describe(), **result}, f, indent=1, sort_keys=True)
            f.write("\n")
        print("\nbaseline saved: " + baseline_path)


if __name__ == "__main__":
    main()
