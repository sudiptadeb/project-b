#!/bin/bash
# Builds the first app's library and two binaries. Flat on purpose; safe to re-run.
# RELEASE=1 OUT=out/rel tools/build.sh makes an official-style build: no DCHECKs, link-time
# optimisation, no symbols. It takes hours and can't be rebuilt quickly.
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SRC=$ROOT/chromium/src
OUT=${OUT:-out/pb}
RELEASE=${RELEASE:-0}
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
symbol_level = 0
blink_symbol_level = 0
v8_symbol_level = 0
use_remoteexec = false
use_siso = true
target_cpu = "arm64"
root_extra_deps = [ "//project_b/engine" ]
ARGS
if [ "$RELEASE" = 1 ]; then
  cat >> "$OUT/args.gn" <<ARGS
is_official_build = true
chrome_pgo_phase = 0
enable_dsyms = false
ARGS
else
  echo "is_official_build = false" >> "$OUT/args.gn"
fi

echo "=== $(date '+%F %T') gn gen"
gn gen "$OUT" || { echo "RESULT=failed gn"; exit 1; }

echo "=== $(date '+%F %T') build $TARGETS"
START=$(date +%s)
autoninja -C "$OUT" -k 0 $TARGETS
RC=$?
echo "=== $(date '+%F %T') build rc=$RC in $(( $(date +%s) - START ))s"
[ "$RC" -eq 0 ] || { echo "RESULT=failed build"; exit 1; }

# One folder per page: shell/shell/, shell/settings/.
rm -rf "$OUT/shell"
cp -R "$ROOT/shell/dist" "$OUT/shell"
ls -la "$OUT/project_b" "$OUT/project_b_helper" "$OUT/libproject_b_content.dylib" 2>/dev/null
echo "RESULT=ok $(date '+%F %T')"
