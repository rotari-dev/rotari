# Work log: A right-click menu in the Web terminal

## 2026-11-15 03:20 +09:00 — Copy, Paste, and Select all

- **Change:** each pane's viewport listens for `contextmenu` (`openTerminalMenu`): without Shift, and with no mouse
  tracking asked for by the program, it focuses the pane and opens the panel's menu (`openPanelMenu`, which keeps the
  browser's from opening) with **Copy** (`term.getSelection()`, off without a selection, through `copyText` as "the
  selection"), **Paste** (`pasteClipboard`: `navigator.clipboard.readText()` given to `term.paste`, a note naming
  Ctrl+V and Cmd+V when the browser refuses, and another for an empty clipboard), and **Select all**
  (`term.selectAll()`). Shift+right-click, and a right click a program takes, are left to the browser and the
  program. The fake terminal gains `getSelection` and `selectAll`.
- **Reason:** plan phase 1, after the user accepted the plan with the open questions answered as proposed.
- **Plan impact:** none.
- **Validation:** `TestTerminalPanel` (the menu and the browser's kept from opening; Copy off and on, copying;
  Select all; Paste pasting, refused, and empty; Shift and mouse tracking getting no menu) passed; `tsc` clean;
  prettier unchanged. Not seen in a real browser here.
- **Remaining:** phases 2 (Cut and the block items) and 3 (contracts, docs, the browser step).

## 2026-11-15 04:00 +09:00 — Cut and the block items

- **Change:** the menu leads with **Cut**, on only when `cutPlace` finds the selection (`term.getSelectionPosition()`)
  wholly in the command line typed at the prompt: from `B` (not before it) to the line's end (the last wrapped row's
  text, or the cursor if further), a selection run past the text ending with it, as character indexes counted as WEB-50
  counts them. `cutSelection` copies through `copyText`, which now says whether it could, and on success moves the
  cursor to the selection's end (`moveToIndex`, split out of `moveByClick`: zsh's index when announced, else arrows),
  sends one `DEL` per character, clears the selection, and says "Cut the selection"; a failed copy sends nothing. A
  right click on a command's prompt or output (`blockAt` from `screenCell`) adds its **Copy output** and **Copy
  command**. The fake terminal gains `getSelectionPosition` and `clearSelection`.
- **Reason:** plan phase 2.
- **Plan impact:** none.
- **Validation:** `TestTerminalPanel` (Cut off for output and the prompt; `本a` cut from the end of `echo 日本abc` as
  two left arrows and two `DEL`s, copied, the selection cleared; a selection past the text; a failed copy sending
  nothing; zsh's index; the block's copies on output and on a prompt, none on a prompt with no command yet) passed;
  `tsc` clean; prettier formatted.
- **Remaining:** phase 3 (contracts, docs, the browser step).
