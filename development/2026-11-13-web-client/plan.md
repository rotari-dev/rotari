# Plan: A run-style client for rotari web

Created: 2026-11-13
Status: Proposed. Second of two plans following [`rotari web --detach`](../2026-11-13-web-detach/plan.md); the first
is [stopping a background rotari web](../2026-11-13-web-shutdown/plan.md), which this plan assumes.

## Goal

`rotari web` and `rotari run` work the same way at the keyboard. Ctrl-C stops, Ctrl-D detaches, and
`--disconnect-action` chooses what a closed terminal does. Today `rotari run` detaches on Ctrl-D or when its session
ends, while `rotari web` detaches only when started with `--detach`.

## Decisions (2026-11-13)

- **Same shape as `run`, a different default.** `--disconnect-action stop|detach` defaults to `stop` for `web`,
  where `run` defaults to `detach`. A run ends by itself, but a server runs until stopped, so a server left behind by
  every closed terminal would pile up. Other Web tools (jupyter, `python -m http.server`, vite,
  `docker compose up`) stop when their terminal closes, and users expect that.
- **Ctrl-D detaches,** as in `run` and `wait`. Ctrl-D does nothing in `rotari web` today, so nothing that works now
  changes.
- **`--detach` stays** as "start already detached", like `run --async`.
- **Startup says how to detach and how long shells last** (proposal 7).
- **`--shutdown` and the Settings button** ([first plan](../2026-11-13-web-shutdown/plan.md)) stop a server however
  it was started.

## Existing implementation

- `rotari run`'s client: `watchClientInput` (`cmd/rotari/run_command.go`) reads stdin. EOF at the start of a line
  (Ctrl-D in a canonical-mode terminal) or the detach byte means detach. Any other read error means the terminal
  went away and follows `--disconnect-action`. SIGHUP and SIGTERM also count as a disconnect.
  `ROTARI_DISCONNECT_ACTION` sets the default. The run itself lives in a supervisor process, so the client can exit.
- `rotari web --detach` already starts the server in a session of its own, with a readiness pipe, and gives it the
  host's lease and record (`internal/webdetach`). Plain `rotari web` serves in its own process.
- `internal/launcher/wrapper.go` stops a launch when descriptor 3, the read end of a pipe from the Web server, closes.
  That is a precedent for noticing a parent that died without saying anything.

## Proposals

1. **`rotari web` becomes a client of its own server.** It starts the server as `--detach` does: in a new session,
   with a readiness pipe. It stays attached, showing the server's output and `Press Ctrl-D to detach; Ctrl-C to stop
   the server.` The server's stdout and stderr go to the client through a pipe while it is attached.
2. **A lifeline.** The client holds the write end of a second pipe whose read end the server watches. A clean detach
   is a message on it. If it closes without one (the client was killed, even by SIGKILL), the server follows the
   disconnect action. Without this, a killed client would leave a server that nobody chose to keep.
3. **Ctrl-C** (SIGINT) stops the server through the shared shutdown path. The client waits for it to exit and exits 0,
   or with the server's failure. **SIGHUP, SIGTERM, a lost terminal, or a lost lifeline** follow
   `--disconnect-action`: `stop` (the default) stops the server; `detach` detaches it.
4. **Detaching is taking the host's lease.** Only a detached server holds the lease and record, so "one detached
   server per user and host" keeps its meaning, and attached servers, like foreground ones today, do not count.
   - On Ctrl-D the server tries to take the lease. If it gets it, it records itself, moves its stdout and stderr onto
     the host's log (`dup2`), and the client prints the URL, PID, and `rotari web --shutdown`, then exits 0.
   - If another detached server holds the lease, the client says so, names it, and stays attached.
   - With `--disconnect-action detach`, the command checks at start that no detached server runs, so it does not
     fail only when the terminal closes. If one appears meanwhile, the disconnect stops the server and the log says
     why.
5. **Ctrl-D only from a terminal.** `rotari run` treats any end of input as Ctrl-D. `rotari web` watches stdin only
   when it is a terminal. A server started from a script, a test harness, or a systemd unit (stdin on `/dev/null`)
   would otherwise detach the moment it started. This is a deliberate difference from `run`, to be stated in the docs.
6. **The startup lines keep their shape.** `rotari web listening at http://ADDR` is still printed on stdout once the
   server listens, so scripts and the conformance harness that read it keep working. The sign-in lines are printed
   once, by the client.
7. **Startup explains Ctrl-D and the idle timeout** (user request, 2026-11-13). Once the server listens, the client
   prints these on stderr, after the sign-in lines:
   - when stdin is a terminal (proposal 5): `Press Ctrl-D to detach and keep the server running; Ctrl-C to stop
     it.`;
   - with `--allow-terminal`: `Shells end 3h after the last page closes (--terminal-idle-timeout), and when the
     server stops.` The duration is the setting in effect, whether it came from the flag, the environment, or a
     config, written without zero units (`3h`, `90m`, `1h30m`) rather than Go's `3h0m0s`. Once
     [shells last as long as the server by default](../2026-11-13-terminal-idle-off/plan.md), the line with no
     limit set reads `Shells end when the server stops or you sign out.`

   `rotari web --detach` prints the idle-timeout line too, both when it starts a server and when it reports a
   running one (from the record's settings), where the Ctrl-D line has nothing to describe. A detached server's
   remaining lifetime matters most to someone about to close the terminal.

## Interfaces and documents

- **CLI:** `--disconnect-action` for `web`, with `ROTARI_WEB_DISCONNECT_ACTION`; regenerated views.
- **Web UI/API:** none, beyond the first plan's button.
- **Docs:** WEB.md explains Ctrl-C, Ctrl-D, and `--disconnect-action`, with the default and why it differs from
  `run`. RUNNING.md or the FAQ compares the two. ARCHITECTURE.md describes the client, the server, and the lifeline.
- **Contracts:** WEB-34 (a shell ends when the server stops; now also when the client stops it) and WEB-40 change. A
  new WEB-42 covers the client's keys and disconnect actions.

## Steps

1. The server side: accept a lifeline and an output pipe, take the lease on request, and move output to the log,
   with unit tests.
2. The client: start, show output, Ctrl-C, Ctrl-D (terminal only), signals, and `--disconnect-action`.
   `rotari web --detach` becomes "start, detach at once" on the same code.
3. Conformance tests through the binary. Ctrl-D needs a pty; `TestTerminal*` already drive one. They cover:
   Ctrl-D detach and report, Ctrl-D refused while another server is detached, Ctrl-C stop, SIGHUP with each
   action, SIGKILL of the client with each action, and stdin on `/dev/null` staying attached. They also check the
   startup lines: the Ctrl-D line only at a terminal, the idle-timeout line only with the terminal on, and the
   configured duration in it, including from `--detach` for both a new and an already-running server. Check that every
   existing `rotari web` conformance test still passes unchanged.
4. Docs, contracts, generated views, work log.

## Questions

Decided:

- **Environment variable** (agreed 2026-11-13): only `ROTARI_WEB_DISCONNECT_ACTION`. `ROTARI_DISCONNECT_ACTION`
  keeps applying to runs alone, since its values (`cancel` against `stop`) and intent differ.
- **Process identity** (agreed 2026-11-13): a plain `rotari web` runs as two processes, client and server, for as
  long as it stays attached, as `rotari run` does. `--detach` already used the same mechanism, but its starting
  process exits at once. SIGTERM to the client still stops the server by default, and the server's PID is printed
  at start and kept in a detached server's record.

## Risks

- This changes how every `rotari web` runs, not only detached ones. Everything that starts it (conformance harness,
  `scripts/screenshot-web.sh`, docs examples, the Python client if it starts a server) must keep working. Step 3
  checks the harness, and the others need a search before implementation.
- Moving output with `dup2` while goroutines write is safe for whole writes, but a line can be split between the
  pipe and the log.
