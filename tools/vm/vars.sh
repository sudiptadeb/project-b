# Shared by the shell scripts in this folder. Nothing here is a secret.
#
# PB_VM names the VM, pb-test by default. Each VM has its own amarit unit, Screen Sharing
# address file and shared folder, so two can run side by side:
#   PB_VM=pb-build tools/vm/setup.sh    # once: a VM of its own from the image
#   PB_VM=pb-build tools/vm/start.sh
VM_HOME="$HOME/PW_TOOLS/macos-vm"
TART_VERSION=2.41.1
TART="$VM_HOME/bin/tart.app/Contents/MacOS/tart"
PY="$VM_HOME/venv/bin/python"
VM=${PB_VM:-pb-test}
IMAGE=ghcr.io/cirruslabs/macos-tahoe-base:latest
CPUS=8
MEMORY_MB=16384
DISPLAY_SIZE=1440x900pt
UNIT=macos-vm-$VM
VNC_URL_FILE="$VM_HOME/vnc-url-$VM"
SHARE="$VM_HOME/share-$VM"
MIN_FREE_GB=100

export TART_HOME="$VM_HOME/tart-home"
# amarit, which start.sh runs the VM under, for runs started where it isn't on PATH (amarit's
# own units, cron).
export PATH="$PATH:$HOME/go/bin"
export PB_VM="$VM"  # For check.py and rfb.py, which the scripts start.
GUEST_LOGIN=admin:admin   # The base image's public default.
