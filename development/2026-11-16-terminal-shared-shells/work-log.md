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
