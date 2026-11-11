# Shared-rule violations: run order, task origins, and PID identity

Created 2026-11-11. Status: done; decisions D1 to D3 settled with the user
on 2026-11-11, each with the recommended option. Phase 1 (survey) done the
same day, Phases 2 (run order), 3 (task origins), and 4 (PID identity) on
2026-11-12.

## Purpose

AGENTS.md: "Implement each cross-command rule once in its owning package;
keep lower-level helpers unexported when exposing them would let a caller
bypass the shared rule." Three open items in [ISSUES.md](../ISSUES.md) are
places where a rule exists but some callers decide it their own way, and get
a different answer:

1. **The Web index picks the latest runs by sorting run IDs.**
2. **The generic origin lookup does not inherit an array's origin for its
   tasks.**
3. **Local job and client PIDs are checked without regard to PID namespace.**

This is the second of three stabilization plans agreed on 2026-11-11. The
first, [2026-11-11-no-silent-fallbacks](../2026-11-11-no-silent-fallbacks/plan.md),
is done; the third is test-suite flakiness.

## Findings

Read from the code on 2026-11-11; Phase 1 reproduces each with the binary.

### Run order (item 1)

"Which run is newer" is decided five ways:

| Where | Rule |
| --- | --- |
| `resolve.RunID` (no run given) | `meta.LastRunID`; if missing, the run directory with the newest mtime |
| `resolve.newestSettledRunID` (`latest`) | newest mtime among settled runs |
| `queueops.latestRunID` (after deleting a run) | newest mtime |
| `webRunIDs` in `internal/webui` (index, full state, history search, All shells' run count) | name order; the index shows the last two names and the running run |
| `runlist.RunIDStart` / `LatestStart` | the start second in the run ID |

A run ID is `YYYYMMDD-HHMMSS-XXXXXXXX`: the start second, then a random
tail. Name order is therefore wrong for two runs begun in the same second.
A directory's mtime changes whenever an entry is added or removed in it, so
mtime orders by last activity, not by start. The summary's `started_at` has
only second precision. The coordinator's `recorded_at` in the run's
`context.json` is written with nanoseconds when the run starts
(`projectrun.Runner.timestampNano`); Phase 1 corrected the plan's first
reading on this. The run ID format is also defined three times:
`runIDLength` in `internal/state/attempts.go`, `runIDLen` in
`cmd/rotari/main.go`, and the layout string in `runlist.RunIDStart`.

### Task origins (item 2)

`model.TaskOrigin(command, task)` gives a task its own entry in
`TaskOrigins`, or else the command's array origin narrowed to that task.
The planner (`projectrun/plan.go`, through `run.TaskOrigin`, a wrapper) and
`project/repair.go` use it. `model.Queue.OriginOf(jobID)` does not: it
returns `TaskOrigins[jobID]` or the command's origin for an exact command
ID, so a task of an array that has only an array-level origin gets none.
Its callers are `joblist` (Recent jobs), `cmd/rotari/show.go`,
`project/repair.go`, and, through `state.LoadRunOrigin`, `jobstatus/facts.go`
(filter facts), `internal/webui`, and `show.go` again.
`state.LoadRunOrigin` also returns nil when `commands.json` cannot be read,
which is a success-shaped fallback. The ISSUES entry asks that each reader's
filter and status behavior be reproduced before its lookup changes.

### PID identity (item 3)

The identity of a recorded process (host, PID namespace, PID, process start)
is checked carefully in two places, each in its own way:
`executor/execution_probe.go` (verified unlock) and `coordinatorLocation` in
`project/coordinator.go`. A third, `state.OtherPIDNamespace`, covers the run
lock (COORD-2, COORD-3). The following do not check the namespace:

- `executor.Local.Cancel`, `Suspend`, `Resume` (`signal`), and
  `LocalProcessGroupAlive` read the job's `pid` file and signal or probe the
  PID directly. New local attempts already record the namespace in
  `job.json` (`localIdentity`), so the information exists.
- `attachment.processMayBeSame` and `model.LaunchOrigin` (client
  attachments) record host, PID, and process start, but no namespace.

From a sandbox or container on the same host, a cancel writes `cancelled`
and signals nothing, suspend and resume report "not running", `show` and
`wait` read the job's process group as gone, and a live client reads as
dead. COORD-1 already refuses control from another host. This extends it to
another namespace.

## Decisions

- **D1. One run order.** The newest run is `meta.LastRunID` when it names
  a run directory. Otherwise, and for every other position, runs are ordered
  by the start second in the run ID. Runs that share a second are ordered by
  the coordinator's `recorded_at` in `context.json`, read only for those
  runs; a run without it, from an earlier rotari, sorts by its ID among them. mtime is no
  longer used. The order and the run ID format live in `internal/state`, next
  to the run directory layout. `resolve` keeps the "latest settled" rule on
  top of it. The Web index shows the two newest runs in this order.
- **D2. One task-origin rule.** `Queue.OriginOf` applies `TaskOrigin` for a
  task ID of an array command. `state.LoadRunOrigin` returns an error for an
  unreadable `commands.json` instead of nil, and each caller says what that
  means for it. The `run.TaskOrigin` wrapper goes.
- **D3. One process-identity rule.** A single check in `internal/state`
  says whether a recorded process (host, PID namespace, PID, process start)
  can be observed from here: same, gone, reused, another host, another
  namespace, or unverifiable. It is used by local cancel, suspend, resume,
  and liveness, by attachments, by the coordinator check, and by the
  execution probe.
  - Control of a local job from another namespace fails with an error naming
    both namespaces, as COORD-1 does for another host.
  - Display reads such a job as unknown, not gone.
  - Client attachments record the namespace.
  - A record without a namespace, from an earlier rotari, keeps today's
    behavior.

### Phase 1 results (2026-11-11)

Reproduced with the binary at `d9512117`, in an isolated `HOME`, config,
and basedir.

- **Run order.** Three real runs were renamed so that runs 1 and 2 share a
  second, with name order opposite to start order:
  `20260101-000000-ffffffff` (started :17), `-00000000` (:18), and
  `20260101-000001-aaaaaaaa` (:20). Run 1's directory was given the newest
  mtime.
  - The Web index (`/api/state`) listed `aaaaaaaa` and `ffffffff`; the two
    newest are `aaaaaaaa` and `00000000`.
  - With `last_run_id` cleared, `show` and `show latest` both chose
    `ffffffff`, the oldest run, by mtime.
  - After `delete` of the newest run, `meta.json` named `ffffffff` as the
    last run instead of `00000000`.
  - `show` with `last_run_id` set chose correctly.
- **Task origins.** An array of three tasks with task 2 failing was run and
  then retried; the retry's `commands.json` has explicit `task_origins`.
  Removing them, leaving the array-level origin, changed:
  - `show -j TASK --json` and `show -r RUN --json`: a carried task lost
    `success (carried)`, `carried`, and its recorded working directory.
  - `show --success`: carried tasks lost their status suffix and finish time.
  - `jobs`: carried tasks showed `success` instead of `success (carried)`.
  - `/api/run`: tasks lost `origin`, `carried`, and the origin's times.
  - `show --failed` and `show --success` selected the same tasks either way,
    so selection, which goes through the planner rule, was not affected.
- **PID identity.** `unshare --user --map-root-user --pid --fork
  --mount-proc` works unprivileged on this machine. With a local
  `sleep 117` job started outside (namespace `pid:[4026531836]`, recorded in
  `job.json`), from inside a new namespace:
  - `suspend` and `resume` said "job is not running".
  - `cancel` reported the job cancelled and wrote `cancelled`, but the
    process kept running.
  - `show` read `running (recorded)` from the run lock, so it was not wrong
    here, contrary to the ISSUES entry.
  - The client-attachment case was not reproduced.
- **Found outside the scope:** the no-silent-fallbacks Phase 2 change made
  `run` and `cancel` fail in a workspace from `rotari init`, whose `[retry]`
  and `[wait]` sections were read as option values. It was fixed separately;
  see that plan's work log.

## Phases

1. **Survey and reproduce.** Done 2026-11-11; results above. Reproduce item 1 with two runs in the same
   second, and show which callers give different answers. Reproduce item 2
   with a snapshot that has only an array-level origin, for each caller of
   `OriginOf` and `LoadRunOrigin`, and record what each shows and filters.
   Reproduce item 3 by running rotari in another PID namespace on this host
   (for example `unshare --pid --fork --mount-proc`, if permitted) against a
   local job. Record anything outside the scope in ISSUES.md.
2. **Run order (item 1).** Done 2026-11-12 (RES-29). Implement D1, move the five rules onto it, and
   consolidate the run ID format. A sixth rule, the Web page's own latest
   run, was found and moved too; see the work log.
3. **Task origins (item 2).** Done 2026-11-12 (RES-30); see the work log. Implement D2 with the per-caller behavior
   recorded in Phase 1 kept or deliberately changed.
4. **PID identity (item 3).** Done 2026-11-12 (COORD-1); see the work log. Implement D3. Extend COORD-1 to another
   namespace.
5. **Contracts and documentation**, with each phase: the affected RES, SEL,
   COORD, and WEB notes, the user docs, and the ISSUES.md entries moved to
   Resolved.

## Tests and validation

- Run order: a conformance test with two runs whose IDs share a second, in
  an order where name order and start order disagree. Check the Web index,
  `show` with no run, `latest`, and the project's last run after a delete.
- Task origins: an inherited-array case for `show`, Recent jobs, filter
  facts (`--failed` and similar through `jobstatus.FilterJob`), and the Web
  job view, through the binary and the Web API.
- PID identity: unit tests of the shared check for each verdict. A
  conformance test in another namespace if the CI environment permits
  `unshare`; otherwise the test skips with the reason, and Phase 1 records
  how it was verified by hand.
- `go test ./conformance`, then the `scripts/check.sh` steps.

## Non-goals

- Changing what "latest" means for settled runs, or how runs are listed by
  time window (`runs --since`).
- Process identity on remote executors (SSH and schedulers), which have their
  own native IDs.
