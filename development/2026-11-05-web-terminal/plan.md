# Plan: A built-in terminal in `rotari web`

Created: 2026-11-05
Status: Steps 1 to 5 are implemented, and the user has tried the panel by hand. Tabs for several
shells followed; see [Tabs](#tabs). What remains is the first run of the browser tests on CI; see
[the tests](#tests-and-validation). This replaces
phases 3 to 5 of [the hardening plan](../2026-11-04-web-terminal/plan.md), whose hardening is done.

## Goal

Type commands while watching jobs. A panel along the bottom of the Web UI holds a shell
on the server's host, and stays with the user from page to page. A terminal in its own
tab would add little over a real terminal, so being beside the jobs is the point of this
feature.

## Existing implementation

- [internal/webui/page_policy.go](../../internal/webui/page_policy.go) gives live
  pages a nonce-based Content-Security-Policy, and stored values are checked to stay
  text (WEB-28, WEB-29). `connect-src 'self'` should cover a same-origin WebSocket.
- `WithAuthToken` ([auth.go](../../internal/webui/auth.go)) already restricts browser
  sessions to HTTPS or a direct loopback connection. `WithExecGuard`
  ([launch_guard.go](../../internal/webui/launch_guard.go)) checks `Host` and `Origin`,
  including WebSocket handshakes, but is installed only with `--allow-exec`.
- `/_launch/` ([launch_proxy.go](../../internal/webui/launch_proxy.go)) sits outside
  basedir mounts because launches belong to the server. A terminal does too.
- The application pages have no client-side routing: each link reloads the whole page,
  so nothing in a page can hold a shell across navigation. The Recent jobs page has its
  own template, and every page inlines its scripts under a nonce.
- `launchBaseEnvironment` ([web_launch.go](../../cmd/rotari/web_launch.go)) removes the
  Web token from a child's environment.
- [internal/terminal](../../internal/terminal/session.go) runs a shell on a
  pseudo-terminal and ends it when the terminal closes. `internal/launcher` runs commands
  without stdin, so its process model does not fit.
- [web.go](../../cmd/rotari/web.go) sets no server timeouts, so long connections work,
  and stops launches and then calls `server.Close`, which does not close hijacked
  connections.
- Go's standard library has no server WebSocket, and `go.mod` has no WebSocket module.

Built-in VS Code and Jupyter Lab launches can probably also give a browser terminal, though
neither has been tried for this. They open in their own tab, away from the jobs.

## Scope and non-goals

In scope: one user's shell on the host running `rotari web`, on Linux and macOS, in a panel
on the application and Recent jobs pages.

Non-goals, at least at first:

- Multi-user terminals, or running as another user.
- A sandbox. The shell has all the user's permissions.
- A shell on a compute node, or attaching to a running job.
- A shell that survives `rotari web` stopping, recording, and sharing. `tmux` inside the
  terminal covers long-lived work.
- The Table viewer, static exports, the Python client's API, and MCP. The flag is CLI only.

## Decisions

1. **Off by default, behind `--allow-terminal`.** It needs job control, and is rejected
   with `--allow-control=false` and `--static-dir`. `--allow-exec` keeps its promise that
   the browser only names registered commands.
2. **The entrance is the existing one.** A token is always required, even on loopback,
   generated and printed when none is set, as with `--allow-exec`. `WithExecGuard` is
   installed, and `--allowed-host` also applies with this flag.
3. **No second token prompt.** The hardening plan proposed typing the token on each
   opening. Signing in is the entrance, and what runs after it is the user's own, as with
   a launched Jupyter. So script on a page is kept out by WEB-28 and WEB-29.
4. **It combines with `--allow-exec`.** A registered service is already trusted code that
   shares the origin ([SECURITY.md](../../docs/SECURITY.md#launching-services-and-commands)).
5. **The shell opens only over a direct loopback connection or HTTPS.** The rest of the API
   accepts a token over plain HTTP on a trusted network, but this would expose a shell.
6. **A panel along the bottom, not a page of its own.** Decided with the user: the value is
   seeing jobs while typing.
7. **The shell lives on the server, and pages only attach to it.** Each page load reattaches
   and replays recent output, so navigation does not end the shell, and neither does a
   dropped connection. Rejected: rewriting the UI with client-side routing, which is far
   larger, and putting the whole application in an iframe, which breaks the URL and the back
   button.
8. **One shell per server; the newest tab takes it over.** Decided with the user. The tab that
   lost it says so and offers to take it back. This avoids two tabs typing into one shell and
   disagreeing about its size.
9. **An unattended shell ends after 30 minutes.** Decided with the user, counted from the last
   connection closing. Whatever is running in the foreground ends with it, so long work
   belongs in `rotari run` or `tmux`. This is documented where the terminal is introduced.
10. **xterm.js loads when the panel first opens,** as its own file served at `/_terminal/` under
    the page's nonce, so pages stay as light as they are for those who never open it.
11. **xterm.js, `creack/pty`, and `coder/websocket`.** Each saves real work. Before adding
    `coder/websocket`, confirm it has no transitive dependencies and supports the release
    targets. `creack/pty` has none.

## Design

```text
Browser: panel with xterm.js, on every application page
   | WebSocket /_terminal/ws, reattached on each page load
rotari web: internal/webui/terminal.go   frames, handshake, take-over
   | attach, input, resize, end
internal/terminal: Keeper                one shell, recent output, idle timer
   | io.ReadWriteCloser, Resize, Close
internal/terminal: Session               a shell on a PTY (done)
```

The keeper:

- One goroutine always reads the terminal into a ring of recent output, 256 KiB at first, and
  on to the attached reader. With nothing attached, output goes only to the ring, so a program
  never waits, and what falls out of the ring is lost.
- An attached reader that falls behind makes the program wait, as a real terminal does, so a
  burst loses nothing. A first version detached readers whose queue filled, which caught even
  a healthy reader within a second of `yes`. A reader that takes nothing for 30 seconds counts
  as gone, so a dead connection cannot hold a program for good.
- Attaching replays the ring and then streams.
- After replay, it sends the program a resize, so that a full-screen program such as `vim`
  redraws. The ring can start in the middle of a sequence or character; start it at a line
  boundary where one exists.
- Attaching when another connection is attached closes that one with a code the page
  recognizes.
- Starting takes the first size from the page. When no shell is running, opening the panel
  starts one.
- Only opening a page or pressing a button takes the shell. Reconnecting by itself attaches only
  when nothing else is attached, or an old tab coming back would take the shell from the tab in
  use. A tab that is not visible neither connects nor retries, since many old tabs retrying held
  up a new tab's connection for a long time when the user tried it. After a server restart the
  sign-in is gone, so the page stops retrying and says so.

The shell is `$SHELL -l` in the home directory, or `/bin/sh` when `$SHELL` is unset, as
documented. A failed start is shown in the panel rather than retried with another shell. The
environment filter is shared with launches. Each shell's start and end go to the server's
standard error, with no content. Keepalive pings stop proxies dropping an idle connection.

The `pty` package leaves its file blocking, where `Close` waits for a `Read` in progress, so
the terminal would never close while in use. `Session` uses a pollable copy and must not call
`Fd` on it. This relies on the poller supporting terminals, checked only on Linux so far.

Lifecycle:

| Event | Behavior |
| --- | --- |
| Page reload or link | Reattach; the shell continues. |
| Panel hidden | Stays attached; the shell continues. |
| Connection lost, or last tab closed | The shell continues; the 30-minute timer starts. |
| Reattached | The timer stops. |
| The panel's end button, or 30 minutes unattended | Close the PTY; the shell gets `SIGHUP`. |
| Shell exits | The panel shows its status and offers a new shell. |
| Sign out | The shell ends, since the next person to sign in on that browser should not get it. |
| `rotari web` stops | The shell ends before the server does. |
| `rotari web` is killed | The kernel closes the PTY and sends `SIGHUP`. |
| `nohup`, `setsid`, rotari supervisors | Keep running, as after closing any terminal. |

This is deliberately weaker than launches, which always stop with the server: a run started in
the terminal must outlive it. Shells are not runs, take no project lock, and write nothing under
a basedir.

The panel:

- A **Terminal** control in the top bar of the application and Recent jobs pages, only when
  enabled. Both templates share `web_shell.js` and `web_shell.css`, where the panel belongs.
- Height and whether it is open are kept per browser, as the sidebar's width is, so the panel
  comes back open after a link. The page's content shrinks to make room rather than sitting
  under it, and dialogs stay above it.
- It follows the theme. On a narrow screen it takes the full width.
- A log or other dialog ends where the panel begins, so a job's log and the shell stay side by
  side, which is the panel's reason to exist.
- A browser keeps some keys for itself, such as Ctrl-W, and these cannot reach the shell. The
  documentation says so.

The 30-minute idle limit, the 256 KiB ring, the 1 MiB of waiting output, and the 30-second stall
limit are provisional. They are meant to become configuration in the end, so nothing should
depend on their values.

## Tabs

Added after the first version, as the first of several small improvements the user wants. Decided
with the user:

- A tab for each shell, up to eight (provisional). Tabs show numbers for now.
- Only the shown tab is connected. Switching replays up to 256 KiB, as following a link does.
- **The idle limit covers the shells together.** Per shell, tabs nobody is looking at would end after
  30 minutes while the user works in another. It now runs only while nothing is attached to any.
- Closing a tab asks, then ends its shell. A shell ending moves to a neighbor; the last one ending
  leaves a **New shell** button.
- Take-over is per shell, so two browser tabs can use two shells.

Implementation choices: `Keeper` holds the shells itself rather than one `Keeper` per shell, because
the idle limit and the sign-out need them together and a connection must not start a shell by naming
one that has ended. IDs are never reused, while numbers are, so a stale page cannot reach a later
shell. Each browser tab remembers its shell in `sessionStorage`. The list is read every five seconds
while in view, so tabs started elsewhere appear.

## Projects

Second improvement, on the model of an editor's terminals. Decided with the user:

- A project's pages show that project's tabs, and a list reaches other projects with shells.
- A new shell starts where the project's latest run started, or the run page's run, or at home on
  other pages. Eight per project and 32 in all, both provisional.

Choices made while implementing, which the user has not reviewed one by one:

- Following a link with the panel open starts nothing, because otherwise browsing would leave a
  shell in every project visited. Pressing Terminal or **+** starts one.
- Choosing another project in the list goes to its page, so the panel always follows the page and
  **+** has one meaning.
- The server looks the directory up from the project and run. If it cannot, the shell starts at home
  and says why in a first line kept with its output, escaped because the path comes from state.
- Existence on this host is the only check on the directory. A run's host is not compared, since a
  compute node's directory on a shared filesystem is what is wanted.
- The panel skips the application's `fetch` wrapper, which prefixes a mounted basedir. The first tabs
  commit missed this, so on such a page the shells could not be listed.

The user decided that any open page keeps the shells, however it is showing them: otherwise someone
on a project page with no shell would count as idle while their shells run in another project. Every
page offering the terminal says so each minute, so the limit now runs out only after the last page
closes, the computer sleeps, or the sign-in expires.

## Sending suggested commands

Third improvement, asked for by the user: a button beside **Copy** on the commands pages suggest.
It sends no Enter, which the user intended from the start, and starts no shell when the project has
none. Each guide says which commands it sends, joined with `&&`, rather than sending its text: comments
are of no use, the retry guide's `--job-id JOB_ID` line is an alternative, and on a shell without
bracketed paste a line break would run what came before it. Text with control characters is refused,
since it can come from values in the state.

The user then asked for it beside the other copy buttons too: IDs, names, and paths go in as one word,
quoted if needed, so they can finish a half-typed command, and a job's command goes in as it is. After
trying it, they asked for logs too. Several lines go in only with bracketed paste on, because elsewhere each
line would run. xterm.js does not strip escapes from a bracketed paste, so all control characters but line
breaks and tabs are removed first; otherwise a log holding the paste's end sequence could run what follows. A note at the page's corner says what happened, since pages redraw
their buttons every couple of seconds.

## Small improvements

From here the user decides each one in turn, easiest first: Clear, a maximized panel, font size,
project variables in the shell, and clickable links, then a tab menu. Clear also forgets the server's
replay, or following a link would bring back what it cleared. The user found that it also blanked `vim`,
so it now does nothing on the alternate screen, as they chose.

## Interfaces and documents

- CLI: `--allow-terminal`, in `cli_spec.go`, and the generated Python CLI metadata through its
  generator.
- [WEB.md](../../docs/WEB.md): the mode table, the panel and its 30-minute limit, and the
  statement that execution starts only through `rotari run`, which a terminal changes in
  practice.
- [RUNNING.md](../../docs/RUNNING.md): `rotari wait` and terminal controls inside the panel, if
  they differ.
- [SECURITY.md](../../docs/SECURITY.md), [ARCHITECTURE.md](../../docs/ARCHITECTURE.md), the CLI
  reference, and reverse proxy notes for WebSocket.
- A new WEB contract ID with conformance, and the status table in `contracts/README.md`.

## Steps

Each is a commit or two that passes alone.

| Step | Work | Estimate |
| --- | --- | --- |
| Done | `Session` in `internal/terminal` | |
| 1 | The keeper: ring, attach and take-over, idle timer, with unit tests | 1 to 2 days |
| 2 | The WebSocket handler, `--allow-terminal`, the guard, sign-out, and shutdown | 2 days |
| 3 | A plain panel: vendored xterm.js and its fit addon with licenses, reattach, resize | 1 to 2 days |
| 4 | The panel's finish: height, open state, content layout, theme, take-over and exit states | 2 days |
| 5 | Contracts, conformance, and documents | 2 to 3 days |

A version that merely works is steps 1 to 3, about three to four days. These are rough
person-day estimates, eight to twelve days in all.

## Tests and validation

Conformance through the binary:

| Case | Expected |
| --- | --- |
| Without the flag | Routes are absent and the control is hidden. |
| Invalid flag combinations | Rejected with an error, not ignored. |
| No session or token | Handshake and script refused. |
| Cross-origin, missing `Origin`, or unexpected `Host` | Refused. |
| Plain HTTP beyond direct loopback | Refused, including with a token. |
| A command typed through the WebSocket | Its output returns; a resize reaches `stty size`. |
| Reconnecting | The same shell, with recent output replayed. |
| A second connection | Takes over; the first closes with its code. |
| Unattended past the limit, tested with a shorter one | The shell ends; a new connection starts another. |
| Sign out, server stopped, server killed | The shell ends; no ordinary process remains. |
| `rotari run` in the shell, then the shell ended | The run continues. |
| Shell environment | No `ROTARI_WEB_AUTH_TOKEN`. |

`TestTerminalPanelInBrowser` has not yet run anywhere, since Chrome is not on the development
machine and it needs a push to reach CI.

In Chrome, extending the browser checks of WEB-28 and WEB-29: the panel opens with no policy
violation, a typed command's output appears, following a link keeps the shell and its output,
and xterm.js has not loaded before the panel opens.

The user tried the panel by hand on 2026-11-05: the shell, `vim` kept open across a link, `less`,
Japanese input, copy and paste, dragging its height, light and dark colors, typing with a log open,
and take-over by a second tab all worked, and it works on macOS.

By hand, since CI has neither: that `Close` hangs up at once on macOS, and an HTTPS reverse
proxy. Also Japanese input and wide characters, copy and paste, `vim`, `less`, `top`,
reattaching while each runs, and `rotari run` with Ctrl-C and Ctrl-D. Safari's handling of
`connect-src 'self'` for WebSocket needs checking there.

## Risks

- Anyone who signs in has a shell as this user, so a leaked token is worth more.
- A shell now outlives the connection, so one left open by mistake stays reachable for 30
  minutes to anyone with the session, and something typed earlier can be replayed. Accepted
  with the user: a launched VS Code server survives a closed tab in the same way.
- A launched service's page and any future hole in WEB-28 can reach the shell. The policy does
  not cover a launched service's own pages.
- Replaying raw output may not restore a full-screen program exactly. The resize should
  redraw most of them, which has not been tried.
- A shell over a badly configured proxy may drop or buffer. Reattaching and keepalive help but
  cannot fix every proxy.
