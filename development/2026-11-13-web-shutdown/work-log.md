# Work log: Stopping a background rotari web

## 2026-11-13 15:23 +09:00 — `a780193f` Stop a rotari web with --shutdown or from Settings

**Change:**

- `cmd/rotari/web_shutdown.go` (new)
  - `parseWebShutdownFlags` accepts only `--shutdown`, `--yes`, and `--config`. It refuses `--yes` alone, any
    other option given with `--shutdown` (naming it), and positional arguments.
  - `shutdownDetachedWeb`:
    1. Finds the host's detached server (`webdetach.Find`). With none, it says so and exits 0.
    2. Lists busy shells (`webShellLines`), or one line saying why that could not be checked, and asks
       (`confirmWebShutdown`): y/N at a terminal, `--yes` required without one, and no question when nothing runs.
    3. Checks that the record still names the same PID, then sends SIGTERM.
    4. Polls the lease for up to 30 s and says it stopped. Running out of time is an error naming the PID and log.
- `internal/terminal/outside.go` (new)
  - `ServerShells(pid)` treats as a shell any child of the server that leads its own session on a terminal, so
    launches are left out.
  - The foreground command is the terminal's foreground group when that is not the shell's own group: its leader's
    command line, else a member's, else "a command". The other descendants are background processes.
  - `descendantsOf` moved here, and `Session.Descendants` uses it.
- `internal/terminal/process_linux.go` and `process_darwin.go`: `processes()` also reads the process group, whether
  the process leads its session, whether it has a terminal, and that terminal's foreground group (`/proc/<pid>/stat`;
  `ps -o pgid,tpgid,tty,stat`). The new fields of `Process` are unexported, and the preview API is unchanged.
- `internal/webui/shutdown.go` (new) serves `POST /_server/shutdown` with `Options.Shutdown`. It answers 404 when
  that is nil, 405 for other methods, 403 without job control, and 415 without a JSON content type. Otherwise it
  answers 202, flushes, and calls `Shutdown` in a goroutine. It is registered in `site.handler` beside the terminal
  routes.
- `cmd/rotari/web.go`
  - `serveWeb` gives `options.Shutdown` a `sync.Once` channel close, and its signal goroutine waits on either the
    signal or that channel, so every stop takes the same path.
  - `--shutdown` and `--yes` are parsed, routed first in `cmdWeb`, and counted among the live-server options.
- `cmd/rotari/web_detach.go`: the help line and the different-options error now point to
  `rotari web --shutdown` instead of `kill`.
- Web UI:
  - `livePage.stoppable` (job control and `Shutdown` set) renders `stopServerControl` through
    `__ROTARI_STOP_SERVER__` in both templates. The button and its status line are styled in `web_shell.css`.
  - In `web_shell.js`, `confirmSignOut`'s asking became `confirmEndingShells`, which sign-out and the new
    `stopServer` share. Stop server still asks when nothing runs, or without the terminal. It posts JSON, shows
    "Not stopped: ..." and re-enables the button on a refusal, and on success shows "The server stopped." and
    rewrites the disconnect banner.
- Tests:
  - `TestServerShellsSeesWhatRunsFromOutside` covers an idle shell, a background sleep, a foreground sleep, and a
    child that is not a shell.
  - `TestShutdownRoute` covers the route's guards and when the page offers the button.
  - `TestConfirmWebShutdown` covers the prompt.
  - `TestTerminalPanel` (JSDOM) covers the button's questions, the refusal, success, an idle page, and a page
    without the terminal. Its fixture pages are now stoppable, and its fake server answers `/_server/shutdown`.
  - Conformance: `TestWebShutdownStopsTheDetachedServer` (WEB-40) and `TestServerShutdownRoute` (WEB-41). WEB-40's
    detach test expects the new wording.
  - `pairweb` samples `--shutdown` and `--yes` and expects them refused beside `--static-dir`. The web flag
    inventory, the golden schema, the CLI reference, and the Python schema were regenerated.
- Docs and contracts:
  - WEB-40 note and row: `--shutdown`. New WEB-41 note and row.
  - `docs/WEB.md`: Settings list and "Stopping it".
  - `docs/SECURITY.md`: `--shutdown`, and who may press Stop server and why a cross-site form cannot.
  - `docs/ARCHITECTURE.md`: the shutdown flow.

**Reason:** The plan's goal: stop a detached server politely from the CLI and the Web UI instead of with `kill`.

**Plan impact:** All proposals implemented. One departure, recorded in the plan: Stop server asks even when nothing
runs, since the page loses its server. The plan's question was decided: job control is enough, with no extra token.

**Validation:**

- `go vet ./...`, `gofmt -l`, and `tsc -p internal/webui/tsconfig.json` (with the VS Code server's node v24): clean.
- `go test ./cmd/rotari ./internal/... ./conformance/...` passed in full, with `taskset -c 0-7`.
- The new shutdown conformance tests passed `-count=15`. An earlier version raced, because the mark printed before
  `sleep` started; the foreground command now prints the mark itself.
- By hand, with an isolated master directory: no server, refused combinations, an idle server stopped without a
  terminal, and the route refusing without a token (403), a form body (415), and GET (405), then stopping the
  server with 202 and removing its record.
- `generate_cli_reference.py --check` and `sync_readme.py --check`: clean.
- Not run: macOS (the `ps` parsing in `process_darwin.go` is untested here), `scripts/check.sh` as a whole (npm
  is broken here), and pre-commit (not installed).

**Remaining:** The next plans are [web-client](../2026-11-13-web-client/plan.md) and then
[terminal-idle-off](../2026-11-13-terminal-idle-off/plan.md). Check the macOS `ps` columns on a Mac.
