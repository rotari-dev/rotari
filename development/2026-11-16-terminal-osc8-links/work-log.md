# Work log: OSC 8 hyperlinks in the Web terminal

## 2026-11-16 16:05 +09:00 — The panel

- **Change:** `web_shell.js`: each pane's terminal gets a `linkHandler` (`osc8Links`, with `allowNonHttpProtocols` so
  that xterm.js keeps `file:` and other links rather than dropping them). `addressLink` reads a link by scheme:
  `http:`/`https:` open through `openWebAddress`, which asks `confirm()` with the address before a `noopener` tab;
  `file:` is decoded by `fileURL` (now shared with OSC 7's `readDirMark`) and asked of the links route as one
  absolute candidate with its host, its answer opening as a printed path does; anything else, or a path that does not
  decode or is not found here, opens nothing and shows a note saying why. Hover puts the address in the title at once
  and what it opens once answered; a right click on a path gives the Explorer's menu, otherwise **Open link** (only for
  a web address) and **Copy link address**. Plain web addresses (the web links addon) now go through
  `openWebAddress` too.
- **Reason:** plan phase 1, after the user accepted the plan with a prompt for every web address (open question 1
  answered the other way) and other schemes copy-only.
- **Plan impact:** decision 2 (the prompt) added; open questions settled.
- **Validation:** `TestTerminalPanel` (a plain web address refused and then opened, with the question; OSC 8: the
  handler and `allowNonHttpProtocols`; `https:` asked first; a `file:` file opening the Table viewer and a `file:`
  directory, percent-encoded, a tab there, with the request's `dir` and `host` and no question; another host's
  missing path, an undecodable path, and `javascript:` opening nothing with their notes; titles; the menus for a
  path, a web address with Copy link address, a `mailto:` with Open link disabled, and off a link) passes; `tsc` and
  prettier clean.
- **Remaining:** phase 2 (contracts, docs, and the browser step).
