# Plan: Recent directories in the Web terminal

Created: 2026-11-14
Status: Accepted by the user on 2026-11-14, with the open questions answered as proposed; phase 1 next.

## Goal

With `--allow-terminal`, the panel offers a dropdown of the directories its shells have been in, and of where the
project's recent runs started, so that going back to one is a choice rather than a long `cd` typed again. Choosing
one types `cd '/the/path'` into the focused pane without running it, or shows it in the Explorer, or opens a new
tab there.

This is the first of the terminal features proposed on 2026-11-14 (recent directories, Blocks, links for rotari
IDs); each has a plan of its own. Blocks may later group commands by directory; nothing here depends on it.

## Existing implementation

- **Where a shell is.** `Keeper.Dir` ([keeper.go](../../internal/terminal/keeper.go)) reads the shell's own working
  directory (`/proc/PID/cwd` on Linux), and `/_terminal/shells/ID/preview` reports it. The Explorer
  ([plan](../2026-11-13-terminal-explorer/plan.md)) follows it by listing again on the shell list's refresh and
  shortly after Enter.
- **Back at the prompt.** `Keeper.deliver` already reads whether a command holds the foreground (`session.busy`)
  each time output arrives, under the Keeper's lock; the command bell
  ([plan](../2026-11-09-command-bell/plan.md)) uses its change from busy to not.
- **Groups.** Shells belong to a group, which `internal/webui` makes from the page's place: a project, a basedir's
  own pages, or no project. The Keeper treats the key as opaque.
- **Where runs started.** `site.terminalRunDir` ([terminal.go](../../internal/webui/terminal.go)) reads a run's
  `CWD` from its context; a project's shells start where its latest run started.
- **Sending.** `sendToTerminal` and `shellWord` in [web_shell.js](../../internal/webui/assets/web_shell.js) type a
  value as one quoted word without Enter, and refuse control characters.
- **Starting a shell in a directory.** `POST /_terminal/shells` with `dir` (WEB-34), which the Explorer's **Open
  in a new terminal tab** uses.

## Decisions

1. **One history per group** (settled): the shells of a project share one, as do a basedir's own and those of no
   project. A new tab in the project offers what its other shells visited.
2. **Run directories are candidates too** (settled): beside the history, the dropdown lists where the project's
   recent runs started, so it is useful before any `cd` and without setting up the shell.
3. **Nothing is typed and run** (settled in the Explorer's decision 10 and kept): choosing types `cd` and a quoted
   path, and the user presses Enter.
4. **How the history is gathered: by the Keeper, without shell setup.** When input with a carriage return has gone
   to a shell, the next output that finds the shell holding its own foreground (at its prompt) has the Keeper read
   the shell's directory, and a directory other than the last one goes to the front of the group's history. So
   `cd`, `pushd`, `popd`, and `z` are all seen, and a shell's start directory is its first entry. Not seen: a
   directory changed in a program in front, such as a nested shell, `ssh`, or `tmux`, as the Explorer's Follow does
   not see them either, and a directory left before the prompt returns. OSC 7 from the shell is not read here; it
   belongs with Blocks, which needs shell setup anyway.
5. **Kept in memory only, and briefly.** Each group keeps its 20 most recent distinct directories, newest first,
   with when each was last entered. They are never written to disk. A shell ending leaves its group's history; all
   of them go whenever every shell ends (signing out, the idle limit, `rotari web` stopping), in `Keeper.EndAll`.
   The FAQ's "does not log what you type" stays true, and the docs say that the directories are remembered this
   long.
6. **Run candidates.** For a project: the distinct start directories of its runs, newest first, at most five, read
   from the latest 20 runs only, so that a project with thousands of runs on NFS costs 20 context reads. Each says
   its run ID and when it started. A basedir's own pages offer the workspace directory, as a new shell there
   starts; pages of no project offer none. A directory missing on this host is listed greyed out with "not on this
   host", rather than left out.
7. **API.** `GET /_terminal/dirs?shell=ID` answers `{"history": [{"dir", "entered_at"}], "runs": [{"dir", "run",
   "started_at", "missing"}]}` for the shell's group, behind the terminal's own checks (the token, `Host`/`Origin`,
   HTTPS or a direct loopback connection). An unknown shell is 404; a run that cannot be read fails the request
   with the reason, rather than giving a shorter list. The page asks each time the dropdown opens.
8. **Where it is.** A **Recent directories** icon in the panel's bar, acting on the focused pane as **Clear** does,
   so it works with the Explorer closed; the Explorer's path line opens the same dropdown. The dropdown has a
   filter field, then **Recent** and **Runs** sections. Arrow keys, Enter, and Escape work as in the All shells
   dialog.
9. **What choosing does.** Enter or a click types `cd 'PATH'` into the focused pane through `sendToTerminal`'s
   rules. Just before typing, the page asks the shell's preview; when a command holds its foreground (`vim`, `ssh`,
   a REPL), it types nothing and says what runs there, so the `cd` cannot land in another program. Text already on
   the prompt stays, and the `cd` goes where the cursor is, as with the send buttons. A row's other actions: **Show
   in the Explorer** (its root there, Follow off), **Open in a new terminal tab** (WEB-34's `dir`), and **Copy the
   path**.
10. **Paths are arbitrary strings.** A directory whose name is not valid UTF-8 is marked as in the Explorer's
    decision 4 and is shown but not typed or copied. Control characters are refused by `sendToTerminal`, with its
    note.

## Phases

1. **Keeper.** The group history in [internal/terminal](../../internal/terminal): the carriage-return mark set in
   `Attachment.Write`, the directory read in `deliver`, the start entry, the limit, clearing in `EndAll`, and
   `Keeper.History(group)`. Unit tests in `keeper_test.go`.
2. **Server.** `/_terminal/dirs` in `internal/webui`, with the run candidates through the same context read as
   `terminalRunDir`. Unit tests beside it.
3. **Panel.** The bar icon, the dropdown with its filter, sections, keys, and row actions, and the preview check
   before typing, in web_shell.js and web_shell.css. JSDOM tests in `TestTerminalPanel`.
4. **Contracts and docs.** A new WEB-45 for the history and its API, conformance through the binary, and a step in
   `TestTerminalPanelInBrowser` (cd twice, open the dropdown, choose the first directory, see the `cd` typed). The
   Terminal section of [docs/WEB.md](../../docs/WEB.md), the FAQ's answer on what the shells record, the terminal
   routes in [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md), and [docs/SECURITY.md](../../docs/SECURITY.md).

## Tests and validation

- Keeper: `cd` in a shell adds the new directory after the prompt returns; the same directory twice is one entry,
  moved to the front; the 21st drops the oldest; two shells of a group share it and another group's is separate; a
  directory changed while a command holds the foreground is not recorded; `EndAll` clears every group; one shell
  ending does not.
- Server: history and runs for a project, a basedir, and no project; the five distinct run directories from the
  latest 20; a missing directory marked; an unreadable run refused with the reason; unknown shell 404; no terminal
  404; plain HTTP to another host 403.
- JSDOM: the dropdown from the bar and from the Explorer's path; filtering; keys; `cd` typed quoted without Enter;
  refused with a note while a command runs; the other actions; a non-UTF-8 entry shown but not sendable.
- Browser (CI only): the WEB-33 step above.

## Non-goals

- Reading OSC 7 or any other shell setup (see Blocks).
- Keeping the history across `rotari web` restarts or on disk.
- Directories of programs in front of the shell, such as a remote shell over `ssh`.
- Output and job directories of runs; only where each run started.
- Changing the shell's directory without the user pressing Enter.

## Settled questions

- 20 directories per group, and five run directories from the latest 20 runs (accepted 2026-11-14).
- The history goes only when every shell ends, not when the last shell of its group does (accepted 2026-11-14).

## Coordination

Other workspaces touch the terminal (`terminal-explorer`, `terminal-split`, `basedir-shells`, and others in
`jj workspace list`). Implementation starts from `main` at `396157e2` in a workspace of its own and is rebased onto
`main` before integrating.
