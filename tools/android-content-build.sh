#!/bin/bash
# Builds Android targets on the Linux amd64 host that tools/android-content-fetch.sh made. Flat on
# purpose; safe to re-run. TARGETS picks what to build.
set -uo pipefail

WORK=/work
SRC=$WORK/chromium/src
OUT=${OUT:-out/android}
TARGETS=${TARGETS:-project_b_apk}
GO_VERSION=1.26.5
# The tree to build: this worktree, mounted into the container, or a release's, which
# tools/android-release.sh copies in.
REPO=${REPO:-/repo}
TREE=$WORK/tree

export PATH=$WORK/depot_tools:/usr/local/go/bin:/usr/sbin:/usr/bin:/sbin:/bin
# Rosetta deadlocks on its own lock when Go's preemption signal lands while it's translating, so
# Go tools like cipd and siso hang with every thread in the kernel's PI futex wait.
export GODEBUG=asyncpreemptoff=1
export PYTHONUNBUFFERED=1 DEPOT_TOOLS_UPDATE=0

[ -x /usr/local/go/bin/go ] || curl -fsSL "https://go.dev/dl/go$GO_VERSION.linux-amd64.tar.gz" | tar -C /usr/local -xz

# GN builds a copy of the tree in which a file is rewritten only when its content changes. Ninja
# goes by file times, so building another checkout straight from where it is could leave an
# output newer than a changed source, and the output would be kept.
echo "=== $(date '+%F %T') copy $REPO to $TREE by content"
python3 - "$REPO" "$TREE" <<'PY' || { echo "RESULT=failed copy"; exit 1; }
import filecmp, os, shutil, sys
src, dst = sys.argv[1:3]
keep = set()
for top in ("engine", "core", "shell/dist", "VERSION"):
    base = os.path.join(src, top)
    files = [base] if os.path.isfile(base) else [os.path.join(d, f) for d, _, fs in os.walk(base) for f in fs]
    for path in files:
        rel = os.path.relpath(path, src)
        if "__pycache__" in rel:
            continue
        keep.add(rel)
        to = os.path.join(dst, rel)
        if os.path.isfile(to) and filecmp.cmp(path, to, shallow=False):
            continue
        os.makedirs(os.path.dirname(to), exist_ok=True)
        shutil.copyfile(path, to)
        shutil.copymode(path, to)
for d, _, fs in os.walk(dst):
    for f in fs:
        if os.path.relpath(os.path.join(d, f), dst) not in keep:
            os.remove(os.path.join(d, f))
PY

echo "=== $(date '+%F %T') expose the tree to GN, with no upstream edits"
mkdir -p "$SRC/project_b"
ln -sfn "$TREE/engine" "$SRC/project_b/engine"
ln -sfn "$TREE/core" "$SRC/project_b/core"
ln -sfn "$TREE/shell" "$SRC/project_b/shell"

# The APK's versionCode grows with VERSION: 0.2.13 is 2013.
VERSION=$(tr -d '[:space:]' < "$TREE/VERSION")
IFS=. read -r MAJOR MINOR PATCH <<< "$VERSION"
VERSION_CODE=$(( MAJOR * 1000000 + MINOR * 1000 + PATCH ))

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
project_b_version = "$VERSION"
project_b_version_code = $VERSION_CODE
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
