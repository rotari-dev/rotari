# Work log: Verify recorded processes on macOS

## 2026-11-14 — `9d88728b` Verify recorded processes on macOS with a no-namespace marker and the kernel's start time

**Change:**

- `internal/state`: `ProcessStart` and `PIDNamespace` moved from `lock.go` to `process_platform.go`, which calls
  per-platform `processStart` and `pidNamespace`:
  - Linux (`process_platform_linux.go`): `/proc` as before.
  - macOS (`process_platform_darwin.go`): `NoPIDNamespaces` (`none`), and `p_starttime` from
    `unix.SysctlKinfoProc("kern.proc.pid", pid)` as `seconds.microseconds`; empty for a PID with no process.
  - The BSDs: `none` and no start time. Other platforms: neither.
  - `Locate`, `ClassifyProcess`, and the unlock and execution policies are unchanged.
- Tests: `TestPIDNamespaceIsKnownWhereProcessesAreVerified` and `TestProcessStartTellsProcessesApart` (a running
  process's start time is known and stable, a later child's differs, a reaped child's is empty), on Linux and
  macOS.
- `conformance/support.ProcessIdentity` off Linux returns `state.PIDNamespace` and `state.ProcessStart`
  instead of empty values.
- Docs and contracts: the COORD note on `state.ObserveProcess`, OPERATIONS.md (coordinator identity), FAQ (local
  job control), and ARCHITECTURE.md's `internal/state` row.

**Reason:** The plan's goal; on macOS `unlock` refused every interrupted run without `--assume-stopped` and every
local job read as unknown (Go CI run 43575930113).

**Plan impact:** Steps 1, 2, and 4 done. Step 3 (the macOS CI leg) is next; the ISSUES entry stays open until then.

**Validation:**

- `go vet ./...` for linux, darwin/arm64, darwin/amd64, and `go vet ./internal/state` for freebsd: clean.
- On Linux: `go test ./internal/... ./cmd/rotari ./conformance/... ./scripts` passed (59 packages, node v24);
  the two new tests passed `-count=3`.
- Not run: anything on macOS, where the start time's stability matters most; `scripts/check.sh` and pre-commit.
