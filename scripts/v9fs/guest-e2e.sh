#!/usr/bin/env bash
set -euo pipefail

echo "[guest] running inside chroot: $(uname -rm)"

KERNEL9P_FS="${KERNEL9P_FS:-ufs}" exec /opt/v9fs/go9p/scripts/v9fs/guest-e2e-fs.sh

echo "[guest] PASS"
exit 0

