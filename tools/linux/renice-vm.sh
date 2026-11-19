#!/bin/bash
# Renices one VM's host process to 15: tools/linux/renice-vm.sh <a piece of its disk's path>
# nice inside a guest doesn't lower the VM on macOS, so long builds and test VMs are lowered on
# the host, beside the emulators. Every VM runs as a com.apple.Virtualization.VirtualMachine
# process whose command line doesn't name it, so the VM is found by the disk it holds open:
# colima-pb-android/ for the build container's VM, vms/pb-ubuntu/ for the Ubuntu test VM.
# Never touches another VM. Waits up to 30s for the VM to appear.
want=${1:?usage: $0 <disk path piece, e.g. colima-pb-android/>}
for i in $(seq 30); do
  for pid in $(pgrep -f com.apple.Virtualization.VirtualMachine); do
    if lsof -p "$pid" 2>/dev/null | grep -q -- "$want"; then
      [ "$(ps -o nice= -p "$pid" | tr -d ' ')" -ge 15 ] 2>/dev/null && { echo "=== VM pid $pid ($want) already at nice 15"; exit 0; }
      renice -n 15 -p "$pid" >/dev/null && { echo "=== reniced VM pid $pid ($want) to 15"; exit 0; }
      exit 1
    fi
  done
  sleep 1
done
echo "=== no running VM holds $want"; exit 1
