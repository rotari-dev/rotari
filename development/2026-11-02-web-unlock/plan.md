# Plan: Recover Interrupted Runs from the Web UI

Created: 2026-11-02
Status: Implemented (phases 1-4); see [work-log.md](work-log.md). A manual
check in `rotari web` remains.

## Purpose

Let a user recover an interrupted run (`rotari unlock`, SAFE-4) from the
live Web UI, with the same assessment, safeguards, and outcome as the CLI
and MCP. Since [persistent notifications](../2026-10-31-persistent-notifications/plan.md),
`rotari web` records interrupted runs in the inbox (NOT-4); a user who opens
that entry lands on the run page and should be able to recover the run there
instead of switching to a terminal.

## Background

The [verified-unlock plan](../2026-10-20-verified-unlock/plan.md) excluded
Web unlock (2026-10-20): adding Web controls "would expand the interface and
testing scope without resolving the ownership and termination questions",
and Web assessment was left as an open extension. Those questions are now
answered in one shared implementation, `project.Unlock`
([unlock.go](../../internal/project/unlock.go)), which the CLI and the MCP
tools both call:

- A dry run (`Guard{DryRun: true}`) returns the full `UnlockResult`:
  `action` (`finalize` from a complete trustworthy summary, or `release`),
  the coordinator assessment, `repair` with every managed execution's
  evidence, `jobs_may_be_running`, `allowed`, `refusal_code`, and the
  project `revision`.
- An apply is guarded by the previewed revision (`Guard{IfRevision}`) and
  reassesses under the state lock.
- Refusals are typed: `coordinator_live` and `execution_active` can never be
  overridden; `coordinator_unknown` and `execution_unknown` need explicit
  attestation (`--assume-stopped` / `assume_stopped`) that the user
  confirmed the work stopped.
- SAFE-7 promises that CLI and MCP previews show the same ownership and
  execution evidence.

So the Web needs an adapter and a careful UI, not new recovery rules.

## Scope

1. A Web API that previews and applies unlock for one project through
   `project.Unlock`, with the same inputs and result as MCP
   (`rotari_preview_unlock` / `rotari_unlock`).
2. A **Recover…** control on the run page (and the project page) of an
   interrupted run, opening a dialog that shows the preview and applies it.
3. Contracts and conformance that put the Web beside the CLI and MCP in
   SAFE-4 and SAFE-7, and documentation.

## Non-goals

- Changing what unlock decides, its refusal rules, or its outcome.
- Unlock from a static export or a read-only server.
- Retrying the run from the dialog (decided 2026-11-02). After a release the
  dialog only names the `retry --run-id` command, as the CLI does.
- Notifications on unlock (dropped on 2026-11-02: whoever unlocks a run
  already knows it stopped).

## Decisions

### API: preview and apply, as MCP

- `GET /api/unlock?project_name=NAME[&run_id=ID][&assume_stopped=true]`
  returns the dry-run `UnlockResult` as JSON. It is read-only and available
  on any live server, like the other `GET` routes; a project with no
  interrupted run answers `allowed: true` with no `run_id`, as the CLI's dry
  run does.
- `POST /api/unlock` with
  `{"project_name", "run_id", "if_revision", "assume_stopped"}` applies it.
  `if_revision` is required (as MCP requires it) and comes from the preview;
  a changed project answers 409 with the error, so the dialog previews again.
  A refusal answers 409 with the result and its `refusal_code`, not a
  success-shaped body. Without control (`--allow-control=false`) it answers
  403 like every other control endpoint.
- Both routes are bound to the basedir in the URL mount, like the other
  project routes, and validate `project_name` and `run_id` as path elements
  (`/` and `\` refused).
- The handler lives in `internal/webui` and only translates HTTP to
  `project.Unlock`; it does not reimplement any policy. If the result needs a
  presentation shared with MCP (field names, the release hint), move that
  into one owning function rather than copying it.

### UI: a dialog that shows the evidence before acting

- The control appears on both the run page and the project page of an
  interrupted run (decided 2026-11-02: an interrupted project blocks new runs,
  so its page is where a user notices), only on a live server with control
  enabled; on a read-only server it is
  absent or disabled with the reason, as other control buttons are.
- Opening it calls the preview and shows:
  - what will happen: **Finalize** (the run is marked finished from its
    recorded results) or **Release** (the project returns to idle; the
    run's unfinished jobs can be rerun with `retry --run-id ID`);
  - the coordinator state and reason, and each managed execution's state
    (`stopped`, `active`, `unknown`) with its source and observation time;
  - why recovery is refused, in words, when it is.
- `coordinator_live` or `execution_active`: the apply button stays
  disabled; the dialog says the run or a job is still running.
- `coordinator_unknown` or `execution_unknown`: the apply button is enabled
  only after the user ticks "I confirmed that the coordinator and all of the
  run's jobs have stopped", which re-runs the preview with
  `assume_stopped=true` and then applies with it. The wording states that
  rotari cannot verify this.
- After applying, the page reloads the project and run, and the dialog shows
  the outcome (finalized, or released with the retry command).
- The dialog never sends `assume_stopped` without the tick, and never
  applies without a preview whose revision it sends.

### Interfaces

- CLI and MCP: unchanged.
- Python client: unchanged (it wraps the CLI).
- Static export: no control and no API, as for other control actions.

### Implementation notes

- The Web UI has no read-only flag on the page; every control button is
  shown and a read-only server answers 403. Recover follows that: on a
  `--allow-control=false` server the dialog still previews, and applying
  shows the 403 reason. Contract WEB-25.
- The dialog reuses the output modal (`recover-view`).

## Implementation phases

1. **API.** Handlers for `GET` and `POST /api/unlock`, with tests for
   preview, apply, revision mismatch (409), refusals (409 with code),
   read-only (403), invalid names, and no interrupted run.
2. **UI.** The Recover dialog on the run and project pages; JSDOM tests for
   each refusal code, the attestation tick, the revision round trip, and the
   read-only state. Prettier and the `tsc` check pass.
3. **Contracts and docs.** SAFE-4 and SAFE-7 name the Web; the Web contract
   documents the routes and the dialog; `docs/WEB.md`,
   `docs/OPERATIONS.md` (unlock section), and the WEB-21/NOT-4 notes that an
   inbox entry leads to a recoverable run page; `docs/ARCHITECTURE.md` if the
   handler adds a shared presentation function.
4. **Conformance.** Extend the unlock conformance table
   (`TestUnlockFinalizationRepair`, `TestUnlockRefusesLiveExecutions`,
   `TestUnlockOwnershipAndDryRun`) so each variant runs over CLI, MCP, and
   the Web API and gets the same preview evidence and outcome (SAFE-7),
   including a refused apply leaving state unchanged.

## Validation for completion

- `go test ./...` and the conformance suite pass, with the Web variants in
  the unlock tables.
- Manual check in `rotari web`: kill a run's supervisor, open the inbox
  entry, recover the run from the dialog for both a finalize and a release
  case, and see a live-execution case refused.
