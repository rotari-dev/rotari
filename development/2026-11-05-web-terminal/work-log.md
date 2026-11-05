# Work History: A Built-in Terminal in rotari web

See [plan.md](plan.md) for current scope and status.

## Plan the terminal

**Commits:** 2026-11-05 10:12:30 `ee45fc98`.

**Change:** Added `plan.md`.

**Reason:** The user asked for a plan without deciding whether to build it. It is separate from [the hardening plan](../2026-11-04-web-terminal/plan.md), whose hardening is done, under one plan per feature.

**Plan impact:** Replaces that plan's phases 3 to 5. Compared with them, it drops the token prompt on each opening and allows combining with `--allow-exec`, since signing in is the entrance and a launched service is already trusted.

**Validation:** Documentation only; no tests run. Relative links were checked to exist.

**Remaining:** The three questions under "Before building". Not checked: the Go modules' transitive dependencies, Safari's `connect-src` handling of WebSocket, and whether the VS Code and Jupyter Lab terminals work.
