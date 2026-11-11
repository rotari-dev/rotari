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

Commit: recorded with this entry (plan and work log only).

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
