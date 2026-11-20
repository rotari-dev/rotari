# Plan: Search Sticky Notes

Created: 2026-11-20
Status: Implemented. Its browser check is under Remaining work.
Related: [sticky notes](../2026-11-18-sticky-notes/plan.md)

## Purpose

Sticky notes are kept per page, and the **Stickies** menu lists every note
newest first. Past a few dozen notes the list cannot be scanned. Let a user
find a note by what it says, from the Web UI and from the command line.

## Scope

1. `internal/stickies.Search` keeps the notes whose text holds every
   whitespace-separated word of a query, ignoring case.
2. `GET /api/stickies` takes `q=TEXT`, with or without `page`.
3. `rotari stickies --search TEXT` (read-only, combines with `--page` and
   `--json`).
4. A search box in the **Stickies** menu that narrows its list.

## Non-goals

- Ranking, highlighting, regular expressions, stemming, or fuzzy matching.
- Searching page keys or colours. `--page` already selects a page.
- Search through the Python client beyond its generated CLI metadata; the
  client has no sticky-note API.
- Static exports, which have no sticky notes.

## Decisions

- **One matcher.** `stickies.Search` owns the rule; the API and the CLI call
  it, and the browser sends the text to the API rather than matching itself.
- **All words must match (AND), case-insensitive substring.** Words are split
  on whitespace; `strings.ToLower` is the folding.
- **A query without words is an error**, not "everything": the API answers
  400 and the CLI prints `--search: ...` and exits 1. The browser does not
  send `q` for an empty box and shows the full list.
- **Pages left with no notes are dropped** from a list search. A search of one
  page answers its normal `{page, notes}` shape with the matching notes,
  possibly none.
- **The sidebar marks stay whole.** A search response holds only some notes, so
  the menu does not feed it to the sidebar's page marks.
- **`--search` is command-line only**, like `--page`; it has no environment
  default.

## Interfaces

| Interface | Change |
| --- | --- |
| CLI | `rotari stickies --search TEXT` |
| Server / Web API | `GET /api/stickies?q=` and `?page=&q=` |
| Web UI | Search box above the **Stickies** list, 200 ms debounce |
| Python client | Generated CLI metadata and reference only |

## Tests

- `TestSearchMatchesEveryWordIgnoringCase` in `internal/stickies`.
- `TestWebStickiesAreSearchable` in `conformance/05-web`, covering WEB-62
  through the API and the binary: all words, within a page, no match, empty
  search refused, `--search` with and without `--page`, empty `--search`.

## Remaining work

- The Node on the development machine is v12, so the Web UI type check and
  browser tests did not run; a browser check that typing in the search box
  narrows the list, and that clearing it restores the list, is left to CI.
