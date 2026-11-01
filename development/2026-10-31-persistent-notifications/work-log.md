# Work History: Persistent Notifications

See [plan.md](plan.md) for current scope and status.

## Plan persistent notifications

**Commits:** 2026-10-31 08:19:09 `227493d6`.

**Change:** Added `plan.md` and the `ISSUES.md` entry "Abnormally terminated runs are never notified".

**Reason:** The Web inbox only held events the page detected by diffing polls, so nothing was kept while no page was open and a reload cleared it.

**Plan impact:** Decided with the user: record batches from the run process under the master directory, one file per run, for 10 days; accept up to ten seconds of delay for job entries; share read state; report the completion on unlock repair; accept that runs under another master directory are not shown; leave abnormally terminated runs out of scope.

**Validation:** Documentation only; no tests run.

**Remaining:** Implementation (phases 1-6).

## Record notifications and show them in the inbox

**Commits:** 2026-10-31 09:03:57 `d91d35ef`.

**Change:** New `internal/notificationlog` (append, read with a per-file cache, retention, shared cursors). `internal/notification` gained `Event.Recordable` and `NewBrowserMessage`. `cmd/rotari`'s Webhook batch manager became `runNotificationManager`, which records every batch before channel selection and still sends Webhooks; `project.Guard.RunFinished` lets CLI and MCP unlock report a repaired run's completion. `internal/webui` serves `GET /api/notifications` and `POST /api/notifications/state` and no longer injects browser settings into the page. `web_shell.js` now owns the inbox poll for both pages; `web_app_notifications.js` and `jobs_template.html` lost their poll-diff detection, and the Recent jobs rows lost the data attributes only that used. Contracts NOT-2 (new), WEB-21, SAFE-15 and the related Web text, `docs/NOTIFICATIONS.md`, `OPERATIONS.md`, `MCP.md`, `FAQ.md`, and `ARCHITECTURE.md` were updated. JS tests for the removed diff logic were replaced by Go tests of the same rules and by inbox tests.

**Reason:** Plan phases 1-6.

**Plan impact:** All phases done. Recent jobs now shows the shared entry text, and blocked or cancelled-before-start jobs count as failures there. SAFE-15's "never replays notifications" now excepts the run's single completion. `/api/active-runs`, `/api/notification-settings`, and the notification-only parts of `/api/state` were kept and recorded in `ISSUES.md`.

**Validation:** `go vet ./...` and `tsc -p internal/webui/tsconfig.json` passed. Full `go test -timeout 15m ./...` passed after fixing `TestCLIFlagPairUnlock*`, which compare the whole state tree and now allow and check the notification record. In the race step, `cmd/rotari` hit the known webhook-timer/color race (one of three repeated `-race` runs; recorded under the existing `ISSUES.md` entry), and `TestSSHJobReadsAsRunning` failed once and then passed three isolated `-race` runs (new `ISSUES.md` entry). `prettier --check` (3.9.8, not the pinned v4 alpha) and `gofmt -l` were clean; `pre-commit` is not installed.

**Remaining:** A CLI command to read the records (separate plan); "Abnormally terminated runs are never notified" and "Notification-only Web API routes have no page client" in `ISSUES.md`.

## Add `rotari notifications`

**Commits:** 2026-11-01 22:18:00 `d19e20c1`.

**Change:** New `cmd/rotari/notifications.go` and its command spec, dispatch, completion and config scope (`projectRunListing` now names `runs` and `notifications`; `projectListingFilter` is shared with `runs`). `notificationlog.Entries` now renders records for both the Web handler and the command. Contract NOT-3, `docs/NOTIFICATIONS.md`, `INSPECT.md`, `ARCHITECTURE.md`, the flag-pair coverage document, regenerated `CLI_REFERENCE.md`, `ENVIRONMENT_VARIABLES.md` and `python/rotari/generated_cli.py`, the help/schema goldens, and the flag-pair inventory (read-only adapter). The Web inbox conformance test now waits for its condition instead of a fixed 200 ms.

**Reason:** Plan phase 7, decided with the user on 2026-11-01: name `notifications`, read only, `--unread` to narrow to unread.

**Plan impact:** All planned phases are done.

**Validation:** Generator `--check` runs, `ruff check`/`ruff format --check`, `gofmt`, and `go vet ./...` passed. In the full `go test ./...` at a load average near 40, `TestCLIFlagPairSuspend` timed out (new `ISSUES.md` entry; an isolated rerun passed in 15 s) and `TestWebNotificationInboxWithoutDesktopPermission` failed once, which led to the wait fix (three isolated repetitions then passed). The race step failed only in `TestCLIFlagPairSuspend`. Every other package passed. Python tests were not run: `pytest` is not installed.

**Remaining:** `ISSUES.md` entries "Abnormally terminated runs are never notified" and "Notification-only Web API routes have no page client".

## Keep inbox read state per basedir

**Commits:** 2026-11-02 07:40:07 `0935278f`.

**Change:** `notificationlog.InboxState` now maps each absolute basedir to its `Cursors` (read and cleared), and `Advance` takes per-basedir changes. `/api/notifications` names each entry's basedir ID and returns the requested basedirs' cursors; `/api/notifications/state` takes and returns `{"basedirs": {ID: cursors}}`. `web_shell.js` keeps server and page cursors per basedir and moves only those of the basedirs it shows; `rotari notifications` compares each entry with its basedir's read cursor. WEB-21, NOT-3, `docs/NOTIFICATIONS.md`, `ARCHITECTURE.md`, and the plan were updated; store, handler, JS, and conformance tests now check that reading one basedir leaves another's older entries unread.

**Reason:** Found by the user's question on 2026-11-02: with one cursor pair, reading or clearing a page that showed basedir A also read or cleared older entries of an unshown basedir B.

**Plan impact:** Changes the inbox-state decision (shared, now per basedir). A state file written by the earlier single-pair format is read as empty, so every kept notification shows unread once; that format never left local `main`.

**Validation:** `go vet ./...`, `gofmt`, and `prettier --check` passed. `go test` passed for `internal/notificationlog`, `internal/notification`, `internal/webui`, `cmd/rotari`, `conformance`, `conformance/02-lifecycle`, `conformance/05-web`, and `conformance/03-interfaces/...`; `-race` passed for `internal/notificationlog` and `internal/webui`. The full suite was not rerun.

**Remaining:** As before.

## Report runs that unlock releases

**Commits:** 2026-11-02 08:35:04 `b0a404df`.

**Change:** `internal/notification` gained the `run.interrupted` event (`NewInterruptedRunEvent`), selected by `run_failure` and titled "rotari: run interrupted" in the inbox. `project.Guard.RunInterrupted` is called when `unlock` releases a run, with the run's name and start time from its partial summary or lock; `cmd/rotari` unlock and the MCP unlock tools pass `runNotificationManager.RunInterrupted`, which records and sends at once. Webhook titles it "rotari run interrupted" in the failure color. NOT-2, SAFE-4, `docs/NOTIFICATIONS.md`, `OPERATIONS.md`, `MCP.md`, the plan, and the `ISSUES.md` entry (renamed "Abnormally terminated runs are notified only when unlocked") were updated.

**Reason:** Plan phase 8, requested by the user on 2026-11-02 as a first step toward notifying abnormally terminated runs.

**Plan impact:** Abnormal termination is now reported when `unlock` recovers the run; reporting it when it stops stays open.

**Validation:** `go vet ./...` and `gofmt` passed. `go test` passed for `internal/notification`, `internal/notificationlog`, `internal/project`, `internal/mcp`, `internal/webui`, `cmd/rotari`, `conformance`, `conformance/02-lifecycle`, `conformance/05-web`, and `conformance/03-interfaces/...` (including `TestUnlockFinalizationRepair`, which now checks one `run.interrupted` record per released variant over CLI and MCP); `-race` passed for `internal/project` and `internal/notification`. The full suite was not rerun.

**Remaining:** The `ISSUES.md` entry for reporting the stop itself, and "Notification-only Web API routes have no page client".
