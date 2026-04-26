## go9p client examples (`p/clnt/examples`)

This directory contains small client programs built on top of `p/clnt` (the go9p client).

They are intentionally minimal and are meant to show how to:

- Dial a server (TCP or TLS)
- Mount/attach to the remote root
- Perform basic operations (open/read/write, readdir, etc.)

### Common flags

Most examples accept:

- `-addr`: server address, e.g. `127.0.0.1:5640`
- `-m`: msize (message size), default usually `8192`

Run any example with `-h` to see its flags.

### `ls` (list directory)

List the entries in a directory:

```bash
go run ./p/clnt/examples/ls -addr 127.0.0.1:5640 /
```

### `read` (read a file)

Read and print a file:

```bash
go run ./p/clnt/examples/read -addr 127.0.0.1:5640 /time
```

### `write` (write a file)

Write data to a file (see the example’s `-h` output for the exact behavior/flags):

```bash
go run ./p/clnt/examples/write -addr 127.0.0.1:5640 /hello.txt
```

### `tls` (TLS transport)

Connect to a TLS-wrapped 9P server (like `p/srv/examples/tlsramfs`) and list a directory:

```bash
go run ./p/clnt/examples/tls -addr 127.0.0.1:5640 /
```

This uses `InsecureSkipVerify` because the server example uses an embedded self-signed test certificate.

### Under the hood (what these examples exercise)

The example programs generally follow the same pattern:

- **Connect** to the server (`net.Dial` or `tls.Dial`)
- **Negotiate** protocol version / `msize` and (optionally) `Dotu` via the client constructor
- **Attach** to the remote root with the current user (see `p.OsUsers`)
- **Walk/Open/Read/Write** using either:
  - the lower-level `Clnt` methods (`Walk`, `Open`, `Read`, `Write`, etc.), or
  - the higher-level `clnt.File` helpers (`FOpen`, `FCreate`, `Readdir`, `Read`, `Write`, etc.)

If you’re debugging behavior:

- Increasing `-d` (debuglevel) typically enables client-side tracing via `clnt.DefaultDebuglevel`.
- `-m` (msize) affects maximum message payloads (and can change chunking behavior for large reads/writes).

