### Go / tooling

- Add `go.mod` and enable module-based builds (`module github.com/lionkov/go9p`)
- Target modern Go (`go 1.26.0`) and set `toolchain go1.26.0`

### Tests / CI hygiene

- Fix `go test ./...` failures on modern Go:
  - Resolve `go vet` “non-constant format string” warnings
  - Make Unix socket-based tests portable by using a temp socket path instead of `net.Listen("unix", "")`
  - Avoid flaky failures when shutting down the listener (ignore expected “use of closed network connection” on close)
- Add Docker-based Linux test runner (`Dockerfile`) with `test` and `race` targets.
- Add GitHub Actions CI that runs Docker-based tests on every push/PR.
- Add end-to-end client/server integration tests:
  - UFS-backed e2e test in `p/clnt`
  - Fsrv synthetic-tree e2e test in `p/srv`
- Add QEMU-based Linux kernel 9p client smoke test (`Dockerfile.kernel9p-qemu`) and run it in CI (arm64 only for now).
- Switch the kernel-client CI harness to run inside the prebuilt `ghcr.io/v9fs/docker:v2.0.0` image, using a u-root initrd + chroot flow (modeled after `github.com/v9fs/test`), instead of building a bespoke v9fs Docker image.
- Add per-example filesystem tests:
  - Unit tests for each 9P server in `p/srv/examples/*`.
  - Kernel-client QEMU e2e matrix for `ufs`, `ramfs`, `clonefs`, and `timefs`.
  - Userspace TLS e2e stage for `tlsramfs`.
- CI: pin kernel9p Docker build and `docker run` to `linux/${{ matrix.arch }}` so Buildx does not load the wrong CPU architecture on split amd64/arm64 runners.
- CI: build the kernel9p Docker image **once per architecture** per workflow run, then run `qemu` / `diod` / `u9fs` smoke tests against that image (still uses BuildKit GHA cache across commits).

