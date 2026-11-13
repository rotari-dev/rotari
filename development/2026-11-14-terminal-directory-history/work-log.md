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
