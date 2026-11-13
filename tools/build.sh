#!/bin/bash
# Builds the first app's library and two binaries. Flat on purpose; safe to re-run.
# RELEASE=1 OUT=out/rel tools/build.sh makes an official-style build: no DCHECKs, link-time
# optimisation, no symbols. It takes hours and can't be rebuilt quickly.
# Every build flag (core/flags/registry.json) is built in unless WITHOUT names it:
# WITHOUT=extensions tools/build.sh. GN, Go's build tags and the shell all read it from
# there (docs/flags.md). Keep such a build in a folder of its own (OUT=out/noext), so
# switching doesn't rebuild either.
# Ninja builds the shell's pages into $OUT/shell, as it builds Go (engine/build_shell.py),
# so any build of project_b has a current shell.
# Compiles go to a remote build server when one answers at PB_REMOTE (default
# localhost:8980), and the build is local otherwise; PB_REMOTE=off keeps it local. The
# commands are the same either way. Setting a server up: docs/remote-build.md.
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SRC=$ROOT/chromium/src
# One build at a time: every build re-points chromium/src/project_b at its own checkout and
# shares out/, so build.sh holds out/.pb-build.lock for its whole run. A caller that already
# holds it around several builds (the release) says so with PB_BUILD_LOCKED=1.
if [ "${PB_BUILD_LOCKED:-}" != 1 ]; then
  mkdir -p "$SRC/out"
  exec python3 -I -c 'import fcntl, os, sys
f = open(sys.argv[1], "a")
fcntl.flock(f, fcntl.LOCK_EX)
os.set_inheritable(f.fileno(), True)
os.environ["PB_BUILD_LOCKED"] = "1"
os.execv("/bin/bash", ["/bin/bash"] + sys.argv[2:])' "$SRC/out/.pb-build.lock" "$0" "$@"
fi
OUT=${OUT:-out/pb}
RELEASE=${RELEASE:-0}
export WITHOUT=${WITHOUT:-}
TARGETS=${TARGETS:-project_b project_b_helper}
PB_REMOTE=${PB_REMOTE:-localhost:8980}
# How many compiles wait at the server at once. Siso's own default suits a fleet of
# thousands of workers; against one Mac, steps at the back of the queue time out.
PB_REMOTE_JOBS=${PB_REMOTE_JOBS:-60}

# depot_tools goes in front of the caller's PATH; everything else (node, go...) stays.
export PATH=$ROOT/depot_tools:$PATH
export PYTHONUNBUFFERED=1 DEPOT_TOOLS_UPDATE=0

# Failures we have met, as "text in the output|what to do". When a step fails, its output
# is matched against these and each hit is printed with its fix. Add a line whenever a new
# one turns up.
KNOWN_FAILURES='missing Metal Toolchain|xcodebuild -downloadComponent MetalToolchain (or Xcode > Settings > Components)
Symbol not found: _XPCTypeBool|Xcode components are stale for this macOS: sudo xcodebuild -runFirstLaunch
Error loading required libraries|Xcode components are stale for this macOS: sudo xcodebuild -runFirstLaunch
agreed to the Xcode|accept the Xcode license: sudo xcodebuild -license accept
xcrun: error: unable to lookup item|xcrun cannot see the SDK: check xcode-select -p, then sudo xcodebuild -runFirstLaunch
SDKSettings.plist|xcrun cannot see the SDK: check xcode-select -p, then sudo xcodebuild -runFirstLaunch
No space left on device|the disk is full: free space (the first build needs about 100 GB)
go: command not found|gn cannot find Go: check project_b_go in out/pb/args.gn
no such file or directory: '"'"'/opt/homebrew/bin/go|gn is using the default Go path: re-run tools/build.sh so it writes project_b_go
requires go >=|Go is older than core/go.mod asks for: install a newer Go from go.dev/dl
hook table is|hook tables differ: python3 tools/gen_hooks.py, then rebuild
Too many open files|raise the file limit: ulimit -n 10240, then re-run
/Users/Shared/pb-rbe/toolchains|the remote build server lacks this checkout'"'"'s clang: tools/remote/toolchain.sh
remote exec timeout|remote compiles waited too long at the server: lower PB_REMOTE_JOBS, or PB_REMOTE=off'

# explain FILE prints the fix for each known failure found in FILE.
explain() {
  local hits=0 pattern fix
  [ -s "$1" ] || return
  while IFS='|' read -r pattern fix; do
    if grep -qF -- "$pattern" "$1"; then
      [ "$hits" -eq 0 ] && echo "=== known failure(s):"
      echo "  - $pattern"
      echo "    fix: $fix"
      hits=$((hits + 1))
    fi
  done <<< "$KNOWN_FAILURES"
  [ "$hits" -eq 0 ] && echo "=== not a known failure; add it to KNOWN_FAILURES in tools/build.sh once solved"
}

# Check the machine first and report every problem at once, each with its fix, rather
# than failing an hour in or deep inside gn.
echo "=== $(date '+%F %T') check the machine"
PROBLEMS=0
problem() { echo "  FAIL $1"; echo "       fix: $2"; PROBLEMS=$((PROBLEMS + 1)); }
ok() { echo "  ok   $1"; }

if NODE=$(command -v node); then ok "node $(node --version) at $NODE"
else problem "node is not on PATH" "brew install node, then open a new terminal"; fi

if [ -d "$ROOT/shell/node_modules" ]; then ok "shell dependencies installed"
else problem "shell/node_modules is missing" "(cd shell && npm install)"; fi

if GO=$(command -v go); then
  GO_HAVE=$(go env GOVERSION | sed 's/^go//')
  GO_WANT=$(awk '$1 == "go" { print $2 }' "$ROOT/core/go.mod")
  if [ "$(printf '%s\n%s\n' "$GO_WANT" "$GO_HAVE" | sort -V | head -1)" = "$GO_WANT" ]; then
    ok "go $GO_HAVE at $GO (core/go.mod wants $GO_WANT)"
  else
    problem "go $GO_HAVE at $GO is older than the $GO_WANT core/go.mod wants" \
            "install the latest Go from https://go.dev/dl"
  fi
else problem "go is not on PATH" "brew install go, or add /usr/local/go/bin to PATH"; fi

if [ -d "$SRC" ] && [ -x "$ROOT/depot_tools/gn" ]; then ok "chromium and depot_tools present"
else problem "chromium/src or depot_tools is missing" "tools/fetch-chromium.sh"; fi

DEV_DIR=$(xcode-select -p 2>/dev/null)
case "$DEV_DIR" in
  *.app/Contents/Developer) ok "xcode-select points at $DEV_DIR" ;;
  *) problem "xcode-select points at '${DEV_DIR:-nothing}', not the full Xcode app" \
             "install Xcode, then sudo xcode-select -s /Applications/Xcode.app" ;;
esac

if xcodebuild -checkFirstLaunchStatus >/dev/null 2>&1; then ok "Xcode's first-launch setup is done"
else problem "Xcode's system components are missing or out of date for this macOS" \
             "sudo xcodebuild -runFirstLaunch (and sudo xcodebuild -license accept if asked)"; fi

if xcodebuild -license check >/dev/null 2>&1; then ok "Xcode license accepted"
else problem "the Xcode license has not been accepted" "sudo xcodebuild -license accept"; fi

if SDK=$(xcrun -sdk macosx --show-sdk-path 2>/dev/null); then ok "macOS SDK at $SDK"
else problem "xcrun can't find the macOS SDK" \
             "fix the Xcode items above; if they pass, open Xcode once or reinstall it"; fi

# Xcode 26 ships the Metal compiler separately; ANGLE's shaders need it.
if xcrun -sdk macosx metal --version >/dev/null 2>&1; then ok "Metal toolchain installed"
else problem "the Metal toolchain is missing (ANGLE compiles its shaders with it)" \
             "xcodebuild -downloadComponent MetalToolchain"; fi

# Disk space is only reported: a first build fills about 100 GB, later ones little.
if [ -d "$SRC/$OUT/obj" ]; then NEED_GB=20; else NEED_GB=100; fi
FREE_GB=$(df -g "$ROOT" | awk 'NR == 2 { print $4 }')
if [ "$FREE_GB" -ge "$NEED_GB" ]; then ok "$FREE_GB GB free (this build needs about $NEED_GB)"
else echo "  warn only $FREE_GB GB free; this build needs about $NEED_GB GB"; fi

if [ "$PROBLEMS" -gt 0 ]; then
  echo "RESULT=failed machine: $PROBLEMS problem(s) above"
  exit 1
fi

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
project_b_go = "$GO"
project_b_node = "$NODE"
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
echo "project_b_without = \"$WITHOUT\"" >> "$OUT/args.gn"

echo "=== $(date '+%F %T') gn gen"
gn gen "$OUT" 2>&1 | tee "$OUT/gn.log"
[ "${PIPESTATUS[0]}" -eq 0 ] || { explain "$OUT/gn.log"; echo "RESULT=failed gn"; exit 1; }

REMOTE_HOST=${PB_REMOTE%:*}
REMOTE_PORT=${PB_REMOTE##*:}
if [ "$PB_REMOTE" != off ] && nc -z -G 3 "$REMOTE_HOST" "$REMOTE_PORT" 2>/dev/null; then
  # Siso directly, not autoninja: args.gn keeps use_remoteexec off, so the compile
  # commands match a local build exactly, and tools/remote/siso/main.star chooses what
  # runs remotely (only compiles). autoninja would force offline mode for such a build.
  echo "=== $(date '+%F %T') build $TARGETS, compiles on the remote build server at $PB_REMOTE"
  START=$(date +%s)
  "$SRC/third_party/siso/cipd/siso" ninja -C "$OUT" -k 0 \
    -load "$ROOT/tools/remote/siso/main.star" \
    -reapi_address "$PB_REMOTE" -reapi_insecure -reapi_instance default_instance -project "" \
    -remote_jobs "$PB_REMOTE_JOBS" \
    $TARGETS
  RC=$?
else
  [ "$PB_REMOTE" = off ] || echo "  (no remote build server answers at $PB_REMOTE; building here)"
  echo "=== $(date '+%F %T') build $TARGETS"
  START=$(date +%s)
  autoninja -C "$OUT" -k 0 $TARGETS
  RC=$?
fi
echo "=== $(date '+%F %T') build rc=$RC in $(( $(date +%s) - START ))s"
if [ "$RC" -ne 0 ]; then
  # siso keeps every failed step's output here, so the build's own output stays as it is.
  FAILED=$(grep -c '^FAILED:' "$OUT/siso_output" 2>/dev/null)
  echo "=== ${FAILED:-0} failed step(s); their output is in $SRC/$OUT/siso_output"
  explain "$OUT/siso_output"
  echo "RESULT=failed build"
  exit 1
fi

ls -la "$OUT/project_b" "$OUT/project_b_helper" "$OUT/libproject_b_content.dylib" 2>/dev/null
echo "RESULT=ok $(date '+%F %T')"
