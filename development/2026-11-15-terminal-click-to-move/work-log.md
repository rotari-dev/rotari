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
