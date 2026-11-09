#!/bin/bash
# An incremental rebuild, printing only what matters. Takes under a minute.
"$HOME/Official/workspace/ulaa/project-b/tools/build.sh" 2>&1 | tr '\r' '\n' | sed 's/\x1b\[[0-9;]*m//g' | grep -E '^=== .*build rc|RESULT|error:|FAILED:|ld64|undefined|Build Succeeded' | tail -25 | cut -c1-240
