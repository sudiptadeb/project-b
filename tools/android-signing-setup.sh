#!/bin/bash
# Makes the one stable Android signing key, once. Losing it forces every phone to uninstall
# before it can update. The password goes into lab-secrets.env and nowhere else.
set -euo pipefail

SECRETS=$HOME/PW_TOOLS/lab-secrets.env
KEYSTORE=$HOME/PW_TOOLS/project-b-signing/android.keystore
KEY_ALIAS=project-b

[ ! -e "$KEYSTORE" ] || { echo "$KEYSTORE already exists, leaving it alone"; exit 0; }

mkdir -p "$(dirname "$KEYSTORE")"
PROJECT_B_ANDROID_KEYSTORE_PASSWORD=$(openssl rand -hex 24)
export PROJECT_B_ANDROID_KEYSTORE_PASSWORD

keytool -genkeypair -keystore "$KEYSTORE" -storetype PKCS12 \
  -storepass:env PROJECT_B_ANDROID_KEYSTORE_PASSWORD \
  -alias "$KEY_ALIAS" -keyalg RSA -keysize 4096 -validity 36500 \
  -dname "CN=Project B, O=Project B"
chmod 600 "$KEYSTORE"

{
  echo "# Project B Android APK signing, keystore at $KEYSTORE (made by tools/android-signing-setup.sh)"
  echo "PROJECT_B_ANDROID_KEYSTORE_PASSWORD=$PROJECT_B_ANDROID_KEYSTORE_PASSWORD"
} >> "$SECRETS"
echo "made $KEYSTORE"
