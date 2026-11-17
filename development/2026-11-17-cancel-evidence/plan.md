# Evidence of what a cancel did

Created: 2026-11-17

## Purpose

When a cancel goes wrong, `rotari bug-report` cannot show what it did. No
file records a cancel request: who asked, which jobs it targeted, what it
did to each one (an executor cancel, or a `cancelled` marker for a job not
yet submitted), or its errors. `cancelWaitTarget` reports an error only
while the run still looks running, and `server.log` has nothing. Also,
without `--run-id`, `bug-report` collects only the latest *settled* run, so
a stuck active or interrupted run, which is exactly the one that needs
evidence, is left out ("no settled runs"). Recorded in
[ISSUES.md](../ISSUES.md) ("`bug-report` cannot show what a cancel did").

## Current paths

Every cancel goes through `jobcontrol.Controller.Cancel`:

| Caller | Path |
| --- | --- |
| `rotari cancel` (IDs, `--job-name`, filters) | `cmd/rotari/job_control.go` |
| `wait` / attached `run` Ctrl-C | `cmd/rotari/wait.go` `cancelWaitTarget` |
| A disconnected client whose disconnect action is cancel | `internal/supervisor/run.go` |
| Web Cancel job / Cancel run | `internal/webui/webui.go` |

A run-wide cancel marks the project `cancelling`, then calls `CancelJobs`
for every unfinished job. `CancelJobs` either cancels a submitted job
through its executor, or writes `cancelled` for a job not yet submitted.

## Decisions

Decided by the user on 2026-11-17: all four as recommended below.

1. **Record.** Recommended: `Controller.Cancel` appends one JSON line per
   request to `cancels.jsonl` in the run directory. The line holds the time
   (nanoseconds), who asked (`source`), its PID and host, run-wide or by
   job, the requested and resolved targets, and for each target what was
   done (`executor`, `marker`, `already finished`) and its error. A failure
   before any target, such as a host mismatch, is recorded too. It is
   append-only and bounded by the number of requests. It holds no command
   lines or environment, so it needs no extra redaction. `bug-report`
   bundles it. The alternative, a log under the master directory, would not
   travel with the run.
2. **`source`.** Recommended: callers say who they are: `cli`, `wait`,
   `disconnect`, `web`. This changes `Cancel`'s signature at the four call
   sites. Without it, the record cannot tell a user's cancel from an
   automatic one.
3. **`bug-report` without `--run-id`.** Recommended: collect the run that
   `show` would show (the active run, then an interrupted run, then the
   latest), not only the latest settled run. This changes which run a
   bare `bug-report` collects, so CLI-26 and the FAQ change.
4. **Out of scope.** Process-tree snapshots and `status.json.stopping`
   (from the same item) need a privacy-safe design for command arguments.
   They stay Open.

## Tests and validation

- Unit: `jobcontrol` writes one line per request for a run-wide cancel, a
  job cancel, a marker for an unsubmitted job, and a refused cancel.
- Conformance: a cancel through the binary leaves a `cancels.jsonl` line
  naming the source and targets, and `bug-report` (no `--run-id`) of a
  project with an active run collects that run and the file.
- `go test ./conformance`, then the `scripts/check.sh` steps.

## Status

Implemented on 2026-11-17; see [work-log.md](work-log.md). MCP turned out to be a fifth source (`mcp`). `CancelJobs` stays exported and records too, so no caller can cancel without a record. `bug-report` reads the project with `project.InspectWithoutCleanup`, so a stale lock is collected rather than removed.
