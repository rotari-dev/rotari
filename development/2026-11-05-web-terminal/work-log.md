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

## Serve the shell over a WebSocket

**Commits:** 2026-11-05 16:07:17 `da294e38`.

**Change:** Added `internal/webui/terminal.go` with `/_terminal/ws`, `--allow-terminal` in `cmd/rotari`, shell start and end logging, ending the shell on sign-out and server stop, `github.com/coder/websocket`, and the generated CLI files. The flag pair tests and schema golden gained the flag.

**Reason:** Step 2 of the plan.

**Plan impact:** Step 2 is done. Conformance for it moves to step 5, with the contract.

**Validation:** `go test ./...` and `go vet ./...` passed, and `go test -race` on the changed packages passed before the last test records changed. A throwaway test through the built binary showed no Web token in the shell's environment, and that SIGINT, SIGTERM, and SIGKILL of the server end the shell while a `setsid` process survives. After rebasing, the changed packages' tests passed again. No browser or macOS run.

**Remaining:** Steps 3 to 5.

## Fix a race in a terminal test

**Commits:** 2026-11-05 17:14:37 `f78833df`.

**Change:** `TestCloseLeavesProcessesThatIgnoreTheHangup` in `internal/terminal/session_test.go` waits for the process to report after `nohup` has started it, sharing `readPID` with its neighbor.

**Reason:** CI failed once on `main`: the test could close the terminal before `nohup` ignored the hangup, and the process then rightly died.

**Plan impact:** None.

**Validation:** 400 repetitions on one CPU did not reproduce it. Delaying `nohup` by 200 ms made the old test fail exactly as on CI, and the new one passed with the same delay. The two `TestCloseLeaves` tests passed 100 repetitions, also on one CPU, and the package passed twice under `-race`.

**Remaining:** None.

## Show the terminal in a panel

**Commits:** 2026-11-05 17:14:43 `1edafb3c`.

**Change:** A Terminal button and panel in `web_shell.js` and `web_shell.css`, xterm.js 5.5.0 and its fit addon vendored after checking npm's SHA-512, served from `/_terminal/`, `livePage` carrying the nonce and the button, `TestTerminalPanel`, and `TestTerminalPanelInBrowser` with WEB-33.

**Reason:** Step 3 of the plan.

**Plan impact:** Step 3 is done. It stays off `main` until the browser test has run on CI.

**Validation:** `go test ./...`, `go vet`, and `tsc` passed; the browser test skips without Chrome. The user tried it by hand: the shell, `vim` across a link, `less`, Japanese input, copy and paste, and macOS.

**Remaining:** Steps 4 and 5; the browser test's first run.

## Finish the panel

**Commits:** 2026-11-05 17:17:21 `02ff9b58`.

**Change:** Height by drag and arrow keys, kept by the browser; colors from new `--terminal-*` tokens; dialogs ending where the panel begins; status text for each wait and for a failed start; `AttachIfFree` and `takeover=0`; tabs not visible neither connecting nor retrying; stopping when the sign-in has gone.

**Reason:** Step 4 of the plan, and what the user's trial found: with many old tabs, a new tab sat at "Connecting" for a long time, with nothing shown until status text was added.

**Plan impact:** Step 4 is done. Automatic reconnection no longer takes the shell, which it could have from the tab in use. That browsers handle WebSocket handshakes one at a time is from memory and was not checked; the user reported the startup fine after the change.

**Validation:** `go test ./...`, `go vet`, and `tsc` passed. Letting background tabs retry, or reconnection take over, each failed a test. Real token values through real xterm.js in jsdom showed no error. The user tried it by hand: startup, dragging, light and dark, typing with a log open, Japanese input, and take-over by a second tab.

**Remaining:** Step 5; the browser test's first run.

## Promise and document the terminal

**Commits:** 2026-11-05 17:28:07 `d9be68b8`.

**Change:** `conformance/05-web/terminal_test.go`, contract WEB-34, and `docs/WEB.md`, `docs/SECURITY.md`, `docs/ARCHITECTURE.md`, and `docs/FAQ.md`.

**Reason:** Step 5 of the plan.

**Plan impact:** All steps are implemented.

**Validation:** `go test ./...` and `go vet ./...` passed, and the new tests passed five times on one CPU. `-race` over them was run but its result was not read. The idle limit is checked at the package level, since 30 minutes cannot be shortened through the binary.

**Remaining:** Push the branch so `TestTerminalPanelInBrowser` and the policy check run on CI, then integrate into `main`. Trying it behind an HTTPS reverse proxy and in Safari, once the sign-in fix is in.

