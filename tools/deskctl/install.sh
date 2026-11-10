#!/bin/bash
# Builds deskctl and installs it with its `desk` wrapper, signed with a local key that stays
# the same across builds. macOS ties Screen Recording and Accessibility to that key and
# this identifier, so a rebuild keeps both. The key never leaves the machine.
#
#   tools/deskctl/install.sh              # into .tools/desk in this repo
#   DESK_DIR=<dir> tools/deskctl/install.sh   # into a folder that already holds a key
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
DEST=${DESK_DIR:-$ROOT/.tools/desk}
KEY=$DEST/signing.pem
IDENTIFIER=dev.local.deskctl
RCODESIGN_VERSION=0.29.0
RCODESIGN_URL=https://github.com/indygreg/apple-platform-rs/releases/download/apple-codesign/$RCODESIGN_VERSION/apple-codesign-$RCODESIGN_VERSION-macos-universal.tar.gz
RCODESIGN=$DEST/rcodesign

mkdir -p "$DEST"
BUILD=$(mktemp -d "$DEST/build.XXXXXX")
trap 'rm -rf "$BUILD"' EXIT

# Apple's codesign won't sign with a key macOS doesn't trust, so rcodesign signs.
if [ ! -x "$RCODESIGN" ]; then
    curl -fsSL -o "$BUILD/r.tar.gz" "$RCODESIGN_URL"
    curl -fsSL -o "$BUILD/r.sha256" "$RCODESIGN_URL.sha256"
    [ "$(shasum -a 256 "$BUILD/r.tar.gz" | cut -d' ' -f1)" = "$(cut -d' ' -f1 "$BUILD/r.sha256")" ]
    tar xzf "$BUILD/r.tar.gz" -C "$BUILD"
    install -m 700 "$(find "$BUILD" -type f -name rcodesign | head -1)" "$RCODESIGN"
fi

if [ ! -f "$KEY" ]; then
    (umask 077
     openssl req -x509 -newkey rsa:2048 -nodes -days 3650 -subj "/CN=deskctl local" \
         -addext "extendedKeyUsage=codeSigning" -addext "keyUsage=critical,digitalSignature" \
         -keyout "$BUILD/key.pem" -out "$BUILD/cert.pem" 2>/dev/null
     cat "$BUILD/cert.pem" "$BUILD/key.pem" > "$KEY")
fi

# The requirement macOS checks permissions against: this name, signed by this key.
LEAF=$(openssl x509 -in "$KEY" -outform der | shasum -a 1 | cut -d' ' -f1)
csreq -r- -b "$BUILD/requirement.bin" <<< "identifier \"$IDENTIFIER\" and certificate leaf = H\"$LEAF\""

swiftc -O -o "$BUILD/deskctl" "$HERE/deskctl.swift"
codesign --remove-signature "$BUILD/deskctl"
"$RCODESIGN" sign --pem-file "$KEY" --timestamp-url none --binary-identifier "$IDENTIFIER" \
    --code-requirements-path "$BUILD/requirement.bin" "$BUILD/deskctl" >/dev/null 2>&1
codesign --verify --strict "$BUILD/deskctl"
codesign --verify -R="identifier \"$IDENTIFIER\" and certificate leaf = H\"$LEAF\"" "$BUILD/deskctl"

# Replace, don't overwrite: macOS can kill a binary rewritten in place.
mv -f "$BUILD/deskctl" "$DEST/deskctl"
cp "$HERE/desk" "$DEST/desk.new"
chmod +x "$DEST/desk.new"
mv -f "$DEST/desk.new" "$DEST/desk"
echo "installed $DEST/desk"
echo "allow $DEST/deskctl once in System Settings > Privacy & Security, under both"
echo "Screen & System Audio Recording and Accessibility; then run: $DEST/desk status"
