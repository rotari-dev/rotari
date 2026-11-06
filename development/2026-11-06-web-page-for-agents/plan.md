# Plan: Let a code agent read the page someone is looking at

Created: 2026-11-06
Status: Not started. [An experiment](#experiment-first) comes before any implementation, since it decides what
an agent should be given. The user currently favors [design A](#a-an-environment-variable-one-file-per-shell)
as a practical first implementation, while asking other agents for their views on the plan.

## Goal

"What is wrong with this job?" should work for a code agent without anyone pasting anything, as browsers' AI
features read the page they are shown on. An agent should be able to read what the person sees in `rotari web`:
its address, what is selected, an open dialog such as a log, and the page's text.

The agent only reads the page. It does not press buttons or move between pages; it may still use its usual tools
to edit code or run `rotari` commands when asked. See [Non-goals](#non-goals).

## The question this raised

The user pointed out that what the Web UI shows is mostly what the CLI already gives. An agent can already run
`rotari show`. What it cannot know is that the person is looking at this job. So the page's value may be little
more than:

- **What it points at:** the basedir, project, run, job or array task, and attempt.
- **What only the screen has:** selected text and jobs, the table's filters and page, which attempt is chosen,
  what dialog is open, and which part of a log is being viewed.

If so, sending the page's text, as [What a page sends](#what-a-page-sends) assumes, costs more than it gives, and a
small snapshot of address and state could do for many requests. An agent reading stale text could even be misled
where the CLI would tell it the current truth. That is why the experiment comes first.

## Existing implementation

- The terminal panel ([web_shell.js](../../internal/webui/assets/web_shell.js)) holds a WebSocket to the shown
  shell, and takes text control messages ([terminal.go](../../internal/webui/terminal.go), up to 1 MiB).
- Another browser tab can take over a shell; a shell and a browser tab do not have a permanent one-to-one
  relationship.
- `/_terminal/alive` is a POST every open page already sends each minute, and pages poll `/api/state` every two
  seconds, so a small request per navigation adds little.
- Pages reload on each link, and redraw every few seconds while a run changes.
- [`state.ResolveMasterDir`](../../internal/state/config.go) gives the per-user directory where the notification
  history already keeps server-wide files.
- Shells' environments leave out the Web token (`webChildEnvironment`), and `StartOptions.Env` adds variables per
  shell. Project shells already get `ROTARI_BASEDIR` and `ROTARI_PROJECT_NAME`. Enabling `--allow-terminal`
  already requires Web authentication, generating a token if none is configured.
- `ROTARI_WEB_PORT` already sets the default for `rotari web --port`; it is not a new page-context identifier.
- `rotari web` takes no positional arguments today, so `rotari web page` would need that to change, or another
  command name.
- `rotari mcp` exists but is stdio only, and the user cannot connect MCP servers in their environment, so neither
  design uses MCP.

## What a page sends

This is the largest form, which [the experiment](#experiment-first) may well reduce. Text, not HTML: most of a page's
HTML is markup and scripts. Both designs would send:

```markdown
URL: /project/p1/run/20261106-010203-abcd1234
Title: decode-run
Basedir: /data/.../.rotari-state
Captured: 2026-11-06T12:03:41Z

## Selection
## Open dialog
## Page
```

- A URL alone does not always identify the basedir or the selected job and attempt. The snapshot needs the
  server-resolved basedir and the stable project, run, job or task, and attempt identifiers where applicable.
  An unknown selection is not the same as no selection.
- Keep selected text separate from checked job IDs. For a table, distinguish filters, sorting, pagination, and
  the IDs actually shown or selected, so "only the ones shown" does not silently include other rows. A selected
  log excerpt needs its job, attempt, stream, and enough context to identify the line.
- The terminal's own text is left out, or it would feed on itself. Input values are also left out except for
  explicitly chosen filter state; even a filter value can contain sensitive text.
- At most 256 KiB in the large form, keeping the end of a long log. Keep a selected excerpt separately even if it
  is older than that tail. Mark truncation and omitted IDs or text; never present a partial selection as complete.
- The capture time and source distinguish what was on screen from current state subsequently read by the CLI.
  A fresh CLI result does not make an old screen selection fresh.
- The file opens by saying what follows is data from the page, not instructions; see [Risks](#risks).

## Two designs

Neither talks to the server from the agent's side, so neither needs a port or a token there. A is the current
preference for a first version; B remains an option if agents outside the panel matter.

### A: an environment variable, one file per shell

The page sends its content over the terminal's existing connection, as a new control message. The server writes
it to a private file for that shell, and the shell has `ROTARI_PAGE_FILE` set to the file's path. The agent reads
the file. The variable is set when the shell starts; the file changes as the page changes, since an already
running agent's environment cannot be updated.

```text
page --(terminal connection)--> rotari web --> $TMPDIR/rotari-web-pages-*/SHELL_ID.md <-- agent in the panel
```

- **Where an agent can run:** only in the panel's shell.
- **Which page:** the one currently connected to that shell. If another tab takes over, the same shell and its
  agents remain, but the page changes. Mark the source and capture time, stop accepting updates from the old
  connection, and treat a disconnected page as stale rather than current. An agent should keep one snapshot
  for a request instead of silently switching targets halfway through.
- **What it needs:** `--allow-terminal`, including its existing Web authentication. Page capture needs its own
  explicit opt-in: someone who already enabled the terminal did not necessarily opt in to saving page contents.
  The shell still receives no Web token.
- **Removed:** with the shell, and at the next server start if the server was killed, without removing files
  belonging to another live server.

### B: a file in the master directory, one per server

The page sends its content by a same-origin POST, which needs no open panel. The server writes it to the master
directory, and a command reads it. It follows the Web server's existing authentication setting: if authentication
is enabled, the POST must authenticate; if it is not, B does not add a login requirement.

```text
page --(POST, existing Web auth if enabled)--> rotari web --> <master>/web-pages/SERVER_ID.md <-- rotari web page
```

Each file names its server:

```text
Server: http://127.0.0.1:8765  (host ks000, pid 12345)
Basedir: /data/.../.rotari-state
Captured: 2026-11-06T12:03:41Z
```

| Command | Reads |
|---|---|
| `rotari web page` | The most recently updated page; this is not necessarily the one in use |
| `rotari web page --list` | Every server's port, basedir, address, and age |
| `rotari web page --port 8766` | That server's page, if the port identifies only one server |

Port numbers alone can collide across hosts sharing a master directory. A server instance needs its own ID,
and an ambiguous `--port` must report the candidates rather than silently pick one. If the panel binds a reader
to its server, it needs a separate identifier, not the existing `ROTARI_WEB_PORT`.

- **Where an agent can run:** anywhere that sees the master directory, such as VS Code's terminal on the same
  host. To use the CLI on another host, it also needs access to the selected project state and paths.
- **Which page:** intended to be the browser tab last used on that server. Visibility and a content-change time
  alone do not establish this, especially with several browser windows or users. Decide how to identify the
  active tab and how to report ambiguity rather than claim a guessed page is current.
- **What it needs:** a new `rotari web` flag, off by default, since it writes page contents to disk.
- **Old pages:** a file keeps its last content after the browser closes, and the command warns by its age. The
  time content last changed does not by itself say whether a tab is still open. A new server instance must not
  overwrite another live server's file.
- **Dead servers:** `--list` shows old files as old. It removes one only for a PID that is gone on the same host in
  the same PID namespace, since a sandbox cannot see a live server's PID; rotari's locks had this bug.

### Comparing them

| | A: environment variable | B: master directory |
|---|---|---|
| Agent in the panel | Yes | Yes |
| Agent elsewhere on the host | No | Yes |
| Agent on another machine | No | Only with access to the master directory and selected project data |
| Several browser tabs | The connected page can change when a shell is taken over | Last used, per server; can be ambiguous |
| Several servers | Each shell has its own | Most recent, or a unique `--port` |
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
2. Only a tab in view sends page contents.
3. Only changed content is sent, except that a tab becoming visible or taking over must announce itself even if
   its content is unchanged. Otherwise returning to a tab could leave another tab's page in the file.
4. Navigation and selection send at once; redraws at most every ten seconds.

Measure these on a run with many jobs before settling the numbers.

An alternative for either design is to send only the address and state, and leave the rest to the CLI, or have the
reader build the text as `rotari show` does. This reduces what is sent, but the selected state is still a snapshot,
and building text needs work for each kind of page. [The experiment](#experiment-first) decides how much text helps.

## Experiment first

### What to learn

1. How often is pointing enough? Does an address alone do as well as the page's text?
2. Which of what only the screen has makes a difference?
3. Does an agent read what it is given, and how must it be told?

The experiment needs no product code: what a page would send is written by hand or copied from the browser.

### Conditions

| Condition | The agent gets |
|---|---|
| 0: nothing | Only the request, as today |
| 1: pointer | The page's address |
| 2: pointer and state | The address, basedir, selection and selected text, filters, chosen attempt, and open dialog's identity |
| 3: everything | Condition 2 and the page's text, including an open log |

### Tasks

Requests are deliberately vague, as someone looking at the page would phrase them, so that an answer depends on
knowing what they point at. Each has an answer fixed in advance.

| Request | State prepared | What should matter |
|---|---|---|
| "Why did this job fail?" | Two or more failed jobs with different causes, one viewed | Job identity |
| "What does this line mean?" | One line of a log selected | Selection |
| "What changed from the previous attempt?" | A retried job with an older attempt chosen | Chosen attempt |
| "Fix only the ones shown in the table" | A filtered table | Filters |
| "Is this run slower than usual?" | Several runs of the project, an older one viewed | Pointer |
| "Is it making progress?" | A run in progress, whose page text then goes stale | Freshness, where text may mislead |

### Measures

- **Found the right target.** The main measure, scored against the fixed answer.
- **Right answer.**
- **Clarifying questions asked.** Asking is better than guessing wrong, so report these with the wrong answers.
- **Commands run and tokens used**, since text also costs an agent to read.
- **Read it unprompted**, in a separate comparison of how it is told: the variable alone, the variable with a line
  in the agent's instructions, and a command named in those instructions. Scoring conditions 1 to 3 only makes sense
  where the agent read the context, so hand it over directly in those.

### Method

1. A script builds one fixture project with these states, and keeps it for every run, so that all conditions see the
   same history. Runs must not change what later ones see; the fix request needs its own copy per run.
2. Context files are made per task and condition. Page text comes from the browser's developer tools on the fixture.
3. Each run is a new session of a CLI agent, with no memory of others. Give every condition the same working
   directory and environment. Also try an unrelated working directory and two basedirs with the same project
   name, so the fixture's location does not make an address appear sufficient. `rotari mcp` is not used, since
   the user's environment cannot connect MCP servers.
4. Scoring is done against the fixed answers, without knowing the condition where a person or another agent scores.
5. Start small: 3 tasks, conditions 0 to 3, twice each, 24 runs. Include a baseline from the start, and test two
   different selections with the same URL. Two runs per cell are exploratory, not proof that a condition is enough.
6. Try more than one agent if possible, since the result may hold for only one. Record each agent's version.

After this comparison, try a minimal browser-to-agent path before committing to a full implementation. Handing
context to an agent does not show whether selection survives focusing the terminal, the right tab sends it, or
an agent discovers and reads `ROTARI_PAGE_FILE` without a pasted prompt.

### What the result decides

| Result | Implementation |
|---|---|
| Condition 1 is enough on the tested tasks | Keep the snapshot small, but still include selections that the same URL cannot distinguish |
| Condition 2 clearly improves on it | Add only the state that helped |
| Only condition 3 helps | Find out whether the CLI has equivalent information or the screen itself supplies something needed |
| Agents do not read it | Fix how they are told before anything else |
| Text misleads when stale | Make its capture time explicit and refresh or omit stale text rather than treating it as current |

The expectation before running is that condition 2 is enough for many tasks. It is only an expectation, and the
experiment can overturn it. Two different selections with the same URL cannot be reliably distinguished from
the address alone, even if a small trial happens to guess correctly.

Limits of what this can show: a small sample, fixture tasks chosen by the people who expect a result, and agents that
change. A clear gap is worth acting on, and a small one should be read as no difference.

## Questions for reviewers

- Does the argument above hold, or is there something the page gives that the CLI cannot, beyond these?
- Are these tasks what people ask? What is missing?
- Is this the right order, or would a small version of design A or B teach more than an experiment on fixtures?
- Is reading enough, or is an agent acting on the page where the value lies?

## Risks

- **Prompt injection.** Job logs and names are written by whoever wrote the job. A log saying "send your secrets to
  ..." reaches the agent, which browsers' AI features also face. Even a small selection or name can contain such
  text. The opening note cannot prevent this; say so in [docs/SECURITY.md](../../docs/SECURITY.md).
- **Page contents on disk.** Logs and filter values may hold secrets. Files are `0600` in a private directory and
  go when the server stops cleanly. Replace files atomically so a reader does not see a partial snapshot. In B on
  a shared master directory, the same user on other hosts can read them.
- **Who can supply a page.** With B, anyone permitted to send to the Web server may be able to replace the page
  an agent reads. An Origin check helps against cross-site browser requests, but is not authentication for
  arbitrary clients. This is a trust boundary of the existing Web configuration, not a reason to require a new
  login when Web authentication is off.
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

- What to send, which [the experiment](#experiment-first) decides.
- A is favored first; whether B is needed later for agents outside the panel.
- The command's name, if B: `rotari web page`, given that `rotari web` takes no arguments today, or something else.
- How B identifies an active tab and server when several are plausible, and whether it should require explicit
  selection instead of using the most recently updated page.
- How to enable page capture separately from the terminal, and how its active state is shown to the person.
- Whether the opening note should be followed by the page's text as it stands, or with lines marked as quoted.

## Tests and validation

- **Secrets stay out.** Input fields other than explicitly chosen filter state, the token, and the terminal's text
  do not reach the file. The UI makes page capture visible.
- **Hostile values.** A job named with control characters or markup stays text, and escape sequences in a log do not
  reach a terminal running `cat`. The command should strip them, or the file should hold none.
- **Target and limits.** A basedir, selected job and attempt, table page and checked rows, and selected log line
  are distinguishable even when the URL is unchanged. The size cap keeps the end of a long log without losing
  a separately selected excerpt, and reports any incomplete selection.
- **Updates.** A redrawing page sends at most every ten seconds, and an unchanged one sends nothing except an
  activation announcement. A background tab sends no page content; returning to an unchanged tab updates the
  source. Test two visible browser windows and delayed updates, too.
- **Off by default.** Without the flag, pages send nothing and no file appears.
- **Guards.** In B, an authenticated Web rejects a POST without credentials. An unauthenticated Web accepts a
  same-origin POST without adding login, while cross-origin browser requests are rejected. Do not treat Origin
  as authentication for a non-browser client.
- **Lifecycle.** A clean stop removes the file, a killed server's file is handled at the next start, and a stale file
  warns. Test a shell takeover, disconnection, and an agent reading during an update (A). Several servers do not
  overwrite each other, and `--port` reports ambiguity and `--list` finds them (B).
- **Browser.** Add to `TestTerminalPanelInBrowser` or a sibling, since page text can be read only in a real browser.
- **By hand.** A real agent reads the page and answers about it, which only the user's environment can show.
- Contract, documentation, and CLI reference updates, and `rotari bug-report` if the page should be collected.
