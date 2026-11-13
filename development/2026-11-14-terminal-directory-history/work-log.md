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
