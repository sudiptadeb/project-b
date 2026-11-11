#!/bin/bash
# Builds the first app's library and two binaries. Flat on purpose; safe to re-run.
# RELEASE=1 OUT=out/rel tools/build.sh makes an official-style build: no DCHECKs, link-time
# optimisation, no symbols. It takes hours and can't be rebuilt quickly.
# Chrome extensions are in the build unless EXTENSIONS=0: the engine's GN arg, Go's build
# tag and the shell's extension UI, all three from this one switch. They run only while
# the Extensions flag is on (docs/flags.md). Keep a build without them in a folder of its
# own (EXTENSIONS=0 OUT=out/noext), so switching doesn't rebuild either.
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SRC=$ROOT/chromium/src
OUT=${OUT:-out/pb}
RELEASE=${RELEASE:-0}
export EXTENSIONS=${EXTENSIONS:-1}
TARGETS=${TARGETS:-project_b project_b_helper}

# depot_tools goes in front of the caller's PATH; everything else (npm, go...) stays.
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
Too many open files|raise the file limit: ulimit -n 10240, then re-run'

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

if command -v npm >/dev/null; then ok "npm $(npm --version) at $(command -v npm)"
else problem "npm is not on PATH" "brew install node, then open a new terminal"; fi

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
project_b_go = "$GO"
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
[ "$EXTENSIONS" = 1 ] && echo "project_b_extensions = true" >> "$OUT/args.gn"

echo "=== $(date '+%F %T') gn gen"
gn gen "$OUT" 2>&1 | tee "$OUT/gn.log"
[ "${PIPESTATUS[0]}" -eq 0 ] || { explain "$OUT/gn.log"; echo "RESULT=failed gn"; exit 1; }

echo "=== $(date '+%F %T') build $TARGETS"
START=$(date +%s)
autoninja -C "$OUT" -k 0 $TARGETS
RC=$?
echo "=== $(date '+%F %T') build rc=$RC in $(( $(date +%s) - START ))s"
if [ "$RC" -ne 0 ]; then
  # siso keeps every failed step's output here, so the build's own output stays as it is.
  FAILED=$(grep -c '^FAILED:' "$OUT/siso_output" 2>/dev/null)
  echo "=== ${FAILED:-0} failed step(s); their output is in $SRC/$OUT/siso_output"
  explain "$OUT/siso_output"
  echo "RESULT=failed build"
  exit 1
fi

# One folder per page: shell/shell/, shell/settings/.
rm -rf "$OUT/shell"
cp -R "$ROOT/shell/dist" "$OUT/shell"
ls -la "$OUT/project_b" "$OUT/project_b_helper" "$OUT/libproject_b_content.dylib" 2>/dev/null
echo "RESULT=ok $(date '+%F %T')"
