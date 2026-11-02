# Work History: Recover Interrupted Runs from the Web UI

See [plan.md](plan.md) for current scope and status.

## Plan Web unlock

**Commits:** 2026-11-02 10:27:08 `23cdb952`.

**Change:** Added `plan.md`.

**Reason:** The user proposed Web unlock after interrupted runs started reaching the inbox (NOT-4).

**Plan impact:** Decided with the user: Recover on both the run and project pages; the dialog shows the retry command after a release but does not retry.

**Validation:** Documentation only; no tests run.

**Remaining:** Implementation.

## Recover interrupted runs from the Web UI

**Commits:** 2026-11-02 10:40:40 `abb4203cacde`.

**Change:** New `internal/webui/unlock.go` (`GET`/`POST /api/unlock` over `project.Unlock`, with `refusal` and `rerun_command`) and the Recover dialog in `web_app_actions.js` (shown from `runActionsHTML` and `projectRunsHTML` for interrupted runs, in the output modal's new `recover-view`). Tests: `TestWebUnlockPreviewsAndAppliesThroughProjectUnlock`, `TestRecoverDialogFollowsThePreview`, Web variants of `TestUnlockFinalizationRepair`, and a Web case in `TestUnlockRefusesLiveExecutions`. Contracts WEB-25 (new), SAFE-4, SAFE-7; `docs/OPERATIONS.md`, `WEB.md`, `ARCHITECTURE.md`; plan status.

**Reason:** Plan phases 1-4.

**Plan impact:** All phases done. As other control buttons, Recover is shown on a read-only server, which answers the apply with 403 (recorded in the plan).

**Validation:** `go vet ./...`, `gofmt`, `prettier --check`, and `tsc` passed. `go test` passed for `internal/webui`, `internal/project`, `internal/mcp`, `cmd/rotari`, `conformance`, `conformance/03-interfaces/...`, `conformance/04-coordination`, and `conformance/05-web`; `-race` passed for `internal/webui`. The full suite was not rerun, and the manual check in `rotari web` has not been done.

**Remaining:** The manual check in the plan's validation section.
