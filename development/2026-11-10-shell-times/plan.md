# Plan: When each shell started, and how long it has been idle

Created: 2026-11-10
Status: Done, not yet committed or tried by hand. See the [work log](work-log.md). The user asked whether the terminal could show start and idle times, and chose per-shell times,
as tmux's session list shows them, over the time left before the idle limit ends every shell.

## Goal

Each shell's All shells card, and its tab's tooltip, say when it started and how long it has been idle, so that a
shell left from yesterday, or one whose job has gone quiet, is easy to spot.

## Existing implementation

- `ShellInfo` ([keeper.go](../../internal/terminal/keeper.go)) has no times. Output passes through `Keeper.deliver`,
  and input through `Attachment.Write`.
- `GET /_terminal/shells` lists the shells every five seconds while the panel is open.

## Proposals

1. **The Keeper keeps two times per shell:** `StartedAt`, set by `Start`, and `ActiveAt`, set by `Start` and then by
   every output and every input. Idle is the time since `ActiveAt`, so a build that keeps printing is not idle.
2. **The list gives them** as `started_at` and `active_at`, with the server's `now`, so that a page works out the
   ages by the server's clock, whatever the browser's.
3. **A card** has a line `started 2h 5m ago · idle 15m`, or `active now` under a minute's quiet. **A tab's tooltip**
   adds `Started 14:02, 2h 5m ago. Idle 15m.`, worked out as the pointer reaches it, since the list's refresh does not
   redraw the tabs.

## Steps

1. The Keeper's times and the list's fields, with tests.
2. The card's line and the tab's tooltip, with `TestTerminalPanel`; WEB-33, WEB-34, and `docs/WEB.md`.
