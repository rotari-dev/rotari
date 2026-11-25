# Plan: Open in VS Code

Created: 2026-11-24
Status: Design confirmed by phase 1's deciding checks on 2026-11-24. Phases 2
(the rename), 3 (opening without a line, which the user then tried in a browser),
and 4 (lines in shell output) are implemented, and the user has tried both in a
browser. Phase 5 (lines in job logs) is implemented and not yet tried in a browser.
Related: [artifact launcher](../2026-10-20-artifact-launcher/plan.md),
[terminal Explorer](../2026-11-13-terminal-explorer/plan.md),
[terminal output links](../2026-11-14-terminal-output-links/plan.md)

## Purpose

People work with `rotari web` and VS Code side by side. Seeing `train.py:123`
in a log means finding the file again in VS Code and going to the line by hand.
**Open in VS Code** opens the file, at the line, in the VS Code on the machine
the browser runs on.

The Web UI stays short of an IDE: it hands files to the editor instead of
editing them.

## Scope

1. **Open in VS Code** on files and directories in the terminal panel's
   Explorer, on paths a shell prints, and on Artifacts and **Files** rows.
2. Line and column: a printed `PATH:LINE` or `PATH:LINE:COLUMN`, and Python's
   `File "PATH", line N`, open at that place. This covers shell output and job
   logs.
3. The built-in launcher **VS Code**, which starts the Web version, becomes
   **VS Code (Web)**.

## Non-goals

- An editor, a diff view, or search in the Web UI.
- Embedding VS Code in a page, or a VS Code extension.
- Other editors and VS Code Insiders, which use other URL schemes.
- WSL, dev containers, and tunnels, which use other remote names.
- Going from VS Code to a rotari page.
- Choosing the VS Code window; VS Code does.
- Static exports, whose paths belong to another machine.
- A CLI or Python interface. In a VS Code terminal, `code --goto` does this.

## How it works

The page opens a URL, which the browser hands to the operating system, and that
to the installed VS Code:

| Where VS Code runs | URL |
| --- | --- |
| On the machine `rotari web` runs on | `vscode://file/PATH:LINE:COLUMN` |
| On another, through Remote SSH | `vscode://vscode-remote/ssh-remote+NAME/PATH:LINE:COLUMN` |

VS Code connects by itself, so nothing depends on how or when `rotari web` was
started, and the server runs no program.

`NAME` is what the user's own `ssh` is given on the browser's machine: a `Host`
alias, a host name, or `USER@HOST`. `ssh` takes the user name from its own
configuration when `NAME` has none. The server cannot know `NAME`; it can only
suggest its own host name. So the choice belongs to the browser.

On the machine checked on 2026-11-24, the host name is an FQDN, while VS Code
had connected by its first label in 74 of the 75 recent history entries sampled.
VS Code is expected to treat two names of one host as two remotes, so another
name would open another window.

### Rejected

- **Running `code` on the server.** It needs no name, and with VS Code 1.147 over
  Remote SSH it exited 0 in the environment of a running `rotari web`. But `code`
  reaches VS Code through a socket made by a VS Code terminal or window, which
  does not last as long as the server: 495 of that user's 498 socket files were
  dead. The server would have to look for a live one, and cannot trust exit 0,
  which `code` also returns when it has no socket. It also opens the VS Code of
  the person running the server, whoever clicked.
- **A name in a rotari config.** It would be one name for every browser, while
  two machines may call one host by different names.
- **A built-in launcher.** A launch takes no line, asks before each start, and
  leaves an entry in **Launches**.

## Decisions

1. **Names.** The action is **Open in VS Code**; the built-in becomes
   **VS Code (Web)**, as **MLflow (SQLite)** is named.
2. **The rename replaces nothing.** An entry named `VS Code` in someone's
   `launchers.toml` no longer replaces the built-in and appears beside
   **VS Code (Web)**. The built-in has shipped since v0.1.25, so the release
   notes and [docs/CONFIGURATION.md](../../docs/CONFIGURATION.md) say to rename
   the entry. No alias for the old name is kept.
3. **Asked once per browser.** The first **Open in VS Code** asks:

   ```text
   Open in VS Code
     ( ) VS Code runs on the same machine as rotari web
     (•) Connect with Remote SSH
         SSH host: [ ks000        ]   the name you use with ssh

                            [Cancel]  [Open]
   ```

   Later ones open at once. **Settings** has **VS Code connection**, which shows
   the same dialog. Cancel opens and keeps nothing.
4. **What the dialog starts with.** Remote SSH when the server's environment has
   `SSH_CONNECTION`, the same machine otherwise; the host is the server's host
   name up to the first dot. These are only suggestions: the server sees a
   forwarded browser as `127.0.0.1` either way, so it cannot tell.
5. **Where it is kept.** The browser's `localStorage`, under a key that holds the
   server's host name. One address such as `http://127.0.0.1:8787` may be
   forwarded to different servers at different times, and one server's name must
   not be used for another. When storage is refused, the page asks each time and
   says why.
6. **The result is not known.** The browser does not tell a page whether a URL
   opened. The page says "Opening FILE in VS Code as NAME" with a **Change**
   button for the dialog, and never that it opened. VS Code shows a wrong name's
   error.
7. **The name is checked whenever it is used**, not only when typed, since a
   launched service shares the origin and could write the storage: an optional
   `USER@`, then letters, digits, `.`, `_`, and `-`, neither part starting with
   `-`, which `ssh` would read as an option. Anything else opens the dialog with
   the reason. IPv6 addresses need an alias.
8. **Building the URL.** Only absolute paths. Each path segment is
   percent-encoded. Line and column are positive integers, a column needs a
   line, and a directory takes neither. A remote file without a line gets `:1`:
   without one, VS Code replaced the window's folder with that one file (phase 1).
   A path that itself has `:NUMBER` in it is refused, with the
   reason, unless phase 1 finds an unambiguous form. Opening goes through one
   shared function that takes no other scheme.
9. **A default click does not change.** A printed file path still opens the
   Table viewer on a click, and a directory a terminal tab. **Open in VS Code**
   is in the right-click menu, and an icon on Explorer rows. Where a path in a
   job log becomes a link, which has no click today, a click opens it in VS
   Code.
10. **Which server offers it.** Any live one, since it runs nothing:
    - Explorer rows and shell output exist only with `--allow-terminal`.
    - Artifacts and **Files** rows have it on every live server, including a
      read-only one, for what artifact requests already serve.
    - Log links need the server to say which paths exist. With the terminal, any
      path may be asked about, as for shell output. Without it, only paths under
      the roots the job's artifacts are served from, so that a page cannot probe
      the file system.
11. **Directories** open as folders in a window of their own, with
    `?windowId=_blank`, so that none takes the place of an open folder.
12. **The terminal's own menus** (asked for by the user on 2026-11-24) open the
    directory the shell is in now, where the Table viewer is offered: the bar, All
    shells cards, and, as well, a pane's right-click menu.
13. **Hiding it** (asked for by the user on 2026-11-25). A page cannot learn whether
    VS Code is installed, and an icon is too small a thing for the first dialog to
    ask about, so **Show Open in VS Code** in Settings hides it per browser.
14. **Paths on another host.** A path is the server's. Output a shell says came
    from another host has no path links today, and gets none.

## Design

- **`#vscode-connection`**, an element of live pages that holds the server's host
  name and the suggestions, for the storage key and the dialog. Its presence is
  what offers the action, so a static export needs no check and no route is
  needed (planned as `GET /api/editor`). No credentials or user names.
- **`GET /api/vscode-target`**: the artifact fields of a content request;
  answers `{path, kind}` through `resolveArtifact`, so its trust rules stay in
  one place, and refuses what it refuses.
- **`terminal_links.go`**: the position that `terminalLinePosition` drops today
  is answered as `line` and `column` beside `path`.
- **New candidate forms**, today not links:
  - a bare `train.py:12`, without `/`, only with a position and only when the
    file exists;
  - `File "PATH", line N`, whose path may hold spaces.
- **Job logs**: a route for an attempt's log candidates shares the finder of the
  shell links. Relative paths are taken from that attempt's recorded working
  directory. The log dialog's text stays text, and only lines in view are asked
  about. Only files are links: a directory would open another window.
- **`web_vscode.js`** (new): the stored choice, its check, the dialog, and the
  URL; used by the terminal, Artifacts, and logs. The click follows an anchor in
  place, which left the page and its shells as they were in phase 1. The page
  policy (`frame-src 'self'`) rules out a hidden frame.

## Variants to cover

| Dimension | Values |
| --- | --- |
| Target | file, directory, symbolic link, removed since listed; spaces, `%`, `#`, `?`, non-ASCII, `:NUMBER` in the name |
| Position | none, line, line and column, zero, overflow |
| Stored choice | none, same machine, remote, invalid, another server's, storage refused |
| Server | terminal, launching, both, neither, read-only, static, another basedir |
| Logs | plain job, array task, matrix member, latest and older attempt, carried result, running job |
| Page | application page, Recent jobs, terminal window |

A carried job's log uses the working directory of the attempt that wrote it.

## Phases

1. **Check the URLs by hand**, before any code (below). Done for what decides the
   design: the right window, and no question at every open. The rest is checked
   in phase 3.
2. **Rename** the built-in: registry and template, tests, conformance, contract,
   CONFIGURATION, WEB, SECURITY, and FAQ. Implemented; the release notes are
   still to say it (decision 2).
3. **Open without a line**: the route, the dialog, Settings, Explorer rows,
   the path menu, Artifacts and **Files**.
4. **Lines in shell output**: positions in `terminal_links.go` and the new forms.
5. **Lines in job logs.**

Phases 2 and 3 are useful alone.

### Phase 1 checks

With the repository open in a Remote SSH window:

```text
vscode://vscode-remote/ssh-remote+ks000/data/store5/2026/kamo/rotari/internal/webui/page_policy.go:34:1
```

Checked by the user on 2026-11-24, from the browser's address bar: the window
of the repository showed line 34. The browser asked each time whether a website
may open the application.

Checked by the user on 2026-11-24, from a script-clicked anchor in a page of the
live `rotari web` (the browser's name was not recorded): after allowing it once,
the browser did not ask again, and the terminal's shells stayed connected. An
address typed in has no website for the browser to remember.

Checked by the user on 2026-11-24, from the address bar: without a position, the
file opened in the existing window, whose folder closed, leaving that file alone.
The directory `docs` with `?windowId=_blank` opened in another window and left
the existing one as it was.

Checked by the user on 2026-11-24, with phase 4: a Python traceback's line opened,
and a shell's directory opened from the terminal's bar and menu in the same window,
which the user prefers. Taken to be a directory that window already had open.

Not yet checked:

- With another window open on something else, which one is used.
- The same with the connection down, and with no VS Code running.
- Names with spaces, non-ASCII characters, `#`, and `:12`.
- `vscode://file/PATH:LINE:COLUMN`, a file without a position, and a directory, on
  a local machine.
- From a page in VS Code's integrated browser.
- The same in the other browsers among Chrome, Safari, and Firefox.

## Contracts and documentation

- New contract row (next free ID, WEB-64 today) for the action; WEB-46 gains
  positions and the new forms; the built-in's list in
  [contracts/05](../../contracts/05-web-assets-and-static-export.md) is renamed.
- [docs/WEB.md](../../docs/WEB.md) (including its statement that `:12` is left
  out), CONFIGURATION, SECURITY, FAQ, and ARCHITECTURE.

## Tests and validation

- `internal/webui`: the route in each server mode, refused artifacts,
  positions in links, both new forms, and log paths inside and outside the roots
  without the terminal.
- Page tests: the dialog and its suggestions, each stored choice, the key per
  server, refused names (a leading `-`, `/`, spaces, control characters), URL
  building for each target name and position, and nothing opened on Cancel.
- Conformance through the binary: the route, and positions in links.
- The phase 1 checks, done and not yet done, on the finished feature.
- Node-based tests and the type check need a newer Node than this machine's
  default v12.
