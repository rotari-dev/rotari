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

## 2026-11-16 16:07 +09:00 — Contracts and docs

- **Change:** WEB-46 in `contracts/05-web-assets-and-static-export.md` and its row in `contracts/README.md`: OSC 8
  links by scheme, pointing at and right-clicking one, and the prompt before any web address.
  `TestTerminalCommandDirsInBrowser` gains a step: bash prints, with `printf`, an OSC 8 link whose text is
  `open-me` and whose address is a percent-encoded `file:` path; pointing at it says it opens that file in the
  Table viewer. docs/WEB.md says web addresses ask first and how OSC 8 links open; docs/SECURITY.md that only
  `file:` links are asked about and that a web address is confirmed.
- **Reason:** plan phase 2.
- **Plan impact:** the right-click entries are in WEB-46 with the path link's menu rather than in WEB-51, which keeps
  the terminal's own menu; WEB-51 is unchanged.
- **Validation:** `go test ./conformance -run 'TestContractStatus|TestConformanceLayout'` and `go test
  ./conformance/05-web -run 'TestTerminalCommandDirsInBrowser|TestTerminalOutputLinks'` pass; the browser test skips
  here (no Chrome) and runs in CI; its script passes `node --check`, and the command it types was run through bash
  to check the bytes it prints. `scripts/check.sh` was not run (npm is broken here, and the full run is too heavy for
  the shared machine).
- **Remaining:** CI's run of `TestTerminalCommandDirsInBrowser`.
