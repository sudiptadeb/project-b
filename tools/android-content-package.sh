#!/bin/bash
# Takes the APK the Linux host built, shrinks it and signs it with our stable key, on this Mac,
# where the key lives. Flat on purpose; safe to re-run.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SDK=$HOME/Library/Android/sdk
BUILD_TOOLS=$SDK/build-tools/35.0.0
DOCKER_SOCK=$HOME/.colima/pb-android/docker.sock
CONTAINER=pb
FROM=/work/chromium/src/out/android/apks/ProjectB.apk

OUT=$ROOT/build/android
APK=$OUT/project-b-content.apk

SECRETS=$HOME/PW_TOOLS/lab-secrets.env
KEYSTORE=$HOME/PW_TOOLS/project-b-signing/android.keystore
KEY_ALIAS=project-b

mkdir -p "$OUT"
DOCKER_HOST=unix://$DOCKER_SOCK "$HOME/.local/bin/docker" cp "$CONTAINER:$FROM" "$OUT/content-unsigned.apk"
# Chromium stores libraries uncompressed. Compressing them halves the download, for a
# phone to fetch over a link, and Android unpacks them at install (extractNativeLibs in the manifest).
rm -rf "$OUT/content-libs"
mkdir -p "$OUT/content-libs"
(cd "$OUT/content-libs" && unzip -q "$OUT/content-unsigned.apk" 'lib/*' &&
  zip -q -d "$OUT/content-unsigned.apk" 'lib/*' && zip -q -9 -r "$OUT/content-unsigned.apk" lib)
rm -rf "$OUT/content-libs"
"$BUILD_TOOLS/zipalign" -f -P 16 4 "$OUT/content-unsigned.apk" "$OUT/content-aligned.apk"

PROJECT_B_ANDROID_KEYSTORE_PASSWORD=$(grep '^PROJECT_B_ANDROID_KEYSTORE_PASSWORD=' "$SECRETS" | cut -d= -f2-)
export PROJECT_B_ANDROID_KEYSTORE_PASSWORD
"$BUILD_TOOLS/apksigner" sign --ks "$KEYSTORE" --ks-key-alias "$KEY_ALIAS" \
  --ks-pass env:PROJECT_B_ANDROID_KEYSTORE_PASSWORD --out "$APK" "$OUT/content-aligned.apk"
"$BUILD_TOOLS/apksigner" verify "$APK"
rm "$OUT/content-unsigned.apk" "$OUT/content-aligned.apk"

ls -la "$APK"
echo "RESULT=ok"
