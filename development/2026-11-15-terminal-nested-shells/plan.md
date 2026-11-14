# Plan: Shells started inside the Web terminal's shells

Created: 2026-11-15
Status: Accepted by the user on 2026-11-15, with the open questions answered as proposed; phase 1 is done and
phase 2 (contracts and docs) is next. See [work-log.md](work-log.md).

## Goal

With `--allow-terminal`, a shell started by hand inside one of the panel's shells (`zsh` typed at a bash prompt, `bash`
inside `zsh`) is followed as the shell the user is in: the Explorer follows its directory, **Recent directories** records
where it goes, and the panel counts it as at its prompt when it waits there, so that typing `cd`, a command again, or a
sent command is allowed, and the tab does not say a command runs. Today all of these see only the shell rotari started,
and take the inner shell for a command that never ends.

This is the first of two plans proposed on 2026-11-15 for inner shells. The second, a line to add to one's own
`.bashrc` or `.zshrc` so that an inner shell also marks its prompts (blocks, click to move, Cut), as VS Code offers for
sub-shells, gets a plan of its own.

## Existing implementation

- **`internal/terminal`.** `Session.Dir` reads the working directory of the shell rotari started (`processDir` of its
  PID). `Session.busy` and `Session.Foreground` compare the terminal's foreground process group (`TIOCGPGRP`) with
  that shell's PID: the same group means the shell is at its prompt; another means a command runs, named by
  `processCommand` of the group's leader. `Session.Descendants` lists the processes below the shell
  (`processes`, from `/proc` on Linux and `ps` on macOS).
- **Users of "at its prompt" and "where".** The Keeper reads a shell's directory for **Recent directories** only when it
  is not busy after Enter (WEB-45); `command_finished` and `running` in the shell list come from `busy` (WEB-34);
  `GET /_terminal/shells/ID/preview` gives `cwd` from `Dir` and `command` from `Foreground`, which the Explorer's
  Follow, the launch dialog, and `typeAtPrompt` (recent directories, typing a command again) read (WEB-42, WEB-45,
  WEB-48); All shells shows `command`.
- **An inner shell** with job control is the leader of its own process group, which holds the terminal's foreground
  while it waits at its prompt; a command it runs gets a group of its own. A script run by a shell (`bash x.sh`,
  `sh -c '...'`) is a shell process too, but not one waiting for the user.

## Decisions

1. **The current shell** is found from the terminal's foreground process group: its leader, if that is an interactive
   shell, or else the nearest interactive shell above it, up to the shell rotari started, which is always one. A
   process is an interactive shell when its program's name (without a leading `-`) is `bash`, `zsh`, `fish`, `sh`,
   `dash`, `ksh`, `mksh`, `tcsh`, or `csh`, and its command line has no operand and no `-c`: options only, with the
   ones that take a value (`--init-file`, `--rcfile`, `-o`, `+o`, and the like) skipped with their value. `-i` makes
   any shell interactive. So `zsh`, `-zsh`, `bash -l`, and `zsh -i` are; `bash x.sh` and `sh -c 'make'` are not.
2. **What follows it.** `Dir` is the current shell's working directory. The shell is at its prompt when the foreground
   group's leader is the current shell itself; otherwise `Foreground` names the leader's command as today, and `busy`
   is true. Everything that reads these (decision list above) follows without change of its own: Recent directories,
   `command_finished`, `running`, the preview's `cwd` and `command`, the Explorer's Follow, the launch dialog, typing
   at the prompt, and All shells.
3. **What does not.** A shell on another terminal (inside `tmux`, `screen`, or over `ssh`) is not an inner shell of
   this terminal: the foreground is `tmux` or `ssh`, a command, as today. `sudo -i` or `su -` shows as `sudo` or `su`,
   whose shell's directory rotari may not be able to read. The processes list of the preview is unchanged.
4. **Nothing to set up**, no option: the shell rotari started is the current shell whenever nothing else is, so a
   terminal without inner shells behaves exactly as now. The contract notes that say "the shell" for these readings
   (WEB-34, WEB-42, WEB-45) say "the current shell", defined once in a new WEB-52.

## Phases

1. **The current shell.** In `internal/terminal`: recognizing an interactive shell from a command line, finding the
   current shell from the foreground group and the process table, and `Dir`, `Foreground`, and `busy` on it. Unit tests
   with real shells: `sh` (or `bash`) started inside the session shell, `cd` there followed, its prompt not busy, a
   command run in it busy and named, and a script (`sh -c 'sleep 30'`) still busy; recognition tests for the command
   lines above, on Linux and macOS.
2. **Contracts and docs.** WEB-52 and the wording of WEB-34, WEB-42, and WEB-45; conformance through the binary (a
   shell of `/bin/sh` starting `sh`, `cd /tmp` in it, and the preview saying `cwd` `/tmp` and no `command`, then
   `sleep` in it named); the Terminal section of [docs/WEB.md](../../docs/WEB.md).

## Tests and validation

- Go: as in phase 1, and the Keeper's Recent directories recording a `cd` made in an inner shell.
- Conformance: as in phase 2.
- `go test ./internal/terminal ./internal/webui ./conformance/05-web`.

## Non-goals

- Marks (OSC 133) from inner shells: the second plan.
- Following into `tmux`, `screen`, `ssh`, containers, or `sudo`/`su` shells.
- Telling an interactive shell from a script by more than its command line.

## Settled questions

Both were accepted as proposed on 2026-11-15.


1. **Which programs count as shells.** Proposed: the list in decision 1. Others (`nu`, `xonsh`, `elvish`, `pwsh`) could
   be added, but their command lines follow other conventions.
2. **A shell with an operand but `-i`** (`bash -i x.sh`). Proposed: counted as interactive, as `-i` asks; it is rare.

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace and is rebased onto `main` before integrating.
