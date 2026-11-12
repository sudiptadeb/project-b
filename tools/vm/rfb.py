#!/usr/bin/env python3
"""The VM's desktop: real frames in, real mouse and keys out.

Input goes through a small VNC client to macOS's own Screen Sharing inside the VM, on the
VM's private address, so nothing listens on this Mac's network. The frames come from the VM's
window on this Mac, which `desk watch` keeps in $FRAME_FILE (start.sh): Screen Sharing sends
its pictures up to seconds late, while the window shows the VM's screen as it is
(docs/vm-testing.md). Coordinates are points; shots are pixels.

As a tool, one action per run:
    rfb.py shot out.png | click X Y | dbl X Y | move X Y | drag X1 Y1 X2 Y2 | key cmd+t | type "text"

Run it with the VM's own Python, ~/PW_TOOLS/macos-vm/venv/bin/python, which setup.sh makes
with numpy, pillow and cryptography. The VM's address comes from the file start.sh writes.
"""

import hashlib
import mmap
import os
import socket
import struct
import sys
import threading
import time
from urllib.parse import urlparse

VM_HOME = os.path.expanduser("~/PW_TOOLS/macos-vm")
VM = os.environ.get("PB_VM", "pb-test")  # vars.sh says which VM; start.sh writes its address.
URL_FILE = f"{VM_HOME}/vnc-url-{VM}"
FRAME_FILE = f"{VM_HOME}/frame-{VM}.bin"


def refuse(message):
    """Stops before anything ran, with exit 2: something this needs is missing."""
    print(message, file=sys.stderr)
    sys.exit(2)


try:
    import numpy as np
    from cryptography.hazmat.decrepit.ciphers.algorithms import TripleDES
    from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
    from PIL import Image
except ImportError as e:
    refuse(f"{e}: this Python, {sys.executable}, lacks what the VM tools need. Run them with the VM's own, "
           f"{VM_HOME}/venv/bin/python (tools/test.sh vm does). tools/vm/setup.sh makes it.")

POINTS_WIDE = 1440  # The VM's display, from DISPLAY_SIZE in vars.sh.
INPUT_SETTLE = 1.0  # Screen Sharing drops input for about half a second after a connection opens.

KEYS = {
    "cmd": 0xffeb, "opt": 0xffe9, "ctrl": 0xffe3, "shift": 0xffe1,
    "return": 0xff0d, "esc": 0xff1b, "tab": 0xff09, "delete": 0xff08, "space": 0x20,
    "left": 0xff51, "up": 0xff52, "right": 0xff53, "down": 0xff54,
}
SHIFTED = '~!@#$%^&*()_+{}|:"<>?'


def keysym(name):
    return KEYS[name] if name in KEYS else ord(name)


class Window:
    """The VM's window on this Mac, as `desk watch` keeps it: a header, then BGRA rows
    (tools/deskctl/deskctl.swift). The window's title bar sits above the VM's screen."""

    def __init__(self, width, height):
        if not os.path.exists(FRAME_FILE):
            refuse(f"{FRAME_FILE} is missing: it holds the VM's latest frame. Run tools/vm/start.sh, "
                   "which starts the watch on the VM's window once its desktop is up.")
        self.width, self.height = width, height  # The VM's screen in pixels, as Screen Sharing has it.
        self.file = open(FRAME_FILE, "rb")
        self.map = None

    def _header(self):
        if self.map is None or self.map.size() != os.fstat(self.file.fileno()).st_size:
            self.map = mmap.mmap(self.file.fileno(), 0, prot=mmap.PROT_READ)
        return struct.unpack_from("<8sQIII4xd", self.map, 0)

    def count(self):
        return self._header()[1]

    def image(self):
        while True:
            magic, count, w, h, stride, _ = self._header()
            if magic != b"DESKWAT1":
                raise RuntimeError(f"{FRAME_FILE} isn't a desk watch frame")
            if count % 2 == 0:
                rows = np.frombuffer(self.map, np.uint8, h * stride, 40).reshape(h, stride)[:, :w * 4]
                img = rows.reshape(h, w, 4)[:, :, 2::-1].copy()
                if self._header()[1] == count:
                    break
            time.sleep(0.001)
        # The VM's screen is the bottom of the window, below the title bar, at the window's scale.
        screen = img[h - round(self.height * w / self.width):]
        if w != self.width:
            screen = np.asarray(Image.fromarray(screen).resize((self.width, self.height), Image.BILINEAR))
        return screen

    def wait(self, timeout):
        seen, end = self.count(), time.time() + timeout
        while self.count() == seen and time.time() < end:
            time.sleep(0.002)


class Vnc:
    def __init__(self):
        if not os.path.exists(URL_FILE):
            refuse(f"{URL_FILE} is missing: it holds the VM's Screen Sharing address. Run tools/vm/start.sh, "
                   "which writes it once the VM's desktop is up and leaves a VM that already runs alone.")
        url = urlparse(open(URL_FILE).read().strip())
        self.sock = socket.create_connection((url.hostname, url.port))
        self.sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
        self.file = self.sock.makefile("rb", buffering=1 << 20)
        self._login(url.username or "", url.password or "")
        self.cond = threading.Condition()
        self.frames = 0
        self.closed = False
        self.buttons = 0
        self.x = self.y = 0
        self.sock.sendall(struct.pack(">BxxxBBBBHHHBBBxxx", 0, 32, 24, 0, 1, 255, 255, 255, 0, 8, 16))
        self.sock.sendall(struct.pack(">BxHii", 2, 2, 0, -223))
        self._request(False)
        threading.Thread(target=self._read, daemon=True).start()
        with self.cond:
            self.cond.wait_for(lambda: self.frames or self.closed, 20)
        self.window = Window(self.width, self.height)
        time.sleep(INPUT_SETTLE)

    def _login(self, user, password):
        self.file.read(12)
        self.sock.sendall(b"RFB 003.008\n")
        kinds = self.file.read(self.file.read(1)[0])
        if 30 in kinds:
            self.sock.sendall(bytes([30]))
            self._apple_login(user, password)
        elif 2 in kinds:
            self.sock.sendall(bytes([2]))
            self._vnc_login(password)
        else:
            raise RuntimeError(f"no login type we know in {list(kinds)}")
        if struct.unpack(">I", self.file.read(4))[0] != 0:
            raise RuntimeError("VNC login failed")
        self.sock.sendall(bytes([1]))
        self.width, self.height = struct.unpack(">HH", self.file.read(4))
        self.file.read(16)
        self.file.read(struct.unpack(">I", self.file.read(4))[0])
        self.fb = np.zeros((self.height, self.width, 4), dtype=np.uint8)
        self.scale = self.width / POINTS_WIDE

    def _apple_login(self, user, password):
        # Apple's type 30: Diffie-Hellman, then the name and password under AES with the
        # MD5 of the shared secret as the key.
        generator, size = struct.unpack(">HH", self.file.read(4))
        prime = int.from_bytes(self.file.read(size), "big")
        theirs = int.from_bytes(self.file.read(size), "big")
        secret = int.from_bytes(os.urandom(size), "big") % (prime - 2) + 2
        shared = pow(theirs, secret, prime).to_bytes(size, "big")

        def field(text):
            raw = text.encode() + b"\0"
            return raw + os.urandom(64 - len(raw))

        enc = Cipher(algorithms.AES(hashlib.md5(shared).digest()), modes.ECB()).encryptor()
        sealed = enc.update(field(user) + field(password)) + enc.finalize()
        self.sock.sendall(sealed + pow(generator, secret, prime).to_bytes(size, "big"))

    def _vnc_login(self, password):
        # Plain VNC login: DES with the bits of each key byte reversed. Virtualization's own
        # VNC server asks for this one.
        key = bytes(int(f"{c:08b}"[::-1], 2) for c in password.encode()[:8].ljust(8, b"\0"))
        enc = Cipher(TripleDES(key * 3), modes.ECB()).encryptor()
        self.sock.sendall(enc.update(self.file.read(16)) + enc.finalize())

    def _request(self, incremental):
        self.sock.sendall(struct.pack(">BBHHHH", 3, int(incremental), 0, 0, self.width, self.height))

    def _read(self):
        try:
            while True:
                kind = self.file.read(1)[0]
                if kind == 0:
                    count = struct.unpack(">xH", self.file.read(3))[0]
                    for _ in range(count):
                        x, y, w, h, enc = struct.unpack(">HHHHi", self.file.read(12))
                        if enc == 0:
                            data = self.file.read(w * h * 4)
                            self.fb[y:y + h, x:x + w] = np.frombuffer(data, np.uint8).reshape(h, w, 4)
                        elif enc == -223:
                            self.width, self.height = w, h
                            self.fb = np.zeros((h, w, 4), dtype=np.uint8)
                            self.scale = w / POINTS_WIDE
                        else:
                            raise RuntimeError(f"unexpected encoding {enc}")
                    with self.cond:
                        self.frames += 1
                        self.cond.notify_all()
                elif kind == 2:
                    pass
                elif kind == 3:
                    self.file.read(3)
                    self.file.read(struct.unpack(">I", self.file.read(4))[0])
                else:
                    raise RuntimeError(f"unexpected message {kind}")
        except (IndexError, OSError, ValueError):
            with self.cond:
                self.closed = True
                self.cond.notify_all()

    def wait_frame(self, timeout=20):
        """Until the VM's screen changes, or timeout seconds."""
        self.window.wait(timeout)

    def image(self):
        """The VM's screen now, from its window, as an array of RGB, row then column, in pixels."""
        return self.window.image()

    def shot(self, path):
        Image.fromarray(self.image()).save(path)
        return path

    def _pointer(self):
        self.sock.sendall(struct.pack(">BBHH", 5, self.buttons, int(self.x * self.scale), int(self.y * self.scale)))

    def move(self, x, y, pause=0.08):
        self.x, self.y = x, y
        self._pointer()
        time.sleep(pause)

    def press(self):
        self.buttons = 1
        self._pointer()

    def release(self):
        self.buttons = 0
        self._pointer()

    def click(self, x, y):
        self.move(x, y, 0.25)
        self.press()
        time.sleep(0.06)
        self.release()

    def dbl(self, x, y):
        self.move(x, y, 0.25)
        self.press()
        time.sleep(0.04)
        self.release()
        time.sleep(0.08)
        self.press()
        time.sleep(0.04)
        self.release()

    def drag(self, x1, y1, x2, y2, steps=20):
        self.move(x1, y1, 0.3)
        self.press()
        time.sleep(0.2)
        for i in range(1, steps + 1):
            self.move(x1 + (x2 - x1) * i / steps, y1 + (y2 - y1) * i / steps, 0.02)
        time.sleep(0.2)
        self.release()

    def _key(self, sym, down):
        self.sock.sendall(struct.pack(">BBxxI", 4, int(down), sym))

    def key(self, combo):
        """A combination such as cmd+t or ctrl+cmd+f, pressed and released in order."""
        syms = [keysym(k) for k in combo.split("+")]
        for s in syms:
            self._key(s, True)
            time.sleep(0.02)
        for s in reversed(syms):
            self._key(s, False)
            time.sleep(0.02)

    def type(self, text):
        for c in text:
            self.key("shift+" + c if c.isupper() or c in SHIFTED else c)
            time.sleep(0.05)

    def close(self):
        self.sock.close()


if __name__ == "__main__":
    action, args = sys.argv[1], sys.argv[2:]
    vnc = Vnc()
    if action == "shot":
        time.sleep(0.5)
        print(vnc.shot(args[0]))
    elif action in ("click", "dbl", "move"):
        getattr(vnc, action)(float(args[0]), float(args[1]))
    elif action == "drag":
        vnc.drag(*[float(a) for a in args])
    elif action == "key":
        vnc.key(args[0])
    elif action == "type":
        vnc.type(args[0])
    else:
        sys.exit("unknown action " + action)
    time.sleep(0.1)
    vnc.close()
