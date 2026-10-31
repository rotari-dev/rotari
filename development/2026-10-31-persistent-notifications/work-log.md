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
