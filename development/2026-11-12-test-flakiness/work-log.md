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

Commit: `7180a200` (2026-11-12).

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

## Phase 3, first group: F1, the SSH start delay

Commit: `6626af04` (2026-11-12).

- **Change:** taken ahead of Phase 2 with the user, because the cause is in
  the product.
  - [internal/executor/ssh.go](../../internal/executor/ssh.go): the remote
    wrapper calls `cleanup_old_state` in the background after the job has
    started and recorded its process, and waits for it before exiting.
    `find -mindepth 1 -maxdepth 1 -type d -mmin +43200` selects the expired
    token directories, so retained ones are not examined; `id -u` and the
    process-group table are read once. In a subshell, the cleanup can no
    longer overwrite the wrapper's `remote_pid`. The pruning rule (30 days,
    and only when the recorded group is absent) is unchanged.
  - `TestSSHWrapperStartsTheJobBeforePruning` (unit): a `stat` on `PATH`
    logs each token directory examined and whether the job's `process`
    record existed then.
  - `TestSSHJobReadsAsRunning` (conformance): the loopback sshd sets
    `XDG_RUNTIME_DIR` to the test's directory (`SetEnv`), so tests no longer
    leave token directories in the user's `/tmp/rotari-UID`; the test seeds
    5000 retained directories and checks they are kept. Its `show` loop now
    waits for `running (recorded)` instead of taking the first status after
    `pending`.
  - SAFE-16 note, [ARCHITECTURE.md](../../docs/ARCHITECTURE.md), and
    [OPERATIONS.md](../../docs/OPERATIONS.md) say pruning follows the start
    and examines only expired state. No CLI, Web, or Python client change.
- **Reason:** Phase 1 found the delay. Fixing the delay alone left 12 of 30
  failures under load, all `show --json job = unknown`: a dispatched attempt
  reads `unknown` until `Submit` records the running phase, as
  `DisplayStatus` defines, and the fixture kept that first snapshot. That is
  the symptom of all three recorded recurrences.
- **Plan impact:** F1 resolved, with both causes shown. `scripts/flake-repeat.sh`
  gained `--skip`, for the whole-package round.
- **Validation:**
  - Repeat runner, CPUs 0-3, two busy loops per CPU, `-race`, 10 x 3:
    30 of 30 failed at `7180a200`; 18 of 30 passed with the product fix
    alone; 30 of 30 passed with both.
  - At `7180a200` in a temporary workspace: the unit test fails (both
    directories examined, before the record); the conformance test fails
    with `info ... State:unknown`. Both pass after the fix; the conformance
    test takes 1.8 s instead of over 20 s.
  - The user's `/tmp/rotari-UID` held 893 entries before and after a run of
    the SSH conformance tests.
  - `go vet`, the Web UI type check, `go test ./...`, `go test -race ./...`:
    all passed (the race step included the conformance contract table).
- **Remaining:** whole-package round (running); Phase 2; F9; Phase 4.

## Phase 2: evidence capture, and F9

Commit: `0ab50c41` (2026-11-12).

- **Change:**
  - [conformance/support/evidence.go](../../conformance/support/evidence.go):
    `Transcript` records each rotari command's start, directory, exit code,
    elapsed time, and full output; `KeepOnFailure` moves a failed test's
    state directory to `$TMPDIR/rotari-conformance-failed/TEST-NNNN/state`,
    writes `commands.log` beside it, and logs the place. It is registered
    before the cleanups that stop servers and processes, so it runs after
    them. A passing test keeps nothing.
  - All three conformance harnesses use it: `support.NewEnv`, `newEnv` in
    [conformance/harness_test.go](../../conformance/harness_test.go), and the
    selectors' own `newEnv` in
    [conformance/06-selectors/harness_test.go](../../conformance/06-selectors/harness_test.go).
    The first full run showed the selectors harness had none, when a
    positional row failed without evidence.
  - F8's fixture fails with the `retry` output when its notice is missing.
    F2 already used `RotariRun` and F3 already printed both commands; the
    other `run` and `retry` calls that check only an exit code are refusal
    tests that also check stderr, and now leave a transcript.
  - `scripts/flake-repeat.sh` keeps a failing process's `TMPDIR` (tmp-K).
  - [conformance/README.md](../../conformance/README.md) says where the
    evidence goes.
  - F9: the fixture waits for the process group to be gone, as it already
    waited for the child.
- **Reason:** Phase 2 of the plan (D4), and F9 from Phase 1.
- **F9 cause shown:** a debug copy listed the group's members on failure:
  none were left in `/proc`, and the group was gone 3 microseconds later, in
  both failures of 500. The group's `sh` leader ends with `sleep` and is
  reaped just after it.
- **New observation:** the full suite failed once in
  `TestPositionalArguments/unlock/complete_run_ID` (`repaired finalization`
  instead of `recovered queue`). Each selector row builds its own fixture,
  so the "shared parallel fixtures" hypothesis does not hold for
  06-selectors. Killing the client before the supervisor did not reproduce
  it. A second experiment, killing the job's process group first, was
  stopped by the tool's permission check: it matched `sleep 300` across the
  machine instead of the test's own processes. Recorded in ISSUES.md under
  the positional item.
- **Validation:**
  - `TestKeepEvidenceMovesStateAndWritesTranscript`; a deliberately failing
    test kept `state/` and a `commands.log` with both commands and their
    output, and named the place.
  - F9 under the repeat runner (CPUs 0-3, two busy loops per CPU): 7 of
    1000 failed before (5 of 500 in Phase 1, 2 of 500 in the debug copy),
    0 of 1000 after.
  - `go vet ./...`, `go test ./...` (one failure, above), and
    `go test -race ./...` (passed) before the selectors harness and F9
    edits; after them, `go vet ./...` and `go test` of the changed packages
    (`internal/launcher`, `conformance`, `conformance/support`,
    `conformance/06-selectors`, `conformance/03-interfaces`) and `-race` of
    the first four passed.
- **Remaining:** whole-package round (running); the positional `unlock`
  row; Phase 4.

## The positional `unlock` row

Commit: `bf6d94a2` (2026-11-12).

- **Change:** `setupInterrupted` in
  [conformance/06-selectors/positional_test.go](../../conformance/06-selectors/positional_test.go)
  kills the run's supervisor first, then the job. `support.KillSupervisors`
  returns only once the supervisors are gone (its other caller, in
  04-coordination, still passes).
- **Reason:** the user chose to run the ordering experiment scoped to the
  test's own processes. Killing only the job group recorded in the test's
  own `job.json` (PID checked against its recorded start time), then the
  rest 0 or 500 ms later, let the supervisor record the job's result and
  finish the run both times (`state=empty`). The recurrence was the narrow
  case in between: the supervisor recorded the result and died during its
  final steps, leaving an interrupted run with a complete summary, which
  `unlock` correctly finalizes. The earlier experiment, killing the client
  first, left the summary incomplete. The product behaves as specified.
- **Plan impact:** this recurrence is explained and fixed; the positional
  item's older symptoms stay open.
- **Validation:** the `unlock` rows under the repeat runner, 100 of 100
  passed (as before the change, so the evidence is the ordered experiment);
  `go vet ./conformance/...`; `TestPositionalArguments` and 04-coordination,
  and `-race` of 06-selectors, 04-coordination, and conformance/support,
  passed.
- **Remaining:** whole-package round (running); Phase 4.

## Phase 4: close

Commit: recorded with this entry (2026-11-12).

- **Whole-package round** (CPUs 0-3, two busy loops per CPU, nice 19, at
  `7180a200` in a separate workspace; counts are top-level tests):

  | Package | Runs | Failures |
  | --- | --- | --- |
  | 06-selectors | 6 | 0 / 54 |
  | 06-selectors, `-race` | 4 | 0 / 36 |
  | 02-lifecycle (F1 skipped) | 6 | 0 / 300 |
  | conformance root | 6 | 0 / 24 |
  | 03-interfaces | 6 | 0 / 516 |
  | 04-coordination | 6 | 0 / 240 |
  | 03-interfaces/pairruns | 4 | 0 / 48 |
  | 03-interfaces/pairedits | 2 | 0 / 20 |

- **Outcome per item:**

  | Item | Outcome |
  | --- | --- |
  | F1 | fixed, two causes shown: the SSH start delay (product) and the `show` snapshot (fixture), `6626af04` |
  | F9 | fixed, cause shown: a single check of a group still being reaped (fixture), `0ab50c41` |
  | F2, F3, F4, F5, F6, F8, F10, F11 | closed without reproduction (D3), with the counts in ISSUES.md; F11's evidence is thin (6 runs) |
  | F7 | left open: its 2026-11-12 recurrence was explained and fixed (`bf6d94a2`), but its older symptoms (a missing `second`, `lineage/latest_run` choosing `first`) were not reproduced, and whether `8c122ccd` fixed them is not shown |

- **Findings against the plan's hypotheses:**
  - Run order within a second: not reproduced at `17cb01ea` either, so this
    measurement says nothing about `8c122ccd`.
  - Shared fixtures in parallel tables: false for 06-selectors, where each
    row builds its own fixture.
  - Load: confirmed for F1 and F9, where it widened a real window.
  - Missing evidence: fixed by Phase 2 in all three conformance harnesses.
- **Out of scope, still open in ISSUES.md:** about fifteen other
  intermittent test items not among F1 to F11, among them the
  `pairjobcontrol` timeouts under load, the unlock flag-pair and
  `unlock_test.go` items, `TestUnlockDerivesInterruptedRun/lock`, the
  runtime-directory sweep, the `wait` disconnect items, the elapsed-time
  assertion, the jj revision read, and the terminal test that failed in CI.
  They are candidates for a further plan with the same tools.
- **Validation:** documentation only for this entry; the code changes were
  validated in the entries above.
- **Remaining:** none in this plan. Nothing is pushed.
