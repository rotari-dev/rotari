# Work log: A state directory's shells beside its projects' in the Web terminal

## 2026-11-16 00:45 +09:00 — The shells a page shows

- **Change:** `web_shell.js`: `sharedGroup()` (a project's page's state directory's own group, none on other pages),
  `isShared`, `inView`, `sharedShells()`, and `shellsInView()` (the group's shells, then the shared ones). The tabs draw
  the group's tabs, then a `.terminal-tab-divider` (role presentation, "All projects", with " · " and the state
  directory when the sidebar lists more than one, its title naming it) and the shared tabs, whose label adds "of all
  projects". Choosing what a pane shows uses the view: opening the panel (so a project with no shells of its own shows
  the shared ones and starts none, decision 3, which phase 2 had listed), the remembered layout's candidates, split,
  a shell's end, the send buttons' fallback, and All shells (a shared card is chosen as a tab of this page, and marked
  current or in another pane). Arranging keeps to a tab's own group: the tab menu's moves and closes and a drag work
  among that group's tabs, a drag across the divider is refused, and the order sent to the server is that group's
  (the server already reorders the first id's group). **+**, the group limit, the project list, and counting still
  use the group alone.
- **Reason:** plan phase 1, after the user accepted the plan with the open questions as proposed.
- **Plan impact:** opening the panel starting none when shared shells exist landed here rather than in phase 2, since
  it follows from choosing among the shells in view.
- **Validation:** `TestTerminalPanel` (a project's page with its and the state directory's shells, another state
  directory's, no project's, and another project's: the order and divider, the label and title, choosing a shared tab,
  All shells choosing one without moving the page, the shared tab menu's moves and the server's order, a drag across the
  divider refused, the divider naming the state directory with two; a project with no shells showing the shared one and
  starting none, **+** starting the project's; All projects and Activity unchanged) passes; `tsc` and prettier clean.
- **Remaining:** phases 2 (keeping shared shells across a link) and 3 (contracts and docs).

## 2026-11-16 00:52 +09:00 — Keeping shared shells across a link

- **Change:** `web_shell.js`: `rememberLayout` also writes `rotari-terminal-last-panes`, the panes last shown (one
  pane too) with their group, handed to and from a window like the per-group keys. `carriedLayout`, read by `begin`
  before the group's own remembered layout, applies when that record is another group's and holds a state directory's
  own shell still in view here: those rows keep their shells and places; of the other rows, the one that had focus (or
  the first) shows this group's last shown shell, else its first, else any free one in view, and the rest go; the focus
  stays on a carried shell that had it, else goes to the filled row. `readLayout` is the parsing `rememberedLayout`
  had. `preferShell` (All shells choosing another group's shell) removes the record, here and in the opener, so the
  chosen shell is shown in that group's layout.
- **Reason:** plan phase 2.
- **Plan impact:** with the link on, following a link loads the new page (the plan had it as `switchGroup` in place),
  so "keeps" is the new page showing the shared shells again in their places, connecting again as every pane does on
  a page load; decision 2 and Existing implementation reworded. Because `switchGroup` ends in the same `begin`, a
  group chosen with the link off, and turning the link on, carry the shared panes the same way; with the link off a
  page of another project leaves the terminal as it was, as before.
- **Validation:** `TestTerminalPanel`: p1 with its shell and the shared one focused, then p2 (both panes, the shared
  one focused, nothing started), back to p1 (p1's shell again), p2 with the focus on p1's shell (p2's takes the
  focus), a project with no shells (the shared pane alone, nothing started), another state directory's page and
  Activity (their own shells), the link off (unchanged), and an All shells choice clearing the record; the window
  handover test now expects the record among the saved entries. `tsc` and prettier clean.
- **Remaining:** phase 3 (contracts and docs).

## 2026-11-16 01:02 +09:00 — Contracts and docs

- **Change:** WEB-56 in `contracts/05-web-assets-and-static-export.md` (the shared tabs and divider, what they share
  with the project's tabs and what keeps to the project's own, starting none, carrying the panes to a page of another
  group, an All shells choice, the link off) and its row in `contracts/README.md`; WEB-34's panel paragraph says a
  project's page shows its basedir's own shells after its own, and WEB-54 points to WEB-56 for following a link.
  `TestTerminalSharedShellsInBrowser` (new, `conformance/05-web/terminal_shared_shells_browser_test.go`): shells of p1,
  p2, and the basedir's own started through the API; p1's page shows its tab, the divider, and the shared tab; a split
  shows the shared shell with the keys, which prints a marker; following a link to p2 shows p2's shell and the shared
  one (its output replayed) with the keys still on it; back on p1, p1's shell beside it. docs/WEB.md: a paragraph on
  the shared tabs after the grouping one.
- **Reason:** plan phase 3.
- **Plan impact:** the browser step is a test of its own, in the manner of `TestTerminalProjectLinkInBrowser`, rather
  than a step in `TestTerminalPanelInBrowser`, which another thread is also changing; it covers WEB-56 alone.
- **Validation:** `go vet ./...`; `go test ./internal/webui ./conformance/05-web` (Node 24) and `go test ./conformance
  -run 'TestContractStatus|TestConformanceLayout'` pass. The browser test skips here (no Chrome) and runs in CI; its
  script passes `node --check`. `scripts/check.sh` was not run: npm is broken here, and the full and race runs are too
  heavy for the shared machine.
- **Remaining:** CI's run of `TestTerminalSharedShellsInBrowser`.

## 2026-11-16 01:19 +09:00 — The divider says only All projects

- **Change:** the divider before the shared tabs says **All projects** whatever the number of state directories; the
  state directory's path, which the user found too long beside the tabs (a `.rotari-state` under a deep workspace), is
  only in its tooltip. `sharedTabsLabel` goes. WEB-56, docs/WEB.md, and plan decision 1 say so.
- **Reason:** the user's request after seeing "All projects · /data/…/.rotari-state".
- **Plan impact:** decision 1's mark changed.
- **Validation:** `TestTerminalPanel` (two state directories: the divider says All projects, its title names /b);
  `tsc` and prettier clean.
- **Remaining:** none.

## 2026-11-16 01:31 +09:00 — A + on each side of the divider

- **Change:** the bar is now the project's tabs, its **+**, then `.terminal-shared`: the **All projects** divider, a
  `.terminal-shared-tabs` tablist ("Shells of all projects") of the state directory's own tabs, and
  `.terminal-shared-add`, a **+** that starts another of them (`newSharedShell`: the place of the state directory's
  All projects page from `sharedPlace`, retried the same way) and shows it in the pane with the keys, off at that
  group's limit. The part is shown on every project's page, with no shared shell too, and hidden elsewhere; the
  divider is no longer inside a tablist. Drag marks clear in both lists. WEB-56, its row, and docs/WEB.md say so.
- **Reason:** the user asked for **+** left of the divider and another after the shared tabs, so that both kinds can
  be started from a project's page.
- **Plan impact:** decision 3 ("**+** starts a shell of the project"; **New shell in** for the state directory's) gains
  the shared **+**, and the divider shows with no shared shells.
- **Validation:** `TestTerminalPanel` (the bar's order; the shared + starting `basedir=` and showing it, + starting
  the project's; the shared + off at its limit with the project's on; All projects and Activity hiding the part) passes;
  `TestTerminalSharedShellsInBrowser` checks the order and the shared + (CI only; `node --check` here). `tsc` and
  prettier clean.
- **Remaining:** CI's browser run.

## 2026-11-16 01:57 +09:00 — TestTerminalPanelInBrowser after WEB-56

- **Change:** `TestTerminalPanelInBrowser` opens the panel on All projects, so its first shells are the state
  directory's own; on `/project/p1` WEB-56 shows them after the project's tabs and starts none, where the test waited
  for **Start a shell** (CI: "timed out waiting for the project page"). The project step now waits for the two shared
  tabs and a shown shell, finds no tab of the project's, and starts the project's shell with its **+**; the later tab
  counts count the project's tablist (`.terminal-tabs`); the split step, whose new pane now shows a shared shell
  in no pane rather than starting one, waits for both panes to show a shell.
- **Reason:** phase 3 should have adapted this test with WEB-56 and did not; the browser tests run only in CI.
- **Plan impact:** none.
- **Validation:** the script passes `node --check`; no Chrome here, so CI runs it.
- **Remaining:** CI's run.
