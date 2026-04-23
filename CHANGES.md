### Go / tooling

- Add `go.mod` and enable module-based builds (`module github.com/lionkov/go9p`)
- Target modern Go (`go 1.26.0`) and set `toolchain go1.26.0`

### Tests / CI hygiene

- Fix `go test ./...` failures on modern Go:
  - Resolve `go vet` “non-constant format string” warnings
  - Make Unix socket-based tests portable by using a temp socket path instead of `net.Listen("unix", "")`
  - Avoid flaky failures when shutting down the listener (ignore expected “use of closed network connection” on close)

