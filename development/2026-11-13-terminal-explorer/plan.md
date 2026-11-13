# Plan: An Explorer in the Web terminal

Created: 2026-11-13
Status: Implemented (all three phases); see [work-log.md](work-log.md). Not checked in a real browser: there is
none on this machine, and `TestTerminalPanelInBrowser` (WEB-33), which runs only in CI, was not extended. The JSDOM
test `TestTerminalPanel` covers the panel's side. Decision 10's menu items wait for the user.

## Goal

With `--allow-terminal`, the terminal panel has an Explorer, as an editor has beside its terminal: a tree of the
files where the focused pane's shell is, which follows the shell as it changes directory, and from which a file
opens in the Table viewer or its path goes into the shell.

The mockup the user agreed to on 2026-11-13 is <https://claude.ai/artifact/QVy23puH6RTcmp1EsXFRJE> (boards: the
panel at the bottom, the panel at a side, and the details).

## Existing implementation

- **The panel.** [web_shell.js](../../internal/webui/assets/web_shell.js) builds `#terminal-panel` as the resizer,
  the bar, and `.terminal-panes`, which holds up to four panes laid out by `applyLayout`. `focused` is the pane the
  bar and sending act on. The panel sits at the bottom, at either side (`data-terminal-place`), maximized, or in a
  window of its own (`inWindow`); under 1000 pixels wide it stays at the bottom.
- **Where a shell is.** `Keeper.Dir` reads the shell's own working directory (`/proc/PID/cwd` on Linux), and
  `/_terminal/shells/ID/preview` reports it as `cwd`.
- **Sending.** `sendToTerminal` refuses text with control characters and pastes the rest into the focused pane;
  `asWord` quotes a value that is not plain. `shellQuote` in
  [web_app_logs.js](../../internal/webui/assets/web_app_logs.js) quotes values in the commands copy buttons show.
  Both close the quote around `'` as `'\''`, which fish reads wrongly when the name also holds `\`.
- **The Table viewer on a shell.** `POST /api/tableview-sources` names a running shell, and the server pins the
  directory it is in under a random source id (WEB-39); `/tableview/?source=ID&child=NAME` opens a file in it.

## Decisions

1. **Where.** A column at the left of the panel's body, beside the panes, wherever the panel is, with its own resize
   handle; the panes keep at least 200 pixels. Its button is the first in the bar. It is open unless closed, and
   whether it is open, and its width, are remembered by the browser. (Changed on 2026-11-13 after the user tried it:
   the first version opened over the panes as a drawer where the body was narrower than the Explorer and 480
   pixels, which at a side hid the terminal's text and could not be resized, and it started closed.)
2. **One Explorer per panel**, following the focused pane's shell. Its root is the shell's directory while
   **Follow** is on. **Up** shows the parent directory and turns Follow off; the tree then stays where it is when
   the shell moves, and the footer says where the shell is, with a **Follow** button back. Nothing limits how far up
   or down it goes: a shell can already go anywhere.
3. **Listing API.** `GET /_terminal/files?shell=ID` lists the directory a running shell is in, and
   `GET /_terminal/files?path=/ABS` any directory, both only behind the terminal's own checks (the terminal on,
   the token, `Host`/`Origin`, and HTTPS or a direct loopback connection). The answer is the directory and its
   immediate entries, folders first, then by name, each with its kind (`dir`, `file`, `symlink`, `other`) and,
   for a symbolic link, its target and the kind of what it reaches (`missing` if nothing). At most 2,000 entries
   come back, with `total` saying how many there are; the tree says how many it does not show and offers the
   folder in the Table viewer. A failure is an error with the reason (not a directory, permission denied, the
   shell's directory unknown), never an empty listing.
4. **Names are bytes.** A name that is not valid UTF-8 cannot travel in JSON unchanged, so the server marks it
   (`not_utf8`) and the Explorer shows it but will not open, expand, send, or copy it, saying why. A shell whose
   directory is not valid UTF-8 cannot be followed, and the Explorer says so. Control characters in a name are shown
   as escapes (`\n`, `\t`, `\x1b`) in a mark of their own, written as text.
5. **The Table viewer on a path.** `POST /api/tableview-sources` may name `path`, an absolute path, instead of
   `shell`, but only with the terminal on and over HTTPS or a direct loopback connection, as the terminal is
   served. The page naming a path is new for the Table viewer; the user accepted it, since whoever can use the
   terminal can read the file anyway. The server follows symbolic links first, since the viewer opens files
   through `os.Root`, which refuses a link to an absolute path; then it pins a folder as a source of its own, or a
   file's folder with the file's name returned as `child`. The viewer reads under `/`, so the media a file names
   are found wherever they are. Each opens in a new tab.
6. **Clicks and keys.** A click selects, and on a folder expands or collapses it; a double click or Enter on a file
   opens it in the Table viewer. Arrow keys move and expand as in a tree. A row being pointed at or selected shows
   **Send the path to the terminal** and **Open in the Table viewer**. Right-click offers those, **Copy the path**,
   and **Copy the path from here** (relative to the shell's directory). Dragging a row onto a pane types its path
   there.
7. **The path that goes in.** Relative to the shell's directory when the file is under it, so that it finishes a
   command typed there; otherwise absolute. Never `..`. It goes through `sendToTerminal`'s rules: as one word,
   quoted if it needs to be, and refused with a note if it has control characters.
8. **One quoting rule for every shell.** `shellQuote` moves to web_shell.js, which every page and the window load
   first, and `asWord` becomes `shellWord`, which uses it. A value is wrapped in `'...'`, with each `'` and `\`
   written outside the quotes as `\'` and `\\`. This reads the same in POSIX shells, tcsh, and fish (checked on
   2026-11-13 with sh, dash, bash, zsh, and tcsh; fish is not installed here, and its rules say the same). The shell is
   not detected: a program in front, such as a REPL or `ssh`, could not be told apart anyway, and nothing is run
   until the user presses Enter. PowerShell and nushell are not served by this.
9. **Freshness.** The tree lists again when the panel's shell list refreshes (every five seconds in view), shortly
   after Enter is typed in the focused pane (for `cd`), when the focused pane changes, and on **Refresh**. Expanded
   folders are listed again with the root. A folder that has gone collapses.
10. **Not in this plan:** **Open a terminal in** and **Launch in** for a folder, shown in the mockup's menu. Each
    needs a shell start or a launch to take a directory from the page, a further change to WEB-34 and WEB-38;
    they wait for the user to ask. Creating, renaming, and deleting files stay the shell's.

## Phases

1. **Server.** `/_terminal/files` in `internal/webui` (`terminal_files.go`), and `path` on
   `/api/tableview-sources`. Unit tests beside them.
2. **Panel.** The Explorer in web_shell.js and web_shell.css: the column, Follow and Up, the tree with
   keys, the row buttons and menu, drag to a pane, the refusals, and the quoting change with `shellQuote` moved.
   JSDOM tests in `TestTerminalPanel`.
3. **Contracts and docs.** A new WEB-42 for the Explorer and its API; WEB-39 says a source may name a directory
   for the Explorer. The Terminal section of [docs/WEB.md](../../docs/WEB.md), the terminal routes in
   [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md), and the Web terminal section of
   [docs/SECURITY.md](../../docs/SECURITY.md).

## Tests and validation

- Server: a listing of a shell's directory and of a path, folders first; a symbolic link to a folder, to a file,
  and to nothing; a name that is not UTF-8 marked; the 2,000 limit with `total`; a relative path, a missing one, a
  file, and an unreadable directory refused with the reason; no terminal is 404; plain HTTP to another host is
  403. `path` sources: a folder pinned, a file pinned in its folder with `child`, a link followed, refused without the terminal, over plain HTTP from another host, or with both `path` and `shell`.
- Conformance under WEB-42 through the binary, and WEB-39 extended.
- JSDOM: open by default and remembered closed; the column kept and clamped in a narrow panel, and its edge dragged; Follow moving with the shell and Up stopping it; the
  path sent relative and absolute, quoted, and refused for control characters and non-UTF-8 names; a drop on a
  pane; the menu's copies; the limit's note; a refused listing shown in the tree. `shellWord` and `shellQuote`
  for `'`, `\`, and plain values.
- `TestTerminalPanelInBrowser` (WEB-33) runs in CI only; it is not extended here.

## Coordination

Other workspaces touch the terminal. This work was based on `main` at `1553b248`; before integrating, it is rebased
onto `main` and the panel tests run again.
