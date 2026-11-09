#!/bin/bash
# Shuts the VM down so it stops using memory. The disk stays.
set -eu
. "$(dirname "$0")/vars.sh"

"$TART" stop "$VM" --timeout 60 || true
amarit rm "$UNIT" || true
rm -f "$VNC_URL_FILE"
"$TART" list
