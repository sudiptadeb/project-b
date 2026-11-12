#!/bin/bash
# Packs a built app into "Project B.app" inside build/project-b.dmg. Flat on purpose.
# OUT=out/rel tools/package.sh packs the release build instead.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT=${OUT:-out/pb}
FROM=$ROOT/chromium/src/$OUT
STAGE=$ROOT/build/package
DMG=$ROOT/build/project-b.dmg
APP="$STAGE/Project B.app"
MACOS=$APP/Contents/MacOS
RESOURCES=$APP/Contents/Resources
LIBRARIES=$APP/Contents/Libraries
FRAMEWORKS=$APP/Contents/Frameworks
. "$ROOT/tools/keys.sh"                       # the signing key, from tools/signing-setup.sh
need_key identity.p12
RCODESIGN=$HOME/PW_TOOLS/project-b-signing/rcodesign   # tools/signing-setup.sh fetches it
VERSION=$(tr -d '[:space:]' < "$ROOT/VERSION")   # what the installed app says it is, and compares releases against

rm -rf "$STAGE" "$DMG"
mkdir -p "$MACOS" "$RESOURCES/shell" "$LIBRARIES" "$FRAMEWORKS"

cp "$FROM/project_b" "$FROM/project_b_helper" "$MACOS/"
cp "$FROM/libproject_b_content.dylib" "$FRAMEWORKS/"
cp "$FROM/project_b.pak" "$FROM/icudtl.dat" "$FROM/snapshot_blob.bin" "$FROM"/v8_context_snapshot.*.bin "$RESOURCES/"
cp -R "$FROM"/shell/* "$RESOURCES/shell/"
cp "$FROM/libEGL.dylib" "$FROM/libGLESv2.dylib" "$FROM/libvk_swiftshader.dylib" "$FROM/libvulkan.dylib" \
   "$FROM/vk_swiftshader_icd.json" "$LIBRARIES/"

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDisplayName</key><string>Project B</string>
  <key>CFBundleExecutable</key><string>project_b</string>
  <key>CFBundleIdentifier</key><string>dev.projectb.browser</string>
  <key>CFBundleName</key><string>Project B</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleVersion</key><string>155.0.8059.41</string>
  <key>LSMinimumSystemVersion</key><string>13.0</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>NSSupportsAutomaticGraphicsSwitching</key><true/>
</dict>
</plist>
PLIST

# Symbol names are about a third of a non-official binary and do nothing at run time.
# Stripping breaks the linker's signature, and rcodesign crashes reading what's left on a
# release build, so the old signature goes first. The graphics libraries keep the linker's
# signature, because rcodesign can't parse them after a release build.
for binary in "$MACOS/project_b" "$MACOS/project_b_helper" "$FRAMEWORKS"/*.dylib; do
    codesign --remove-signature "$binary"
    strip -x "$binary" 2>/dev/null
done
"$RCODESIGN" sign \
    --p12-file "$KEYS/identity.p12" \
    --p12-password-file <(grep '^PROJECT_B_SIGNING_P12_PASSWORD=' "$HOME/PW_TOOLS/lab-secrets.env" | cut -d= -f2- | tr -d '\n') \
    --timestamp-url none \
    --exclude "Contents/Libraries/**" \
    --binary-identifier "Contents/MacOS/project_b_helper:dev.projectb.helper" \
    "$APP"
codesign --verify --deep --strict "$APP"
codesign --verify -R="certificate leaf = H\"$(cat "$KEYS/identity.sha1")\"" "$APP"

ln -s /Applications "$STAGE/Applications"
# hdiutil create -srcfolder gives up with "Resource busy" when its temporary volume is slow
# to unmount, so we mount and unmount it ourselves. Its -hfs makehybrid avoids the mount but
# adds an attribute that fails strict signature checks.
RAW=$ROOT/build/project-b-rw.dmg
MOUNT=$ROOT/build/package-mount
mkdir -p "$MOUNT"
hdiutil create -quiet -size "$(( $(du -sk "$STAGE" | cut -f1) / 1024 + 64 ))m" -fs HFS+ -volname "Project B" -ov "$RAW"
hdiutil attach -quiet -nobrowse -noverify -mountpoint "$MOUNT" "$RAW"
ditto "$STAGE" "$MOUNT"
for attempt in 1 2 3 4 5 6; do
    hdiutil detach -quiet "$MOUNT" && break
    sleep 5
done
hdiutil convert -quiet "$RAW" -format ULMO -ov -o "$DMG"
rm -f "$RAW"
rmdir "$MOUNT"
hdiutil verify -quiet "$DMG"

du -sh "$APP" "$DMG"
echo "RESULT=ok $DMG"
