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
