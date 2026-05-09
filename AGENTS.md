# AGENTS.md

This file captures the working preferences for AI agents contributing to this repo.
Tweak freely.

## Branching and change hygiene

- Do active work on `rework` unless told otherwise.
- Keep `README.md`, `CHANGES.md`, and `TODO.md` updated as changes land.
- Do not commit generated outputs (`logs/`, `kernel/`, `tmp/`, `initrd.cpio`, pid files).

## Test philosophy
- Prefer **guest-direct execution** (Option A): run tests **inside the QEMU guest**.
- Avoid SSH/port-forwarding flows unless explicitly requested.
- use u-root based minimal initrd as root filesystem
- use u-root/cpu with NFS option to expose tools, benchmarks, tests, and results directories to the guest running in qemu
- be able to run as github actions or using act locally
- provide easy mechanism for running local tests (make or script based)
- verify workflow locally before pushing to github
- CI should surface failures (red dashboards) while still running the full suite:
 - Run the whole matrix
 - Record failures
 - Exit non-zero at the end

## Local/dev environment assumptions

- Primary local dev is **macOS via Docker + QEMU**.
- Prefer solutions that work on Docker Desktop (no reliance on KVM).
- After any manual experiment that uses `docker run`, ensure containers are not left running:
 - Prefer `docker run --rm` plus a project label `v9fs.harness=v9fs-test`.
 - If you suspect a hung run left containers behind, clean up with `make docker-clean` or `./scripts/v9fs-docker-clean`.

## CI architecture preferences
- Use http://github.com/v9fs/docker published base image instead of building custom docker for kernel build and/or test frameworks
- Default to **ARM64** (`ubuntu-24.04-arm`) for builds/tests unless asked otherwise.
- Build and/or test will be triggered by external triggers (such as v9fs/linux changes) or user request in addition to any changes to this repo
- Separate concerns:
 - **Kernel publishing** workflow: builds `v9fs/linux` arm64 `Image` and publishes it.
 - **Harness CI** workflows: download a published kernel `Image` and run tests.
- Publishing:
 - Prefer a stable, `wget`-able GitHub Release asset `Image` tagged `kernel-main`, `kernel-nightly`, or `kernel- `.
 - GHCR is optional/secondary; keep it consistent if used.

## Logging and debuggability

- Always preserve logs for failures (artifact upload `if: always()`).
- When tests fail, also dump the relevant tails into the CI console output:
 - `logs/*/qemu.log`
 - per-test `*.log`
 - `guest.exitcode` markers (or equivalent)

## Style

- Prefer small, explicit scripts over complex magic.
- Keep paths stable and explicit (`/workspaces/share`, `kernel/.build/...`).
- Avoid large refactors unless requested; preserve working behavior first.

