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

Commit: `8c122ccd` (2026-11-12).

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

## Phase 3: task origins

Commit: `9e4cb8e8` (2026-11-12).

- **Change:**
  - `model.Queue.OriginOf` gives a task of an array its `TaskOrigin`: its
    own entry, or the array's origin narrowed to the task.
  - A second copy of the lookup, found in this phase, does the same:
    `model.QueueOriginsByJobID`, used by `show --json`, the Web run view, and
    `runview`. This is why `/api/run` lost origins in Phase 1.
  - Removed the `run.TaskOrigin` wrapper. The planner and `rerun.go` call
    `model.TaskOrigin`, and `repair.go` uses `OriginOf`.
  - `state.LoadRunOrigin` returns an error naming `commands.json` when it
    cannot be read. `locateAttempt` and `jobstatus.ResultAttemptDir` pass
    that error on; a carry from a deleted run is still "not found".
  - What each reader does with the error:
    - `show --follow` fails.
    - `show` of a carried job prints the error.
    - The run report says it in place of the log.
    - `/api/log` answers with it.
    - `FilterJob` and `jobstatus.Artifacts` keep "not found", with a
      comment: filters have no way to report an error.
- **Reason:** Phase 3 of the plan, item 2.
- **Plan impact:**
  - Phase 1's per-caller differences in `show`, `jobs`, and `/api/run` are
    now deliberately changed to carried. Selection was the same before and
    is the same now.
  - Added RES-30, and updated the `TaskOrigins` note in the run-lifecycle
    contract and ARCHITECTURE.md.
  - Moved the ISSUES.md item to Resolved, and opened one: job filters cannot
    report a state file they fail to read.
  - No user guide describes the inherited case, so no guide changed.
- **Validation:**
  - `TestOriginOfArrayTasks` (unit), added beside the existing
    `TestTaskOrigin`. A first version of the commit overwrote that file and
    lost `TestTaskOrigin`; it was restored before the commit was final. Its
    fixture showed that a `nil` `TaskOrigins` entry inherits in
    `TaskOrigin`, while `OriginOf` returned `nil` for it. `OriginOf` and
    `QueueOriginsByJobID` now skip `nil` entries, as `TaskOrigin` does.
  - `TestArrayTasksInheritTheArrayOrigin` (conformance, RES-30) fails on
    `8c122ccd` with all six checks, and passes now.
  - A first version of the test followed a second retry through an
    unreadable first retry and passed on both versions. A carried task's
    origin records the producing attempt ID, so the chain is never read.
    The test now corrupts the shown run's own `commands.json`.
  - `go vet ./...`, `tsc`, `go test ./...`, `go test -race ./...`.
  - The ISSUES.md `bug-report` check was not run, as in Phase 2.
- **Remaining:** Phase 4.

## Phase 4: PID identity

Commit: recorded with this entry (2026-11-12).

- **Change:**
  - Added `state.ProcessIdentity` and `state.ObserveProcess` /
    `ObserveProcessGroup`
    ([process_identity.go](../../internal/state/process_identity.go)),
    implementing D3. They return one of: same, gone, reused, another host,
    another namespace, or unverifiable, and say whether the location was
    verified.
  - Pure parts for callers and tests: `ProcessIdentity.Locate`,
    `ClassifyProcess`, `ProbeOutcome`, and `Elsewhere` /
    `ElsewhereError`.
  - Moved onto it:
    - Local `Cancel`, `Suspend`, `Resume`, and `LocalProcessGroupAlive`,
      through one reader, `executor.LocalJobIdentity` (`job.json`, else the
      `pid` file).
    - `executor.LocalElsewhere`, which replaces `LocalHostMismatch`, and
      `runnerHostMismatch`.
    - `state.OtherPIDNamespace` (the run lock).
    - `coordinatorLocation` and `coordinatorProcess`.
    - The execution probe's local check and `localGroupEvidence`.
    - Attachment liveness (`clientMayBeLive`, which replaces
      `processMayBeSame`) and `SameLaunchOrigin`.
  - Attachments and `model.LaunchOrigin` record `pid_namespace`.
  - Each caller keeps its wording: the coordinator and probe reasons are
    unchanged, and a verified location is still required for them to probe.
- **Reason:** Phase 4 of the plan, item 3.
- **Plan impact:**
  - COORD-1 is extended to another PID namespace. Job control refuses with
    `job "ID" runs in PID namespace X, not Y; run OP outside this sandbox or
    container` before anything is written or signalled.
  - Local control now reads a reused leader PID as not running, and does not
    signal it. `suspend` and `resume` probe the process group, like `cancel`
    and liveness, rather than the leader alone.
  - Liveness still counts a reused leader's group as alive, as the probe
    calls it possibly active.
  - Records from an earlier rotari, without a namespace or host, are probed
    as before.
  - Updated COORD-1 and its README row, the coordination notes, FAQ, and
    ARCHITECTURE.md, and moved the ISSUES.md item to Resolved.
- **Validation:**
  - `TestProcessLocation`, `TestClassifyProcess`, `TestObserveProcess`, and
    `TestLocalElsewhere` (unit). `TestLocalElsewhere` replaces
    `TestLocalHostMismatch` with the same host case, plus namespace cases.
  - The coordinator and probe evidence tables and the attachment PID-reuse
    test now go through the shared check, with the same cases. The
    attachment test adds another namespace.
  - `TestControlFromAnotherPIDNamespace` (conformance, COORD-1) runs rotari
    under `unshare --user --map-root-user --pid --fork --mount-proc`, and
    skips where that is not permitted. On `9e4cb8e8` it fails as Phase 1
    found: `suspend` and `resume` say not running, and `cancel` exits 0 and
    writes `cancelled`. It passes now.
  - `go vet ./...`, `tsc`, `go test ./...`, `go test -race ./...`.
    - The first full run failed `TestControlFromAnotherHost`. It stands for
      another host by rewriting only `context.json`, and `LocalElsewhere`
      let `job.json` decide.
    - A local job runs on its coordinator's host, so the two should agree.
      `LocalElsewhere` now refuses when either places the job elsewhere,
      which keeps the earlier host check exactly.
  - The ISSUES.md `bug-report` check was not run, as in Phases 2 and 3.
- **Remaining:** none in this plan. Next is the third stabilization plan,
  test-suite flakiness.
