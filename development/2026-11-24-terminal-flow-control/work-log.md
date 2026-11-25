# Work log: Fast output in the Web terminal's page

Plan: [plan.md](plan.md)

## 2026-11-24 18:14:11 +0900 - `7eaa49e6` - Phase 1, the reproduction

- **Change:** the plan and ISSUES.md. The reproducer ran in CI from a temporary branch, since deleted, and is not in
  `main`.
- **Reason:** Phase 1 decided whether the rest was done.
- **Plan impact:** the loss needs a hidden page or a very large replay, so the acknowledgements first planned gave way
  to a change to the page alone.
- **Validation:** the CI runs the plan lists. A client in Node had shown losses that a visible Chrome page did not.
- **Remaining:** none.

## 2026-11-24 22:03:16 +0900 - `c0a298a2` - The queue

- **Change:** `giveOutput` in `web_shell.js`, and the rewrite without its own list of waiting output. The stand-in for
  xterm.js in `terminal_panel_test.go` parses in order and refuses more than 50 MB unparsed, as xterm.js does.
- **Reason:** Phase 2.
- **Plan impact:** decisions 1 to 4 and 7. Output during a rewrite, which the plan had only from reading the code, was
  reproduced by the new test.
- **Validation:** the new test failed before the change with `write data discarded`, and passed after. Each of seven
  faults put into the change was caught. `internal/webui`, `internal/terminal`, and the terminal tests of
  `conformance/05-web` passed, and the type check, with Go 1.25.8 and Node 24.
- **Remaining:** kept out of `main` until the limit.

## 2026-11-24 22:12:44 +0900 - `b330152e` - The limit, contracts, and docs

- **Change:** `fallBehind` and `sayFellBehind` in `web_shell.js`; WEB-33, WEB-34, and the row of WEB-33; WEB; ISSUES.
  A test for moving to another project with output waiting, which phase 2 lacked.
- **Reason:** Phases 3 and 4.
- **Plan impact:** decisions 5 and 6. A visible page reconnects after about a second, by the same way as any other,
  rather than at once. A very large replay with very fast output can start over (the plan's risks).
- **Validation:** each of ten faults put into the change was caught. `go vet ./...`, the type check, and `go test` of
  `conformance`, `conformance/05-web`, `internal/webui`, and `internal/terminal` passed, with Go 1.25.8 and Node 24.
  Not run: the browser tests (no Chrome here), `go test -race`, pre-commit, the other packages, and
  `scripts/check.sh`. Prettier 3.9.8 wants two places in `web_shell.js` that were there before.
- **Remaining:** the browser test of the plan (below), and the checks by hand.

## 2026-11-24 23:45:49 +0900 - `81482c4c` - The browser test

- **Change:** `TestTerminalFastOutputInBrowser` (a hidden page, and a full 64 MiB replay); WEB-33, its row, ISSUES, and
  the plan's results after the change.
- **Reason:** the plan's tests, and the change had not been in a browser.
- **Plan impact:** none to the design. A slowdown suspected from the first run was not there: its `burst` scenario could
  not see its own end marker, and a hidden page falls behind without the change too. 2 MiB pieces, tried for it, were
  dropped (`8405b980`, not in `main`).
- **Validation:** in CI, with 256 KiB and with 2 MiB pieces, three runs each on Ubuntu and macOS passed; the replay
  scenario failed on both without the change. The committed test differs from what ran only in shorter waits, one figure
  less, and no burst; that form has not run in Chrome. `go vet` of the package and `go test ./conformance` passed here.
- **Remaining:** the checks by hand. The committed test's first CI run, for `52347ad5` on 2026-11-25, passed on Ubuntu
  and macOS.
