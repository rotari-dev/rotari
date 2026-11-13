# Work log: An Explorer in the Web terminal

## 2026-11-13 22:40 +09:00 — `68f98f00` Add an Explorer of the shell's files to the Web terminal

**Change:**

- `internal/webui/terminal_files.go` (new): `GET /_terminal/files`, dispatched from `serveTerminal`, so it sits
  behind the terminal's token, `Host`/`Origin`, and HTTPS-or-loopback checks and is 404 without the terminal.
  `?shell=ID` lists the directory the shell is in now (`Keeper.Dir`; 404 for a shell not running, 422 when the
  directory is unknown or not valid UTF-8); `?path=/ABS` lists any directory (`checkTerminalDir`: one of the two,
  absolute, valid UTF-8, no NUL). Entries are sorted folders first (including links to folders), then by name
  ignoring case; each has its kind, and a link its target and the kind of what it reaches (`missing` if nothing). A
  name or target that is not UTF-8 is sent with U+FFFD and marked `not_utf8` / `target_not_utf8`. At most 2,000
  entries, with `total`. A directory that cannot be listed is a 422 with the reason.
- `internal/webui/table_view_shell.go`: `/api/tableview-sources` takes `path` as well as `shell`
  (`serveExplorerTableView`). It needs the terminal (403) and `allowsBrowserSession` (403), takes one of the two
  (400 for both), follows symbolic links with `EvalSymlinks`, and pins a folder, or a file's folder with the file
  returned as `child`, reading under `/` with the trail `Explorer`. The links are followed because `os.Root`
  refuses a link to an absolute path even under `/`, which the first version (a `dir` field) ran into in its test.
- `internal/webui/assets/web_shell.js`: the Explorer (the block from `explorerOpenKey` to `takeExplorerDrops`). It
  is a column in a new `.terminal-body` beside `.terminal-panes`, or a drawer with a scrim when the body is
  narrower than the Explorer plus 480 px, and its button is first in the bar. It follows the focused pane's shell
  until **Up** is pressed, with **Follow**, **Refresh**, and **Collapse all**. It lists again from `refreshShells`
  (the five-second poll), 400 ms after Enter in the focused pane, on `focusPane` and `show`, and on Refresh. Rows
  are a `role="tree"` with arrow keys, Enter, and Home/End; they show escapes for control characters, links with
  their target, and nothing to act on for `not_utf8`. Row buttons, the panel's menu, and drag to a pane send a path
  relative to the shell's directory when under it (`pathFromShell`) through `sendToTerminalAndSay`, open the
  Table viewer in a tab opened before the request (closed if refused), or copy. A drawer closes after a row is
  used. The open state and width are kept in `localStorage`.
- Quoting: `shellQuote` and `shellWord` are now global functions in `web_shell.js`, which every page and the window
  load before the application script. `asWord` in the panel and `shellQuote` in `web_app_logs.js` are gone.
  `'` and `\` go outside the single quotes as `\'` and `\\`. `web_globals.d.ts` declares `shellQuote` for the type
  check.
- `internal/webui/assets/web_shell.css`: the column, drawer, scrim, rows, escapes, actions, footer, resizer, and a
  pane's drop mark, all from tokens.
- Tests: `terminal_files_test.go` (new; listing, limit, refusals), `TestTableViewOnAnExplorerDirectory` and changed
  cases in `TestTableViewOnAShellsDirectory`: `{"shell","path"}` is now "not both", and an unknown field still
  "names only a running shell". The files route was added to `TestTerminalShellRoutesAreGuarded` and
  `TestTerminalIsAbsentWithoutTheOption`. `TestTerminalPanel` has a fake `/_terminal/files`, a backslash case and
  `shellQuote`/`shellWord` asserts beside the send buttons' quoting, and an Explorer section in a block of its own.
  Conformance: `conformance/05-web/terminal_explorer_test.go` (`TestTerminalExplorerListsAndOpensFiles`, WEB-42).
- Contracts and docs: new WEB-42 note and row; the WEB-39 note and row say a `path` source is the Explorer's; the
  note also records the quoting rule. `docs/WEB.md` (the Explorer, and the quoting of sent values),
  `docs/SECURITY.md` (the Explorer reads nothing a shell could not), and `docs/ARCHITECTURE.md` (the route and the
  files).

**Reason:** The user asked for a VS Code-like Explorer in the Web terminal and agreed the design on 2026-11-13: no
limit on going up, one quoting form for every shell without detecting it, the page may name a path with the
terminal on, and layout A from the mockup.

**Plan impact:** All three phases are done. Decision 5 changed from `dir` to `path` with links followed, as above.
Decision 10 (Open a terminal in, Launch in) is still open.

**Validation:**

- `go test ./internal/webui/ -run 'TestTerminalFiles|TestTerminalShellRoutesAreGuarded|TestTerminalIsAbsentWithoutTheOption|TestTableViewOn'`: pass.
- `go test ./internal/webui/ -run 'TestTerminalPanel$'` with Node v24: pass (69 s). A deliberately wrong last
  Explorer assertion failed as expected, showing that the section runs to its end.
- `go test ./conformance/05-web/ -run 'TestTerminalExplorerListsAndOpensFiles|TestTableViewerOnAShellsDirectory'`: pass.
- `tsc -p internal/webui/tsconfig.json`: pass. `npm` does not work on this machine, so `scripts/check.sh` was run
  as its steps: `go vet ./...`, `go test -timeout 15m ./...`, and `go test -race -timeout 15m ./...` (under
  `taskset -c 0-11`), all passed with go1.25.8. After that run the viewer tab's address was made absolute and
  `TestTerminalPanel` passed again.
- Prettier 3.9.8 (pre-commit pins v4.0.0-alpha.8) formatted the new code. Its two remaining differences in
  `web_shell.js` are in `confirmEndingShells` and are on `main` as well, so they were left alone.
- The quoting form was checked by hand with sh, dash, bash, zsh, and tcsh; fish is not installed here.
- Not run: a real browser (none here) and `TestTerminalPanelInBrowser`, which runs only in CI.

**Remaining:** Decision 10's menu items, if the user wants them. The column, the drawer, and dragging have not been
seen in a real browser.
