#!/usr/bin/env python3
"""Checks that a build made without EXTENSIONS=1 has no extension code in it.

    python3 tools/check_no_extensions.py [path to project_b]

Chromium's extension runtime is all in the C++ namespace `extensions`, and ours is in
files only an extensions build compiles, so the library is searched for any symbol of
that namespace, and Go's part of the binary for the package it would be in. The shell's
side of the same promise is the last check of shell/e2e/extensions.cjs.
"""
import os
import re
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
BIN = sys.argv[1] if len(sys.argv) > 1 else os.path.join(ROOT, "chromium/src/out/pb/project_b")
LIB = os.path.join(os.path.dirname(BIN), "libproject_b_content.dylib")
if not os.path.exists(LIB):
    # In an app bundle the library is in Contents/Frameworks.
    LIB = os.path.join(os.path.dirname(BIN), "../Frameworks/libproject_b_content.dylib")

failures = 0


def check(name, ok, detail=""):
    global failures
    print(("ok    " if ok else "FAIL  ") + name + ("" if ok else f"  {detail}"), flush=True)
    failures += not ok


def symbols(path):
    return subprocess.run(["nm", path], capture_output=True, text=True, errors="replace").stdout.splitlines()


# A mangled C++ name starts with _Z, and spells a member of namespace extensions as
# "10extensions" and then the length of the member's own name. A function that is itself
# called extensions, which Blink has, is followed by a letter instead; Rust's names, which
# have a module called extensions, start with _R.
found = [s for s in symbols(LIB) if re.search(r" __Z.*10extensions\d", s)]
check("the library has no symbol of Chromium's extensions namespace", not found, f"{len(found)}, such as {found[:2]}")
found = [s for s in symbols(BIN) if "projectb/extensions" in s]
check("the main binary has none of Go's extension packages", not found, f"{len(found)}, such as {found[:2]}")
sys.exit(1 if failures else 0)
