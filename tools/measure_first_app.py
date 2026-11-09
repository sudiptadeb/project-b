#!/usr/bin/env python3
"""Numbers for the first app: startup, memory, boundary cost, overlay cost."""

import importlib.util
import json
import os
import re
import subprocess
import tempfile
import time

spec = importlib.util.spec_from_file_location(
    "drive", os.path.join(os.path.dirname(os.path.realpath(__file__)), "drive_first_app.py"))
drive = importlib.util.module_from_spec(spec)
spec.loader.exec_module(drive)
os.makedirs(drive.OUT, exist_ok=True)

ANIMATED = ("data:text/html,<body style='margin:0'><canvas id=c width=1100 height=650></canvas><script>"
            "const x=c.getContext('2d');let i=0;(function f(){x.fillStyle='hsl('+(i++%25360)+',70%25,50%25)';"
            "x.fillRect(0,0,1100,650);requestAnimationFrame(f)})()</script>")


def cpu_seconds(pids):
    out = subprocess.run(["ps", "-o", "pid=,time=", "-p", ",".join(map(str, pids))], capture_output=True, text=True).stdout
    result = {}
    for line in out.splitlines():
        pid, t = line.split()
        m = re.match(r"(?:(\d+):)?(\d+):(\d+\.\d+)", t)
        result[int(pid)] = int(m.group(1) or 0) * 3600 + int(m.group(2)) * 60 + float(m.group(3))
    return result


def main():
    print("== first paint, warm, milliseconds after process start")
    profile = tempfile.mkdtemp(prefix="pb-measure-")
    samples = []
    for _ in range(6):
        app = drive.App(profile)
        app.wait("!!document.querySelector('input')", 30)
        time.sleep(0.2)
        samples.append(json.loads(app.cmd("stats", timeout=60))["firstPaintMs"])
        app.close()
    print(samples, "median", sorted(samples)[len(samples) // 2], flush=True)

    app = drive.App(profile)
    app.wait("!!document.querySelector('input')", 30)
    drive.visit(app, "https://example.com/")
    time.sleep(1)

    print("== processes")
    pids = json.loads(app.cmd("pids"))
    table = drive.process_table(app.proc.pid)
    for row in table:
        role = "shell" if row["pid"] == pids["shell"] else "page" if row["pid"] == pids["page"] else ""
        print(json.dumps({**row, "role": role}), flush=True)
        if row["type"] == "renderer" or row["type"] == "gpu-process":
            command = subprocess.run(["ps", "-o", "command=", "-p", str(row["pid"])], capture_output=True, text=True).stdout
            print("  seatbelt client:", "--seatbelt-client" in command)

    print("== Go on the main thread")
    print("cgo call", json.loads(app.cmd("stats", timeout=60))["cgoCallNs"], "ns")
    gc = sorted(float(app.cmd("gc")) for _ in range(20))
    print("forced full GC, main thread blocked for: median", gc[10], "us, max", gc[-1], "us")
    print(app.cmd("stats", timeout=60))

    print("== CPU seconds over 10s with an animated page, shell shown and hidden")
    drive.focus_bar(app)
    app.js(f"(()=>{{const i={drive.INPUT};i.value={json.dumps(ANIMATED)};i.dispatchEvent(new Event('input'));return 1}})()")
    app.cmd("key", "enter")
    time.sleep(2)
    print("page rAF check", app.js("(()=>{window.__r=0;const f=()=>{window.__r++;requestAnimationFrame(f)};requestAnimationFrame(f);return 1})()", page=True))
    time.sleep(1)
    print("page frames in a second", app.js("window.__r", page=True))
    table = drive.process_table(app.proc.pid)
    watch = [r["pid"] for r in table]
    names = {r["pid"]: r["type"] for r in table}
    for hidden in ("0", "1", "0", "1"):
        app.cmd("shellhidden", hidden)
        time.sleep(1)
        before = cpu_seconds(watch)
        time.sleep(10)
        after = cpu_seconds(watch)
        print("shell hidden" if hidden == "1" else "shell shown", {names[p]: round(after[p] - before[p], 2) for p in watch if p in after}, flush=True)
    app.cmd("shellhidden", "0")
    app.close()

    print("== a C++ crash on the main thread, with Go's signal handlers present")
    app = drive.App(tempfile.mkdtemp(prefix="pb-segv-"))
    app.wait("!!document.querySelector('input')", 30)
    log_start = os.path.getsize(os.path.join(drive.OUT, "app.log"))
    app.writer.write("x segv\n")
    app.writer.flush()
    try:
        app.proc.wait(timeout=20)
    except subprocess.TimeoutExpired:
        app.proc.kill()
    time.sleep(0.5)
    print("exit code", app.proc.returncode)
    with open(os.path.join(drive.OUT, "app.log")) as f:
        f.seek(log_start)
        text = f.read()
    print(text[:1500])


if __name__ == "__main__":
    main()
