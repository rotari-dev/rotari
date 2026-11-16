# Plan: The command's line kept at the top of the Web terminal while its output scrolls

Created: 2026-11-16
Status: Accepted by the user on 2026-11-16 ("go"), with the open question as proposed. See [work-log.md](work-log.md).

## Goal

With `--allow-terminal`, while you scroll through a command's output and its prompt has gone above the top of the
pane, a line at the top of the pane keeps saying which command printed what you see, as VS Code's terminal sticky
scroll and Warp's block headers do. Clicking that line scrolls back to the command's prompt. This is one of the block
features left out of the [Blocks plan](../2026-11-14-terminal-blocks/plan.md), chosen on 2026-11-16 with [folding
output](../2026-11-16-terminal-fold-output/plan.md).

## Existing implementation

- **Blocks** (WEB-48): each block has its prompt's marker, its command line (`block.command()`, from `C;cmdline_url=`
  or the screen), its output's start and end, and its state (`blockState`: running, ok, failed, unknown), shown by the
  mark beside its prompt. `blockAt(pane, line)` finds a buffer line's block; `goToBlock` scrolls to a prompt and
  lights it up.
- **Scrolling.** `term.onScroll` tells when the view moves; `term.buffer.active.viewportY` is the top line in view.
  Folding (WEB-58) writes the screen again, which moves blocks' markers but keeps the blocks.
- **The pane's right-click menu** (WEB-51) has Cut, Copy, Paste, Select all, the cursor's moves, and a block's copies.

## Decisions

1. **When it shows.** The line shows while the top line in view belongs to a block's output (`blockAt` of
   `viewportY`, at or after the line where its output starts) and that block's prompt is above the view. It goes when
   the prompt comes back into view, at a prompt with no output, in the alternate screen (`vim`, `less`), and for a
   shell that marks no commands. A running command's output counts, so a long build's output keeps saying it.
2. **What it shows.** Over the pane's first row, in the terminal's font and colours, with a thin line below it: the
   block's mark (the same dot and colour as beside its prompt), then `$ ` and its command line, the first line only
   with `…` after a command on several lines, cut with `…` at the pane's width. Hovering it says the whole command
   and how it ended, as the mark's title does. It covers the row under it, as VS Code's does.
3. **Clicking it** goes to the command's prompt (`goToBlock`), where the line then goes away. It takes no text
   selection and passes no click to the shell.
4. **Updates.** It is drawn again when the view scrolls, when a block's state changes (a running command ending), when
   the pane is resized or its font changes, and after the screen is written again (a replay, a fold).
5. **Turning it off.** On by default. The pane's right-click menu gets a checked item, **Keep the command at the
   top**, which turns it off and on for every pane, remembered by the browser (`localStorage`). (See open question.)
6. **Contracts.** A new ID, WEB-59, for the line (when it shows, what it shows, clicking it, turning it off).

## Phases

1. **The line.** Its element in each pane's viewport, when it shows and what it shows, clicking it, and its updates.
   JSDOM tests: scrolled into a block's output (shown, with its mark and command), its prompt in view (hidden), a
   prompt without output, the alternate screen, a command on several lines, a running command ending (the mark
   changes), clicking it (scrolls to the prompt), a fold written again.
2. **Turning it off, contracts, and docs.** The menu item and its remembered setting; WEB-59; docs/WEB.md; a browser
   test (CI only: `seq 200`, scroll to the middle, the line saying `seq 200`, a click taking the view to its prompt).

## Tests and validation

- JSDOM in `TestTerminalPanel`; `tsc` and prettier; `go test ./internal/webui ./conformance/05-web` with Node 24.
- The browser test runs only in CI (no local Chrome).

## Non-goals

- More than one line (the nested commands of an inner shell each on a line of their own).
- Showing the prompt as the shell drew it (its colours, its directory); the line shows the command, not the prompt.
- Grouping blocks by directory (dropped).

## Open question

1. **Where to turn it off.** Proposed: a checked item in the pane's right-click menu (decision 5). The alternative is no
   switch at all, as the line is small and goes away when the prompt is in view.

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace.
