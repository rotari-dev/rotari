# Plan: Shells that outlive rotari web

Created: 2026-11-09
Status: Dropped on 2026-11-09. The user concluded that keeping `rotari web` itself running, under tmux or the like,
is enough: a terminal server kept alive would only be a second process to keep alive, and the shells already live as
long as `rotari web` does. A long `--terminal-idle-timeout` covers time; sign-out still ends the shells. The user asked whether shells could persist as tmux's do, as an option: surviving
`rotari web` stopping and signing out, and never ending by time.

## Goal

With the option on, a shell started from the Web terminal runs until someone ends it: stopping or restarting
`rotari web`, signing out, and any length of time with no page open leave it running, and the next `rotari web`
shows it again with its recent output.

## Existing implementation

- **Shells are children of `rotari web`.** [internal/terminal](../../internal/terminal) `Keeper` holds each shell's
  pseudo-terminal in the Web server's process, with its ring of recent output. When the process exits, the terminals
  close and the shells get SIGHUP, so no flag on the Keeper alone can keep them past `rotari web`.
- **They end on purpose today:** after `--terminal-idle-timeout` (default 30 minutes) with no page open, on sign-out
  (`OnSignOut` runs `EndAll`), and when the server stops. WEB-34 promises the last two.
- **The Web server reaches shells only through the Keeper** (start, list, attach, rename, reorder, end, and now
  `Dir` and `Foreground`), which is a clean line to cut along.

## Proposals

1. **A terminal server of its own.** With `rotari web --terminal-persist` (and the same in a config, as the other
   terminal options), the Keeper runs in a separate process, `rotari terminal-server`, started on first need, one per
   user and host, listening on a Unix socket in a private directory (mode 0700). `rotari web` talks to it through the
   Keeper's operations instead of holding the shells itself. The process keeps no idle limit and ignores sign-out.
2. **The Web server's side does not change shape.** `internal/webui` uses an interface with the Keeper's methods;
   the in-process Keeper and a client of the terminal server both satisfy it. This follows the refactoring guidance:
   domain (`internal/terminal`), transport (a new `internal/terminalserver`), and the Web adapter stay apart.
3. **Ending persisted shells without the Web UI:** `rotari terminal list` and `rotari terminal end ID|--all`, and
   `rotari terminal-server stop`, so that shells nobody can see are not left with no way to end them.
4. **What stays:** the token, `Host`/`Origin` checks, and HTTPS-or-loopback rule still guard every route. Only
   connections from the same user reach the socket. Shells still have no Web token in their environment.

## Alternatives

- **tmux underneath:** each shell a tmux session, attached through a pty. Less code, but tmux becomes a dependency,
  its prefix key and status line show through, and its scrollback and ours differ. Not recommended.
- **Only drop the idle limit and sign-out ending:** simple, but shells still die with `rotari web`, which is what a
  restart or upgrade does.

## Questions

- **Sign-out:** today it ends every shell so that a signed-out browser leaves nothing running. With persistence, a
  shell survives sign-out by design; is that acceptable for this option, or should sign-out still end them?
- **Upgrades:** a running terminal server keeps the old binary. Should a newer `rotari web` say so, or restart it
  when no shell is attached?
- **Limits:** keep the 32-shell and 8-per-project limits for persisted shells?

## Steps (if agreed)

1. An interface over the Keeper in `internal/webui`, with no change in behavior.
2. `internal/terminalserver`: the socket protocol (operations and attachments), the server, and the client.
3. `rotari terminal-server`, `--terminal-persist`, and starting the server on demand.
4. `rotari terminal list|end`, contracts (WEB-34 changes for the option), docs, and conformance tests: a shell that
   outlives `rotari web` stopping and starting again, sign-out, and an idle period.

## Risks

- The socket protocol has to carry attachments (output, input, resize, replay) as the WebSocket does now, and their
  back-pressure (`stallLimit`), which is the bulk of the work.
- A terminal server left running after the user stops caring about it; `rotari terminal list` and a line in
  `rotari web`'s startup output naming the running shells keep it visible.
