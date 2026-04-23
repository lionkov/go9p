#!/usr/bin/env bash
set -euo pipefail

echo "[guest] running inside chroot: $(uname -rm)"

SERVER_ADDR="${KERNEL9P_TCP_ADDR:-10.0.2.2}"
SERVER_PORT="${KERNEL9P_TCP_PORT:-564}"
# The virtio-9p "hostshare" mount is the container root; we may not be able to
# create new top-level directories there. /tmp should be writable.
MNT="${KERNEL9P_MOUNT:-/tmp/kernel9p-mnt}"

mkdir -p "${MNT}"

echo "[guest] mounting kernel 9p client via tcp ${SERVER_ADDR}:${SERVER_PORT} -> ${MNT}"
mount -t 9p -o "trans=tcp,version=9p2000.L,msize=262144,port=${SERVER_PORT}" "${SERVER_ADDR}" "${MNT}"

echo "[guest] running kernel9p-e2e against mount ${MNT}"
KERNEL9P_SERVER=go9p-ufs KERNEL9P_TCP_ADDR="${SERVER_ADDR}" KERNEL9P_TCP_PORT="${SERVER_PORT}" KERNEL9P_MOUNT="${MNT}" /opt/v9fs/kernel9p-e2e

echo "[guest] PASS"
exit 0

