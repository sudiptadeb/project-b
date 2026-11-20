#!/bin/bash
# The spike's helper experiment: content_shell with spike-helper-exe.patch, which points
# base::FILE_EXE at --browser-subprocess-path so the zygote, and all it forks, start from the
# helper. Applies the patch to the container's tree, builds out/linux-arm64, stages the result
# into <dest.tar.gz>, and reverts the patch in the same locked step, whatever the build does.
# A run killed before its revert leaves the patch in the shared tree, so each run first takes
# back a patch it finds already applied. Run it as a one-time amarit job, as build-content-shell.sh.
#   tools/linux/spike-helper-exe.sh <dest.tar.gz>
dest=${1:?usage: $0 dest.tar.gz}
here=$(cd "$(dirname "$0")" && pwd)
export DOCKER_HOST=unix://$HOME/.colima/pb-android/docker.sock
colima start -p pb-android --activate=false >/dev/null 2>&1
"$here/renice-vm.sh" colima-pb-android/
docker start pb >/dev/null
docker cp "$here/spike-helper-exe.patch" pb:/work/linux-stage/spike-helper-exe.patch
docker exec pb flock /work/.vm-in-use.lock bash -c '
cd /work/chromium/src && export PATH=/work/depot_tools:$PATH GODEBUG=asyncpreemptoff=1 DEPOT_TOOLS_UPDATE=0
p=/work/linux-stage/spike-helper-exe.patch
git apply -R --check "$p" 2>/dev/null && { git apply -R "$p" && echo "=== took back a patch left applied"; }
git apply "$p" || exit 1
trap "git apply -R \"$p\" && echo === patch reverted" EXIT
for try in 1 2 3; do
  nice -n 19 autoninja -C out/linux-arm64 -j 12 content_shell && exit 0
  grep -q "^err: exit=-1" out/linux-arm64/siso_output || exit 1
done; exit 1' || { echo "=== build failed"; exit 1; }
"$here/stage.sh" arm64 "$dest"
