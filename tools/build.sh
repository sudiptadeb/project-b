#!/bin/bash
# Builds the first app's two binaries. Flat on purpose; safe to re-run.
set -uo pipefail

ROOT=$HOME/Official/workspace/ulaa/project-b
SRC=$ROOT/chromium/src
OUT=out/pb
TARGETS=${TARGETS:-project_b project_b_helper}

export PATH=$ROOT/depot_tools:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin
export PYTHONUNBUFFERED=1 DEPOT_TOOLS_UPDATE=0

echo "=== $(date '+%F %T') shell"
(cd "$ROOT/shell" && npm run build 2>&1 | tail -2) || { echo "RESULT=failed shell"; exit 1; }

echo "=== $(date '+%F %T') expose the repo to GN, with no upstream edits"
mkdir -p "$SRC/project_b"
ln -sfn ../../../engine "$SRC/project_b/engine"
ln -sfn ../../../core "$SRC/project_b/core"
ln -sfn ../../../shell "$SRC/project_b/shell"

cd "$SRC" || exit 1
mkdir -p "$OUT"
cat > "$OUT/args.gn" <<ARGS
is_debug = false
is_component_build = false
is_official_build = false
symbol_level = 0
blink_symbol_level = 0
v8_symbol_level = 0
use_remoteexec = false
use_siso = true
target_cpu = "arm64"
root_extra_deps = [ "//project_b/engine" ]
ARGS

echo "=== $(date '+%F %T') gn gen"
gn gen "$OUT" || { echo "RESULT=failed gn"; exit 1; }

echo "=== $(date '+%F %T') build $TARGETS"
START=$(date +%s)
autoninja -C "$OUT" -k 0 $TARGETS
RC=$?
echo "=== $(date '+%F %T') build rc=$RC in $(( $(date +%s) - START ))s"
[ "$RC" -eq 0 ] || { echo "RESULT=failed build"; exit 1; }

mkdir -p "$OUT/shell"
cp -f "$ROOT/shell/dist/"* "$OUT/shell/"
ls -la "$OUT/project_b" "$OUT/project_b_helper" 2>/dev/null
echo "RESULT=ok $(date '+%F %T')"
