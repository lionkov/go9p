#!/usr/bin/env bash
set -euo pipefail

# Run go9p kernel-client e2e against a chosen example filesystem server.
# This script is meant to run inside ghcr.io/v9fs/docker:v2.0.0.

FS="${1:-${KERNEL9P_FS:-ufs}}"

VMLINUX_TAG="${V9FS_TEST_KERNEL_TAG:-kernel-main}"
KERNEL_IMAGE="${KERNEL_IMAGE:-/opt/v9fs/Image}"
INITRD="${INITRD:-/opt/v9fs/initrd-go9p.cpio}"
QEMULOG="${QEMULOG:-/opt/v9fs/qemu.log}"
PIDFILE="${PIDFILE:-/opt/v9fs/qemu.pid}"

echo "[host] fs=${FS}"
echo "[host] fetching kernel Image (${VMLINUX_TAG})"
curl -fsSL "https://github.com/v9fs/test/releases/download/${VMLINUX_TAG}/Image" -o "${KERNEL_IMAGE}"

echo "[host] building go9p binaries"
cd /opt/v9fs/go9p
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -buildvcs=false -o /opt/v9fs/kernel9p-e2e ./cmd/kernel9p-e2e
go build -buildvcs=false -o "/opt/v9fs/go9p-${FS}" "./p/srv/examples/${FS}"

echo "[host] building u-root initrd (uinitcmd: mount hostshare -> chroot -> guest-e2e-fs.sh)"
UROOTVERS="${UROOTVERS:-v0.16.0}"
UROOT_DIR="$(GO111MODULE=on go list -f '{{.Dir}}' -m github.com/u-root/u-root@${UROOTVERS})"
mkdir -p /opt/v9fs/uimage-go9p
cd /opt/v9fs/uimage-go9p
rm -f go.work go.work.sum "${INITRD}" || true
go work init "${UROOT_DIR}"

GOWORK=/opt/v9fs/uimage-go9p/go.work /opt/v9fs/go/bin/u-root \
  -o "${INITRD}" \
  -files /opt/v9fs/go9p/scripts/v9fs/guest-e2e-fs.sh:guest-e2e-fs.sh \
  -initcmd=/bbin/init \
  -uinitcmd="/bbin/gosh -c \"mkdir -p /mnt/9; mount -t 9p -o trans=virtio,version=9p2000.L,msize=262144 hostshare /mnt/9; KERNEL9P_FS=${FS} chroot /mnt/9 /bin/bash /opt/v9fs/go9p/scripts/v9fs/guest-e2e-fs.sh; shutdown -h now\"" \
  github.com/u-root/u-root/cmds/core/{init,gosh,mount,chroot,shutdown,poweroff,mkdir}

echo "[host] starting go9p ${FS} server on :564"
if [ "${FS}" = "ufs" ]; then
  mkdir -p /opt/v9fs/share
  chmod 0777 /opt/v9fs/share || true
  "/opt/v9fs/go9p-${FS}" -addr 0.0.0.0:564 -root /opt/v9fs/share >/dev/null 2>&1 &
else
  "/opt/v9fs/go9p-${FS}" -addr 0.0.0.0:564 >/dev/null 2>&1 &
fi
FS_PID=$!
trap 'kill ${FS_PID} >/dev/null 2>&1 || true' EXIT

echo "[host] starting QEMU"
rm -f "${PIDFILE}" || true
ARCH=aarch64 INITRD="${INITRD}" KERNEL="${KERNEL_IMAGE}" QEMULOG="${QEMULOG}" PIDFILE="${PIDFILE}" \
  /opt/v9fs/go9p/scripts/v9fs/qemu.bash

QEMUPID="$(cat "${PIDFILE}")"
echo "[host] QEMU pid=${QEMUPID}"

echo "[host] waiting for QEMU exit"
while kill -0 "${QEMUPID}" >/dev/null 2>&1; do
  sleep 2
done

echo "--- QEMU log tail (${QEMULOG}) ---"
tail -250 "${QEMULOG}" || true

grep -q "PASS: kernel9p e2e" "${QEMULOG}"
echo "[host] PASS"

