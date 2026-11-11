# Work log: test-suite flakiness

## Plan and decisions

Commit: recorded with this entry (2026-11-12, plan only).

- **Change:** Added [plan.md](plan.md) for the eleven open intermittent-test
  items in ISSUES.md (F1 to F11), with what they have in common and four
  phases.
- **Reason:** The third of three stabilization plans agreed with the user on
  2026-11-11. The second,
  [2026-11-11-shared-rule-violations](../2026-11-11-shared-rule-violations/plan.md),
  finished at `5c45398b`.
- **Plan impact:** The user settled D1 to D4, each with the recommended
  option:
  - D1: a repeat runner under load.
  - D2: "fixed" means a cause shown by a reproduction, with no loosened
    bounds.
  - D3: an item that does not reproduce is closed with that evidence.
  - D4: conformance tests keep their state and output when they fail.
- **Validation:** Documentation only; no tests run.
- **Remaining:** Phases 1 to 4.
