#!/bin/bash
# Puts this checkout's clang on the remote build Mac, where the workers run it. Run it on
# the laptop, before a remote build (docs/remote-build.md). Safe to re-run: it copies only when the
# server's copy is missing or differs.
#
# Why: macOS scans an executable the first time it runs from a new file, and Chromium's
# clang is 100 MB, so that takes over a minute. A step that brings clang with its inputs
# gets a new file every time, and spends that minute before compiling anything. So the
# workers run clang from one folder on their own disk instead, scanned once, holding this
# checkout's exact binary: the folder is named by Chromium's clang package version and
# its hash is checked against ours. tools/remote/siso/main.star points compiles there.
#
# PB_RBE_SSH is the ssh host of the build Mac (default debmin).
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
SRC=$ROOT/chromium/src
LLVM=$SRC/third_party/llvm-build/Release+Asserts
HOST=${PB_RBE_SSH:-debmin}
# Must match _TOOLCHAINS in tools/remote/siso/main.star.
TOOLCHAINS=/Users/Shared/pb-rbe/toolchains

version() {
  local rev sub
  rev=$(sed -n "s/^CLANG_REVISION = '\(.*\)'$/\1/p" "$SRC/tools/clang/scripts/update.py")
  sub=$(sed -n 's/^CLANG_SUB_REVISION = \([0-9]*\)$/\1/p' "$SRC/tools/clang/scripts/update.py")
  [ -n "$rev" ] && [ -n "$sub" ] || { echo "toolchain: can't read clang's version from update.py" >&2; exit 1; }
  echo "$rev-$sub"
}

VERSION=$(version)
DEST=$TOOLCHAINS/$VERSION
HASH=$(shasum -a 256 "$LLVM/bin/clang" | cut -d' ' -f1)

if [ "$(ssh "$HOST" "shasum -a 256 '$DEST/bin/clang' 2>/dev/null | cut -d' ' -f1")" = "$HASH" ]; then
  echo "  ok   clang $VERSION on $HOST"
  exit 0
fi

echo "  copying clang $VERSION to $HOST:$DEST"
ssh "$HOST" "mkdir -p '$DEST.partial/bin' '$DEST.partial/lib'"
rsync -a "$LLVM/bin/clang" "$LLVM/bin/clang++" "$HOST:$DEST.partial/bin/"
rsync -a "$LLVM/lib/clang" "$HOST:$DEST.partial/lib/"
# Check the copy, run it once so macOS scans it now rather than in the first build, and
# only then put it where the workers look.
ssh "$HOST" "set -e
  [ \"\$(shasum -a 256 '$DEST.partial/bin/clang' | cut -d' ' -f1)\" = '$HASH' ]
  '$DEST.partial/bin/clang' --version > /dev/null
  rm -rf '$DEST' && mv '$DEST.partial' '$DEST'"
echo "  ok   clang $VERSION on $HOST"
