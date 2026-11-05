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


## Open several shells in tabs

**Commits:** 2026-11-05 20:26:14 `02a5a3ad`.

**Change:** `internal/terminal/keeper.go` holds the shells itself, up to a limit, with `Start`, `Shells`, `End`, and `EndAll`; `Attach` names a shell and never starts one. `internal/webui/terminal.go` adds `/_terminal/shells` and `/_terminal/shells/ID`, close code 4005, and drops the `end` control message. `web_shell.js` and `web_shell.css` add the tabs. `cmd/rotari` sets a provisional limit of eight and ends all shells on signing out. Contracts WEB-33 and WEB-34, `docs/WEB.md`, `docs/SECURITY.md`, `docs/ARCHITECTURE.md`, and the plan's [Tabs](plan.md#tabs) section.

**Reason:** The first of several small improvements the user asked for after trying the panel.

**Plan impact:** Removes "several shells" from the non-goals. The idle limit now covers the shells together, by the user's choice: per shell, a tab nobody is looking at would end after 30 minutes while another is in use. A server restart now leaves a **New shell** button rather than starting a shell by itself, until the page reloads.

**Validation:** The user tried it in a browser and it worked. `tsc`, `go vet ./...`, and `gofmt` found nothing. In `go test ./...`, 54 of 55 packages passed; `internal/terminal` failed once in `TestCloseHangsUpTheShellAndIsRepeatable`, which this change did not touch, and that package then passed five repetitions; see [ISSUES.md](../ISSUES.md). `-race` passed on `internal/terminal` and on the terminal tests of `internal/webui` and `conformance/05-web`; the whole suite was not run with it. Moving to a neighbor with take-over, and leaving the idle timer unstarted when an attached shell ends, each failed a test. `TestTerminalPanelInBrowser`, extended with a second tab, was skipped here for lack of Chrome.

**Remaining:** The browser test's first run on CI; this change has not been on CI. Further panel improvements.

## Keep shells with their project

**Commits:** 2026-11-05 21:03:45 `0371601e`.

**Change:** `Keeper` gains groups, a per-group limit, a start directory, a first line kept with the output, and `KeepAlive`. `internal/webui/terminal.go` looks up a page's place from its basedir ID, project, and run, adds `/_terminal/here` and `/_terminal/alive`, and gives each shell its project and page link. `web_shell.js` shows the page's project's tabs and a list of the others, and `web_app_core.js` adds **Open a terminal here** to the run page. Contracts WEB-33 and WEB-34, `docs/WEB.md`, `docs/SECURITY.md`, `docs/ARCHITECTURE.md`, and the plan's [Projects](plan.md#projects) section.

**Reason:** The user wants terminals by project, like an editor's. They chose switching tabs by page over listing all tabs, and said any open page should keep the shells, which replaced the rule that counted only connections.

**Plan impact:** Limits are eight per project and 32 in all, provisional. Choices the user has not reviewed one by one are listed in the plan. Two bugs in earlier commits were fixed here. The first tabs commit `02a5a3ad` could not list shells on a mounted basedir's page, because the application's `fetch` wrapper added the prefix; a jsdom test on such a path found it. Since `02ff9b58`, a saved height set `--terminal-height` on the page even with the panel closed, which shortened the sidebar; this was found by reading the code after the user reported a dark area, and it was not confirmed that this was what they saw. A project with no shell now hides the empty terminal, the other candidate for it.

**Validation:** The user tried the build and said OK. `go test ./...` passed all 55 packages, and `go vet ./...`, `gofmt`, and `tsc` found nothing. `-race` passed on `internal/terminal` and the terminal tests of `internal/webui` and `conformance/05-web`, before the dark-area fix. Starting a shell on page load, and an unescaped first line, each failed a test. `TestTerminalPanelInBrowser`, extended with a project page, was skipped here for lack of Chrome. No CI result has been read for the panel or tabs.

**Remaining:** The browser test's first run on CI. An HTTPS reverse proxy is still untried by hand. Further panel improvements.

## Send from copy buttons

**Commits:** 2026-11-05 21:32:58 `a38d5829`.

**Change:** `commandGuideHTML` takes the commands a guide sends, and `copyIconForValue` and `copyButtonHTML` add a send button; `web_shell.js` adds `sendToTerminal` and the same buttons on the server-drawn Recent jobs page. Contract WEB-33, `docs/WEB.md`, and the plan's [Sending suggested commands](plan.md#sending-suggested-commands) section.

**Reason:** The user asked for it on suggested commands, with no Enter and no shell started for a project with none, then beside every copy button.

**Plan impact:** Each guide names what it sends, because its text holds comments and alternatives and a line break could run what came before it. A browser test for the real xterm.js was added.

**Validation:** The user answered "ok" to integrating it after being given the build. `go test ./...` passed all 55 packages, and `tsc` and `gofmt` found nothing. `-race` was not run for this change, which is mostly JavaScript. `TestTerminalPanelInBrowser` was skipped here for lack of Chrome, so the real xterm.js has not run this, and it uses stand-ins in `TestTerminalPanel`.

**Remaining:** The browser test's first run on CI.

## Clear the terminal

**Commits:** 2026-11-05 21:47:13 `c26b20ac`.

**Change:** `Attachment.ClearOutput`, a `clear` control message, and a **Clear** button in `web_shell.js`. Contract WEB-34, `docs/WEB.md`, and the plan's [Small improvements](plan.md#small-improvements) section.

**Reason:** The first of the small improvements the user is deciding one at a time.

**Plan impact:** The user tried a first version and found it blanked `vim`. They chose to do nothing on the alternate screen, with a message, from the options offered.

**Validation:** `go test ./...` passed all 55 packages, and `tsc`, `go vet` on the changed packages, and `gofmt` found nothing. `-race` passed on the terminal tests before the alternate-screen change, which is JavaScript only. Removing the server's clear failed `TestTerminalShellOutlivesItsConnections`. The alternate-screen check ran against a stand-in, and the user has not tried that version in a browser.

**Remaining:** Trying Clear with `vim` open in a browser.

## Layers, dialogs, and sending from them

**Commits:** 2026-11-05 22:03:40 `7ced0396`.

**Change:** `web_shell.css` puts the panel under the top bar and sizes dialogs to the room above it. `web_template.html` and `web_app_logs.js` add a dialog send button and **Send last 100 lines**. `sendToTerminal` takes pasted text. Contract WEB-33, `docs/WEB.md`, and the plan's sending section.

**Reason:** Three things the user reported: Settings and Inbox opened behind the panel, an artifact dialog ran off both ends with no way to scroll to them, and the path and log copy buttons had no send button.

**Plan impact:** Reverses "logs keep only Copy". Several lines go in only with bracketed paste on. xterm.js 5.5 does not strip escapes from a bracketed paste, so a log holding its end sequence could otherwise run what follows; all control characters but line breaks and tabs are removed first.

**Validation:** The user tried the build and said OK. `go test ./...` passed all 55 packages, and `tsc`, `go vet` on the changed packages, and `gofmt` found nothing. `-race` was not run; the change is CSS and JavaScript. Layout was not checked here without a browser, beyond the CSS rules a test reads, and `TestTerminalPanelInBrowser`'s new fit check was skipped for lack of Chrome.

**Remaining:** That browser test's first run on CI.

## Maximize the panel

**Commits:** 2026-11-05 23:08:14 `62cc45fd`.

**Change:** A **Maximize** button in `web_shell.js`, rules in `web_shell.css`, contract WEB-33, `docs/WEB.md`, and the plan's [Small improvements](plan.md#small-improvements) section.

**Reason:** The second small improvement, for full-screen programs.

**Plan impact:** The panel covers the page rather than changing its height, so the page keeps its layout. Page menus and dialogs are out of reach until **Restore**.

**Validation:** The user tried the build and said OK. `go test ./...` passed all 55 packages, and `tsc`, `go vet` on the changed packages, and `gofmt` found nothing. `-race` was not run; the change is CSS and JavaScript. The new check in `TestTerminalPanelInBrowser` was skipped here for lack of Chrome.

**Remaining:** That browser test on CI.
