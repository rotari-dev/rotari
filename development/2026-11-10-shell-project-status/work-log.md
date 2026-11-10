# Work log: Each project's state in All shells

## Steps 1 and 2: the route and the heading line

**Commits:** 2026-11-10 16:34:27 `9532482c`.

**Change:** [terminal_projects.go](../../internal/webui/terminal_projects.go) serves `GET /_terminal/projects`: for
each project with shells, its run count and its last run, chosen by `resolve.RunID` as `rotari show` chooses it,
with its lifecycle (`runview.RunLifecycleLabel`), that label's tone, its start and finish times, and its jobs
(`internal/web.LoadRunJobs`) counted by a Go `jobCountGroup` in [status.go](../../internal/webui/status.go), the twin
of the run page's; or `status_error`. In [web_shell.js](../../internal/webui/assets/web_shell.js), `drawOverview`
puts a `.terminal-overview-project-state` line under each project's heading and `drawProjectStates` fills it once per
draw; the line is a button that opens the run's page, as choosing a shell of another project goes there.
[web_shell.css](../../internal/webui/assets/web_shell.css) styles it. WEB-33, WEB-34, their row,
[WEB.md](../../docs/WEB.md), and [ARCHITECTURE.md](../../docs/ARCHITECTURE.md) describe it.

The first version took the newest run by its ID. The conformance test failed one time in a few: its two runs began
in the same second, and IDs of one second sort by their random tail. `resolve.RunID` reads the project's recorded
last run instead, which `project.Overviews` (`rotari projects`, MCP) also names. Those overviews count failures from
the summary's results, which can differ from the Jobs card for a blocked job; the line follows the card, since it
links to the run page.

The card's own `.terminal-overview-state` class was taken, so the line's classes are `terminal-overview-project-state*`.

**Reason:** [The plan](plan.md).

**Plan impact:** Proposal 1 names `resolve.RunID` rather than the newest run directory.

**Validation:** `TestTerminalProjectsSayWhatTheLastRunIsDoing` (a fixed failure not counted, a new one counted, no
runs, unreadable runs, no entry for shells without a project, 405 on POST), `TestJobCountGroupMatchesInGoAndJS`,
`TestTerminalPanel` (the lines' text, tone, dimmed zeros, one read per draw and none from the five-second list, an
error from the route, a hidden line for no project, and the window's click telling its page to open
`/project/p1/run/r%201`), and `TestTerminalProjectsSayWhatTheirLastRunDid` in conformance (the built binary; five
runs).

**Remaining:** Trying it by hand; the browser test was not extended.

## Follow-up: an ended run says when it ended

**Commits:** None yet.

**Change:** Recording the promo video showed `Last run finished, finished just now`. In
[web_shell.js](../../internal/webui/assets/web_shell.js), `projectStateButton` now says when an ended run ended,
`Last run failed 2h ago`, and when one still going started, `Last run running, started 12m ago`. WEB-33 and the plan
say so.

**Reason:** The word repeated the lifecycle for finished runs.

**Plan impact:** Proposal 4 gives the ended-run form.

**Validation:** `TestTerminalPanel` (`Last run failed 2d 1h ago`), and the promo capture's frames.

**Remaining:** None.
