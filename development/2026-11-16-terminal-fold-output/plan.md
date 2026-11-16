# Plan: Folding a command's output in the Web terminal

Created: 2026-11-16
Status: Accepted by the user on 2026-11-16 ("go"), with the open questions as proposed. See [work-log.md](work-log.md).

## Goal

With `--allow-terminal`, a finished command's long output can be folded: its lines give way to a short summary in the
terminal, so that the commands before and after it are close together and the scroll bar is shorter, and unfolding
brings the output back as it was. This is one of the block features left out of the [Blocks
plan](../2026-11-14-terminal-blocks/plan.md) (non-goals), chosen on 2026-11-16 together with sticky command lines, which
gets a plan of its own; grouping blocks by directory was dropped.

## Existing implementation

- **Blocks** (WEB-48). The panel reads OSC 133 into blocks, each with its prompt's marker, its command line, and its
  output from `C` to `D`; a mark beside each prompt opens a menu (Copy output, Copy command, Select output, Type
  command again), a copy button shows on the prompt's line, and the **Commands** dropdown lists the blocks.
- **The screen is xterm.js's.** xterm.js 5.5 cannot hide lines: a buffer line can be decorated or overlaid, but not
  taken out of the scrollback while others stay. Folding has to change what is written.
- **Replay.** Each connection starts with the server's ring (the last 256 KiB of raw bytes, WEB-33): `open` resets the
  terminal and writes it, and the blocks are read again from its marks (`dropBlocks`, then the OSC handlers).
  Scrollback is 10,000 lines (`terminalScrollback`).

## Decisions

1. **Folding rewrites the screen from the bytes.** Each pane keeps the raw bytes it has written to its terminal
   since its last reset (the replay, then live output), up to 8 MiB, dropping the oldest, and notes where in them each
   block's output starts (`C`) and ends (`D`). Folding a block resets the terminal and writes those bytes again with
   that block's output replaced by a summary (decision 3); unfolding writes them again whole. Blocks, their marks, the
   scroll bar's failures, and links come back as on a replay. The scroll position is kept on the folded block's
   prompt. Output that arrives while the bytes are written again waits and follows.
2. **What can fold.** A block whose command has ended (`D`), whose output is wholly in the kept bytes, while the
   terminal is not in the alternate screen (`vim`, `less`). A running command's output does not fold.
3. **What a folded block shows.** Its prompt and command line as they were, then its first 3 and last 3 output lines,
   and between them one line the page writes, dim: `⋯ 1,234 lines folded — click to unfold`. An output of 10 lines or
   fewer is not offered to fold. (See open question 2.)
4. **How to fold and unfold.** The prompt mark's menu gets **Fold output** / **Unfold output**; the copy button on the
   prompt's line gets a fold button beside it; the summary line is a button (a decoration over it) that unfolds; the
   **Commands** dropdown's rows say "folded" and offer the same. Folding is by hand only. (See open question 1.)
5. **Copying a folded block** (Copy output, the copy buttons, the Commands dropdown) copies the whole output from the
   kept bytes, as text, not the summary. **Select output** unfolds first.
6. **Folds last while the page shows the shell.** A pane remembers its shell's folds by the block's command line and
   output, hashed; after a reconnect, a tab switch, or a link followed, the replay is folded again where the same
   command and output are found. Two identical blocks fold together. Folds are the page's alone: another tab or
   browser shows the shell unfolded, and the server keeps nothing.
7. **Width.** The bytes are written again at the pane's width now, so lines wrap as they would if printed now, as a
   replay already does.
8. **What it does not keep.** A terminal mode that the folded output itself left changed (a scroll region, a colour
   never reset) is lost while it is folded; the prompt, which the shell draws after the output, sets its own. The
   docs say so.
9. **Contracts.** A new ID, WEB-58, for folding (what can fold, what shows, how to unfold, copying, what lasts).

## Phases

1. **Kept bytes and rewriting.** Each pane's byte record, block offsets, folding and unfolding by rewrite, the summary
   line, and the scroll position. JSDOM tests: a block folded and unfolded, the bytes written each time, later output
   held and then written, a running block and the alternate screen refused, the 8 MiB limit.
2. **Controls and copying.** The menu items, the fold button, the summary's unfold button, the Commands dropdown, and
   copying a folded block. JSDOM tests for each.
3. **Lasting across reconnects.** Folds remembered by hash and applied to a replay. JSDOM test: fold, switch tabs,
   come back folded.
4. **Contracts and docs.** WEB-58, docs/WEB.md, and a `TestTerminalPanelInBrowser` step or a test of its own (CI only:
   `seq 500`, fold it, the prompt after it moves up, unfold).

## Tests and validation

- JSDOM in `TestTerminalPanel`; `tsc` and prettier; `go test ./internal/webui ./conformance/05-web` with Node 24.
- The browser test runs only in CI (no local Chrome).

## Non-goals

- Folding by the server, or for other viewers of the shell.
- Folding a running command's output, or output inside the alternate screen.
- Sticky command lines (a plan of its own) and grouping by directory (dropped).

## Open questions

1. **Folding long outputs by itself.** Proposed: by hand only (decision 4). The alternative folds any output over 200
   lines when its command ends, with a setting to turn that off.
2. **What a folded block keeps.** Proposed: the first and last 3 lines and a summary line (decision 3). The alternative
   keeps only the summary line.

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace.
