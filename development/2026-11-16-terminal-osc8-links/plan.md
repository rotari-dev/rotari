# Plan: OSC 8 hyperlinks in the Web terminal

Created: 2026-11-16
Status: Accepted by the user on 2026-11-16, with open question 1 answered the other way (a prompt for every web
address) and 2 as proposed. See [work-log.md](work-log.md).

## Goal

With `--allow-terminal`, a hyperlink a program prints with OSC 8 (`ls --hyperlink`, gcc's and GNU tools' diagnostics,
`systemd`, newer CLIs) opens the way rotari's other terminal links do: a web address in a new tab once confirmed, and a
`file://`
path of this host as a path link (a directory opens a shell there, a file the Table viewer, right-click the Explorer's
menu). Today it opens through xterm.js's fallback, a browser `confirm()` and then a blank window, for any scheme.

This is a non-goal left in the [output links plan](../2026-11-14-terminal-output-links/plan.md), chosen on 2026-11-16
from the remaining terminal candidates.

## Existing implementation

- **OSC 8 parsing.** The vendored xterm.js 5.5 already reads `ESC ] 8 ; params ; URI ST`, keeps the link with its
  cells, underlines it on hover, and calls the `linkHandler` option's `activate(event, uri)`, or, unset as now, its
  fallback: `confirm("Do you want to navigate to …")`, then `window.open()` with that URI, whatever its scheme.
- **Plain web addresses** (`WebLinksAddon`) open with `window.open(uri, "_blank", "noopener")`, no prompt.
- **Path links** (WEB-46). `askLinks` asks `POST /_terminal/shells/ID/links` with `{candidates, dir?, host?}`; the
  answer's `kind` (`dir`, `file`) drives `followOutputLink` (a directory: `newShell(focused, path)`; a file:
  `openInViewer`), `linkTitle` the tooltip, and `pane.hoveredLink` the right-click menu (`pathChoices`). The server
  treats a `host` other than its own as remote and links no paths there (`terminalLocalHost`).

## Decisions

1. **A `linkHandler` replaces the fallback.** `activate`, `hover`, and `leave` handle OSC 8 links; nothing else
   changes about how xterm.js parses or draws them.
2. **Web addresses are confirmed.** Opening any web address from the terminal, an OSC 8 link's or a plain one the web
   links addon finds, first asks `confirm()` with the address: an OSC 8 link's text can hide where it goes, and a
   plain one is text any program printed. Path, run, and job links open without asking, as now; they stay on this
   server.
3. **By scheme.**
   - `http:` and `https:` open in a new tab with `noopener` once confirmed (decision 2).
   - `file:` is a path: the host and the percent-decoded path (UTF-8, absolute) go to `askLinks` as one absolute
     candidate with `host`, so the server decides this host or another and directory or file, as for printed paths.
     An answer opens as a path link does; another host's path, a path that does not exist, or one that does not
     decode opens nothing and shows a note saying why.
   - Every other scheme (`javascript:`, `data:`, `mailto:`, `ssh:`, …) opens nothing; the note names the scheme. Its
     address can still be copied (decision 5).
4. **Hover.** The tooltip shows the address (for `file:`, the path and what it is once answered), with
   "; right-click for more" as for other links; `pane.hoveredLink` is set from `hover` and cleared on `leave`.
5. **Right-click.** On a `file:` link that answered, the Explorer's menu (`pathChoices`, with Show in the Explorer),
   as for a printed path. On any other OSC 8 link, Open link (when decision 2 opens it) and **Copy link address**.
6. **The typed command line.** OSC 8 is output; a program does not print it into the line being typed, so the rule
   that the command line has no links needs nothing here.
7. **Server.** No change: the links route already takes an absolute candidate with `host`.
8. **Contracts.** WEB-46 gains the OSC 8 rule (schemes, `file:` as a path, the prompt for web addresses) and WEB-51
   the right-click entries; no new ID.

## Phases

1. **Panel.** The `linkHandler` (activate, hover, leave), the `file:` parsing, the notes, and the menu entries. JSDOM
   tests: an `https:` link, OSC 8 and plain, opening a tab only once confirmed; a `file:` directory opening a shell and a file the Table
   viewer; another host's `file:` and a missing path opening nothing with a note; `javascript:` opening nothing; the
   tooltip; right-click on a `file:` link (the Explorer's menu) and on an `https:` link (Copy link address).
2. **Contracts and docs.** WEB-46 and WEB-51 wording, a browser conformance step (CI only: `printf` an OSC 8 `file:`
   link to a file, click it, the Table viewer opening), and docs/WEB.md.

## Tests and validation

- JSDOM in `TestTerminalPanel`; `go test ./internal/webui ./conformance/05-web` with Node 24, `tsc`, and prettier.
- The browser step runs only in CI (no local Chrome).

## Non-goals

- Making OSC 8 links in the log viewer or other pages.
- Opening other schemes, such as `mailto:` or `ssh:`.
- The `id=` parameter beyond what xterm.js already does with it (joining a link's parts across lines).

## Settled questions

1. **A prompt for web addresses.** Proposed: none, as for plain web addresses. The user chose a prompt for both, OSC 8
   and plain (decision 2).
2. **Other schemes.** Proposed and accepted: open nothing, copy only (decision 3). The alternative let `mailto:` open
   the mail client.

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace.
