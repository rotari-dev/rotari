# Work log: Click to move the cursor in the Web terminal's command line

## 2026-11-15 01:30 +09:00 — Clicks by arrow keys

- **Change:** `web_shell.js` adds `clickToMove` to each pane's viewport: a `mousedown` with the left button and no
  modifier, and a `mouseup` within 2px of it, that left no selection (`term.hasSelection()`), with no link under the
  pointer (`.xterm-cursor-pointer` in the terminal's element) and no mouse tracking (`modes.mouseTrackingMode`), goes
  to `moveByClick` with its screen cell (`screenCell`, from `.xterm-screen`'s box). `clickTarget` takes it only while
  `promptBlock` finds the last block with `B` and no `C` or end, outside the alternate screen, with the view at the
  bottom, on a row from `B`'s to the cursor's or the rows wrapped on from it, and not before `B`; past a row's text
  (`rowEnd`) it goes to the row's end, or the cursor's place if further, and the second cell of a wide character to
  the character. `charsFrom` counts characters from `B`: a width-0 cell is none, and a row that ends without wrapping
  is one. The difference between the target's count and the cursor's is sent as `ESC [ C` / `ESC [ D`, or `ESC O`
  in application cursor mode, through `send` as typing is. The fake terminal gains wide cells (a character past
  U+2E80, then a NUL), `hasSelection`, and the cursor key and mouse tracking modes.
- **Reason:** plan phase 1, after the user accepted the plan with the open questions answered as proposed.
- **Plan impact:** none.
- **Validation:** `TestTerminalPanel` (no marks: nothing; `echo 日本x`: to the wide character by either cell, left
  and right, past the end; application cursor mode; the prompt, output, and an earlier command: nothing; a drag, Alt,
  Shift, a selection, a link, mouse tracking, the alternate screen, the view scrolled back, and a running command:
  nothing; a wrapped line both ways) passed; `tsc` clean; prettier formatted. Not tried in a real browser: there is
  none here, and the browser step is phase 3.
- **Remaining:** phases 2 (zsh exactly) and 3 (contracts, docs, the browser step).

## 2026-11-15 02:10 +09:00 — zsh exactly

- **Change:** rotari's `.zshrc` defines `__rotari_goto`, a widget bound to `ESC [ 9998 ~` in the `emacs`, `viins`, and
  `vicmd` keymaps, which reads up to ten digits with `read -k` and, on a closing `;`, sets `CURSOR` to them, capped
  at `${#BUFFER}`; anything else ends it doing nothing. When the widget is made, `__rotari_prompt` prints `A` as
  `A;rotari_goto=1`. The panel records that on the block (`goto`), and `moveByClick` then sends `ESC [ 9998 ~`, the
  target's character index from `B`, and `;`, instead of arrows.
- **Reason:** plan phase 2: arrows past the typed text would take a zsh-autosuggestions suggestion, and `CURSOR`
  cannot pass the buffer's end.
- **Plan impact:** none.
- **Validation:** with a pty and `LANG=C.UTF-8`, `echo 日本abc` then index 6 and `Z`, then index 99 and `Q`, ran
  `echo 日Z本abcQ`, with a spy on `forward-char` never called. `TestZshIntegrationPutsTheCursorWhereClicked` does the
  same with ASCII (the test's environment may have no UTF-8 locale) and passes three times in a row; the zsh tests now
  expect `A;rotari_goto=1`. `TestTerminalPanel` has a shell that announced it told index 5 for the wide character and
  8 past the end; passed. Not tried with the user's own zsh, which would add the test's lines to their history file.
- **Remaining:** phase 3 (contracts, docs, the browser step).

## 2026-11-15 02:50 +09:00 — Contracts and docs

- **Change:** contract **WEB-50** (the click: which clicks, where, how counted, and what is sent) with its row, and
  WEB-49's note and row on zsh's widget and `A;rotari_goto=1`. `TestTerminalShellIntegration` expects zsh's `A` with
  the parameter and types `echo abc`, index 5 and `Z`, index 99 and `Q`, through the binary, awaiting `ZabcQ`.
  `TestTerminalPanelInBrowser` now covers WEB-50 too: in the `/bin/sh` tab it starts `bash --norc --noprofile -i`
  (sh has no line editor to move), gives it a `PS1` with `A` and `B`, types `echo $((40+2))x` without Enter, clicks
  the cell of `$` with the browser's mouse, types `X` and Enter, awaits `X42x`, and exits bash. docs/WEB.md says what a
  click does, and the zsh-autosuggestions caveat for a zsh rotari did not start.
- **Reason:** plan phase 3.
- **Plan impact:** none.
- **Validation:** `TestTerminalShellIntegration` passed. It had not been run since phase 2, which changed zsh's `A`;
  it would have failed on its exact `\x1b]133;A\a` for zsh, fixed here before any push. `TestContractStatus` passed.
  The browser script passes `node --check`; the step runs only in CI.
- **Remaining:** nothing in this plan; the right-click menu is the next plan.

## 2026-11-15 05:30 +09:00 — The line's start after a redraw elsewhere

- **Change:** rotari's zsh adds `__rotari_cursor` as a `line-pre-redraw` hook, printing `133;rotari_cursor=$CURSOR`
  before each redraw. The panel records it on the prompt's block (`lineCursor`), and a new `commandLine` gives click to
  move and Cut the line's start: `walkBack`, from the cursor by that many characters (counted as `charsFrom` counts),
  when the shell said it; else `B`, unless a row between `B` and the cursor is not wrapped, when it returns null and
  nothing moves rather than moving wrong. `clickTarget` and `cutPlace` use it. WEB-50, WEB-49, and WEB-51 say so; the
  terminal tests' `marksAfter` leaves the cursor marks out.
- **Reason:** the user found click to move, and Cut with it, worked only sometimes. zsh redraws the prompt elsewhere
  without any mark after Ctrl+L and after listing completions (seen with a pty: no `A` or `B` for either; Ctrl+C does
  send `D A B`), which left `B` on the old row, so the start and every count from it were wrong.
- **Plan impact:** none; decision 1's "from `B`" becomes "from the line's start".
- **Validation:** with a pty, zsh said `rotari_cursor=4`…`6` while typing, `6` after Ctrl+L, `5` after a left arrow,
  and `6` around a completion list. `TestTerminalPanel` has a `B` left behind on an old row: no move without the
  cursor's word, the right index with it, nothing on the redrawn prompt; passed. `TestZshIntegrationPutsTheCursorWhereClicked`
  checks `rotari_cursor=7` after `echo ab` and Ctrl+L; the integration tests pass twice, and so does
  `TestTerminalShellIntegration`. Bash has no hook for this, so after Ctrl+L or a completion list a click there does
  nothing until the next prompt.
