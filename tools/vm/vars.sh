# Shared by the shell scripts in this folder. Nothing here is a secret.
VM_HOME="$HOME/PW_TOOLS/macos-vm"
TART_VERSION=2.41.1
TART="$VM_HOME/bin/tart.app/Contents/MacOS/tart"
PY="$VM_HOME/venv/bin/python"
VM=pb-test
IMAGE=ghcr.io/cirruslabs/macos-tahoe-base:latest
CPUS=8
MEMORY_MB=16384
DISPLAY_SIZE=1440x900pt
UNIT=macos-vm
VNC_URL_FILE="$VM_HOME/vnc-url"
SHARE="$VM_HOME/share"
MIN_FREE_GB=100

export TART_HOME="$VM_HOME/tart-home"
GUEST_LOGIN=admin:admin   # The base image's public default.
