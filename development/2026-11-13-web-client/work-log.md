# Work log: A run-style client for rotari web

## 2026-11-14 11:12 +09:00 — `1e31805e` Make rotari web a run-style client at a terminal

**Change:**

- `cmd/rotari/web_client.go` (new)
  - `webRunsAsClient` is true at a terminal or with `--disconnect-action detach`.
  - `runWebClient` checks first, with `detach`, that no detached server runs. It then starts the same command with
    `ROTARI_WEB_DETACHED_CHILD=attached-<action>` in a new session, with four pipes: stdout and stderr (relayed),
    readiness/events (fd 3), and the lifeline (fd 4). Its loop maps events and input to lifeline messages:
    - Ctrl-C (SIGINT) sends `stop`. SIGHUP, SIGTERM, or a failed terminal read sends `disconnect`. Ctrl-D (EOF at a
      terminal, or the detach byte, from `watchWebClientInput`) sends `detach`.
    - `detached` prints the URL, PID, log, and `--shutdown` hint, and exits 0 after a short drain of the relays.
    - `refused` prints the reason and watches keys again.
    - The server's exit ends the client with its status.
  - `printWebClientHints` prints the Ctrl-D line (only with a terminal) and the idle-limit line (only with the
    terminal on). `webIdleTimeoutLine` and `shortDuration` write the limit as `3h` or `1h30m`.
- `cmd/rotari/web_detach.go`
  - The child takes its mode from `takeWebChildMode`: `1` (started detached, as before) or `attached-stop|detach`.
    `attachWebChild` opens fds 3 and 4 close-on-exec and ignores SIGPIPE, since a vanished client must not kill
    the server on its next line.
  - `listening` keeps the record for later. `announced`, called after the "listening at" line is printed, sends
    `ready` and starts `followClient`.
  - `followClient` follows `stop`, `detach`, and `disconnect`, and treats the end of the lifeline as `disconnect`.
  - `detach` takes the lease, publishes the record, and points fds 1 and 2 at the host's log (`redirectOutput`:
    `unix.Dup3` on Linux, `Dup2` elsewhere). It starts log rotation and reports `detached`, or `refused` with the
    other server's URL and PID.
  - With `detach` as the action, a disconnect that cannot detach stops the server and appends why to the log.
  - `--detach` now prints the idle-limit line, both when it starts a server and when it reports one (from the
    record).
- `cmd/rotari/web.go`
  - `cmdWeb` routes by child mode first, then `--detach`, then the client, else serves in process.
  - `--disconnect-action stop|detach` takes its default from `webDisconnectActionDefault`: the `[web]` section,
    then `ROTARI_WEB_DISCONNECT_ACTION`, with bad values reported with their source. The flag is refused with
    `--detach` (explicitly) and `--static-dir`.
  - The foreground note about a detached server is printed by attached servers too.
- `cmd/rotari/cli_spec.go`: the flag. `cliCommandEnvironmentVariable` maps `web`'s `disconnect-action` to the new
  `envWebDisconnectAction`; the global map would otherwise have given it run's `ROTARI_DISCONNECT_ACTION`, found
  while testing. Regenerated: CLI reference, environment-variable docs, Python schema, golden schema. The
  `pairweb` sample and web inventory fingerprint were updated.
- Tests:
  - `conformance/05-web/web_client_test.go` (WEB-47) drives the client through a pty (`creack/pty`). It covers:
    Ctrl-D detach with the hints (with `--terminal-idle-timeout 90m`) and a shell that keeps running; Ctrl-C;
    Ctrl-D refused beside another detached server; SIGHUP and SIGKILL with each action; no child and no Ctrl-D
    line without a terminal; and the disconnect action's own sources.
  - Unit tests `TestShortDuration` and `TestWatchWebClientInput`.
- Docs and contracts:
  - WEB-47 note and row.
  - `docs/WEB.md`: the options table, a new "Stopping and detaching", and "Keeping shells through a disconnect"
    leading with Ctrl-D.
  - FAQ and ARCHITECTURE.

**Reason:** The plan's goal: `rotari web` and `rotari run` work the same at the keyboard; the user also asked that
startup explain Ctrl-D and the idle timeout.

**Plan impact:** All proposals implemented, with two departures recorded in the plan:

- The client starts only when the server could be detached (at a terminal or with `detach`). Otherwise `rotari web`
  serves in its own process as before, so nothing started without a terminal changes.
- The contract is WEB-47, since WEB-42 to WEB-46 were taken by other work meanwhile.

**Validation:**

- `go vet ./...`, `gofmt -l`, `generate_cli_reference.py --check`, and `sync_readme.py --check`: clean.
- `go test ./cmd/rotari ./internal/... ./conformance/...` passed in full, with node v24 for JSDOM and
  `taskset -c 0-7`. After the rebase onto `c995d437` (a plan document only), `go test ./conformance` passed again.
- The detach, shutdown, and client conformance tests passed `-count=10`. The client tests passed `-race -count=5`,
  after a race in the test's own reading of stderr was fixed.
- Mutation check: with the server ignoring the end of the lifeline, both SIGKILL cases failed as they should.
- By hand, through a pty: Ctrl-D, Ctrl-C, SIGHUP and SIGKILL with each action, a refused Ctrl-D, one process
  without a terminal, and the sources of the disconnect action.
- Not run: macOS (`Dup2` path), `scripts/check.sh` as a whole, and pre-commit.

**Remaining:** [terminal-idle-off](../2026-11-13-terminal-idle-off/plan.md) next, which changes the idle-limit line
to "Shells end when the server stops or you sign out." with no limit set. The binary-mismatch message plan comes
after it.
