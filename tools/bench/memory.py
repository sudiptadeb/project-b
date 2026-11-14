#!/usr/bin/env python3
"""Memory of the Mac app per shell, once start-up has settled (docs/benchmarks.md).

Starts the app on each shell given, waits five seconds, and reads macOS's footprint, the
number Activity Monitor shows, of the browser and each child process by its kind, and the
shell page's JS heap and element count. The first start of each shell is a warm-up; the
rest give the median.

    python3 tools/bench/memory.py standard minimal-svelte minimal-js terminal fox
    python3 tools/bench/memory.py -n 5 --out build/bench/memory.json standard

PB_BIN picks the binary, as for tools/drive_first_app.py; point it at the package
(build/package/Project B.app/Contents/MacOS/project_b) for the numbers a user gets.
"""

import argparse
import collections
import json
import os
import re
import statistics
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
import drive_first_app as d  # noqa: E402

SETTLE = 5


def footprint(pid):
    """A process's footprint in MiB."""
    out = subprocess.run(["footprint", "-f", "bytes", "--noCategories", "-p", str(pid)],
                         capture_output=True, text=True).stdout
    m = re.search(r"Footprint:\s*(\d+)", out)
    return int(m.group(1)) / 2**20 if m else 0


def kind(pid):
    """A child process's kind from its command line: renderer, gpu-process, utility <sub-type>."""
    args = subprocess.run(["ps", "-o", "args=", "-p", str(pid)], capture_output=True, text=True).stdout
    m = re.search(r"--type=(\S+)", args)
    if not m:
        return "browser"
    if m.group(1) == "utility":
        sub = re.search(r"--utility-sub-type=(\S+)", args)
        return "utility " + (sub.group(1).split(".")[-1] if sub else "")
    return m.group(1)


def measure(shell):
    profile = tempfile.mkdtemp(prefix="pb-memory-")
    with open(os.path.join(profile, "prefs.json"), "w") as f:
        json.dump({"shell": shell}, f)
    app = d.App(profile)
    try:
        time.sleep(SETTLE)
        assert app.js("location.pathname") == f"/{shell}/", app.js("location.href")
        row = collections.Counter({"browser": footprint(app.proc.pid)})
        children = subprocess.run(["pgrep", "-P", str(app.proc.pid)], capture_output=True, text=True).stdout.split()
        for pid in children:
            row[kind(pid)] += footprint(pid)
            if kind(pid) == "renderer":
                row["renderers"] += 1
        row["shell JS heap"] = app.js("performance.memory.usedJSHeapSize") / 2**20
        row["shell elements"] = app.js("document.getElementsByTagName('*').length")
        return row
    finally:
        app.close()


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("shells", nargs="+")
    p.add_argument("-n", type=int, default=3, help="measured starts per shell, after one warm-up")
    p.add_argument("--out", help="also write the medians as JSON")
    a = p.parse_args()
    os.makedirs(d.OUT, exist_ok=True)
    res = {}
    for shell in a.shells:
        measure(shell)
        runs = [measure(shell) for _ in range(a.n)]
        res[shell] = {k: statistics.median(r[k] for r in runs) for k in runs[0]}
    if a.out:
        os.makedirs(os.path.dirname(a.out) or ".", exist_ok=True)
        with open(a.out, "w") as f:
            json.dump(res, f, indent=1)
    keys = sorted({k for r in res.values() for k in r}, key=lambda k: (k.startswith("shell") or k == "renderers", k))
    print(f"{'MiB, median of %d' % a.n:22}" + "".join(f"{s:>15}" for s in res))
    print(f"{'total footprint':22}" + "".join(
        f"{sum(v for k, v in res[s].items() if not k.startswith('shell') and k != 'renderers'):15.1f}" for s in res))
    for k in keys:
        print(f"{k:22}" + "".join(f"{res[s].get(k, 0):15.1f}" for s in res))


if __name__ == "__main__":
    main()
