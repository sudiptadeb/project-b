#!/usr/bin/env python3
"""Builds the native shell's Swift into a static archive and its Objective-C header.

Chromium's Mac toolchain has no swift tool (build/toolchain/apple/toolchain.gni keeps it
to iOS), so Swift is built beside GN, as Go is: Xcode's swiftc, one action. Spike only.

  build_swift.py out.a header.h link.rsp deployment_target sdk_path source.swift...

link.rsp gets the -L flags for the folders the link finds the Swift libraries in: the
SDK's (the OS's runtime, as .tbd files) and the toolchain's (libswiftCompatibilityPacks).
"""

import os
import subprocess
import sys


def toolchain_lib():
    swiftc = subprocess.check_output(["xcrun", "--find", "swiftc"], text=True).strip()
    return os.path.join(os.path.dirname(os.path.dirname(swiftc)), "lib", "swift", "macosx")


def main():
    out, header, rsp, deployment_target, sdk = sys.argv[1:6]
    sources = sys.argv[6:]
    with open(rsp, "w") as f:
        f.write(f"-L{os.path.join(sdk, 'usr', 'lib', 'swift')}\n-L{toolchain_lib()}\n")
    subprocess.check_call([
        "xcrun", "swiftc", "-parse-as-library", "-module-name", "ProjectBNative",
        "-emit-library", "-static", "-o", out,
        "-emit-objc-header-path", header,
        "-target", f"arm64-apple-macos{deployment_target}", "-sdk", sdk,
        "-O", *sources,
    ])


if __name__ == "__main__":
    main()
