# Work History: Export a Table Viewer's Selected Rows

See [plan.md](plan.md) for current scope and status.

## Export rows

**Commits:** 2026-11-05 13:44:37 `dc0b1679`.

**Change:** `internal/tableview/export.go` adds `Table.PrepareExport` and `Export.Stream`, with `exportHeader` and `exportRecords` on each source; `jsonl.go` shares its line reading through `lines`. `internal/webui/table_view.go` adds `/api/export`, with `openTableViewSelection` now shared with `/api/rows`. The page gets **Export rows** in the pager. Added contract WEB-31, `conformance/05-web/table_view_export_test.go`, unit and jsdom tests, and a paragraph in `docs/WEB.md`.

**Reason:** The user chose full rows in the original format.

**Plan impact:** Preparing and streaming are two steps rather than the plan's flag for whether writing had started, because that makes refusing before the first byte structural. The page hides the link on the existing `info.static` rather than a new flag, and a snapshot's existing refusal of unknown routes is enough.

**Validation:** `go vet ./...`, `tsc`, and `go test -timeout 15m ./...` passed in a separate workspace at the regex filter commit `41466558`, the last of the three, and Prettier passed on the JS and CSS. `TestSSHJobReadsAsRunning` failed once in the run at this commit, then passed in the run at `41466558` and five repetitions alone; see `ISSUES.md`. `go test -race`, `pre-commit`, and `scripts/check.sh` did not run, and no one has tried it in a browser yet. A large sorted export was not timed.

**Remaining:** Trying it in a browser, including with a Web token.
