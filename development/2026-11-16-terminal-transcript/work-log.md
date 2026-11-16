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
