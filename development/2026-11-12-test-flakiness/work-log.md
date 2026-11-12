# Work log: test-suite flakiness

## Plan and decisions

Commit: `65b28ecb` (2026-11-12, plan only).

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

## Phase 1: measure

Commit: recorded with this entry (2026-11-12).

- **Change:** Added [scripts/flake-repeat.sh](../../scripts/flake-repeat.sh),
  the D1 repeat runner, and measured each item with it. Amended D1 with the
  user: the machine is shared (56 CPUs, nine users), so the test, its child
  processes, and the busy loops are confined to CPUs 0-3 at nice 19.
- **Reason:** Phase 1 of the plan.
- **Results** (`--cpus 0-3 --load 2`, runs x count per item; `main` is
  `65b28ecb`, "old" is `17cb01ea`, before the run order was unified):

  | Item | Tree | Mode | Failures |
  | --- | --- | --- | --- |
  | F1 `TestSSHJobReadsAsRunning` | main | race | **30 / 30** |
  | F2 `TestFilteredRerunCarriesCompletedResults` | main, old | normal | 0 / 50 each |
  | F3 `TestReadCountsStayBounded` | main, old | normal | 0 / 30 each |
  | F5 `TestSelectorTable` (covers F4, F6) | main | normal, race | 0 / 20 each |
  | F5 `TestSelectorTable` | old | normal | 0 / 20 |
  | F7 `TestPositionalArguments` | main, old | race | 0 / 20 each |
  | F8 `TestRefusalAndSourceHints...` | main | normal | 0 / 50 |
  | F9 `TestKilledOwnerStopsLaunches` | main | normal | **5 / 500** (and 1 / 20 in a trial) |
  | F10 `TestCLIFlagPairWaitJSONAndEarlyFailure` | main | normal | 0 / 30 |
  | F10 `TestJobNameLookupReportsSnapshotErrors` | main | normal | 0 / 50 |
  | F11 `TestCLIFlagPairEdits` | main | normal | 0 / 4 |

- **F1 cause (product):** the remote SSH wrapper's `cleanup_old_state`
  runs before the job starts and forks several commands for every token
  directory under `/tmp/rotari-UID`, including those its 30-day retention
  keeps. With the 891 directories a month of test runs left here, the loop
  alone took 5.4 s on idle CPUs and 45 s on the contended CPUs, so the
  remote process was recorded after the fixture's 15 s deadline. Shown by a
  debug copy of the test that kept its state: the remote `process` file was
  written at 13:05:30.355, 15 s after dispatch, just before the cleanup
  `cancel`. Without contention the test passed, under `taskset` too. The
  directories were left in place as evidence. Recorded in ISSUES.md.
- **F9 suspected cause (fixture):** the fixture waits for the `sleep`
  child to be gone and then checks the process group once, while the
  wrapper may not yet have reaped the group's `sh` leader. To be confirmed
  in Phase 3 by recording the surviving member.
- **Not reproduced:** F2 to F8, F10, F11, at main or at the old tree, so
  this measurement cannot say whether `8c122ccd` fixed any of them. The
  runner ran one test at a time; the recurrences were in full suites, where
  other tests of the same package and other packages ran beside them.
- **CI and the PID-namespace test:** not checked; CI logs need signing in.
  Locally, `unshare --user --pid` works, so the test runs here.
- **Environment notes:** in this tool environment `BASH_ENV` sources
  `.bashrc`, which puts an older Go first on `PATH` in child shells; run the
  script with `BASH_ENV` unset. The first version's final `wait` also
  waited for the busy loops; fixed before the measurements.
- **Plan impact:** F1 moves from "fixture" to "product" (Phase 3, through
  the behavior-change workflow). Before closing the non-reproduced items
  under D3, a second round should run whole packages, not single tests,
  under the same load.
- **Validation:** the measurements above. Raw output was kept in the
  session scratchpad, not in the repository.
- **Remaining:** whole-package round; Phases 2 to 4.
