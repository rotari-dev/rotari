# Work log: Command results in the Web terminal's server

## 2026-11-15 11:30 +09:00 — The server reads the marks

- **Change:** `internal/terminal/commandmarks.go`: `commandMarks`, a byte-at-a-time reader of a shell's output that
  holds an escape sequence across chunks (OSC ended by BEL or ST, CSI by its final byte; one past 64 KiB dropped),
  follows the alternate screen (private modes 1049, 1047, 47), and reads OSC 133 outside it: `C` starts a command, its
  line from `cmdline_url=` (`url.PathUnescape`, none when it does not decode); `D;STATUS`, or an `A` before any `D`,
  ends it, with the status when it is an integer; a `D` or `A` with no command running is nothing. Each end is a
  `LastCommand` (line, exit, started and ended, a sequence counting the shell's ends). The Keeper reads each chunk in
  `deliver`, keeps the last end in `ShellInfo.LastCommand`, and marks `CommandFinished` for an end while nothing is
  attached. `GET /_terminal/shells` gives it as `last_command` (`command`, `exit`, `started_at`, `ended_at`,
  `sequence`), absent before one.
- **Reason:** plan phase 1, after the user accepted the plan with both events on and 30 seconds by default, a section
  `[terminal]` of its own, and the settings in the notification config.
- **Plan impact:** none.
- **Validation:** `TestCommandMarks` (marks split across chunks with the line and ST; a bad status and an empty line;
  an end by a prompt; the alternate screen; a sequence past the limit; an undecodable line) and
  `TestKeeperKeepsTheLastCommand` (rotari's bash: `sleep 0.2; false` as exit 1 lasting at least 0.2 s, then `true`
  as exit 0, sequence 2) pass twice in a row; see the commit for the package runs.
- **Remaining:** phases 2 (the panel), 3 (the notification config), 4 (desktop notifications), 5 (contracts and docs).
