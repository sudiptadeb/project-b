#!/bin/bash
# One time: installs Tart and the VNC client's Python under ~/PW_TOOLS/macos-vm, pulls the
# macOS image and makes the VM. Needs no sudo. Safe to run again.
set -eu
. "$(dirname "$0")/vars.sh"

mkdir -p "$VM_HOME/bin" "$VM_HOME/dl" "$VM_HOME/shots" "$SHARE" "$TART_HOME"
echo "TART_HOME=$TART_HOME" > "$VM_HOME/vm.env"

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
    VIRTUAL_ENV="$VM_HOME/venv" uv pip install numpy pillow cryptography
fi

if ! "$TART" list -q | grep -qx "$VM"; then
    "$TART" clone "$IMAGE" "$VM"
    "$TART" set "$VM" --cpu "$CPUS" --memory "$MEMORY_MB" --display "$DISPLAY_SIZE" --no-display-refit
fi
"$TART" list
