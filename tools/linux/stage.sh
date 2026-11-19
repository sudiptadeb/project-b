#!/bin/bash
# Gathers what the Flatpak needs from out/linux-<cpu> in container pb into one tarball:
#   tools/linux/stage.sh [arm64|x64] <dest.tar.gz on the host>
# The tarball unpacks into stage/, beside the manifest: stage/files is content_shell's run
# files, plus project-b-helper, a copy of content_shell under another name that plays our
# separate helper binary.
cpu=${1:-arm64}; dest=${2:?usage: $0 [arm64|x64] dest.tar.gz}
here=$(cd "$(dirname "$0")" && pwd)
export DOCKER_HOST=unix://$HOME/.colima/pb-android/docker.sock
docker exec pb bash -c "set -e
cd /work/chromium/src/out/linux-$cpu
s=/work/linux-stage/$cpu; rm -rf \"\${s:?}\"; mkdir -p \$s/stage/files
cp -a content_shell chrome_crashpad_handler *.pak *.bin *.dat *.so *.so.* vk_swiftshader_icd.json \$s/stage/files/ 2>/dev/null || true
[ -d locales ] && cp -a locales \$s/stage/files/
# Build and test leftovers: link tables, Vulkan's validation layer and mock driver, the trace
# processor, and the Qt shims (we draw our own UI and never load Qt).
cd \$s/stage/files && rm -f *.TOC libVkICD_mock_icd.so libVkLayer_khronos_validation.so libtest_trace_processor.so libqt5_shim.so libqt6_shim.so ui_test.pak
cp content_shell \$s/stage/files/project-b-helper
ls \$s/stage/files" || exit 1
for f in project-b.sh io.github.sudiptadeb.project-b.desktop icon.svg; do
  docker cp "$here/$f" "pb:/work/linux-stage/$cpu/stage/$f"
done
docker exec pb tar -C /work/linux-stage/$cpu -czf - stage > "$dest"
ls -l "$dest"
