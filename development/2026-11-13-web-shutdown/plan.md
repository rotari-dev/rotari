# Plan: Stopping a background rotari web

Created: 2026-11-13
Status: Implemented; see [work-log.md](work-log.md). First of the plans following [`rotari web --detach`](../2026-11-13-web-detach/plan.md); the second is
[a run-style client for rotari web](../2026-11-13-web-client/plan.md).

## Goal

A server started with `rotari web --detach` can be stopped as politely as a run can be cancelled: from the command
line with `rotari web --shutdown`, and from the Web UI's Settings menu. Today the only way is `kill PID`, which the
user found crude. Both ways say what is still running in its shells before ending them.

## Decisions (2026-11-13)

- **The name is `--shutdown`.** `rotari server shutdown` already means "stop a background server". `cancel`
  means cancelling a run's jobs, and `--kill` suggests SIGKILL, so neither fits.
- **A flag, as `--detach` is,** rather than a subcommand of `web`.
- **The Web UI gets a stop button too.** It is the natural place in a Web tool, and the CLI is needed as well.

## Existing implementation

- A detached server holds this host's lease and record (`internal/webdetach`: `Find` returns its PID and URL). On
  SIGINT or SIGTERM, `serveWeb` closes launches, the terminal (shells get SIGHUP), and the listener. The record is
  then removed as `cmdWeb` returns.
- The CLI has no channel to the server other than signals. HTTP would need the token, which is never stored.
- The shells are children of the server process. `internal/terminal` finds a shell's foreground command through
  `TIOCGPGRP` on the pty master (`Session.Foreground`), which only the server can do. From outside, the same fact
  is in `/proc/<shell>/stat` (`tpgid` against the shell's own process group) on Linux, and in `ps -o tpgid` on
  macOS. `internal/terminal` already reads process command lines on both (`processCommand`).
- Sign-out in the Web UI lists what runs in each shell and asks before ending them (`confirmSignOut` in
  `web_shell.js`, from `GET /_terminal/shells` and each shell's `running`).
- The Settings menu ends with `aboutSection` (`internal/webui/assets.go`).

## Proposals

1. **`rotari web --shutdown`** finds this host's detached server (`webdetach.Find`). If there is none, it says so and
   exits 0, since the server is already not running; a probe error is still an error. If there is one, it lists
   what runs in its shells and, when anything does, asks before going on. It then sends SIGTERM, waits for the lease
   to be released (up to 30 s), and says the server stopped. A server still running after the wait is an error
   naming its PID; there is no automatic SIGKILL.
2. **What runs in the shells, seen from outside.** A new function in `internal/terminal` lists, for a server PID,
   its shell children whose terminal's foreground group is not the shell's own, with that group leader's command
   line. On Linux this reads `/proc`, on macOS `ps`. Anything it cannot read is reported as "could not be checked",
   as the Web UI's sign-out does, never as "nothing running".
3. **Asking.** At a terminal, a y/N prompt that names up to five shells and their commands. Without a terminal and
   without `--yes`, it fails, listing them and saying to pass `--yes`. `--yes` skips the question. When nothing
   runs, no question is asked.
4. **Exclusive.** `--shutdown` takes only `--yes` (and global options such as `--config`). With any server option,
   or with `--detach` or `--static-dir`, it is an error rather than an option silently ignored.
5. **Stop button in Settings.** (Implementation: with nothing running, or without the terminal, it still asks
   whether to stop the server, since the page loses it; signing out keeps not asking then.) Live pages with job control on get **Stop server** in the Settings menu, above the
   About section. It asks as sign-out does: what runs in each shell, or that this could not be checked. On
   confirmation it sends `POST /_server/shutdown`. The server answers 202, then shuts down through the same path as
   SIGTERM (a detached one also removes its record), and the page says the server stopped and offers no further
   action. The route needs the token and the `Host`/`Origin` checks whenever the server has a token, and is refused
   without job control (403) and for other methods (405). Static exports show no button. This works for any live
   server, detached or not: stopping a foreground one from its page ends that `rotari web` command too.
6. **Same shutdown code.** `serveWeb`'s signal goroutine becomes a function that both SIGINT/SIGTERM and the route
   call, so launches, shells, and the listener are closed in one order however the stop is asked for.

## Interfaces and documents

- **CLI:** `--shutdown`, `--yes` in `cli_spec.go`; regenerated CLI reference, Python schema, golden schema, and the
  flag-pair samples (`pairweb`, which must not start or stop real servers; both flags are refused with
  `--static-dir`).
- **Web UI/API:** the button and `POST /_server/shutdown`.
- **Python client:** it runs the CLI, so the regenerated schema is all it needs.
- **Docs:** WEB.md's "Keeping shells through a disconnect" stops with `rotari web --shutdown` or the button instead
  of `kill`; Settings menu description; SECURITY.md (the route and who may call it); ARCHITECTURE.md.
- **Contracts:** WEB-40 changes from `kill` to `--shutdown`. A new WEB-41 covers the route and button.

## Steps

1. The shared shutdown function in `serveWeb`, with no change in behavior.
2. The outside foreground-command lister in `internal/terminal`, with unit tests that start a shell in a pty and
   run `sleep` in its foreground.
3. `--shutdown`/`--yes`: finding, asking, signalling, waiting, and every refusal; conformance tests through the
   binary. The tests cover: no server, an idle server, one with a foreground `sleep` (refused without `--yes` and
   no terminal, stopped with it), and server options given with `--shutdown`.
4. `POST /_server/shutdown` and the Settings button; Go tests for the route's guards, a JSDOM test for the dialog,
   and a conformance test that the route stops a detached server and removes its record.
5. Docs, contracts, generated views, work log.

## Questions

Decided:

- **Who may press Stop server** (agreed 2026-11-13): job control is enough, as for cancelling jobs; no token is
  required beyond what the server already asks for.

## Risks

- `tpgid` from outside can race with a command starting or ending. The prompt describes a moment, as the Web UI's
  does.
- A server stopped from the button by mistake ends its shells. The confirmation and the list of what runs are the
  guard, as for sign-out.
