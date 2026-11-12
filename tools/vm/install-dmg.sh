#!/bin/bash
# Installs a DMG, given as a path or a URL, to /Applications in the running VM, quitting the
# app first:
#   tools/vm/install-dmg.sh https://.../project-b.dmg
set -eu
. "$(dirname "$0")/vars.sh"

deskrig install "$1"
deskrig run -- /usr/bin/defaults read "/Applications/Project B.app/Contents/Info" CFBundleVersion
