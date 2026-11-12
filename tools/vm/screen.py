"""The VM's screen for check.py and prepare.py: frames in, real mouse and keys out, through
deskrig (github.com/sudiptadeb/deskrig, a copy of its Python client is deskrig.py here).

deskrig sends the input inside the VM, through Tart's guest agent, and keeps the VM's latest
frame in a mapped file from its window on this Mac, so a picture is never older than the screen
(docs/vm-testing.md, Pictures). Coordinates are points; frames are pixels.
"""

import os
import sys
import time

VM_HOME = os.path.expanduser("~/PW_TOOLS/macos-vm")
VM = os.environ.get("PB_VM", "pb-test")  # vars.sh says which VM, and the deskrig target is named after it.


def refuse(message):
    """Stops before anything ran, with exit 2: something this needs is missing."""
    print(message, file=sys.stderr)
    sys.exit(2)


try:
    import numpy as np
    from PIL import Image
except ImportError as e:
    refuse(f"{e}: this Python, {sys.executable}, lacks what the VM tools need. Run them with the VM's own, "
           f"{VM_HOME}/venv/bin/python (tools/test.sh vm does). tools/vm/setup.sh makes it.")

import deskrig


class Screen:
    def __init__(self):
        self.target = deskrig.Target(VM)
        try:
            self.frames = self.target.frames()
        except Exception as e:
            refuse(f"no frames from the VM {VM} ({e}): run tools/vm/start.sh. deskrig needs Screen Recording "
                   "on this Mac for them (deskrig setup permissions).")
        w, h = self.frames.read()[:2]
        self.width, self.height = w, h
        self.scale = w / self.frames.points[0]

    def image(self):
        """The VM's screen now, as an array of RGB, row then column, in pixels."""
        return self.frames.array()

    def wait_frame(self, timeout=20):
        """Until the VM's screen changes, or timeout seconds."""
        self.frames.wait(timeout)

    def shot(self, path):
        Image.fromarray(self.image()).save(path)
        return path

    def move(self, x, y, pause=0.08):
        self.target.move(x, y)
        time.sleep(pause)

    def press(self):
        self.target.down()

    def release(self):
        self.target.up()

    def click(self, x, y):
        # A person's pointer arrives before it clicks, and hover can change what's under it.
        self.move(x, y, 0.25)
        self.target.click(x, y)

    def dbl(self, x, y):
        self.move(x, y, 0.25)
        self.target.dblclick(x, y)

    def drag(self, x1, y1, x2, y2):
        self.target.drag(x1, y1, x2, y2)

    def key(self, combo):
        """A combination such as cmd+t or ctrl+cmd+f."""
        self.target.key(combo)

    def type(self, text):
        self.target.type(text)

    def dismiss_notifications(self):
        self.target.run("dismiss-notifications", capture_output=True)

    def close(self):
        self.target.close()
