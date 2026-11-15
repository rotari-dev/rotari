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

## 2026-11-15 12:50 +09:00 — The panel shows it

- **Change:** `lastCommand` in `web_shell.js` reads a shell's `last_command`: failed when its exit is a number other
  than 0 (none is no failure), a sentence ("make failed (exit 2) after 3 min 0 s.", "… finished after …", "… ended
  without an exit code after …"), and a card line ("last: make — exit 2 after 3 min 0 s, at 10:43"). A tab whose
  command ended out of sight with a failure gets `terminal-tab-failed` (the bell and the edge in `--s-bad`), its label
  says "command failed", and its title leads with the sentence instead of "A command finished."; an All shells card gets
  `terminal-overview-failed` and "command failed" the same way, and a `.terminal-overview-last` line, red for a
  failure, whenever its shell has a last command. `blockDuration` now says "30 s" under a minute instead of
  "0 min 30 s", for blocks too.
- **Reason:** plan phase 2.
- **Plan impact:** none.
- **Validation:** `TestTerminalPanel` (a failure, a success, no exit code, a failure in the shown shell, and one not
  finished out of sight: their tabs' classes, labels, and titles, the cards' states, classes, and lines; no line without
  a last command) passed; `tsc` clean; prettier unchanged.
- **Remaining:** phases 3 (the notification config), 4 (desktop notifications), 5 (contracts and docs).

## 2026-11-15 13:40 +09:00 — The notification config

- **Change:** `internal/notification` has `TerminalSettings` (`command_failure`, `command_success`, `min_seconds`;
  defaults true, true, 30) as `Settings.Terminal`, read from `[terminal]` (unknown keys refused, a negative
  `min_seconds` refused), written by `Marshal` and so in `Template` and `rotari config --notifications`, and
  `Notifies(exit, took)`: long enough, and a success (exit 0) or a failure (any other exit, or none) asked for. `Load`
  with no basedir reads only the global file, where it used to take a relative `notifications.toml` for the basedir's.
  The shell list's `last_command` gains `notify`, from the shell's group's file read as the list is made, or
  `notify_error` with why it could not be read. The editor (`web_app_core.js`) gains a **Terminal commands** group with
  the two events and the length, saved as `settings.terminal`; the save handler starts from the defaults, so a client
  that sends no `terminal` keeps them rather than zeroes.
- **Reason:** plan phase 3.
- **Plan impact:** decision 4 said a config that cannot be read would be named in the server's log; reading it as the
  list is made, every poll, would fill the log, so the list says it in `notify_error` instead.
- **Validation:** `TestTerminalSettings` (defaults; parsed keys; a negative length and an unknown key refused; the round
  trip; the template; `Notifies` for a failure, a success, none, too short, and each event off) and
  `TestLoadWithNoBasedirReadsTheGlobalFile` pass; the editor's JSDOM test in `webui_test.go` shows the group with the
  file's values and saves the form's; see the commit for the package runs.
- **Remaining:** phases 4 (desktop notifications) and 5 (contracts and docs).

## 2026-11-15 14:30 +09:00 — Desktop notifications

- **Change:** `refreshShells` passes each list to `noteCommandEnds`, which remembers each shell's last `ended_at` and,
  after the page's first list, raises `notifyCommandEnd` for a newer one marked `notify` in a shell the page does not
  show, or any while the page is hidden. `canNotify` is the runs' rule: `Notification.permission` granted, the page's
  toggle (`notificationsEnabled`) on, and `notificationUnavailableReason` empty; a page without those (no runs'
  notifications script) notifies nothing. The notification's title is `lastCommand`'s sentence, its body the shell and
  its project, its tag `rotari-terminal-ID-ENDED_AT`, also kept in `localStorage` (`rotari-terminal-notified`) so that
  a second page skips it; a click focuses the page, closes it, opens the panel, and chooses the shell as an All shells
  card does. The list poll now runs while the page is hidden too, every 30 s instead of 5, and only when `canNotify`, so
  that a hidden page still asks nothing unless it can notify.
- **Reason:** plan phase 4.
- **Plan impact:** none.
- **Validation:** `TestTerminalPanel` (nothing for the first list's command; a new failure in a shell not shown told
  with its title, body, and tag; not twice; nothing for the shown shell, one not asked for, one another page took, or
  with the toggle off; a click showing the shell; a hidden page asking only once it can notify), and the existing
  "a page out of sight went on asking" check, passed; `tsc` clean; prettier formatted.
- **Remaining:** phase 5 (contracts and docs).
