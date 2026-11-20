#!/usr/bin/env python3
"""Checks that the real window on a Linux desktop shows what our driver says it shows: the
driver's composed view (the shell's surface over the page's) against a deskrig shot of the
window on screen. The headless suite only ever sees the composed view, so it passed while the
real window on GNOME Wayland had no shell (docs/research/linux.md, step 2). Run on the Mac:

    python3 tools/linux/check_real_window.py --target deskrig-ub --bundle <app.flatpak> [--commit <sha>]

The target VM must be running (deskrig vm start <name>, in a slot from the tester). It installs
the bundle, fetches this repository at --commit (default: HEAD, which must be pushed) into the
VM, runs real_window_guest.py there against the installed app's binary, takes the window's
shot, and compares the two at a small size. Results go to build/real-window/<target>/.
"""

import argparse
import base64
import io
import os
import subprocess
import sys
import time

from PIL import Image, ImageChops, ImageStat

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
APP_ID = "io.github.sudiptadeb.project-b"
REPO = "https://codeload.github.com/sudiptadeb/project-b/tar.gz/"
GUEST = "$HOME/pb-real-window"


def deskrig(target, *args, stdin=None, timeout=300):
    r = subprocess.run(["deskrig", "--target", target, *args], input=stdin, capture_output=True,
                       timeout=timeout)
    if r.returncode != 0:
        raise RuntimeError(f"deskrig {' '.join(args[:2])}: {r.stderr.decode(errors='replace')[-400:]}")
    return r.stdout


def run(target, script, timeout=300):
    return deskrig(target, "run", "--", "sh", "-c", script, timeout=timeout).decode(errors="replace")


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--target", required=True)
    p.add_argument("--bundle", required=True)
    p.add_argument("--commit", default=subprocess.run(["git", "-C", ROOT, "rev-parse", "HEAD"],
                                                      capture_output=True, text=True).stdout.strip())
    p.add_argument("--limit", type=float, default=18.0,
                   help="the largest mean difference, of 255, between the two at 160x90")
    a = p.parse_args()
    out = os.path.join(ROOT, "build", "real-window", a.target)
    os.makedirs(out, exist_ok=True)

    print(f"=== install {os.path.basename(a.bundle)} on {a.target}", flush=True)
    deskrig(a.target, "install", a.bundle, timeout=900)
    print(f"=== fetch {a.commit[:8]} into {GUEST}", flush=True)
    run(a.target, f"rm -rf {GUEST}/repo && mkdir -p {GUEST}/repo && "
                  f"python3 -c \"import urllib.request,sys; sys.stdout.buffer.write(urllib.request.urlopen('{REPO}{a.commit}').read())\" "
                  f"| tar -xz -C {GUEST}/repo --strip-components=1", timeout=600)
    binary = run(a.target, f"echo $(flatpak info --show-location {APP_ID})/files/project-b/project_b").strip()
    print(f"=== the app's binary: {binary}", flush=True)
    run(a.target, f"cd {GUEST}/repo && (PB_BIN={binary} PB_ARGS=--no-sandbox nohup python3 "
                  f"tools/linux/real_window_guest.py {GUEST}/out > {GUEST}/guest.log 2>&1 &)")

    deadline = time.time() + 120
    while time.time() < deadline:
        state = run(a.target, f"cat {GUEST}/out/ready >/dev/null 2>&1 && echo ready; cat {GUEST}/out/error 2>/dev/null").strip()
        if state:
            break
        time.sleep(2)
    if not state.startswith("ready"):
        print(run(a.target, f"tail -20 {GUEST}/guest.log"))
        print(f"FAIL the app reached its page and the driver's view -> {state or 'timed out'}", flush=True)
        return 1
    pid = run(a.target, f"cat {GUEST}/out/pid").strip()
    time.sleep(1)
    deskrig(a.target, "raise", pid)
    time.sleep(1)
    real_path = os.path.join(out, "real.png")
    deskrig(a.target, "shot", real_path, pid)
    composed = base64.b64decode(run(a.target, f"base64 -w0 {GUEST}/out/composed.png"))
    with open(os.path.join(out, "composed.png"), "wb") as f:
        f.write(composed)
    run(a.target, f"touch {GUEST}/out/done")

    small = (160, 90)
    real = Image.open(real_path).convert("L").resize(small)
    view = Image.open(io.BytesIO(composed)).convert("L").resize(small)
    diff = ImageStat.Stat(ImageChops.difference(real, view)).mean[0]
    spread = ImageStat.Stat(real).stddev[0]
    ImageChops.difference(real, view).save(os.path.join(out, "difference.png"))
    failures = 0
    for name, ok, detail in (
            ("the real window isn't blank", spread > 8, f"stddev {spread:.1f}"),
            ("the real window shows what the driver composes", diff <= a.limit, f"mean difference {diff:.1f} of 255, limit {a.limit}")):
        print(("ok   " if ok else "FAIL ") + name + ("" if ok else " -> " + detail), flush=True)
        failures += not ok
    print(f"=== {a.target}: mean difference {diff:.1f}, spread {spread:.1f}; shots in {out}", flush=True)
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
