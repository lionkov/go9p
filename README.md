# go9p

`go9p` is a Go implementation of the **9P2000** protocol (Plan 9 file protocol). It contains:

- **Protocol definitions + packing/unpacking** in `p/` (`package p`)
- A **client** in `p/clnt` (`package clnt`)
- A **server framework** in `p/srv` (`package srv`)
- A reference **Unix filesystem server** in `p/srv/ufs` (`package ufs`)
- Small example programs in `p/clnt/examples` and `p/srv/examples`

This repository is the upstream `github.com/lionkov/go9p`.

## Status / Compatibility

- **Protocol**: 9P2000 with optional 9P2000.u fields (see `Dotu` usage in server/client code).
- **Go**: This forked branch adds a Go module and is intended to work with modern Go toolchains.

### 9P2000 vs 9P2000.u (`Dotu`)

- **9P2000**: the “base” protocol.
- **9P2000.u**: an extension that adds (among other things) numeric uid/gid fields and Unix-y metadata (see `Dir.Uidnum`, `Dir.Gidnum`, and related fields in `package p`).

In this codebase you’ll see a boolean called **`Dotu`** on both client and server types. In practice:

- **Server**: `Srv.Dotu` indicates the server *can* speak 9P2000.u.
- **Client**: `Clnt.Dotu` indicates the client *wants* to speak 9P2000.u.
- The negotiated connection behavior is exposed as `Conn.Dotu` (server side) based on the `Tversion`/`Rversion` handshake.

If you’re targeting the **Linux kernel 9p client**, it most commonly uses the `9p2000.L` family (a different dialect from 9P2000.u). This repository’s code supports 9P2000 and 9P2000.u; the QEMU kernel-client harness in this fork validates kernel-client behavior against QEMU’s virtio-9p server rather than validating dialect parity with go9p itself.

## Install (module mode)

This repository is now module-enabled:

```bash
go get github.com/lionkov/go9p@latest
```

## Quick start

### Run the reference server (UFS)

The UFS server exports a local directory tree over 9P:

```bash
go run ./p/srv/examples/ufs -addr 127.0.0.1:5640
```

### Run a client example

List files from a 9P server:

```bash
go run ./p/clnt/examples/ls -addr <network address>
```

The example programs have their own flags; run them with `-h` to see usage.

### End-to-end example (UFS server + client)

In one terminal, run a server exporting a local directory tree:

```bash
go run ./p/srv/examples/ufs -addr 127.0.0.1:5640 -root .
```

In another terminal, list the root directory via 9P:

```bash
go run ./p/clnt/examples/ls -addr 127.0.0.1:5640 /
```

Expected output is one name per line, for example:

```text
.git
LICENSE
p
README.md
```

### Example servers and clients

More detailed documentation for the example programs lives alongside the code:

- Server examples: `p/srv/examples/README.md`
- Client examples: `p/clnt/examples/README.md`

## Testing

```bash
go test ./...
```

### Docker (Linux) tests

Run the full test suite under Linux from macOS/Windows:

```bash
docker build -t go9p:test --target test .
docker run --rm go9p:test
```

Optional race run:

```bash
docker build -t go9p:race --target race .
docker run --rm go9p:race
```

### Kernel 9P client smoke test (QEMU)

This boots an upstream Linux kernel in QEMU, mounts a virtio-9p export using the
**kernel 9p client**, and runs a smoke test against that mount.

```bash
docker build -f Dockerfile.kernel9p-qemu --target kernel9p-test .
```

Pin kernel version and/or architecture:

```bash
docker build -f Dockerfile.kernel9p-qemu --target kernel9p-test \
  --build-arg LINUX_VERSION=7.0 \
  --build-arg KERNEL_ARCH=amd64 \
  .
```

This fork also includes a `github.com/v9fs/test`-style harness that runs inside the prebuilt
`ghcr.io/v9fs/docker:v2.0.0` image (no custom Dockerfile) and uses a u-root initrd + chroot flow:

```bash
docker run --rm --privileged --platform linux/arm64 \
  -v "$PWD:/opt/v9fs/go9p" -w /opt/v9fs/go9p \
  ghcr.io/v9fs/docker:v2.0.0 \
  bash /opt/v9fs/go9p/scripts/v9fs/ci-e2e-fs.sh ufs
```

You can also run the kernel-client harness against other example servers:

```bash
docker run --rm --privileged --platform linux/arm64 \
  -v "$PWD:/opt/v9fs/go9p" -w /opt/v9fs/go9p \
  ghcr.io/v9fs/docker:v2.0.0 \
  bash /opt/v9fs/go9p/scripts/v9fs/ci-e2e-fs.sh ramfs
```

Supported `ci-e2e-fs.sh` filesystem arguments: `ufs`, `ramfs`, `clonefs`, `timefs`.

Note: `tlsramfs` is **TLS-only** and is exercised via userspace (Go) tests rather than a Linux kernel mount.

## Repository layout

- `p/`: core protocol + helpers (`package p`)
- `p/clnt/`: client implementation
- `p/srv/`: server framework
- `p/srv/ufs/`: Unix filesystem server
- `cmd/kernel9p-smoke/`: guest-side smoke test used by the QEMU kernel-client harness

## License

BSD-style license; see `LICENSE`.

