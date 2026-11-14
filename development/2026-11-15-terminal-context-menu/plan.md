# Plan: A right-click menu in the Web terminal

Created: 2026-11-15
Status: Accepted by the user on 2026-11-15, with the open questions answered as proposed; phases 1 and 2 are done and phase 3 (contracts and docs) is next.
See [work-log.md](work-log.md).

## Goal

With `--allow-terminal`, right-clicking in a pane opens the panel's own menu: **Copy** the selection, **Paste** the
clipboard, **Cut** a selection inside the command line being typed at the prompt, **Select all**, and, on a command's
block, that command's **Copy output** and **Copy command**. Today the browser's own menu opens there, whose Copy and
Paste work but which offers nothing of the terminal's.

This is the second of the two features proposed on 2026-11-15, after [click to
move](../2026-11-15-terminal-click-to-move/plan.md), whose way of moving the cursor Cut builds on.

## Existing implementation

- **xterm.js 5.5**, on a right click, moves its hidden textarea under the pointer and fills it with the selection, so
  that the browser's menu can copy it and paste into it; on macOS (`rightClickSelectsWord`) it first selects the word
  under the pointer. Pasting goes through `term.paste`, which brackets the text when the program asked for bracketed
  paste.
- **Panel menus.** `openPanelMenu` draws the tab, panel position, and mark menus, with labels, actions, and whether
  each can be chosen; `copyText` copies and says so.
- **Blocks and click to move** know whether the shell is at its prompt, where the command line starts (`B`), how to
  count characters in it, and how to move the cursor to an index (zsh's widget, WEB-49) or by arrows (WEB-50).
- **Clipboard.** `navigator.clipboard.readText` needs a secure context, which the terminal already requires (HTTPS or
  a loopback connection), and the browser asks the user once before the page may read; `writeText` needs none.

## Decisions

1. **The menu.** A right click in a pane, outside the alternate screen or not, opens the panel's menu by the pointer
   and keeps the browser's from opening; **Shift+right-click** still opens the browser's, for its spelling, search, or
   extensions. A program that asked for mouse tracking (`vim` with `mouse=a`, `htop`) gets the right click as today,
   and no menu.
2. **Copy** copies the terminal's selection (`term.getSelection()`), and is off without one. **Select all** selects the
   buffer (`term.selectAll()`).
3. **Paste** reads the clipboard and gives it to `term.paste`, as Ctrl+V or the browser's Paste would: bracketed when
   the program asked, never with an Enter of its own. When the browser refuses to let the page read the clipboard, a
   note says to use Ctrl+V (Cmd+V on macOS), which needs no permission.
4. **Cut** is offered only when the selection lies wholly within the command line being typed at the prompt (from
   `B` to the line's end, as WEB-50 counts it). It copies the selection and then removes it from the line by moving
   the cursor to the selection's end (zsh's index, or arrows) and sending one Backspace (`DEL`) per character. If the
   copy fails, nothing is removed, so the text is never lost. Elsewhere Cut is off, since output cannot be cut.
5. **Block items.** Right-clicking on a command's prompt or output (the block `blockAt` finds) adds that command's
   **Copy output** and **Copy command**, as its mark's menu has them.
6. **Nothing reaches the server** but what typing would send. No route, option, or Keeper change; the contract is a
   new WEB-51.

## Phases

1. **The menu with Copy, Paste, and Select all.** The `contextmenu` handling on each pane's viewport, Shift for the
   browser's, mouse tracking left alone, the clipboard read and its refusal. JSDOM tests with a fake clipboard.
2. **Cut and the block items.** The selection's place in the command line (`term.getSelectionPosition()` against `B`
   and the line's end), the move and the Backspaces, nothing removed on a failed copy; the block's items. JSDOM tests.
3. **Contracts and docs.** WEB-51 and its row; a `TestTerminalPanelInBrowser` step (in the bash tab of the click step:
   type a line, select part of it, Cut from the menu, run it, and see the part gone; the clipboard granted to the page
   by CDP's `Browser.grantPermissions` if the helper allows it, else Paste checked as far as the refusal's note); the
   Terminal section of [docs/WEB.md](../../docs/WEB.md).

## Tests and validation

- JSDOM: the menu opens on a right click and the browser's does not; Shift+right-click and mouse tracking get no menu;
  Copy with and without a selection; Select all; Paste through `term.paste`, and the note when the clipboard refuses;
  Cut offered only for a selection inside the command line, sending the move and the right number of Backspaces (a
  wide character once), zsh's index when announced, and nothing when the copy fails; the block items on a block and
  not elsewhere.
- Browser (CI only): the step in phase 3.
- `go test ./internal/webui ./conformance/05-web` with Node 24, `tsc`, and prettier.

## Non-goals

- Cutting output, or anything in `vim`, `less`, or other full-screen programs.
- A menu item that runs anything, or Paste that presses Enter.
- Changing what Ctrl+C, Ctrl+V, or the browser's keys do.

## Settled questions

All three were accepted as proposed on 2026-11-15.


1. **The browser's menu.** Proposed: Shift+right-click opens it. The alternative is to drop it in the terminal
   entirely.
2. **Block items in the menu.** Proposed: yes, on a command's prompt or output. The alternative is to leave them to the
   mark's menu and the Copy output button.
3. **Cut when the copy fails.** Proposed: remove nothing, and say the copy failed. The alternative is to remove the text
   anyway, as a terminal's own Cut would not exist to compare with.

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace and is rebased onto `main` before integrating.
