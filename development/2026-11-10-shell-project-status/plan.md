# Plan: Each project's state in All shells

Created: 2026-11-10
Status: Done in `9532482c`, not yet tried by hand. See the [work log](work-log.md). The user asked for All shells to tell what each project is
doing, and agreed to three job counts and the last run, on one line beside the project's name.

## Goal

Choosing a shell in All shells is often choosing a project to look at. Each project heading says what its last run
is doing, in one line, so that the project with a failure, or the one still running, stands out before a shell is
chosen. The run page has the details; the line links to it.

## Existing implementation

- The overview ([web_shell.js](../../internal/webui/assets/web_shell.js), `drawOverview`) heads each project's cards
  with its name, `+`, and End all. It reads `GET /_terminal/shells`, every five seconds while the panel is open.
- `/api/projects` gives project names only. The run page's Jobs card (`runCardsHTML`, web_app_core.js) counts a run's
  jobs by `jobCountGroup`: succeeded, failed, in progress, pending, blocked, other. The tone of a displayed status
  is `statusTone`, the same table in Go and JS (status.go, `TestStatusToneMatchesInGoAndJS`).
- A run's jobs and their displayed statuses come from `internal/web.LoadRunJobs` (`ExecutionStatus`, the
  `internal/jobstatus` chain); its lifecycle label from `runview.RunLifecycleLabel`.
- The overview is drawn on pages that do not load web_app_core.js (Recent jobs, the terminal window).

## Proposals

1. **The last run** is the one `rotari show` reads when given none (`resolve.RunID`): the last run started, as
   `rotari projects` names it, so the one going on while one is. Run IDs start with the second they began, so
   sorting them by name is not enough for two runs in the same second. Its state is its lifecycle label (running,
   finished, failed, interrupted, cancelled, incomplete), when it started, and when it finished.
2. **Three counts** of its jobs, grouped as the run page's Jobs card groups them: in progress, pending, and failed.
   A Go `jobCountGroup` beside `statusTone` does the grouping, and a test keeps it in step with the JS one, so the line
   and the card never disagree. Old runs' failures are not counted: a later run that fixed them shows none.
3. **`GET /_terminal/projects`** gives, for each project with shells, its last run's state, or `runs: 0`, or
   `status_error` when its runs cannot be read. It is read once each time the overview is drawn (opening, Refresh, and
   after the overview's own actions), not with the five-second list, since reading runs is far heavier than listing
   shells.
4. **The heading line** reads `Last run running, started 12m ago · 3 in progress · 5 pending · 2 failed`, in the
   run's tone, with zero counts dimmed; `No runs yet` for a project without runs, and `Its state could not be read:
   …` on an error. Clicking it opens the run's page and closes the overview, as choosing a shell of that project
   would. Shells without a project have no line.

## Steps

1. The route, its Go grouping and tests (`TestTerminalProjectsSayWhatTheLastRunIsDoing`,
   `TestJobCountGroupMatchesInGoAndJS`).
2. The heading line and its link, with `TestTerminalPanel`; WEB-33, WEB-34, `docs/WEB.md`, and a conformance test.
