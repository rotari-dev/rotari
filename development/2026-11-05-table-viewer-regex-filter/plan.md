# Plan: Regular Expression Filters in the Table Viewer

Created: 2026-11-05
Status: Implemented. Its browser checks are under Remaining work; see
[work-log.md](work-log.md).
Related: [row export](../2026-11-05-table-viewer-export-rows/plan.md), which will
export whatever these filters select, and
[Describe distributions](../2026-11-05-table-viewer-describe-distributions/plan.md).

## Purpose

Filters can test whether a cell contains fixed text, and every condition must
hold. Selecting "these three datasets" or "IDs of this shape" is not possible.
Add regular expression operators. An alternation such as `a|b` also gives OR
within one column.

## Scope

1. Operators `regex` and `not_regex` in `/tableview/api/rows` and
   `/tableview/api/describe`, offered in the filter bar as **matches regex** and
   **does not match regex**.
2. A refused filter no longer strands the user. See "Recovering from a refused
   filter" under Decisions. A mistyped pattern makes this much more likely than
   it is today.

## Non-goals

- OR across columns. The user chose regular expressions alone on 2026-11-05.
  Conditions still all have to match.
- Case-insensitive or whole-cell options as separate controls. The pattern
  expresses them: `(?i)`, `^...$`.
- Filters in static snapshots, which remain fixed.
- Validating the pattern in the browser. JavaScript's regular expressions differ
  from Go's, so the server's answer is the only true one.

## Decisions

- **Operator names are `regex` and `not_regex`.** `~` is rejected because
  `TestBadFiltersAreRefused` uses it as the unknown operator, and because the
  existing word-named operators read the same way.
- **Go's `regexp` (RE2) syntax.** It adds no dependency, and its matching time is
  linear in the input, so a pattern from a shared link cannot hang the server with
  backtracking. Lookaround and backreferences are not available, and the error
  says so through Go's own message. The docs state this.
- **It searches, like `contains`.** The pattern may match anywhere in the cell;
  `^` and `$` anchor it. It is case-sensitive unless it uses `(?i)`, like
  `contains`.
- **Raw cell text, like `contains`.** A typed column is matched on its stored
  value, not on its formatted display: a duration shown as `1:30` is matched
  against `90`. The docs say so, because this will surprise someone.
- **A missing cell reads as blank,** as in all operators. `not_regex` therefore
  selects a blank cell unless its pattern matches the empty string.
- **The empty pattern is allowed.** It matches everything, as an empty `contains`
  does today, so it adds no special case.
- **Refused before reading.** An invalid pattern is an error from
  `compileConditions`, naming the column and giving the reason. The error appears
  on `rows`, `describe`, and any later route that compiles conditions, as for other
  bad filters.
- **The compiled pattern is unexported.** `condition` is JSON-encoded for the
  result cache key, so the new field must not enter it. `Condition.Value`, which
  holds the pattern, already identifies the result.
- **No pattern length limit of its own.** `regexp` rejects programs that are too
  large, and the request's size is already limited. Confirm that a long pattern
  gets a refusal, not a crash.

### Recovering from a refused filter

From reading `table_view.js`, a refused filter appears to leave the user with no
way out except editing the URL. `loadRows` sets `rows` to null, `renderFilterBar`
then empties the bar, and the condition is already in `view.filters` and in the
URL, so a reload refuses again. Reproduce this first, using `>` with a text value,
which reaches the same path today.

- **While adding a condition:** a refusal removes the condition again, reloads
  the previous rows, keeps the error on screen, and restores the typed value and
  operator in the form, so the pattern can be corrected.
- **With a bad filter from a URL:** there are no earlier rows to go back to, so the
  bar still shows the chips and their **×** when `rows` is null, naming columns as
  `column N`. The error stays visible.
- **It applies to every operator,** since this is one code path. `> high` gets the
  same recovery.

## Variants to cover

| Variant | Expected |
| --- | --- |
| `regex`, `not_regex` | Search anywhere; `^...$` anchors; `(?i)` ignores case |
| Alternation `a\|b` | Rows with either |
| Invalid pattern, lookaround, very long pattern | Refused with a reason on both API routes |
| Blank cell, short row | Treated as an empty string by both operators |
| Multi-byte text and invalid UTF-8 cells | No crash; multi-byte patterns match |
| With other conditions | All must match |
| With a sort; Describe over the result | Same selection; no cache mix-up between two patterns |
| JSON Lines; nested value's compact JSON | Matched as the cell's text |
| Directory Files, Group by stem | Matches paths; typed columns on their raw value |
| Pattern with characters needing URL encoding | Survives the fragment and a reload |
| Pattern containing markup | The chip shows it as text |
| Bad filter while adding | Previous rows return; form restored; error shown |
| Bad filter from a URL | Chips removable; error shown |
| Static snapshot | No filter form; refused as before |
| `/tableview/` and `rotari tableview` | Same page, same behavior |

## Implementation

### Phase 1: `internal/tableview/filter.go`

- Add the operators and their doc comment.
- Compile in `compileConditions`, and test in `condition.matches`.
- Extend `TestFiltersSelectRows` and `TestBadFiltersAreRefused`, and test that two
  patterns on one source do not share a cached result.

### Phase 2: Page

In `internal/webui/assets/table_view.js`:

- Add both to `filterOperators`.
- Roll back in `addFilter`, using the message `loadRows` already returns. Keep the
  error shown across the reload, which otherwise clears it.
- Render removable chips in `renderFilterBar` when `rows` is null.
- Do the bug-fix order: write the failing test for the recovery first.

### Phase 3: Documents and contracts

- Add the operators and semantics to WEB-19 in
  `contracts/05-web-assets-and-static-export.md`, and adjust its row in
  `contracts/README.md`. Add the recovery to the page's behavior there.
- Add a short note on syntax and raw values to "Viewing tables" in `docs/WEB.md`.

## Tests and validation

- **Conformance:** extend `TestWebTableViewerReadsArtifacts` in
  `conformance/05-web/table_view_test.go` with a match, a refusal, and Describe
  through the binary and Web API.
- **jsdom:** extend `TestTableViewFiltersInBrowser` in
  `internal/webui/table_view_test.go` with the operators and both recovery paths.
- **Checks:** `go vet`, `tsc`, `go test ./internal/tableview ./internal/webui`,
  `go test ./conformance/...`, and `scripts/check.sh` once. Report any that cannot
  run on the development machine, and run their steps directly.

## Risks

- **Rolling back on any failure.** A network error while adding a condition also
  restores the form, losing nothing the user typed; the error says what happened.
- **A slow pattern on a large file.** Matching is linear, but each new pattern
  rescans the file, as other new filters do. The cost is per cell, and cells can
  reach 16 MiB. Nothing new is needed unless measurement says otherwise.

## Remaining work

Trying it in a browser.
