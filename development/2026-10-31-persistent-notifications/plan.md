# Plan: Persistent Notifications

Created: 2026-10-31
Status: Implemented (phases 1-8; unlock reporting was dropped); see [work-log.md](work-log.md).

## Purpose

The Web UI inbox only holds notifications detected while a page is open. The
page derives them by diffing successive `/api/state` and `/api/active-runs`
polls, so nothing is recorded while no page is open, and a reload clears the
inbox. Make notifications persistent: record them when they happen, whether
or not a page is open, and show the recorded history in the existing inbox.

Keep the current Web experience as unchanged as possible: the same Inbox
button, unread count, panel, Clear notifications, desktop popups, toggle,
`[browser]` settings, and per-basedir bells.

## Scope

1. Record every job-result and run-completion notification event in the
   master directory when the run process produces it.
2. Keep recorded events for 10 days.
3. Serve recorded events to the live Web UI, filtered and rendered with the
   project's current `[browser]` settings, and limited to the basedirs whose
   bell is selected.
4. Persist inbox read and cleared state in the master directory so it
   survives reloads.
5. Replace the page's poll-diff event detection with reading the record, on
   both the main Web UI and Recent jobs.

## Non-goals

- MCP access. The Python client reaches `rotari notifications` through its
  generated CLI schema, like other commands.
- Static exports: they keep the local toggle and have no inbox history.
- Changing Webhook delivery, its settings, or its payload.
- New `notifications.toml` settings. Recording is always on.
- Notifying abnormally terminated runs on other hosts, or to the Webhook (see
  Known limitations); `rotari web` records same-host ones for the inbox
  (phase 8).

## Current behavior

- Webhook events are produced in the run process. `projectrun.Runner` calls
  `JobFinished` once per job's final, diagnosis-enriched result
  (`internal/projectrun/execute.go`) and `RunFinished` after finalization
  (`internal/projectrun/lifecycle.go`). `cmd/rotari/webhook_batch.go` turns
  them into `notification.Event`s, batches them per run (ten-second window,
  flushed by run completion), filters them with the run's snapshotted
  `[webhook]` settings, and posts them.
- Browser events are produced in the page.
  `internal/webui/assets/web_app_notifications.js` diffs polls, applies the
  project's live `[browser]` settings from `/api/notification-settings`,
  formats fields in JavaScript (`notificationFieldValues`), merges one poll's
  events per run, and calls `addInboxNotification` and `new Notification`.
  The inbox (`web_shell.js`) keeps 50 entries in page memory.
  `jobs_template.html` (Recent jobs) has its own job-result detection.
- So the event rules (final results only, retries reported once, run success
  by exit code) and the field formatting exist twice, in Go and JavaScript.

## Decisions

### Producer: the run process

Record events from the same `JobFinished` / `RunFinished` hooks that feed
Webhooks. Recording in the Web server or the page cannot cover time when
they are not running. This also removes the JavaScript copy of the event
rules: the Web UI shows what the run process decided.

Finalization by repair (`internal/project/repair.go`, reached only from an
explicit `rotari unlock` or the MCP unlock tool) does not call `RunFinished`
today, so neither a Webhook nor, after this change, an inbox entry would
report such a run. The page's diff currently does notice it. Repair was at
first made to call `RunFinished` (and, later, releasing to report
`run.interrupted`), but on 2026-11-02 the user dropped both: whoever runs
`unlock` already knows the run stopped, so a notification at that point says
nothing new. `unlock` reports nothing, and SAFE-15's "never replays
notifications" stands as before.

### Unit of record: the existing batch

Record the same per-run batches the Webhook manager builds, before any
channel's filtering. One inbox entry is one batch, which matches today's
"one notification per run per poll" merging with a rule implemented once.

Consequence: a job result can appear up to ten seconds later than today's
two-second poll. A run's completion flushes its batch immediately, so run
completion notifications are not delayed. Accepted on 2026-10-31.

The batch manager is refactored so batching does not depend on a Webhook
URL; on flush it hands the unfiltered batch to two sinks: the inbox record
(always) and Webhook delivery (when a URL is set, filtered by `[webhook]`
as now).

### Storage: the master directory, one file per run

```text
<masterdir>/notifications/
  <run-id>.jsonl        # one line per batch, appended by that run's process
  inbox-state.json      # read and cleared cursors
```

- The master directory is the one place the Web server can read without
  scanning basedirs; run IDs are already registered there
  (`<masterdir>/runs/<run-id>.json`).
- One file per run gives each file one writer at a time (the run's
  supervisor, or later a repair), so concurrent runs never append to the
  same file. This matters on NFS, where `O_APPEND` is not atomic across
  clients. Each batch is one `write` of a complete line; readers ignore an
  incomplete trailing line.
- Each line holds the record schema version, a record ID, the batch time,
  the absolute basedir, project, run ID and name, and its events. Events
  store the values of every shared-vocabulary field except `link`, which is
  derived when read. The full `JobSpec` (environment, executor options) is
  not stored. `working_directory` and `diagnosis_evidence` are stored, since
  the selected fields are decided when read; files use the state file mode.
- Run IDs are validated as path elements at the store boundary
  (`state.IsValidPathElement`, `state.SafeJoin`).
- The store is a new package, `internal/notificationlog`, separate from the
  `internal/notification` domain types: it owns the file layout, append,
  listing, retention, and inbox state. `cmd/rotari` and `internal/webui`
  use it only through its exported API.

### Retention: 10 days

A run file whose last modification is older than 10 days is deleted. Each
append prunes expired files (best effort; a failure is a warning and does not
affect the run). Reads also ignore expired files, so retention does not
depend on pruning having run. A run still producing events keeps its file
fresh.

### Inbox state

Opening the inbox marks every entry read; Clear notifications hides every
current entry. Both are recorded as cursors in `inbox-state.json` (record
time plus record ID), written atomically, and only advanced. Each basedir has
its own pair (changed 2026-11-02): a single pair let reading a page that
showed basedir A mark older entries of an unshown basedir B read, so turning
B's bell on later hid them. Opening or clearing moves only the cursors of the
basedirs the page shows. The state is
deliberately shared (decided 2026-10-31) by every page that uses the same
master directory, so reading in one tab clears the unread count in others on
their next poll.

A server started with `--allow-control=false` shows the record but cannot
write the cursors; its read and cleared state stay in page memory, as today.

### Web server

- `GET /api/notifications?after=<cursor>&basedirs=<ids>` returns entries
  after a cursor (or the latest 50 when omitted) for the selected basedirs,
  each rendered with the project's current `[browser]` settings: event
  selection (`job_failure`, …), `fields` order, `max_jobs`, the 1000-character
  body limit, and the run link when `link` is selected. Entries left empty by
  the settings are omitted. It also returns the read and cleared cursors.
- `POST /api/notifications/read` and `POST /api/notifications/clear` advance
  the cursors; they require control.
- Rendering moves to Go and reuses the `internal/notification` field
  formatting used by Webhooks, so browser and Webhook values stay identical.
  Titles keep today's wording ("rotari: run failed (2 jobs failed)", …).

### Page behavior

- On load, the inbox shows recorded entries after the cleared cursor (at most
  50) with the persisted unread count.
- Each poll fetches entries after the newest one shown, adds them to the
  inbox, announces them to screen readers, and shows desktop popups for them.
  Entries present at page load never pop up, preserving "existing history
  never triggers a desktop notification".
- The basedir bells select which basedirs' entries are fetched, as they now
  select which basedirs are polled. Newly selecting a basedir loads its
  history into the inbox without popups.
- The `/api/state` and `/api/active-runs` polls stay for the page's own
  display; only notification detection stops using them.

### Implementation notes

- `rotari web` no longer injects `[browser]` settings into the page: the
  server renders entries. It still validates its basedir's
  `notifications.toml` at startup.
- Recent jobs shows the same entries as the main page, so its old titles
  ("rotari: job failed", "project / job") are replaced by the shared ones, and
  blocked or cancelled-before-start jobs are reported as failures there too.
- `/api/active-runs`, `/api/notification-settings`, and the notification-only
  parts of `/api/state` were kept for API clients; see ISSUES.md.

### Phase 7: `rotari notifications` (decided 2026-11-01)

A read-only CLI listing of the records, named like the other list commands
(`runs`, `jobs`) rather than `inbox`, because it does not manage read state.

- `rotari notifications [PROJECT]` with `-b/--basedir`, `-p/--project-name`,
  `--masterdir`, `--since` (default `1d`, as `runs` and `jobs`), `--unread`,
  and `--json`; basedir and project scope follow `rotari runs`
  (`projectRunListing` in `cmd/rotari/config.go`, `projectListingFilter`).
- Entries are the inbox's: `notificationlog.Entries` renders records with
  each project's current `[browser]` settings for both the Web handler and
  the CLI. Listed oldest first.
- Never writes the inbox state. `--unread` and the unread marks only read
  the shared read cursor; the cleared cursor does not hide records here.
- Contract NOT-3; conformance
  `TestNotificationsListRecordedNotificationsWithoutReadingThem`; the command
  joins the read-only flag-pair adapter.

### Phase 8: `rotari web` records interrupted runs (decided 2026-11-02)

Reporting at `unlock` was dropped (whoever unlocks already knows). Instead
the server, which may run while nobody looks, records them for the inbox
only (NOT-4):

- `runwatch.ReportInterrupted`, at startup and every 30 seconds from
  `cmd/rotari/web.go`, inspects every project of the server's basedir and
  the registered basedirs with `project.Inspect` under the shared state lock
  (a held lock skips the project until the next pass), so a starting or
  finishing run is never mistaken for an interrupted one.
- Each interrupted run gets one `run.interrupted` record through
  `notificationlog.Log.AppendOnce`, which checks the run's file under a lock
  in the notification directory, so several servers record it once.
- Selected by `run_failure`; never sent to the Webhook, since the server
  would then need to send once across servers and own outbound delivery.
- Limits: coordinators on other hosts count as running; nothing is found
  while no server runs (the next server's first pass finds it); an expired
  record is recorded again if the run is still interrupted.

## Known limitations to document

- Runs whose process resolves a different master directory than the Web
  server (`--masterdir`, `ROTARI_MASTERDIR`, `XDG_STATE_HOME`, or a different
  home on another host) are not shown. The poll diff showed any run in a
  watched basedir. With the default under `~/.local/state`, hosts that share
  a home directory share the record. Accepted on 2026-10-31.
- Runs started by an older rotari binary record nothing.
- A run process killed before flushing loses its pending batch (as Webhooks
  do today); its completion is not recorded.
- An abnormally terminated run (coordinator killed, host lost) cannot report
  itself. The page diff used to report such a run as failed when some
  command on the same host happened to remove its dead local lock while a
  page was open; since phase 8, `rotari web` records it for the inbox when
  its coordinator was on the server's host. Runs on other hosts, and the
  Webhook, remain unreported; see "Abnormally terminated runs on other hosts
  are never notified" in [ISSUES.md](../ISSUES.md).

## Implementation phases

1. **Store.** `internal/notificationlog`: record type, append, list after a
   cursor across run files with basedir filtering, retention, cursors.
   Unit tests: concurrent runs, incomplete trailing line, invalid run IDs,
   expiry, cursor monotonicity.
2. **Producer.** Refactor `webhookNotificationManager` into a notification
   dispatcher with inbox and Webhook sinks. Keep existing Webhook tests
   unchanged and passing; add tests that a run without a Webhook URL still
   records. (Unlock reporting, added here at first, was removed on
   2026-11-02.)
3. **Server.** Notification endpoints, Go rendering with `[browser]`
   settings, control gating, basedir filtering. Handler tests.
4. **Pages.** Replace poll-diff detection in `web_app_notifications.js` and
   `jobs_template.html`; load history into the inbox; persist read/clear.
   Update `internal/webui/run_notifications_test.go` and the existing
   `TestBrowserNotifications…` tests to the new source of events.
5. **Docs and contracts.** `docs/NOTIFICATIONS.md` (in-page inbox section),
   `docs/WEB.md`, contract sections in `contracts/02-run-lifecycle-and-execution.md`
   (NOT-1, recording and retention) and
   `contracts/05-web-assets-and-static-export.md` (inbox persistence, first
   poll rule), the master directory layout in `docs/ARCHITECTURE.md`, and the
   `contracts/README.md` status table.
6. **Conformance.** Through the binary: finish a run with no Web server
   running, then start `rotari web` and see its entry in
   `/api/notifications`; read state survives a server restart; an entry older
   than 10 days is not returned; `--allow-control=false` rejects cursor
   writes. Update `conformance/05-web/notifications_test.go`.

## Validation for completion

- `go test ./...` and the conformance suite pass.
- Manual check in `rotari web`: close the page, let a run fail, reopen; the
  entry is in the inbox, unread, with no desktop popup; a new failure while
  open pops up.
