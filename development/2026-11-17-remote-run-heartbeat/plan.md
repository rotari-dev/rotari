# Notice runs whose coordinator on another host stopped

Created: 2026-11-17

## Purpose

A run whose coordinator is killed, or whose host is lost, never writes
`RunFinished`. `rotari web` records such runs for the inbox (NOT-4, phase 8
of [persistent-notifications](../2026-10-31-persistent-notifications/plan.md)),
but only when `project.Inspect` can prove the coordinator is gone: the lock
names this host and PID namespace, and the PID is dead. A lock from another
host or PID namespace is `LockRemote` and counts as running forever, so a
lost login node's or container's runs are never reported. They also stay
"running" in `show`, `runs`, and the Web UI until someone runs `unlock`.
Recorded in [ISSUES.md](../ISSUES.md) on 2026-10-31.

Closing the gap needs evidence that can be read from any host: a heartbeat
that the coordinator writes into the run directory.

## Constraints from earlier decisions

- A stale heartbeat is **not** proof that a run stopped. The coordinator may
  be suspended (`SIGSTOP`, a sleeping laptop), its host partitioned, or the
  shared file system stalled. [verified-unlock](../2026-10-20-verified-unlock/plan.md)
  already says a heartbeat timeout cannot authorize `unlock`. Here it only
  produces a notice that the run *may* have stopped. The project stays
  locked, and nothing is finalized or released.
- `unlock` reports nothing (decided 2026-11-02); unchanged.
- One record per run across servers comes from
  `notificationlog.Log.AppendOnce`, as for local interrupted runs.

## Design

### Heartbeat

- The coordinator writes `heartbeat` in the run directory when the run
  starts and every 30 s until it finishes. The file holds a sequence number,
  the coordinator's own clock, and its host and PID namespace. It is written
  with a temporary file and a rename, so a reader never sees half of it.
  A failed write is logged in the run's progress events. It does not stop
  the run.
- `load_samples.jsonl` is not reused. It is written only when the load
  average can be read, and its write errors are dropped.
- The heartbeat is removed, or left as the last beat, when the run
  finishes. The lock and summary already say the run finished, so readers
  ignore the heartbeat of a run that is not active.

### Staleness without trusting clocks

Hosts that share a file system do not share a clock, and NFS caches file
attributes, so neither the recorded time nor the file's mtime can be
compared with the reader's `now` exactly.

- **`rotari web` (the watcher):** it remembers, per remote run, the
  sequence number it last saw and when it saw it, by its own clock. A run
  whose sequence has not changed for the threshold (default 5 min, ten
  missed beats) is stale. Clock skew does not matter. A restarted server
  waits one threshold before it can call a run stale.
- **One-shot readers (`show`, `runs`, Web pages):** they have no earlier
  observation, so they compare the recorded time with `now`, with the same
  threshold plus a margin for skew. They only *display* this (decision 1);
  they never record notifications.

A run with no `heartbeat` file was started by an older binary. It is
never called stale, because nothing can be told about it.

### What is reported

- A new inbox event `run.unresponsive` ("no heartbeat from host H for 12m;
  the run may have stopped"), recorded once per run through `AppendOnce`.
  It is selected by `run_failure`, like `run.interrupted`. It is a separate
  event, so that its wording and the actions offered say that nothing has
  been verified.
- If the heartbeat resumes, nothing is retracted automatically. The run's
  own `RunFinished` arrives later as usual.
- Local runs keep today's proof-based `run.interrupted`. The heartbeat is
  read only when the lock is `LockRemote`.

## Decisions

Decided by the user on 2026-11-17:

1. **Display: yes, as phase 3.** `show`, `runs`, and the Web UI mark an
   active remote run whose heartbeat is stale, such as
   `running (no heartbeat 12m)`. This helps when no `rotari web` is running.
2. **Webhook: no.** `run.unresponsive` goes to the inbox only, as
   `run.interrupted` does under NOT-4.
3. **Threshold: fixed at 5 minutes** (ten missed 30-second beats). No
   setting until one is needed.

Whether a resumed heartbeat records anything is left as it is: nothing is
retracted, and the run's own `RunFinished` arrives as usual.

## Non-goals

- Unlocking, releasing, or finalizing a run based on its heartbeat.
- Jobs' own liveness (scheduler and SSH evidence belong to verified-unlock).
- Reporting while no `rotari web` runs (the display in decision 1 covers
  part of this).

## Phases

1. **Heartbeat writer.** `projectrun` writes `heartbeat`; add a `state`
   reader. Unit tests for the format, atomic writes, and the writer
   stopping at finish.
2. **Watcher.** `runwatch` keeps per-run observations, records
   `run.unresponsive` for stale `LockRemote` runs, and keeps the existing
   interrupted path. The notification model, inbox rendering, and `run_failure`
   selection are added. Tests use a fake lock from another host and a
   frozen heartbeat with an injected clock.
3. **Display** (decision 1): `show`, `runs`, the Web run list and
   run page, through one shared function in `runview`/`jobstatus`.
4. Contracts: a new NOT row for `run.unresponsive`, plus the run-state
   display rows. Docs: `docs/NOTIFICATIONS.md`, and
   `docs/RECOVERING.md` on what a notice means and how to verify before
   `unlock`. Conformance through the binary: a run whose lock names
   another host and whose heartbeat stopped is reported by `rotari web`
   once, a live heartbeat is not, and an older run without one is not.

## Tests and validation

Per phase as above. Then `go test ./conformance` and `scripts/check.sh`
(run `tsc` directly, since npm is broken here). A real two-host check, which
kills a coordinator on one login node and watches from another, if the user
has such a setup.

## Status

Phases 1–4 done on 2026-11-17; see [work-log.md](work-log.md). The display
(phase 3) became NOT-7, separate from the notification (NOT-6). A real
two-host check has not been done.
