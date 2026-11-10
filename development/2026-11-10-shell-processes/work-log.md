# Work log: The processes a shell has started

## Steps 1 and 2: the processes, and the card

**Commits:** 2026-11-10 14:37:07 `2aed49d3`, which holds steps 1 to 4 together.

**Change:** `processes` in [process_linux.go](../../internal/terminal/process_linux.go) reads every process's parent
from `/proc/<pid>/stat` (after the last `)`, since the name may hold anything) and its command line, or its name in
brackets; [process_darwin.go](../../internal/terminal/process_darwin.go) reads `ps -A -o pid=,ppid=,command=`.
`Session.Descendants` in [session.go](../../internal/terminal/session.go) walks them from the shell, each child
followed by its own, with its depth, and `Keeper.Processes` finds the shell. The preview in
[terminal.go](../../internal/webui/terminal.go) adds `processes`, or `null` with `processes_error`. In
[web_shell.js](../../internal/webui/assets/web_shell.js), a card's item holds the choosing button, then
`.terminal-overview-run` with the command and the count button `.terminal-overview-processes`, which opens
`.terminal-overview-process-list`. [web_shell.css](../../internal/webui/assets/web_shell.css) gives the item the box
the button had. WEB-33 and WEB-34, their row, and [WEB.md](../../docs/WEB.md) describe it.

While moving the card's frame, a replacement whose range came out empty put the new rule between every two
characters of `web_shell.css`; removing every copy of the inserted text restored the file, and the replacement was
redone within a checked range before anything else touched it.

**Reason:** [The plan](plan.md#decided).

**Plan impact:** None.

**Validation:** `TestProcessesAreTheShellsTree` (none at the prompt; a background child, a child with its own child,
and a process that left the tree; three runs), `TestTerminalPreviewSaysWhereTheShellIs` (`sleep 30` as the one
process), `TestTerminalPanel` (the count, the tree's depth and order, a click that opens and closes it without
choosing the shell, background wording, an unknown list), and `TestTerminalPreviewListsTheShellsProcesses` in
conformance (the built binary, a background `sleep`). `go vet` passed for darwin, and the package builds for
freebsd; the macOS path was not run.

**Remaining:** Trying it by hand, and on a Mac.

## Steps 3 and 4: signing out and ending shells count the processes

**Commits:** 2026-11-10 14:37:07 `2aed49d3`, which holds steps 1 to 4 together.

**Change:** In [web_shell.js](../../internal/webui/assets/web_shell.js), `confirmSignOut` looks at every shell's
preview through `signOutLine`: a shell counts when a command runs in front or any process runs below it, its line
naming the command and the number of processes, or the background processes, up to three, or that they are not
known. `askToEnd` gives every question to end shells (`endShell`, `endShells`, and through them the tab's close, the
tab menu, and All shells' End, End all, and End all shells) the number of processes in them and up to three of their
commands, or "Nothing runs in it.", or why that is not known, in place of "and anything running in it";
`projectEndQuestion` and `everyEndQuestion` give only the question's first half. The End icon's title follows.
WEB-33 and [WEB.md](../../docs/WEB.md) describe both.

**Reason:** The user asked on 2026-11-10 for background processes in signing out's question, then for counts when
ending shells.

**Plan impact:** Steps 3 and 4 added.

**Validation:** `TestTerminalPanel`: signing out with a command and two processes, a running shell whose preview has
no command, four background processes, an idle shell left out, and one whose processes are not known; ending two
shells with five processes and a repeated command; one with a single process; one not known; and the existing end
questions with nothing running.

**Remaining:** Trying it by hand.
