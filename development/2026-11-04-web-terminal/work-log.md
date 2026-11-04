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

## Check the remaining stored values

**Commits:** 2026-11-04 16:35:49 `c0985fc6`.

**Change:** `TestStoredValuesStayText` now stores basedir paths, config files, diagnoses, executor options, and launcher names, targets, and output, and opens the recovery dialog. Each page lists text it must show. WEB-28 moved from partial to conformance, and `plan.md` lists the gaps that remain.

**Reason:** These were the gaps the previous entry left open, at the user's request.

**Plan impact:** Phase 1's value coverage is complete for what the CLI can record. Phase 2 is next.

**Validation:** The test passes in all four contexts, passed two more runs and once under `-race`, and failed with `esc` weakened. After rebasing onto `main`, `go test ./conformance ./conformance/05-web` passed. No real browser was used. Full `go test ./...` was not rerun, since only a test and documents changed.

**Remaining:** Recovery dialog reasons rest on review; a service launch's progress view is not rendered; phase 2.

## Fix a launcher test's race

**Commits:** 2026-11-04 18:32:38 `a7d038c1`.

**Change:** `TestKilledOwnerStopsLaunches` in `internal/launcher/process_test.go` reads the launch's process group before killing the fake Web process.

**Reason:** The longer CI run exposed it: the command writes its PID before the fake process prints the group, so on a loaded host the test read nothing.

**Plan impact:** None; it unblocked CI for the browser check.

**Validation:** With the print delayed by two seconds, the old test failed as in CI and the new one passed. It passed 50 repetitions and 20 under `-race` on one CPU, and the package passed. Go CI passed on the branch.

**Remaining:** The same test's surviving-group failure in `ISSUES.md` is a different symptom and stays open.

## Check stored values in a real browser

**Commits:** 2026-11-04 18:32:38 `9a72dad9`.

**Change:** Added `TestStoredValuesStayTextInBrowser`, `ROTARI_REQUIRE_BROWSER_TESTS` in `ci.yml`, and a `beforeNavigate` option and a longer start limit in `scripts/web-browser.mjs`. Both stored-value checks wait for pages and requests instead of fixed times.

**Reason:** Phase 2 needs a check that enforces a policy, which jsdom cannot; the user chose CI, with a skip where Chrome is absent.

**Plan impact:** Phase 2's check is in place, and nothing runs against it until a policy exists.

**Validation:** Go CI, including `go` with the browser required and `race`, passed on the branch at `35b8c735`, before these commits were squashed and rebased. Earlier runs failed on the contract table, on Chrome's ten-second start limit, and on fixed waits, each fixed here. After rebasing, `go vet ./...` and `go test ./internal/launcher ./internal/webui ./conformance ./conformance/05-web` passed, with the browser test skipped locally because Chrome is absent. Fetching Chrome locally was denied by a permission check. CI logs were not read, so the browser test's run is inferred from the required setting and the job's success. No weakened code was run against the browser test.

**Remaining:** Phase 2. The full CI time rose from about 400 to as much as 657 seconds in one run.
