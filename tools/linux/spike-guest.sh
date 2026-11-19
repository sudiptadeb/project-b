#!/bin/bash
# The spike's steps inside the Ubuntu VM, as the desktop user (over ssh is fine):
#   spike-guest.sh build <stage.tar.gz>   unpack beside the manifest, flatpak-builder, install
#   spike-guest.sh run [args...]          start it in the user's GNOME session, log in ~/pb-run.log
#   spike-guest.sh procs                  each of its processes: type, seccomp, PID namespace
#   spike-guest.sh exes                   each process's binary, and whether it maps the main
#                                         binary or the helper (the separate-helper check)
#   spike-guest.sh stop
# PB_HELPER=1 with run starts the children from project-b-helper (see project-b.sh).
set -u
app=io.github.sudiptadeb.project-b
work=$HOME/pb-flatpak
uid=$(id -u)
export XDG_RUNTIME_DIR=/run/user/$uid
export DBUS_SESSION_BUS_ADDRESS=unix:path=$XDG_RUNTIME_DIR/bus
export WAYLAND_DISPLAY=${WAYLAND_DISPLAY:-wayland-0}
case ${1:-} in
build)
  tarball=${2:?stage tarball}
  mkdir -p "$work" && rm -rf "${work:?}/stage" "${work:?}/build-dir"
  tar -C "$work" -xzf "$tarball"
  cp "$(dirname "$0")/$app.yml" "$work/"
  cd "$work" && time flatpak-builder --user --install --force-clean build-dir $app.yml
  flatpak info --user $app | head -20
  du -sh "$work/stage/files"
  ;;
run)
  # Started as a unit of the user's systemd manager, which has the GNOME session's environment
  # (XDG_SESSION_TYPE=wayland and the rest), as a launch from the desktop would.
  # flatpak run moves the app into a scope of its own, so stopping the unit doesn't end it.
  shift
  flatpak kill $app 2>/dev/null; sleep 1
  systemctl --user stop pb-run 2>/dev/null; systemctl --user reset-failed pb-run 2>/dev/null
  systemd-run --user --unit=pb-run --collect -p StandardOutput=truncate:$HOME/pb-run.log \
    -p StandardError=file:$HOME/pb-run.log \
    flatpak run ${PB_HELPER:+--env=PB_HELPER=$PB_HELPER} ${ZYPAK_DEBUG:+--env=ZYPAK_DEBUG=$ZYPAK_DEBUG} \
    $app "$@"
  sleep "${PB_WAIT:-8}"; tail -30 "$HOME/pb-run.log"
  ;;
procs)
  self_ns=$(readlink /proc/self/ns/pid)
  printf '%-7s %-22s %-8s %-7s %-22s %-22s %s\n' PID BINARY TYPE SECCOMP PIDNS USERNS NSPID
  for p in $(pgrep -f '/app/project-b/(content_shell|project-b-helper)'); do
    [ -r /proc/$p/status ] || continue
    cmd=$(tr '\0' ' ' < /proc/$p/cmdline)
    bin=$(basename "$(echo "$cmd" | awk '{print $1}')")
    type=$(echo "$cmd" | grep -o -- '--type=[a-z-]*' | head -1 | cut -d= -f2); type=${type:-browser}
    sec=$(awk '/^Seccomp:/{print $2}' /proc/$p/status)
    nspid=$(awk '/^NSpid:/{$1=""; print}' /proc/$p/status)
    printf '%-7s %-22s %-8s %-7s %-22s %-22s %s\n' "$p" "$bin" "$type" "$sec" \
      "$(readlink /proc/$p/ns/pid)" "$(readlink /proc/$p/ns/user)" "$nspid"
  done
  echo "this shell's PID namespace: $self_ns"
  ;;
exes)
  for p in $(pgrep -f "/app/project-b/"); do
    exe=$(readlink /proc/$p/exe 2>&1)
    case $exe in */bwrap|*/bash|*/sh) continue;; esac
    main=$(grep -c " /app/project-b/content_shell$" /proc/$p/maps 2>/dev/null)
    helper=$(grep -c " /app/project-b/project-b-helper$" /proc/$p/maps 2>/dev/null)
    t=$(tr '\0' ' ' </proc/$p/cmdline | grep -o -- '--type=[a-z-]*' | head -1 | cut -d= -f2)
    sub=$(tr '\0' ' ' </proc/$p/cmdline | grep -o -- '--utility-sub-type=[a-zA-Z.]*' | cut -d= -f2)
    f=$(awk '/^Seccomp_filters/{print $2}' /proc/$p/status)
    printf '%-6s %-12s %-30s %-34s main=%-2s helper=%-2s filters=%s %s\n' "$p" "${t:-browser}" \
      "$sub" "$exe" "$main" "$helper" "$f" "$(readlink /proc/$p/ns/pid)"
  done
  ;;
stop)
  flatpak kill $app 2>/dev/null; systemctl --user stop pb-run 2>/dev/null; true
  ;;
*) sed -n "2,10p" "$0"; exit 2;;
esac
