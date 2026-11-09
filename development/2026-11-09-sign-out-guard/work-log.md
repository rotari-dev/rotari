# Work log: Asking before signing out ends running commands

## Steps 1 and 2: the list's running, and the question

**Commits:** 2026-11-09 20:59:54 `3c7d30c7`.

**Change:** `Keeper.Shells` fills `ShellInfo.Running` from the shell's `busy`, and
[terminal.go](../../internal/webui/terminal.go) lists it as `running`. In
[web_shell.js](../../internal/webui/assets/web_shell.js), `guardSignOut` catches the submit of a form posting to
`/_auth/logout` on a page with the terminal, and `confirmSignOut` lists the shells, names up to five running ones
with their commands, asks, and submits the form itself only on a yes, or at once with nothing running; a list it
cannot get is told in the question. WEB-33, WEB-34, and [WEB.md](../../docs/WEB.md) describe it.

**Reason:** The user asked on 2026-11-09 for a message at sign-out when a command is running in a shell.

**Plan impact:** None.

**Validation:** `TestCommandFinishedIsAnEndNoAttachmentShowed` (not running at the prompt, running under `sleep`),
`TestTerminalShellRoutes` (the list's field), `TestTerminalMarksACommandFinishedOutOfSight` in conformance (running
while `sleep` runs, not after), and `TestTerminalPanel` (the question naming two running shells and one command,
declined and accepted; nothing running; a refused list).

**Remaining:** Trying it by hand.
