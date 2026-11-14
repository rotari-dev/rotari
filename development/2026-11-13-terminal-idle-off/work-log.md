# Work log: Shells last as long as the server by default

## 2026-11-14 11:21 +09:00 — `28ff734c` Let Web terminal shells last as long as the server by default

**Change:**

- `internal/terminal/keeper.go`: `KeeperOptions.IdleLimit` of 0 means no limit. `NewKeeper` refuses only a negative
  one, and `armIdleTimer` starts no timer when the limit is 0. `/_terminal/alive` still answers, so pages are
  unchanged.
- `cmd/rotari/web_terminal.go`: `webTerminalIdleLimit` is 0. `newWebTerminalSettings` accepts 0 and otherwise
  requires at least 2 minutes, with the message `must be 0 (no limit) or at least 2m0s`. This holds for the flag,
  the environment, and configs alike.
- `cmd/rotari/web_client.go`: `webIdleTimeoutLine(0)` reads `Shells end when the server stops or you sign out.`.
  It is used by the attached client and by `--detach` (which reads a running server's record, where the limit is
  `0s`).
- `cmd/rotari/cli_spec.go`: the flag's description. The CLI reference, Python schema, and golden schema were
  regenerated.
- Tests:
  - `TestNoIdleLimitKeepsTheShells` (keeper): a shell left with nothing attached survives and no timer runs.
  - `TestNewKeeperRejectsBadOptions` now checks a negative limit instead of 0, since 0 is now valid on purpose.
  - `TestWebTerminalIdleLimitZeroIsNone` (CLI). `TestWebTerminalSettings` drops 0 from its refusals and expects
    the new message.
  - Conformance:
    - `TestTerminalShellsHaveNoIdleLimitByDefault` (WEB-34): a shell with no page stays for 3 seconds, which shows
      only that there is no short default.
    - `TestTerminalNeedsItsFlag` and `TestTerminalOptionsRejectBadConfiguredValues` expect the new message,
      replacing the `0` refusal with `-1h`.
    - `TestWebDetachRunsInTheBackground` checks the no-limit line.
- Docs and contracts:
  - WEB-34 note and row: no limit by default, with `0` or at least two minutes when set.
  - WEB-47 note: the no-limit line.
  - `docs/WEB.md`: the startup example, when shells end, the limits table, and "Keeping shells through a
    disconnect" (step 2 now says to leave the limit unset).
  - FAQ and SECURITY.

**Reason:** The plan's goal: with the run-style client making the server's lifetime the user's choice, a timer on
top is a second lifetime to explain, and it ended work after a laptop slept.

**Plan impact:** All proposals implemented, with no departures.

**Validation:**

- `go vet ./...`, `gofmt -l`, `generate_cli_reference.py --check`, and `sync_readme.py --check`: clean.
- `go test ./cmd/rotari ./internal/... ./conformance/...` (node v24 for JSDOM, `taskset -c 0-7`) passed except
  `TestNewKeeperRejectsBadOptions`, which asserted the old refusal of 0. After it was changed to a negative limit,
  `go test ./internal/terminal` passed.
- Not run: `scripts/check.sh` as a whole and pre-commit, as before.

**Remaining:** The binary-mismatch message plan is next. As the plan's risk says, someone who relied on the 3-hour
default to clean up forgotten shells loses it on upgrade; the release notes should mention it.
