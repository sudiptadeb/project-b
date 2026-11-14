#!/usr/bin/env python3
"""Compiles the native shells' Kotlin into a jar for the APK.

Chromium's android_library compiles .kt files only from an allowlist in
build/config/android/internal_rules.gni, and we make no upstream edits, so Kotlin is built
beside GN's Java rules, as Go and the Mac's Swift are: Chromium's own kotlinc and JDK, one
call, against the Android SDK's android.jar. engine/android/BUILD.gn takes the jar in as an
android_java_prebuilt, with Kotlin's standard library. docs/native-shell.md.

  build_kotlin.py kotlinc jdk android.jar out.jar source.kt...
"""

import os
import subprocess
import sys


def main():
    kotlinc, jdk, android_jar, out = sys.argv[1:5]
    sources = sys.argv[5:]
    env = dict(os.environ, JAVA_HOME=jdk, PATH=os.path.join(jdk, "bin") + ":/usr/bin:/bin")
    subprocess.check_call(
        [kotlinc, "-no-reflect", "-jvm-target", "11", "-classpath", android_jar, "-d", out,
         "-Xno-call-assertions", "-Xno-param-assertions", "-Xno-receiver-assertions", *sources],
        env=env)


if __name__ == "__main__":
    main()
