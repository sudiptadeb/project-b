#!/bin/bash
# Gathers what the Flatpak needs from out/linux-<cpu> in container pb into one tarball:
#   tools/linux/stage.sh [arm64|x64] <dest.tar.gz on the host> [project_b|content_shell]
# The tarball unpacks into stage/, beside the manifest: stage/files is the app's run files.
# Our app is project_b and the Go core beside it, libcore.so, as built by tools/build.sh linux.
# The spike's content_shell came with project-b-helper, a copy of it under another name that
# played a separate helper binary.
cpu=${1:-arm64}; dest=${2:?usage: $0 [arm64|x64] dest.tar.gz [project_b|content_shell]}
app=${3:-project_b}
case $app in project_b|content_shell) ;; *) echo "no app $app"; exit 2 ;; esac
here=$(cd "$(dirname "$0")" && pwd)
export DOCKER_HOST=unix://$HOME/.colima/pb-android/docker.sock
docker exec pb bash -c "set -e
cd /work/chromium/src/out/linux-$cpu
s=/work/linux-stage/$cpu; rm -rf \"\${s:?}\"; mkdir -p \$s/stage/files
cp -a $app chrome_crashpad_handler *.pak *.bin *.dat *.so *.so.* vk_swiftshader_icd.json \$s/stage/files/ 2>/dev/null || true
[ -d locales ] && cp -a locales \$s/stage/files/
# Build and test leftovers: link tables, Vulkan's validation layer and mock driver, the trace
# processor, and the Qt shims (we draw our own UI and never load Qt).
cd \$s/stage/files && rm -f *.TOC libVkICD_mock_icd.so libVkLayer_khronos_validation.so libtest_trace_processor.so libqt5_shim.so libqt6_shim.so ui_test.pak
[ $app = content_shell ] && cp content_shell \$s/stage/files/project-b-helper
[ $app = project_b ] && rm -f \$s/stage/files/content_shell.pak
true
ls \$s/stage/files" || exit 1
for f in project-b.sh io.github.sudiptadeb.project-b.desktop icon.svg; do
  docker cp "$here/$f" "pb:/work/linux-stage/$cpu/stage/$f"
done
# The metainfo carries the release: VERSION and today's date.
meta=io.github.sudiptadeb.project-b.metainfo.xml
sed -e "s/@VERSION@/$(tr -d '[:space:]' < "$here/../../VERSION")/" -e "s/@DATE@/$(date +%F)/" "$here/$meta" |
  docker exec -i pb sh -c "cat > /work/linux-stage/$cpu/stage/$meta"
docker exec pb tar -C /work/linux-stage/$cpu -czf - stage > "$dest"
ls -l "$dest"
