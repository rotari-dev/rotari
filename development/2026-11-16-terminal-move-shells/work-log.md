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
