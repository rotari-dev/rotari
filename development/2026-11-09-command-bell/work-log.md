# Work log: A bell when a command finishes out of sight

## Steps 1 and 2: the Keeper's tracking, and the bell

**Commits:** 2026-11-09 20:59:45 `896744f0`.

**Change:** In [session.go](../../internal/terminal/session.go), `foregroundGroup` reads the terminal's foreground
process group, which `Foreground` and the new `busy` use. In [keeper.go](../../internal/terminal/keeper.go), a kept
shell's `busy` is taken as each output arrives and as an attachment ends; output with nothing attached that finds
the shell back at its prompt after being busy sets `ShellInfo.CommandFinished`, which attaching clears.
[terminal.go](../../internal/webui/terminal.go) lists it as `command_finished`. In
[web_shell.js](../../internal/webui/assets/web_shell.js), a tab not shown with it set is `.terminal-tab-finished`, in
place of `.terminal-tab-unseen`, labelled "command finished" and titled "A command finished.", and an All shells card
is `.terminal-overview-finished`, saying "command finished"; [web_shell.css](../../internal/webui/assets/web_shell.css)
draws a bell before the name. WEB-33 and WEB-34, their row, and [WEB.md](../../docs/WEB.md) describe it.

**Reason:** [The plan](plan.md). The user asked for it on 2026-11-09, on tabs not shown and on cards.

**Plan impact:** None.

**Validation:** `TestCommandFinishedIsAnEndNoAttachmentShowed` (a command ending in view, a silent `sleep` ending out
of sight, attaching, a background job's output; passed three times in a row), `TestTerminalShellRoutes` (the list's
field), `TestTerminalPanel` (the bell in place of the dot on a tab not shown, none on the shown one, the label and
title, the cards), and `TestTerminalMarksACommandFinishedOutOfSight` in conformance (the built binary: running,
finished after the connection closes, cleared on reconnecting; passed twice).

**Remaining:** Trying it by hand, with bash and zsh, whose job control gives commands a group of their own as dash's
does.
