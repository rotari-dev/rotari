# Plan: The processes a shell has started

Created: 2026-11-10
Status: Done in `2aed49d3`, not yet tried by hand. See the [work log](work-log.md).

## Goal

An All shells card says how many processes its shell has started, as `▶ make · 3 processes`, so that background
jobs, which the bell does not follow, are seen too; clicking the count shows each process and its command, as a
tree under the shell.

## Decided

The user asked on 2026-11-10 for the count beside the running command, and for a click on it to show the processes'
names. Signing out's question was offered the count as well; the user asked for it, and then for the questions to end shells to give counts in place of "anything running in it" (steps 3 and 4).

## Proposals

1. **The processes below a shell** are its descendants by parent ID: from `/proc` on Linux, from
   `ps -A -o pid=,ppid=,command=` on macOS, and unknown elsewhere. They are listed with the shell's preview, which a
   card fetches as the overview opens, not with the list every five seconds. A process that left the tree, by
   `nohup`, `setsid`, or a daemon's double fork, is not among them, and neither is a run handed to a rotari
   supervisor.
2. **The card's line** of what runs moves out of the button that chooses the shell, since a button cannot hold
   another, into the box around it: the command, then the count, `· 3 processes`, or `2 background processes` with
   nothing in the foreground. The count is a button that opens the tree below: each process's ID and command,
   indented under its parent.

## Steps

1. `Session.Descendants`, `Keeper.Processes`, and the preview's `processes`, with tests.
2. The card's line and tree, with `TestTerminalPanel`; WEB-33, WEB-34, and `docs/WEB.md`.
3. Signing out asks about a shell with processes in the background too.
4. Ending shells says how many processes run in them.
