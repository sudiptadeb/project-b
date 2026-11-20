#!/usr/bin/env python3
"""Builds the Go core (core/) for the target GN builds, and writes a depfile: libcore.a, a
c-archive the Mac's main binary links, or libcore.so, which Linux's browser process loads
(engine/app/main_linux.cc) and Android's library links, built with Chromium's own clang and
the target's sysroot (the NDK's for Android, as Chromium's copy of it has no compilers).

    build_go.py <go> <out> <depfile> <target_os> <target_cpu> <the target's own arguments...>

The Mac's own arguments are its deployment target, then the build flags built in, each a Go
build tag. Linux's are clang and the sysroot, then the build flags. Android's are clang, the
NDK, the minimum SDK and the app's version."""

import os
import shutil
import subprocess
import sys

# GN's names for the target, as Go names them.
GOOS = {"mac": "darwin", "linux": "linux", "android": "android"}
GOARCH = {"arm64": "arm64", "x64": "amd64"}

# What a phone serves of the shell's built pages, under shell/dist: our pages and the
# phone's shells, the registry's "phone": true. A phone loads no other shell.
PHONE_SHELL = ("settings", "downloads", "history", "bookmarks", "welcome", "shell/mobile")


def main():
    go, out, depfile, target_os, target_cpu = sys.argv[1:6]
    rest = sys.argv[6:]
    repo = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
    build_dir = os.getcwd()
    # Inside the source tree the repo is reached through src/project_b.
    link = os.path.relpath(os.path.join(build_dir, "../../project_b"), build_dir)
    dist = os.path.join(repo, "core", "shellfiles", "dist")

    env = dict(os.environ)
    env.update({
        "CGO_ENABLED": "1",
        "GOOS": GOOS[target_os],
        "GOARCH": GOARCH[target_cpu],
        "GOCACHE": os.path.join(build_dir, "gocache"),
        "GOFLAGS": "-mod=mod",
        "PATH": os.path.dirname(go) + ":/usr/bin:/bin",
    })
    if target_os in ("mac", "linux"):
        # Chromium's version, for --version, which Go prints (core/cmd/lib PBInit).
        version_file = os.path.join(build_dir, "../../chrome/VERSION")
        parts = dict(line.strip().split("=", 1) for line in open(version_file) if "=" in line)
        chromium = ".".join(parts[k] for k in ("MAJOR", "MINOR", "BUILD", "PATCH"))
        deps = [os.path.relpath(version_file, build_dir)]
        shutil.rmtree(dist)
        # Each build flag the build has is a Go build tag of the same name.
        tags = ["sqlite_fts5"]
        mode = "c-archive"
        ldflags = "-X main.chromiumVersion=" + chromium
    if target_os == "mac":
        deployment_target, built = rest[0], rest[1:]
        tags += built
        # The shell's pages, which ninja built into shell/ first, go inside Go
        # (core/shellfiles): the app carries no folder of them.
        shutil.copytree(os.path.join(build_dir, "shell"), dist, ignore=shutil.ignore_patterns(".*"))
        deps.append(os.path.join("shell", ".stamp"))
        env.update({
            "CGO_CFLAGS": f"-mmacosx-version-min={deployment_target} -O2",
            "CGO_LDFLAGS": f"-mmacosx-version-min={deployment_target}",
            "MACOSX_DEPLOYMENT_TARGET": deployment_target,
        })
    elif target_os == "linux":
        clang, sysroot, built = rest[0], rest[1], rest[2:]
        tags += built
        # The shell's pages, all of shell/dist, which tools/build.sh linux built on the Mac
        # (the container has no Node), as for Android.
        shell_dist = os.path.join(repo, "shell", "dist")
        shutil.copytree(shell_dist, dist, ignore=shutil.ignore_patterns(".*"))
        for root, _, files in os.walk(shell_dist):
            deps += [os.path.join(link, os.path.relpath(os.path.join(root, name), repo))
                     for name in files]
        triple = {"arm64": "aarch64-linux-gnu", "x64": "x86_64-linux-gnu"}[target_cpu]
        env.update({
            "CC": " ".join([os.path.abspath(clang), "--target=" + triple,
                            "--sysroot=" + os.path.abspath(sysroot)]),
            "CGO_CFLAGS": "-O2",
            "HOME": build_dir,
        })
        mode = "c-shared"
        ldflags += " -extldflags '-fuse-ld=lld -Wl,-soname,libcore.so'"
    else:
        clang, ndk, min_sdk, version = rest
        # The phone's part of shell/dist, which tools/build.sh android built: the APK
        # carries no folder of them.
        built = os.path.join(repo, "shell", "dist")
        shutil.rmtree(dist)
        deps = []
        for folder in PHONE_SHELL:
            shutil.copytree(os.path.join(built, folder), os.path.join(dist, folder))
            for root, _, files in os.walk(os.path.join(built, folder)):
                deps += [os.path.join(link, os.path.relpath(os.path.join(root, name), repo))
                         for name in files]
        sysroot = os.path.join(os.path.abspath(ndk), "toolchains/llvm/prebuilt/linux-x86_64/sysroot")
        env.update({
            "CC": " ".join([os.path.abspath(clang), "--target=aarch64-linux-android" + min_sdk,
                            "--sysroot=" + sysroot]),
            "HOME": build_dir,
        })
        tags = ["sqlite_fts5"]
        mode = "c-shared"
        # Phones with 16 KB memory pages refuse libraries aligned for 4 KB. The soname is how
        # the library that links this one finds it at runtime: without it, it would record the
        # build-time path.
        ldflags = ("-s -w -X main.version=" + version +
                   " -extldflags '-fuse-ld=lld --unwindlib=none"
                   " -Wl,-z,max-page-size=16384 -Wl,-soname,libcore.so'")
    open(os.path.join(dist, ".keep"), "w").close()

    # sqlite_fts5 always: the store's search needs SQLite's FTS5 (docs/store.md).
    subprocess.check_call(
        [go, "build", "-buildmode=" + mode, "-trimpath", "-tags", ",".join(tags),
         "-ldflags", ldflags, "-o", os.path.join(build_dir, out), "./cmd/lib"],
        cwd=os.path.join(repo, "core"), env=env)

    deps.append(os.path.join(link, "engine/abi/pb_abi.h"))
    # core, and browserdata, the module of its own that core's go.mod replaces with ../browserdata.
    for module in ("core", "browserdata"):
        top = os.path.join(repo, module)
        for root, _, files in os.walk(top):
            if "testdata" in os.path.relpath(root, top).split(os.sep):
                continue
            # Each folder too: a Go file added or removed changes its folder's time, and ninja,
            # which knows only the files that were there last time, would keep the old library.
            deps.append(os.path.join(link, module, os.path.relpath(root, top)))
            for name in files:
                if name.endswith((".go", ".mod", ".json")) and not name.endswith("_test.go"):
                    rel = os.path.relpath(os.path.join(root, name), top)
                    deps.append(os.path.join(link, module, rel))
    with open(depfile, "w") as f:
        f.write(out + ": " + " ".join(sorted(deps)) + "\n")


if __name__ == "__main__":
    main()
