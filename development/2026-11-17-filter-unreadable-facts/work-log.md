# Work log: filters that cannot read a fact

## 2026-11-17 17:33:36 +0900 — 805fae7e: fail a filter on a fact it cannot read

**Change:** `jobfilter.Job.LoadAttributes` and `Job.Diagnosis` return an
error, and so do `Filter.Selects`, `Filter.SelectsArray`, and
`Job.ExecutionAttributes`. `Selects` reads attributes and the log only after
the result selection, the result conditions, and changed/new. `SelectsArray`
tries tasks in order until one matches. `jobstatus.FilterJob`,
`AttemptAttributes`, and `ReadLog` treat `fs.ErrNotExist` as "not recorded"
and return any other read or decode error, including an unparsable recorded
time and the carry error from `locateAttempt`, with the job, run, and file in
the message. `FilterJob.Diagnosis` does not read a successful job's log.
`state` defines the submitted_at/finished_at fallback chain once:
`ResolveAttemptTimestamp` still skips unreadable sources for display, and the
new `LoadAttemptTimestamp` reports them. Callers propagate the error:
`show` (job table, run `--json`, job/array `--json`), `queueedit.selectCommands`
(`copy`), `run.PlanRerun` (`run`/`retry`, plain jobs, partial arrays, whole
arrays), `projectrun` plan, and `jobcontrol.selectJobs`, which now decides
through `Selects` instead of `MatchesAttributes`. Contracts SEL-11/SEL-12,
the status table, `docs/INSPECT.md`, and `docs/ARCHITECTURE.md` say so.

**Reason:** the ISSUES.md item "Job filters cannot report a state file they
fail to read" (2026-11-12). The user chose to fail the command (plan option
A).

**Plan impact:** every step of the plan is done. The `jobstatus.Artifacts`
part of the original item is a separate Open item.

**Validation:** before the fix, `TestFiltersFailOnFactsTheyCannotRead`
(conformance, 7 cases) failed its five error cases: the commands exited 0,
or `run` ran without naming the file. The two cases that must succeed
passed. By hand, writing `garbage` to a successful job's `submitted_at`
made it drop out of `show --filter-started-after 2000-01-01`, with exit 0.
After the fix: the conformance test passes, as do the new unit tests
(`TestSelectsReadsFactsLastAndReturnsTheirErrors`,
`TestFilterJobReportsFactsItCannotRead`,
`TestFilterJobReportsADamagedCarryTime`,
`TestFilterJobDoesNotReadASuccessfulJobsLog`, and the damaged-`status.json`
part of `TestSelectJobsFiltersUnfinishedJobs`). `go vet ./...` and `tsc`
passed, and so did `go test -race` on the changed packages and
`cmd/rotari`. The full `go test -timeout 15m ./...`, conformance included,
passed with no failures under `taskset -c 0-7` at load average about 45.
`scripts/check.sh` itself cannot run here because npm is broken; its steps
were run one at a time, except the full `-race` suite, which was run only
on the changed packages. pre-commit was not run, for the same npm reason.

**Remaining:** ISSUES.md "Artifact listings read an attempt whose carry
cannot be read as one without a record". The `rotari bug-report` check was
not run.
