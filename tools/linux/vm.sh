#!/bin/bash
# The Ubuntu test VM, pb-ubuntu, on this Mac:  tools/linux/vm.sh start|stop|ssh [cmd]|vnc <vncdo args>
# start: runs it as amarit unit pb-ubuntu-vm with Tart's VNC server and no window, renices the
#   new VM process to 15 so the emulators beside it aren't starved, and closes the Screen
#   Sharing window that `tart run --vnc` opens on the desktop by itself (only the one it opened).
# State (ssh key, vncdo venv, logs) lives in $PB_LINUX, outside the repo.
VM=pb-ubuntu
PB_LINUX=${PB_LINUX:-$HOME/PW_TOOLS/project-b-linux}
export TART_HOME=$HOME/PW_TOOLS/macos-vm/tart-home
TART=$HOME/PW_TOOLS/macos-vm/bin/tart.app/Contents/MacOS/tart
ssh_opts=(-i "$PB_LINUX/vm_key" -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR)
case ${1:-} in
start)
  if amarit ls | grep -q "^$VM-vm .*running"; then echo "already running"; "$TART" ip $VM; exit 0; fi
  shares=$(pgrep -f 'Screen Sharing.app' | sort)
  : > "$PB_LINUX/vm.log"
  # amarit run on a unit that exists starts nothing; an existing one is restarted.
  if amarit ls | grep -q "^$VM-vm "; then
    amarit restart $VM-vm
  else
    amarit run -restart never -name $VM-vm -dir "$PB_LINUX" env TART_HOME="$TART_HOME" \
      bash -c "'$TART' run --vnc-experimental $VM > vm.log 2>&1"
  fi
  for i in $(seq 30); do grep -q vnc:// "$PB_LINUX/vm.log" && break; sleep 1; done
  grep -q vnc:// "$PB_LINUX/vm.log" || { echo "the VM didn't start:"; cat "$PB_LINUX/vm.log"; exit 1; }
  "$(dirname "$0")/renice-vm.sh" vms/$VM/
  for i in $(seq 15); do
    opened=$(comm -13 <(echo "$shares") <(pgrep -f 'Screen Sharing.app' | sort))
    [ -n "$opened" ] && { kill $opened; echo "closed the Screen Sharing it opened ($opened)"; break; }
    sleep 1
  done
  for i in $(seq 60); do "$0" ssh true 2>/dev/null && break; sleep 2; done
  "$TART" ip $VM
  ;;
stop) "$TART" stop $VM ;;
ssh) shift; exec ssh "${ssh_opts[@]}" admin@"$("$TART" ip $VM)" "$@" ;;
vnc)
  shift
  url=$(grep -o 'vnc://[^ ]*' "$PB_LINUX/vm.log" | tail -1 | sed 's/\.\.\.$//')
  pass=${url#vnc://:}; pass=${pass%@*}; port=${url##*:}
  "$PB_LINUX/venv/bin/vncdo" -s "127.0.0.1::$port" -p "$pass" "$@" 2>&1 | grep -v -i -E 'tripledes|encryptor'
  ;;
*) sed -n '2,7p' "$0"; exit 2 ;;
esac
