#!/usr/bin/env python3
"""Builds core/ into libcore.so for Android, and writes a depfile. cgo uses Chromium's own clang, with
the NDK's sysroot. Chromium's copy of the NDK has no compilers of its own."""

import os
import subprocess
import sys


def main():
    go, out, depfile, clang, ndk, min_sdk = sys.argv[1:7]
    repo = os.path.dirname(os.path.dirname(os.path.dirname(os.path.realpath(__file__))))
    build_dir = os.getcwd()
    link = os.path.relpath(os.path.join(build_dir, "../../project_b"), build_dir)
    sysroot = os.path.join(os.path.abspath(ndk), "toolchains/llvm/prebuilt/linux-x86_64/sysroot")
    cc = " ".join([os.path.abspath(clang), "--target=aarch64-linux-android" + min_sdk,
                   "--sysroot=" + sysroot])

    env = dict(os.environ)
    env.update({
        "CGO_ENABLED": "1",
        "GOOS": "android",
        "GOARCH": "arm64",
        "CC": cc,
        "GOCACHE": os.path.join(build_dir, "gocache"),
        "GOFLAGS": "-mod=mod",
        "HOME": build_dir,
        "PATH": os.path.dirname(go) + ":/usr/bin:/bin",
    })
    # Phones with 16 KB memory pages refuse libraries aligned for 4 KB. The soname is how the library
    # that links this one finds it at runtime: without it, it would record the build-time path.
    subprocess.check_call(
        [go, "build", "-buildmode=c-shared", "-trimpath",
         "-ldflags", "-s -w -extldflags '-fuse-ld=lld --unwindlib=none -Wl,-z,max-page-size=16384 -Wl,-soname,libcore.so'",
         "-o", os.path.join(build_dir, out), "./cmd/lib"],
        cwd=os.path.join(repo, "core"), env=env)

    deps = [os.path.join(link, "engine/abi/pb_abi.h")]
    core = os.path.join(repo, "core")
    for root, _, files in os.walk(core):
        for name in files:
            if name.endswith((".go", ".mod")) and not name.endswith("_test.go"):
                rel = os.path.relpath(os.path.join(root, name), core)
                deps.append(os.path.join(link, "core", rel))
    with open(depfile, "w") as f:
        f.write(out + ": " + " ".join(sorted(deps)) + "\n")


if __name__ == "__main__":
    main()
