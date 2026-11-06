# Work log: The Web terminal panel on the left or right

## Step 1: name the room the panel takes

**Commits:** 2026-11-06 14:20:06 `9f4565ef`.

**Change:** In `web_shell.css`, `body`, dialogs, and notes keep clear of `--terminal-left` and `--terminal-right`, which nothing yet sets. In `web_shell.js`, the size and resizer work in either direction, with `panelPlace` always the bottom. No test changed.

**Reason:** [Step 1](plan.md#step-1-name-the-room-the-panel-takes-still-at-the-bottom). The user reviewed the plan's proposals and said to go ahead.

**Plan impact:** `--terminal-height` kept its name as the room at the bottom, so no test read a renamed rule. The sidebar's resizer goes by how far the pointer moved, which removes a risk the plan named.

**Validation:** The `Terminal` tests of `internal/webui` and `conformance/05-web` passed unchanged.

**Remaining:** None.

## Step 2: left and right

**Commits:** 2026-11-06 14:22:42 `0c090f18`.

**Change:** `rotari-terminal-place` and `rotari-terminal-width` in `localStorage`, `applyPlace` setting `data-terminal-place` before the page is drawn, and the fallback under 1000 pixels by `matchMedia` in `web_shell.js`; the side's rules in `web_shell.css`. Stacking now measures the panes' box with a `ResizeObserver`. `TestTerminalPanel`'s stand-ins gained observers that a test can fire, and its stacking checks set the box's size rather than a narrow window.

**Reason:** [Step 2](plan.md#step-2-left-and-right).

**Plan impact:** Stacking by the panel's width is the same as before at the bottom.

**Validation:** See step 4.

**Remaining:** None.

## Step 3: the bar

**Commits:** 2026-11-06 14:41:27 `07bbe505`.

**Change:** **Panel position**, an icon showing the panel's place, opening a menu of Bottom, Left, and Right through `openPanelMenu`, which the tab menu now also uses. The bar wraps at a side, with the tabs on a row of their own. `TestTerminalPanelInBrowser` puts the panel at each side, measures it, the page, the top bar, a dialog, and the terminal, follows a link, and narrows the window.

**Reason:** [Step 3](plan.md#step-3-the-bar).

**Plan impact:** In a narrow window, Left and Right are disabled and say why, since choosing one would visibly do nothing.

**Validation:** See step 4.

**Remaining:** The look of the wrapped bar.

## Step 4: promise and document

**Commits:** 2026-11-06 14:41:28 `8bc7cac6`.

**Change:** Contract WEB-33 and its row in `contracts/README.md`, `docs/WEB.md`, this plan, and the columns plan and log, including the result of CI on `7078e316`. WEB-34 did not change, as the server did not.

**Reason:** [Step 4](plan.md#step-4-promise-and-document).

**Plan impact:** All steps are implemented.

**Validation:** `go test ./...` passed, and `go vet ./...`, `gofmt`, `tsc`, and `-race` on the terminal tests of `internal/webui` found nothing. Fifteen deliberate breakages of steps 1 to 3 each failed `TestTerminalPanel`. `tsc` does not cover `web_shell.js`. **`TestTerminalPanelInBrowser` has not run on this,** for lack of Chrome here, so only its script's syntax was checked, and no one has seen a panel at a side. `scripts/check.sh` and pre-commit could not be run here, so their parts were run by hand.

**Remaining:** The user's look at each place, as listed under [Tests and validation](plan.md#tests-and-validation), and the browser test's first run on CI, before integrating.

## The top bar under a panel at a side

**Commits:** 2026-11-06 15:00:36 `f99d7745`.

**Change:** In `web_shell.css`, the top bar and its toolbar wrap with the panel at a side. In `web_shell.js`, `pageWidthNeeded` counts the sidebar's width and 400 pixels for the content, replacing 480 for the whole page, and sizes apply again when the page exists and when the sidebar is resized. `TestTerminalPanel` covers the limit, and `TestTerminalPanelInBrowser` grows the panel as far as it goes at each side, at 1280 and 1000 pixels, and measures the top bar against it and that the page does not scroll sideways.

**Reason:** The user put the panel at the right and widened it, and Refresh and Settings ended up over the panel. The toolbar has `flex: none` and did not wrap, so it ran past the content, and the top bar's `z-index` of 6, which lets its menus open over the panel at 5, put it on top.

**Plan impact:** Decision 4's limit changed as above. A saved width stays as chosen and comes back when there is room.

**Validation:** The cause was read from the CSS, not reproduced, for lack of a browser here. The `Terminal` tests of `internal/webui` and `conformance/05-web` and `gofmt` passed. The browser test's measurement has not run, and the page's own sideways scroll check may find something else on CI.

**Remaining:** The user's look.

## Tried and integrated

**Commits:** The entries above, moved onto `main` together with this one.

**Change:** This plan's status and this log. No code.

**Reason:** `main` was held back for the user's look, since no one had seen a panel at a side.

**Plan impact:** All steps are done.

**Validation:** The user tried the build with the top bar's fix and said it looks good. Which of the checks by hand they went through was not stated. `TestTerminalPanelInBrowser`'s checks at a side had still not run when this was pushed.

**Remaining:** The result of CI on this push.

## A panel at the bottom going back to its height

**Commits:** 2026-11-06 16:34:05 `59bb8c58`.

**Change:** In `web_shell.js`, the observer of the sidebar applies only the saved width, and only when the sidebar's width changes. `TestTerminalPanel` drags and presses a key at the bottom with observers firing, and `TestTerminalPanelInBrowser` measures the height after a key.

**Reason:** The user reported that resizing the panel at the bottom worked only sometimes: it flashed and went back. `f99d7745` added the observer to reapply every saved size, and the sidebar's height follows a panel at the bottom, so each change set the panel back to the saved height. Whether a drag survived depended on the order of pointer moves and the observer, and a release could save the old height. This was pushed in `34a8551b`.

**Plan impact:** None.

**Validation:** The new test failed before the fix with `'300px' !== '400px'`, and passes after it. `go test ./...` passed on top of `4c81ccb1`, and `go vet ./...`, `gofmt`, and `tsc` found nothing. `TestTerminalPanelInBrowser` was skipped here for lack of Chrome, and the user trying it is the check in a real browser.

**Remaining:** The user's look, then integrating and pushing, and CI's result for the browser checks at a side.
