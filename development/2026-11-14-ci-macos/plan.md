# Plan: Run the Go CI on macOS too

Created: 2026-11-14
Status: In progress; see [work-log.md](work-log.md). Decided with the user on 2026-11-14.

## Goal

rotari supports Linux and macOS (`docs/FAQ.md`), and the release ships darwin binaries, but no test has ever run on
macOS. The Go CI runs its tests on both. A test that fails on macOS is a defect to fix, never a reason to leave
macOS out:

- a product bug is fixed in the product;
- a test that assumes Linux without need is fixed in the test;
- a test of something that exists only on Linux (PID namespaces, strace) skips on other systems with that reason
  in its skip message, and the contract note, if any, says the behavior is Linux-only.

## Existing implementation

- `.github/workflows/ci.yml` has two jobs on `ubuntu-latest`: `go` (vet, all tests with coverage, build) and `race`.
  Both install `zsh fish strace` with `apt-get`. `go` sets `ROTARI_REQUIRE_BROWSER_TESTS=1`, which makes a missing
  Chrome a failure, and uploads coverage to Codecov.
- `release.yml` cross-builds darwin amd64 and arm64 binaries, only at release time.
- Code that runs only off Linux, never executed so far:
  - `internal/terminal/process_darwin.go` (`ps` parsing for `web --shutdown`) and `process_other.go`;
  - `cmd/rotari/web_redirect_other.go` (`Dup2` on Ctrl-D detach) and `terminal_bsd.go`;
  - `internal/launcher/listener_darwin.go` and `listener_other.go`;
  - `internal/fileops/rename_other.go` and `internal/bugreport/fstype_other.go`.
- Features that degrade without `/proc`, by design but unverified:
  - `state.ProcessStart` returns empty, so PID reuse is not detected for stale locks;
  - `projectrun.readLoadAverage` returns nil;
  - `state.PIDNamespace` returns empty, which is correct on macOS.
- Tests that read `/proc` directly: `conformance/harness_test.go`, `conformance/06-selectors/harness_test.go`,
  `04-coordination/pid_namespace_test.go`, `05-web/web_detach_test.go`, `05-web/web_client_test.go`,
  `internal/artifact/classify_test.go`, and `scripts/web_browser_test.go`. Only `conformance/read_count_test.go`
  already skips off Linux (strace).

## Proposals

1. **A matrix on the `go` job:** `os: [ubuntu-latest, macos-latest]`, `fail-fast: false`.
   - Dependencies per OS: `apt-get install zsh fish strace` on Linux; `brew install fish` on macOS, where zsh is
     present and strace does not exist.
   - Codecov upload from Linux only.
   - Browser tests: macOS runners include Chrome; check that `browserForTests` finds it (it looks on `PATH`, where
     the macOS app bundle is not), and set `ROTARI_TEST_CHROME` in the workflow if needed, so
     `ROTARI_REQUIRE_BROWSER_TESTS` stays on for both.
2. **No separate macOS `race` job; the macOS leg runs with `-race` once triaged** (decided 2026-11-14). Code that
   runs only on macOS is never run by the Linux `race` job, and scheduling differs, so races there need macOS. The
   macOS leg collects no coverage, so its one test run can carry `-race` instead of adding a job. It switches only
   after the plain run passes, so a failure caused by macOS is not confused with one caused by `-race` timing.
3. **Triage before requiring it.** The macOS leg starts with `continue-on-error: true`. Each failure is classified
   as in the Goal and fixed in its own commit. When the leg passes, `continue-on-error` is removed and it becomes a
   required check like the Linux leg.
4. **Not in this plan:** making `ProcessStart` work on macOS (`sysctl kern.proc.pid`) and the load average
   (`sysctl vm.loadavg`). Each is a behavior change with its own plan; they are recorded in `development/ISSUES.md`
   when this plan starts.

## Steps

1. Workflow: the matrix, per-OS dependencies, Codecov and browser settings, macOS leg non-blocking. Push and read
   the first macOS result.
2. Triage: one commit per cause, each saying in its work-log entry whether it was a product bug, a Linux assumption
   in a test, or a Linux-only feature now skipped. Product bugs found here are also checked for other interfaces as
   AGENTS.md asks.
3. Switch the macOS leg's test run to `-race` and fix what it finds.
4. Make the macOS leg required; update `docs/FAQ.md` or contributor docs if they describe what CI covers.
5. Work log.

## Risks

- Cost: macOS runner minutes are billed at about ten times Linux on private repositories (free on public ones).
  The leg runs the same suite once per push.
- Timing: macOS runners are slower and their process and filesystem timing differs, so tests with tight waits may
  flake there. Such flakes are fixed by waiting on conditions, not by longer sleeps or skips.
- The triage may find more than expected, since the code above has never run. Step 2 can span several commits;
  the leg stays non-blocking until it is done.
