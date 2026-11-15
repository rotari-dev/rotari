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
