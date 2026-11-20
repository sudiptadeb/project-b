#!/usr/bin/env python3
"""Checks Linux's one binary (docs/research/linux.md, 3), headless, inside the run container
(tools/linux/headless.sh --script tools/linux/check_one_binary.py):

1. Only the browser process maps the Go core: no child does, the zygotes, renderers, the GPU
   process and the utilities alike, by each one's /proc/<pid>/maps.
2. A process started with --type never loads the Go core: the dynamic loader lists what it
   opens (LD_DEBUG), and libcore.so isn't among it.
3. Go's crash handling still works in the browser process: a Go panic and a C++ fault each
   leave crash.log, and the next start says the last run crashed; the panic's log has Go's
   stacks.
4. A missing libcore.so ends the start with a message naming it, not a crash.
"""

import glob
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import drive_first_app as d  # noqa: E402

BIN = d.BIN
CORE = os.path.join(os.path.dirname(BIN), "libcore.so")


def maps(pid):
    try:
        with open(f"/proc/{pid}/maps") as f:
            return f.read()
    except OSError:
        return ""


def threads(pid):
    names = set()
    for t in glob.glob(f"/proc/{pid}/task/*/comm"):
        try:
            with open(t) as f:
                names.add(f.read().strip())
        except OSError:
            pass
    return names


def kind(pid):
    """What a process is. One forked from a zygote keeps the zygote's command line, so a
    renderer and the GPU process are known by their threads: a renderer's Compositor, the GPU
    process's VizCompositorThread."""
    try:
        with open(f"/proc/{pid}/cmdline") as f:
            args = f.read().split("\0")
    except OSError:
        return "?"
    names = threads(pid)
    if "VizCompositorTh" in names:
        return "gpu-process"
    if "Compositor" in names:
        return "renderer"
    for a in args:
        if a.startswith("--type="):
            sub = next((x.split("=", 1)[1] for x in args if x.startswith("--utility-sub-type=")), "")
            return a.split("=", 1)[1] + (f" ({sub})" if sub else "")
    return "main"


def descendants(pid):
    out, todo = [], [pid]
    while todo:
        p = todo.pop()
        try:
            with open(f"/proc/{p}/task/{p}/children") as f:
                kids = [int(k) for k in f.read().split()]
        except OSError:
            kids = []
        out += kids
        todo += kids
    return out


def children_check():
    app = d.App(tempfile.mkdtemp(prefix="pb-profile-"))
    d.from_box(app, d.PAGES)
    time.sleep(1)
    rows = []
    for pid in descendants(app.proc.pid):
        m = maps(pid)
        if m:
            rows.append((pid, kind(pid), "/libcore.so" in m))
    for r in rows:
        print("process", r, flush=True)
    d.check("the browser process maps the Go core", "/libcore.so" in maps(app.proc.pid))
    kinds = {r[1].split(" ")[0] for r in rows}
    # In the container, with --no-sandbox, the network service runs in the browser process,
    # so a utility process shows only when one is running.
    d.check("the children include the zygotes, a renderer and the GPU process",
            {"zygote", "renderer", "gpu-process"} <= kinds, sorted(kinds))
    d.check("no child maps the Go core", rows and not any(r[2] for r in rows), [r for r in rows if r[2]])
    app.close()


def type_check():
    env = dict(os.environ, LD_DEBUG="files")
    try:
        r = subprocess.run([BIN, "--type=utility", "--no-sandbox"], env=env, capture_output=True, text=True, timeout=15)
        out = r.stdout + r.stderr
    except subprocess.TimeoutExpired as e:
        out = (e.stdout or b"").decode(errors="replace") + (e.stderr or b"").decode(errors="replace")
    opened = [line for line in out.splitlines() if "file=" in line and "libcore" in line]
    d.check("a --type process never loads the Go core", "file=" in out and not opened, opened[:3])


def missing_check():
    folder = tempfile.mkdtemp(prefix="pb-nocore-")
    shutil.copy(BIN, folder)
    r = subprocess.run([os.path.join(folder, "project_b"), "--no-sandbox"], capture_output=True, text=True, timeout=30)
    d.check("a missing libcore.so ends the start with a message, not a crash",
            r.returncode == 1 and "libcore.so" in r.stderr and "can't start" in r.stderr, (r.returncode, r.stderr[-300:]))
    shutil.rmtree(folder, ignore_errors=True)


def crash_check(name, command):
    profile = tempfile.mkdtemp(prefix="pb-profile-")
    app = d.App(profile)
    app.send(*command)
    try:
        code = app.proc.wait(timeout=20)
    except subprocess.TimeoutExpired:
        code = None
    d.App.running.remove(app)
    log = os.path.join(profile, "crash.log")
    text = open(log, errors="replace").read() if os.path.exists(log) else ""
    d.check(f"{name} ends the app, leaving crash.log", code not in (None, 0) and os.path.exists(log), (code, text[:200]))
    if command[0] == "gocrash":
        d.check(f"{name}: crash.log has Go's stacks", "goroutine" in text, text[:300])
    again = d.App(profile)
    shown = d.holds(again, "document.body.innerText.includes('crashed last time')", 15)
    kept = [f for f in os.listdir(profile) if f.startswith("crash-") and f.endswith(".log")]
    d.check(f"{name}: the next start says the last run crashed, and keeps its log", shown and kept, (shown, kept))
    again.close()


def main():
    os.makedirs(d.OUT, exist_ok=True)
    for part in (children_check, type_check, missing_check):
        try:
            part()
        except Exception as e:  # noqa: BLE001
            d.check(f"{part.__name__} ran to its end", False, e)
    for name, command in (("a Go panic", ("gocrash", "1")), ("a C++ fault on the main thread", ("segv",))):
        try:
            crash_check(name, command)
        except Exception as e:  # noqa: BLE001
            d.check(f"{name} check ran to its end", False, e)
    for app in list(d.App.running):
        app.close()
    print(f"{d.failures} FAILED" if d.failures else "all passed", flush=True)
    sys.exit(1 if d.failures else 0)


if __name__ == "__main__":
    main()
