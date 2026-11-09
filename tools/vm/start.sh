#!/bin/bash
# Starts the VM under amarit with no window and waits for its desktop. Nothing listens on this
# Mac's network: the VM's Screen Sharing and SSH are on its private address, reachable from
# this Mac only. The address goes to $VNC_URL_FILE, which rfb.py reads.
set -eu
. "$(dirname "$0")/vars.sh"

RUNNING="tart run .* $VM\$"

if ! pgrep -f "$RUNNING" > /dev/null; then
    rm -f "$VNC_URL_FILE"
    amarit run -force -name "$UNIT" -restart never -dir "$VM_HOME" -env-file "$VM_HOME/vm.env" \
        "$TART" run --no-graphics --no-audio --no-clipboard "--dir=share:$SHARE:ro" "$VM" > /dev/null
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
        echo "ready: $IP, macOS $("$TART" exec "$VM" /usr/bin/sw_vers -productVersion)"
        exit 0
    fi
    /bin/sleep 5
done
echo "the desktop did not come up"
exit 1
