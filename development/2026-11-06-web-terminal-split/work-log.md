# Work log: Split the Web terminal panel

## Step 1: one pane object

**Commits:** 2026-11-06 10:26:52 `80595dfd`.

**Change:** In `internal/webui/assets/web_shell.js`, `newPane` holds the terminal, fit addon, viewport, connection, shown shell, waiting command, connect state, and retry state, including `whenVisible`. `connect`, `open`, `show`, `newShell`, `shellGone`, `reconnect`, `send`, `settleSend`, `pasteProblem`, and `deliverSend` take a pane, and `openTerminal` gives a pane its screen. `window.sendToTerminal` keeps its signature and checks the text, then calls `sendToPane`. No test, contract, or user document changed.

**Reason:** [Step 1](plan.md#step-1-one-pane-object-no-change-in-behavior): the panel assumed one of everything, and two panes need two of each.

**Plan impact:** Step 1 is done, and its diff waits to be looked at before step 2. The one pane is named `focused` and exists from page load, because a command can wait in it before the panel is built. Code acting for the whole panel (`begin`, the bar's buttons, the tabs, the visibility and refresh checks, and the theme) reads `focused` rather than taking a pane; steps 2 and 3 decide what each does with two. The status and action are still the bar's, as step 2 is to decide. The [decisions](plan.md#decisions) are still proposals.

**Validation:** `go test ./...` passed, with no file of tests changed. Run verbosely, `TestTerminalPanel`, `TestTerminalThemeTokens`, and the `Terminal` tests of `conformance/05-web` ran and passed, except that `TestTerminalPanelInBrowser` was skipped for lack of Chrome, so the real xterm.js has not run this. `tsc` found nothing, though `internal/webui/tsconfig.json` does not include `web_shell.js`; `node --check` parsed it. `go vet` and `-race` were not run, since only this JavaScript changed. `scripts/check.sh` and pre-commit could not be run here (`npm` and `pre-commit` are broken or missing), so their parts were run by hand as above. A scan of the panel's code found no use of the moved state other than local names taken from a pane. Nothing was tried by hand in a browser.

**Remaining:** Looking at the diff, then step 2. The plan's note that `TestTerminalPanelInBrowser` fails on CI at its Maximize check could not be confirmed here: `gh` is not installed and the Web terminal plan's work log does not record it. It should be understood before step 3 changes that test.
