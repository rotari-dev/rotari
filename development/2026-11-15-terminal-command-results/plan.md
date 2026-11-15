# Plan: Command results in the Web terminal's server

Created: 2026-11-15
Status: Accepted by the user on 2026-11-15, after asking for the settings to be in the notification config, with the
open questions answered as proposed; all five phases are done. See [work-log.md](work-log.md).

## Goal

With `--allow-terminal`, the server knows how each shell's last command ended, from the marks the shell prints (OSC
133, WEB-48), so that what is shown without looking at the shell says it: a tab's bell is red when the command that
ended out of sight failed, **All shells** says each shell's last command, its exit code, and how long it took, and a
long command that ends while its shell is not in view raises a desktop notification, "make failed (exit 2) after
3 min" or "train.py finished after 2 h 4 min". Today the bell only says that something ended, and nothing reaches a
user who is in another browser tab.

This was a non-goal of the [Blocks plan](../2026-11-14-terminal-blocks/plan.md) ("exit codes on the server: in All
shells, the bell, or notifications"), proposed on 2026-11-15 as the next terminal feature.

## Existing implementation

- **Marks.** bash and zsh started by rotari, and those started inside them (WEB-49, WEB-53), print `C` (with
  `cmdline_url=` and the command line, percent-encoded, as kitty does) before a command and `D;STATUS` after it;
  fish 4 prints them by itself. Only the panel reads them now; the server passes the bytes through and keeps the last
  256 KiB of them for the replay (WEB-33).
- **`command_finished`** (WEB-34). The Keeper marks a shell's `CommandFinished` when output finds its current shell back
  at its prompt after a command, while nothing is attached; a connection clears it. The tab shows a bell in place of
  the unseen-output dot, and the overview card the same.
- **All shells** shows each shell's directory and, from the preview, what runs now.
- **Desktop notifications** (the Settings toggle, `--notifications` for its default, `web_app_notifications.js`) come
  from runs' notification records; the browser's permission and the toggle are per page origin.
- **Notification config** ([docs/NOTIFICATIONS.md](../../docs/NOTIFICATIONS.md)). `notifications.toml`, the first
  found in the project, the basedir, then the global config directory (`notification.Load(baseDir, project)`), holds
  `[browser]` and `[webhook]` settings; the Web UI's **Notification config** (Settings) edits it as a form through
  `/api/notification-config`, and `rotari config --notifications` writes a template.

## Decisions

1. **The server reads the marks.** The Keeper scans each shell's output for OSC 133 as it passes (across chunk
   boundaries, ignoring what is printed while the alternate screen is on): `C` starts a command, taking its command line
   from `cmdline_url=` when there is one, and `D;STATUS` ends it with the status (none when missing or not an integer).
   It keeps only the last ended command per shell, in memory, as it keeps the directory history: command line (or
   none), exit code (or none), when it started and ended. A `D` with no `C` before it (an empty line) changes nothing.
2. **The list says it.** Each shell in `GET /_terminal/shells` gains `last_command`: `{"command", "exit", "started_at",
   "ended_at"}`, absent until a marked command has ended, and a `sequence` that counts the shell's ended commands, so
   that a page can tell a new one. `command_finished` keeps its meaning; a marked command's end counts as one too.
3. **The panel shows it.** A tab's bell is red when the command that ended out of sight exited with a code other than
   0, with a title saying the command, its code, and how long it took; the All shells card gains a line, "last:
   `make` — exit 2 after 3 min, at 10:42", in the run's tone (red for a failure).
4. **Set in the notification config.** `notifications.toml` gains a `[terminal]` section:
   ```toml
   [terminal]
   command_failure = true   # a command that ended with a code other than 0, or none
   command_success = true   # a command that ended with 0
   min_seconds = 30         # only commands that ran at least this long
   ```
   with those defaults when the section or a key is missing. The file that applies is the shell's group's, as for its
   runs: a project's shell the project's (then the basedir's, then the global one), a basedir's own shell the
   basedir's (then the global one), a shell of no project the global one. The server reads it as the list is made and
   says in `last_command` whether to notify (`"notify": true`); a file that cannot be read notifies nothing and is named
   in `notify_error`. The Web UI's **Notification config** form gains a **Terminal** group with the two
   events and the length, and `rotari config --notifications`'s template the section.
5. **Desktop notifications.** A page with the terminal, seeing in the list a newly ended command marked `notify` in a
   shell it does not show (or with the page itself hidden), raises a desktop notification under the same toggle and
   permission as runs' notifications: "`make` failed (exit 2) after 3 min" or "`make` finished after 3 min", with the
   shell's project; clicking it brings the page forward with that shell shown. Pages open on the same browser give it
   the same tag (shell and sequence), so the browser shows one. Nothing is notified with no page open, and nothing is
   written to the runs' notification records or the inbox.
6. **Marks are only output.** A file shown with `cat` can print a false `D;1`, which would at worst ring a false red
   bell or notification, as it already can draw a false mark; SECURITY.md says so. The command line kept is what was
   already in the output, held only in memory with the shell, so "rotari records nothing typed" still holds.
7. **No command-line option.** It follows the marks, and the notification config: a shell that prints none gets no
   `last_command`, as today. The contract is a new WEB-55 (WEB-54 having gone to other work first), and additions to WEB-34 and to the notification config's
   contract notes.

## Phases

1. **The server reads the marks.** In `internal/terminal`: the OSC 133 scanner on the output path (`deliver`), the
   alternate screen's modes, the last command, `ShellInfo.LastCommand` and the sequence; in `internal/webui` the list's
   fields. Unit tests: marks split across chunks, a status that is no number, the command line decoded, a `D` without
   `C`, the alternate screen, and a real bash and zsh with the integration.
2. **The panel shows it.** The red bell and its title, and the All shells line. JSDOM tests.
3. **The notification config.** `[terminal]` in `internal/notification` (parsing, defaults, validation), the server's
   `notify` from the shell's group's file, the editor form's Terminal group and its saving, and the template. Go tests
   and JSDOM tests of the form.
4. **Desktop notifications.** Seeing a new sequence marked `notify` for a shell out of view, the toggle and
   permission, the tag, and the click. JSDOM tests with a fake `Notification`.
5. **Contracts and docs.** WEB-55 and the additions, conformance through the binary (a bash shell running `false` and
   `true` with a `[terminal]` setting, the list's `last_command`, `sequence`, and `notify`), docs/WEB.md,
   docs/NOTIFICATIONS.md, docs/SECURITY.md, and the generated CLI reference if the template's text is in it.

## Tests and validation

- Go: as in phase 1; conformance as in phase 4.
- JSDOM: as in phases 2 and 3.
- `go test ./internal/terminal ./internal/webui ./conformance/05-web` with Node 24, `tsc`, and prettier.

## Non-goals

- Notifications with no page open (by email, chat, the Webhook, or the runs' notification records); a
  `[webhook]` event for terminal commands could follow.
- A history of commands; only the last one is kept.
- Reading exit codes without marks (a shell rotari does not start or integrate).

## Settled questions

Both were accepted as proposed on 2026-11-15.


1. **Defaults.** Proposed: both events on and 30 seconds, so that a long command's end is announced with nothing set.
   The alternative is failures only by default.
2. **Where the section lives.** Proposed: `[terminal]` of its own, since its events and fields are not a run's. The
   alternative is keys in `[browser]` (`terminal_command_failure`, and so on).

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace and is rebased onto `main` before integrating.
