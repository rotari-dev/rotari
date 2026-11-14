# Work log: Shells started inside the Web terminal's shells

## 2026-11-15 06:30 +09:00 — The current shell

- **Change:** `internal/terminal/shells.go` has `interactiveShell`, from a command line: a program named `bash`, `zsh`,
  `fish`, `sh`, `dash`, `ksh`, `mksh`, `tcsh`, or `csh` (a leading `-` taken off), with options only (those taking a
  value, such as `--init-file`, `-o`, and `-O`, skipped with it), no operand, no `-c` (alone or among other letters)
  or `--command`, or with `-i` or `--interactive`. `processArgs` gives a process's arguments (`/proc/PID/cmdline` on
  Linux, `ps`'s command line split at spaces on macOS, unknown elsewhere). `Session.atPrompt` says the terminal's
  foreground group is the session's shell or led by an interactive shell; `busy` and `Foreground` use it, so that the
  first check stays one read of the leader's command line. `Session.Dir` is the leader's directory when it is an
  interactive shell, else, while a command runs, the nearest interactive shell above the leader in the process table,
  else the session's shell. The Keeper and the server read these as before.
- **Reason:** plan phase 1, after the user accepted it with the open questions answered as proposed (the list of
  shells; `-i` counting with an operand).
- **Plan impact:** none.
- **Validation:** `TestInteractiveShell` (the command lines of decision 1); `TestAShellStartedInsideIsTheCurrentShell`
  with real shells (`sh -i` started in the session's `sh`: at its prompt, its `cd` followed, `sleep 30` in it busy and
  named with the directory kept, `sh -c 'sleep 30'` busy, and back to the session's shell after `exit`) passes twice in
  a row; with the old check put back it fails as the user saw it ("busy yes, command sh -i" at the inner prompt).
  `go test ./internal/terminal` passed; `go vet` for Linux and macOS and a FreeBSD build are clean.
- **Remaining:** phase 2 (contracts, conformance, docs).

## 2026-11-15 07:10 +09:00 — Contracts and docs

- **Change:** contract **WEB-52** (the current shell, how an interactive shell is told, and what follows it) and its
  row; WEB-34's preview `cwd` and `command`, `command_finished`, and `running`, WEB-42's Follow, and WEB-45's reading
  of a directory now say the current shell's. `TestTerminalFollowsAShellStartedInside` covers it through the binary:
  in a `/bin/sh` shell, `PS1='inner$ ' sh -i`, then `cd` to a new directory there; the preview gives that directory
  and no command and the list no `running`; `sleep 30` in it is named and `running`; Ctrl-C brings its prompt back;
  Recent directories has the directory; and after `exit` the session's shell, in the home directory, is current again.
  docs/WEB.md says inner shells are followed, and what is not.
- **Reason:** plan phase 2.
- **Plan impact:** none.
- **Validation:** the new test passed twice in a row; `TestContractStatus` passed; see the commit for the runs.
- **Remaining:** nothing in this plan. Marks from inner shells (a line for one's own startup file) are the second plan.
