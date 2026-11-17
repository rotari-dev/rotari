# Failure kinds from diagnoses

Created: 2026-11-17

## Purpose

`model.FailureKinds` reads only a result's recorded `error` text. A job
that ran out of memory but whose error is a bare exit code, such as a
Python `MemoryError` or a host OOM that only the log shows, gets a memory
diagnosis (`Host memory exhausted`, `Python memory error`, ...) but not the
kind `oom`. So `--filter-failure-kind oom` misses it, and so do `retry
--filter-failure-kind oom` and the Web history search. Recorded in
[ISSUES.md](../ISSUES.md) on 2026-11-04 ("The `oom` failure kind reads only
the recorded error"). That item asks to decide this together with rule
categories ("Diagnosis rules have no class for several common failures").

## Decisions

Decided by the user on 2026-11-17:

1. **A.** Each rule declares the failure kinds it implies, and the saved
   diagnosis records them in an optional `kinds` field (no state version
   bump). `FailureKinds` adds them. Results saved before this change keep
   today's kinds.
2. **`oom` includes GPU memory.** The rules: `Slurm memory limit exceeded`,
   `LSF memory limit exceeded`, `Host memory exhausted`, `Runtime memory
   exhausted`, `Python memory error`, and `CUDA/GPU memory exhausted`.
3. **Scope.** Only the kind mechanism and the `oom` mapping. Rule categories
   and new rules are separate plans.

## Design

- `diagnose.Rule.Kinds`. A test checks every value is a failure kind, and
  that only the rules listed in decision 2 carry `oom`.
- `diagnose` copies a matched rule's `Kinds` into the `RuleDiagnosis` it
  returns. That covers both the saved analysis and the recomputed one
  (`--filter-diagnosis`, Web, reports).
- `model.FailureKinds(result)` adds each saved diagnosis's kinds, in
  `failureKinds` order and without duplicates. `error` is still the
  fallback when no kind applies. A success has no kinds.
- `FailureCause` is unchanged. Its order (recorded causes, then the first
  saved diagnosis, then the first remaining kind) already groups such a job
  under its memory diagnosis.
- Every interface that reads kinds goes through `FailureKinds`: job
  filters (`show`, `copy`, `run`/`retry`), failure summaries, the Web
  history search, and notifications' failure kind field. Check each one.

## Contracts and documentation

- SEL-11's `--filter-failure-kind` bullet says a kind may also come from a
  saved diagnosis.
- The failure-kind description in `docs/INSPECT.md` and
  `docs/LOCAL_DIAGNOSIS.md` (where rules are listed).
- A conformance row: a job that prints a Python `MemoryError` and exits 1
  is selected by `--filter-failure-kind oom` in `show`/`retry`.

## Tests and validation

- Regression first: the conformance row fails before the change.
- Unit: `model.FailureKinds` with saved kinds; `diagnose` copies kinds;
  the rule-kind vocabulary test.
- `go test ./conformance`, then `scripts/check.sh` steps (npm is broken
  here).

## Status

Implemented on 2026-11-17; see [work-log.md](work-log.md).
