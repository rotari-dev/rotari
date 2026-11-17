# Filters that cannot read a fact

Created: 2026-11-17

## Purpose

`--filter-host`, the `--filter-started-*` / `--filter-finished-*` options,
`--filter-longer-than` / `--filter-shorter-than`, and `--filter-diagnosis`
judge a job by facts read from its attempt: `status.json` (hosts, start),
`submitted_at` / `finished_at` / `job.json` (times), `output` / `stdout` /
`stderr` (the log), and, for a carried job, the source run's
`commands.json` (the carry). These are read through closures and helpers
that return no error, so a file that exists but cannot be read or decoded
reads as a job without hosts, times, or a log, and the job is quietly not
selected. `retry --filter-host` can then skip a job it should run, and
`show` hides it, with exit status 0. AGENTS.md forbids this kind of
success-shaped fallback. Recorded in [ISSUES.md](../ISSUES.md) on
2026-11-12, during [shared-rule-violations](../2026-11-11-shared-rule-violations/)
Phase 3.

## Decision

What a filter does when a fact it needs is recorded but unreadable. The
user chose A on 2026-11-17.

- **A. Fail the command, naming the file (recommended).** Matches the
  AGENTS.md rule and `state.LoadRunOrigin`, which already returns its error.
  The user can repair or remove the named file. One damaged file blocks
  the filtered command, but only when that job's fact is actually needed
  (see Laziness below).
- **B. Skip the job with a warning on stderr, and exit 0.** Never blocks,
  but the selection, and for `run`/`retry` what runs, is still decided
  without the fact. Scripts that check only the exit status miss it.

## Rules

1. **Absent is not an error.** A fact that was never recorded still does
   not match a positive condition, as today: an attempt that has not
   started, a log not yet written, a run that was deleted, a carry with no
   origin. "Absent" means the file does not exist (`fs.ErrNotExist`), or a
   timestamp field is empty.
2. **Recorded but unreadable is an error.** A permission or I/O error, JSON
   that does not decode, or a non-empty timestamp that does not parse fails
   the command with an error that names the file, the job ID, and the run.
3. **Laziness stays.** `Selects` checks the result selection, the result
   conditions, and the definition conditions first, and reads attempt files
   only for a job that is still selected. A damaged file of a job that the
   other conditions already excluded does not fail the command. The order
   is documented so the outcome does not depend on the caller.
4. **One rule, in `jobfilter`/`jobstatus`.** `Job.LoadAttributes` and
   `Job.Diagnosis` return an error, and `Filter.Selects` /
   `Filter.SelectsArray` return `(bool, error)`. Callers propagate the error
   and do not implement a policy of their own.

## Scope

Variants to cover: plain jobs; array tasks under `--partial-array`; arrays
decided as a whole (`SelectsArray`: any task's error fails, since "some task
matches" cannot be decided without it, but tasks are read in order and the
first matching task decides, so tasks after it are not read); matrix members; carried results
(the error names the source run's file); latest versus specific attempts
(`AttemptID` origins).

Callers:

| Command | Path |
| --- | --- |
| `show` (job table, run view, `--json`) | `cmd/rotari/show.go` `selectsShownJob`, `show_json_job.go` |
| `copy` | `queueedit.selectCommands` via `queueops/copy.go` |
| `run` / `retry`, including a remote run through the server | `run.PlanRerun` via `projectrun/origins.go`; check that `server` returns the error to the client |
| `cancel` / `suspend` / `resume` | `jobcontrol.Controller.attributes` (calls `jobstatus.AttemptAttributes` and `MatchesAttributes` directly) |

The Web UI/API does not apply `--filter-*` conditions. The Python client
wraps the CLI (`generated_cli.py`), so it sees the CLI's failure. Neither
needs code; both are confirmed during implementation.

Helpers that change: `jobstatus.FilterJob`, `AttemptAttributes`, `ReadLog`,
and the carry lookup in `locateAttempt`, which already returns the error.
They gain error results that keep `fs.ErrNotExist` as absence.
`state.ResolveAttemptTimestamp` and `executor.LoadWrapperStatus` gain
error-returning forms for this use. Their other callers are left alone
unless they share the same fallback.

## Non-goals

- `jobstatus.Artifacts`, which also drops a carry error, is not a filter.
  It stays an ISSUES.md item.
- No change to what matches when facts are readable.
- No `--ignore-unreadable` escape hatch. Add one later only if needed.

## Contracts and documentation

- SEL-11 (Filters) gains a bullet for rules 1–3. SEL-12 says the same for
  job control.
- `docs/CLI_REFERENCE.md`, where the filter options are described.
- Conformance: a `TestSelectorTable` row (or a sibling table) with a
  damaged `status.json` and an unreadable log for `show`, `copy`, and
  `retry`, expecting a failure that names the file; a row showing that a
  missing file still just does not match; and a damaged file on a job the
  result filter already excludes, which still succeeds. A job-control
  case goes under SEL-12.

## Tests and validation

- Unit: `jobfilter` (error propagation through `Selects`/`SelectsArray`,
  laziness order), `jobstatus` (absent versus unreadable for each fact,
  carried origin), `run`, `queueedit`, `jobcontrol`.
- Regression first: a test with a damaged `status.json` under
  `--filter-host` that fails before the change because the job is silently
  unselected.
- `go test ./conformance`, then `scripts/check.sh` (npm is broken on this
  machine: run `tsc` directly).

## Status

Implemented on 2026-11-17; see [work-log.md](work-log.md). The Web UI has no
`--filter-*` path, and the Python client runs the CLI, so neither needed a
change. `jobcontrol` now decides through `Filter.Selects` instead of calling
`MatchesAttributes` itself, so it reads an attempt only for a host or time
condition. `jobstatus.Artifacts` remains an Open item in ISSUES.md.
