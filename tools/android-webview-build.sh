#!/bin/bash
# Builds build/android/project-b-webview.apk: the Go core and the Vue shell on Android System
# WebView, signed with our stable key. Flat on purpose, no Gradle; safe to re-run.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SDK=$HOME/Library/Android/sdk
NDK=$SDK/ndk/29.0.14206865
BUILD_TOOLS=$SDK/build-tools/35.0.0
ANDROID_JAR=$SDK/platforms/android-35/android.jar
MIN_SDK=26
TARGET_SDK=34
CC=$NDK/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android$MIN_SDK-clang
# Phones with 16 KB memory pages refuse libraries aligned for 4 KB.
PAGE_FLAG=-Wl,-z,max-page-size=16384

SRC=$ROOT/engine/webview
OUT=$ROOT/build/android-webview
LIBS=$OUT/apk/lib/arm64-v8a
APK=$ROOT/build/android/project-b-webview.apk

SECRETS=$HOME/PW_TOOLS/lab-secrets.env
KEYSTORE=$HOME/PW_TOOLS/project-b-signing/android.keystore
KEY_ALIAS=project-b

VERSION_CODE=$(git -C "$ROOT" rev-list --count HEAD)
VERSION_NAME=0.1.$VERSION_CODE-webview

rm -rf "$OUT"
mkdir -p "$LIBS" "$OUT/classes" "$OUT/res" "$OUT/apk/assets/shell" "$(dirname "$APK")"

echo "=== shell"
(cd "$ROOT/shell" && { [ -d node_modules ] || npm ci --silent; } && npm run build --silent)
cp "$ROOT"/shell/dist/* "$OUT/apk/assets/shell/"

echo "=== go core"
(cd "$ROOT/core" && CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC=$CC go build \
  -buildmode=c-shared -trimpath \
  -ldflags "-s -w -X main.engineName=webview -extldflags=$PAGE_FLAG" \
  -o "$LIBS/libcore.so" ./cmd/lib)

echo "=== jni"
"$CC" -shared -fPIC -O2 -Wall -Werror $PAGE_FLAG \
  -I"$LIBS" -I"$ROOT/engine/abi" \
  "$SRC/jni.c" -L"$LIBS" -lcore -o "$LIBS/libproject_b_jni.so"
rm "$LIBS/libcore.h"

echo "=== java"
javac --release 11 -classpath "$ANDROID_JAR" -d "$OUT/classes" "$SRC"/java/dev/projectb/webview/*.java
"$BUILD_TOOLS/d8" --release --min-api $MIN_SDK --lib "$ANDROID_JAR" --output "$OUT/apk" "$OUT"/classes/dev/projectb/webview/*.class

echo "=== apk"
"$BUILD_TOOLS/aapt2" compile --dir "$SRC/res" -o "$OUT/res.zip"
"$BUILD_TOOLS/aapt2" link -o "$OUT/base.apk" -I "$ANDROID_JAR" \
  --manifest "$SRC/AndroidManifest.xml" \
  --min-sdk-version $MIN_SDK --target-sdk-version $TARGET_SDK \
  --version-code "$VERSION_CODE" --version-name "$VERSION_NAME" \
  "$OUT/res.zip"
# Libraries are stored, not compressed, so Android can map them straight from the APK.
(cd "$OUT/apk" && zip -q "$OUT/base.apk" classes.dex assets/shell/* && zip -q -0 "$OUT/base.apk" lib/arm64-v8a/*)
"$BUILD_TOOLS/zipalign" -f -P 16 4 "$OUT/base.apk" "$OUT/aligned.apk"

echo "=== sign"
PROJECT_B_ANDROID_KEYSTORE_PASSWORD=$(grep '^PROJECT_B_ANDROID_KEYSTORE_PASSWORD=' "$SECRETS" | cut -d= -f2-)
export PROJECT_B_ANDROID_KEYSTORE_PASSWORD
"$BUILD_TOOLS/apksigner" sign --ks "$KEYSTORE" --ks-key-alias "$KEY_ALIAS" \
  --ks-pass env:PROJECT_B_ANDROID_KEYSTORE_PASSWORD --out "$APK" "$OUT/aligned.apk"
"$BUILD_TOOLS/apksigner" verify --min-sdk-version $MIN_SDK "$APK"

ls -la "$APK" "$LIBS"
echo "RESULT=ok $VERSION_NAME"
