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

## Sidebar marks for pages with notes

**Commits:** see the CI fix entry below.

**Change:** `markSidebarStickies` in `web_app_stickies.js` puts a dot on the
sidebar links of pages with notes and a faint one on their parents, from the
all-pages `GET /api/stickies`; `web_app_bootstrap.js` wraps `renderSidebar`
so every drawing keeps the marks. Styles in `web_shell.css`. Contract WEB-62,
`docs/WEB.md`, and the plan updated.

**Reason:** The user asked how to see which pages have notes from the
sidebar, then to try the dot, parent mark, and tooltip option.

**Plan impact:** Count badges and a "pages with notes" filter were left out.

**Validation:** Syntax check of the scripts and the colour-token, placeholder,
and sticky tests pass; the jsdom failures are the same 78 as before. Not
tried in a browser.

**Remaining:** Try it in a browser, including a collapsed project and a
basedir other than the current one.

## Fix the Settings menu and tests that CI caught

**Commits:** see the commit that adds this entry; its date, time, and ID are
in `jj log` and are not repeated here.

**Change:** The Stickies menu used the Settings menu's `settings-menu` and
`settings-panel` classes, so `web_shell.js` (which closes Settings on a click
outside the page's first `.settings-menu`) treated every Settings click as
outside and the menu never opened. It now has its own `stickies-menu` and
`stickies-panel` classes and styles. Updated the toolbar order in
`TestWebHTMLRendersState`, made `TestJobsRangeBarInBrowser` ignore the top
bar's launch and sticky fetches, and added
`TestRecentJobsPageMarksStickiesAndKeepsSettingsMenuWorking`.

**Reason:** CI failed `TestWebShellMenuAndDrawer`, `TestWebHTMLRendersState`,
`TestJobsRangeBarInBrowser`, and `TestTerminalPanelInBrowser` (its Settings
path buttons were hidden because the menu never opened).

**Plan impact:** None. The earlier entries said the jsdom failures matched the
parent commit; that comparison was meaningless, because with Node 12 every
jsdom test failed on both. The jsdom tests ran for real only with Node 22.20.

**Validation:** With Node 22.20, the new test fails with the old class
(`Settings does not open beside Stickies`) and passes with the fix; `go vet`,
`go test ./internal/... ./cmd/...`, and `go test ./conformance/...` pass except
`TestWebArtifactPreviewInBrowser` and `TestStaticExportWithArtifactContents`,
which also fail on `9bd33582` (recorded in `ISSUES.md`).

**Remaining:** Try the pages in a real browser; the two artifact tests.

## Align two artifact tests with the listing's new order

**Commits:** see the commit that adds this entry (`jj log`).

**Change:** `TestWebArtifactPreviewInBrowser` now expects the directory
columns Launch, Download, and `TestStaticExportWithArtifactContents` the
working-directory row `directory`, `working directory`, `.`.

**Reason:** CI failed these two after the artifact listing was reordered; they
were missed then, and also failed before the sticky notes. Not a sticky-note
change; fixed here to get CI green, and its `ISSUES.md` entry removed.

**Plan impact:** None.

**Validation:** Both pass with Node 22.20; the whole `internal/webui` package
run below.

**Remaining:** None.

## Sonar quality gate: annotate the Explorer launch path checks

**Commits:** see the commit that adds this entry (`jj log`).

**Change:** `explorerLaunchStart` in `internal/webui/launches.go` now carries
the `NOSONAR` reason that `listTerminalDir` already uses on its two
filesystem calls (`EvalSymlinks` and `os.Stat` of the Explorer path).

**Reason:** The Sonar quality gate failed on `new_security_rating` for two
`gosecurity:S6549` findings on those lines, added by the Explorer launch
commit, not by the sticky notes. The path is user input by design, checked by
`checkTerminalDir` and the terminal's connection rules, so the findings are
not exploitable; restructuring cannot satisfy a taint rule for a path the
feature has to accept.

**Plan impact:** None.

**Validation:** `go build`, `go vet ./internal/webui`, and the Launch and
Explorer tests pass. Whether the quality gate turns green is known only after
the next SonarCloud run.

**Remaining:** Check the gate after the push.

## Format the generated Python CLI metadata

**Commits:** see the commit that adds this entry (`jj log`).

**Change:** `python/rotari/generated_cli.py` is now the generator's output run
through `ruff check --fix` and `ruff format`, as the Python workflow's
"Check generated Python CLI metadata" step does.

**Reason:** The Python workflow's `ruff format --check` failed on the file.
I had regenerated it for the new `stickies` command and copied the
generator's raw output, without the ruff step. The file in `9bd33582` was
also unformatted (the commit "Update getting started and generated CLI"
regenerated it raw), so `main` was already failing this check; it only
showed once the Python workflow ran on a push (path filter `python/**`).

**Plan impact:** None. The tag `v0.1.31` points at the commit before this fix,
so its Python workflow run stays red; the shipped binaries are not affected.

**Validation:** With ruff 0.16.9 (CI uses 0.17.1, which could not be
installed here): `ruff format --check` and `ruff check` with
`python/pyproject.toml` on `python scripts` pass, the formatted generator
output equals the committed file, `generate_cli_reference.py --check` and
`generate_python_api_docs.py --check` pass, and the module imports with 39
commands. `pytest` is not installed here, so the Python tests did not run.

**Remaining:** Confirm the Python workflow on the pushed commit, since its
ruff is newer.
