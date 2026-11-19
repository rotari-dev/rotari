# Plan: Cross-Basedir Concurrency Pools

Created: 2026-11-19
Status: Proposed; implementation not started

## Purpose and scope

Let runs in different projects and basedirs on one host share a concurrency
limit. Today `--local-concurrency`, `--batch-concurrency`, and
`--<executor>-concurrency` bound only the jobs of one run (one supervisor
process), and a project has one active run at a time, so nothing bounds the sum
across projects or basedirs.

A **named pool** is a limit that is defined once, by name, for the whole master
directory. Every run that names the pool shares it, whatever its basedir or
project. Pools with different names are independent. A run that names no pool
behaves exactly as today.

### Non-goals

- Sharing a limit across hosts, or over NFS. Pools are host-local; see
  "Constraints".
- A central scheduler daemon. Runs coordinate only through files.
- Priorities or fairness between pools. Waiting order inside a pool is
  best-effort.
- Changing scheduler-side (Slurm/PBS/LSF/SGE) queue priority.
- Changing what `--*-concurrency` mean; a pool is an additional limit.

## Decisions

1. **Scope:** a pool name is global to the master directory, not to a basedir.
2. **Where limits live:** pool limits are defined in configuration
   (`[pools]` table: `name = limit`), not on the command line, so two runs cannot
   disagree about the size of one pool. A run selects a pool with `--pool NAME`
   (also settable in project/basedir config as `pool`). An unknown pool name is
   an error, not a fallback to "unlimited".
3. **Mechanism:** one slot file per permitted slot,
   `<masterdir>/pools/<name>/slot-<n>.lock`, held with an exclusive `flock`
   for as long as a job occupies the slot. The kernel releases the lock when the
   holding process dies, so a killed supervisor never leaks a slot and no stale
   cleanup is needed. A holder writes run ID and job ID into the file for display
   only; correctness never depends on its contents.
4. **Acquisition:** non-blocking try-lock over slots `0..limit-1`, retried by the
   dispatcher while the job waits (short interval with jitter). Waiting must stay
   cancellable: a cancel reaching a job that waits for a slot records a cancelled
   result and never starts it.
5. **Slot lifetime:** a job holds a slot from just before submission to the moment
   its result is recorded, matching how a lane slot is held today. For scheduler
   executors this means submitted-and-not-finished, as `--batch-concurrency` does.
6. **Combination with lanes:** a job needs both a free lane slot and a free pool
   slot. Take the lane slot first, then the pool slot, and release in reverse
   order, so a waiting job never holds a pool slot.
7. **Limit changes:** the limit is read when a run starts. Lowering it does not
   evict running jobs; it only stops new acquisitions beyond the new limit.
   Raising it takes effect for runs started afterwards. Slot files beyond the
   limit are ignored, not deleted while held. See risk 2 for why the limit is
   exact only while the supervisor lives (or, for local jobs, while the job lives).

## Constraints

- `flock` is reliable on local filesystems only. The master directory may be
  on NFS in some deployments. Decided: check at pool open and fail or warn
  (see "Lock capability checks"). Follow the locking convention in
  `internal/state/lock.go` rather than adding a second.
- Pool names are arbitrary strings, not paths. Reject `/` and `\` (and empty,
  `.`, `..`) at the shared boundary before building a directory name.
- Errors (cannot create the pool directory, cannot lock) must surface and fail
  the job or run; they must not degrade to running without the limit.

## Implementation approach

### Phase 1: pool package and configuration

- `internal/pool`: add the capability checks (self-test, error on unusable
  locks, network filesystem warning) and their tests.
- New package `internal/pool` (no CLI, Web, or executor knowledge):
  `Open(masterDir, name, limit)`, `TryAcquire(owner) (Slot, bool, error)`,
  `Slot.Release()`, and `Status(...)` listing holders and free slots. Name
  validation lives here and is the only way to build a pool path.
- `internal/config`: parse and merge the `[pools]` table and the `pool`
  setting; include them in the canonical snapshot. Reject non-positive limits.
- Unit tests: mutual exclusion across goroutines and across processes (a helper
  subprocess), release on process death, name validation, limit shrink/grow.

### Phase 2: dispatcher integration

- In [internal/run/dispatch.go](../../internal/run/dispatch.go), give a `lane` an
  optional pool. Acquire after the lane slot (decision 6) in the paths that start
  a job: local (`runLocal`) and one-by-one batch submission (`submitAndWait`).
  Native arrays are the exception; see risk 1.
- Re-check `cancelledBeforeStart` after a pool slot is obtained and before
  submission.
- Plumb `Pool` through `projectrun` options and `cmd/rotari` flags
  (`--pool`, env `ROTARI_RUN_POOL`), with the usual config/flag precedence.
- Record the pool name in the run's persisted settings so retry and any
  recovery path use the same pool.

### Phase 3: visibility

- A job waiting for a pool slot is distinguishable from one waiting for a
  dependency or a lane slot, in `show`/`wait`/queue views, Web UI/API, and the
  Python client. Reuse the existing waiting-state vocabulary if it fits; do not
  invent a new job state without reading `contracts/02`.
- Pool status (limit, in use, holders) via CLI, Web API, and Python client.
  Decide between a new command and an existing list command after reading
  `cmd/rotari/lists.go`.

### Phase 4: contracts and documentation

- Contract IDs in `contracts/04-coordination-and-safety.md`: pools are shared
  across basedirs; a dead holder frees its slot; an unknown pool is an error; a
  job waiting for a slot can be cancelled; held slots never exceed the configured
  limit while their holders live (best-effort after a supervisor crash for jobs
  that run elsewhere, risk 2). Update the status table in `contracts/README.md`.
- Conformance tests through the binary: two projects in two basedirs with a
  pool of 1 never overlap; kill a supervisor mid-job and confirm the slot is
  reusable.
- User docs: `docs/CONFIGURATION.md`, `docs/CLI_REFERENCE.md`,
  `docs/ENVIRONMENT_VARIABLES.md`, `docs/EXECUTORS.md` (concurrency section),
  `docs/FAQ.md` ("How are concurrency and executor options selected?"), and
  `docs/ARCHITECTURE.md` (package table). Do not edit `README.md` directly.

## Risks found in review

Found by reading `internal/run/dispatch.go` and the local executor. Each needs a
decision before or during phase 1/2.

1. **Native arrays bypass lane slots.** `runArray` submits a whole array to the
   scheduler without taking concurrency slots ("the scheduler limits its tasks").
   A pool applied only where lane slots are taken would not bound arrays at all.
   Options: (a) exempt native arrays and document it; (b) hold one pool slot per
   task, which needs all-or-nothing acquisition and can never succeed if the
   array is larger than the limit, and two runs each holding part of their array
   can deadlock; (c) submit pooled arrays task by task (the existing fallback
   path), losing native array submission. Proposed: (c) for pooled runs, since
   it reuses the existing path and keeps the limit true; document the cost.
2. **A pool slot can outlive its lock.** Local jobs run in independent
   wrappers that survive a killed supervisor, and scheduler jobs survive it too.
   The `flock` is released when the supervisor dies, so the pool then undercounts
   running jobs and the limit can be exceeded. For local jobs, pass the slot's
   file descriptor to the wrapper (`ExtraFiles`; Go otherwise sets close-on-exec)
   so the lock lives as long as the job process. Do not inherit it into
   anything that outlives the job. For SSH and scheduler executors the job runs
   elsewhere, so only the supervisor can hold the lock: the limit is
   best-effort after a supervisor crash. The contract must say so; do not
   promise "never exceeds" for those executors.
3. **Submission order chain.** `lastSubmitted` makes each job wait for the
   previous job's submission to preserve scheduler order. A job waiting for a
   pool slot therefore holds back every job behind it in the same lane, and all
   of them hold lane slots. That keeps order and means one poller per lane per
   run, but it leaves lane slots idle while the pool is full. Accept, and say so
   in the docs. Pool acquisition must come before `submitted()` is called and
   must call it on every exit path (cancel, error), or the lane stalls.
4. **Cancel and shutdown while waiting.** The wait loop must re-check
   `JobCancelled`, the project's `cancelling` phase, and run shutdown on each
   poll, and record the cancelled result. Without it, `run cancel` hangs behind a
   full pool.
5. **Suspended jobs keep their slot.** `Suspend` sends SIGSTOP; the job still
   counts. This is consistent with lane slots, but state it in the docs because a
   suspended job blocks other runs.
6. **Pool definitions must not diverge.** Config merges global, workspace,
   basedir, and project layers. If `[pools]` were allowed in basedir or project
   config, two runs could define different limits for one name. Allow `[pools]`
   only in global config (reject elsewhere with an error), and let the run's
   frozen snapshot record the limit it started with.
7. **Same master directory required.** Runs share a pool only if they share a
   master directory (`--masterdir`, `ROTARI_MASTERDIR`). Different users or
   a custom `--masterdir` get separate pools without any error. Document it. If
   several users share a host, slot files need permissive modes (and a directory
   with the right group); decide whether multi-user is in scope. Default:
   single user, files `0600`.
8. **Fairness and starvation.** Try-lock polling has no queue, so an unlucky
   waiter can starve. Jitter makes this unlikely, and one poller per lane limits
   the load, but the plan promises no FIFO. If starvation shows up, add a ticket
   file for ordering.
9. **Retries and carried results.** A retry goes back through the dispatcher and
   takes a new slot; it must not hold the old one across the retry delay.
   Carried results never reach the dispatcher and take no slot.
10. **Existing runs and snapshots.** The pool name and limit become run
    settings. Runs created before the change have none and must behave as before.
    Old binaries ignore an unknown setting, so a mixed fleet silently skips the
    limit; mention it in the release notes.
11. **Interface coverage is large.** `--pool` must reach `cmd/rotari`, the web
    server's run launch, the MCP run start, the Python client, and config. Miss
    one and that path runs unlimited. Add a conformance test per entry point,
    and a test that every run-start path rejects an unknown pool.
12. **Scale of change.** A new package, config schema, dispatcher change,
    executor change (descriptor passing), run settings, status views, contracts,
    and docs. Land it in the four phases above as separate commits, with phase 1
    and 2 usable behind `--pool` before the visibility work.

## Decided

Decisions by the requester on 2026-11-19, which close the matching open
questions and risks below:

- **Arrays:** a run that uses a pool submits array tasks one by one (risk 1,
  option c). Native array submission is lost for pooled runs; accepted.
- **Supervisor crash:** for SSH and scheduler executors the limit is best-effort
  after a supervisor crash (risk 2); accepted and documented.
- **Locking must not fail silently:** see "Lock capability checks" below.

### Lock capability checks

A pool whose locks do not work must produce an error or a warning, never a
silently unlimited run.

- **Error** (the run does not start, or the job fails with the cause) when:
  `flock` returns anything other than success or `EWOULDBLOCK`/`EAGAIN`
  (for example `ENOLCK`, `EOPNOTSUPP`), or when the self-test fails.
- **Self-test** when a run opens a pool: open one probe file twice, take the
  exclusive lock on the first descriptor, and require that a non-blocking
  exclusive lock on the second fails with `EWOULDBLOCK`. A filesystem or mount
  that ignores or fakes `flock` fails this test. Run it once per run and pool,
  in the pool directory, so it tests the filesystem that will hold the slots.
- **Warning** (logged by the run and shown by the pool status command) when the
  filesystem holding the pool directory is a network filesystem (NFS, CIFS,
  FUSE), even if the self-test passes. The self-test cannot show that another
  host sees the same locks, and pools are specified as host-local. Reuse the
  filesystem type detection in `internal/bugreport/fstype_linux.go`, moved to a
  shared package rather than copied, and the existing lock conventions in
  `internal/state/lock.go`.
- An unsupported platform (no `flock`) is an error when a pool is requested.
- Unit tests: a fake failing lock yields the error; a network filesystem type
  yields the warning; a passing local filesystem yields neither.

## Variants to cover

Plain jobs, array tasks and arrays, matrix members, retries and older attempts,
carried (not re-executed) results which take no slot, local and each batch
executor, and every interface: CLI, server, Web UI/API, Python client.

## Open questions

- NFS master directory: resolved, see "Lock capability checks".
- Per-job pool selection: proposed per run and config default first, per job
  later.
- Polling versus a wake-up mechanism: proposed polling with jitter first;
  measure before optimizing.

## Tests and validation for completion

- `go test ./internal/pool/... ./internal/run/... ./internal/projectrun/... ./internal/config/...`
- The conformance tests above, with contract IDs in sync with
  `contracts/README.md`.
- Manual check: two `rotari run` invocations in different basedirs sharing a
  pool of 1; observe serialized execution, a waiting state in `show`, and slot
  recovery after `kill -9` of one supervisor.
