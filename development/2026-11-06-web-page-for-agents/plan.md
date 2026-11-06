# Plan: Let a code agent read the page someone is looking at

Created: 2026-11-06
Status: Not started. [An experiment](#experiment-first) comes before any implementation, since it decides what
an agent should be given. The user will then choose between [the two designs](#two-designs), and is asking other
agents for their views on this plan.

## Goal

"What is wrong with this job?" should work for a code agent without anyone pasting anything, as browsers' AI
features read the page they are shown on. An agent should be able to read what the person sees in `rotari web`:
its address, what is selected, an open dialog such as a log, and the page's text.

The agent only reads. It does not press buttons or move between pages; see [Non-goals](#non-goals).

## The question this raised

The user pointed out that what the Web UI shows is mostly what the CLI already gives. An agent can already run
`rotari show`. What it cannot know is that the person is looking at this job. So the page's value may be little
more than:

- **What it points at:** the project, run, job, and attempt.
- **What only the screen has:** the selection, the table's filters, which attempt is chosen, what dialog is open.

If so, sending the page's text, as [What a page sends](#what-a-page-sends) assumes, costs more than it gives, and a
few hundred bytes of address and state would do. An agent reading stale text could even be misled where the CLI
would tell it the truth. That is why the experiment comes first.

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

This is the largest form, which [the experiment](#experiment-first) may well reduce. Text, not HTML: most of a page's
HTML is markup and scripts. Both designs would send:

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

An alternative for either design is to send only the address and state, and leave the rest to the CLI, or have the
reader build the text as `rotari show` does. It costs almost nothing and is never stale, but building text needs work
for each kind of page. [The experiment](#experiment-first) decides between this and sending text.

## Experiment first

### What to learn

1. Is pointing enough? Does an address alone do as well as the page's text?
2. Which of what only the screen has makes a difference?
3. Does an agent read what it is given, and how must it be told?

The experiment needs no product code: what a page would send is written by hand or copied from the browser.

### Conditions

| Condition | The agent gets |
|---|---|
| 0: nothing | Only the request, as today |
| 1: pointer | The page's address |
| 2: pointer and state | The address, selection, filters, chosen attempt, and open dialog's name |
| 3: everything | Condition 2 and the page's text, including an open log |

### Tasks

Requests are deliberately vague, as someone looking at the page would phrase them, so that an answer depends on
knowing what they point at. Each has an answer fixed in advance.

| Request | State prepared | What should matter |
|---|---|---|
| "Why did this job fail?" | Two or more failed jobs with different causes, one viewed | Pointer |
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
3. Each run is a new session of a CLI agent, with no memory of others, started in the fixture's directory. `rotari
   mcp` is not used, since the user's environment cannot connect MCP servers.
4. Scoring is done against the fixed answers, without knowing the condition where a person or another agent scores.
5. Start small: 3 tasks, conditions 1 to 3, twice each, 18 runs. One run per cell says little, since agents vary, so
   repeat only what comes out split, and add condition 0 once there is a baseline worth comparing.
6. Try more than one agent if possible, since the result may hold for only one. Record each agent's version.

### What the result decides

| Result | Implementation |
|---|---|
| Condition 1 is enough | Write the address only. Very small, and costs almost nothing |
| Condition 2 clearly improves on it | Add only the state that helped |
| Only condition 3 helps | Send text, but first find out why the CLI did not give it, and improve the CLI if that is the real gap |
| Agents do not read it | Fix how they are told before anything else |
| Text misleads when stale | Do not send it, whatever else it helps |

The expectation before running is that condition 2 is enough. It is only an expectation, and the experiment can
overturn it.

Limits of what this can show: a small sample, fixture tasks chosen by the people who expect a result, and agents that
change. A clear gap is worth acting on, and a small one should be read as no difference.

## Questions for reviewers

- Does the argument above hold, or is there something the page gives that the CLI cannot, beyond these?
- Are these tasks what people ask? What is missing?
- Is this the right order, or would a small version of design A or B teach more than an experiment on fixtures?
- Is reading enough, or is an agent acting on the page where the value lies?

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

- What to send, which [the experiment](#experiment-first) decides.
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
