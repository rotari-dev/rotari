# Plan: Shells last 3 hours without a page, and how to keep them longer

Created: 2026-11-10
Status: In progress. See the [work log](work-log.md). The user found 30 minutes short, compared it with VS Code's
remote server (which waits about 3 hours for a client to reconnect), and asked for a 3-hour default and a tip on
keeping shells.

## Goal

A laptop closed over lunch or a commute finds its shells, and what runs in them, still there. Someone who wants them
through a night knows how: run `rotari web` where it outlives the login, and lengthen the timeout.

## Existing implementation

- `webTerminalIdleLimit` ([web_terminal.go](../../cmd/rotari/web_terminal.go)) is 30 minutes; the flag's description
  in [cli_spec.go](../../cmd/rotari/cli_spec.go) says so, and so do the generated CLI reference and Python schema.
- WEB-34 says thirty minutes; [WEB.md](../../docs/WEB.md) says 30 minutes in its terminal section and table.
- Making shells persist beyond the server was [dropped](../2026-11-09-persistent-shells/plan.md): running
  `rotari web` under tmux is enough.

## Proposals

1. **The default is 3 hours.** Forgotten shells stay longer, but at most 32 run, and signing out or stopping the server
   still ends them all.
2. **WEB.md gains "Keeping shells through a disconnect":** run `rotari web` in `tmux` or `screen` on the host, and set
   `--terminal-idle-timeout` (or `terminal-idle-timeout` under `[web]`) longer than the time away; what still ends
   them, and that only the last 256 KiB of output comes back.

## Steps

1. The default, its description and generated views, WEB-34, WEB.md, and the tip.
