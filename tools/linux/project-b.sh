#!/bin/sh
# Starts the app through zypak: our project_b, which picks the display itself (PBInit), or the
# spike's content_shell. With PB_HELPER=1 content_shell's children started from a separate
# helper binary.
dir=/app/project-b
if [ -x "$dir/project_b" ]; then
  exec zypak-wrapper "$dir/project_b" "$@"
fi
if [ "$PB_HELPER" = 1 ]; then
  set -- --browser-subprocess-path="$dir/project-b-helper" "$@"
fi
exec zypak-wrapper "$dir/content_shell" --ozone-platform-hint=auto "$@"
