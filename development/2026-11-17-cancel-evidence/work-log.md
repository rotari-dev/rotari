# Work log: evidence of what a cancel did

## 2026-11-17 21:31:48 +0900 — 91b04790: cancels.jsonl and the run bug-report collects

**Change:** `model.CancelRecord`/`CancelTarget` and
`state.AppendCancelRecord` write `cancels.jsonl` in the run directory.
`jobcontrol.Controller.Cancel` gained a `CancelSource`, and a
`cancelRecorder` writes one record per request: once before any wait and
otherwise at the end, with the error of a failed request. A request refused
before its run is known writes nothing. `cancelJobs` acts in job ID order
through the new `cancelJob`, which returns `executor`, `marker`, or
`marker+executor`. The exported `CancelJobs` now takes a source and records
too, so no caller can cancel unrecorded. Callers: `cmd/rotari`
(`cli`, `wait`), `supervisor` (`disconnect`), `webui` (`web`), and `mcp`
(`mcp`, a fifth source found while building). `bug-report` bundles
`cancels.jsonl`, and without `--run-id` or `--job-id` it selects the active
or interrupted run through the new `project.InspectWithoutCleanup`, which
does not remove a stale lock. CAN-9 is new, and CLI-26, `docs/FAQ.md`, and
`docs/CONCEPTS.md` say so. In ISSUES.md the item moved to Resolved, and the
process-state part was split off as its own Open item.

**Reason:** the plan's decisions 1–3; decision 4 left process state out.

**Plan impact:** plan complete.

**Validation:** `TestBugReportCollectsTheActiveRunAndItsCancels`
(conformance) failed on `main` before the change, because there was no
`cancels.jsonl`. It passed after. `TestCancelRecordsEachRequest` and
`TestCancelWithoutAnActiveRunRecordsNothing` passed. gofmt, `go vet
./...`, and `-race` on the changed packages passed. The full
`go test -timeout 15m ./...` at load average 70–75 failed in
`conformance/03-interfaces` (two tests), `pairedits`, `pairjobcontrol`,
`pairruns`, `05-web`, and `06-selectors`, the last three with timeouts. The
two `03-interfaces` tests passed alone. `TestJobControlSelectors` also
failed alone, and failed the same way ("run first of sweep did not start")
on the pre-change `main`. A rerun of all seven failed packages at load
average 43–51 passed. pre-commit was not run (npm is broken here).

**Remaining:** ISSUES.md "`bug-report` cannot tell a live command from a
waiting watchdog".
