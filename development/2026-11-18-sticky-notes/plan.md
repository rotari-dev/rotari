# Sticky notes (2026-11-18)

## Purpose

Let users stick free-form reminders ("sticky notes") on any Web UI page, and
read them from the CLI. Stickies are working memory for a person looking at a
page, not a record of an experiment.

## Decisions

* **Stored on the server**, in the basedir, so every browser and the CLI see
  the same notes. Single file `<basedir>/stickies.json`, guarded by a lock
  file, written atomically through `state.Store`.
* **Notes belong to a whole page.** The page key is the Web UI route without
  the `/_basedir/<id>` prefix (`pageParts().join("/")`, `/` for home). It is
  an arbitrary string, never used as a path.
* **Independent of run notes** (`rotari note`, `queueops.AddNote`). Run notes
  are part of a run's history; stickies are not tied to any job, run, or
  result and never affect status, selection, or scheduling. Revisit the
  relation later (for example promoting a sticky to a run note).
* **Read-only web servers** (no `--allow-control`) can read but not change
  stickies, like every other state-changing API.
* **Static exports** carry no stickies and hide the controls.

## Non-goals

* Anchoring a note to a row or element on the page.
* Merging concurrent edits of one page: the last save of a page wins.
* Python client and MCP support (CLI only for now).

## Implementation

1. `internal/stickies` (imports only `internal/state`): `Note`, `Validate`,
   `Load`, `Save`, plus `Pages` to list every page that has notes for the CLI.
   Limits: 100 notes per page, 1000 pages, 4000 characters, fixed colour set,
   non-negative bounded coordinates. Add to the archtest rules and to the
   package table and diagram in `docs/ARCHITECTURE.md`.
2. Web API `GET/POST /api/stickies` (`?page=` / `{page, notes}`); POST needs
   `allowControl`.
3. Web UI (`web_app_stickies.js`, top-bar buttons `+ Sticky` and
   `Show/Hide stickies`, styles in `web_styles.css`): draggable coloured
   notes in a fixed layer, debounced save, reload on client-side navigation,
   save errors shown on the note.
4. CLI `rotari stickies [--page PAGE] [--json]`: lists notes of all pages or
   one page, straight from `internal/stickies` (not through HTTP). Read-only.
5. Contracts: new rules in `contracts/05-web-assets-and-static-export.md` and
   `contracts/03-server-and-command-interfaces.md`, status table in
   `contracts/README.md`, conformance tests (Web API round trip, read-only
   403, invalid input 400, CLI listing). User docs for the Web UI and CLI.

## Tests and validation

* Unit: `internal/stickies`, archtest, web handler (round trip, read-only,
  validation), Web UI script test with jsdom like the existing ones, CLI.
* `npm run typecheck`, `go test ./conformance`, `scripts/check.sh`.

## Follow-up: one Stickies menu (2026-11-18)

The top bar is full, so the two sticky buttons become one **Stickies** menu
(a dropdown like Settings) holding:

* **+ New sticky** and **Hide / Show stickies on this page**. "Close all on
  this page" means hide, not delete: nothing is lost and Show brings them
  back (the existing per-browser setting).
* **All stickies**: every sticky of every page, newest first, as creation
  time plus the first characters of its text and its page; choosing one goes
  to that page.

Changes: a note gets `created_at` (RFC 3339, set by the browser, filled by
the server when missing, kept on later saves); `GET /api/stickies` without
`page` returns `{"pages": [{page, notes}]}`; `rotari stickies` prints the
creation time. WEB-62 and its conformance test cover both.

Later the same day: the menu's list rows got an `×` that deletes the note
(at once in the open list, with the save in the background), and notes can be
resized by a corner grip (`width` and `height`, 0 for the default 220 by 160,
120 to 2000 by 80 to 2000).

Then: the Recent jobs page (`/jobs/`, server-rendered, no application script)
gets the top-bar controls it lacked, as the user asked: **Sign out**, the
Stickies menu, and **Launches**. The menu's markup is one Go constant shared
by both templates; `jobs_page_controls.js` gives that page the helpers
`web_app_stickies.js` expects, and its Launches button opens the application
page with `?launches=1`. The sticky styles moved to `web_shell.css`, which
both pages load.

Then: the sidebar marks pages that have notes (asked for after a discussion of
options): a dot in the colour of the first note, titled with its first
characters, on the page's link, and a faint dot on the links of its parent
pages. Done in `markSidebarStickies` from the page keys of `GET
/api/stickies`, redrawn after every sidebar draw; no per-row marks or counts
for now.

## Status

Implemented as planned (WEB-62). Not validated here: the Web UI script and
`npm run typecheck`, since the sandbox's Node 12 cannot run TypeScript or
jsdom; run `scripts/check.sh` and the browser tests where a current Node is
available.
