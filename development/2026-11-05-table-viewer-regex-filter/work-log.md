# Work History: Regular Expression Filters in the Table Viewer

See [plan.md](plan.md) for current scope and status.

## Regex filters and refused filters

**Commits:** 2026-11-05 13:49:07 `41466558`.

**Change:** `internal/tableview/filter.go` adds `regex` and `not_regex`. `table_view.js` offers them, rolls back a refused new condition in `addFilter`, and keeps chips when `rows` is null. Extended WEB-19, `TestBadFiltersAreRefused`, `TestWebTableViewerReadsArtifacts`, and `docs/WEB.md`, and added `TestRegexFiltersSearchCellText` and `TestTableViewRecoversFromARefusedFilterInBrowser`.

**Reason:** The user chose regular expressions alone, without OR across columns. A mistyped pattern would strand the user far more often than other refusals did.

**Plan impact:** As planned, and the recovery covers every operator.

**Validation:** `go vet ./...`, `tsc`, and `go test -timeout 15m ./...` passed in a separate workspace at the regex filter commit `41466558`, the last of the three, and Prettier passed on the JS and CSS. The plan asked to reproduce the stranded page first; the recovery test was written with the fix and not run against the old code, so the old failure rests on reading the code. `go test -race`, `pre-commit`, and `scripts/check.sh` did not run, and no one has tried it in a browser yet.

**Remaining:** Trying it in a browser.
