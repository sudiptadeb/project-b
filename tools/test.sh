#!/bin/bash
# Runs the tests, a group at a time. The groups differ in what they need, not in what they test:
#
#   tools/test.sh mock      Go's own tests, and the shell in a browser on the mock engine.
#                           Needs Go and `npm install`. No Chromium build, runs anywhere.
#   tools/test.sh app       The real app through its driver and through ChromeDriver.
#                           Needs the build, or PB_BIN, and ChromeDriver. No display.
#   tools/test.sh basics    One check per area of the whole-browser checklist, on the real
#                           app. Most fail today, on purpose: the result is a map of what
#                           isn't built. Needs what the app group needs. Not part of a release.
#   tools/test.sh screen    The real app in a person's desktop session: full screen, copy and
#                           paste, and first paint in the real window.
#                           NEEDS A PERSON LOGGED IN at this Mac's screen, with it unlocked,
#                           and desk allowed to record the screen.
#   tools/test.sh vm        Real clicks, drags and keys in the macOS VM, on build/project-b.dmg
#                           or PB_DMG (a path or URL): tools/vm/run.sh starts the VM, installs
#                           it, checks and stops the VM again. Known bugs don't fail it.
#                           NEEDS A PERSON to have logged in at this Mac's desktop once since
#                           it booted.
#   tools/test.sh android   The Android app on the emulator.
#                           Needs the emulator running and the APK, or PB_APK.
#
# The tests follow the build flags (core/flags/registry.json) as tools/build.sh does: all
# in unless WITHOUT names them. With extensions in, mock and app add Go's tests with the
# tag, the extension UI on the mock engine's fake runtime, and the real app installing
# from the mock store with the Extensions flag on, and running none of it with the flag
# off. They need tools/fetch-extensions.sh and tools/pack-test-extension.py to have filled
# .tools/extensions/. WITHOUT=extensions tests a build made without them (PB_BIN), and app
# checks that it has no extension code in it.
#
# Several groups can be named at once. A group or a test that lacks what it needs says so
# and ends with exit 2 instead of running. Each test prints ok or FAIL per check; the end is
# one line per test: passed, FAILED or DID NOT RUN. Exit 1 if any failed. README.md, "Test
# it", says how to get each need.
set -u
cd "$(dirname "$0")/.."

export WITHOUT=${WITHOUT:-}
# The build flags this run has, comma-separated: Go's build tags.
BUILT=$(python3 -c 'import json, os, sys
out = set(filter(None, os.environ["WITHOUT"].split(",")))
print(",".join(f["id"] for f in json.load(open("core/flags/registry.json")) if f.get("build") and f["id"] not in out))')
built() { case ",$BUILT," in *",$1,"*) return 0 ;; esac; return 1; }
BIN=${PB_BIN:-chromium/src/out/pb/project_b}
APK=${PB_APK:-build/android/project-b-content.apk}
ADB="$HOME/Library/Android/sdk/platform-tools/adb -s ${ADB_SERIAL:-emulator-5600}"
VM_PYTHON=$HOME/PW_TOOLS/macos-vm/venv/bin/python
SUMMARY=""
FAILED=0
DID_NOT_RUN=0

run() {
    local name=$1
    shift
    printf '\n=== %s\n' "$name"
    "$@"
    case $? in
        0) SUMMARY="$SUMMARY"$'\n'"passed       $name" ;;
        2) SUMMARY="$SUMMARY"$'\n'"DID NOT RUN  $name"; DID_NOT_RUN=2 ;;
        *) SUMMARY="$SUMMARY"$'\n'"FAILED       $name"; FAILED=1 ;;
    esac
}

cannot() {
    echo "can't run the $1 group: $2" >&2
    exit 2
}

need_store() {
    [ -f .tools/extensions/catalogue.json ] || cannot "$1" "no mock store in .tools/extensions. Run tools/fetch-extensions.sh and tools/pack-test-extension.py."
}

need_app() {
    [ -x "$BIN" ] || cannot "$1" "no app at $BIN. Build it with tools/build.sh, or point PB_BIN at a project_b binary."
}

mock() {
    [ -d shell/node_modules ] || cannot mock "run \`npm install\` in shell/ first."
    run "Go managers" go -C core test ./...
    [ -n "$BUILT" ] && run "Go managers, with $BUILT" go -C core test -tags "$BUILT" ./...
    run "build the shell" npm --prefix shell run build
    run "the shell on the mock engine" node shell/e2e/first-app.cjs
    run "the settings page on the mock engine" node shell/e2e/settings.cjs
    built extensions || return
    need_store mock
    run "extensions on the mock engine" node shell/e2e/extensions.cjs
}

app() {
    need_app app
    built extensions && need_store app
    run "the real app through its driver" python3 tools/drive_first_app.py
    run "the real app through ChromeDriver" python3 tools/drive_chromedriver.py
    if built extensions; then
        run "extensions in the real app" env PB_BIN="$BIN" python3 tools/drive_extensions.py
    else
        run "no extension code in the app" python3 tools/check_no_extensions.py "$BIN"
    fi
}

basics() {
    need_app basics
    run "the basics, one check per area" python3 tools/check_basics.py
}

screen() {
    need_app screen
    echo "The screen group NEEDS A PERSON LOGGED IN at this Mac's screen. The app's window will show there."
    python3 tools/on_screen.py || exit 2
    run "full screen on the real screen" python3 tools/on_screen.py python3 tools/check_full_screen.py
    run "copy and paste through the Mac's clipboard" python3 tools/on_screen.py python3 tools/check_clipboard.py
    run "first paint in the real window" python3 tools/on_screen.py python3 tools/check_first_paint.py
}

vm() {
    echo "The vm group NEEDS A PERSON to have logged in at this Mac's desktop once since it booted."
    [ -x "$VM_PYTHON" ] || cannot vm "no $VM_PYTHON, the Python with numpy that the VM's tools use. Run tools/vm/setup.sh once."
    run "real clicks in the macOS VM" tools/vm/run.sh "${PB_DMG:-build/project-b.dmg}"
}

android() {
    [ -f "$APK" ] || cannot android "no APK at $APK. Build it (docs/android.md), or point PB_APK at one."
    [ "$($ADB get-state 2>/dev/null)" = device ] || cannot android "no emulator answers. Start it with \`amarit start project-b-emu\`."
    until [ "$($ADB shell getprop sys.boot_completed | tr -d '\r')" = 1 ]; do /bin/sleep 2; done
    run "the Android app on the emulator" env PB_APK="$APK" python3 tools/drive_android.py
}

[ $# -gt 0 ] || { sed -n '2,/^set -u/p' "$0" | sed '$d' | cut -c3-; exit 2; }
for group in "$@"; do
    case $group in
        mock | app | basics | screen | vm | android) $group ;;
        *) echo "no group called $group: mock, app, basics, screen, vm or android" >&2; exit 2 ;;
    esac
done
echo "$SUMMARY"
[ $FAILED = 1 ] && exit 1
exit $DID_NOT_RUN
