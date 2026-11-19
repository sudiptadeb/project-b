#!/bin/sh
# Starts content_shell through zypak. With PB_HELPER=1 the children start from a separate
# helper binary, as our app's do, instead of from content_shell re-running itself.
dir=/app/project-b
if [ "$PB_HELPER" = 1 ]; then
  set -- --browser-subprocess-path="$dir/project-b-helper" "$@"
fi
exec zypak-wrapper "$dir/content_shell" --ozone-platform-hint=auto "$@"
