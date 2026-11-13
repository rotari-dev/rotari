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

## 2026-11-14 03:50 +09:00 — The panel's link provider (`ad789891da33`)

- **Change:** `web_shell.js` registers `outputLinkProvider` on each pane's terminal. For the line under the pointer,
  joined with the lines it wraps onto (`joinedLine`, by cell, skipping the second half of wide characters), it picks
  candidates (`linkCandidates`): run IDs, job IDs, and words holding `/`, with a trailing full stop, comma, colon,
  or similar left off, web addresses and bare slashes skipped. `askLinks` posts at most 64 unknown ones to the
  shell's `/links` and keeps the answers per shell: run and job links for the page's life, everything else for five
  seconds. A link inside a longer one (a run ID in a path) is dropped. Links are underlined with a pointer cursor; hovering
  sets the pane's title to what the link opens. A run or job goes through the new `goToPage` (which the overview's
  shell and run choices now use too), or `window.open` with Ctrl or Cmd; a directory goes to `showDirInExplorer`
  and a file to `openInViewer`. If the server cannot be asked, nothing is linked and a note says so once per page.
  `dirsURL` became `shellURL`. The JSDOM fake terminal gains `registerLinkProvider` and a line buffer, and the fake
  server a `/links` route; the test is at the end of `TestTerminalPanel`.
- **Reason:** plan phase 2, decisions 5 and 6.
- **Plan impact:** phase 2 done. A plain click's navigation is checked from the panel's own window, where the path
  also goes to the opener, since JSDOM does not follow `location.assign`.
- **Validation:** `tsc` clean; prettier applied. `go test ./internal/webui ./conformance/05-web` passed with Node 24
  and `NODE_PATH` (under `taskset -c 0-7`). Not tried in a real browser.
- **Remaining:** phase 3: WEB-46, conformance, a `TestTerminalPanelInBrowser` step, and the docs.
