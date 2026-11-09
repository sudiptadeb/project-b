#!/bin/bash
# Builds Android targets on the Linux amd64 host that tools/android-content-fetch.sh made. Flat on
# purpose; safe to re-run. TARGETS picks what to build.
set -uo pipefail

WORK=/work
SRC=$WORK/chromium/src
OUT=${OUT:-out/android}
TARGETS=${TARGETS:-project_b_apk}
GO_VERSION=1.26.5
REPO=/repo   # This worktree, mounted into the container.

export PATH=$WORK/depot_tools:/usr/local/go/bin:/usr/sbin:/usr/bin:/sbin:/bin
# Rosetta deadlocks on its own lock when Go's preemption signal lands while it's translating, so
# Go tools like cipd and siso hang with every thread in the kernel's PI futex wait.
export GODEBUG=asyncpreemptoff=1
export PYTHONUNBUFFERED=1 DEPOT_TOOLS_UPDATE=0

[ -x /usr/local/go/bin/go ] || curl -fsSL "https://go.dev/dl/go$GO_VERSION.linux-amd64.tar.gz" | tar -C /usr/local -xz

echo "=== $(date '+%F %T') expose the repo to GN, with no upstream edits"
mkdir -p "$SRC/project_b"
ln -sfn "$REPO/engine" "$SRC/project_b/engine"
ln -sfn "$REPO/core" "$SRC/project_b/core"
ln -sfn "$REPO/shell" "$SRC/project_b/shell"

cd "$SRC" || exit 1
mkdir -p "$OUT"
cat > "$OUT/args.gn" <<ARGS
target_os = "android"
target_cpu = "arm64"
is_debug = false
is_official_build = false
is_component_build = false
dcheck_always_on = false
symbol_level = 0
blink_symbol_level = 0
v8_symbol_level = 0
use_remoteexec = false
use_siso = true
android_static_analysis = "off"
root_extra_deps = [ "//project_b/engine/android:project_b_apk" ]
ARGS

echo "=== $(date '+%F %T') gn gen"
START=$(date +%s)
gn gen "$OUT" || { echo "RESULT=failed gn"; exit 1; }
echo "=== gn took $(( $(date +%s) - START ))s"

echo "=== $(date '+%F %T') build $TARGETS"
START=$(date +%s)
autoninja -C "$OUT" -k 0 $TARGETS
RC=$?
# Under Rosetta about 1 compile in 15,000 dies with no message and passes when run again.
if [ "$RC" -ne 0 ]; then
  echo "=== $(date '+%F %T') once more for steps that died"
  autoninja -C "$OUT" -k 0 $TARGETS
  RC=$?
fi
echo "=== $(date '+%F %T') build rc=$RC in $(( $(date +%s) - START ))s"
df -h "$WORK" | tail -1
du -sh "$OUT"
[ "$RC" -eq 0 ] || { echo "RESULT=failed build"; exit 1; }
ls -la "$OUT"/apks/ 2>/dev/null
echo "RESULT=ok $(date '+%F %T')"
