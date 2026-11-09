#!/usr/bin/env python3
"""Builds core/ into a c-archive for the main binary, and writes a depfile."""

import os
import subprocess
import sys


def main():
    go, out, depfile, deployment_target = sys.argv[1:5]
    repo = os.path.dirname(os.path.dirname(os.path.realpath(__file__)))
    build_dir = os.getcwd()
    # Inside the source tree the repo is reached through src/project_b.
    link = os.path.relpath(os.path.join(build_dir, "../../project_b"), build_dir)

    env = dict(os.environ)
    env.update({
        "CGO_ENABLED": "1",
        "GOOS": "darwin",
        "GOARCH": "arm64",
        "CGO_CFLAGS": f"-mmacosx-version-min={deployment_target} -O2",
        "CGO_LDFLAGS": f"-mmacosx-version-min={deployment_target}",
        "MACOSX_DEPLOYMENT_TARGET": deployment_target,
        "GOCACHE": os.path.join(build_dir, "gocache"),
        "GOFLAGS": "-mod=mod",
        "PATH": os.path.dirname(go) + ":/usr/bin:/bin",
    })
    subprocess.check_call(
        [go, "build", "-buildmode=c-archive", "-trimpath", "-o",
         os.path.join(build_dir, out), "./cmd/lib"],
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
