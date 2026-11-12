#!/bin/bash
# The remote build server: Buildbarn, run natively on the Mac that does the compiling.
# Run this on that Mac:
#
#   tools/remote/server.sh install    fetch the pinned Buildbarn binaries
#   tools/remote/server.sh start      start storage, frontend, scheduler, worker and runner
#   tools/remote/server.sh stop
#   tools/remote/server.sh status     which processes run, and whether the frontend answers
#   tools/remote/server.sh logs       the last lines of each process's log
#
# Everything listens on 127.0.0.1 only: a remote execution port runs whatever command it
# is sent, so it is never opened to the network. Builds reach it through ssh, see
# docs/remote-build.md. The config is in tools/remote/buildbarn/.
#
# PB_RBE_DIR is where binaries, caches and logs live (default ~/pb-rbe); PB_RBE_SLOTS how
# many steps run at once (default: the Mac's cores less two).
set -uo pipefail

CONFIG=$(cd "$(dirname "$0")/buildbarn" && pwd)
DIR=${PB_RBE_DIR:-$HOME/pb-rbe}
BIN=$DIR/bin
STATE=$DIR/state
LOGS=$DIR/logs

# The versions bb-deployments pins (MODULE.bazel at 7cd419f), which the config matches.
BB_STORAGE=20261102T091443Z-ca3d8b4
BB_REMOTE_EXECUTION=20261102T094740Z-c9ba98c

# name|binary|config, in start order.
PROCESSES='storage|bb_storage|storage.jsonnet
frontend|bb_storage|frontend.jsonnet
scheduler|bb_scheduler|scheduler.jsonnet
runner|bb_runner|runner.jsonnet
worker|bb_worker|worker.jsonnet'

die() { echo "server: $*" >&2; exit 1; }

install() {
  [ "$(uname -sm)" = "Darwin arm64" ] || die "the worker runs compile steps natively, so this must be an Apple silicon Mac"
  mkdir -p "$BIN"
  local repo version name
  for name in bb_storage bb_scheduler bb_worker bb_runner; do
    case $name in
      bb_storage) repo=bb-storage version=$BB_STORAGE ;;
      *) repo=bb-remote-execution version=$BB_REMOTE_EXECUTION ;;
    esac
    echo "  $name $version"
    curl -fsSL -o "$BIN/$name.download" \
      "https://github.com/buildbarn/$repo/releases/download/$version/$name.darwin_arm64" \
      || die "could not download $name"
    chmod +x "$BIN/$name.download" && mv "$BIN/$name.download" "$BIN/$name"
  done
  echo "$BB_STORAGE $BB_REMOTE_EXECUTION" > "$BIN/VERSIONS"
  echo "installed in $BIN"
}

pid_of() { [ -f "$STATE/$1.pid" ] && kill -0 "$(cat "$STATE/$1.pid")" 2>/dev/null && cat "$STATE/$1.pid"; }

start() {
  [ "$(cat "$BIN/VERSIONS" 2>/dev/null)" = "$BB_STORAGE $BB_REMOTE_EXECUTION" ] \
    || die "the binaries are missing or not the pinned versions: run tools/remote/server.sh install"
  # The directories bb-deployments' bare launcher makes before starting.
  local d
  for d in storage-ac storage-cs storage-cms storage-fsac worker/cs; do
    mkdir -p "$STATE/$d/persistent_state"
  done
  mkdir -p "$STATE/worker/build" "$STATE/worker/nfsv4" "$LOGS"
  # The NFS server's socket gets mode 0777, so the folder around it is the access control.
  chmod 700 "$STATE/worker/nfsv4"
  # Spotlight would index every step's inputs as they appear.
  touch "$DIR/.metadata_never_index"
  unmount_build_directory

  export PWD=$STATE
  export PB_RBE_SLOTS=${PB_RBE_SLOTS:-$(( $(sysctl -n hw.ncpu) - 2 ))}
  export PB_RBE_HOST=$(hostname -s)
  local name binary config pid
  while IFS='|' read -r name binary config; do
    if pid=$(pid_of "$name"); then
      echo "  $name already runs ($pid)"
      continue
    fi
    (cd "$STATE" && exec nohup "$BIN/$binary" "$CONFIG/$config" >> "$LOGS/$name.log" 2>&1 < /dev/null) &
    echo $! > "$STATE/$name.pid"
    echo "  $name started ($!)"
    sleep 1
  done <<< "$PROCESSES"
  echo "slots: $PB_RBE_SLOTS"
  sleep 2
  status
}

stop() {
  local name binary config pid
  # Reverse order: the worker first, storage last.
  while IFS='|' read -r name binary config; do
    if pid=$(pid_of "$name"); then
      kill "$pid"
      # A worker finishes its steps before it exits; a new one started meanwhile would
      # clash with its worker IDs. Wait, then insist.
      local waited=0
      while kill -0 "$pid" 2>/dev/null && [ "$waited" -lt 60 ]; do sleep 1; waited=$((waited + 1)); done
      kill -0 "$pid" 2>/dev/null && kill -9 "$pid"
      echo "  $name stopped ($pid)"
    fi
    rm -f "$STATE/$name.pid"
  done <<< "$(echo "$PROCESSES" | tail -r)"
  sleep 2
  unmount_build_directory
}

# The worker mounts its build directory over NFS. A worker that died leaves the mount
# behind, and the next one can't mount over it.
unmount_build_directory() {
  if mount | grep -q " on $STATE/worker/build "; then
    umount -f "$STATE/worker/build" && echo "  unmounted a stale build directory"
  fi
}

status() {
  local name binary config pid down=0
  while IFS='|' read -r name binary config; do
    if pid=$(pid_of "$name"); then echo "  ok   $name ($pid)"
    else echo "  DOWN $name, see $LOGS/$name.log"; down=1; fi
  done <<< "$PROCESSES"
  if nc -z -G 3 127.0.0.1 8980 2>/dev/null; then echo "  ok   frontend answers on 127.0.0.1:8980"
  else echo "  DOWN nothing answers on 127.0.0.1:8980"; down=1; fi
  return $down
}

logs() {
  local name binary config
  while IFS='|' read -r name binary config; do
    echo "== $name"
    tail -n "${LINES:-5}" "$LOGS/$name.log" 2>/dev/null
  done <<< "$PROCESSES"
}

case "${1:-}" in
  install|start|stop|status|logs) "$1" ;;
  *) sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'; exit 2 ;;
esac
