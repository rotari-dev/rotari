# Plan: `rotari web --detach`

Created: 2026-11-13
Status: Implemented; see [work-log.md](work-log.md). Where the implementation departs from a proposal below, the
proposal says so.

## Goal

Keeping Web terminal shells through a disconnect takes one command. Today
[WEB.md](../../docs/WEB.md#keeping-shells-through-a-disconnect) asks for `tmux new`, `rotari web ...`, Ctrl-B D, and
`tmux attach` to find the URL again, so using the terminal well requires knowing tmux. With `--detach`, `rotari web`
starts the server in the background, prints where it is, and returns; the server outlives the SSH session.

## Background and decisions

- **Shells stay children of `rotari web`.** A separate terminal server was
  [dropped](../2026-11-09-persistent-shells/plan.md): it would be one more process to keep alive. This plan keeps that
  decision and makes `rotari web` itself easy to keep alive.
- **No tmux integration.** Discussed on 2026-11-13: tying Web terminal tabs to tmux windows (as iTerm2's control mode
  does) only pays off for people moving between a terminal and the browser on the same shells, which few do, and a
  rotari-only tmux socket would hide the user's own tmux anyway. tmux would become a dependency with version
  differences across hosts.
- **No IDs or names for detached servers, for now.** tmux-style IDs would need listing, stopping, and attaching by ID,
  which pulls toward subcommands, and several servers would split the shells. A later `--name` (same name reports,
  another starts a new one) stays possible by adding the name to the record's and log's file names.
- **A flag, not subcommands.** `rotari web` takes only flags today; `start`/`status`/`stop` subcommands would not fit
  it. Running `--detach` again reports the running server, which serves as status; stopping is `kill PID`.
- **The idle timeout keeps its 3-hour default** with `--detach`. A default that changes with another flag is hard to
  predict; `--terminal-idle-timeout` or the config lengthens it, as today.
- **Out of scope:** shells surviving `rotari web` stopping, upgrades, or sign-out (unchanged, WEB-34); a `--stop`
  flag (can follow if `kill` proves awkward); `rotari tableview --detach`.

## Existing implementation

- [cmd/rotari/web.go](../../cmd/rotari/web.go) parses flags, generates a token for `--allow-exec`/`--allow-terminal`
  when none is set and prints it on stderr, records the start through `weblog`, listens (`webui.Listen`, picking a
  free port unless `--port` is explicit), prints `rotari web listening at http://ADDR` on stdout, and stops on
  SIGINT/SIGTERM, closing launches and the terminal first.
- `server.Start` in [internal/server/client.go](../../internal/server/client.go) already starts a detached process:
  `Setsid`, stdio on `/dev/null`, a pipe for readiness, and an error naming the log when the child exits or does not
  become ready.
- [internal/server/lease.go](../../internal/server/lease.go) holds a supervisor's lease with `flock` for its life;
  `Running` probes it with a non-blocking shared lock. Both are tied to a project directory (`LockPath(dir)`), and
  `Running` reads any probe error as "not running".
- The master directory (`stateinternal.ResolveMasterDir`) holds per-user server state such as the run registry and
  `weblog`'s records.
- `web` flags are declared in [cmd/rotari/cli_spec.go](../../cmd/rotari/cli_spec.go), which also generates the CLI
  reference and the Python client's schema (`python/rotari/generated_cli.py`).

## Proposals

1. **`--detach` (command line only).** The parent resolves flags and config as today, so bad options fail in the
   foreground. It then re-executes itself without `--detach` in a new session (`Setsid`), stdin on `/dev/null`, stdout
   and stderr on the log of proposal 9, and waits for the child to report that it is listening. It
   prints the URL, the PID, the log path, and the sign-in line, then exits 0. Incompatible with `--static-dir`.
2. **The token never goes through argv.** When a token is needed and none is set, the parent generates it and hands
   it to the child in `ROTARI_WEB_AUTH_TOKEN`, so `ps` does not show it, and prints it once as today.
3. **Readiness and failure are explicit.** The child reports readiness over an inherited pipe after `Listen`
   succeeds, with its address. If it exits or does not report within a timeout, the parent fails with a non-zero
   status, the reason it can tell, and the log path; on timeout it kills the child. No success is printed for a server
   that is not serving.
4. **A record and a lease for each detached server.** The child writes `<masterdir>/web/<host>.json` (mode
   0600: host, PID, URL, basedir, start time, version, executable, whether the terminal and launching are on; never the
   token) and holds an exclusive `flock` on a lease file beside it until it exits, as a run's supervisor does with its
   server lease (`server.Acquire` / `server.Running` in [lease.go](../../internal/server/lease.go)). It removes the record on
   a clean exit. The lease file itself stays (implementation): removing it would let a process that opened it before
   the removal lock a file no one else sees, and two servers start.
5. **Liveness comes from the lease, not the PID.** A server is running when a non-blocking shared lock on its lease
   fails with `EWOULDBLOCK`. The kernel drops the lock however the process dies, so a left-over record of a crashed
   server reads as stopped and is removed when found, and a reused PID cannot pass for it. Only records of the current
   host count: another host's `127.0.0.1` URL is no use here and its PID cannot be killed from here, and on a shared
   (NFS) master directory the lock is then only ever contended within one host. An error other than "not locked" or
   "no such file" while probing (for example a permission error) is reported, never read as "not running". An HTTP
   request to the recorded URL is not the test, since another program may have taken the port.
6. **One detached server per user and host.** Running `--detach` while one is running prints that server's URL, PID,
   and log path and exits 0 without starting another, so re-running the same command is how to find it again. A token
   it generated is not shown again: the message says to use the one printed at start, or to set
   `ROTARI_WEB_AUTH_TOKEN`. If the options given differ from that server's (for example `--allow-terminal`, the
   basedir, or an explicit `--port`), the command fails, naming the PID to stop first, rather than silently ignoring
   them. One server already serves several basedirs, so several detached servers are rarely needed; two servers would
   also split the shells between URLs.
7. **A server not detached is unchanged.** Plain `rotari web` writes no record and is not counted by `--detach`; it
   starts whether or not a detached server runs, as two foreground servers can today. When a detached server is
   running on this host, it prints one line naming that server's URL and PID, so it is clear which one holds the
   shells. Ports do not clash unless both name the same `--port`, which fails at listen as today.
8. **Stopping.** `kill PID` sends SIGTERM, which already shuts down cleanly and ends the shells. The log and the
   record say so.
9. **The detached server's output goes to one rotated log per host.** The child's stdout and stderr are the file
   `<masterdir>/web/<host>.log` itself, opened for append, so even a crash's last words reach it; there is one file
   because there is one detached server per host. Like `weblog`'s `web-events.jsonl`, it keeps at most about 1 MiB
   plus one rotated `<host>.log.1`: the starting parent renames an oversized log to `.1` (replacing the old one), and
   the running server checks the size on a timer and, past the limit, copies the log to `.1` and truncates it in
   place, since it cannot reopen its own standard streams (lines written between the copy and the truncate may be
   lost; acceptable for diagnostics). Nothing relies on `rotari gc`.
10. **Unsupported systems.** rotari builds only for Unix-like systems (it already uses `Setsid` and `flock`
    elsewhere), so there is no system without a new session to refuse (implementation).

## Interfaces and documents

- **CLI:** the flag in `cli_spec.go`; regenerated CLI reference and Python schema.
- **Server, Web UI/API, Python client:** no behavior change. The Settings menu could later say the server is
  detached; not in this plan.
- **Docs:** WEB.md "Keeping shells through a disconnect" leads with `rotari web --detach --allow-terminal
  --terminal-idle-timeout 24h`, how to find it again (run the same command), and how to stop it (`kill`), keeping
  tmux/screen as an alternative. FAQ's answer on surviving a closed tab points there.
  [ARCHITECTURE.md](../../docs/ARCHITECTURE.md) gains the detached process flow and the record directory.
- **Contracts:** a new WEB-40 for `--detach` (returns once listening and reports URL and PID; a failure to start is
  an error naming the log; the server outlives the starting session; a second `--detach` on the same host reports the
  running one or fails on different options; a crashed one does not count; no token in argv or the record).

## Steps

1. Readiness pipe and detached re-exec in `cmd/rotari`, sharing what fits with `server.Start` without exporting
   helpers that would let callers skip its checks; the flag; unit tests for flag conflicts and the token hand-off.
   The log's rotation at start and its copy-and-truncate while running, with unit tests at the size limit.
2. A lease helper both the supervisor and the Web server use, separating the `flock` lease from project directories
   in its own small package, whose probe reports errors instead of reading them as "not running". `server.Running`
   keeps its current behavior unless the user agrees to change it. Then the record directory (write, remove on exit,
   stale removal, current host only), the one-per-user rule, and the line plain `rotari web` prints, with unit tests
   for a running server, a crashed one, another host's record, differing options, and a probe error.
3. Conformance tests for WEB-40 through the binary: start with `--detach` and reach the URL after the parent exits;
   run it again and get the same PID; different options fail naming it; a port already in use fails with the log
   path; SIGTERM removes the record; after SIGKILL, `--detach` starts a new server. Each
   test kills only the PID it started.
4. Docs, generated views, contract row and note, and `scripts/check.sh`.

## Questions

Decided:

- **NFS** (agreed 2026-11-13): before relying on the lease, check that `flock` on the shared master directory
  behaves within one host on the clusters in use.
- **The log is bounded by rotation, not `rotari gc`** (agreed 2026-11-13): people rarely run `gc`, so nothing may
  depend on it to stay small. See proposal 9.

- **The token is not shown again** (agreed 2026-11-13): a generated token is never stored on disk, so a second
  `--detach` cannot print it and points to `ROTARI_WEB_AUTH_TOKEN` instead.
- **One detached server per user and host; plain `rotari web` unchanged apart from one line; no IDs yet; liveness by
  lease on the current host** (agreed 2026-11-13).

## Risks

- A detached server forgotten for weeks keeps a port and, with the terminal on, a way into the host behind its token.
  The record and the second `--detach` keep it visible; `rotari web` could later list running detached servers at
  start.
- If `flock` misbehaves on a master directory's file system, a running server could read as stopped and a second
  one start; the NFS check above, and the probe reporting errors, are meant to catch that.
