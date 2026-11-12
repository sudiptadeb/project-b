#!/bin/bash
# Shuts the VM down so it stops using memory. The disk stays.
set -eu
. "$(dirname "$0")/vars.sh"

deskrig vm stop "$VM" || true
