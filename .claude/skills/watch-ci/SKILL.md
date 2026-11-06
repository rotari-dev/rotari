---
name: watch-ci
description: "Use after pushing rotari to wait for GitHub Actions, or to check how CI ended for a commit, including whether browser tests such as TestTerminalPanelInBrowser passed."
---

# Watching rotari's CI

Tests that need Chrome are skipped here, so CI is the first place they run.

1. Get the full commit ID that was pushed: `git rev-parse <revision>`. From a
   non-colocated workspace, run this in the default workspace after
   `jj git export`.
2. Run `python3 .claude/skills/watch-ci/watch_ci.py <SHA>` as a background
   command, and carry on. You are told when it ends; do not poll it, and do not
   start a second watcher for the same commit.
3. Read how it ended: 0 means every run succeeded, 1 that some did not (their
   jobs and steps are listed), and 2 that it could not tell in time. Report
   which it was and the runs' links.

## What it can and cannot say

- It reads the public API without signing in, so it sees conclusions and not
  logs. Say that a test passed only by inference: the `go` job of Go CI sets
  `ROTARI_REQUIRE_BROWSER_TESTS`, so browser tests cannot skip there. To say
  what a job printed, ask the user for the log.
- A job failing with steps that have no conclusion usually means the runner was
  lost rather than a test failing. Ask the user to look; do not guess from it.
- It cannot start a run again. Ask the user to press **Re-run failed jobs**.
- A network's sign-in page can answer in place of GitHub, which the script
  reports and retries. Do not work around it. Tell the user if it lasts.
- Requests without signing in are limited to about 60 an hour per address, shared
  with anything else there, so several watchers at once can run out.
