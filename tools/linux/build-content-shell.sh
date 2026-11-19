#!/bin/bash
# Builds stock content_shell for Linux in container pb: tools/linux/build-content-shell.sh [arm64|x64]
# Writes out/linux-<cpu> only. Installs the sysroot and runs gn gen the first time.
# Starts the colima VM itself and renices its host process (renice-vm.sh), since nice inside the
# guest doesn't lower the VM on macOS. Holds /work/.vm-in-use.lock while building, so the APK
# script doesn't stop the VM under it. If the VM is stopped anyway, it starts it again and
# resumes (ninja keeps its progress), up to 6 times. Chromium's x64 clang under Rosetta now and
# then dies from a signal (siso reports exit=-1, after a few seconds of a step); that is retried
# too, with -k 0 so the rest of the build goes on first. A real compile error ends it.
cpu=${1:-arm64}
case $cpu in arm64|x64) ;; *) echo "usage: $0 [arm64|x64]"; exit 2;; esac
out=out/linux-$cpu
here=$(cd "$(dirname "$0")" && pwd)
export DOCKER_HOST=unix://$HOME/.colima/pb-android/docker.sock
inner="cd /work/chromium/src && export PATH=/work/depot_tools:\$PATH GODEBUG=asyncpreemptoff=1 DEPOT_TOOLS_UPDATE=0
set -e
python3 build/linux/sysroot_scripts/install-sysroot.py --arch=$cpu
if [ ! -f $out/args.gn ]; then
  mkdir -p $out
  printf '%s\n' 'target_os=\"linux\"' 'target_cpu=\"$cpu\"' 'is_debug=false' 'is_component_build=false' \
    'symbol_level=0' 'blink_symbol_level=0' 'use_remoteexec=false' 'dcheck_always_on=false' > $out/args.gn
  gn gen $out
fi
nice -n 19 autoninja -C $out -j 12 -k 0 content_shell"
rc=1
for try in 1 2 3 4 5 6; do
  echo "=== try $try start $(date '+%F %T')"
  colima start -p pb-android --activate=false >/dev/null 2>&1
  "$here/renice-vm.sh" colima-pb-android/
  docker start pb >/dev/null
  docker exec pb flock /work/.vm-in-use.lock bash -c "$inner"
  rc=$?
  echo "=== try $try end $(date '+%F %T') exit=$rc"
  [ $rc -eq 0 ] && break
  if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx pb; then
    # Every failed step died from a signal: a crash under Rosetta, worth another go.
    errs=$(docker exec pb sh -c "grep -h '^err: exit=' /work/chromium/src/$out/siso_output" | sort -u)
    if [ "$errs" = "err: exit=-1" ]; then
      echo "=== only steps killed by a signal failed (Rosetta); again"; continue
    fi
    echo "=== a real build failure: $errs"; break
  fi
  echo "=== the VM was stopped under the build; starting it again"; sleep 30
done
echo "=== end $(date '+%F %T') exit=$rc"
exit $rc
