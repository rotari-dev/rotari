# Work History: Table Viewer Files in Their Own Windows

See [plan.md](plan.md) for current scope and status.

## Prototype, then windows that stay in step

**Commits:** 2026-11-05 14:23:59 `20fa5e80` (prototype); 2026-11-05 14:32:56 `8fac8823`.

**Change:** The prototype's window opening moved to a new `internal/webui/assets/table_view_windows.js`, served and copied beside the page's other scripts. It adds the channel, dragging tabs between windows, and returning a file. `table_view_panes.js` accepts drags from other windows and pauses players on request; `table_view.js` sends row moves and `Space` to the active window. Added contract WEB-32, `TestTableViewerMovesTabsToWindows`, `TestTableViewPopsOutATabInBrowser`, and a paragraph in `docs/WEB.md`.

**Reason:** The user asked to move panes between browser windows by dragging. Whether a browser allows a window from the end of a drag could only be found by trying, so a prototype came first.

**Plan impact:** The user tried the prototype in Chrome and said it looked fine. Closing a window closes its files rather than returning them, as this was first proposed, because a reload fires the same event.

**Validation:** Prettier, `go vet` on the changed packages, `tsc`, the Table viewer tests in `internal/webui`, the Table viewer conformance tests, and `TestContractStatus` passed. They use stand-ins for `window.open`, the drag events, and the channel, so real windows, drags between them, and the channel have not been exercised by anything since the prototype. The full suite's result is not recorded here.

**Remaining:** The manual checks in `plan.md`, including other browsers.
