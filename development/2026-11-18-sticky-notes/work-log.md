# Work History: Sticky notes

See [plan.md](plan.md) for current scope and status.

## Sticky notes on Web UI pages, listed by the CLI

**Commits:** 2026-11-18 18:25:26 `aaec2a17` (one commit for all the entries
below, which were made before anything was committed).

**Change:** New `internal/stickies` (`<basedir>/stickies.json`, `Load`, `Save`,
`Pages`, limits) with an archtest rule and an `ARCHITECTURE.md` row. Web API
`GET/POST /api/stickies` in `internal/webui/webui.go`. Web UI script
`web_app_stickies.js` with its styles and colour tokens, and a **Stickies**
top-bar menu. `rotari stickies [--page] [--json]` in `cmd/rotari/stickies.go`,
with the regenerated CLI reference, `python/rotari/generated_cli.py`, and
conformance goldens. Contract WEB-62, `docs/WEB.md`, and the conformance test
`TestWebStickiesAreSharedWithTheCLI`.

**Reason:** The user asked for sticky notes on each Web UI page; decided on
server storage, whole-page scope, CLI listing, and independence from run
notes.

**Plan impact:** Plan written before the work, as the user asked. Open: a
relation to run notes (for example promoting a sticky), left independent for
now.

**Validation:** `go test` of `internal/stickies`, the `webui` sticky and
colour-token tests, `conformance/05-web` `Stickies`, `conformance` (goldens),
the CLI flag-pair tests, `internal/archtest`, and `internal/doclinks` passed;
`go vet ./...` passed; the generated CLI reference and README sync checks
passed. The remaining failures in `internal/webui`, `cmd/rotari`, and
`conformance/05-web` (78 in the first two) are jsdom tests that this
machine's Node 12 cannot run, and the same 78 fail on the parent commit.
`npm run typecheck`, prettier, and `scripts/check.sh` did not run, for the
same reason. The script was only syntax-checked, and nothing was tried in a
browser.

**Remaining:** Check the Web UI in a browser and run `scripts/check.sh` with a
current Node; add a jsdom test for the Stickies menu.

## Menu list, delete, per-page hide, and resize

**Commits:** 2026-11-18 18:25:26 `aaec2a17`, as above.

**Change:** One **Stickies** menu replaced two top-bar buttons: **+ New
sticky**, **Hide / Show on this page** (per page, per browser), and a list of
all notes (newest first, time and first 24 characters, going to the page)
with an `×` that deletes. Notes gained `created_at` (filled by the server when
missing), `GET /api/stickies` without `page` lists all pages, and notes gained
`width` and `height` with a corner grip. `rotari stickies` prints creation
times. Contract, docs, and plan updated.

**Reason:** The top bar had no room for more buttons, and the user asked for a
list, closing a page's notes, deleting from the list, and resizing.

**Plan impact:** "Close all on this page" was read as hide, not delete.

**Validation:** As above for the Go side and conformance; the new limits and
`created_at` have unit tests. The list and resize behaviour was not tried in
a browser.

**Remaining:** Same as above.

## Top-bar controls on the Recent jobs page

**Commits:** 2026-11-18 18:25:26 `aaec2a17`, as above.

**Change:** `jobs_template.html` now has Sign out, the Stickies menu (one
shared `stickiesMenuHTML`), and a Launches button, live page only.
`jobs_page_controls.js` supplies the helpers the sticky script needs and the
Launches button; `web_app_bootstrap.js` opens the Launches view for
`?launches=1`. Sticky styles moved from `web_styles.css` to `web_shell.css`.
Contract WEB-62 and `TestRecentJobsPageHasTheSharedTopBarControls`. The
`ISSUES.md` entry about the missing controls was removed as fixed.

**Reason:** The user noticed the page lacked Sign out and Stickies and asked
for them, and for Launches too.

**Plan impact:** Launches opens the application page instead of porting its
dialog, which depends on the application script.

**Validation:** The new test, the colour-token and placeholder tests, and
`doclinks` pass; the `internal/webui` and `cmd/rotari` failures are the same
78 jsdom tests as on the parent commit. The scripts were only syntax-checked
and nothing was tried in a browser.

**Remaining:** Try the page in a browser; the page's Launches count and the
deep link have no jsdom test.
