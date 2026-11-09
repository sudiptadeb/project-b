#!/bin/bash
# Makes the one signing key for Project B builds, fetches the tool that signs with it, and
# writes the profile a Mac installs once to let builds signed by that key open. It never
# replaces an existing key, because a new key means every Mac needs a new profile.
#
# The key stays on this Mac, in identity.p12. Its password is PROJECT_B_SIGNING_P12_PASSWORD
# in lab-secrets.env. The certificate is only ever compared by hash: it isn't a CA, and
# nothing asks a Mac to trust it for anything else.
#
# rcodesign signs because Apple's codesign won't use a key macOS doesn't trust, and adding
# that trust needs a password prompt, which a headless Mac can't show.
set -euo pipefail

DIR=$HOME/PW_TOOLS/project-b-signing
SECRETS=$HOME/PW_TOOLS/lab-secrets.env
PROFILE=$DIR/project-b-trust.mobileconfig
OPENSSL=/usr/bin/openssl
RCODESIGN_VERSION=0.29.0
RCODESIGN_URL=https://github.com/indygreg/apple-platform-rs/releases/download/apple-codesign/$RCODESIGN_VERSION/apple-codesign-$RCODESIGN_VERSION-macos-universal.tar.gz

mkdir -p "$DIR"
chmod 700 "$DIR"
umask 077

if [ -e "$DIR/identity.p12" ]; then
    echo "Keeping the existing key in $DIR"
else
    if ! grep -q '^PROJECT_B_SIGNING_P12_PASSWORD=' "$SECRETS"; then
        {
            printf '\n# Password of the Project B code signing key (%s).\n' "$DIR/identity.p12"
            printf '# Source: project-b/tools/signing-setup.sh. Created %s.\n' "$(date +%F)"
            printf 'PROJECT_B_SIGNING_P12_PASSWORD=%s\n' "$($OPENSSL rand -hex 24)"
        } >> "$SECRETS"
    fi
    PASSWORD=$(grep '^PROJECT_B_SIGNING_P12_PASSWORD=' "$SECRETS" | cut -d= -f2-)

    cat > "$DIR/cert.conf" <<CONF
[req]
distinguished_name = dn
x509_extensions = ext
prompt = no
[dn]
CN = Project B Code Signing
[ext]
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature
extendedKeyUsage = critical,codeSigning
CONF
    $OPENSSL req -x509 -newkey rsa:2048 -nodes -days 3650 -config "$DIR/cert.conf" \
        -keyout "$DIR/key.pem" -out "$DIR/cert.pem" 2>/dev/null
    $OPENSSL pkcs12 -export -inkey "$DIR/key.pem" -in "$DIR/cert.pem" -out "$DIR/identity.p12" \
        -name "Project B Code Signing" -passout "pass:$PASSWORD"
    rm -f "$DIR/key.pem" "$DIR/cert.conf"
    echo "Created a new key in $DIR"
fi

$OPENSSL x509 -in "$DIR/cert.pem" -noout -fingerprint -sha1 | cut -d= -f2 | tr -d : | tr 'A-F' 'a-f' > "$DIR/identity.sha1"
SHA1=$(cat "$DIR/identity.sha1")

if [ ! -x "$DIR/rcodesign" ]; then
    T=$(mktemp -d)
    curl -fsSL -o "$T/r.tar.gz" "$RCODESIGN_URL"
    curl -fsSL -o "$T/r.sha256" "$RCODESIGN_URL.sha256"
    [ "$(shasum -a 256 "$T/r.tar.gz" | cut -d' ' -f1)" = "$(cut -d' ' -f1 "$T/r.sha256")" ]
    tar xzf "$T/r.tar.gz" -C "$T"
    install -m 700 "$(find "$T" -type f -name rcodesign | head -1)" "$DIR/rcodesign"
    rm -rf "$T"
fi

if [ ! -e "$PROFILE" ]; then
    cat > "$PROFILE" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>PayloadType</key><string>Configuration</string>
  <key>PayloadVersion</key><integer>1</integer>
  <key>PayloadIdentifier</key><string>dev.projectb.trust</string>
  <key>PayloadUUID</key><string>$(uuidgen)</string>
  <key>PayloadDisplayName</key><string>Project B: allow builds signed by its key</string>
  <key>PayloadDescription</key><string>Lets apps signed by the Project B key open on this Mac. Nothing else changes. Remove this profile to undo it.</string>
  <key>PayloadScope</key><string>System</string>
  <key>PayloadContent</key>
  <array>
    <dict>
      <key>PayloadType</key><string>com.apple.systempolicy.rule</string>
      <key>PayloadVersion</key><integer>1</integer>
      <key>PayloadIdentifier</key><string>dev.projectb.trust.execute</string>
      <key>PayloadUUID</key><string>$(uuidgen)</string>
      <key>PayloadDisplayName</key><string>Project B Gatekeeper rule</string>
      <key>Comment</key><string>Project B</string>
      <key>OperationType</key><string>operation:execute</string>
      <key>Priority</key><real>101</real>
      <key>Requirement</key><string>certificate leaf = H"$SHA1"</string>
    </dict>
  </array>
</dict>
</plist>
PLIST
    plutil -lint "$PROFILE"
fi

echo "Identity $SHA1"
echo "Profile $PROFILE"
