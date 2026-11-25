# Work log: Choosing the Web terminal's font

Plan: [plan.md](plan.md)

## 2026-11-25 15:00:17 +0900 - `beb78ef3` - The font and the Options button

- **Change:** `fontFamily`, `savedFont`, `fontLooks`, `applyFont`, and `openFontDialog` in `web_shell.js`, the bar's
  **Options** button and its menu, and the dialog's CSS. WEB-33's note and WEB.
- **Reason:** the whole plan.
- **Plan impact:** no contract ID of its own, since no conformance test checks it. Format characters, such as those that
  reverse text, are refused with control characters, which the plan did not name.
- **Validation:** `TestTerminalPanel` covers the name, storage, panes, sticky line, dialog, and late fonts. Of 15 faults
  put into the code, 14 were caught at first; the sticky line's was not, and is since its check was added. The first run
  found the Options button disabled, as bar buttons start. The type check, `go vet ./...`, and `go test` of
  `internal/webui`, `conformance`, and `conformance/05-web` passed, with Go 1.25.8 and Node 24; after a change of
  formatting alone, only the font's and sticky line's tests ran again. Not run: the browser tests (no Chrome here),
  `go test -race`, pre-commit, the other packages, and `scripts/check.sh`.
- **Remaining:** the checks by hand, with a real font: nothing here has measured one, since JSDOM has no canvas. The
  Options and Font... parts have not been in a browser.

## 2026-11-25 15:23:53 +0900 - `b6870e87` - The list

- **Change:** the dialog's text box became a list: `knownFonts`, `measureFont`, `fontPresent`, and `fontMonospaced` in
  `web_shell.js`, **List installed fonts**, and **Other...**. WEB-33's note and WEB.
- **Reason:** the user could not be expected to type a font's name.
- **Plan impact:** decision 9; listing the installed fonts is no longer a non-goal.
- **Validation:** of 16 faults put into the code, 11 were caught at first and three more once the tests were
  strengthened. Two change nothing that can be seen: a second check that the dialog is still open, and choosing
  **Other...** again, which the moved option already does in JSDOM. `go vet ./...`, the type check, and `go test` of
  `internal/webui`, `internal/terminal`, `conformance`, and the terminal tests of `conformance/05-web` passed on
  `9ddaaf63`; the whole of `conformance/05-web` had passed before that rebase. Not run: the browser tests (no Chrome
  here), `go test -race`, pre-commit, the other packages, and `scripts/check.sh`.
- **Remaining:** the checks by hand, above all the measuring and `queryLocalFonts`, which only stand-ins have answered.
