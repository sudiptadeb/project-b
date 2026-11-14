#!/usr/bin/env python3
"""The floor: a bare native Mac app's start-up and memory (docs/benchmarks.md).

Builds tools/bench/native/bare.m into build/bench/bare, then for the plain AppKit window
and the window with a WKWebView prints the median of each start-up mark, in ms since the
process started, as the app's timeline counts, and the footprint of the app and, for the
web one, of WebKit's own WebContent, GPU and Networking processes.

    python3 tools/bench/native.py
    python3 tools/bench/native.py -n 11
"""

import argparse
import collections
import os
import re
import statistics
import subprocess
import time

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
SOURCE = os.path.join(ROOT, "tools/bench/native/bare.m")
BINARY = os.path.join(ROOT, "build/bench/bare")
WEBKIT = ("com.apple.WebKit.WebContent", "com.apple.WebKit.GPU", "com.apple.WebKit.Networking")
SETTLE = 5


def footprint(pid):
    out = subprocess.run(["footprint", "-f", "bytes", "--noCategories", "-p", str(pid)],
                         capture_output=True, text=True).stdout
    m = re.search(r"Footprint:\s*(\d+)", out)
    return int(m.group(1)) / 2**20 if m else 0


def timings(mode, n):
    marks, order = collections.defaultdict(list), []
    for i in range(n + 1):
        out = subprocess.run([BINARY, mode], capture_output=True, text=True, timeout=30).stdout
        if i == 0:
            continue  # the first start of a new binary is cold
        for line in out.splitlines():
            t, name = line.split()
            marks[name].append(float(t))
            if name not in order:
                order.append(name)
    return [(name, statistics.median(marks[name])) for name in order]


def memory(mode):
    p = subprocess.Popen([BINARY, mode], stdout=subprocess.DEVNULL, env={**os.environ, "KEEP": "1"})
    try:
        time.sleep(SETTLE)
        row = {"app": footprint(p.pid)}
        if mode == "web":
            for name in WEBKIT:
                # WebKit's processes are XPC services, not children: the newest is this app's.
                pid = subprocess.run(["pgrep", "-n", "-f", name], capture_output=True, text=True).stdout.strip()
                row[name.split(".")[-1]] = footprint(pid) if pid else 0
        return row
    finally:
        p.terminate()
        p.wait()


def main():
    a = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    a.add_argument("-n", type=int, default=7, help="measured starts per mode, after one warm-up")
    n = a.parse_args().n
    os.makedirs(os.path.dirname(BINARY), exist_ok=True)
    subprocess.run(["clang", "-O2", "-fobjc-arc", "-framework", "Cocoa", "-framework", "WebKit",
                    "-framework", "QuartzCore", SOURCE, "-o", BINARY], check=True)
    for mode, name in (("plain", "Bare AppKit"), ("web", "AppKit with a WKWebView")):
        print(f"{name}: ms since the process started, median of {n}")
        for mark, t in timings(mode, n):
            print(f"  {mark:28} {t:6.0f}")
        row = memory(mode)
        print(f"  footprint, MiB: total {sum(row.values()):.1f} (" + ", ".join(f"{k} {v:.1f}" for k, v in row.items()) + ")")


if __name__ == "__main__":
    main()
