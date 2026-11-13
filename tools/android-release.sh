#!/bin/bash
# Builds and signs the APK of this checkout, for tools/release.sh: the files git tracks in the
# trees the APK takes, and the built shell, copied into the Linux VM's container and built there
# incrementally, whichever worktree the container mounts. Starts the VM, and stops it after.
# Flat on purpose.
#
#   tools/android-release.sh      # build/android/project-b-content.apk
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
export PATH=$HOME/.local/bin:/opt/homebrew/bin:$PATH
export DOCKER_HOST=unix://$HOME/.colima/pb-android/docker.sock

cd "$ROOT"
[ -f shell/dist/shell/standard/standard.js ] || { echo "build the shell first"; exit 1; }

echo "=== $(date '+%F %T') start the VM"
colima start -p pb-android --activate=false
trap 'colima stop -p pb-android' EXIT
docker start pb >/dev/null

echo "=== $(date '+%F %T') copy the tree in"
docker exec pb sh -c 'rm -rf /work/release && mkdir /work/release'
{ git ls-files -z core engine tools/android-content-build.sh VERSION; find shell/dist -type f -print0; } |
  COPYFILE_DISABLE=1 tar -c --no-xattrs --no-mac-metadata --null -T - |
  docker exec -i pb tar -x -C /work/release

echo "=== $(date '+%F %T') build"
docker exec -e REPO=/work/release pb nice -n 10 bash /work/release/tools/android-content-build.sh | tail -5

echo "=== $(date '+%F %T') sign"
tools/android-content-package.sh
