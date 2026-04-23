#!/usr/bin/env bash
set -euo pipefail

# Minimal QEMU launcher based on github.com/v9fs/test/qemu.bash.
# Uses a u-root initrd which mounts hostshare and runs our tests via chroot.

ARCH="${ARCH:-$(uname -m)}"
INITRD="${INITRD:-/opt/v9fs/initrd-go9p.cpio}"
KERNEL="${KERNEL:-/opt/v9fs/Image}" # arm64 Image from v9fs/test release
LOG="${QEMULOG:-/opt/v9fs/qemu.log}"
PIDFILE="${PIDFILE:-/opt/v9fs/qemu.pid}"

if test -f "${PIDFILE}"; then
  kill "$(cat "${PIDFILE}")" || true
fi

QEMU="qemu-system-aarch64"
MACHINE="virt"
# Ensure the guest gets a usable network config for trans=tcp mounts.
APPEND="earlycon console=ttyAMA0 ip=dhcp"

"${QEMU}" -kernel \
  "${KERNEL}" \
  -cpu max \
  -machine "${MACHINE}" \
  -smp 2 \
  -m 4096m \
  -initrd "${INITRD}" \
  -object rng-random,filename=/dev/urandom,id=rng0 \
  -device virtio-rng-device,rng=rng0 \
  -device virtio-net-device,netdev=n1 \
  -netdev user,id=n1 \
  -serial file:"${LOG}" \
  -fsdev local,security_model=none,id=fsdev0,path=/ \
  -device virtio-9p-device,id=fs0,fsdev=fsdev0,mount_tag=hostshare \
  -append "${APPEND}" \
  -daemonize -display none -pidfile "${PIDFILE}"

