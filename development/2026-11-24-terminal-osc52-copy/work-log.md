# Work log: Copying from programs with OSC 52

Plan: [plan.md](plan.md)

## 2026-11-25 15:46:13 +0900 - `41e37982` - Phase 1, tmux and Neovim

- **Change:** the plan only. tmux 3.2a and Neovim 0.12.6 ran on ptys of their own, with no user configuration and a tmux
  server of its own, answered as xterm.js 5.5.0 answers.
- **Reason:** Phase 1 decided whether the rest was done.
- **Plan impact:** answering a query as empty, proposed at first, breaks putting in Neovim, and was dropped. The user
  decided that a query is never answered, and that a copy while the setting is off is told once per page.
- **Validation:** the plan's tables.
- **Remaining:** why `load-buffer -w` of 50,000 bytes showed no OSC 52.

## 2026-11-25 15:58:27 +0900 - `9a52297c` - The setting, the handler, and the documents

- **Change:** `setProgramsCopy`, `programCopy`, `takeCopy`, `offerCopy`, and `readProgramCopy` in `web_shell.js`, and
  **Let programs copy** in the Options menu. WEB-33's note, WEB, and SECURITY.
- **Reason:** Phases 2 and 3.
- **Plan impact:** text with control characters is refused, which the plan did not say. The Neovim configuration first
  recorded, which pastes from the unnamed register, was misread as working: `"+yy"+p` puts nothing with it. The one in WEB
  keeps what it last copied, and was measured for ten kinds of put. No contract ID of its own, as for the font.
- **Validation:** of 21 faults put into the code, 19 were caught at first, and the two about hidden pages once tests for
  them were added. `go vet ./...`, the type check, and `go test` of `internal/webui`, `conformance`, and
  `conformance/05-web` passed, with Go 1.25.8 and Node 24. Not run: the browser tests (no Chrome here), `go test -race`,
  pre-commit, the other packages, and `scripts/check.sh`.
- **Remaining:** the checks by hand in Chrome, Firefox, and Safari: no real clipboard has been written yet.
