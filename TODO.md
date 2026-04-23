# TODO

This is a working list for the modernization effort (module + current Go) and for follow-up items needed to support downstream consumers (e.g., projects using 9P as an IPC/filesystem transport).

## Done (in this branch)

- Add `go.mod` so `go9p` builds in module mode.
- Update project to current Go toolchain (`go 1.26.0`, `toolchain go1.26.0`).
- Fix modern-Go test issues:
  - `go vet` format-string issues
  - Unix socket tests: use a real temp socket path (portable on macOS/Linux)
  - Listener shutdown: tolerate expected close errors
- Verify `go test ./...` passes.
- Add Docker-based Linux test runner (`Dockerfile`) with `test` and `race` targets.
- Add end-to-end integration tests covering client↔server:
  - `p/clnt/e2e_ufs_test.go` (UFS server)
  - `p/srv/e2e_fsrv_test.go` (Fsrv synthetic tree)
- Add GitHub Actions CI that runs Docker-based tests on push/PR.
- Add QEMU harness to validate the **Linux kernel 9p client** against QEMU virtio-9p server (`Dockerfile.kernel9p-qemu`).
- Run the kernel-client harness in CI on **arm64** GitHub-hosted runners (dropping amd64 for now).
- Switch kernel-client CI to use the prebuilt `ghcr.io/v9fs/docker:v2.0.0` image (no custom v9fs Dockerfile), with a u-root initrd that mounts/chroots and runs `cmd/kernel9p-e2e`.
- Add `AGENTS.md` synced from `github.com/v9fs/test` and follow it for future harness work.
- Expand README with:
  - Concrete end-to-end example (UFS server + client) with flags and expected output
  - Notes on 9P2000 vs 9P2000.u behavior and what `Dotu` changes
- Make kernel-source retrieval in CI more reliable (shallow git checkout of tag in `Dockerfile.kernel9p-qemu`).
- Pluggable kernel-client 9P backends (`qemu`, `diod`, `u9fs`) with guest config via kernel cmdline.
- CI: kernel9p job builds the test image **once per arch** per run, then runs all server backends against it (plus BuildKit GHA cache across commits).

## Next

- **Add `go.sum` if/when dependencies are introduced** (currently the module has no external requirements).
- **Tests**:
  - Add more unit coverage for pack/unpack edge cases (size bounds, malformed packets) ✅
  - Expand kernel-client smoke tests (symlinks, permissions, xattrs, error mapping, rename across dirs) ✅
  - Optional: split kernel9p server runs into parallel jobs again if wall-clock time matters more than kernel compile duplication
  - Reduce CI cost/time by caching or using prebuilt kernels for the QEMU job (optional) ✅

