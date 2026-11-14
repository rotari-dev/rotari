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
