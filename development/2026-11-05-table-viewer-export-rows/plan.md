# Plan: Export a Table Viewer's Selected Rows

Created: 2026-11-05
Status: Implemented. Its browser checks are under Remaining work; see
[work-log.md](work-log.md).
Related: [keyboard row navigation](../2026-11-05-table-viewer-row-navigation/plan.md),
which is independent of this plan.

## Purpose

Filters find the rows that matter, such as utterances with a high error rate,
but the result stays on screen: **Download** gives the whole file, and only
when the viewer has one to offer. Let a user download the selected rows, in the
file's own format, so another tool or a later job can read them.

## Scope

1. A new `GET /tableview/api/export` returns every row the current filters
   select, in the current sort order, across all pages.
2. An **Export rows** link in the Table viewer downloads them.
3. Both live entry points get it: `rotari web` artifacts and `rotari tableview`.

## Non-goals

- A single-column list, clipboard copy, or a choice of output format. A column
  can be cut from the export with other tools; add this later if it is missed.
- Saving on the server, or any write to the project or the job's directory.
- Exports from static snapshots. See Decisions.
- Hiding columns from the export. There is no column selection in the viewer.
- Excel, Parquet, or other formats.
- Neutralizing spreadsheet formulas in cells. The export reproduces the data.

## Decisions

The user chose full rows in the original format on 2026-11-05.

- **Format by source.**

  | Source | Output |
  | --- | --- |
  | Delimited | The chosen separator; the header record first when `header=1`, none when `header=0` |
  | JSON Lines | Each selected record's original line |
  | Directory **Files** or **Group by stem** | CSV of the listing's columns, as displayed, with a header |

- **JSON Lines keeps its original lines.** Cells turn numbers into text, nulls
  into blanks, and nested values into compact JSON, so rebuilding a record from
  them would change the data.
- **Delimited rows are re-encoded, not copied.** Reading normalizes quoting
  (`LazyQuotes`) and line endings, and blank lines are skipped. The promise is
  that reading the output with the same settings gives the same cells, not the
  same bytes. Short rows stay short.
- **The header is the file's own first record.** `columns()` substitutes
  `column N` for a blank header cell and pads to the widest row; that name must
  not enter the output.
- **Full cells.** Export must not go through `Page`, which cuts cells to 4 KiB
  and adds media properties. It reads through the sources directly, with the
  same 16 MiB record limit as reading.
- **Selection is shared with paging.** Export uses `Table.selection`, so it gets
  the same filters, the same cache, and the same 1,000,000-row sort limit.
  Exported rows match what the pages show, in the same order, including a sort
  by a column of a filtered table.
- **A directory exports what the listing holds.** Only expanded directories'
  files appear, as on screen. A listing cut short keeps its note on the page; the
  export is the same rows. The link's title mentions this.
- **Streamed, with no row limit of its own.** Rows are read and written in
  batches of up to 1,000 display positions. Each batch is read in ascending order
  (as the sources require) and written back in display order, so memory does
  not grow with the file. A sorted export of a big file is slow but bounded.
- **Failures cannot look like success.**
  - Everything checkable is checked before the first byte: source, filters,
    sort, and `base`-independent parameters. These get the same refusals and
    statuses as `/tableview/api/rows`.
  - An error after the first byte aborts the connection with
    `http.ErrAbortHandler`, so the browser reports a failed download rather than
    keeping a short file. This includes a file changed underneath the export,
    when reading notices it.
  - A file rewritten to a same-shaped one mid-download is not detected. Paging
    has the same limit.
- **Unknown paging is refused.** `offset` and `limit` on this route are errors,
  since a caller passing them expects a part of the result.
- **Security stays the rows route's.** Same `view.resolve` and `Target` roots,
  no new file is reachable, and only cells of the source are returned: it does
  not read the files they name. It is a `GET` with no side effects, so it needs
  neither `--allow-exec` nor control and works on a read-only server. It uses
  `noStore` headers and `Content-Disposition: attachment`.
- **The file name is derived from the source.** `<stem>.export<ext>`, such as
  `utterances.export.csv`, and `<directory>.export.csv` for a listing. It is
  encoded with `mime.FormatMediaType`, because names are arbitrary strings.
- **Static snapshots have no export.** They have no server, and their filters
  are fixed, so they would offer little beyond the sort. The page hides the link
  on the existing `info.static`, and the snapshot's API replacement already refuses
  routes it does not have, with a reason.

## Variants to cover

| Variant | Expected |
| --- | --- |
| Delimited: comma, tab, another separator | Same separator; cells round-trip |
| `header=1` and `header=0` | Header record present or absent |
| Blank header cell, ragged rows, quoted separators and line breaks | Original header; short rows stay short; cells round-trip |
| A row holding one empty cell | Survives, written as `""`. `csv.Writer` would write a blank line, which reading skips; confirm and handle this |
| JSON Lines: objects, arrays, scalars, nested values, nulls | Original lines |
| JSON Lines with a malformed record | Refused or aborted, never a silently shorter file |
| No filters, filters, sort, filters and sort | Matches the concatenated pages exactly |
| No rows matched | The link is hidden; the API returns the header only |
| Cell over 4 KiB | Whole in the export |
| Directory **Files**, expanded, and **Group by stem** | CSV of the listing |
| Non-ASCII or odd source names | Valid `Content-Disposition` |
| Web token set; `rotari tableview --auth-token` | Same authentication as other API routes; the link works from a signed-in page |
| Path outside roots or an escaping symlink | 403, as other routes |
| Document preview | No table, no link |
| Static snapshot | No link; route refused with a reason |

The Python client and the CLI do not read the Table viewer's API, so they do not
change. `rotari tableview` gets this through the shared handler.

## Implementation

### Phase 1: `internal/tableview`

- Add `Table.Export(query, writer)`, which validates, then streams. It keeps
  HTTP out of the package, and returns whether it has started writing so the
  caller can choose between a refusal and an abort.
- Add a source method that writes rows by number. Each source implements it:
  - delimited writes with `encoding/csv` using its separator;
  - JSON Lines writes original lines; factor the line loop in `rows` so both
    paths share reading, and the "file changed" check;
  - directory writes `row()` as CSV.
- Unit tests go in `delimited_test.go`, a new `jsonl_test.go` if none fits, and
  `directory_test.go`.

### Phase 2: `internal/webui`

- Add the route in `table_view.go` beside `/api/rows`, reusing its parsing of
  filters and sort. Do not duplicate it: move it into a helper used by both
  routes if that is what it takes.
- Add the live-only info flag; leave `GenerateStaticTableView` without it.
- Update the comment at the top of `table_view.go` that lists parameters.
- Refuse the route in `table_view_static.js`.

### Phase 3: Page

- Render an **Export rows** link in `renderPager` beside the count, built with
  `apiURL` from the same parameters as `loadRows`. Render it only after rows have
  loaded without error and matched at least one row. A browser shows no response
  body for a failed `download` link, so the page's own query must have proven
  valid first.
- Its title gives the row count, so the user knows it covers every page.

### Phase 4: Documents and contracts

- Add it to "Viewing tables" in `docs/WEB.md`, including that a snapshot has no
  export.
- Add a contract with the next free ID (WEB-30 is reserved by the row navigation
  plan, so WEB-31 if that lands first) in
  `contracts/05-web-assets-and-static-export.md`, and its row in
  `contracts/README.md`.
- Check `docs/SECURITY.md` for a list of what Table viewer routes may read; add
  this route if it enumerates them.

## Tests and validation

- **Conformance:** a table test in `conformance/05-web/table_view_test.go`
  through the binary and Web API, with `covers(t, ...)`: each format against the
  concatenated `/tableview/api/rows` output (delimited) or the original lines
  (JSON Lines), refusals, and headers. Fixture data must tell the variants apart.
- **jsdom in `internal/webui/table_view_test.go`:** the link's visibility and
  its URL following filters and sort; no link in the snapshot, in
  `table_view_static_test.go`.
- **Abort:** a test that a read failure after the first byte ends the response
  in a client-visible error. Use a source that fails, since a real file is
  awkward to change at the right moment.
- **Scale:** a sorted export of a file over one batch, checking display order
  across batch boundaries.
- **Checks:** `go vet`, `tsc`, `go test ./internal/tableview ./internal/webui`,
  `go test ./conformance/...`, and `scripts/check.sh` once. Report any that cannot
  run on the development machine, and run their steps directly.
- **Manual:** clicking the link in a real browser, including with a Web token,
  and opening the result. No browser is installed on the development machine, so
  the user needs to try this.

## Risks

- **Large sorted exports hold a request and read much of the file repeatedly.**
  Batching keeps memory bounded, and cancellation should stop the work when the
  client disconnects. Check for the request context between batches.
- **Spreadsheet formulas.** A cell starting with `=` opens as a formula in a
  spreadsheet. That is true of the source file too, and the export must reproduce
  data, so this is a documented non-goal rather than a filter.

## Remaining work

Trying it in a browser, including with a Web token.
