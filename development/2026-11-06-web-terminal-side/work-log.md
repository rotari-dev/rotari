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
