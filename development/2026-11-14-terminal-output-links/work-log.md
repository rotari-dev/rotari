# Work log: Links for run IDs, job IDs, and paths in the Web terminal

## 2026-11-14 03:38 +09:00 — `/_terminal/shells/ID/links` (`9f95030b27e9`)

- **Change:** `internal/webui/terminal_links.go` serves `POST /_terminal/shells/ID/links`, routed beside `dirs` and
  `preview`. A JSON body (415 otherwise; unknown fields 400) of at most 64 candidates of at most 4 KiB each (400)
  is answered with the candidates that are links, deduplicated, in the order asked. A run ID (`^[0-9]{8}-[0-9]{6}-
  [0-9a-f]{8}$`) is found in the shell's project, then the other projects of its basedir, or in every basedir for a
  shell of no project, by its run directory; a job ID (nine hex digits, or an array task's `-N`) through
  `resolve.JobInRun` in the project's newest 20 runs and then `resolve.JobInQueue` on its queue; a word with `/`
  through `os.Stat`, relative to `Keeper.Dir`, with `:LINE[:COL]` left off. `dir_error` and `problems` say what
  could not be read while looking. `site.basedirLinkPrefix` now builds the `/_basedir/ID` prefix for both these
  links and `terminalShellResponse.Link`. `terminalServerWith` lets tests pass `Options`. ARCHITECTURE lists the
  route.
- **Reason:** plan phase 1, decisions 1–5 and 7.
- **Plan impact:** phase 1 done, as planned.
- **Validation:** `TestTerminalLinksAreWhatExists` passed (runs of the project, another project, another basedir,
  none; jobs in the newest run, an array task in an older one, only queued, beyond 20 runs, and in another project;
  relative, `:12:3`, `./`, absolute, and missing paths; the request limits and statuses). `go test ./internal/webui
  ./conformance/05-web` passed with Node 24 and `NODE_PATH` (under `taskset -c 0-7`).
- **Remaining:** phases 2 (the panel's link provider) and 3 (WEB-46, conformance, docs). No page asks yet.
