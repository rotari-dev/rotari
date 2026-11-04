# Work History: Web UI Hardening and a Built-in Terminal

See [plan.md](plan.md) for current scope and status.

## Plan hardening before a terminal

**Commits:** 2026-11-04 14:29:42 `3fa4157e`.

**Change:** Added `plan.md`.

**Reason:** The user asked how hard a terminal in `rotari web` would be, then for a plan to reach it after deciding that script-injection hardening was worth doing regardless.

**Plan impact:** Hardening comes first and ships alone; a gate decides whether the terminal is built. Decided a separate `--allow-terminal`, a token typed on each opening, and no reattach, with the last two open for the user's confirmation.

**Validation:** Documentation only; no tests run.

**Remaining:** Phases 1 to 5.

## Keep job identity as data in action handlers

**Commits:** 2026-11-04 14:47:37 `5b74db70`.

**Change:** `showArtifacts` and `selectJobAttempt` handlers in `web_app_core.js` now use `jsArg`. Added `TestWebJobActionHandlersKeepIdentityAsData`, and updated `plan.md` and `ISSUES.md`.

**Reason:** Checking the plan's assumptions showed that `esc` does not protect a JavaScript argument: a browser decodes an attribute before parsing it, so a quote in a project name ran script on click.

**Plan impact:** Phase 1 started with a real defect.

**Validation:** The test failed before the fix with the planted statement running, and passed after. The built binary no longer ran it in jsdom. `go vet`, `tsc`, and `go test ./...` passed. `go test -race ./...` failed once in `TestUnlockDerivesInterruptedRun/lock`, then passed ten repetitions and a package rerun; recorded in `ISSUES.md`. `scripts/check.sh` did not run here, because `BASH_ENV` restores another Go and `npm` is broken, so its steps ran directly. `pre-commit` is not installed.

**Remaining:** The rest of the audit; the `unlock` test's cause is unconfirmed.

## Insert template values in one pass

**Commits:** 2026-11-04 15:36:44 `7544154d`.

**Change:** `composeStaticBootstrap`, `composeWebHTMLWithSession`, and `composeInfoHTML` in `internal/webui/assets.go` use `strings.NewReplacer`. Added `TestComposeKeepsPlaceholderTextInValues` and `TestPagesKeepPlaceholderTextInValues`, contract WEB-26, and the audit's first pass in `plan.md`.

**Reason:** Sequential replacement searched values already inserted. A job command holding a placeholder's text broke a static export's script, and a project named after one moved the Recent jobs table into the sidebar.

**Plan impact:** Added a defect class to phase 1 that the `innerHTML` review alone would have missed.

**Validation:** Both tests failed before the fix and passed after, and the original reproductions were fixed. In the commit's own tree, `go vet` on the changed packages and `go test ./internal/webui ./conformance/05-web ./conformance` passed. Full `go test` and `-race` passed only on a tree that may have held another thread's edits. Running script through this defect was not demonstrated.

**Remaining:** `bug-report` path redaction, found during the check, is in `ISSUES.md`.

## Check that stored values stay text

**Commits:** 2026-11-04 16:22:25 `64647bc4`.

**Change:** Added `conformance/05-web/hostile_values_test.go` (`TestStoredValuesStayText`), contract WEB-28, and the phase 1 results in `plan.md`.

**Reason:** Phase 1's remaining work: a check of rendered pages and controls, which also reaches values a source scan could miss.

**Plan impact:** Phase 1 is done, with an area table instead of the planned per-sink table, since the test now checks the pages directly. The contract is marked partial for the listed gaps. Phase 2 is next.

**Validation:** The test passes on the current code and failed when `esc`, `jsArg`, or the Markdown escape was weakened, or when the original `showArtifacts` bug was restored; a first version with one combined value missed the `jsArg` case. It passed three more runs and once under `-race`. After rebasing onto `main`, `go test ./conformance ./conformance/05-web` passed. No real browser was used.

**Remaining:** Gaps listed in [the phase 1 results](plan.md#phase-1-results); phase 2.
