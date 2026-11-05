# Work History: Show Distributions in Table Viewer Describe

See [plan.md](plan.md) for current scope and status.

## Describe distributions

**Commits:** 2026-11-05 13:48:35 `d360585a`.

**Change:** `internal/tableview/describe.go` adds quartiles, a 20-bin histogram, and top values to `ColumnStats`, and `distribution_over` to `Description`. The page adds the rows in `table_view.js`, with plain elements for bars. Extended WEB-19, `TestWebTableViewerReadsArtifacts`, and `docs/WEB.md`, and added `TestDescribeShowsDistributions`, `TestDescribeDistributionEdges`, `TestDescribeLimitsDistributions`, and `TestTableViewDescribesDistributionsInBrowser`.

**Reason:** The user chose quartiles, top values, and histograms together.

**Plan impact:** As planned. Three existing tests that counted seven Describe rows or read `count` as the first now expect the twelve rows and find `count` by name, since the rows are what changed.

**Validation:** `go vet ./...`, `tsc`, and `go test -timeout 15m ./...` passed in a separate workspace at the regex filter commit `41466558`, the last of the three, and Prettier passed on the JS and CSS. The expected quartiles are hand-computed with pandas' linear method; pandas itself was not run. `go test -race`, `pre-commit`, and `scripts/check.sh` did not run, and no one has tried it in a browser yet. The plan's before-and-after timing on a large file was not done, and the bars have not been seen in either theme.

**Remaining:** Trying it in a browser in both themes, and timing a large file.
