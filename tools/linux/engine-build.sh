#!/bin/bash
# Builds the Linux app (our engine, the Go core and the helper) in container pb, from the tree
# tools/build.sh linux copied to $REPO. Runs inside the container; safe to re-run.
#   REPO=/work/linux-release CPU=arm64 bash tools/linux/engine-build.sh
# It takes the container's build lock, as Android's build does: the tree in /work/tree and the
# //project_b links are shared, so Linux and APK builds take turns (docs/android.md). Chromium's
# x64 clang under Rosetta now and then dies from a signal (siso's exit=-1); those steps are
# built again, up to 5 times, and a real compile error ends it.
set -uo pipefail

WORK=/work
SRC=$WORK/chromium/src
CPU=${CPU:-arm64}
OUT=out/linux-$CPU
REPO=${REPO:?the tree to build}
TREE=$WORK/tree
WITHOUT=${WITHOUT:-extensions}

export PATH=$WORK/depot_tools:/usr/local/go/bin:/usr/sbin:/usr/bin:/sbin:/bin
export GODEBUG=asyncpreemptoff=1 PYTHONUNBUFFERED=1 DEPOT_TOOLS_UPDATE=0

exec 9>"$WORK/.android-build.lock"
if ! flock -n 9; then
  echo "=== $(date '+%F %T') another build holds the container; waiting for it to finish"
  flock 9
fi

echo "=== $(date '+%F %T') copy $REPO to $TREE by content"
python3 - "$REPO" "$TREE" <<'PY' || { echo "RESULT=failed copy"; exit 1; }
import filecmp, os, shutil, sys
src, dst = sys.argv[1:3]
keep = set()
for top in ("engine", "core", "browserdata", "shell/dist", "VERSION"):
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

mkdir -p "$SRC/project_b"
for part in engine core browserdata shell VERSION; do
  ln -sfn "$TREE/$part" "$SRC/project_b/$part"
done

cd "$SRC" || exit 1
python3 build/linux/sysroot_scripts/install-sysroot.py --arch="$CPU" >/dev/null || { echo "RESULT=failed sysroot"; exit 1; }
mkdir -p "$OUT"
# The spike's content_shell args, so what it built is kept, and our engine's.
cat > "$OUT/args.gn.new" <<ARGS
target_os = "linux"
target_cpu = "$CPU"
is_debug = false
is_component_build = false
symbol_level = 0
blink_symbol_level = 0
use_remoteexec = false
dcheck_always_on = false
root_extra_deps = [ "//project_b/engine" ]
project_b_without = "$WITHOUT"
ARGS
if ! cmp -s "$OUT/args.gn.new" "$OUT/args.gn"; then
  mv "$OUT/args.gn.new" "$OUT/args.gn"
  echo "=== $(date '+%F %T') gn gen"
  gn gen "$OUT" || { echo "RESULT=failed gn"; exit 1; }
else
  rm "$OUT/args.gn.new"
fi

for try in 1 2 3 4 5; do
  echo "=== $(date '+%F %T') build, try $try"
  START=$(date +%s)
  nice -n 19 autoninja -C "$OUT" -j 12 -k 0 project_b/engine:engine
  RC=$?
  echo "=== $(date '+%F %T') rc=$RC in $(( $(date +%s) - START ))s"
  [ "$RC" -eq 0 ] && { echo "RESULT=ok $(date '+%F %T')"; exit 0; }
  # Only steps a signal killed are worth another go: every failure there must be one.
  ERRS=$(grep '^err:' "$OUT/siso_output" 2>/dev/null)
  if [ -z "$ERRS" ] || echo "$ERRS" | grep -v -q 'exit=-1'; then
    break
  fi
done
echo "=== the end of siso's output:"
tail -c 8000 "$OUT/siso_output" 2>/dev/null
echo "RESULT=failed build"
exit 1
