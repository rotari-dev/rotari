# Work log: Say when a running server is another rotari binary

## 2026-11-14 11:40 +09:00 — `cd6349de` Say when a running server or run is another rotari binary

**Change:**

- `internal/model/model.go`: new `RunBinary` (version, executable, size, modification time). Its `Differs` compares
  only the path, size, and time; the version does not decide. `RunCoordinator.Binary` is optional.
- `internal/projectrun`: `Runner.Binary` is written into the coordinator identity by `WriteContext`.
  `cmd/rotari/project_run.go` sets it from `recordedBinary()`, which is nil when the binary cannot be looked at, so
  such a run records nothing rather than failing.
- `internal/webdetach`: `Record.Binary`, written by the detached or detaching server in `listening`.
- `cmd/rotari/binary_note.go` (new):
  - `currentBinary` (`os.Executable` and `os.Stat`) and `recordedBinary`.
  - `binaryNote(subject, other)` is empty for nil or the same binary, says "a different build of …" for an equal
    version, names both versions otherwise, and says the comparison could not be made when this binary cannot be
    looked at.
  - `noteRunBinary`, used by `wait`, asks `project.AssessCoordinator` and notes only for a live coordinator.
- Callers:
  - `followRunWithOutput` in `wait.go` (after attaching, not for the initial `run` client).
  - `reportDetachedWeb`, `noteDetachedWeb`, and `shutdownDetachedWeb`, through `printDetachedWebBinaryNote`.
- Tests:
  - `TestBinaryNote` and `TestRunBinaryDiffers` (unit).
  - `conformance/03-interfaces/binary_note_test.go` `TestNotesAnotherRotariBinary` (CLI-31). A copy of the test
    binary at another path is "a different build". It starts a run and a detached server, and checks the note
    from `wait`, `web --detach`, and `web --shutdown`, and its absence with the record's own binary.
- Docs and contracts:
  - CLI-31 note and row. WEB-40 says the record holds the binary.
  - `docs/RUNNING.md` (`wait`), `docs/WEB.md` (finding a detached server again), and a FAQ entry on upgrades.
  - The plan's decisions (path, size, and time; no note for `run`) and a memo on the Web UI idea, which was left
    out.

**Reason:** The plan's goal: catch "I upgraded, but the old server or run is still going", which the user proposed on
2026-11-13.

**Plan impact:** Implemented as decided on 2026-11-14. The Web UI idea is recorded in the plan's memo, not done.

**Validation:**

- `go vet ./...`, `gofmt -l`, `generate_cli_reference.py --check`, and `sync_readme.py --check`: clean.
- `go test ./cmd/rotari ./internal/... ./conformance/...` passed in full, with node v24 for JSDOM and
  `taskset -c 0-7`. `TestNotesAnotherRotariBinary` passed `-count=5`. After the rebase onto `8347f3e1` (one
  browser-test file from other work), the code and tests are unchanged.
- By hand, with two builds (`-X main.version=v9.0.1` and `v9.0.2`): notes from `web --detach` and `--shutdown` for
  the other binary and none for its own, and a note from `wait` for a run the other started.
- Not run: macOS, `scripts/check.sh` as a whole, and pre-commit.

**Remaining:** None planned. The Web UI note is in the plan's memo.
