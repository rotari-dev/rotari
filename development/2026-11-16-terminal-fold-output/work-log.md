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
