# Work log: Folding a command's output in the Web terminal

## 2026-11-16 22:34 +09:00 — Kept bytes, rewriting, controls, and copying

- **Change:** `web_shell.js`: each pane keeps the raw bytes it writes since its last reset (`pane.record`, up to
  8 MiB, the oldest chunks going first; reset on a connection and a group change; Clear keeps them from the last
  prompt on). Every chunk goes through `feed`, which scans it for OSC 133 marks (`scanMarks`: BEL, ESC \\, or a lone
  ESC ending them, across chunks) before `term.write`, queueing each with its place; `readBlockMark` takes them in
  order, so each block has `bytes.prompt`, `outStart` (after C), and `outEnd` (at D); a mark of another kind than
  the one parsed drops the queue and stops folding until the next reset. At D, `foldInfo` says whether the block can
  fold (ended, all kept, not in the alternate screen, over 10 lines by line feeds) and where its first and last 3
  lines are. `foldBlock` / `unfoldBlock` change `pane.folds` and `rewrite`: after the terminal's pending writes,
  it drops the blocks, resets, writes the kept bytes with each folded middle given way to a dim line
  ("⋯ N lines folded — click to unfold", preceded by OSC 133 `rotari_fold`, which the panel reads into the block's
  fold line and decorates as a button that unfolds), keeps the blocks' times, holds output that arrives meanwhile and
  writes it after, and scrolls to the folded block's prompt. Controls: **Fold output** / **Unfold output** in the
  mark's menu (only when it can fold or is folded), a **Fold** / **Unfold** button among the prompt line's copy
  buttons, and in Commands a "folded" note and the same button; a fold that is no longer possible says why. Copying
  a folded output reads the kept bytes as text (`bytesText`), and Select output unfolds first.
- **Reason:** plan phases 1 and 2, after the user said "go", together: folding needs its controls to be tried.
- **Plan impact:** the mark's menu offers Fold output only for an output that can fold, rather than showing it
  disabled on every short one, as decision 3 meant.
- **Validation:** `TestTerminalPanel`, whose stand-in terminal can now parse what is written (OSC to its handlers at
  the cursor, CSI skipped, lines filled, markers gone on reset): `seq 20` folded to its first and last 3 lines and
  the line, written once, blocks read again with their exit; a short output not offered; Copy output of the folded
  one giving all 20 lines; the prompt line's Unfold; Commands' "folded" and Unfold; the fold line unfolding while a
  write is held and late output following once; the alternate screen and output past 8 MiB refused with the note;
  the older block menu test unchanged. `tsc` and prettier clean.
- **Remaining:** phases 3 (lasting across reconnects) and 4 (contracts and docs).

## 2026-11-16 22:41 +09:00 — Lasting across reconnects

- **Change:** `web_shell.js`: each pane keeps `foldMemory`, per shell ID for the page's life, of folded blocks'
  `foldHash` (FNV-1a over the command line and the output bytes, with the output's length); folding adds, unfolding
  removes. When a connection's replay has been written, `refold` folds every block whose hash is remembered and
  writes the screen again once.
- **Reason:** plan phase 3.
- **Plan impact:** none.
- **Validation:** `TestTerminalPanel`: a block folded, a switch to another shell with the same output (not folded),
  back to the first with more output after the block (folded again, the rest kept), then unfolded and switched away
  and back (not folded) passes; `tsc` and prettier clean.
- **Remaining:** phase 4 (contracts and docs).

## 2026-11-16 22:43 +09:00 — Contracts and docs

- **Change:** WEB-58 in `contracts/05-web-assets-and-static-export.md` and its row in `contracts/README.md`.
  `TestTerminalFoldInBrowser` (`conformance/05-web/terminal_fold_browser_test.go`): bash prints `seq 101 140`, its
  Commands row's **Fold** folds it to 101–103, the line "⋯ 34 lines folded — click to unfold", and 138–140, and a
  click on that line brings 104 back. docs/WEB.md: a paragraph on folding after the blocks' menu.
- **Reason:** plan phase 4.
- **Plan impact:** the browser step is a test of its own, as the plan allowed, rather than a step in
  `TestTerminalPanelInBrowser`.
- **Validation:** `go test ./conformance -run 'TestContractStatus|TestConformanceLayout'` passes and `go vet
  ./conformance/05-web` is clean; the browser test skips here (no Chrome) and runs in CI; its script passes `node
  --check`. `scripts/check.sh` was not run (npm is broken here, and the full run is too heavy for the shared machine).
- **Remaining:** CI's run of `TestTerminalFoldInBrowser`.

## 2026-11-16 23:02 +09:00 — CI: the line count, the Fold button's place, and a held scroll

- **Change:** `foldInfo` counts what follows the last line feed as a line only when it holds text; the Fold button
  goes first among the prompt line's buttons, and first on a Commands row, so that Copy output stays at the right end
  (WEB-48). `holdScroll` (a command chosen in Commands, or folded) holds for half a second and then until xterm.js's
  scroll bar agrees with the line, up to three seconds.
- **Reason:** CI: `TestTerminalFoldInBrowser` showed "35 lines folded" with two kept lines after: bash and zsh started
  by rotari print OSC 7 after the output and before D, which counted as a 41st line. `TestTerminalPanelInBrowser`
  found Copy output 58 px from the right, the Fold button of the long command beside it. On macOS,
  `TestTerminalPanelInBrowser` found the view at line 42 rather than the long command's prompt at 9 after choosing it
  from Commands on a reloaded page: a late scroll event of the scroll bar after the half-second hold, it seems, as the
  earlier step of that test already waits for.
- **Plan impact:** none.
- **Validation:** `TestTerminalPanel` (the fold test's commands now print OSC 7 before D, as rotari's bash does, and
  fold to 14 lines; Commands' row reads Unfold, Copy command, Copy output) passes; CI to confirm the browser tests.
- **Remaining:** CI.
