# Work log: The Web terminal panel in a window of its own

## Step 1: the window's page

**Commits:** 2026-11-09 12:22:22 `45bef46a`, which holds steps 1 to 6 together.

**Change:** `/_terminal/window` and `/_terminal/window/<page path>` serve
[terminal_window.html](../../internal/webui/assets/terminal_window.html) from
[terminal_window.go](../../internal/webui/terminal_window.go): the page frame's script and styles under the page's
policy and nonce, with `data-terminal-window` on `<html>` and nothing of the application. In
[web_shell.js](../../internal/webui/assets/web_shell.js), `inWindow` makes the panel open at once and ask for a shell,
read its project from the path after `/_terminal/window`, put the window's own path on the project list's options,
name the window after the project, and leave out the resizer, Panel position, Maximize, and Hide. It does not write
`rotari-terminal-open`, and its place is always the bottom. [web_shell.css](../../internal/webui/assets/web_shell.css)
makes the panel fill the window.

**Reason:** [Step 1](plan.md#step-1-the-windows-page). The user chose the recommended answers on 2026-11-09.

**Plan impact:** The window's address carries a page's path rather than a query, so that `pagePlace` reads it the same
way and a project list option is the window's path followed by the shell's `link`. Panel position leaves the window for
now; step 2 brings it back offering only **Back to page**.

**Validation:** `TestTerminalPanel` loads the window's page on a project with shells and on one without;
`TestTerminalShellRoutes`, `TestTerminalShellRoutesAreGuarded`, and `TestTerminalIsAbsentWithoutTheOption` cover the
route, and `TestTerminalWindowPage` its policy and nonce. `go test ./internal/webui/` passed. Not looked at in a
browser: there is no Chrome here.

**Remaining:** None. The user tried the window's page by its address and said it looked right.

## Step 2: opening and closing

**Commits:** 2026-11-09 12:22:22 `45bef46a`, which holds steps 1 to 6 together.

**Change:** In [web_shell.js](../../internal/webui/assets/web_shell.js), **Window** follows Bottom, Left, and Right in
the Panel position menu. `popOut` opens `/_terminal/window` with the page's project path and, after `#`, the
browser tab's `rotari-terminal-shell:*` and `rotari-terminal-layout:*` entries, straight from the click; `letGo` closes
every pane's connection with its `pane.socket` cleared first, so no close says another tab took it, and hides the panel
without writing `rotari-terminal-open`. `rotari-terminal-windowed` in `sessionStorage` marks the browser tab. The
page looks every half second for the window having closed, since a closed window sends nothing, and `windowGone`
closes the extra panes, shows the panel, and runs `begin`, which restores the layout the window last reported.
`rememberLayout` writes nothing in a page whose panel is in a window; in the window it also posts the two entries to
its opener. The window takes the entries from its address once and drops the hash, says it is there to its opener
every second, and its Panel position menu offers only **Back to page**, which tells the opener and closes, or says
the page is gone and stays. A page loaded with `rotari-terminal-windowed` set opens no panel and takes the first
window that says it is there, or puts the panel back after three seconds. Messages are taken only from this origin,
and on the page only from the window it opened, except that announcement while it waits.

**Reason:** [Step 2](plan.md#step-2-opening-and-closing).

**Plan impact:** Following a link and the page reloading needed handling here rather than in step 3: without it the
next page would open its panel and take the window's shells. Timers are `setTimeout` chains, since the test stand-ins
run no `setInterval` but the two they count.

**Validation:** `TestTerminalPanel`: the handover in the address, no shell or takeover message left in the page, the
Terminal button, reports only from the opened window and only of pane entries, closing, Back to page, a blocked
window, a page loaded while the window is open with and without an answer, and the window taking its address, posting
its panes, and Back to page with and without its page. The place menu's existing checks gained **Window**.

**Remaining:** None.

## Step 3: the page and the window together

**Commits:** 2026-11-09 12:22:22 `45bef46a`, which holds steps 1 to 6 together.

**Change:** `sendToTerminal` in a page whose panel is in a window posts the text to it and waits up to ten seconds
for its answer; the window types it with `sendToPane` and answers with what stopped it and, when the page's project
path differs from its own, its project, which the note names. **Open a terminal here** posts the page's project path
and place: on the same project the window starts the shell there, and on another it goes to that project's window
with `?start=`, which it reads once and uses for the next shell started. A page still waiting for its window refuses
to send, saying so.

**Reason:** [Step 3](plan.md#step-3-the-page-and-the-window-together).

**Plan impact:** None.

**Validation:** `TestTerminalPanel`: sending from the page with each answer, Terminal here from the page, the window
typing only what its opener sends, starting a shell in the page's place, and a window opened with `?start=`. Going to
another project's window is not checked: jsdom does not navigate.

**Remaining:** None.

## Step 4: promise and document

**Commits:** 2026-11-09 12:22:22 `45bef46a`, which holds steps 1 to 6 together.

**Change:** WEB-33 describes **Window**, and WEB-34 the route; the status table's WEB-33 row names the window.
[docs/WEB.md](../../docs/WEB.md) explains it and [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md) names the route.
`TestTerminalIsOfferedAndGuarded` checks the route's guards, and `TestTerminalPanelInBrowser` opens the window in
Chrome, sends to it from the page, closes it, and finds the panel back on the same shell.

**Reason:** [Step 4](plan.md#step-4-promise-and-document).

**Plan impact:** The side plan's non-goal of a window of its own is taken back by this plan, which says so at its top.

**Validation:** `go test ./conformance/05-web/ -run TestTerminal` passed, with the browser test skipped: there is no
Chrome here. It runs on CI.

**Remaining:** Trying steps 2 and 3 by hand, and the browser test on CI.

## Step 5: the window follows the page's project

**Commits:** 2026-11-09 12:22:22 `45bef46a`, which holds steps 1 to 6 together.

**Change:** A page that finds its window again after a link posts `rotari-terminal-follow` with its project path when
it is of a project other than the one in `rotari-terminal-followed` (`sessionStorage`), which popping out also sets.
The window goes to that project's window page unless it already shows it. Pages of no project send nothing.

**Reason:** After trying the window, the user asked why it did not switch when the page moved to another project, and
agreed to it following the page while keeping its own list, the latest move winning.

**Plan impact:** Proposal 2 and the table changed, and the non-goal of following the page was removed.

**Validation:** `TestTerminalPanel`: a page of another project sends it, and one of the same project, Recent jobs, or
the home page do not. The window's navigation is not checked: jsdom does not navigate.

**Remaining:** Trying it by hand.

## Step 6: the window's project list moves the page

**Commits:** 2026-11-09 12:22:22 `45bef46a`, which holds steps 1 to 6 together.

**Change:** Choosing a project in the window's list posts `rotari-terminal-go` with that project's page path to the
opener and goes to the project's window page. The page, from its window only and only for a path on this server,
records the project as followed and goes there, so that the next page does not send the window back.

**Reason:** The user asked for the window and the page to behave as the panel does, where the list goes to the page.

**Plan impact:** Proposal 2 and the table changed.

**Validation:** `TestTerminalPanel`: the window posts the page path, and the page goes only for its window and a path
on this server. jsdom does not navigate, so the page's going is checked by what it records.

**Remaining:** Trying it by hand.

## Step 7: the window closes with its tab

**Commits:** Not yet committed.

**Change:** In [web_shell.js](../../internal/webui/assets/web_shell.js), the window's heartbeat, every second, closes
the window when the page that opened it had one and `window.opener` is now gone or closed. Following a link or
reloading keeps the browser tab, whose window object the window keeps as its opener, so only closing the tab closes
the window. A window opened by its address never had an opener and stays. Closing ends no shell.

**Reason:** On 2026-11-09 the user asked for the window to close when its browser tab is closed, taking back the
decision that it stays as a terminal of its own.

**Plan impact:** The "When the page goes away" decision and the table's row changed. Back to page saying it has
nowhere to go is now reached only from a window opened by its address. Done during the shell overview work, in
the same working copy.

**Validation:** `TestTerminalPanel`: a window whose opener stays open is not closed after a heartbeat, is closed
within the next once the opener is closed, and ends no shell; a window opened by its address, whose `close` fails the
test, stays. `go test ./internal/webui -run 'TestTerminalPanel$'` passed. Not tried in a browser.

**Remaining:** Trying it by hand: closing the tab, and reloading and following links in it.
