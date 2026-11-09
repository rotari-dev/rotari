# Plan: A bell when a command finishes out of sight

Created: 2026-11-09
Status: Done in `896744f0`, not yet tried by hand. See the [work log](work-log.md). The user asked for the usual mark that a shell's command has finished, on tabs not shown and
on All shells' cards.

## Goal

When a command finishes in a shell no pane shows, its tab and its All shells card show a bell, until the shell is
shown, so that a long build or training run in another tab is noticed when it ends.

## Existing implementation

- **Unseen output** (`ShellInfo.UnseenOutput`, set in `Keeper.deliver` when output arrives with nothing attached,
  cleared on attach) gives a tab a dot and a card "new output". It says nothing of whether a command ended.
- **The foreground program:** `Session.Foreground` reads the terminal's foreground process group with `TIOCGPGRP`;
  it is the shell's own group while the shell is at its prompt.
- The page refreshes the shell list every five seconds.

## Proposals

1. **The Keeper notices a command ending.** Each kept shell remembers whether something other than the shell held
   the foreground (`busy`), taken when an attachment ends and whenever output arrives. Output that finds the shell
   back in the foreground after being busy, with nothing attached, sets `ShellInfo.CommandFinished`; attaching clears
   it, as it clears unseen output. A command that prints nothing until it ends is caught by the reading taken when
   the pane stopped showing it, and the shell's prompt is the output that follows its end.
2. **The list says so:** `command_finished` in `GET /_terminal/shells`.
3. **A tab not shown** with it set shows a bell in place of the unseen-output dot, titled "A command finished", and
   says so in its label. **An All shells card** shows the bell and "command finished".
4. Background jobs (`&`) and commands started while nothing showed the shell are not tracked: the shell keeps the
   foreground for them.

## Steps

1. The Keeper's tracking and the list field, with tests.
2. The tab's and the card's bell, with `TestTerminalPanel`; WEB-33, WEB-34, and `docs/WEB.md`.

## Non-goals

- Exit status: the command has gone by the time its end is seen.
- A sound, or a desktop notification.
