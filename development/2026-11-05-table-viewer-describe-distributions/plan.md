# Plan: Show Distributions in Table Viewer Describe

Created: 2026-11-05
Status: Proposed. Nothing is implemented.
Related: [keyboard row navigation](../2026-11-05-table-viewer-row-navigation/plan.md)
and [row export](../2026-11-05-table-viewer-export-rows/plan.md), which are
independent of this plan.

## Purpose

Describe gives each column's count, distinct values, mean, standard deviation,
minimum, maximum, and sum. Those do not show skew, outliers, or dominant
values, which is what a user checking evaluation results is looking for. Add
quartiles, most frequent values, and a small histogram per column.

## Scope

`/tableview/api/describe` and the Describe rows gain:

1. **Quartiles:** 25%, 50%, and 75% of a column's numeric cells.
2. **Top values:** a column's up to 5 most frequent non-blank values and their
   counts.
3. **Histogram:** a small bar chart of a column's numeric cells.

The display order follows pandas: `histogram`, `count`, `unique`, `top`, `mean`,
`std`, `min`, `25%`, `50%`, `75%`, `max`, `sum`.

## Non-goals

- Clicking a bar or a top value to add a filter. It is a natural follow-up, but
  deserves its own decision.
- Choosing bin counts, log scales, or quantiles other than the three.
- Value-count or chart views outside the Describe rows.
- Approximate statistics. See Decisions.
- New contract IDs. This extends the existing WEB-19 Describe rule.

## Decisions

The user chose quartiles, top values, and histograms together on 2026-11-05.

- **Exact, or absent with a reason.** Results are never approximate, because a
  number in this table should mean what it says. Sampling is rejected.
- **Quartiles and histograms share one memory budget.** Both come from the
  numeric cells held in memory during the single scan. One Describe holds at most
  10,000,000 numeric values across all columns (about 80 MiB). When the selected
  rows exceed it, every column's values are dropped, `distribution_over` is set on
  the response, and the page says so and suggests a filter. Other statistics are
  unaffected. The values are released once computed; the cached result keeps only
  the small summaries.
- **Quartiles use linear interpolation,** like pandas' default, in keeping with
  WEB-19's "as pandas' `describe` does". They are over cells that read as
  numbers, as `mean` is.
- **Histograms use 20 equal-width bins from minimum to maximum.** A value at the
  maximum goes in the last bin. If all values are equal, there is one bin. If the
  range overflows `float64`, the histogram is left out, as an overflowing sum is
  today.
- **A single extreme value flattens the rest of the histogram.** This is left as
  it is. It shows that an outlier exists, and `max` and the quartiles give the
  scale. A way to zoom would be a separate decision.
- **Mixed columns are stated, not hidden.** A histogram counts only numeric cells,
  as `mean` does, and `count` next to it shows how many cells there are.
- **Top values are bounded on memory and exact when present.**
  - One column tracks up to 10,000 distinct values, by the same hash as `unique`,
    with a stored copy of at most 256 bytes each (at most about 2.5 MiB per
    column).
  - Past 10,000, tracking for that column stops and it reports `top_over`, since
    any listed value would be a guess. The page shows that there are too many
    distinct values to list.
  - A value cut to 256 bytes, on a rune boundary, is marked `cut`.
  - Ties go to the value seen first, so results are repeatable.
  - If every value appears once, no list is returned, since it would be an
    arbitrary five rows. This is common for IDs and paths.
- **Same selection as today.** Results cover the rows the filters select, cells
  are non-blank, and the existing cache key still applies.
- **Same scan.** No second pass is added, so the number of file reads per Describe
  stays one. It matters on NFS.
- **API changes are additive.** Existing fields keep their meaning. Fields not
  applicable to a column are omitted:

  ```json
  {
    "columns": [{
      "q25": 0.5, "q50": 1.0, "q75": 2.5,
      "histogram": { "min": 0, "max": 10, "counts": [3, 1, 0] },
      "top": [{ "value": "a", "count": 4, "cut": false }],
      "top_over": false
    }],
    "distribution_over": false
  }
  ```

- **Bars are plain elements.** Each is a `div` with an inline height, which the
  page policy (WEB-29) already allows for data-driven sizes. No chart library or
  canvas, so jsdom can test it and a static snapshot needs nothing more.
- **Text stays text.** Top values are inserted as text nodes, as WEB-28 requires,
  and can never become markup.
- **Typed columns format the same way.** Quartiles and bar titles use
  `formatValue` with the column, so durations and sizes in a directory listing
  read as such.

## Variants to cover

| Variant | Expected |
| --- | --- |
| Numeric column | All three, with quartiles matching a pandas-computed fixture |
| Text column | Top values only |
| Numeric with few values, such as 0/1 labels | Histogram and top values |
| Mixed numeric and text | Quartiles and histogram over numeric cells; top values over all |
| All blank column, or one short row missing the cell | Blanks excluded, nothing shown where nothing applies |
| One numeric value, or all equal | Quartiles equal it; one bin |
| Range overflowing `float64` | No histogram; quartiles if finite; no `Inf` or `NaN` in JSON |
| All-unique column | No top list and no `top_over` |
| Over 10,000 distinct values | `top_over` set; the page says so |
| Over the 10,000,000-value budget | `distribution_over`; other statistics are correct |
| Very long cell or multi-byte text at the cut | Valid UTF-8, marked `cut` |
| Value containing markup | Shown as text |
| Filters | Everything recomputed over selected rows, including no rows |
| JSON Lines; directory Files and Group by stem | Same rules; typed columns formatted |
| Static snapshot | Works from the embedded Describe, and its size stays reasonable |
| `/tableview/` and `rotari tableview` | Same page, same behavior |

## Implementation

### Phase 1: `internal/tableview/describe.go`

- Extend `ColumnStats` and `Description`, and `columnTally` with held values and
  value counts. Keep all of it behind `table.describe`, so there is one scan.
- Reuse `seen` for `unique` as it is. `uniqueLimit` remains 100,000, separately
  from the top-value limit.
- Add the budget as a constant beside `uniqueLimit`, and make limits injectable
  for tests the way `TestDescribeCountsUniqueValuesUpToALimit` handles its limit.
- Extend `TestDescribeSummarizesColumns`, and add tables for the variants above.

### Phase 2: Page

- Add the new rows to `describeRows` in `internal/webui/assets/table_view.js`. A
  row's renderer may now return an element as well as text.
- Add the note for `distribution_over` near Describe's rows.
- Add bar and list styles in `table_view.css`, in both themes. Describe rows are
  already not sticky, so taller rows scroll away with the page rather than
  covering the table.
- Give each histogram an accessible summary with `role="img"` and an `aria-label`
  stating its range and total, and give each bar a `title` with its range and
  count.
- Load the `dataviz` skill before writing the chart code.

### Phase 3: Documents and contracts

- Update WEB-19 in `contracts/05-web-assets-and-static-export.md` and its row in
  `contracts/README.md`.
- Update "Viewing tables" in `docs/WEB.md`, including the limits.

## Tests and validation

- **Conformance:** extend `TestWebTableViewerReadsArtifacts` in
  `conformance/05-web/table_view_test.go` through the binary and Web API.
- **`internal/tableview`:** the variant table. Compare quartiles against values
  computed with pandas once and written into the test, not against the code's
  own formula.
- **jsdom in `internal/webui/table_view_test.go`:** the rows, bars, notes, and
  markup-in-a-value case, and the snapshot in `table_view_static_test.go`.
- **Budget:** a test that crossing the limit reports it and releases nothing
  wrong, using a small injected limit rather than ten million rows.
- **Checks:** `go vet`, `tsc`, `go test ./internal/tableview ./internal/webui`,
  `go test ./conformance/...`, and `scripts/check.sh` once. Report any that cannot
  run on the development machine, and run their steps directly.
- **Manual:** the look of the bars in both themes and in a wide table, in a real
  browser.

## Risks

- **Describe gets slower.** Each numeric cell is stored, then sorted per column,
  and each cell is hashed into a second count. Measure a large file before and
  after. The cost applies only while Describe is open, and the result is cached.
- **Peak memory rises** by up to about 80 MiB for values plus 2.5 MiB per column,
  per concurrent uncached Describe. Requests are not limited in number today.
- **Taller Describe rows push the table down.** The toggle remains, and the top
  list is capped at 5 lines. Check this in a real browser.

## Remaining work

All phases. Add `work-log.md` after the first commit, as
[development/README.md](../README.md) describes.
