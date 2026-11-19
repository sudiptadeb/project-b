#!/bin/bash
# Runs inside the Ubuntu 24.04 arm64 VM, once, as root: GNOME on Wayland with auto-login,
# Flatpak, flatpak-builder, Flathub, and the runtime, SDK and base app the manifest names.
set -euo pipefail
user=${1:-admin}
export DEBIAN_FRONTEND=noninteractive
# Our network lets only 443 out, and has no IPv6: apt over https, IPv4 only.
sed -i 's#http://ports.ubuntu.com#https://ports.ubuntu.com#g' /etc/apt/sources.list.d/*.sources \
  /etc/cloud/templates/sources.list.ubuntu.deb822.tmpl 2>/dev/null || true
echo 'Acquire::ForceIPv4 "true";' > /etc/apt/apt.conf.d/99ipv4
apt-get update
apt-get install -y --no-install-recommends ubuntu-desktop-minimal gnome-terminal \
  flatpak flatpak-builder xdg-desktop-portal-gnome cups
# The desktop brings a netplan file that hands the network to NetworkManager, which
# --no-install-recommends leaves out, so the VM would boot with no network: mask it.
ln -sf /dev/null /etc/netplan/00-network-manager-all.yaml
# Its update pop-up would cover the screenshots.
apt-get purge -y update-notifier || true
# Log the user straight in to GNOME on Wayland.
mkdir -p /etc/gdm3
cat > /etc/gdm3/custom.conf <<CONF
[daemon]
AutomaticLoginEnable=true
AutomaticLogin=$user
WaylandEnable=true
CONF
# No screen lock or blanking: the screenshots need the desktop.
mkdir -p /etc/dconf/profile /etc/dconf/db/local.d
printf 'user-db:user\nsystem-db:local\n' > /etc/dconf/profile/user
cat > /etc/dconf/db/local.d/00-pb <<CONF
[org/gnome/desktop/session]
idle-delay=uint32 0
[org/gnome/desktop/screensaver]
lock-enabled=false
[org/gnome/desktop/lockdown]
disable-lock-screen=true
[org/gnome/settings-daemon/plugins/power]
sleep-inactive-ac-type='nothing'
CONF
dconf update
systemctl set-default graphical.target
flatpak remote-add --if-not-exists flathub https://dl.flathub.org/repo/flathub.flatpakrepo
flatpak install -y --noninteractive flathub \
  org.freedesktop.Platform//26.08 org.freedesktop.Sdk//26.08 org.electronjs.Electron2.BaseApp//26.08
echo "=== guest setup done"
