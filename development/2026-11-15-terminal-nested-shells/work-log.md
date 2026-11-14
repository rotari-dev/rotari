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
