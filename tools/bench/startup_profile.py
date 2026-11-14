#!/usr/bin/env python3
"""Where the Mac app's start-up time goes before the shell runs (docs/benchmarks.md).

Two views of the same start:

    python3 tools/bench/startup_profile.py trace   # Chromium's startup trace
    python3 tools/bench/startup_profile.py cpu     # CPU samples of the main thread, 3 starts

trace starts the app with --trace-startup and prints the main thread's tasks, the long
work on other threads, the child process launches, the shell's navigation from LoadURL to
commit, and every stretch of 3 ms or more where the main thread ran no Chromium task.
Its times are from the trace's start, which is a little after the process's: the shell's
navigation start is printed so it lines up with the timeline's shell.navigation_start.

cpu records each start with Instruments' Time Profiler (xcrun xctrace) and prints the main
thread in 20 ms buckets from the process's first sample, each with what it was doing, then
the engine calls Go made during Go's start. The package is stripped, so this one runs the
build tree's binary, which keeps its symbols: build/pb/project_b, the copy tools/build.sh makes, or PB_BIN.
Samples are of a running thread only, so a bucket's busy count below 20 is time the main
thread waited.

--keep DIR keeps the trace and the .trace recordings there (default: a temporary folder).
"""

import argparse
import collections
import json
import os
import pickle
import re
import shutil
import subprocess
import sys
import tempfile
import time
import xml.etree.ElementTree as ET

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PACKAGE = os.path.join(ROOT, "build/package/Project B.app/Contents/MacOS/project_b")
SYMBOLS = os.path.join(ROOT, "build/pb/project_b")


# --- trace -------------------------------------------------------------------------

def record_trace(binary, out):
    profile = tempfile.mkdtemp(prefix="pb-trace-")
    log = out + ".log"
    with open(log, "w") as f:
        p = subprocess.Popen([binary, f"--user-data-dir={profile}", "--trace-startup=*",
                              "--trace-startup-format=json", f"--trace-startup-file={out}",
                              "--trace-startup-duration=3"], stdout=f, stderr=subprocess.STDOUT)
    try:
        end = time.time() + 20
        while time.time() < end and "Completed startup tracing" not in open(log).read():
            time.sleep(0.25)
    finally:
        p.terminate()
        p.wait()
        shutil.rmtree(profile, ignore_errors=True)


def show_trace(path):
    ev = json.load(open(path))["traceEvents"]
    proc = {e["pid"]: e["args"]["name"] for e in ev if e.get("ph") == "M" and e["name"] == "process_name"}
    thread = {(e["pid"], e["tid"]): e["args"]["name"] for e in ev if e.get("ph") == "M" and e["name"] == "thread_name"}
    t0 = min(e["ts"] for e in ev if e.get("ts") and e.get("ph") != "M")
    ms = lambda e: (e["ts"] - t0) / 1000  # noqa: E731
    main = lambda e: proc.get(e["pid"]) == "Browser" and thread.get((e["pid"], e["tid"])) == "CrBrowserMain"  # noqa: E731
    ev = sorted((e for e in ev if e.get("ph") != "M" and "ts" in e), key=lambda e: e["ts"])

    print("Browser main thread, top-level slices of 2 ms or more")
    end = -1
    for e in ev:
        if e.get("ph") == "X" and main(e) and e["ts"] >= end:
            end = e["ts"] + e["dur"]
            if e["dur"] >= 2000:
                print(f"  {ms(e):7.1f} +{e['dur'] / 1000:6.1f}  {e['name']}")

    print("\nThe shell's navigation, the launches and the network service")
    marks = ("NavigationControllerImpl::LoadURLWithParams", "LoaderStartToReceiveResponse",
             "ThrottlingURLLoader::OnReceiveResponse", "RenderFrameHostImpl::DidCommitNavigation",
             "ChildProcessLauncher", "NetworkService::Initialize", "RenderThreadImpl::Create",
             "EnableAllExceptBestEffortQueues", "OnStartupComplete")
    for e in ev:
        a = e.get("args") or {}
        if ms(e) > 400 or not any(m == e["name"] or m == a.get("src_func") for m in marks):
            continue
        url = a.get("url") or a.get("URL") or ""
        print(f"  {ms(e):7.1f} {e.get('ph')} {proc.get(e['pid'], '?')[:12]:12} {a.get('src_func') or e['name']} {url}")

    print("\nMain thread stretches of 3 ms or more with no Chromium task")
    tasks = [e for e in ev if main(e) and e.get("ph") == "X" and e["name"] == "ThreadControllerImpl::RunTask"]
    for a, b in zip(tasks, tasks[1:]):
        gap = (b["ts"] - (a["ts"] + a["dur"])) / 1000
        if gap >= 3 and ms(b) < 400:
            print(f"  {ms(a) + a['dur'] / 1000:7.1f} to {ms(b):7.1f}: {gap:5.1f} ms")


# --- cpu ---------------------------------------------------------------------------

def record_cpu(binary, trace):
    profile = tempfile.mkdtemp(prefix="pb-cpu-")
    subprocess.run(["xcrun", "xctrace", "record", "--template", "Time Profiler", "--time-limit", "3s",
                    "--output", trace, "--launch", "--", binary, f"--user-data-dir={profile}"],
                   capture_output=True)
    subprocess.run(["pkill", "-f", f"user-data-dir={profile}"])
    shutil.rmtree(profile, ignore_errors=True)
    xml = trace + ".xml"
    subprocess.run(["xcrun", "xctrace", "export", "--input", trace, "--output", xml, "--xpath",
                    '/trace-toc/run[@number="1"]/data/table[@schema="time-profile"]'], capture_output=True)
    return samples(xml)


def samples(xml):
    """(ms, thread, frames leaf first) for every sample, resolving the export's id/ref pairs."""
    ids = {}

    def resolve(el):
        if el.get("ref"):
            return ids[el.get("ref")]
        if el.get("id"):
            ids[el.get("id")] = el
        for c in el:
            resolve(c)
        return el

    out = []
    for row in ET.parse(xml).getroot().iter("row"):
        t = th = None
        frames = []
        for c in row:
            c = resolve(c)
            if c.tag == "sample-time":
                t = int(c.text) / 1e6
            elif c.tag == "thread":
                th = c.get("fmt")
            elif c.tag in ("tagged-backtrace", "backtrace"):
                bt = c if c.tag == "backtrace" else next(iter(c), None)
                for f in (bt.iter("frame") if bt is not None else []):
                    f = ids.get(f.get("ref")) if f.get("ref") else f
                    frames.append(f.get("name") or "?")
        out.append((t, th or "", frames))
    return out


LABELS = [("Window::Window", "new NSWindow"), ("Window::Show", "show window"), ("ShellPage", "shell page"),
          ("CreateWindow", "window, other"), ("SetDockColor", "Dock colour"), ("SetMainMenu", "main menu"),
          ("Engine::", "other engine call"), ("asmcgocall", "Go calling C"), ("projectb/", "Go"),
          ("runtime.", "Go runtime"), ("InitMacApp", "NSApplication init"),
          ("finishLaunching", "NSApp finishLaunching"), ("CA::Transaction::commit", "Core Animation commit"),
          ("RunMainMessageLoop", "message loop"), ("PreSandboxStartup", "resources"),
          ("BrowserMainRunnerImpl::Initialize", "Chromium init"),
          ("ContentMainRunnerImpl::Initialize", "content init"), ("ProjectBMain", "content main")]


def label(frames):
    joined = " ".join(frames)
    return next((name for key, name in LABELS if key in joined), "process load (dyld)")


def show_cpu(runs):
    for i, s in enumerate(runs, 1):
        main = [x for x in s if x[1].startswith("Main Thread")]
        t0 = main[0][0]
        buckets = collections.defaultdict(collections.Counter)
        for t, _, fr in main:
            if t - t0 < 320:
                buckets[int((t - t0) // 20)][label(fr)] += 1
        print(f"start {i}: the main thread, busy samples out of 20 per 20 ms")
        for b in range(16):
            c = buckets[b]
            print(f"  {b * 20:3}-{b * 20 + 20:3} ms  {sum(c.values()):2}  " + ", ".join(f"{k} {v}" for k, v in c.most_common(3)))
    calls = collections.Counter()
    for s in runs:
        main = [x for x in s if x[1].startswith("Main Thread")]
        t0 = main[0][0]
        for t, _, fr in main:
            if t - t0 > 300 or not any("asmcgocall" in f for f in fr):
                continue
            i = next(i for i, f in enumerate(fr) if "asmcgocall" in f)
            c = [re.sub(r"\(.*", "", f)[:60] for f in fr[:i] if f.startswith(("project_b::", "content::", "-["))]
            calls[" > ".join(list(reversed(c))[:3])] += 1
    print("\nGo's calls into the engine in the first 300 ms, ms of CPU per start")
    for k, v in calls.most_common(15):
        print(f"  {v / len(runs):5.1f}  {k}")


def main():
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("view", choices=["trace", "cpu"])
    p.add_argument("--keep", help="folder to keep the recordings in")
    a = p.parse_args()
    keep = a.keep or tempfile.mkdtemp(prefix="pb-startup-")
    os.makedirs(keep, exist_ok=True)
    if a.view == "trace":
        out = os.path.join(keep, "startup-trace.json")
        record_trace(os.environ.get("PB_BIN", PACKAGE), out)
        show_trace(out)
    else:
        binary = os.environ.get("PB_BIN", SYMBOLS)
        runs = []
        for i in range(3):
            path = os.path.join(keep, f"start{i + 1}.trace")
            shutil.rmtree(path, ignore_errors=True)
            runs.append(record_cpu(binary, path))
        with open(os.path.join(keep, "samples.pickle"), "wb") as f:
            pickle.dump(runs, f)
        show_cpu(runs)
    print(f"\nrecordings in {keep}")


if __name__ == "__main__":
    main()
