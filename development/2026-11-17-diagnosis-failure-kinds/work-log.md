# Work log: failure kinds from diagnoses

## 2026-11-17 18:55:12 +0900 — fe2a9a80: oom from memory diagnoses

**Change:** `diagnose.Rule` gained `Kinds`, and `default_rules.go` sets
`oomKind` on `CUDA/GPU memory exhausted`, `Slurm memory limit exceeded`,
`LSF memory limit exceeded`, `Host memory exhausted`, `Runtime memory
exhausted`, and `Python memory error`. `Diagnose` and the exit-code path
copy a rule's kinds (`slices.Clone`) into `model.RuleDiagnosis.Kinds`, a new
optional `kinds` JSON field saved with the summary (no state version bump,
per the version policy). `model.FailureKinds` collects the error-text kinds,
the exit-code kinds, and every saved diagnosis's kinds, returns them in
`FailureKindValues` order without duplicates, and uses `error` only when
none applies. `FailureCause` is unchanged. SEL-11, its status row,
`docs/INSPECT.md`, and `docs/LOCAL_DIAGNOSIS.md` say so. In ISSUES.md the
`oom` item moved to Resolved, and the rule-category item was narrowed.

**Reason:** the plan's decisions 1 (A), 2 (GPU included), and 3 (scope).

**Plan impact:** plan complete. Rule categories and the missing rules
(file locks, corrupt data, NaN losses, Go deadlocks, R errors) remain an
Open item for their own plans.

**Validation:** `TestMemoryDiagnosesAreFailureKindOOM` (conformance: `show`
and `retry --dry-run` with `--filter-failure-kind oom`) failed before the
change: neither memory job was listed. It passed after.
`TestFailureKindsAddSavedDiagnosisKinds` and
`TestRuleKindsAreFailureKindsAndOOMIsMemory` passed. gofmt, `go vet ./...`,
`-race` on `model`, `diagnose`, `jobfilter`, and `runlineage`, and the full
`go test -timeout 15m ./...` passed under `taskset -c 0-7` at load average
about 58. No JavaScript changed, so `tsc` was not run. pre-commit was not
run (npm is broken here).

**Remaining:** none in this plan.
