#!/bin/bash
# Starts the VM with deskrig, in a window on this Mac, and waits for its desktop. deskrig sends
# input and takes shots inside the VM through Tart's guest agent, and reads frames from the
# window (docs/vm-testing.md, Pictures). Nothing listens on this Mac's network. A VM that
# already runs is left alone.
set -eu
. "$(dirname "$0")/vars.sh"

command -v deskrig > /dev/null || {
    echo "deskrig isn't installed: curl -fsSL https://raw.githubusercontent.com/sudiptadeb/deskrig/main/install.sh | sh"
    exit 1
}
deskrig vm create "$VM" > /dev/null  # Makes the Tart VM a deskrig target; nothing if it is one.
deskrig vm start "$VM"
