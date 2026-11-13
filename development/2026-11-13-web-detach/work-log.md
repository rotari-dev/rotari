# Work log: `rotari web --detach`

## 2026-11-13 13:53 +09:00 — `a04330bf` Start rotari web in the background with --detach

**Change:**

- `cmd/rotari/web_detach.go` (new)
  - `startDetachedWeb` first looks for this host's detached server. If one is running it reports it
    (`reportDetachedWeb`): the URL and PID, "already running since", and no token. If the options given differ
    from its settings, the command fails naming them and the PID.
  - Otherwise it generates a token when one is needed and none is set, and warns of an unauthenticated
    non-loopback `--host` (`warnUnauthenticatedHost`, now shared with `serveWeb`). `spawnDetachedWeb` then starts
    `os.Args` again with `Setsid`: stdin on `/dev/null`, stdout and stderr on the host's log, and a pipe as fd 3.
    `ROTARI_WEB_DETACHED_CHILD=1` marks the process, and a generated token goes in `ROTARI_WEB_AUTH_TOKEN`. The
    parent waits up to 30 s for `{"url","pid"}` on the pipe. When the child exits, closes the pipe without an
    address, or times out (the child is killed), the parent fails with the reason, up to 4 KiB of the log written
    since the start, and the log path.
  - The background process (`takeWebDetachedChild`, `holdWebDetached`) clears the mark from its environment,
    sets fd 3 close-on-exec, and takes the host's lease before serving. Once it listens (`listening`), it
    publishes its record, reports its address, and starts `keepLogBounded`, a 30 s tick. On exit it removes the
    record.
  - `noteDetachedWeb`: a foreground `rotari web` prints one stderr line naming a detached server on the host.
- `cmd/rotari/web.go`: `--detach` (command line only) is parsed, rejected with `--static-dir` as a
  live-server option, and routed in `cmdWeb`. The sign-in lines moved into `printWebSignIn`, and the host warning
  into `warnUnauthenticatedHost`, so the detached path prints the same text. `cmd/rotari/cli_spec.go` declares the
  flag. `docs/CLI_REFERENCE.md`, `python/rotari/generated_cli.py` (with `ruff check --fix` and `ruff format`, as CI
  does), and `conformance/testdata/golden/schema.json` were regenerated.
- `internal/webdetach` (new): `PathsFor` (rejects host names holding `/`, `\`, or NUL, and `.` and `..`); `Find`,
  which removes a gone server's record under the shared lock and reports probe or read errors; `Hold`, which
  retries for 1 s so a probe does not read as a holder; `Holder.Publish` (atomic, mode 0600) and `Close`;
  `Settings.Differences`, where the port counts only when explicit, the token only when named, and the terminal
  settings only when both servers have the terminal on; `TokenDigest`; `OpenLog` (rotates at 1 MiB on start);
  `KeepLogBounded` (copy to `.log.1` through a temporary file, then truncate in place); `ReadLogFrom`.
- `internal/lease` (new): `TryLock`, `Hold`, `Probe(path, whileFree)`. `internal/server/lease.go` now uses
  `TryLock` and `Probe`. `server.Running` still reads a probe error as "no supervisor", so its behavior is unchanged.
- Conformance: `conformance/05-web/web_detach_test.go` for WEB-40. It covers background start, a session of its
  own, the record without the token, the report and the refusal of a second `--detach`, and a shell environment
  without the mark or token. It also checks that SIGTERM removes the record, that a generated token is kept out of
  the command line, record, and log, that a busy port fails with the reason and log, that a server can start again
  after SIGKILL, and that a foreground server prints the note. The test only signals PIDs whose environment names
  its own master directory. In `conformance/03-interfaces`, `--detach` is sampled as a live-server-only flag in
  `pairweb`, and the web inventory fingerprint was updated.
- Docs and contracts: WEB-40 note and status row. In `docs/WEB.md`, "Keeping shells through a disconnect" now
  leads with `--detach`: finding the server again, stopping it, and its log. `docs/FAQ.md` points there.
  `docs/ARCHITECTURE.md` describes the process flow and adds rows for both packages. `docs/SECURITY.md` covers a
  forgotten detached server and the token hand-off, and corrects the idle default from 30 minutes to 3 hours, a
  stale leftover of `ab995ca7`.

**Reason:** The plan's goal: keeping shells through a disconnect takes one command instead of a tmux procedure.

**Plan impact:** Every proposal is implemented. The implementation departs from the plan in two places, both
recorded in it: the lease file is never removed (removing it would let two servers lock different inodes), and
proposal 10 is moot because rotari builds only for Unix-like systems.

**Validation:**

- `go vet ./...`, `gofmt -l`: clean.
- `go test ./cmd/rotari ./internal/...` passed, with the VS Code server's node v24 on PATH for the JSDOM tests.
  System node v12 fails `TestWebRunViewDrawsMatrixGrid` and `TestWebRunViewClampsLongCells` on syntax, unrelated
  to this change.
- `go test ./conformance/...` passed in full, confined with `taskset -c 0-7`. Before the rebase onto `c1c721d9`,
  the WEB-40 tests passed `-count=10`. After it, the affected packages passed again: `internal/lease`,
  `internal/webdetach`, `internal/server`, `internal/executor`, `conformance`, `pairweb`, and the terminal and
  detach tests of `05-web`.
- `go test -race -count=40 ./internal/lease` passed after `TestHoldEndsWithTheProcess` was made to wait for the
  release (`flock -o` shares the lock with its forked command for an instant). `-race` passed once for
  `internal/webdetach` and `internal/server`.
- Mutation check: with the mark left in the environment, `TestWebDetachRunsInTheBackground` failed as it should.
- `scripts/generate_cli_reference.py --check` and `scripts/sync_readme.py --check`: clean.
- By hand, with an isolated master directory, every outcome behaved as specified: start, report, refusal on
  different options, refusal with `--static-dir`, a busy port, SIGKILL then restart, SIGTERM cleanup, the
  foreground note, and 401 without a token.
- Not run: `scripts/check.sh` as a whole (system `npm` is broken here; no JS changed), pre-commit (not
  installed), and `flock` on an NFS master directory.

**Remaining:** Check `flock` on the shared master directory of the clusters in use, as the plan decided. Possible
follow-ups, none planned: `--name` for several detached servers, `--stop`, and the Settings menu saying a server
is detached.
