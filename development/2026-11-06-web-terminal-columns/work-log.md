# Work log: Up to four panes in the Web terminal panel

## Step 1: place panes by position, still two

**Commits:** 2026-11-06 11:33:29 `f322c1bd`.

**Change:** In `internal/webui/assets/web_shell.js`, `columns` holds the layout as columns of rows, `applyLayout` places panes and handles by position, one handle is made per boundary, `moveBoundary` moves a share between two neighbors, and `matchMedia` decides stacking. `web_shell.css` positions panes and handles and loses its 720-pixel rule. In `TestTerminalPanel`, checks that read `order` and `flexGrow` read positions instead, the check of the CSS rule became one of stacking through the script, and the stand-in for `matchMedia` can report a narrow window. In `TestTerminalPanelInBrowser`, the panes' edges may now differ from the box's by a pixel, since positions are rounded.

**Reason:** [Step 1](plan.md#step-1-place-panes-by-position-still-two). The user chose to start this while step 0's CI run was still going.

**Plan impact:** The layout's form and the placing are general already, including handles between rows and shares within a stacked column, though only one or two columns of one row can be made. Handles keep the 0.2 limit on a share for now; step 2 replaces it with the smallest pane. The saved form is unchanged until step 2. Decision 3 was accepted tentatively, so this waits for the user's look before step 2.

**Validation:** The `internal/webui` and `conformance/05-web` packages passed, and `go vet` on them, `gofmt`, and `tsc` found nothing. `go test ./...` was not run, since only these changed. Eight deliberate breakages of the new code each failed `TestTerminalPanel`. **`TestTerminalPanelInBrowser` has not run on this,** for lack of Chrome here, and step 0's CI result had not arrived when this was committed, so the real layout has not been seen.

**Remaining:** The user's look at the diff and a build, and step 0's result, before step 2.

## Step 0: the first run on CI

**Commits:** None; see [the split plan's log](../2026-11-06-web-terminal-split/work-log.md#first-run-on-ci).

**Change:** None.

**Reason:** [Step 0](plan.md#step-0-the-first-run-on-ci).

**Plan impact:** Step 1 was started before this finished, as the user chose, and nothing needed undoing: every workflow passed, including the split checks.

**Validation:** As in that log. It covers `a7b14295`, so not step 1 onward.

**Remaining:** None.

## Steps 2 and 3: columns, rows, and the bar

**Commits:** 2026-11-06 11:59:29 `ff72123a`.

**Change:** In `web_shell.js`, `layoutGeometry`, `withSplit`, `removeRow`, `hasRoom`, `splitPane`, `restoreLayout`, and `rememberedLayout` with a `rotari-terminal-layout` key, and **Split right**, **Split down**, and **Close pane** in the bar. `TestTerminalPanel` builds every shape of up to four panes and closes each pane of each, checking that the panes fill the panel without overlapping, and covers the plan's table for rows and columns. `TestTerminalPanelInBrowser` goes on to four panes, types in each, measures them, follows a link, and closes them.

**Reason:** [Steps 2](plan.md#step-2-columns-and-rows) and [3](plan.md#step-3-the-bar), in one commit since the operations cannot be reached without their buttons. The user looked at step 1 and said it looked good.

**Plan impact:** The handle's limit on a share became the smallest pane. A saved share is held to between 0.1 and 1 before shares are made to add up. Labels stayed text rather than icons, which the plan left open until the bar can be seen at ordinary widths.

**Validation:** See step 4. Twenty-four deliberate breakages of the code in this step and step 1 were tried, sixteen of them here. Twelve of those sixteen failed `TestTerminalPanel` at once. Three showed weak tests, on the scale of a stacked boundary, focus going to the neighbor, and a split choosing a shell already in a pane, which were strengthened until they failed. The last, leaving out `fitPanes` after a split, showed the call was not needed, and it was removed. **`TestTerminalPanelInBrowser` has not run on this,** for lack of Chrome here, so only its script's syntax was checked, and no one has yet seen more than two panes.

**Remaining:** The user's look, in particular whether the bar has room for its buttons, and the browser test's first run with four panes.

## Step 4: promise and document

**Commits:** 2026-11-06 12:04:59 `5f8ba486`.

**Change:** Contract WEB-33 and its row in `contracts/README.md`, `docs/WEB.md`, the split plan's status and log, and this plan. WEB-34 did not change, as the server did not.

**Reason:** [Step 4](plan.md#step-4-promise-and-document).

**Plan impact:** The split plan is done.

**Validation:** `go test ./...` passed with these documents and the code above, and `go vet ./...`, `gofmt`, `tsc`, and `-race` on the terminal tests of `internal/webui` found nothing. `scripts/check.sh` and pre-commit could not be run here, so their parts were run by hand.

**Remaining:** Integrating into `main`, once the user has tried it.
