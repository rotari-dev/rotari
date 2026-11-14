# Work log: Run the Go CI on macOS too

## 2026-11-14 — `d209f270` Run the Go CI on macOS too, without failing the workflow yet

**Change:** `.github/workflows/ci.yml`: the `go` job is a matrix over `ubuntu-latest` and `macos-latest` with
`fail-fast: false`. The macOS leg has `continue-on-error` while it is triaged. Per OS: `apt-get install zsh fish
strace` on Linux, `brew install fish` on macOS, and `ROTARI_TEST_CHROME` set to the macOS Chrome app bundle so
`ROTARI_REQUIRE_BROWSER_TESTS` still applies there. Codecov upload runs on Linux only. `race` is unchanged.

**Reason:** The plan's step 1: see what fails on macOS, where no test has run before.

**Plan impact:** Step 1 done. Step 2 (triage) starts from the first macOS result.

**Validation:** The workflow parses as YAML with the expected matrix and steps. Its effect can only be seen in CI.

**Remaining:** Steps 2 to 4.

## 2026-11-14 — `2b03dce2` Open the run terminal test's pty with creack/pty, which also works on macOS

**Change:** `conformance/03-interfaces/run_terminal_test.go`: `openTerminal` calls `pty.Open` (creack/pty, already a
dependency used by `05-web/web_client_test.go`) instead of Linux's `/dev/ptmx` ioctls `TIOCSPTLCK` and `TIOCGPTN`.

**Reason:** The first macOS run (Go CI run 43573390077) failed at `go vet`: `undefined: syscall.TIOCSPTLCK`. A Linux
assumption in a test, not a product bug.

**Plan impact:** First triage fix (step 2). The plan also now says the macOS leg moves to `-race` after triage
instead of a separate macOS `race` job, as decided with the user.

**Validation:** `GOOS=darwin go vet ./...` for arm64 and amd64 is clean, so no other package fails to build for
macOS. `TestRunTerminal*` passed `-count=3` on Linux.

**Remaining:** The macOS test results, which the next run shows.

## 2026-11-14 — `decaddda` and `f65c7601`: the first macOS test failures

The second macOS run reached the tests. The user pasted the failures of `conformance/01-resolution`,
`02-lifecycle`, `03-interfaces`, and `cmd/rotari`; other packages' results were not seen.

**`decaddda` End zsh completions in the shell test with Ctrl-G** (`cmd/rotari/completion_shell_test.go`):
`TestShellCompletionCandidates/zsh` timed out after 120 s. The driver ended each completion with Ctrl-T, which BSD
and macOS terminals take as their status key (the log shows its `load: … cmd: zsh` line), so zsh never saw it.
Ctrl-G has no terminal meaning on either system. A Linux assumption in a test.

**`f65c7601` Read the process table and set PWD the same way on macOS** (`conformance/support`, the three
harnesses, `05-web`, `scripts/web_browser_test.go`):

- New `support.Processes` (and `MustProcesses`, `ProcessArgs`, `ProcessEnvironment`): `/proc` on Linux, `ps -ww`
  (and `ps -E`) elsewhere, where arguments are split at spaces. About ten `/proc` readers now use it: `KillStrays`,
  `KillSupervisors`, `JobProcesses`, the harnesses' `killStrays` (now `support.KillStrays`) and `jobProcesses`,
  `stopDetached`, the server command-line check, `childrenOf`, and the browser-close check (`kill -0`).
  `sessionOf` uses `unix.Getsid`. A process table that cannot be read now fails the test; `KillStrays` used to
  return silently, which left `TestRerunOfAnInterruptedRun`'s run alive on macOS (`state running, want
  interrupted`).
- `support.ProcessIdentity` returns no PID namespace or start time off Linux, as rotari records none there. It
  failed `TestInfoReportsContextAndRunLocks`, `TestUnlockExecutionEvidenceOverridesTerminalOutcome`,
  `TestUnlockFinalizationRepair`, and `TestUnlockOwnershipAndDryRun` with `readlink /proc/…/ns/pid`.
- `support.CommandEnv` sets `PWD` to the command's directory in all three harnesses. They passed an explicit
  environment without it, so `os.Getwd` in rotari returned `/private/var/…` for a test directory under `/var/…`,
  which a shell's `PWD` avoids. It failed `TestBadConfiguredValuesAreErrors` and
  `TestRunRecordsUncommittedGitChanges`, and probably `TestWebWorkspaceSourceSelectionAndRunSnapshot`, whose
  message was not seen.

All Linux assumptions in tests, not product bugs.

**Not fixed: `TestNotesAnotherRotariBinary`.** Its message was not in the paste. From the code: on macOS,
`ProcessStart` and `PIDNamespace` are empty, so a live coordinator is never `CoordinatorLive` and `wait` prints no
note. A product limitation, recorded in `development/ISSUES.md`; the test stays failing rather than skipped until
it is fixed in its own plan.

**Validation:** `GOOS=darwin go vet ./...` (arm64, amd64) clean. On Linux: `go test ./conformance/... ./scripts
./cmd/rotari` passed (node v24 for JSDOM; `05-web` failed once under the system node v12 and passed with v24).
None of it has run on macOS yet.

**Remaining:** Push and read the next macOS run, including the packages not seen so far.

## 2026-11-14 — `bc442ca7` Make the chdir, SIGSTOP, and pty unit tests hold on macOS

The third macOS run (Go CI run 43575930113) no longer showed the failures fixed above, apart from those the
product causes. Its `cmd/rotari` unit tests showed three more Linux assumptions:

- `TestResolveBaseDirPriority`, `TestWorkspaceLocationAndMergedFiles`, and
  `TestWorkspacePositionalProjectLoadsSelectedLayer` changed directory with `os.Chdir`, which leaves `PWD`, so
  `os.Getwd` returned `/private/var/…`. They, and `internal/state`'s
  `TestResolveBaseDirDefaultIgnoresEnvironmentOverride`, now use `t.Chdir`, which also sets `PWD`.
- `TestControlQueueJobsSuspendsAndResumesSelectedLocalJob`: on BSD and macOS, Go's `WaitStatus.Stopped` is false
  for a stop by SIGSTOP itself (`syscall_bsd.go`). The test reads the stop from the raw status.
- `TestIsTerminalAcceptsPseudoTerminal` asked about a bare `/dev/ptmx` master, which macOS refuses. It opens a
  pair with creack/pty and asks about the terminal side, which is what a process gets as stdin.

The run's other failures are the product's: `unlock` refuses with `coordinator_unknown` and local executions read
`unknown`, because macOS has no PID namespace or process start time for rotari to verify. The ISSUES entry now
lists them; they need their own plan.

**Validation:** `go test ./cmd/rotari ./internal/state` passed on Linux (node v24); `GOOS=darwin go vet` of both
clean.

## 2026-11-14 — `aa4e9fbd`, `ed103582`, `ed0b85a8`: packages first seen in the fourth macOS run

The fourth macOS run (Go CI run 43577862862, at `9452a07e`) showed packages not seen before. The pasted part did
not include the `unlock` and `info` tests that the process identity change addresses, so whether they now pass
is not known from it.

- **`aa4e9fbd` Type into the job control confirmation through creack/pty** (`conformance/06-selectors`):
  `TestJobControlSelectors/terminal_confirmation_*` ran `script -q -e -c …`, util-linux options that BSD `script`
  rejects (`illegal option -- c`). The test now starts rotari on a pseudo-terminal itself. A Linux assumption in a
  test.
- **`ed103582` Say the SSH remote host must run Linux** (`internal/executor/ssh_test.go`, `docs/EXECUTORS.md`):
  seven SSH tests run the remote scripts on the test machine. Those scripts use `setsid`, GNU `stat -c`, and
  `/proc` (`invalid SSH job state owner`, exit 1), so the remote host must be Linux; the contract already says the
  remote records a Linux `/proc` start time, but the user guide did not say it. Those tests now skip off Linux,
  naming why, and EXECUTORS.md states the requirement. A Linux-only feature; supporting a macOS remote would be a
  feature of its own.
- **`ed0b85a8` Record the host load average on macOS** (`internal/projectrun`): `TestMalformedLoadSamplesAreSkipped`
  (STATE-4) found no `load_samples.jsonl`, because the load came only from `/proc/loadavg`, so macOS runs recorded
  none. macOS now reads `sysctl vm.loadavg` (`struct loadavg`: three `fixpt_t` and `fscale`). A product gap, small
  and observational, so fixed here rather than in a plan of its own. Tests: `TestDecodeDarwinLoadAverage` (layout,
  short record, zero scale) and `TestHostLoadAverageIsRead` on Linux and macOS.

**Validation:** `go vet ./...` for linux and darwin (arm64, amd64), and `./internal/projectrun` for freebsd: clean.
On Linux, `internal/executor`, `internal/projectrun`, `conformance/04-coordination`, and `conformance/06-selectors`
passed; the terminal confirmation tests passed `-count=3`. Nothing here ran on macOS.
