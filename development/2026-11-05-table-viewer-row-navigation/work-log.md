# Work History: Move Between Table Viewer Rows from the Keyboard

See [plan.md](plan.md) for current scope and status.

## Plan row navigation

**Commits:** 2026-11-05 10:12:08 `3d2013d5` (with the row export plan); 2026-11-05 10:29:00 `611d784d` (a CI test note, with two other plans).

**Change:** Added `plan.md`.

**Reason:** The user asked what the Table viewer lacks compared with similar tools, then for one plan per feature, starting with the feature that most shortens reviewing many rows.

**Plan impact:** The first version chose that all files of the row follow a move.

**Validation:** Documentation only; no tests run.

**Remaining:** All phases.

## Keys, a framed cell, and table scrolling

**Commits:** 2026-11-05 13:38:06 `4d2642dc`.

**Change:** `internal/webui/assets/table_view.js`, `table_view_panes.js`, `table_view.css`, and `table_view.html`: `↓`/`↑` move a current row across pages, `←`/`→` a framed cell; `Enter` opens the framed file, `Shift+Enter` the row's files, and `Space` plays the framed file. The active pane's current tab follows after 150 ms. The row is kept in the fragment as `row`. Added contract WEB-30, `conformance/05-web/table_view_rows_test.go`, jsdom tests in `internal/webui`, and a section in `docs/WEB.md`. Moved Chrome prerequisites in `hostile_browser_test.go` into `browserCheckTools`.

**Reason:** The plan's first phases, then the user's corrections after trying each version.

**Plan impact:** Trying it changed these decisions from the plan:

- Only the active pane's current file follows, not every file of the row. Hidden tabs stay. An intermediate version moved hidden tabs too.
- The clicked cell is framed and the frame moves with the row, replacing a focus ring that stayed behind. Without a click, it starts on the row's first file.
- `Enter` and `Space` act on the framed cell; opening the row moved to `Shift+Enter`.
- `←`/`→` were added, moves scroll only the table, and the pager moved under the filter bar.

Three other fixes came out of this:

- Focus left on a clicked link kept `Enter` and `Space` from reaching the table. The tests sent keys to `body`, so they missed it until a test focused the link first.
- A hidden tab's player kept playing when `Space` played another file. A test found it. Whether switching tabs by mouse leaves the old file playing was not checked or changed.
- The user found that splitting panes down squeezed the spectrogram to nothing, with no way to scroll to it. Its box could shrink to zero. This predates the work. It now keeps 120 px. It shares this commit because it edits the same stylesheet.

Opening Describe now scrolls the page to the table, as the user asked.

**Validation:** `go vet ./...`, `tsc`, Prettier on the JS and CSS, and `go test -timeout 15m ./...` passed on the final code. The user tried the behavior in a browser and said it worked well, without saying which cases. `TestTableViewerRowKeysInBrowser` skipped because Chrome is absent here; its script was only syntax-checked, so the spectrogram height and Describe scrolling in layout are unverified by tests, and jsdom checks only Describe's scroll arithmetic. `go test -race`, `pre-commit`, and `scripts/check.sh` did not run: `pre-commit` is not installed and `npm` is broken, so its steps ran directly, except for the race run.

**Remaining:** The Chrome test's first run in CI. A Prettier failure on `table_view.html` that predates this work is in `ISSUES.md`.
