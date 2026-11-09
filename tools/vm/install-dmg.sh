#!/bin/bash
# Installs a DMG, given as a path or a URL, to /Applications in the running VM:
#   tools/vm/install-dmg.sh https://.../project-b.dmg
set -eu
. "$(dirname "$0")/vars.sh"

FROM=$1
APP="Project B.app"
DMG="$SHARE/project-b.dmg"
GUEST_DMG="/Volumes/My Shared Files/share/project-b.dmg"
MOUNT=/tmp/pb-dmg

case "$FROM" in
    http*) curl -fsS -o "$DMG" "$FROM" ;;
    "$DMG") ;;
    *) cp "$FROM" "$DMG" ;;
esac

"$TART" exec "$VM" /bin/bash -c "
    set -e
    /usr/bin/killall project_b 2>/dev/null || true
    /usr/bin/hdiutil attach -quiet -nobrowse -readonly -mountpoint $MOUNT '$GUEST_DMG'
    rm -rf '/Applications/$APP'
    /usr/bin/ditto '$MOUNT/$APP' '/Applications/$APP'
    /usr/bin/hdiutil detach -quiet $MOUNT
    /usr/bin/codesign -dv '/Applications/$APP' 2>&1 | grep -E '^(Identifier|Authority|TeamIdentifier)' || true
    /usr/bin/defaults read '/Applications/$APP/Contents/Info' CFBundleVersion
"
echo "installed: $(shasum -a 256 "$DMG" | cut -c1-16)"
