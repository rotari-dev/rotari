# Plan: Links for run IDs, job IDs, and paths in the Web terminal

Created: 2026-11-14
Status: Accepted by the user on 2026-11-14, with the open questions answered as proposed; phase 1 next.

## Goal

With `--allow-terminal`, text a shell prints that names something rotari or the host has becomes a link: a run ID
goes to its run page, a job ID to that job on its run page, and a path to the Explorer (a directory) or the Table
viewer (a file). `rotari run`, `rotari show`, `ls`, and a Python traceback all print such text, and today it has to
be copied by hand into the page or another command.

This is the third of the terminal features proposed on 2026-11-14, after [recent
directories](../2026-11-14-terminal-directory-history/plan.md); Blocks has no plan yet.

## Existing implementation

- **Web links.** [web_shell.js](../../internal/webui/assets/web_shell.js) loads xterm.js's web links addon, which
  underlines `http(s)://` addresses and opens them in a new tab on click. xterm.js 5.5 also offers
  `registerLinkProvider`, which asks a provider for the links on a buffer line as the pointer moves over it.
- **IDs.** A run ID is `YYYYMMDD-HHMMSS-` and eight lowercase hex digits (`state.NewRunID`), its UTC start: no other
  common text looks like it. A job ID is nine lowercase hex digits (`makeJobID` in `cmd/rotari/main.go`), which a git
  hash or a checksum can also be.
- **Pages.** A run is `/project/P/run/R`, and `?job_id=J` focuses a job on it (as History search links); a basedir
  other than the server's own prefixes `/_basedir/ID`. The terminal's shell list already builds such links
  (`terminalShellResponse.Link`).
- **Paths.** The Explorer shows any directory with Follow off (`showDirInExplorer`, from recent directories), and
  `openInViewer` opens a file in the Table viewer through `POST /api/tableview-sources` with `path` (WEB-42).
  `Keeper.Dir` gives a shell's directory now.
- **Groups.** A shell belongs to a project of a basedir, a basedir's own pages, or no project (WEB-34).

## Decisions

1. **The server decides what is a link.** The page finds candidates by pattern and asks the server, which links only
   what exists: a run, a job, or a path on this host. A nine-hex word that is no job of the shell's project stays
   text, so git hashes are not underlined. The rules live once, in Go, beside the code that already resolves runs
   and jobs.
2. **Run IDs** are looked up in the shell's project first, then the other projects of its basedir; a shell of no
   project looks in every registered basedir. The first project holding the run wins, and the link is its run page.
3. **Job IDs** are looked up only in the shell's project (a shell of a basedir's own pages or of no project links
   none): in its newest 20 runs, newest first, and then its queue. A job in a run links to that run's page with
   `?job_id=`; one only queued links to the project's page. Bounding the runs read keeps a hover cheap on NFS, as
   recent directories does.
4. **Paths** are words holding a `/`: absolute, or relative to the shell's directory now. One that exists links: a
   directory to the Explorer with Follow off, a file to the Table viewer. A trailing `:LINE` or `:LINE:COL`, as
   compilers and tracebacks print, is left out of the path. A relative path printed before a `cd` is resolved where
   the shell is when the pointer reaches it, so it may name another file or nothing; Blocks, with the directory each
   command ran in, could fix that later.
5. **Asking.** `POST /_terminal/shells/ID/links` with `{"candidates": ["..."]}` (at most 64 per request, each at most
   4 KiB) answers, for each candidate that is a link, its `kind` (`run`, `job`, `dir`, `file`), the `link` for a run
   or job, or the absolute `path` for a path. It is behind the terminal's checks, as `/_terminal/files` is, and a
   JSON body keeps another site from sending it without a preflight. The page caches answers per shell (paths for a
   few seconds, as directories change; IDs for the page's life) and asks once per line hovered, with the line's
   wrapped parts joined as the web links addon joins them.
6. **Clicking.** A run or job link takes the page there, as choosing another project's shell does; the panel and
   its shells stay. With the panel in a window of its own, the page it belongs to goes there. Ctrl- or Cmd-click opens
   it in a new tab instead. A path opens as decision 4 says. Web addresses keep opening in a new tab. Each link says
   on hover what it opens.
7. **Names are bytes, as elsewhere.** A candidate is text from xterm.js, so always valid UTF-8; a path whose real
   name is not cannot be matched, and stays text.

## Phases

1. **Server.** `/_terminal/shells/ID/links` in `internal/webui` (`terminal_links.go`), resolving runs and jobs
   through `internal/resolve` and `state.RunsNewestFirst`, and paths with `os.Stat` against `Keeper.Dir`. Unit tests
   beside it.
2. **Panel.** A link provider in web_shell.js: candidate patterns, the joined wrapped line, batching and caching,
   underlines and hover titles, and the clicks. JSDOM tests in `TestTerminalPanel`.
3. **Contracts and docs.** A new WEB-46, conformance through the binary, and a step in
   `TestTerminalPanelInBrowser`; the Terminal section of [docs/WEB.md](../../docs/WEB.md), the route in
   [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md), and [docs/SECURITY.md](../../docs/SECURITY.md).

## Tests and validation

- Server: a run of the shell's project, of another project of its basedir, and of another basedir for a shell of no
  project, with the right link and `/_basedir/` prefix; a run ID that exists nowhere; a job in the newest run, in an
  older one, only queued, beyond the 20 runs, and in another project (none); a nine-hex word that is no job; an
  absolute and a relative path, a directory and a file, one with `:12:3`, one that does not exist; the limits on
  candidates (400); unknown shell 404; no terminal 404; plain HTTP to another host 403; a non-JSON body 415.
- JSDOM: candidates found on a line and on a wrapped one; one request per line, cached; a run click navigating and a
  Ctrl-click opening a tab; a job link with `?job_id=`; a directory opening the Explorer and a file the Table viewer;
  no link for what the server does not answer; web addresses unchanged.
- Browser (CI only): `rotari show` printing a run ID in a project shell, the link followed to the run page.

## Non-goals

- Links in the log viewer and other pages; only the terminal.
- Attempt IDs, job names, run names, and host names.
- OSC 8 hyperlinks that programs print.
- Resolving a relative path where it was printed rather than where the shell is now (see decision 4).

## Settled questions

- A run or job link takes this page there, and Ctrl- or Cmd-click opens a new tab (accepted 2026-11-14).
- Job IDs are linked, looked up in the project's newest 20 runs and its queue (accepted 2026-11-14).
- Relative paths are linked, resolved where the shell is now, with decision 4's caveat (accepted 2026-11-14).

## Coordination

Implementation starts from `main` at `c69a33ed` in a workspace of its own and is rebased onto `main` before
integrating.
