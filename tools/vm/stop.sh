#!/bin/bash
# Shuts the VM down so it stops using memory. The disk stays.
set -eu
. "$(dirname "$0")/vars.sh"

# The amarit unit stays, exited, for start.sh to restart.
"$TART" stop "$VM" --timeout 60 || true
rm -f "$VNC_URL_FILE"
"$TART" list
