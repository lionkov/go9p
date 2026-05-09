# go9p

`go9p` is a Go implementation of the **9P2000** protocol (Plan 9 file protocol). It contains:

- **Protocol definitions + packing/unpacking** in `p/` (`package p`)
- A **client** in `p/clnt` (`package clnt`)
- A **server framework** in `p/srv` (`package srv`)
- A reference **Unix filesystem server** in `p/srv/ufs` (`package ufs`)
- Small example programs in `p/clnt/examples` and `p/srv/examples`

This repository is `github.com/lionkov/go9p`.

## Status / Compatibility

- **Protocol**: 9P2000 with optional 9P2000.u fields (see `Dotu` usage in server/client code).
- **Go**: A Go module is provided so the code builds with modern Go toolchains.

### 9P2000 vs 9P2000.u (`Dotu`)

- **9P2000**: the “base” protocol.
- **9P2000.u**: an extension that adds (among other things) numeric uid/gid fields and Unix-y metadata (see `Dir.Uidnum`, `Dir.Gidnum`, and related fields in `package p`).

In this codebase you’ll see a boolean called **`Dotu`** on both client and server types. In practice:

- **Server**: `Srv.Dotu` indicates the server *can* speak 9P2000.u.
- **Client**: `Clnt.Dotu` indicates the client *wants* to speak 9P2000.u.
- The negotiated connection behavior is exposed as `Conn.Dotu` (server side) based on the `Tversion`/`Rversion` handshake.

If you’re targeting the **Linux kernel 9p client**, it most commonly uses the `9p2000.L` family (a different dialect from 9P2000.u). This repository’s code supports 9P2000 and 9P2000.u; the QEMU kernel-client harness validates kernel-client behavior against QEMU’s virtio-9p server rather than validating dialect parity with go9p itself.

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

## Building a synthetic filesystem (server-side guide)

This repo’s `p/srv` package lets you serve a **synthetic** filesystem by building a tree of `srv.File`
nodes and attaching behavior to nodes by implementing methods (e.g. `Read`, `Write`, `Create`, `Wstat`).

If you’re looking for working reference implementations, see:

- `p/srv/examples/ramfs` (in-memory file tree with `Create` + `Wstat`)
- `p/srv/examples/timefs` (read-only, synthetic time files)
- `p/srv/examples/clonefs` (Plan 9 “clone” control-file pattern)

### 1) Define your node types

Most synthetic servers embed `srv.File` into a custom type so the type can both:

- carry state (bytes, counters, timestamps, config), and
- implement node operations (read/write/create/wstat/etc.).

Example sketch:

```go
type MyFile struct {
  srv.File
  data []byte
}
```

If you need a directory that can create children dynamically, you typically use the same pattern but implement `Create`.

### 2) Build a tree with `(*srv.File).Add`

You construct a tree of nodes by calling `Add(parent, name, user, group, perm, impl)`.

- `parent`: `nil` means “this is the root”
- `perm`: includes file bits and optionally `p.DMDIR` for directories
- `impl`: usually `yourNode` (the receiver implementing methods); can be `nil` for a “plain” node

Example tree:

```text
/
├── hello          (read/write)
└── ctl            (control file; write triggers side effects)
```

In code (high-level sketch):

```go
root := new(srv.File)
_ = root.Add(nil, "/", user, nil, p.DMDIR|0555, nil)

hello := new(MyFile)
_ = hello.Add(root, "hello", user, nil, 0666, hello)
```

### 3) Implement operations (the “design surface”)

The common methods you’ll implement (depending on your needs):

- **`Read(fid, buf, offset)`**: fill `buf` with bytes starting at `offset`, return `n`.
  - Design choice: return **EOF via `n=0,nil`** (common) vs a real error (rare).
  - Must respect `offset` for “normal” files; for “streaming” files you may intentionally ignore it.
- **`Write(fid, data, offset)`**: store bytes at `offset`, return `n`.
  - For in-memory files, this usually means “grow to `offset+len(data)` then copy”.
- **`Create(fid, name, perm)`** (directories): create a child node and `Add` it under the directory.
  - Pattern: directories are themselves nodes; `Create` is implemented on the directory node type.
- **`Wstat(fid, dir)`**: apply metadata changes (rename, chmod, truncate, uid/gid).
  - Typical minimal subset:
    - `dir.Name` → rename
    - `dir.Mode` → chmod (permissions)
    - `dir.Length` → truncate
- **`Remove(fid)`**: handle unlink semantics.
  - Many examples stub this out; production servers usually remove the node from its parent.

The easiest way to get this right is to mimic the example servers and add functionality incrementally.

### 4) Common design patterns

#### Pattern A: “Control files” (`ctl`)

Expose configuration or commands via a file whose **writes** trigger side effects.

- **Read**: show current config/state (`"debug=1\nmsize=8192\n"`).
- **Write**: parse commands (`"debug=0\n"`, `"reset\n"`, `"mk foo\n"`).

This maps well to Plan 9 style interfaces and keeps the surface small.

#### Pattern B: “Clone” allocators (`/clone` → `/1`, `/2`, …)

This is useful for session allocation (think: allocate a new channel/endpoint).

- Reading `/clone` at offset 0 returns a freshly allocated name.
- The server creates a new child node named with a unique id.

See `p/srv/examples/clonefs` and the notes in `p/srv/examples/README.md`.

#### Pattern C: “Streaming” or “infinite” files

If a file logically represents a stream (time, random bytes, logs), you may choose to:

- ignore `offset`, and/or
- never return EOF.

Be explicit in documentation because some clients/tools read until EOF.
See `timefs`’s `/inftime`.

#### Pattern D: Block-backed in-memory files

For large files, using fixed-size blocks can be simpler than continuously resizing a single byte slice.

- lazily allocate blocks on first write
- treat missing blocks as zero-filled
- maintain a separate authoritative length

See `ramfs` for a working example.

#### Pattern E: “Virtual metadata” (fake perms, timestamps, ids)

Synthetic servers often implement just enough metadata to satisfy clients:

- store mode bits on the node (`File.Mode`)
- treat truncation as data resize
- implement rename as changing a node’s name in its parent

Be clear about what you do *not* implement (ACLs, xattrs, hardlinks, etc.).

### 5) Serving the filesystem

Once you have a root node, you create and start an `Fsrv`:

```go
srv := srv.NewFileSrv(root)
srv.Dotu = true // enable 9P2000.u fields when clients request it
srv.Start(srv)
_ = srv.StartNetListener("tcp", "127.0.0.1:5640")
```

Notes:

- **`Dotu`**: set this based on whether you want to support 9P2000.u extensions (numeric ids, Unix-y metadata).
- **Concurrency**: reads/writes may happen concurrently; guard mutable node state with a mutex where appropriate.
- **Permissions**: examples are permissive; real servers should validate users/groups and enforce access checks.

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

This repository also includes a `github.com/v9fs/test`-style harness that runs inside the prebuilt
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

