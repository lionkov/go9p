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

## Next

- **Add `go.sum` if/when dependencies are introduced** (currently the module has no external requirements).
- **CI**:
  - Add GitHub Actions workflow running `go test ./...` on macOS + Linux
  - Optionally run `-race` for the client/server packages
- **Docs**:
  - Expand README with a concrete end-to-end example (server + client) including flags
  - Document 9P2000 vs 9P2000.u behavior and what `Dotu` changes
- **Tests**:
  - Add more unit coverage for pack/unpack edge cases (size bounds, malformed packets)
  - Add integration tests covering server ↔ client interaction (beyond the `ufs` tests)

