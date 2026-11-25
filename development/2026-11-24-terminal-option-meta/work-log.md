# Work log: Option as Meta in the Web terminal

Plan: [plan.md](plan.md)

## 2026-11-25 15:23:53 +0900 - `8550594b` - The setting

- **Change:** `setOptionMeta` and `onMac` in `web_shell.js`, and **Use Option as Meta** in the Options menu. WEB-33's note
  and WEB.
- **Reason:** the whole plan, but for the Keys item of WEB.
- **Plan impact:** no contract ID of its own, as for the font. The Keys item stays as it was, since which keys each browser
  keeps has not been checked by hand.
- **Validation:** `TestTerminalPanel` covers what is kept, the platforms, every pane and later ones, and storage refused.
  Each of nine faults put into the code was caught. The platform names are the vendored xterm.js's own. See the font's log
  for the runs after the rebase.
- **Remaining:** the checks by hand on a Mac, where nothing of this has run, and the Keys item.
