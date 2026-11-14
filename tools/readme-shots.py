#!/usr/bin/env python3
"""Refresh the README's screenshots in docs/screenshots/ from the last test runs.

The standard shell's shots come from the app driver's run (build/real/, tools/test.sh app).
The other shells' come from the VM's shells check (~/PW_TOOLS/macos-vm/shots/latest-pb-test,
tools/vm/run.sh shells), cut down to the window so no desktop shows. Look at each one before
committing: a shot from a failed run is not one to show.
"""
import os
import sys

from PIL import Image

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
REAL = os.path.join(ROOT, "build", "real")
VM = os.path.expanduser("~/PW_TOOLS/macos-vm/shots/latest-pb-test")
OUT = os.path.join(ROOT, "docs", "screenshots")
WINDOW = (80, 60, 1360, 780)  # where the VM checks place the window on its 1440x900 screen

SHOTS = [
    (REAL, "5-tabs.png", "standard.png", None),
    (REAL, "3-suggestions.png", "suggestions.png", None),
    (REAL, "7-horizontal.png", "horizontal.png", None),
    (REAL, "settings-general.png", "settings.png", None),
    (VM, "terminal-split.png", "terminal.png", WINDOW),
    (VM, "fox-menu.png", "fox.png", WINDOW),
    (VM, "minimal-svelte.png", "minimal-svelte.png", WINDOW),
]

missing = [os.path.join(d, n) for d, n, _, _ in SHOTS if not os.path.exists(os.path.join(d, n))]
if missing:
    sys.exit("missing, run the checks first: " + ", ".join(missing))
os.makedirs(OUT, exist_ok=True)
for d, name, dest, box in SHOTS:
    img = Image.open(os.path.join(d, name)).convert("RGB")
    if box:
        img = img.crop(box)
    if img.width > 1280:
        img = img.resize((1280, round(img.height * 1280 / img.width)), Image.LANCZOS)
    img.save(os.path.join(OUT, dest), optimize=True)
    print(dest, img.size)
