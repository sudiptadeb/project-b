#!/bin/bash
# Runs the Linux app headless with our driver, at native speed: an arm64 Ubuntu 24.04 container
# in the same colima VM as container pb, which has the app pb built in out/linux-<cpu> on the
# shared /work volume, read-only here. This checkout's tracked files go in at /repo for the
# driver's scripts (colima shares only release's worktree with its VM), and the driver's
# results come back out to build/real-linux/.
#   tools/linux/headless.sh [driver parts...]     e.g. tools/linux/headless.sh driver browsing
# Each part runs as its own drive_first_app.py, one after another; none named runs them all.
#   tools/linux/headless.sh --script tools/linux/check_one_binary.py   runs that script instead
# A container gives no user namespaces, so the app runs with --no-sandbox; the sandbox is the
# Flatpak's, tested in the VMs. It holds /work/.android-build.lock while it runs, so a build
# doesn't replace the app under it. Ask the tester for a slot first.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
CPU=${CPU:-arm64}
# Its tag changes with what it installs.
IMAGE=pb-linux-run:2
export DOCKER_HOST=unix://$HOME/.colima/pb-android/docker.sock

colima start -p pb-android --activate=false >/dev/null 2>&1
"$here/renice-vm.sh" colima-pb-android/ || true
docker start pb >/dev/null

# The image: Ubuntu with what Chromium links to, fonts and Python, made once. Our network
# lets only 443 out, with no IPv6, so apt goes over https; the bare image has no CA
# certificates, so the first install, of those alone, can't check its peer.
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  docker build --platform linux/arm64 -t "$IMAGE" - <<'DOCKERFILE'
FROM ubuntu:24.04
ENV DEBIAN_FRONTEND=noninteractive
RUN sed -i 's#http://ports.ubuntu.com#https://ports.ubuntu.com#g' /etc/apt/sources.list.d/ubuntu.sources && \
    echo 'Acquire::ForceIPv4 "true";' > /etc/apt/apt.conf.d/99ipv4 && \
    apt-get -o Acquire::https::Verify-Peer=false update && \
    apt-get -o Acquire::https::Verify-Peer=false install -y --no-install-recommends ca-certificates && \
    apt-get update && \
    apt-get install -y --no-install-recommends python3 libnss3 libnspr4 libatk1.0-0t64 \
      libatk-bridge2.0-0t64 libcups2t64 libdrm2 libxkbcommon0 libxcomposite1 libxdamage1 \
      libxrandr2 libxfixes3 libgbm1 libpango-1.0-0 libcairo2 libasound2t64 libxshmfence1 \
      libglib2.0-0t64 libdbus-1-3 libexpat1 libx11-6 libxcb1 libxext6 libwayland-client0 \
      libwayland-cursor0 libwayland-egl1 fontconfig fonts-dejavu-core binutils procps && \
    rm -rf /var/lib/apt/lists/*
DOCKERFILE
fi

# Holds the build lock on pb's side for the run, so a build waits rather than relinking the
# app under it: a holder in pb keeps it until the run removes its marker.
marker=/work/.linux-headless-run
docker exec pb rm -f "$marker"
docker exec -d pb flock /work/.android-build.lock sh -c "touch $marker; while [ -e $marker ]; do sleep 1; done"
echo "=== $(date '+%F %T') waiting for the container's build lock"
until docker exec pb test -e "$marker"; do sleep 2; done
trap 'docker exec pb rm -f "$marker"' EXIT

docker rm -f pb-linux-run >/dev/null 2>&1 || true
set +e
git -C "$root" archive HEAD | docker run -i --platform linux/arm64 --name pb-linux-run --shm-size=1g \
  -v pb-work:/work:ro \
  -e PB_BIN=/work/chromium/src/out/linux-$CPU/project_b -e PB_ARGS=--no-sandbox \
  "$IMAGE" sh -c 'mkdir -p /repo && tar -x -C /repo && cd /repo || exit 2
    [ "${1:-}" = --script ] && exec python3 "$2"
    [ $# -gt 0 ] || exec python3 tools/drive_first_app.py
    rc=0
    for part in "$@"; do python3 tools/drive_first_app.py "$part" || rc=1; done
    exit $rc' \
  headless "$@"
rc=$?
set -e
rm -rf "${root:?}/build/real-linux"
mkdir -p "$root/build"
docker cp pb-linux-run:/repo/build/real "$root/build/real-linux" >/dev/null 2>&1 || true
docker rm -f pb-linux-run >/dev/null 2>&1 || true
echo "=== results in build/real-linux; rc=$rc"
exit $rc
