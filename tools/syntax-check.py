#!/usr/bin/env python3
"""Compiles engine/ sources with -fsyntax-only using the flags GN generated, so errors
show up without waiting for a full build. Generated headers must already exist."""

import concurrent.futures
import os
import re
import subprocess
import sys

ROOT = os.path.expanduser("~/Official/workspace/ulaa/project-b")
OUT = os.path.join(ROOT, "chromium/src/out/pb")
CLANG = "../../third_party/llvm-build/Release+Asserts/bin/clang++"


def variables(path):
    found = {}
    for line in open(path):
        m = re.match(r"^(\w+) = (.*)$", line)
        if m:
            found[m.group(1)] = m.group(2)
    return found


def sources(path):
    for line in open(path):
        m = re.match(r"^build \S+: (cxx|objcxx) (\S+)", line)
        if m:
            yield m.group(1), m.group(2)


def check(item):
    rule, flags, source = item
    cmd = f"{CLANG} -fsyntax-only {flags} {source}"
    r = subprocess.run(cmd, shell=True, cwd=OUT, capture_output=True, text=True)
    return source, r.returncode, r.stderr


def main():
    items = []
    for target in ("browser", "common", "project_b", "project_b_helper"):
        path = os.path.join(OUT, f"obj/project_b/engine/{target}.ninja")
        v = variables(path)
        base = " ".join(v.get(k, "") for k in ("defines", "include_dirs", "cflags"))
        for rule, source in sources(path):
            extra = v.get("cflags_cc" if rule == "cxx" else "cflags_objcc", "")
            items.append((rule, f"{base} {extra}", source))
    if len(sys.argv) > 1:
        items = [i for i in items if any(a in i[2] for a in sys.argv[1:])]
    failed = 0
    with concurrent.futures.ThreadPoolExecutor(6) as pool:
        for source, rc, err in pool.map(check, items):
            print(("ok   " if rc == 0 else "FAIL ") + source)
            if rc:
                failed += 1
                print("\n".join(err.splitlines()[:40]))
    sys.exit(1 if failed else 0)


if __name__ == "__main__":
    main()
