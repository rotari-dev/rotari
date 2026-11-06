# Work log: Up to four panes in the Web terminal panel

## Step 1: place panes by position, still two

**Commits:** 2026-11-06 11:33:29 `f322c1bd`.

**Change:** In `internal/webui/assets/web_shell.js`, `columns` holds the layout as columns of rows, `applyLayout` places panes and handles by position, one handle is made per boundary, `moveBoundary` moves a share between two neighbors, and `matchMedia` decides stacking. `web_shell.css` positions panes and handles and loses its 720-pixel rule. In `TestTerminalPanel`, checks that read `order` and `flexGrow` read positions instead, the check of the CSS rule became one of stacking through the script, and the stand-in for `matchMedia` can report a narrow window. In `TestTerminalPanelInBrowser`, the panes' edges may now differ from the box's by a pixel, since positions are rounded.

**Reason:** [Step 1](plan.md#step-1-place-panes-by-position-still-two). The user chose to start this while step 0's CI run was still going.

**Plan impact:** The layout's form and the placing are general already, including handles between rows and shares within a stacked column, though only one or two columns of one row can be made. Handles keep the 0.2 limit on a share for now; step 2 replaces it with the smallest pane. The saved form is unchanged until step 2. Decision 3 was accepted tentatively, so this waits for the user's look before step 2.

**Validation:** The `internal/webui` and `conformance/05-web` packages passed, and `go vet` on them, `gofmt`, and `tsc` found nothing. `go test ./...` was not run, since only these changed. Eight deliberate breakages of the new code each failed `TestTerminalPanel`. **`TestTerminalPanelInBrowser` has not run on this,** for lack of Chrome here, and step 0's CI result had not arrived when this was committed, so the real layout has not been seen.

**Remaining:** The user's look at the diff and a build, and step 0's result, before step 2.
