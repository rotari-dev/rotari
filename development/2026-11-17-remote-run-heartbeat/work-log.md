# Work log: notice runs whose coordinator on another host stopped

## 2026-11-17 18:05:38 +0900 — 2d414510: heartbeat writer (phase 1)

**Change:** `projectrun.Runner.Run` starts `startHeartbeat`, which writes
`heartbeat.json` (`state.Heartbeat`: seq, the coordinator's clock, host,
PID, PID namespace) through `Store.WriteJSON`'s rename, right away and then
every `state.HeartbeatInterval` (30 s), until the deferred stop after
`Finish` has removed the lock. A failed write goes to `Runner.Errorf` once
when writing starts to fail and once when it recovers. `state.Store` gained
`WriteHeartbeat`/`LoadHeartbeat`: a missing file is "not found", and a
damaged one is an error naming it. `bug-report` bundles the file.
`docs/CONCEPTS.md` lists it in the run directory.

**Reason:** the plan's phase 1. `load_samples.jsonl` could not serve as a
heartbeat, because it is written only when the load average can be read,
and its write errors are dropped.

**Plan impact:** phase 1 done. `Begin` and `Run` are both called in the
supervisor process that owns the lock, so starting the heartbeat in `Run`
covers the coordinator.

**Validation:** `TestHeartbeatBeatsUntilStopped` and
`TestHeartbeatReportsFailureOnceAndRecovery` passed (with `-race`), as did
`TestHeartbeatRoundTripAndAbsence`. A real `rotari run` wrote
`heartbeat.json` naming its PID and host. `go test` on `projectrun`,
`state`, `bugreport`, `supervisor`, `cmd/rotari`, and `./conformance/...`
passed, except `TestTableViewerMovesTabsToWindows`: that run used the
system's Node v12, which jsdom cannot parse. The later full runs, with the
newer Node, passed it.

**Remaining:** phases 2–3.

## 2026-11-17 18:21:01 +0900 — 51d4bc73: unresponsive runs in the inbox (phase 2)

**Change:** `runwatch.ReportInterrupted` became `Watcher.Report`, and
`cmd/rotari/web.go` keeps one `Watcher` for the server's life. For a project
whose `project.Inspect` is `Running` with `LockRemote`, `inspectRun` reads
the heartbeat. `Watcher.observe` remembers the last sequence seen and when,
by the server's clock. Any change restarts the wait, and
`UnresponsiveAfter` (5 min) unchanged yields a
`notification.RunUnresponsive` event, recorded with `AppendOnce`. Runs that
are no longer active remote runs are forgotten. `notification.Event` gained
`host`, `last_heartbeat_at`, and `silent_seconds`. `Includes` selects the
event with `run_failure`. `NewBrowserMessage` titles it "rotari: run
unresponsive" and always appends `UnresponsiveExplanation`. Added NOT-6 and
its status row; updated NOT-4 and `docs/NOTIFICATIONS.md`. The NOT-4
conformance check for a pass warning was updated to the new warning text,
so that it still checks something.

**Reason:** the plan's phase 2, with decisions 2 (no Webhook) and 3 (fixed
5 minutes).

**Plan impact:** phase 2 done. No test-only timing setting was added to the
binary, so the 5-minute report is covered by unit tests with an injected
clock, and NOT-6 is `partial`.

**Validation:** `TestWatcherReportsARemoteRunWhoseHeartbeatStopped` and
`TestWatcherReportsADamagedHeartbeat` passed, and so did the browser-message
rows. `TestWebWaitsForAStoppedRemoteHeartbeat` (conformance) passed once its
cleanup killed the `rotari run` client. Its first version hung there,
because a lock naming another host keeps the client waiting, which is
correct. gofmt, `go vet ./...`, `-race` on the changed packages, and the
full `go test -timeout 15m ./...` passed under `taskset -c 0-7` at load
average about 55.

**Remaining:** phase 3.

## 2026-11-17 18:39:42 +0900 — 14e8d938: display beside running (phase 3)

**Change:** `runview.HeartbeatWarning` (for a `LockRemote` lock of this run
with a heartbeat whose time is at least `state.HeartbeatStaleAfter +
HeartbeatClockMargin` old) returns "no heartbeat from HOST for Nm", or
"heartbeat unreadable: ...". The thresholds moved into `state`, and
`runwatch.UnresponsiveAfter` uses them. Callers: `show` (run and job
headers through `runLifecycleText`, and both `--json` forms), `runlist.Run`
→ the `runs` table and `--json`, `web.Run` (loader for the active run) and
`webui.loadWebRunSummary`, and the page's run list and run facts
(`heartbeatWarningPill`). Added NOT-7 and its status row, and updated
`docs/NOTIFICATIONS.md`. ISSUES.md item moved to Resolved.

**Reason:** decision 1 (display, as a second phase).

**Plan impact:** plan complete. The lifecycle value stays `running`
everywhere, since filters, JSON consumers, and the page compare it.

**Validation:** `TestHeartbeatWarningOnlyForAStaleRemoteRun` (including
the margin boundary) and `TestStaleRemoteHeartbeatIsShownBesideRunning`
(show, runs, both JSON forms, `/api/run`, `/api/project`) passed. gofmt,
`go vet ./...`, `tsc`, `-race` on the changed packages, and the full
`go test -timeout 15m ./...` passed under `taskset -c 0-7`. Prettier was
run on `web_app_core.js`, which was clean before the change. Not done: a
check with two real hosts, a visual check of the pill in a browser, and
pre-commit (npm is broken here).

**Remaining:** none in the plan.
