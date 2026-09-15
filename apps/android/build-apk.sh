#!/usr/bin/env sh
set -eu
GRADLE_CMD="${GRADLE_CMD:-gradle}"
exec "$GRADLE_CMD" --no-daemon --console=plain clean lintRelease assembleRelease
