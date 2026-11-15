# Plan: A state directory's shells beside its projects' in the Web terminal

Created: 2026-11-16
Status: Accepted by the user on 2026-11-16, with the open questions answered as proposed; all three phases are done. See
[work-log.md](work-log.md).

## Goal

With `--allow-terminal`, the shells of a state directory's own (those started on its **All projects** or **Recent
jobs**, for work across its projects) stay at hand on every page of that state directory: a project's page shows its
project's tabs and, after them, the state directory's shared tabs, and following a link from project to project keeps
a shared shell that a pane shows. Today a project's page shows only its project's shells, so opening a project to look
at a run, with the terminal and project linked (the default), takes the shared shell away; turning the link off keeps
it, but then the project's own shells no longer follow the project either.

Proposed by the user on 2026-11-16, after finding that shells on **All projects** were of little use once a project's
page replaced them.

## Existing implementation

- **Groups** (WEB-34). Each shell belongs to a group: a project of a state directory, a state directory's own pages, or
  no project. `here` is the terminal's group; `sameGroup` and `shellsHere()` (used for the tabs, the panes, the
  project list, counting, and choosing what to show) take only the shells of exactly that group.
- **The link** (WEB-54, by another thread). With **Link terminal and project** on, the terminal's group is the page's:
  following a link loads the other page, whose panel begins with that group's remembered layout and shown shell
  (session keys by group). Choosing a group with the link off, or turning it on, switches in place (`switchGroup`:
  `resetGroupPanes` closes all panes but one and disconnects it, then the same begin). Off, a page keeps the terminal's
  group.
- **Starting.** Opening the panel starts a shell if the group has none; following a link starts nothing, and the panel
  offers to start one (docs/WEB.md).
- **All shells** groups cards by project, the terminal's first; choosing a card of another group takes the page there,
  or, with the link off, switches the terminal.

## Decisions

1. **What a page shows.** A page of a project of state directory B shows the shells of that project and, after them,
   B's own shells. The tabs of the two kinds are set apart by a divider, and B's are marked as **All projects** (with
   B's name when more than one state directory is registered). A page of B's own (**All projects**, **Recent jobs**)
   and a page of no project show their own group alone, as now. The terminal's group (`here`) keeps its meaning: what
   **+**, the send buttons' fallback, and a new shell's directory use, and what the link follows.
2. **Following a link keeps a shared shell.** Moving between pages of B's projects with the link on still switches the
   terminal's group, but a pane showing one of B's own shells shows it again, in its place in the layout (it connects
   again, as every pane does on a new page); of the panes showing the old project's shells, the focused one (or the
   first) shows the new project's last shown or first shell, and the rest go. When the focused pane shows a shared
   shell, it stays focused. The layout remembered for a project holds its shared shells too, so coming back finds them
   where they were. Choosing a shell of another project in All shells shows that shell in that project's own layout.
3. **Starting.** Opening the panel on a project's page with no shells of its own but some of B's shows B's, and starts
   none (see open question 1). **+** starts a shell of the project, as now; **New shell in** under All shells starts B's.
4. **Counting and lists.** The project list beside the tabs counts each group's own shells, and lists B's own as its
   own entry as now; the Terminal button's count and All shells are unchanged. The bell and unseen dot work on shared
   tabs as on any.
5. **Server.** No change: the list already has every shell with its group.
6. **Contract.** WEB-34's grouping says what a project's page shows, WEB-54 what following a link keeps; a new WEB-56
   holds the rule.

## Phases

1. **The shells a page shows.** A `shellsInView()` beside `shellsHere()`, used where the tabs, the panes, choosing a
   shell, and restoring a layout ask "which shells are here", and the tabs' divider and mark. JSDOM tests: a project's
   page with its shells and B's, the order and mark, a basedir page and a no-project page unchanged, two state
   directories.
2. **Keeping shared shells across a link.** `switchGroup`/`resetGroupPanes` keeping panes that show B's shells (and the
   focus), the layout remembered per group with them, opening the panel starting none when B's exist. JSDOM tests:
   project A to project B of one state directory with a shared shell in a pane and focused; to a project of another
   state directory (the shared shell goes, as it is not that one's); back to A; the link off unchanged.
3. **Contracts and docs.** WEB-56 and the WEB-34/WEB-54 wording; a `TestTerminalPanelInBrowser` step (a shared shell
   started on All projects, a project's page showing it after its own tabs, and following a link to another project
   keeping it in the pane); docs/WEB.md.

## Tests and validation

- JSDOM: as in phases 1 and 2, in `TestTerminalPanel`.
- Browser (CI only): the step in phase 3.
- `go test ./internal/webui ./conformance/05-web` with Node 24, `tsc`, and prettier.

## Non-goals

- Showing shells of no project, or of other projects, on a project's page.
- A shell belonging to several projects, or moving a shell between groups.
- Changing what **All projects** or **Recent jobs** show.

## Settled questions

All three were accepted as proposed on 2026-11-16.


1. **Opening the panel on a project with no shells of its own.** Proposed: show the state directory's shells and start
   none, since one is there to use; the panel's **Start a shell** still makes the project's. The alternative is to
   start the project's as now and show the shared ones beside it.
2. **Where the shared tabs go.** Proposed: after the project's, behind a divider. The alternative is before them.
3. **Which panes a link keeps.** Proposed: every pane showing a shared shell, and the focus with it when it is on one.
   The alternative is to keep only the focused pane's shared shell and reset the rest.

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace and is rebased onto `main` before integrating. The
link (WEB-54) and the browser test are also being changed by another thread; this work rebases onto theirs as it lands.
