# Plan: Moving a shell to another project from All shells

Created: 2026-11-16
Status: Accepted by the user on 2026-11-16 ("go"), with the open questions as proposed. See [work-log.md](work-log.md).

## Goal

With `--allow-terminal`, dragging a shell's card in All shells onto another group moves the shell there: it becomes a
tab of that project (or of a basedir's own shells), and the shell itself takes the new place, so that `rotari`
commands typed in it act on the project its tab says. The user chose this on 2026-11-16 over moving only the tab
(which would leave `ROTARI_PROJECT_NAME` naming the old project) and over opening a new shell in the target.

## Existing implementation

- **Groups.** A shell's group (`terminalGroup`: basedir and project, a basedir alone, or empty for no project) is set
  at start (`terminal.StartOptions.Group`) and never changes. Its `Number` is the lowest free in that group, the
  group limit counts it there, `Reorder` arranges within it, and the directory history (WEB-45) is the group's.
- **Environment.** `serveTerminalShells` starts a project's shell with `ROTARI_BASEDIR` and `ROTARI_PROJECT_NAME`, a
  basedir's own with `ROTARI_BASEDIR` and `ROTARI_PROJECT_NAME` unset, and a shell of no project with neither set
  by rotari. A running process's environment cannot be changed from outside.
- **Shell integration** (WEB-49). With `--terminal-shell-integration` (the default), rotari's bash and zsh read
  `marks.bash` / `marks.zsh`, whose prompt hook runs before each prompt; inner bash and zsh load it too.
- **All shells** (WEB-35 and others). Cards are grouped by basedir and project, with headings showing the basedir
  over the project; the tab bar already drags tabs within a group (`makeDraggable`).

## Decisions

1. **Where a shell can go.** A shell of a project or of a basedir's own can move to any other project of any basedir
   listed in All shells, or to a basedir's own shells. Shells of no project neither move nor take moves (see open
   question 1): rotari sets nothing in their environment that a move could replace.
2. **What a move changes on the server.** `POST /_terminal/shells/ID/place` with `{"basedir", "project"}` (project
   empty for a basedir's own). The server checks the place as a page's place is checked (the basedir known to it, the
   project existing there), then `Keeper.Move`: the shell joins the new group at its end with the lowest free number
   there, keeping its name; the group limit there applies (409 as for starting). Its future directories go to the new
   group's history; past ones stay where they were. An unknown shell is 404, a bad place 400.
3. **What a move changes in the shell.** The Keeper gives each shell it starts with the integration a place file of
   its own, in a directory only the user can read and write, named by `ROTARI_TERMINAL_PLACE` in its environment. A
   move writes the new basedir and project there with a generation number. The prompt hook, before each prompt, reads
   the file (no fork: `read` in both shells) and, when its generation is newer than the one it last applied,
   exports `ROTARI_BASEDIR` and sets or unsets `ROTARI_PROJECT_NAME`. It applies each move once, so a value the user
   sets by hand afterwards is kept. Values are assigned, never evaluated.
4. **When it takes effect.** Before the next command typed at the prompt the shell waits at (zsh's `preexec`, bash's
   DEBUG trap once per command line, since bash's PS0 runs in a subshell), and at the next prompt. A command already
   running (vim, a long job, ssh) keeps the old values, as do its children, and the outer shell takes the new ones when
   it returns to its prompt; a bash with a DEBUG trap of the user's takes a move only at its next prompt; a bash or zsh
   started inside also applies it at its own next prompt, since it inherits `ROTARI_TERMINAL_PLACE`. The move itself
   is not refused while a command runs; the note after the move says when it takes effect.
5. **Shells that cannot follow.** A shell started without rotari's integration (fish, other shells, or
   `--terminal-shell-integration=false`) would keep the old values for good, so the server refuses to move it (409,
   saying why), and its card is not draggable, with a title saying why.
6. **All shells.** A card of a movable shell is draggable; each other group's section (heading and cards), except no
   project, takes the drop and shows that it would. Dropping moves the shell and redraws the list. A group at its limit
   shows that it does not take the drop. A keyboard way: the card's menu (or a button) gets **Move to...**, listing
   the same groups.
7. **The page after a move.** The shell leaves this page's tabs if the new group is not in view here, and a pane
   showing it shows another shell of the view, as when a shell ends; a note says "Moved shell N to P (takes effect at
   its next prompt)". Another open page sees the shell under its new group at its next refresh. (See open question 2.)
8. **Contracts.** A new ID, WEB-57, for the move (route, rules, environment, refusals, All shells); WEB-49 says the
   integration also applies a place file.

## Phases

1. **Keeper and integration.** `Keeper.Move`, the place file and `ROTARI_TERMINAL_PLACE`, and the prompt hooks in
   `marks.bash` and `marks.zsh`. Tests: `Move` (number, order, limit, history), and a real bash and zsh whose
   `ROTARI_PROJECT_NAME` changes at the next prompt after a move, is unset for a basedir's own, survives a hand-set
   value after the move, and reaches an inner bash.
2. **Route.** `POST /_terminal/shells/ID/place`: the place checks, the refusals (unknown shell, bad place, group
   limit, no integration), and the shell list showing the new group. Tests in `internal/webui`.
3. **All shells.** Drag and drop, the drop targets, **Move to...**, the redraw and note, and a pane showing the moved
   shell. JSDOM tests in `TestTerminalPanel`.
4. **Contracts and docs.** WEB-57 and WEB-49, conformance through the binary (a shell moved through the route, its
   environment after the next prompt), a browser test (CI only: drag a card onto another project), docs/WEB.md, and
   docs/SECURITY.md (the place file).

## Tests and validation

- `go test ./internal/terminal ./internal/webui ./conformance/05-web` with Node 24, `tsc`, and prettier.
- The browser test runs only in CI (no local Chrome).

## Non-goals

- Moving shells by dragging tabs in the tab bar between the project's tabs and the shared ones.
- Changing the environment of commands already running.
- Moving shells of no project (decision 1, unless open question 1 says otherwise).
- Moving a shell between servers.

## Open questions

1. **Shells of no project.** Proposed: they neither move nor take moves. The alternative lets them move into a
   project (setting both variables) and back (unsetting both), at the cost of losing a `ROTARI_BASEDIR` the server
   itself was started with.
2. **The page after a move.** Proposed: the shell leaves this page if its new group is not in view (decision 7). The
   alternative, with the project link on, takes the page to the new project's page with the shell shown, as choosing
   another project's card in All shells does.

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace.
