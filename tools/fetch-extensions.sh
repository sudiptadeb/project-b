#!/bin/bash
# Real extension packages for the mock store, from the Web Store's public update endpoint.
# Flat on purpose; safe to re-run. Downloads go to .tools/extensions/ and are never committed.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT=$ROOT/.tools/extensions
PROD=155.0.8059.41
UPDATE=https://clients2.google.com/service/update2/crx
IDS="
ddkjiahejlhfcafbddmgiahcphecmpfh uBlock Origin Lite
nngceckbapebfimnlniiiahkandclblb Bitwarden
glnpjglilkicbckjpbgcfkogebgllemb Okta Browser Plugin
aeblfdkhhhdcdjpifhhbdiojplfjncoa 1Password
kbfnbcaeplbcioakkpcpgfkobkghlhen Grammarly
"

mkdir -p "$OUT/crx" "$OUT/icon"

echo "$IDS" | while read -r ID LABEL; do
  [ -n "$ID" ] || continue
  XML=$(curl -fsS "$UPDATE?prodversion=$PROD&acceptformat=crx3&x=id%3D$ID%26v%3D0.0.0%26uc")
  CODEBASE=$(echo "$XML" | sed -n 's/.*codebase="\([^"]*\)".*/\1/p')
  VERSION=$(echo "$XML" | sed -n 's/.*updatecheck[^>]* version="\([^"]*\)".*/\1/p')
  FILE=$OUT/crx/${ID}_$VERSION.crx

  if [ -f "$FILE" ]; then
    echo "have  $LABEL $VERSION"
    continue
  fi

  curl -fsSL -o "$FILE.part" "$CODEBASE"
  mv "$FILE.part" "$FILE"
  echo "got   $LABEL $VERSION"
done

python3 -I "$ROOT/tools/extension_catalogue.py" "$OUT"
