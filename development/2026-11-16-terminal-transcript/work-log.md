# Work log: A shell's transcript on a page of its own

## 2026-11-16 23:54 +09:00 — The transcript in the panel

- **Change:** `web_shell.js`: `transcriptOf(pane, from)` gathers the pane's shell (name, project, basedir), when it
  was taken, whether older output is gone (`partial`), and cards: output before the first prompt (when it holds
  text), then each block with output, from the block whose prompt is at `from` in the kept bytes, with its command,
  state, exit, directory (OSC 7), times, and output up to its D, the next prompt, or the end. `sgrRuns` turns output
  bytes into `{ t, s }` runs: SGR bold, dim, italic, underline, inverse, the 16 colours, 256 colours, and true colour
  (`applySgr`), line breaks as `"\n"` runs, other escape sequences and control characters dropped, a line written
  over after a lone CR read as written last. **Open transcript** in the pane's right-click menu (before the sticky
  line's switch) and **Open transcript from here** in a block's mark menu open `/_terminal/transcript` in a new tab
  and remember it with its pane, shell, and starting prompt; a `rotari-transcript-ask` message from such a page, of
  this origin, is answered with `rotari-transcript` and the transcript, or with a reason when the pane has closed or
  shows another shell.
- **Reason:** plan phase 1, after the user accepted the plan without PDF.
- **Plan impact:** the panel's side of the exchange is in phase 1, as it is the panel's.
- **Validation:** `TestTerminalPanel`: output before the first prompt, `make` (bold green, 256 and true colour runs),
  `false` (an erased line and a progress written over to 100%), and a running `sleep`, each with exit and directory;
  no answer to another origin or to a window it did not open; from a block's menu, from that block on; a pane showing
  another shell says so. The older menus that are listed whole gain the new items, and the mark menu's Type command
  again is found by its label (also in `TestTerminalPanelInBrowser`'s list). `tsc` and prettier clean.
- **Remaining:** phases 2 (the page), 3 (saving), and 4 (contracts and docs).

## 2026-11-17 00:00 +09:00 — The page and saving

- **Change:** `/_terminal/transcript` (`internal/webui/terminal_transcript.go`): GET or HEAD, served only with the
  terminal behind its checks, `no-store`, from `assets/terminal_transcript.html` with `web_transcript.js` inlined under
  the page's nonce at the end of the body; `/_terminal/transcript.css` is served with the terminal's assets.
  `web_transcript.js` applies the stored theme, asks its opener (`rotari-transcript-ask`, this origin), hears only its
  opener of this origin, says why when there is no opener, no answer within 4 seconds, or a reason, and draws a card per
  command: a checkbox, the state's dot, the command (`pre` for one of several lines), Copy command / Copy output, a line
  of the state, directory, and time, and the output as text with styled spans (the theme's `--terminal-*` colours,
  the 256-colour cube and grey ramp, true colour, bold, dim, italic, underline, inverse), drawn as the card comes into
  view, folded past 30 lines with **Show all N lines**. A filter by command and **Failed only**, **Check all** /
  **Check none**, **Refresh**, and **Save Markdown**, **Save HTML**, **Save shell script** for the cards shown and
  checked, each made in the page and downloaded as `<shell name>-<UTC stamp>.<md|html|sh>`: Markdown with code spans and
  fences longer than the backquotes inside; one HTML file with inline styles, a fixed dark palette, escaped output, no
  script; a bash script with `cd` (quoted) where the directory changes and comments above failed and running commands.
- **Reason:** plan phases 2 and 3, together, as saving is the page's.
- **Plan impact:** none.
- **Validation:** `TestTerminalTranscriptPage` (the page with its nonce and script, its styles, POST 405, 404 without the
  terminal), the route in `TestTerminalShellRoutesAreGuarded`, and `TestTerminalTranscript` (JSDOM: no opener; another
  origin and another window ignored; a reason; seven cards with states, metas, a multi-line command, styled spans, 256
  colours, markup kept as text, no output; a long output folded and shown; Copy command; the filter and Failed only; the
  shell script, the Markdown of two checked cards, and the HTML file, each in full; nothing checked; Refresh) pass.
- **Remaining:** phase 4 (contracts and docs).
