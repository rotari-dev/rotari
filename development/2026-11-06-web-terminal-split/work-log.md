# Work log: Split the Web terminal panel

## Step 1: one pane object

**Commits:** 2026-11-06 10:26:52 `80595dfd`.

**Change:** In `internal/webui/assets/web_shell.js`, `newPane` holds the terminal, fit addon, viewport, connection, shown shell, waiting command, connect state, and retry state, including `whenVisible`. `connect`, `open`, `show`, `newShell`, `shellGone`, `reconnect`, `send`, `settleSend`, `pasteProblem`, and `deliverSend` take a pane, and `openTerminal` gives a pane its screen. `window.sendToTerminal` keeps its signature and checks the text, then calls `sendToPane`. No test, contract, or user document changed.

**Reason:** [Step 1](plan.md#step-1-one-pane-object-no-change-in-behavior): the panel assumed one of everything, and two panes need two of each.

**Plan impact:** Step 1 is done, and its diff waits to be looked at before step 2. The one pane is named `focused` and exists from page load, because a command can wait in it before the panel is built. Code acting for the whole panel (`begin`, the bar's buttons, the tabs, the visibility and refresh checks, and the theme) reads `focused` rather than taking a pane; steps 2 and 3 decide what each does with two. The status and action are still the bar's, as step 2 is to decide. The [decisions](plan.md#decisions) are still proposals.

**Validation:** `go test ./...` passed, with no file of tests changed. Run verbosely, `TestTerminalPanel`, `TestTerminalThemeTokens`, and the `Terminal` tests of `conformance/05-web` ran and passed, except that `TestTerminalPanelInBrowser` was skipped for lack of Chrome, so the real xterm.js has not run this. `tsc` found nothing, though `internal/webui/tsconfig.json` does not include `web_shell.js`; `node --check` parsed it. `go vet` and `-race` were not run, since only this JavaScript changed. `scripts/check.sh` and pre-commit could not be run here (`npm` and `pre-commit` are broken or missing), so their parts were run by hand as above. A scan of the panel's code found no use of the moved state other than local names taken from a pane. Nothing was tried by hand in a browser.

**Remaining:** Looking at the diff, then step 2. The plan's note that `TestTerminalPanelInBrowser` fails on CI at its Maximize check could not be confirmed here: `gh` is not installed and the Web terminal plan's work log does not record it. It should be understood before step 3 changes that test.

## Step 2: focus and status per pane

**Commits:** 2026-11-06 10:39:29 `3b9747b0`.

**Change:** In `web_shell.js`, each pane builds its own `.terminal-pane` element with a status and action, `showStatus`, `signInGone`, and `refusal` take a pane, and `focused` follows `focusin`. `web_shell.css` places the status over the screen. No test changed.

**Reason:** [Step 2](plan.md#step-2-focus-and-status-per-pane). The user looked at step 1 and said to go on, and said that the Maximize failure on CI is resolved.

**Plan impact:** The status moved into each pane rather than staying in the bar, because two panes can need different messages. It sits over the screen because a message taking room would resize the shell on every change of tab. The plan said to decide this from how it looks; it was decided without seeing it, for lack of a browser.

**Validation:** `TestTerminal` tests of `internal/webui` and `conformance/05-web` passed unchanged. `TestTerminalPanelInBrowser` was skipped for lack of Chrome.

**Remaining:** Looking at it.

## Step 3: two panes

**Commits:** 2026-11-06 10:58:05 `9838e5f4`.

**Change:** **Split** and **Unsplit**, `openPane`, `closePane`, `swapInto`, a handle, and `rememberLayout` with a `rotari-terminal-split` key in `sessionStorage` in `web_shell.js`; the panes, handle, focus mark, and stacking in `web_shell.css`. `endShells`, `shellGone`, `begin`, the visibility check, marks on tabs, theme, text size, and fitting now consider both panes. `TestTerminalPanel` gained a row-by-row section, with its stand-ins recording writes, pastes, and focus per terminal, and `TestTerminalPanelInBrowser` splits, types in each pane, measures, moves the handle, follows a link, and unsplits. The server did not change.

**Reason:** [Step 3](plan.md#step-3-two-panes).

**Plan impact:** Swapping changes only each pane's `order`, so nothing reconnects. Unsplit was added, since the plan's table had no way to close a pane by choice; it keeps the focused pane. Narrow windows stack under 720 pixels. Both panes are fitted before either connects, so a restored pane does not connect at the whole width. Opening a page by **Open a terminal here** with a split remembered starts the new shell in one pane and forgets that split, since it returns before restoring.

**Validation:** `go test ./...` passed, as did `go vet ./...`, `gofmt`, `tsc`, and `-race` on the terminal tests of `internal/webui`. Fifteen deliberate breakages of `web_shell.js` were tried against `TestTerminalPanel`; thirteen failed it at once, and the other two showed weak tests (a retry under way when its pane closes, and the focused pane's shell ending last), which were strengthened until those failed too. **`TestTerminalPanelInBrowser` has not run:** there is no Chrome here, and downloading one was refused, so only its script's syntax was checked, and the real xterm.js has not run any of this. Layout, focus, and dragging have not been seen.

**Remaining:** The browser test's first run, and the checks by hand in the plan.

## Step 4: promise and document

**Commits:** 2026-11-06 11:03:01 `dd345972`.

**Change:** Contract WEB-33 and its row in `contracts/README.md`, `docs/WEB.md`, the Web terminal plan's status, and this plan. WEB-34 did not change, as the server did not.

**Reason:** [Step 4](plan.md#step-4-promise-and-document).

**Plan impact:** The plan's note on the Maximize failure was removed, as the user said it is resolved.

**Validation:** As for step 3, which ran on this text's code. `go test ./...` passed.

**Remaining:** Integrating into `main`, once the panes have been tried in a browser.
