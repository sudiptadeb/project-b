#!/bin/bash
# Runs the VM checks on one DMG from start to end, with nobody watching:
#   tools/vm/run.sh <path or URL of a DMG>
#   tools/vm/run.sh <path or URL of a DMG> windows    # only the checks of several windows
#   tools/vm/run.sh <path or URL of a DMG> profiles   # only the checks of several profiles
#   tools/vm/run.sh <path or URL of a DMG> update     # an update from it to the stable release
#   tools/vm/run.sh <path or URL of a DMG> workspaces # only the checks of workspaces
#
# It takes the VM's lock, starts the VM if it isn't running, readies it (prepare.py), installs
# the DMG, runs check.py,
# copies any crash reports the app left in the VM, and stops the VM again if it started it.
# Everything lands in one folder under $VM_HOME/shots, which $VM_HOME/shots/latest-<VM> points
# to: summary.txt (each check's line and the verdict), results.json, the shots, and
# index.html to look at them together.
#
# Exit 0: every check passed, or failed only as a known bug in check.py's KNOWN.
# Exit 1: a check failed that isn't known, or the checks stopped early.
# Exit 2: it couldn't run: no DMG, the VM is someone else's right now, or the VM didn't start
#         (macOS starts it only after someone has logged in at this Mac's desktop since boot).
set -u
. "$(dirname "$0")/vars.sh"
HERE=$(cd "$(dirname "$0")" && pwd)

DMG=${1:-}
[ -n "$DMG" ] || { sed -n '2,/^set -u/p' "$0" | sed '$d' | cut -c3-; exit 2; }
case "$DMG" in
    http*) ;;
    *) [ -f "$DMG" ] || { echo "no DMG at $DMG"; exit 2; } ;;
esac

# One run at a time per VM: the VM, its screen and its app are shared by every session on this Mac.
LOCK="$VM_HOME/run-$VM.lock"
if ! mkdir "$LOCK" 2> /dev/null; then
    held=$(cat "$LOCK/pid" 2> /dev/null)
    if [ -n "$held" ] && kill -0 "$held" 2> /dev/null; then
        echo "the VM is in use by run $held ($(cat "$LOCK/what" 2> /dev/null)); try again when it ends"
        exit 2
    fi
    echo "taking over the lock of run $held, which has ended"
fi
echo $$ > "$LOCK/pid"
echo "$DMG, from $(pwd), since $(date '+%H:%M')" > "$LOCK/what"

STARTED=0
cleanup() {
    [ "$STARTED" = 1 ] && "$HERE/stop.sh" > /dev/null 2>&1
    rm -rf "$LOCK"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

OUT="$VM_HOME/shots/$(date +%Y%m%d-%H%M%S)-$VM"
mkdir -p "$OUT"
ln -sfn "$OUT" "$VM_HOME/shots/latest-$VM"
exec > >(tee "$OUT/run.txt") 2>&1

if ! pgrep -f "tart run .* $VM\$" > /dev/null; then
    STARTED=1
fi
echo "== start the VM"
"$HERE/start.sh" || { echo "the VM didn't start"; exit 2; }
"$PY" "$HERE/prepare.py" || { echo "the VM isn't ready for the checks"; exit 2; }
echo "== install $DMG"
"$HERE/install-dmg.sh" "$DMG" || { echo "couldn't install the DMG"; exit 2; }

echo "== the checks"
SINCE=$("$TART" exec "$VM" /bin/date +%s)
PB_VM_SHOTS="$OUT" "$PY" "$HERE/check.py" "${@:2}"
CODE=$?

# The app's crash reports since the checks began, for whoever reads the run later.
"$TART" exec "$VM" /bin/sh -c "find ~/Library/Logs/DiagnosticReports -name 'project_b*' -newermt @$SINCE 2>/dev/null" |
    while read -r report; do
        "$TART" exec "$VM" /bin/cat "$report" > "$OUT/$(basename "$report")"
        echo "crash report: $OUT/$(basename "$report")"
    done

echo "== done, exit $CODE: $OUT/summary.txt"
exit $CODE
