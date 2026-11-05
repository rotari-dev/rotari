# Work History: A Built-in Terminal in rotari web

See [plan.md](plan.md) for current scope and status.

## Plan the terminal

**Commits:** 2026-11-05 10:12:30 `ee45fc98`.

**Change:** Added `plan.md`.

**Reason:** The user asked for a plan without deciding whether to build it. It is separate from [the hardening plan](../2026-11-04-web-terminal/plan.md), whose hardening is done, under one plan per feature.

**Plan impact:** Replaces that plan's phases 3 to 5. Compared with them, it drops the token prompt on each opening and allows combining with `--allow-exec`, since signing in is the entrance and a launched service is already trusted.

**Validation:** Documentation only; no tests run. Relative links were checked to exist.

**Remaining:** The three questions under "Before building". Not checked: the Go modules' transitive dependencies, Safari's `connect-src` handling of WebSocket, and whether the VS Code and Jupyter Lab terminals work.

## Run shells on pseudo-terminals

**Commits:** 2026-11-05 15:18:04 `1a7a0fa9`.

**Change:** Added `internal/terminal` with its tests, its rule in `internal/archtest` and row in `docs/ARCHITECTURE.md`, and `github.com/creack/pty`, which has no dependencies. `golang.org/x/sys` became a direct dependency.

**Reason:** The first small step of phase 1, chosen by the user: sessions alone, before any HTTP.

**Plan impact:** The user decided to go ahead a step at a time. Found that the `pty` package's blocking file stopped `Close` from hanging up during a `Read`, which every session has, and recorded the constraint.

**Validation:** `go test ./internal/terminal` passed, also five times under `-race` and five on one CPU, and the bug's fix was shown to matter by removing it: the test then took the whole grace period and saw SIGKILL. After rebasing, `go vet`, `go build ./...`, and `go test ./internal/terminal ./internal/archtest ./conformance` passed. Bash passed the basic tests; zsh exited with status 1 on hangup, and its `nohup` case was not checked. `GOOS=darwin go vet` passed, but nothing ran on macOS. Full `go test ./...` was not run, since nothing reaches this package yet.

**Remaining:** The WebSocket handler and `--allow-terminal`, then phases 2 and 3; `Close` on macOS.

## Plan a panel beside the jobs

**Commits:** 2026-11-05 15:34:52 `d73f498a`.

**Change:** Rewrote `plan.md` around a bottom panel, a shell kept on the server that pages reattach to, one shell per server with take-over, and a 30-minute limit on an unattended shell.

**Reason:** The user pointed out that a separate tab adds little over a real terminal, so being beside the jobs is the feature. The UI reloads on every link, so a page cannot hold a shell.

**Plan impact:** Replaced decisions on a separate page and on ending with the connection. The estimate rose to eight to twelve days. The user accepted the risk of a shell outliving its connection, as a launched VS Code server does.

**Validation:** Documentation only; no tests run. Relative links were checked to exist.

**Remaining:** Steps 1 to 5 of the plan.

## Keep one shell across connections

**Commits:** 2026-11-05 15:45:49 `bcf4a6f1`.

**Change:** Added `Keeper` and `Attachment` in `internal/terminal/keeper.go` with tests, updated `docs/ARCHITECTURE.md`, and noted the design change and provisional limits in `plan.md`.

**Reason:** Step 1 of the plan: pages reload on every link, so the shell has to outlive its connections.

**Plan impact:** Step 1 is done. An attached reader now applies backpressure. The first design detached a reader whose queue filled, which a test showed cut off a healthy reader within a second of `yes`. The user noted the limits are provisional and will become configuration.

**Validation:** `go test ./internal/terminal` passed, also five times under `-race` and three on one CPU in the final form, and five under CPU load before the last test change. Disabling stall detection, the redraw, or the wait for a reader each failed a test; the last at first did not, so that test was changed to check how much output piles up. After rebasing, `go vet` and `go test ./internal/terminal ./internal/archtest ./conformance` passed. Nothing ran on macOS, and full `go test ./...` was not run, since nothing reaches this package yet.

**Remaining:** Steps 2 to 5; how full-screen programs redraw in practice; `Close` on macOS.

