#!/bin/bash
# Starts the VM under amarit, in a window on this Mac's desktop, and waits for its desktop.
# Nothing listens on this Mac's network: the VM's Screen Sharing and SSH are on its private
# address, reachable from this Mac only. The address goes to $VNC_URL_FILE, which rfb.py
# reads for input. The pictures come from the window: `desk watch` keeps its latest frame in
# $FRAME_FILE, because Screen Sharing sends them seconds late (docs/vm-testing.md).
set -eu
. "$(dirname "$0")/vars.sh"

# The window needs someone at this Mac's desktop, and deskctl allowed to record it.
STATUS=$("$DESK" status 2>&1) || true
if ! grep -q "on console: 1" <<< "$STATUS" || ! grep -q "screen locked: no" <<< "$STATUS" ||
        ! grep -q "screen recording: true" <<< "$STATUS"; then
    echo "the VM's pictures need this Mac's desktop logged in and unlocked, and Screen Recording"
    echo "allowed for deskctl (tools/deskctl/README.md). $DESK status says:"
    echo "$STATUS"
    exit 1
fi

RUNNING="tart run .* $VM\$"

if ! pgrep -f "$RUNNING" > /dev/null; then
    rm -f "$VNC_URL_FILE"
    mkdir -p "$SHARE"
    # stop.sh keeps the unit: amarit run on a unit removed seconds earlier stops the new
    # process at once.
    if amarit ls | awk -v unit="$UNIT" '$1 == unit {found = 1} END {exit !found}'; then
        amarit restart "$UNIT" > /dev/null
    else
        amarit run -force -name "$UNIT" -restart never -dir "$VM_HOME" -env-file "$VM_HOME/vm.env" \
            "$TART" run --no-audio --no-clipboard "--dir=share:$SHARE:ro" "$VM" > /dev/null
    fi
    /bin/sleep 3
fi

for _ in $(seq 1 120); do
    if ! pgrep -f "$RUNNING" > /dev/null; then
        amarit logs "$UNIT" | tail -1 | cut -c1-300
        if amarit logs "$UNIT" | tail -1 | grep -q 'host key'; then
            echo "macOS refused: nobody has logged in to this Mac's desktop since it booted. See docs/vm-testing.md."
        fi
        exit 1
    fi
    if "$TART" exec "$VM" /usr/bin/pgrep -x Dock > /dev/null 2>&1; then
        IP=$("$TART" ip "$VM")
        (umask 077; echo "vnc://$GUEST_LOGIN@$IP:5900" > "$VNC_URL_FILE")
        # The watch ends by itself when the VM does; starting it again replaces it.
        rm -f "$FRAME_FILE"
        "$DESK" watch "$(pgrep -f "$RUNNING" | head -1)" "$FRAME_FILE" 86400 > /dev/null
        for _ in $(seq 1 50); do
            [ -s "$FRAME_FILE" ] && break
            /bin/sleep 0.2
        done
        if [ ! -s "$FRAME_FILE" ]; then
            echo "no pictures from the VM's window: $(cat "$FRAME_FILE.log" 2> /dev/null)"
            exit 1
        fi
        echo "ready: $IP, macOS $("$TART" exec "$VM" /usr/bin/sw_vers -productVersion)"
        exit 0
    fi
    /bin/sleep 5
done
echo "the desktop did not come up"
exit 1
