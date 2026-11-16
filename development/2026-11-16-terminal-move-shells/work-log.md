# Work log: Moving a shell to another project from All shells

## 2026-11-16 16:54 +09:00 — Keeper and integration

- **Change:** `Keeper.Move(id, group, set, unset)` puts a running shell after its new group's shells with the
  lowest number free there, its name kept, within that group's limit (`ErrTooManyInGroup`); its own group is a
  no-op, an unknown shell `ErrNoShell`, a shell without the integration `ErrCannotMove`, and a variable not
  `ROTARI_` or a value with a line break or NUL an error, all before anything changes. With the integration the
  Keeper makes a private `rotari-terminal-places-*` directory (removed on Close) and gives each bash and zsh it starts
  `ROTARI_TERMINAL_PLACE`, its own file there, dropping one inherited from a shell the server runs in; `ShellInfo`
  says `FollowsMoves`. A move writes "rotari-place N" and set/unset lines to that file by rename (removed when the
  shell ends). `marks.bash` and `marks.zsh` get `__rotari_place`, which reads the file with `read` and, for an N
  above `ROTARI_TERMINAL_PLACE_APPLIED`, exports or unsets only `ROTARI_` names, assigning values as they are, and
  records N. It runs at each prompt and before the first command after one: zsh's preexec, and bash's DEBUG trap,
  now installed also on bash 4.4 and later when the shell follows moves and the user has no DEBUG trap (the old
  bash's C moved into the same trap).
- **Reason:** plan phase 1, after the user said "go".
- **Plan impact:** decision 4: a move made while the shell waits at its prompt is taken before the next command, not
  only at the next prompt, since otherwise the first command after a move would still act on the old project; found
  while writing the tests. Decision 3 says `read` rather than `$(<file)`.
- **Validation:** `TestMoveTakesAShellToAnotherGroup` (number, order, name, own group, limit, unknown, bad
  variables refused without moving, history kept by the old group, sh refused), `TestShellsDoNotInheritAPlaceFile`,
  and `TestMovedShellTakesItsPlaceVariables` (real bash and zsh: a move at the prompt seen by the next command, a value
  with `$(...)` and backquotes not evaluated, a basedir's own unsetting the project, a hand-set value kept, a move
  during `sleep` seen after it, an inner shell inheriting the applied move and taking the next, the outer shell after
  it) pass; with the before-command hooks taken out, the at-prompt checks fail. `go test ./internal/terminal` passes.
- **Remaining:** phases 2 (route), 3 (All shells), and 4 (contracts and docs).

## 2026-11-16 16:59 +09:00 — Route

- **Change:** `POST /_terminal/shells/ID/place` in `internal/webui/terminal.go` (`serveTerminalPlace`): the place is
  named in the query as a new shell's is (`basedir` ID, `project`) and read by the same `terminalPlaceFor`; a run,
  a directory, or neither name is 400, a project that `resolve.ProjectExists` does not find 404, as is an unknown
  basedir or shell; a shell of no project 409; then `Keeper.Move` with `ROTARI_BASEDIR` and `ROTARI_PROJECT_NAME`
  (unset for a basedir's own), mapping `ErrCannotMove` and the group limit to 409 and a closed keeper to 503. The
  answer is the shell as listed. The shell list gains `movable` (of a basedir and following moves).
- **Reason:** plan phase 2.
- **Plan impact:** the place is named in the query, as `POST /_terminal/shells` names it, rather than in a JSON body,
  so both share one check.
- **Validation:** `TestTerminalMovesAShell` (a real bash server: to another project, to the basedir's own, each
  refusal, a full project, its own, a GET), `TestTerminalDoesNotMoveAShellWithoutTheIntegration`, and the route added
  to `TestTerminalShellRoutesAreGuarded` pass; `go test ./internal/webui -skip 'TestTerminalPanel$'` passes.
- **Remaining:** phases 3 (All shells) and 4 (contracts and docs).

## 2026-11-16 17:08 +09:00 — All shells

- **Change:** `web_shell.js`: each card gets a **Move** icon (`moveShell`) between Launch and Rename, whose menu lists
  where the shell can go: the groups listed with shells (`overviewGroups`, "(full)" and disabled at the group
  limit) and the projects and basedirs' own with none yet (`overviewNewGroups`, from `drawOverviewNew`'s lists),
  never no project; a shell of no project or one not `movable` has it disabled with a title saying why. A movable
  card's item is draggable; another group's section with a basedir and room takes it (`terminal-overview-drop`
  outline), and a drop or a menu choice calls `moveToGroup` (`POST …/place?` + `placeOfPath` of the group's link),
  then refreshes the list, redraws the overview with the keys on the card, and notes "Moved shell N of P to Q. Its next
  command uses Q; a command running now keeps the project it had."; a refusal is said at the overview's top.
  `refreshShells` calls `followMovedShells` with the list before: a pane showing a shell whose group changed out of
  this page's view shows the next free shell of the view, or closes when other panes remain, or else drops the
  connection and says "Shell N of P moved to Q." with **New shell**, so a move from another page is followed too.
  CSS: the dragged card dims, the drop target is outlined, a disabled icon dims, and the card's top leaves room for
  four icons.
- **Reason:** plan phase 3.
- **Plan impact:** decision 6's keyboard way is the Move icon's menu (a button among the card's icons, reached by Tab),
  not a separate card menu; messages name the shell's project, since numbers repeat across projects.
- **Validation:** `TestTerminalPanel` (Move enabled and labeled, disabled with reasons for no project and no
  integration, draggable only when movable; a drag refused by its own group, no project, and a full group, taken by
  another project and sent there; the pane showing the moved shell taking the next one; the note; the menu with a full
  group and a project with no shells, sent there; a refusal at the top; a move seen in the next list leaving the only
  pane with its message) passes; `tsc` and prettier clean.
- **Remaining:** phase 4 (contracts and docs).
