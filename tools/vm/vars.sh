# Shared by the shell scripts in this folder. Nothing here is a secret.
#
# PB_VM names the VM, pb-test by default. Each VM is a deskrig target of the same name, with
# its own run lock, so two can run side by side:
#   PB_VM=pb-build tools/vm/setup.sh    # once: a VM of its own from the image
#   PB_VM=pb-build tools/vm/run.sh <DMG>
VM_HOME="$HOME/PW_TOOLS/macos-vm"
TART_VERSION=2.41.1
TART="$VM_HOME/bin/tart.app/Contents/MacOS/tart"
PY="$VM_HOME/venv/bin/python"
VM=${PB_VM:-pb-test}
IMAGE=ghcr.io/cirruslabs/macos-tahoe-base:latest
CPUS=8
MEMORY_MB=16384
DISPLAY_SIZE=1440x900pt
MIN_FREE_GB=100

export TART_HOME="$VM_HOME/tart-home"
# deskrig, for runs started where ~/.local/bin isn't on PATH (amarit's units, cron).
export PATH="$PATH:$HOME/.local/bin"
export PB_VM="$VM"  # For check.py and screen.py, which the scripts start.
export DESKRIG_TARGET="$VM"  # Every deskrig command acts on the VM, never this Mac.
