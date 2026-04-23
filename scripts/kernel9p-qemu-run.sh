#!/usr/bin/env bash
set -euo pipefail

# Runs a Linux kernel under QEMU and validates the kernel 9p client by
# mounting a 9P export and executing a small test binary inside the guest.
#
# Select backend with KERNEL9P_SERVER:
#   qemu       - QEMU virtio-9p (default)
#   diod       - external diod over TCP (runs in this container)
#   u9fs       - external u9fs over TCP via socat (runs in this container)
#   go9p-ufs   - go9p UFS server over TCP (runs in this container)
#
# The guest reads kernel9p.* parameters from /proc/cmdline (see scripts/kernel9p-init).

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${OUT_DIR:-${ROOT_DIR}/.kernel9p-out}"

KERNEL_ARCH="${KERNEL_ARCH:-amd64}"
KERNEL_BZIMAGE="${KERNEL_BZIMAGE:-${OUT_DIR}/linux/arch/x86/boot/bzImage}"
KERNEL_IMAGE="${KERNEL_IMAGE:-${OUT_DIR}/linux/arch/arm64/boot/Image}"
INITRAMFS="${INITRAMFS:-${OUT_DIR}/initramfs.cpio}"
SHARE_DIR="${SHARE_DIR:-${OUT_DIR}/share}"

KERNEL9P_SERVER="${KERNEL9P_SERVER:-qemu}" # qemu | diod | u9fs | go9p-ufs
KERNEL9P_TCP_ADDR="${KERNEL9P_TCP_ADDR:-10.0.2.2}"
KERNEL9P_TCP_PORT="${KERNEL9P_TCP_PORT:-564}"

mkdir -p "${OUT_DIR}" "${SHARE_DIR}"

case "${KERNEL_ARCH}" in
  amd64)
    QEMU_BIN="${QEMU_BIN:-qemu-system-x86_64}"
    KERNEL_PATH="${KERNEL_BZIMAGE}"
    CONSOLE="ttyS0"
    VIRTIO_9P_DEVICE=(-device virtio-9p-pci,fsdev=fsdev0,mount_tag=hostshare)
    NETDEV_ARGS=(-netdev user,id=net0 -device virtio-net-pci,netdev=net0)
    MACHINE_ARGS_BASE=(-machine q35 -cpu max -device virtio-rng-pci)
    ;;
  arm64)
    QEMU_BIN="${QEMU_BIN:-qemu-system-aarch64}"
    KERNEL_PATH="${KERNEL_IMAGE}"
    CONSOLE="ttyAMA0"
    VIRTIO_9P_DEVICE=(-device virtio-9p-device,fsdev=fsdev0,mount_tag=hostshare)
    NETDEV_ARGS=(-netdev user,id=net0 -device virtio-net-device,netdev=net0)
    MACHINE_ARGS_BASE=(-machine virt -cpu cortex-a57 -device virtio-rng-device)
    ;;
  *)
    echo "Unsupported KERNEL_ARCH=${KERNEL_ARCH} (expected amd64 or arm64)" >&2
    exit 2
    ;;
esac

if [[ ! -f "${KERNEL_PATH}" ]]; then
  echo "Missing kernel image at ${KERNEL_PATH}" >&2
  exit 2
fi
if [[ ! -f "${INITRAMFS}" ]]; then
  echo "Missing initramfs at ${INITRAMFS}" >&2
  exit 2
fi

echo "Starting QEMU kernel9p test (server=${KERNEL9P_SERVER})..."

"${QEMU_BIN}" --version >/dev/null 2>&1 || {
  echo "Missing QEMU binary at ${QEMU_BIN}" >&2
  exit 2
}

cleanup() {
  if [[ -n "${SOCAT_PID:-}" ]]; then
    kill "${SOCAT_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${DIOD_PID:-}" ]]; then
    kill "${DIOD_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${GO9P_PID:-}" ]]; then
    kill "${GO9P_PID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

KERNEL_APPEND="console=${CONSOLE} panic=1 oops=panic loglevel=7"
KERNEL_APPEND+=" kernel9p.server=${KERNEL9P_SERVER}"
KERNEL_APPEND+=" kernel9p.tcp=${KERNEL9P_TCP_ADDR}"
KERNEL_APPEND+=" kernel9p.port=${KERNEL9P_TCP_PORT}"

QEMU_ARGS=(
  -nodefaults
  -no-reboot
  -m 1024
  -smp 1
  -accel tcg
  -serial mon:stdio
  -nographic
  -kernel "${KERNEL_PATH}"
  -initrd "${INITRAMFS}"
  -append "${KERNEL_APPEND}"
  "${MACHINE_ARGS_BASE[@]}"
)

case "${KERNEL9P_SERVER}" in
  qemu)
    QEMU_ARGS+=(
      -fsdev local,id=fsdev0,path="${SHARE_DIR}",security_model=none
      "${VIRTIO_9P_DEVICE[@]}"
    )
    ;;
  diod)
    if ! command -v diod >/dev/null 2>&1; then
      echo "Missing diod; install it or use KERNEL9P_SERVER=qemu" >&2
      exit 2
    fi
    echo "Starting diod on 0.0.0.0:${KERNEL9P_TCP_PORT} exporting ${SHARE_DIR}..."
    diod --listen="0.0.0.0:${KERNEL9P_TCP_PORT}" --no-auth --export="${SHARE_DIR}" >/dev/null 2>&1 &
    DIOD_PID="$!"
    QEMU_ARGS+=("${NETDEV_ARGS[@]}")
    ;;
  u9fs)
    if ! command -v socat >/dev/null 2>&1; then
      echo "Missing socat (required for u9fs TCP mode)" >&2
      exit 2
    fi
    U9FS_BIN=/usr/local/bin/u9fs
    if [[ ! -x "${U9FS_BIN}" ]]; then
      echo "Missing u9fs at ${U9FS_BIN}" >&2
      exit 2
    fi
    echo "Starting u9fs (via socat) on 0.0.0.0:${KERNEL9P_TCP_PORT} exporting ${SHARE_DIR}..."
    socat TCP-LISTEN:"${KERNEL9P_TCP_PORT}",reuseaddr,fork \
      SYSTEM:"exec ${U9FS_BIN} -n -a none -u root ${SHARE_DIR}" >/dev/null 2>&1 &
    SOCAT_PID="$!"
    QEMU_ARGS+=("${NETDEV_ARGS[@]}")
    ;;
  go9p-ufs)
    if [[ ! -x /work/go9p-ufs ]]; then
      echo "Missing /work/go9p-ufs (go9p UFS server)" >&2
      exit 2
    fi
    echo "Starting go9p ufs server on 0.0.0.0:${KERNEL9P_TCP_PORT}..."
    /work/go9p-ufs -addr "0.0.0.0:${KERNEL9P_TCP_PORT}" >/dev/null 2>&1 &
    GO9P_PID="$!"
    QEMU_ARGS+=("${NETDEV_ARGS[@]}")
    ;;
  *)
    echo "Unsupported KERNEL9P_SERVER=${KERNEL9P_SERVER} (expected qemu, diod, u9fs, or go9p-ufs)" >&2
    exit 2
    ;;
esac

"${QEMU_BIN}" "${QEMU_ARGS[@]}"

