#!/usr/bin/env python3
"""Builds the shell's apps with vite into the build folder, and writes a depfile.

Each app in shell/src/apps/<app>/ lands in shell/<app>/, which the engine serves as
chrome://<app>/. The stamp is written last, inside that folder, so deleting the folder
builds it again.
"""

import os
import shutil
import subprocess
import sys


def main():
    node, stamp, depfile = sys.argv[1:4]
    without = ",".join(sys.argv[4:])
    build_dir = os.getcwd()
    # Inside the source tree the repo is reached through src/project_b.
    link = os.path.relpath(os.path.join(build_dir, "../../project_b"), build_dir)
    shell = os.path.realpath(os.path.join(link, "shell"))
    out = os.path.dirname(os.path.join(build_dir, stamp))

    env = dict(os.environ)
    env.update({
        "WITHOUT": without,
        "PATH": os.path.dirname(node) + ":/usr/bin:/bin",
    })
    vite = os.path.join(shell, "node_modules/vite/bin/vite.js")
    shutil.rmtree(out, ignore_errors=True)
    for app in sorted(os.listdir(os.path.join(shell, "src/apps"))):
        env["VITE_APP"] = app
        subprocess.check_call(
            [node, vite, "build", "--logLevel", "warn", "--outDir", os.path.join(out, app)],
            cwd=shell, env=env)

    deps = [os.path.join(link, "core/flags/registry.json")]
    deps += [os.path.join(link, "shell", name)
             for name in ("vite.config.js", "package.json", "package-lock.json")]
    for root, _, files in os.walk(os.path.join(shell, "src")):
        for name in files:
            rel = os.path.relpath(os.path.join(root, name), shell)
            deps.append(os.path.join(link, "shell", rel))
    with open(depfile, "w") as f:
        f.write(stamp + ": " + " ".join(sorted(deps)) + "\n")
    open(os.path.join(build_dir, stamp), "w").close()


if __name__ == "__main__":
    main()
