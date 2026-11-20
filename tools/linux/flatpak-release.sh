#!/bin/bash
# The Linux release's Flatpak repository (docs/research/linux.md, 5), from what
# `tools/build.sh linux arm64` built and staged (build/linux/stage-arm64.tar.gz): built in the
# Ubuntu VM into an OSTree repository on branch stable, signed with our key, with the
# .flatpakref and .flatpakrepo that point at it and a single-file bundle, into build/linux/:
#   flatpak-repo.tar.gz   the repository, which the Pages workflow publishes at <url>/repo/
#   project-b.flatpakref  one click in GNOME Software or KDE Discover installs the app
#   project-b.flatpakrepo adds the repository itself
#   io.github.sudiptadeb.project-b.flatpak   the app in one file, deskrig's way in
#   tools/linux/flatpak-release.sh
# Needs pb-ubuntu running (tools/linux/vm.sh start) and the signing key, in the kosh at
# project-b/flatpak-repo-signing.asc. The key goes to the VM for the build only, in a GnuPG
# home of its own, and is removed after. Linux releases are built only when Sudipta asks.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
app=io.github.sudiptadeb.project-b
url=${PB_FLATPAK_URL:-https://projectb.debkosh.com}
key=${KEYKOSH_ROOT:-$HOME/mykeykosh}/project-b/flatpak-repo-signing.asc
out=$root/build/linux
stage=$out/stage-arm64.tar.gz
[ -f "$key" ] || { echo "no signing key at $key"; exit 1; }
[ -f "$stage" ] || { echo "no $stage: run tools/build.sh linux arm64 first"; exit 1; }

KEYFILE=$HOME/PW_TOOLS/project-b-linux/vm_key
TART=$HOME/PW_TOOLS/macos-vm/bin/tart.app/Contents/MacOS/tart
ip=$(TART_HOME=$HOME/PW_TOOLS/macos-vm/tart-home "$TART" ip pb-ubuntu)
o=(-i "$KEYFILE" -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=no
   -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR)

echo "=== $(date '+%F %T') copy the stage, the manifest and the key in"
ssh "${o[@]}" admin@"$ip" 'rm -rf ~/pb-release && mkdir -p ~/pb-release && chmod 700 ~/pb-release'
scp "${o[@]}" -q "$stage" "$here/$app.yml" admin@"$ip":pb-release/
scp "${o[@]}" -q "$key" admin@"$ip":pb-release/key.asc

echo "=== $(date '+%F %T') build and sign the repository"
ssh "${o[@]}" admin@"$ip" "bash -s" <<GUEST
set -euo pipefail
cd ~/pb-release
trap 'rm -rf ~/pb-release/gnupg ~/pb-release/key.asc' EXIT
export GNUPGHOME=~/pb-release/gnupg; mkdir -m 700 "\$GNUPGHOME"
gpg --batch --quiet --import key.asc
fpr=\$(gpg --list-secret-keys --with-colons | awk -F: '/^fpr/{print \$10; exit}')
tar -xzf stage-arm64.tar.gz
flatpak-builder --force-clean --default-branch=stable --repo=repo \
  --gpg-sign=\$fpr --gpg-homedir="\$GNUPGHOME" build-dir $app.yml >build.log 2>&1 || { tail -30 build.log; exit 1; }
flatpak build-update-repo --gpg-sign=\$fpr --gpg-homedir="\$GNUPGHOME" --generate-static-deltas \
  --title="Project B" --default-branch=stable repo
flatpak build-bundle --gpg-sign=\$fpr --gpg-homedir="\$GNUPGHOME" --repo-url=$url/repo/ \
  repo $app.flatpak $app stable
pub=\$(gpg --export \$fpr | base64 -w0)
cat > project-b.flatpakref <<REF
[Flatpak Ref]
Name=$app
Branch=stable
Title=Project B (preview)
Url=$url/repo/
RuntimeRepo=https://dl.flathub.org/repo/flathub.flatpakrepo
IsRuntime=false
Homepage=https://github.com/sudiptadeb/project-b
Comment=A browser with a shell anyone can customise (preview)
GPGKey=\$pub
REF
cat > project-b.flatpakrepo <<REPO
[Flatpak Repo]
Title=Project B
Url=$url/repo/
Homepage=https://github.com/sudiptadeb/project-b
Comment=Project B, the Linux preview
GPGKey=\$pub
REPO
tar -czf flatpak-repo.tar.gz repo
ls -l flatpak-repo.tar.gz $app.flatpak
GUEST

echo "=== $(date '+%F %T') copy the release out"
mkdir -p "$out"
for f in flatpak-repo.tar.gz project-b.flatpakref project-b.flatpakrepo $app.flatpak; do
  scp "${o[@]}" -q admin@"$ip":pb-release/$f "$out/$f"
done
ls -l "$out"
echo "RESULT=ok $(date '+%F %T')"
