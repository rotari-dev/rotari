# Work log: When each shell started, and how long it has been idle

## Steps 1 and 2: the times, and where they show

**Commits:** Not yet committed.

**Change:** `ShellInfo` in [keeper.go](../../internal/terminal/keeper.go) has `StartedAt` and `ActiveAt`, both set
by `Start`; `deliver` moves `ActiveAt` with each output and `Attachment.Write` with each input.
[terminal.go](../../internal/webui/terminal.go) lists them as `started_at` and `active_at`, and the list gives the
server's `now`. In [web_shell.js](../../internal/webui/assets/web_shell.js), `refreshShells` keeps how far the
server's clock is from the browser's, `shellTimes` words a shell's start (`HH:MM`, and how long ago) and idle time
(none under a minute), an All shells card has `.terminal-overview-times` ("started 2h 5m ago · idle 15m", or
"active now"), and a tab's tooltip ends "at 14:02, 2h 5m ago. Idle 15m.", worded again as the pointer reaches it.
WEB-33 and WEB-34, their row, and [WEB.md](../../docs/WEB.md) describe it.

**Reason:** [The plan](plan.md). The user asked on 2026-11-10 and chose per-shell times.

**Plan impact:** None.

**Validation:** `TestShellTimesFollowItsActivity` (both times at the start, output and, with echo off, input moving
the activity, the start staying), `TestTerminalShellRoutes` (the list's times and `now` in order),
`TestTerminalPanel` (the tooltips and the cards, dated by the list's `now`: two days, two hours, active now, and a
shell with no times), and `TestTerminalListsWhenShellsStartedAndWereActive` in conformance (the built binary).

**Remaining:** Trying it by hand.
