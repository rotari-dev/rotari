# Plan: A built-in terminal in `rotari web`

Created: 2026-11-05
Status: Proposed. Whether to build it is undecided; see
[before building](#before-building). This replaces phases 3 to 5 of
[the hardening plan](../2026-11-04-web-terminal/plan.md), whose hardening is done.

## Goal

Open a shell in the browser from `rotari web`, in one click and with nothing else
installed, for the user running the server.

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
- `launchBaseEnvironment` ([web_launch.go](../../cmd/rotari/web_launch.go)) removes the
  Web token from a child's environment.
- `internal/launcher` runs commands without stdin, so its process model does not fit.
- [web.go](../../cmd/rotari/web.go) sets no server timeouts, so long connections work,
  and stops launches and then calls `server.Close`, which does not close hijacked
  connections.
- Go's standard library has no server WebSocket, and `go.mod` has no WebSocket or PTY
  module.

Built-in VS Code and Jupyter Lab launches can probably already give a browser terminal
with `--allow-exec`, though neither has been tried for this. They start from an artifact
row, and VS Code's first start is slow. The advantage of this plan is a permanent entry
and no extra software.

## Scope and non-goals

In scope: one user's shell on the host running `rotari web`, on Linux and macOS.

Non-goals:

- Multi-user terminals, or running as another user.
- A sandbox. The shell has all the user's permissions.
- A shell on a compute node, or attaching to a running job.
- Shells that outlive their connection, recording, and sharing. `tmux` inside the
  terminal covers persistence.
- Static exports, the Python client's API, and MCP. The flag is CLI only.

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
6. **Its own page, `/_terminal/`, in a new tab.** It shows no job data, loads only xterm.js
   and a small script, and is outside basedir mounts. Navigating the main UI never touches
   the shell.
7. **Closing the connection ends the session.** No reattach.
8. **xterm.js, `creack/pty`, and `coder/websocket`.** Each saves real work: terminal
   emulation, PTY setup, and the WebSocket protocol. Before phase 1, confirm that both Go
   modules have no transitive dependencies and support the release targets.

## Design

```text
Browser: xterm.js on /_terminal/
   | WebSocket /_terminal/ws
rotari web: internal/webui/terminal.go   frames, handshake, limits
   | io.ReadWriteCloser, Resize, Close
internal/terminal                        PTY sessions; no HTTP
   | PTY
$SHELL -l
```

- `internal/terminal` knows nothing of HTTP, jobs, or runs, with a rule in
  [internal/archtest](../../internal/archtest/boundaries_test.go).
- Terminal output is binary frames; input and resize are explicit message types.
- The shell is `$SHELL -l` in the home directory, or `/bin/sh` when `$SHELL` is unset, as
  documented. A failed start is reported in the terminal rather than retried with another
  shell.
- `TERM` is set, and the environment filter is shared with launches.
- A fixed limit of concurrent sessions, beyond which opening fails with a clear message.
- Keepalive pings stop proxies dropping idle shells. A blocked WebSocket write stops
  reading from the PTY, so output cannot grow memory without bound.
- Each session's open and close go to the server's standard error, with no content.

Lifecycle:

| Event | Behavior |
| --- | --- |
| Tab closed or connection lost | Close the PTY; the shell gets `SIGHUP`. |
| Shell exits | Send a close frame with its status; the page shows it. |
| `rotari web` stops | Sessions close before the server does. |
| `rotari web` is killed | The kernel closes the PTY and sends `SIGHUP`. |
| `nohup`, `setsid`, rotari supervisors | Keep running, as after closing any terminal. |

This is deliberately weaker than launches, which always stop with the server: a run
started in the terminal must outlive it. Sessions are not runs, take no project lock, and
write nothing under a basedir.

## Interfaces and documents

- CLI: `--allow-terminal`, in `cli_spec.go`, and the generated Python CLI metadata through
  its generator.
- Web UI: a **Terminal** link in the top bar of the application and Recent jobs pages,
  only when enabled.
- [WEB.md](../../docs/WEB.md): the mode table, and the statement that execution starts
  only through `rotari run`, which a terminal changes in practice.
- [RUNNING.md](../../docs/RUNNING.md): `rotari wait` and terminal controls inside a Web
  terminal, if they differ.
- [SECURITY.md](../../docs/SECURITY.md), [ARCHITECTURE.md](../../docs/ARCHITECTURE.md),
  the CLI reference, and reverse proxy notes for WebSocket.
- A new WEB contract ID with conformance, and the status table in `contracts/README.md`.

## Phases

| Phase | Work | Estimate |
| --- | --- | --- |
| 1 | `internal/terminal`, the handler, the flag, and the guard, with the lifecycle above | 2 to 3 days |
| 2 | The page: vendored xterm.js and its fit addon with licenses, resize, a clear state on disconnect | 1 to 2 days |
| 3 | Contracts, conformance, and documents | 2 to 3 days |

A version that merely works is about one to two days of this. The rest is shutdown, the
second platform, and tests. These are rough person-day estimates.

## Tests and validation

Conformance through the binary:

| Case | Expected |
| --- | --- |
| Without the flag | Routes are absent and the UI entry is hidden. |
| Invalid flag combinations | Rejected with an error, not ignored. |
| No session or token | Page and handshake refused. |
| Cross-origin, missing `Origin`, or unexpected `Host` | Refused. |
| Plain HTTP beyond direct loopback | Refused, including with a token. |
| A command typed through the WebSocket | Its output returns; a resize reaches `stty size`. |
| Shell environment | No `ROTARI_WEB_AUTH_TOKEN`. |
| Connection closed, server stopped, server killed | The shell ends; no ordinary process remains. |
| `rotari run` in a session, then the session closed | The run continues. |
| Session limit reached | Clear error. |

In Chrome, extending the browser checks of WEB-28 and WEB-29: the page opens a session with
no policy violation, a typed command's output appears, and `/_terminal/` has the nonce.

By hand, since CI has neither: macOS, and an HTTPS reverse proxy. Also Japanese input and
wide characters, copy and paste, `vim`, `less`, `top`, and `rotari run` with Ctrl-C and
Ctrl-D. Safari's handling of `connect-src 'self'` for WebSocket needs checking there.

## Before building

Answer these first:

1. **What is it for?** If only for `rotari` operations, add them to the Web UI instead.
2. **Do the VS Code or Jupyter Lab terminals fall short?** Try one with
   `rotari web --allow-exec`, and note what is missing.
3. **Is one click worth six to nine days, plus keeping three more dependencies current?**

## Risks

- Anyone who signs in has a shell as this user, so a leaked token is worth more.
- A launched service's page and any future hole in WEB-28 can open a shell. The policy does
  not cover a launched service's own pages.
- A shell over a badly configured proxy may drop or buffer. The notes and keepalive help
  but cannot fix every proxy.
