# Plan: A shell's transcript on a page of its own, saved as Markdown, HTML, or a shell script

Created: 2026-11-16
Status: Accepted by the user on 2026-11-16 without PDF (open question 1), the `cd` lines as proposed. See
[work-log.md](work-log.md); all four phases are done.

## Goal

With `--allow-terminal`, a pane's session opens on a page of its own as a transcript: one card per command, with its
command line, where it ran, how it ended, how long it took, and its output in the terminal's colours. From that page
the transcript, or the commands chosen in it, is saved as Markdown, as a self-contained HTML file, or as a shell
script of the commands alone. The user asked for this on 2026-11-16 ("どうせやるならリッチに"), after Warp's
features were discussed; Warp's notebooks were left aside.

## Existing implementation

- **Blocks** (WEB-48): each block has its prompt, command line (`block.command()`), exit, times for commands seen
  running in the page, and the directory the shell said at its prompt (`block.place`, OSC 7, WEB-46).
- **Kept bytes** (WEB-58): each pane keeps the raw bytes written since its last reset, up to 8 MiB, with each block's
  output start and end in them (`block.bytes`); `bytesText` reads bytes as plain text. Folded blocks keep their whole
  output there.
- **Pages.** The server serves its pages with a nonce-based content security policy; the panel's own window
  (`/_terminal/window/...`) is served only where the terminal is, behind its checks, and talks to its opener with
  `postMessage`. The Table viewer opens a window first and then sets its address, so a browser does not block it.

## Decisions

1. **Where it opens.** The pane's right-click menu gets **Open transcript**, and a block's mark menu **Open
   transcript from here**, both opening a new tab of `/_terminal/transcript`, a page the server serves only where
   the terminal is, with its own script (`web_transcript.js`) under the page's nonce. The page asks the tab that opened
   it for the transcript by `postMessage` (same origin checked both ways), so the output never goes through the server;
   the server sends only the page. The page says so, and shows a message when it has no opener or the opener has gone.
   **Refresh** asks again.
2. **What the transcript holds.** The pane's blocks whose output is all in the kept bytes, oldest first, with any
   output before the first prompt as a first card of its own; a running command's output so far, marked running; a
   folded block's whole output. For a shell that marks nothing, the kept output as one card. The shell's name, its
   project, and when the transcript was taken head the page.
3. **How a card shows.** The command line in the terminal's font; a dot and a word for how it ended (ok, failed with
   its exit code, running, unknown); where it ran (OSC 7) when the shell said; how long it took for commands seen
   running; and the output, with its SGR colours and bold, italic, underline turned into styled spans (other escape
   sequences dropped, carriage returns resolved as in `bytesText`), in a box that a long output (over 30 lines) opens
   folded, with **Show all**. Each card has a checkbox, all checked at first, and **Copy command** / **Copy output**.
   A filter at the top narrows the cards by command line, and **Failed only** keeps failed ones.
4. **Saving.** Buttons at the top save the checked cards:
   - **Markdown** (`.md`): a title, then per card a `##` heading with the command in code, a line of its directory,
     exit, and time, and its output in a fenced `text` block (a fence longer than any backquotes in it).
   - **HTML** (`.html`): one self-contained file with the page's look, colours kept, styles inline, no script.
   - **Shell script** (`.sh`): `#!/usr/bin/env bash`, a comment saying where it came from and when, then the
     commands in order, outputs left out; a command whose directory differs from the one before is preceded by `cd`
     and the directory, quoted, so that the script runs where the commands did; a failed command gets a comment with
     its exit code above it.
   The files are made in the page and downloaded (`<shell name>-<date>.<ext>`); nothing is stored on the server.
5. **Size.** At most the kept bytes (8 MiB); the page draws the cards as they come into view, so that a long
   transcript stays usable, and the output box of a folded card is drawn when opened.
6. **Security.** The transcript is only what the page already shows. The page is served behind the terminal's checks
   like the panel's window; it takes messages only from its opener's window and origin, and the opener answers only
   its own transcript pages. Output is put into the page as text, never as HTML, and the HTML file escapes it the same
   way. docs/SECURITY.md says so.
7. **Contracts.** A new ID, WEB-60, for the transcript page, what it holds and shows, and the three ways to save.

## Phases

1. **The transcript in the panel.** Gathering a pane's cards from its blocks and kept bytes, turning output bytes into
   styled runs (SGR) and text, the menu items, and the panel's side of the `postMessage` exchange. JSDOM tests:
   commands with exits, a running one, output before the first prompt, a shell with no marks, a folded block whole,
   colours and bold kept, cursor movement dropped, an answer only to a transcript page it opened.
2. **The page.** The route (served only with the terminal, behind its checks, with the nonce), `web_transcript.js` and
   its CSS, the page's side of the exchange, and the cards (filter, failed only, checkboxes, folded outputs, copies).
   Go tests for the route; JSDOM tests for the page (no opener, another origin ignored).
3. **Saving.** Markdown, HTML, and the shell script. JSDOM tests: each file's content for checked cards (fences,
   escaping, `cd` lines, failed comments).
4. **Contracts and docs.** WEB-60, docs/WEB.md, docs/SECURITY.md, and a browser test (CI only: run two commands, open
   the transcript, see both cards with their outputs, save Markdown and the shell script, and read the files).

## Tests and validation

- JSDOM in `TestTerminalPanel` and a new `TestTerminalTranscript`; Go tests for the route; `tsc` and prettier; `go test
  ./internal/webui ./conformance/05-web` with Node 24.
- The browser test runs only in CI (no local Chrome).

## Non-goals

- Notebooks (runnable documents), as Warp has.
- Keeping transcripts on the server, or a transcript of output the page has not received.
- A live transcript that grows with the shell; **Refresh** takes it again.
- Re-running the shell script from the page.
- PDF (settled question 1).

## Settled questions

1. **PDF.** Proposed: the browser's print dialog with a print layout. The user chose no PDF at all.
2. **The shell script's `cd` lines.** Proposed and accepted: a `cd` before a command whose directory differs (decision
   4), so the script runs where the commands did.

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace.
