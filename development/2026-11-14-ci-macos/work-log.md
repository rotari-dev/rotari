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
