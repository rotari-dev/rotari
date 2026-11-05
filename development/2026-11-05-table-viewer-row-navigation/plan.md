# Plan: Move Between Table Viewer Rows from the Keyboard

Created: 2026-11-05
Status: Proposed. Nothing is implemented.

## Purpose

Reviewing an evaluation table means opening hundreds of rows in turn. Each
row now takes mouse clicks on each of its files, or on **Open row**, which
also resets the pane layout. Let a user press one key to move to the next row
and have the files they opened follow, keeping the panes as they arranged
them.

This is the first of several Table viewer improvements. Each has its own plan;
result export and Describe distributions are not part of this one.

## Scope

1. **A current row.** The table marks one row as current. Clicking a row, one
   of its file cells, or its **Open row** makes it current.
2. **Keys.**

   | Key | Action |
   | --- | --- |
   | `↓` / `↑` | Make the next or previous row current |
   | `Enter` | Same as the current row's **Open row** |
   | `Space` | Play or pause the player in the active pane |

3. **Files follow the row.** A pane's visible tab that came from the current
   row is replaced by the same column's file in the new row.
4. **Discoverability.** A short key hint beside the pager, shown when the
   table has rows.

## Non-goals

- Other keys, such as `j`/`k`, a shortcut help dialog, or configurable keys.
- Automatic playback when a row changes.
- Keeping open panes in the URL. They are not part of the view today.
- Any change to `/tableview/api/*`, `internal/tableview`, the CLI, or the
  Python client.

## Decisions

- **Files change with the row (option A).** Moving replaces the files in the
  panes straight away rather than waiting for `Enter`, so one key moves `ref`
  and `hyp` together. The user confirmed this on 2026-11-05.
- **No automatic playback.** A row change must not make a sound, and browsers
  may refuse to start playback anyway. `Space` plays.
- **Only tabs of the current row follow.** Tabs record the source, row number,
  and column they were opened from. A tab from another row, or a file opened
  from another directory, stays where it is. This keeps today's way of
  comparing files across directories, and lets a user hold one row's file as a
  reference while stepping through another column.
- **A row without the file says so.** If the new row's cell is blank, is a
  directory row, or has a media error, the pane shows that message with the row
  and column. It never keeps the old file, which would look like the new row's
  audio. This is also how a missing file appears in **Group by stem**.
- **Moves follow the displayed order.** They respect the current filters and
  sort, and cross pages: past the page's last row, the next page loads and its
  first row becomes current. At either end of the results, the move does
  nothing.
- **Panes stay as arranged.** Layout, sizes, the link toggle, and which pane is
  active are kept. The zoom range and playback time reset, linked or not,
  because the new file is a different length. The **Channel** choice also
  resets, because it belongs to each file's `TabView` and the new file may have
  different channels.
- **Rapid moves load only where the user stops.** The highlight moves
  immediately; pane replacement waits for about 150 ms without another move.
  `decodeAudio` cannot be cancelled and may read up to 512 MiB per file, so held
  keys must not start a decode per row. Its cache already keeps only 8 files.
- **Controls keep their keys.** Handling is skipped when a modifier is held and
  when focus is in an input, select, textarea, or editable element, or in an
  audio or video element. `Space` and `Enter` are also left alone on a button or
  link. `Alt+↑` keeps meaning "up one directory". `Space` does not call
  `preventDefault` when the active pane has no player, so the page still
  scrolls.
- **The URL keeps the current row.** `row=N` joins the fragment, so a reload or
  shared link highlights it. Files do not open from it, as panes are not in the
  URL. An unknown or malformed value, or a row not on the page, highlights
  nothing, as other view values do.

## Variants to cover

| Variant | Expected behavior |
| --- | --- |
| Delimited, with or without a header | Rows move; media columns follow |
| JSON Lines | Same as delimited |
| Directory **Files** | Moves through rows; file cells follow, as text or media; a directory row shows the message |
| Directory **Group by stem** | Each group's tab follows; a missing file shows the message |
| Expanded directories | Row numbers can change on expansion, so the current row is cleared |
| Sorted or filtered | Display order; row numbers remain the file's |
| Filter, sort, page-size, or separator change | The current row stays only if still on the page; panes are untouched |
| Pager used by mouse | Same as above; the next move starts from the page's first row |
| `moveTo` and browser Back | The current row clears; open tabs stay, as they do now |
| Document preview (`info.preview`) | The table is hidden, so row keys do nothing |
| Static snapshot | Same as a live table, through its existing API replacement |
| `/tableview/` and `rotari tableview` | Same page, same behavior |

The main Web UI's artifact preview has no rows and does not change.

## Implementation

### Phase 1: Current row

In `internal/webui/assets/table_view.js`:

- Add the current row number to `view`, with `readView` and `writeView`.
- Mark the row in `renderTable` with a class and `aria-current="true"`, and
  scroll it into view. Add its style to `table_view.css`.
- Make a click set it, leaving the existing cell, button, and link actions as
  they are. Ignore a click that ends a text selection.
- Extend the `keydown` listener in `start`. Crossing pages goes through
  `loadRows`, whose sequence check already drops stale responses; the move is
  applied only after the requested page has arrived.

### Phase 2: Files follow

In `internal/webui/assets/table_view_panes.js`:

- Record the origin (source, row number, column) on the media passed to
  `openMedia` and `openMediaRow` from `mediaCell`, `fileCell`, and
  `openRowButton`. Leave `sameMedia` as it is.
- Add one function that replaces followed tabs in place, disposing each old
  `TabView`. `selectTab` already clears a pane's range when its tab changes;
  also reset `link.range` and `link.time`.
- Add the message tab for a row without the file, using the existing error
  styling. It must still follow later moves, so it keeps its origin.
- Add the delayed replacement described in Decisions.

### Phase 3: Play key and hint

- `Space` toggles the active pane's player. Linked panes go through the existing
  `playbackMoved`, so the others pause as they do for a click.
- Add the hint in `renderPager`.

### Phase 4: Documents and contracts

- Add the keys and the follow rule to "Viewing tables" in `docs/WEB.md`.
- Add contract WEB-30 (the next free ID at the time of writing) to
  `contracts/05-web-assets-and-static-export.md`, and its row to the status
  table in `contracts/README.md`.

## Tests and validation

- **Conformance:** a table test in `conformance/05-web/table_view_test.go`
  covering WEB-30 through the binary in jsdom: moving, following, the
  missing-file message, page crossing, and the focus guards. Other conformance
  tests in this package already use jsdom.
- **`internal/webui/table_view_test.go`:** the remaining variants above, in the
  style of `TestTableViewOpensFilesInPanesInBrowser`, with
  `table_view_static_test.go` for the snapshot. Check with fake timers or a
  counted fetch that held keys load only the final row.
- **Checks:** `go vet`, `tsc` for `internal/webui`, `go test ./internal/webui`,
  `go test ./conformance/...`, and `scripts/check.sh` once. `scripts/check.sh`,
  `pre-commit`, and the system Node may not work on the development machine;
  report what could not run and run its steps directly.
- **Headless Chrome in CI:** `conformance/05-web/page_policy_test.go` shows how
  a test runs in real Chrome; CI requires it, and it skips where Chrome is absent.
  Add the key and focus cases there, since jsdom does not model a browser's focus
  and key defaults faithfully.
- **Manual, required before calling this done:** jsdom has no canvas, Web Audio,
  or real media playback, and no browser is installed on the development
  machine. The user needs to try these in a real browser:
  - hold `↓` across a page boundary, with audio open in linked panes;
  - press `Space` and the arrows after clicking into an audio player;
  - listen for stale audio after a row change.

## Risks

- **Focus inside a player.** After a mouse click on a player, focus stays in it
  and the arrows belong to the browser's control. Returning focus to the table
  after a click on a player is deferred until this has been tried in a real
  browser, because its behavior differs by browser.
- **Lost pinned files.** A tab from the current row follows even if the user
  wanted to keep it. The workaround is to open the reference from another row.

## Remaining work

All phases. Add `work-log.md` after the first commit, as
[development/README.md](../README.md) describes.
