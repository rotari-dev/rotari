# Work log: shared-rule violations

## Plan and decisions

Commit: `d9512117` (2026-11-11, plan only)

- **Change:** Added [plan.md](plan.md) for three ISSUES.md items: run order,
  task origins, and PID identity.
- **Reason:** The second of three stabilization plans agreed with the user on
  2026-11-11.
- **Plan impact:** The user settled D1 to D3, each with the recommended
  option:
  - D1: the recorded last run first, then start order.
  - D2: `LoadRunOrigin` reports an unreadable file, and `OriginOf` inherits
    array origins.
  - D3: control from another PID namespace is refused.
- **Validation:** Documentation only; no tests run.
- **Remaining:** Phases 1 to 5.

## Phase 1: survey and reproduce

Commit: `17cb01ea` (2026-11-11, plan and work log only).

- **Change:** Added the Phase 1 results to [plan.md](plan.md). Corrected the
  D1 tie-break: the summary's `started_at` has second precision, so ties
  within a second use the coordinator's `recorded_at` in `context.json`.
- **Reason:** Phase 1: reproduce each item before changing behavior.
- **Plan impact:**
  - All three items reproduced; see the plan.
  - The PID-namespace display case did not reproduce for `show`, and the
    client-attachment case was not tried.
  - Found and fixed outside the scope: the no-silent-fallbacks regression
    (`fbed25f0`; see that plan's work log).
- **Validation:** Scratch binary from `d9512117`, run with `env -i` and an
  isolated `HOME`, config, state, and basedir.
  - Run order: three runs renamed to share a second, compared through
    `/api/state`, `show`, `show latest`, and `delete`.
  - Task origins: an array retry with and without `task_origins`, compared
    through `show`, `show --success`/`--failed`, `jobs`, and `/api/run`.
  - PID identity: a local job controlled from
    `unshare --user --map-root-user --pid --fork --mount-proc`.
- **Remaining:** Phases 2 to 5.

## Phase 2: run order

Commit: recorded with this entry (2026-11-12).

- **Change:**
  - Added `state.RunsNewestFirst` and `state.NewestRunID`
    ([run_order.go](../../internal/state/run_order.go)), implementing D1.
  - Moved `resolve.RunID`, `resolve.newestSettledRunID`, and `delete`'s new
    last run onto it, replacing the directory-mtime rules.
  - The Web loaders (`webRunIDs`, the index, full project state, history
    search options, All shells' run count) use the shared order. The index
    shows its two newest runs, and the Web API sends each project's
    `run_order`.
  - Removed a sixth rule found in this phase: the page's `latestRun` picked
    by load sample or start time, and its run lists sorted by ID. Both now
    follow `run_order` (`orderedRuns` in `web_app_tables.js`).
  - `runs` falls back to the start in the run ID instead of the directory
    time when a run has no recorded start.
  - Defined the run ID format once: `state.NewRunID` and `state.RunIDStart`
    replace `makeRunID`'s layout, `runIDLen` in `cmd/rotari`, and
    `runlist.RunIDStart`.
- **Reason:** Phase 2 of the plan, item 1.
- **Plan impact:**
  - Added RES-29 and pointed RES-12 and RES-15 at it.
  - Documented the latest run in CONCEPTS.md, and the new files in
    ARCHITECTURE.md.
  - Moved the ISSUES.md item to Resolved.
  - Decided here: a run directory whose name records no start sorts oldest,
    by name. A `context.json` that cannot be read for a same-second tie is an
    error naming the run.
  - The history search's run options are now newest first; they were in
    name order.
- **Validation:**
  - `TestRunsNewestFirst` and `TestRunIDStart` (unit).
  - `TestOneRunOrder` (conformance, RES-29 and RES-15) fails on `17cb01ea`
    with each Phase 1 finding, and passes now.
  - `TestRunIDFallsBackToNewestRun` asserted the mtime rule. It now has
    directory times opposite to start order.
  - `go vet ./...`, `tsc`, `go test ./...`, `go test -race ./...`. The first
    full run failed `TestReadCountsStayBounded` (`show`: 4 runs listings,
    bound 3). `NewestRunID` now returns a recorded last run that exists
    without listing, as `resolve.RunID` did; it is first in the order anyway.
  - The ISSUES.md `bug-report` check was not run. It needs an independent
    agent, which the user has not asked for in this session.
- **Remaining:** Phases 3 and 4, each with its contracts and docs.
