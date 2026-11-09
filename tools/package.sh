#!/bin/bash
# Packs a built app into "Project B.app" inside build/project-b.dmg. Flat on purpose.
# OUT=out/rel tools/package.sh packs the release build instead.
set -euo pipefail

ROOT=$HOME/Official/workspace/ulaa/project-b
OUT=${OUT:-out/pb}
FROM=$ROOT/chromium/src/$OUT
STAGE=$ROOT/build/package
DMG=$ROOT/build/project-b.dmg
APP="$STAGE/Project B.app"
MACOS=$APP/Contents/MacOS
RESOURCES=$APP/Contents/Resources
LIBRARIES=$APP/Contents/Libraries
SIGNING=$HOME/PW_TOOLS/project-b-signing     # tools/signing-setup.sh makes it

rm -rf "$STAGE" "$DMG"
mkdir -p "$MACOS" "$RESOURCES/shell" "$LIBRARIES"

cp "$FROM/project_b" "$FROM/project_b_helper" "$MACOS/"
cp "$FROM/project_b.pak" "$FROM/icudtl.dat" "$FROM/snapshot_blob.bin" "$FROM"/v8_context_snapshot.*.bin "$RESOURCES/"
cp "$FROM"/shell/* "$RESOURCES/shell/"
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
  <key>CFBundleShortVersionString</key><string>0.1</string>
  <key>CFBundleVersion</key><string>155.0.8059.41</string>
  <key>LSMinimumSystemVersion</key><string>13.0</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>NSSupportsAutomaticGraphicsSwitching</key><true/>
</dict>
</plist>
PLIST

# Symbol names are about a third of a non-official binary and do nothing at run time.
# Stripping breaks the linker's signature. rcodesign signs everything in the bundle again.
strip -x "$MACOS/project_b" "$MACOS/project_b_helper" "$LIBRARIES"/*.dylib 2>/dev/null
"$SIGNING/rcodesign" sign \
    --p12-file "$SIGNING/identity.p12" \
    --p12-password-file <(grep '^PROJECT_B_SIGNING_P12_PASSWORD=' "$HOME/PW_TOOLS/lab-secrets.env" | cut -d= -f2- | tr -d '\n') \
    --timestamp-url none \
    --binary-identifier "Contents/MacOS/project_b_helper:dev.projectb.helper" \
    "$APP"
codesign --verify --deep --strict "$APP"
codesign --verify -R="certificate leaf = H\"$(cat "$SIGNING/identity.sha1")\"" "$APP"

ln -s /Applications "$STAGE/Applications"
hdiutil create -quiet -volname "Project B" -srcfolder "$STAGE" -format ULMO -ov "$DMG"
hdiutil verify -quiet "$DMG"

du -sh "$APP" "$DMG"
echo "RESULT=ok $DMG"
