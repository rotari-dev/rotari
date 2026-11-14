# Work log: Recent directories in the Web terminal

## 2026-11-14 02:39 +09:00 — The Keeper keeps each group's recent directories (`c8d8d85d`)

- **Change:** `internal/terminal` `Keeper` gains `History(group)` and `DirEntry`. `Attachment.Write` marks a shell
  when input holds a carriage return or line feed; a new shell starts marked. `deliver` arms a 150 ms timer
  (`dirSettle`) on each output that finds a marked shell at its prompt; when it fires and the shell is still at
  its prompt, `readDir` reads `Session.Dir` outside the lock and, if the shell has moved, puts the directory at
  the front of its group's history, deduplicated and cut to 20 (`historyLimit`). A serial keeps a read begun before
  another Enter from clearing the mark. `EndAll` (sign-out, the idle limit, `Close`) forgets every group's history;
  `finish` stops the timer. The `internal/terminal` row of `docs/ARCHITECTURE.md` mentions it.
- **Reason:** plan phase 1, decisions 4 and 5: history without shell setup, kept in memory only.
- **Plan impact:** phase 1 done. Found while writing it: the first output after Enter is the echo of the line,
  before the command runs, so reading at once would see the old directory; waiting for the output to settle at
  the prompt was chosen over reading on every output. A directory read failure is logged and left pending for the
  next prompt.
- **Validation:** `go test -race ./internal/terminal` passed, and `-run History -count=5` with `-race` passed
  (both under `taskset -c 0-3`). `go test ./internal/webui ./cmd/rotari` passed with Node 24 and `NODE_PATH` set
  (without them the JSDOM tests fail to start in a fresh workspace, unrelated to this change). `go vet` and `gofmt`
  clean.
- **Remaining:** phases 2–4 (the API with run candidates, the panel, contracts and docs). Nothing serves the
  history yet, so no user-visible behavior changed.

## 2026-11-14 02:49 +09:00 — A shell's start enters the history at once (`aff99307`)

- **Change:** `Keeper.Start` enters the shell's start directory (resolved with `filepath.EvalSymlinks`, or the
  working directory when none is given) instead of waiting for its first prompt; `readDir` and `Start` share
  `enterDir`. If the start cannot be resolved, the first prompt reads it as before.
  `TestHistoryKeepsTheStartOfAShellMovedAtOnce` covers it.
- **Reason:** phase 2's test typed `cd` straight after starting a shell, and the start directory never entered the
  history: the line ran before the first prompt settled. A person typing quickly would lose it the same way.
- **Plan impact:** decision 4 gains a note that the start is entered when the shell starts.
- **Validation:** `go test -race ./internal/terminal` passed (under `taskset -c 0-3`).
- **Remaining:** None for phase 1.

## 2026-11-14 02:49 +09:00 — `/_terminal/shells/ID/dirs` (`533939f4`)

- **Change:** `internal/webui/terminal_dirs.go` serves `GET /_terminal/shells/ID/dirs`, routed in
  `serveTerminalShell` beside `preview`. It answers the group's `history` from `Keeper.History`; for a project,
  `runs` from the newest 20 runs in `state.RunsNewestFirst` order, deduplicated to five directories, each with its
  run ID and `started_at` from the run ID; for a basedir's own shells, `workspace` from `terminalBaseDirStart`.
  Every directory carries `not_utf8`, `missing`, and `stat_error`. Tests in `terminal_dirs_test.go`;
  `terminalServerAt` lets a test choose the server's basedir. `docs/ARCHITECTURE.md` lists the route.
- **Reason:** plan phase 2, decisions 6 and 7.
- **Plan impact:** phase 2 done. Decision 7 changed while implementing: the route sits under the shell, as
  `preview` does, rather than `/_terminal/dirs?shell=ID`; a run that cannot be read is listed with its `error`, and
  runs that cannot be listed give `runs_error` beside the history, rather than failing the whole request.
- **Validation:** `go test -run TestTerminalDirs -count=3 ./internal/webui` passed; `go test ./internal/...
  ./cmd/... ./conformance/...` passed with Node 24 and `NODE_PATH` (under `taskset -c 0-7`); `go test
  ./internal/doclinks ./internal/archtest` passed after the ARCHITECTURE edit. Browser tests skipped (no Chrome
  here).
- **Remaining:** phases 3 (the panel) and 4 (WEB-45, conformance, docs). No page uses the route yet.

## 2026-11-14 03:03 +09:00 — The panel's Recent directories dropdown (`d56e86fd1d60`)

- **Change:** `internal/webui/assets/web_shell.js` adds a **Recent directories** icon to the bar (after **Launch
  here**, disabled with no shell in the focused pane, as it is) and makes the Explorer's path line open the same
  dropdown. The dropdown (`openDirs`) has a filter field and **Recent**, **Runs**, and **Workspace** sections from
  `/_terminal/shells/ID/dirs`; arrow keys skip rows that cannot be chosen, Enter or a click chooses, Escape closes and
  gives the keys back to the shell, and the button closes what it opened. Choosing (`typeCd`) asks the shell's
  `preview` first and types `cd` plus `shellWord(dir)` through `sendToTerminalAndSay` only when no command holds the
  foreground; otherwise, or when that cannot be told, a note says why and nothing is typed. Row actions: **Show in
  the Explorer** (root there, Follow off), **Open in a new terminal tab** (`newShell` with `dir`), **Copy the
  path**. A missing directory keeps only Copy; a non-UTF-8 name or a run with an `error` offers nothing. A refused
  request shows the reason in the list. Styles in `web_shell.css`. JSDOM coverage at the end of
  `TestTerminalPanel`, with a fake for `/dirs`.
- **Reason:** plan phase 3, decisions 8–10.
- **Plan impact:** phase 3 done. Found while testing: the keys must be attached before the list loads, or Escape
  could not close a dropdown showing a refusal.
- **Validation:** `tsc -p internal/webui/tsconfig.json` clean; prettier (from `node_modules`) applied to the JS
  and CSS. `go test ./internal/webui ./conformance/05-web` passed with Node 24 and `NODE_PATH` (under `taskset -c
  0-7`). Not tried in a real browser: none here, and `TestTerminalPanelInBrowser` is extended in phase 4.
- **Remaining:** phase 4: WEB-45 and its conformance test, a `TestTerminalPanelInBrowser` step, and the docs
  (WEB.md, FAQ, SECURITY.md). Until then the feature is undocumented on local `main`.

## 2026-11-14 03:17 +09:00 — WEB-45, conformance, and docs (`e5202dc26632`)

- **Change:** WEB-45 in `contracts/05-web-assets-and-static-export.md` and its row in `contracts/README.md`.
  `conformance/05-web/terminal_dirs_test.go` (`TestTerminalRecentDirectories`) runs the binary: a finished run,
  a project shell that `cd`s, the history and the run's directory from `/dirs`, a second shell sharing the history,
  404 for a shell not running and without the terminal, 405 for POST. `TestTerminalPanelInBrowser` (WEB-33) gains a
  step: `cd` twice, open the dropdown, choose the older directory, and run the typed `cd`. `docs/WEB.md` (Terminal),
  `docs/FAQ.md` (why a job runner has a terminal), and `docs/SECURITY.md` (the Web terminal) describe the list and
  that its directories stay in memory.
- **Reason:** plan phase 4.
- **Plan impact:** the plan is implemented.
- **Validation:** `go test ./conformance/05-web -run TestTerminalRecentDirectories` and `go test ./conformance -run
  TestContract` passed; `go vet ./...` clean and `go test ./...` passed with Node 24 and `NODE_PATH` (under
  `taskset -c 0-7`). The browser step was not run: there is no Chrome here, so `TestTerminalPanelInBrowser` skipped;
  it runs in CI.
- **Remaining:** none in this plan. Watch CI for the browser step after a push. Blocks and links for rotari IDs,
  proposed with this feature, have no plans yet.

## 2026-11-14 10:50 +09:00 — A directory left before its prompt settled (`73ae4483459a`)

- **Change:** `Attachment.Write` calls `readDir` before passing on input with Enter, so a pending read happens at
  once when the shell is at its prompt. `TestHistoryKeepsADirectoryLeftAtOnce` types the second `cd` as soon as the
  first one's output shows. WEB-45's note says so.
- **Reason:** CI's `TestTerminalPanelInBrowser` timed out at "the recent directories": the step types the second
  line as soon as `one-4` shows, within `dirSettle` (150 ms) of the prompt; the second command's output kept
  resetting the timer, so the shell was read only in `dir two` and `dir one` never entered. Reproduced over the
  WebSocket without a browser; the new unit test failed three times in three before the fix. Pasted lines hit the
  same thing.
- **Plan impact:** none to the decisions; reading at the prompt still needs no shell setup. On darwin, where
  finding the directory runs a program, the extra read happens only for Enter while a read is pending.
- **Validation:** the new test passed 5 times in a row with the other history tests; `go test -race
  ./internal/terminal`, and `go test ./conformance ./conformance/05-web -run 'TestTerminal|Contract'` passed. The
  browser step itself runs only in CI.
