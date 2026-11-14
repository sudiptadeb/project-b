#!/usr/bin/env python3
"""The Mac app's start-up marks, in ms from process start, for a reused and a new profile
(docs/benchmarks.md).

A warm start reuses one profile, as a person's every day start does. A fresh start gets a
new, empty profile each time, so it takes the first-start path: the Personal template and
its prefs. The first start of each kind is a warm-up for the OS's caches; the rest give
the median. The marks come from the app's own timeline (docs/logging.md).

    python3 tools/bench/first_paint.py
    python3 tools/bench/first_paint.py -n 10 --out build/bench/first-paint.json

PB_BIN picks the binary, as for tools/drive_first_app.py; point it at the package
(build/package/Project B.app/Contents/MacOS/project_b) for the numbers a user gets. Run it
from a person's desktop session where you can (deskrig run -- ...): AppKit can be slower in
an agent's session.
"""

import argparse
import json
import os
import statistics
import sys
import tempfile
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
import drive_first_app as d  # noqa: E402
import timeline  # noqa: E402

MARKS = ["main.entered", "content.ready", "go.started", "shell.committed", "shell.ready",
         "shell.first_paint", "shell.on_screen", "app.ready", "tab.committed"]


def start(profile):
    """Starts the app on profile once, and returns its marks in ms once the shell is on
    screen and the first tab has committed."""
    app = d.App(profile)
    end = time.time() + 20
    while time.time() < end:
        marks = timeline.marks(json.loads(app.cmd("timeline", 0))["result"]["entries"])
        if {"shell.on_screen", "tab.committed", "app.ready"} <= set(marks):
            break
        time.sleep(0.1)
    app.close()
    return {k: marks[k] / 1e6 for k in MARKS if k in marks}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("-n", type=int, default=10, help="starts of each kind, after one warm-up")
    parser.add_argument("--out", help="write every start's marks here as JSON")
    args = parser.parse_args()
    os.makedirs(d.OUT, exist_ok=True)

    kept = tempfile.mkdtemp(prefix="pb-warm-")
    runs = {"warm": [], "fresh": []}
    for i in range(args.n + 1):
        warm = start(kept)
        fresh = start(tempfile.mkdtemp(prefix="pb-fresh-"))
        if i:
            runs["warm"].append(warm)
            runs["fresh"].append(fresh)
    if args.out:
        with open(args.out, "w") as f:
            json.dump(runs, f, indent=1)
    print(f"{'ms from process start, median of %d' % args.n:40}{'warm':>8}{'fresh':>8}")
    for k in MARKS:
        cells = ""
        for kind in ("warm", "fresh"):
            seen = [r[k] for r in runs[kind] if k in r]
            cells += f"{statistics.median(seen):8.0f}" if seen else f"{'-':>8}"
        print(f"{k:40}{cells}")


if __name__ == "__main__":
    main()
