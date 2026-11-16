# Work log: The command's line kept at the top of the Web terminal while its output scrolls

## 2026-11-16 23:02 +09:00 — The line and its switch

- **Change:** `web_shell.js`: `updateSticky` puts `.terminal-sticky` in a pane's viewport (now `position:
  relative`) over its first row while the top line in view (`viewportY`) is in a block's output and the block's
  prompt is above it, not in the alternate screen: the block's state dot in the gutter, then `$ ` and the command's
  first line (` …` after one of several), in the terminal's font, a row high, its title the mark's with "Click to go
  to its prompt."; a click goes to the prompt (`goToBlock`). It is updated on scroll, resize, a block's end (in
  `readBlockMark`, since a failed block's decoration is remade rather than painted), a painted block, a replay, a
  rewrite, and dropped blocks. **Keep the command at the top**, a checked item last in the pane's right-click menu,
  turns it off and on for every pane, kept as `rotari-terminal-sticky` in `localStorage`.
- **Reason:** plan phase 1, after the user said "go"; the switch, planned for phase 2, came with it.
- **Plan impact:** the switch is the right-click menu's last item, after a command's copies, as a setting.
- **Validation:** `TestTerminalPanel` (its stand-in terminal now keeps every scroll listener): shown in `seq 30`'s
  output with its mark, not with its prompt in view, at the next prompt, or at a prompt without output; a command of
  two lines cut with `…` and its title whole; a running command's mark turning red when it fails; not in the
  alternate screen; a click going to the prompt and hiding it; the menu's item turning it off (stored) and on. The
  older right-click menu lists gain the item at their end. `tsc` and prettier clean.
- **Remaining:** phase 2 (WEB-59, docs, and the browser test).
