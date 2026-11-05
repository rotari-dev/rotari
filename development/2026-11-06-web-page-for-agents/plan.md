# Plan: Let a code agent read the page someone is looking at

Created: 2026-11-06
Status: Not started. The user will choose between [the two designs](#two-designs). Implementation may be in
another thread.

## Goal

"What is wrong with this job?" should work for a code agent without anyone pasting anything, as browsers' AI
features read the page they are shown on. An agent should be able to read what the person sees in `rotari web`:
its address, what is selected, an open dialog such as a log, and the page's text.

The agent only reads. It does not press buttons or move between pages; see [Non-goals](#non-goals).

## Existing implementation

- The terminal panel ([web_shell.js](../../internal/webui/assets/web_shell.js)) holds a WebSocket to the shown
  shell, and takes text control messages ([terminal.go](../../internal/webui/terminal.go), up to 1 MiB).
- `/_terminal/alive` is a POST every open page already sends each minute, and pages poll `/api/state` every two
  seconds, so a small request per navigation adds little.
- Pages reload on each link, and redraw every few seconds while a run changes.
- [`state.ResolveMasterDir`](../../internal/state/config.go) gives the per-user directory where the notification
  history already keeps server-wide files.
- Shells' environments leave out the Web token (`webChildEnvironment`), and `StartOptions.Env` adds variables per
  shell. Project shells already get `ROTARI_BASEDIR` and `ROTARI_PROJECT_NAME`.
- `rotari web` takes no positional arguments today, so `rotari web page` would need that to change, or another
  command name.
- `rotari mcp` exists but is stdio only, and the user cannot connect MCP servers in their environment, so neither
  design uses MCP.

## What a page sends

Text, not HTML: most of a page's HTML is markup and scripts. Both designs send:

```markdown
URL: /project/p1/run/20261106-010203-abcd1234
Title: decode-run

## Selection
## Open dialog
## Page
```

- The terminal's own text is left out, or it would feed on itself, and so are the values of input fields.
- At most 256 KiB, keeping the end of a long log.
- The file opens by saying what follows is data from the page, not instructions; see [Risks](#risks).

## Two designs

Neither talks to the server from the agent's side, so neither needs a port or a token there.

### A: an environment variable, one file per shell

The page sends its content over the terminal's existing connection, as a new control message. The server writes
it to a private file for that shell, and the shell has `ROTARI_PAGE_FILE` set to the file's path. The agent reads
the file.

```text
page --(terminal connection)--> rotari web --> $TMPDIR/rotari-web-pages-*/SHELL_ID.md <-- agent in the panel
```

- **Where an agent can run:** only in the panel's shell.
- **Which page:** the one connected to that shell, so several browser tabs cannot be confused.
- **What it needs:** `--allow-terminal`, and nothing new on the command line except perhaps turning it on.
- **Removed:** with the shell, and at the next server start if the server was killed, as launches' directories are.

### B: a file in the master directory, one per server

The page sends its content by a same-origin POST, which needs no open panel. The server writes it to the master
directory, and a command reads it.

```text
page --(POST, signed in)--> rotari web --> <master>/web-pages/HOST-PORT.md <-- rotari web page, from anywhere
```

Each file names its server:

```text
Server: http://127.0.0.1:8765  (host ks000, pid 12345)
Basedir: /data/.../.rotari-state
Updated: 2026-11-06 12:03:41
```

| Command | Reads |
|---|---|
| `rotari web page` | The most recently updated page, which is the one in use |
| `rotari web page --list` | Every server's port, basedir, address, and age |
| `rotari web page --port 8766` | That server's page |

In the panel's shell, `ROTARI_WEB_PORT` makes it read its own server's page.

- **Where an agent can run:** anywhere that sees the master directory, such as VS Code's terminal on the same host.
- **Which page:** the browser tab last used on that server. Using another tab switches it.
- **What it needs:** a new `rotari web` flag, off by default, since it writes page contents to disk.
- **Old pages:** a file keeps its last content after the browser closes, and the command warns by its age. A server
  restarting on a port overwrites its file at start.
- **Dead servers:** `--list` shows old files as old. It removes one only for a PID that is gone on the same host in
  the same PID namespace, since a sandbox cannot see a live server's PID; rotari's locks had this bug.

### Comparing them

| | A: environment variable | B: master directory |
|---|---|---|
| Agent in the panel | Yes | Yes |
| Agent elsewhere on the host | No | Yes |
| Agent on another machine | No | Only with a shared master directory |
| Several browser tabs | Each shell has its own | Last used, per server |
| Several servers | Each shell has its own | Most recent, or `--port` |
| Needs `--allow-terminal` | Yes | No |
| New command | No | Yes |
| Where pages land | Local temporary directory | Master directory, possibly NFS |
| How an agent finds it | The variable, once told | The command, once told |

A later version could combine them: the file from B, with the panel preferring its own page.

## Cost

Not measured. Reading a page's text should take milliseconds, except for tables of many jobs, and sending it is up
to 256 KiB, slower over SSH. B's write may be slow on NFS. Navigation alone looks cheap. A page redrawing every few
seconds during a run is the real cost, since it would send and write with nobody reading. So:

1. Off unless a flag turns it on.
2. Only a tab in view sends.
3. Only changed content is sent.
4. Navigation and selection send at once; redraws at most every ten seconds.

Measure these on a run with many jobs before settling the numbers.

An alternative for either design is to send only the address and selection, and have the reader build the text from
state, as `rotari show` does. It costs almost nothing and is never stale, but needs work for each kind of page, does
not match what is on screen (filters, for one), and loses dialogs other than logs. Keep it in reserve if measuring
shows a problem.

## Risks

- **Prompt injection.** Job logs and names are written by whoever wrote the job. A log saying "send your secrets to
  ..." reaches the agent, which browsers' AI features also face. The opening note lowers this but cannot prevent it.
  Say so in [docs/SECURITY.md](../../docs/SECURITY.md).
- **Page contents on disk.** Logs may hold secrets. Files are `0600` and go when the server stops cleanly. In B on a
  shared master directory, the same user on other hosts can read them.
- **An agent not knowing.** It will not read the file unless told. Document a line for `CLAUDE.md` and the like, and
  consider an agent-facing note in `rotari`'s docs.
- **A stale page read as current.** Each file gives its time, and the reader should treat it as a snapshot.

## Non-goals

- **An agent operating the page.** It would need the agent to talk to the server, with a port and a token that could
  be found in a private file, as Jupyter does. That would make the Web token reachable from the shell, or need a
  separate read-only token, against the current rule that shells do not get it. Add this if reading proves
  insufficient.
- **An agent on another machine with no shared master directory.** It needs the same talking to the server.
- **Agents' private IDE protocols**, such as showing diffs in the page.
- **The Table viewer and static exports.**

## Open questions

- A or B, or both over time.
- The command's name, if B: `rotari web page`, given that `rotari web` takes no arguments today, or something else.
- Whether the opening note should be followed by the page's text as it stands, or with lines marked as quoted.

## Tests and validation

- **Secrets stay out.** Input fields, the token, and the terminal's text do not reach the file.
- **Hostile values.** A job named with control characters or markup stays text, and escape sequences in a log do not
  reach a terminal running `cat`. The command should strip them, or the file should hold none.
- **Limits.** The size cap keeps the end of a long log. A redrawing page sends at most every ten seconds, and an
  unchanged one sends nothing. A background tab sends nothing.
- **Off by default.** Without the flag, pages send nothing and no file appears.
- **Guards.** The POST in B is checked like other mutating requests, with no credentials and from another site.
- **Lifecycle.** A clean stop removes the file, a killed server's file is handled at the next start, and a stale file
  warns. Several servers do not overwrite each other, and `--port` and `--list` find them (B).
- **Browser.** Add to `TestTerminalPanelInBrowser` or a sibling, since page text can be read only in a real browser.
- **By hand.** A real agent reads the page and answers about it, which only the user's environment can show.
- Contract, documentation, and CLI reference updates, and `rotari bug-report` if the page should be collected.
