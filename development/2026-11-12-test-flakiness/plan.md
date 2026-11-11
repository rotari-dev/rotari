# Test-suite flakiness

Created 2026-11-12. Status: planned; decisions D1 to D4 settled with the
user on 2026-11-12, each with the recommended option. Phase 1 is next.

## Purpose

This is the third of three stabilization plans agreed on 2026-11-11. The
first two were
[2026-11-11-no-silent-fallbacks](../2026-11-11-no-silent-fallbacks/plan.md)
and
[2026-11-11-shared-rule-violations](../2026-11-11-shared-rule-violations/plan.md).

Eleven open items in [ISSUES.md](../ISSUES.md) are tests that failed once,
or a few times, and then passed when run alone. Each failure costs a rerun,
and a suite that fails at random teaches its readers to ignore failures,
which hides real regressions. Each item now says "capture more on
recurrence". None has a cause, a reproduction, or a fix.

The goal is that each item ends in one of three states:

- fixed with its cause shown;
- found to be a real product bug and fixed in the product;
- closed with evidence that it no longer occurs, under a defined load.

## Findings

Read from ISSUES.md on 2026-11-12. Phase 1 reproduces each item.

### The open items

| Item | Test | What failed | Seen |
| --- | --- | --- | --- |
| F1 | `TestSSHJobReadsAsRunning` (02-lifecycle) | `show --json` read the job `unknown`, not `running (recorded)` | race step 2026-10-31; full suite 2026-11-04 (load ~50), 2026-11-05 |
| F2 | `TestFilteredRerunCarriesCompletedResults` (02-lifecycle) | the summary after `run --failed` was still the first run's | 2026-10-28 (load ~30) |
| F3 | `TestReadCountsStayBounded` (conformance root) | setup found one run instead of a run and a retry | 2026-10-24, 2026-10-27 |
| F4 | `TestSelectorTable/run/job_name_shared_by_projects` (06-selectors) | `run --job-name` missed the other project's job | race 2026-10-22 |
| F5 | `TestSelectorTable` (copy, run, retry rows) | a nonempty queue, or `late` missing | 2026-10-21, 2026-10-22 (twice) |
| F6 | `TestSelectorTable/copy/job_ID` | `queue is not empty` on copy | race 2026-10-20 |
| F7 | `TestPositionalArguments` (06-selectors) | `wait/finished_run_name` missed `second`; `lineage/latest_run` chose `first` | race 2026-10-20 |
| F8 | `TestRefusalAndSourceHintsNameOnlyANonImplicitBaseDir/implicit` | no `Include them with:` notice | 2026-10-20 |
| F9 | `TestKilledOwnerStopsLaunches` (internal/launcher) | the process group survived after 0.03 s | 2026-10-21 |
| F10 | `TestCLIFlagPairWaitJSONAndEarlyFailure`, `TestJobNameLookupReportsSnapshotErrors/absent_name` | a 1 s wait read 1.013 s; `server.pid` changed | 2026-10-08 |
| F11 | `TestCLIFlagPairEdits/change/all+clear-retry` | a 5 s per-invocation timeout | 2026-10-07 |

### What they have in common

These are hypotheses for Phase 1 to test, not conclusions.

- **Run order within a second (F2, F7, possibly F3 and F5).** These
  fixtures start two runs in quick succession, then read "the latest run" or
  "the second run". Until `8c122ccd` (2026-11-12), the latest run without a
  recorded last run was chosen by directory mtime, and the Web index by name.
  Two runs begun in the same second could swap. `lineage/latest_run`
  choosing `first` fits this. Phase 2 of the shared-rule plan may already
  have fixed some of these items.
- **Fixtures that accept any failure (F2, F3, F8).** These fixtures take any
  nonzero exit of `run` or `retry` as the job failure they expect. A refused
  start reads the same. One such refusal (`already has a run starting or
  running`, COORD-7) was fixed on 2026-10-28. Since then
  `support.Env.RotariRun` fails with the command's output when no run
  directory appears, but other paths still lose the reason.
- **Shared fixtures in parallel tables (F4 to F7).** The selector tables
  build one fixture state and run their rows in parallel. A row that changes
  the queue (`copy`, `run`) can be seen by a row that expects it empty.
- **Load.** Every recurrence was in a full or race suite, several at load
  averages of 30 to 50, and never in isolation. F9 to F11 compare times
  against fixed bounds of 0.03 s, 1 s, and 5 s.
- **Evidence.** No item kept the failing run's state directory or the
  commands' full output, so no cause could be confirmed after the fact.

### What is not covered

- Browser tests run only in CI (`ROTARI_REQUIRE_BROWSER_TESTS`). CI logs
  need signing in, so failures there are reported to Claude only by
  conclusion; their causes need the user's logs.
- The PID-namespace conformance test, added on 2026-11-12, skips where
  `unshare` is not permitted, which probably includes GitHub's Ubuntu
  runners. That is a coverage gap, not flakiness. It is recorded here so
  that Phase 1 checks it once.

## Decisions

- **D1. How to reproduce.** Recommended: a repeat runner that runs one named
  test N times, in parallel copies, under artificial CPU load near the load
  that the recurrences saw. It records each failure's output and keeps its
  temporary state. The alternative is repeating the full suite, which is
  slower and does not say which test is at fault.
- **D2. What "fixed" means.** Recommended:
  - The cause is shown by a reproduction that fails before the change and
    passes after it.
  - Fixtures wait on observable state, such as a file, an exit, or a log
    line, never on a sleep or a wider bound.
  - When the product is wrong, as COORD-7 was, the product is fixed, with a
    contract test.
  - Timeouts and assertions are not loosened without evidence, as each
    ISSUES item already requires.
- **D3. Items that do not reproduce.** Recommended: run them N times under
  the D1 load. If none fails, close the item with that evidence, and keep
  the evidence capture of D4 in place, so that a recurrence is diagnosable.
  The alternative is to keep every item open until a recurrence.
- **D4. Permanent evidence capture.** Recommended: when a conformance test
  fails, `support.Env` keeps that test's state directory and the full
  output of its rotari commands, and names the place in the failure. This
  costs nothing when tests pass. Optionally, later and separately, a
  scheduled CI job could run the repeat runner. That job is not part of this
  plan.

## Phases

1. **Measure.**
   - Build the D1 repeat runner, under `scripts/` if D1 settles on a lasting
     tool, otherwise in the work log.
   - Run each of F1 to F11 under load, at `main` (after the shared-rule
     plan), and record each item's failure rate and output.
   - Check which items `8c122ccd` already fixed, by also running F2, F3, F5,
     and F7 at `17cb01ea`, its parent.
   - Check whether CI runs the PID-namespace test.
2. **Evidence capture (D4).** On failure, `support.Env` keeps the state and
   the command output, and fixtures that accept any failure check the reason
   (F2, F3, F8). This changes tests only. It does not loosen any assertion.
3. **Fix the reproduced items, one cause at a time.** Expected groups:
   - shared parallel fixtures in the selector tables (F4 to F7);
   - fixtures that read the latest run (F2, F3);
   - timing bounds (F9 to F11);
   - the SSH status read (F1).
   A cause that turns out to be in the product goes through the
   behavior-change workflow, with its contract.
4. **Close.** Move each item to Resolved with its cause, or close it with
   its non-reproduction evidence (D3). Record anything new in ISSUES.md.

## Tests and validation

- For each fixed item: its reproduction under the repeat runner fails
  before the change and passes after it, at the same N and load.
- `go test ./conformance`, then the `scripts/check.sh` steps. Finally, the
  full suite repeated several times under load, with no failures.

## Non-goals

- Making the suite faster, unless speed is the cause of a failure.
- Browser tests whose failures are seen only in CI, unless the user supplies
  logs.
- Loosening timeouts or assertions to hide failures.
