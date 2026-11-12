#!/bin/bash
# One time: installs Tart and the checks' Python under ~/PW_TOOLS/macos-vm, pulls the macOS
# image, makes the VM and adds it to deskrig as a target. Needs no sudo. Safe to run again.
# deskrig itself installs with
#   curl -fsSL https://raw.githubusercontent.com/sudiptadeb/deskrig/main/install.sh | sh
#
# PB_VM names the VM, pb-test by default. Each VM is made from the image on its own, so none
# depends on another; Tart clones the image with APFS's copy-on-write.
set -eu
. "$(dirname "$0")/vars.sh"

mkdir -p "$VM_HOME/bin" "$VM_HOME/dl" "$VM_HOME/shots" "$TART_HOME"

FREE_GB=$(df -g "$VM_HOME" | awk 'END {print $4}')
if [ "$FREE_GB" -lt $((MIN_FREE_GB + 60)) ]; then
    echo "only $FREE_GB GB free; the image needs about 60 and $MIN_FREE_GB must stay free"
    exit 1
fi

if [ ! -x "$TART" ]; then
    curl -fsSL -o "$VM_HOME/dl/tart.tar.gz" "https://github.com/openai/tart/releases/download/$TART_VERSION/tart.tar.gz"
    tar -xzf "$VM_HOME/dl/tart.tar.gz" -C "$VM_HOME/bin"
fi

if [ ! -x "$PY" ]; then
    uv venv --python 3.12 "$VM_HOME/venv"
    VIRTUAL_ENV="$VM_HOME/venv" uv pip install numpy pillow
fi

if ! "$TART" list -q | grep -qx "$VM"; then
    "$TART" clone "$IMAGE" "$VM"
    "$TART" set "$VM" --cpu "$CPUS" --memory "$MEMORY_MB" --display "$DISPLAY_SIZE" --no-display-refit
fi
"$TART" list
deskrig vm create "$VM"
