#!/bin/bash
# Chromium 155 for Android, on a Linux amd64 host. Chromium only builds Android on Linux x86-64,
# so on the Mac this runs in an amd64 container under Rosetta (docs/android.md). Flat on purpose;
# safe to re-run. Runs as root inside its own throwaway container.
set -euo pipefail

TAG=155.0.8059.41
WORK=/work
DT=$WORK/depot_tools
CR=$WORK/chromium
URL=https://chromium.googlesource.com/chromium/src.git
# Port 80 is closed on the office network, and apt uses it.
PROXY=http://192.168.100.100:3128

export PATH=$DT:/usr/local/go/bin:/usr/sbin:/usr/bin:/sbin:/bin
# Rosetta deadlocks on its own lock when Go's preemption signal lands while it's translating, so
# Go tools like cipd and siso hang with every thread in the kernel's PI futex wait.
export GODEBUG=asyncpreemptoff=1
export PYTHONUNBUFFERED=1 DEPOT_TOOLS_UPDATE=0 DEBIAN_FRONTEND=noninteractive
stamp() { echo "=== $(date '+%F %T') $*"; df -h "$WORK" | tail -1; }

stamp packages
echo "Acquire::http::Proxy \"$PROXY\";" > /etc/apt/apt.conf.d/01proxy
apt-get update -qq
apt-get install -y -qq git python3 curl ca-certificates xz-utils file lsb-release sudo >/dev/null

stamp depot_tools
[ -d "$DT/.git" ] || git clone --depth 1 https://chromium.googlesource.com/chromium/tools/depot_tools.git "$DT"
"$DT/ensure_bootstrap"

mkdir -p "$CR"
cd "$CR"
cat > .gclient <<GCLIENT
solutions = [
  {
    "managed": False,
    "name": "src",
    "url": "$URL",
    "custom_deps": {},
    "custom_vars": {"checkout_pgo_profiles": False},
  }
]
target_os = ["android"]
GCLIENT

stamp "src at $TAG"
[ -d src/.git ] || { git init -q src; git -C src remote add origin "$URL"; }
git -C src fetch --depth 1 origin "+refs/tags/$TAG:refs/tags/$TAG"
git -C src checkout -q -f "refs/tags/$TAG"

stamp "gclient sync, no hooks yet"
gclient sync --no-history -D --force --nohooks

stamp "build dependencies"
export http_proxy=$PROXY
src/build/install-build-deps.sh --no-prompt --android --no-arm --no-nacl --no-chromeos-fonts >/dev/null
unset http_proxy

stamp "hooks"
gclient runhooks

stamp verify
git -C src log -1 --format='%h %s'
ls src/buildtools/linux64/gn src/third_party/llvm-build/Release+Asserts/bin/clang
du -sh src
stamp "RESULT=ok"
