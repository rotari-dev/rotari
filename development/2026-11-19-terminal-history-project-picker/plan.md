# Terminal history project picker

Created 2026-11-19. Status: implemented, including other basedirs (server, panel, docs, contract WEB-63); the panel is not yet exercised by a browser test.

## Purpose

The Web terminal's **Recent directories** and **Commands** lists show only the
current project's entries. Add a project pulldown to each list so a user can read
another project's entries, with the current project selected when the list opens.

## Current behavior (verified in code)

- **Recent directories**
  - `GET <shell>/dirs` (`serveTerminalDirs`, [terminal_dirs.go](../../internal/webui/terminal_dirs.go))
    answers for the group of one running shell. The group is `basedir NUL project`
    (`terminalGroup`, [terminal.go](../../internal/webui/terminal.go)).
  - `history` is `Keeper.History(group)`: in memory, 20 entries, dropped when the
    group's shells end ([keeper.go](../../internal/terminal/keeper.go)).
  - `runs` is where the project's newest runs started; a basedir's own shells get
    `workspace` instead.
  - The page side is the Recent directories dropdown in
    [web_shell.js](../../internal/webui/assets/web_shell.js).
- **Commands**
  - `openCommands` lists `pane.blocks` of the focused pane only, built in the
    browser from shell marks. The server keeps no command history
    (only `LastCommand` per shell), so this list is per pane, not per project.

## Scope

1. Recent directories: a project pulldown; choosing one reloads `history` and
   `runs` for that project.
2. Commands: the same pulldown, listing that project's commands.

Non-goals: persisting history across server restarts; changing what a row does
(cd typing, Explorer, copy); reading projects of a basedir other than the current
shell's.

## Decisions and constraints

1. **Project list.** The projects of the current shell's basedir, plus the
   basedir's own group when relevant. The current project is the initial value
   every time the list opens; the choice is not remembered.
2. **Basedir.** Revised after review: the pulldown also offers the other
   registered basedirs (`basedirEntries`, by ID, as `terminalPlaceFor` resolves
   `?basedir=`), so the endpoint can read only basedirs the server already shows.
   Choosing a basedir starts at its own shells (`project=` empty). An unknown
   basedir or project is an explicit 404, never an empty list.
3. **Dirs endpoint.** Add an optional `project` query parameter to
   `<shell>/dirs`. Absent means the shell's own group (today's behavior). The
   group is built server side with `terminalGroup`; `History` and
   `terminalRecentRunDirs` already take a group/project. Project names are
   arbitrary strings: reject `/` and `\` at this boundary.
4. **Commands need server-side data.** The list is pane-local today, so a
   cross-project view requires the Keeper to record each group's commands
   (command line, exit code, time, directory) from the marks it already parses
   (`commandMarks`), in memory with a bounded length like `historyLimit`, and a
   `<shell>/commands?project=` endpoint. For the current project the pane's own
   list (output copy, fold, scroll-to) is kept; other projects' rows are
   read-only (copy the command line, or type it at the prompt without Enter, as
   directories are).
   **Open question:** is this the intended meaning of "command history", or
   should the pulldown cover Recent directories only? The plan assumes both,
   with Commands as a second phase.
5. **Shared behavior.** One server function owns "group for a project of this
   shell's basedir" and is used by both endpoints.
6. **Interfaces.** Web UI/API only; the CLI and Python client have no terminal.

## Implementation phases

1. Server: `project` parameter on `dirs`, shared group resolution, errors.
2. Page: pulldown in Recent directories, reload on change, current project initial.
3. Keeper: per-group command history; `commands` endpoint.
4. Page: pulldown in Commands with read-only rows for other projects.

## Documentation and contracts

- Update `docs/WEB.md` (Recent directories and Commands paragraphs).
- Add contract text in `contracts/05-web-assets-and-static-export.md` and WEB-*
  rows in `contracts/README.md`, with conformance tests.

## Tests and validation

- Go: `terminal_dirs_test.go` cases for `project` (own, other, unknown, name with
  `/` or `\`); Keeper command-history tests; endpoint tests.
- Conformance: another project's directories reachable through the Web API;
  unknown project is an explicit error.
- Browser test in the style of `terminal_panel_test.go`: pulldown defaults to the
  current project, switching changes rows, nothing typed while a program runs.
- `go test ./internal/webui ./internal/terminal ./conformance/...`.
