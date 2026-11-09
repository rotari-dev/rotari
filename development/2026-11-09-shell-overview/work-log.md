# Work log: An overview of every shell, to choose one from

## Step 1: the current directory

**Commits:** 2026-11-09 20:05:10 `a642e843`, which holds steps 1 to 3 and 6 to 11 together.

**Change:** [session.go](../../internal/terminal/session.go) gains `Session.Dir`, the shell process's working
directory, which is an error once the shell has exited. It comes from `processDir`: the link `/proc/<pid>/cwd` on
Linux ([dir_linux.go](../../internal/terminal/dir_linux.go)), `/usr/sbin/lsof -a -p <pid> -d cwd -Fn` with a 5 second
limit on macOS ([dir_darwin.go](../../internal/terminal/dir_darwin.go)), and an error naming the system elsewhere
([dir_other.go](../../internal/terminal/dir_other.go)). `Keeper.Dir(id)` finds the shell under the lock and asks
outside it, attaching nothing. In [terminal.go](../../internal/webui/terminal.go), `GET /_terminal/shells/ID/preview`
returns `{"cwd": ...}`, or `{"cwd_error": ...}` with the reason, 404 for a shell not running, and 405 for other methods,
under the terminal's guards. WEB-34 and [ARCHITECTURE.md](../../docs/ARCHITECTURE.md) describe it.

**Reason:** [Step 1](plan.md#step-1-the-current-directory). The user said "go" to the plan on 2026-11-09.

**Plan impact:** The preview returns only the directory for now; the rows, columns, and screen wait for step 4, which
is the only one that needs them. The Keeper method is `Dir`, not `Preview`, since that is all it says.

**Validation:** `TestDirIsWhereTheShellIsNow` (the start directory, a `cd` into a directory with a space, an attachment
left working, an unknown shell), `TestShellExitIsReported` (an exited shell's directory is an error),
`TestTerminalPreviewSaysWhereTheShellIs`, `TestTerminalShellRoutes`, `TestTerminalShellRoutesAreGuarded`, and
`TestTerminalIsOfferedAndGuarded` in conformance. `go test ./internal/terminal ./internal/webui` and
`go test ./conformance/05-web -run TestTerminal` passed. `go vet` passed for darwin, and the package builds for
freebsd. The macOS path was not run: there is no Mac here.

**Remaining:** None.

## Step 2: the overview

**Commits:** 2026-11-09 20:05:10 `a642e843`, which holds steps 1 to 3 and 6 to 11 together.

**Change:** In [web_shell.js](../../internal/webui/assets/web_shell.js), **All shells** (four squares) follows **+**
in the bar, in the window too, and covers `.terminal-panes` with `.terminal-overview`. `drawOverview` refreshes the
list, groups the shells by project with the page's first, naming the basedir when two projects share a name
(`inBasedir`, which the project list now uses too), and makes a button for each shell: its name or number, "shown",
"in another pane", or "new output", and the directory from its preview. Without one, the card says "started in"
the shell's `dir`, with the reason in its title. Choosing a card of the page's project calls `chooseTab`; one of
another project goes through `preferShell`, which writes that project's `rotari-terminal-shell:*` and points the
focused slot of its saved layout at the shell, and then goes to its `link`, in the window telling the page with
`rotari-terminal-go` first, as the window's project list does. Arrow keys, Home, and End move between cards;
Escape, the button, or hiding the panel or letting go of it for the window closes it. **Refresh** draws it again.
The styles are in [web_shell.css](../../internal/webui/assets/web_shell.css); a long directory loses its start.

**Reason:** [Step 2](plan.md#step-2-the-overview).

**Plan impact:** None. The overview is not redrawn when the list refreshes by itself, as the plan said.

**Validation:** `TestTerminalPanel` covers the grouping and order, each card's text and marks, a directory not
known and a shell ended before its preview, the keys, Refresh, choosing in the page's project (a new connection) and
in another (the stored shell and layout), hiding the panel, the window telling its page, and a list refused. The
check script passed 128 KiB, the most one argument to `node -e` may be, so it now goes through a file that `-e`
reads, which keeps jsdom found from the package directory and the arguments where they were. That change went into `8f336ad1`, the terminal window's, whose test needed it first.
`go test ./internal/webui -run 'TestTerminalPanel$'` passed. Not looked at in a browser: there is no Chrome here.

**Remaining:** None.

## Step 3: promise and document

**Commits:** 2026-11-09 20:05:10 `a642e843`, which holds steps 1 to 3 and 6 to 11 together.

**Change:** WEB-33 in [05-web-assets-and-static-export.md](../../contracts/05-web-assets-and-static-export.md)
and its row in [contracts/README.md](../../contracts/README.md) describe the overview, and
[WEB.md](../../docs/WEB.md) explains it. `TestTerminalPanelInBrowser` gains a scenario: after `cd /`, the cards say
where their shells are, one of them `/` and none "started in", the overview covers the panes, and Escape closes it.

**Reason:** [Step 3](plan.md#step-3-promise-and-document).

**Plan impact:** WEB-34 was described in step 1, with the route.

**Validation:** `go vet ./conformance/05-web`. The browser scenario skipped here for want of Chrome, and runs only on
CI.

**Remaining:** None; step 5 replaced where the overview is.

## Step 5: the overview in a window of its own

**Commits:** None; step 6 replaced it before it was committed.

**Change:** `/_terminal/overview` serves [terminal_overview.html](../../internal/webui/assets/terminal_overview.html),
the terminal window's page with `data-terminal-overview`; [terminal_window.go](../../internal/webui/terminal_window.go)
composes and serves both through `composeTerminalPage` and `serveTerminalPage`. In
[web_shell.js](../../internal/webui/assets/web_shell.js), **All shells** calls `openOverview`, which opens the
overview in the popup `rotari-terminal-overview` and posts `rotari-terminal-overview-owner` every 100 ms, up to 50
times, until the overview answers `-claimed`. Every panel, in a page or the terminal's window, answers `-ask` with
`-state` (its project, the shell with the keys, and its panes' shells) and handles `-choose` through
`chooseFromOverview`: a shell of its project is stored with `preferShell`, the panel opens, and `chooseTab` shows it;
another project's goes as the project list does; a page whose panel is in its window passes the choice on to it.
`asChosenShell` takes only a shell with a link on this server. `buildOverview` draws the overview's page from the
list and the owner's answer (or none after 1.5 s), marks cards from that answer, focuses the owner and posts
`-choose` from a card, takes a new owner from a claim, and draws again on Refresh and on `focus`; Escape closes the
window. The panel's top-level setup leaves the overview's page alone. The overlay, its button state, and its closing
on hide went away, and [web_shell.css](../../internal/webui/assets/web_shell.css) lays the overview out as a page.

**Reason:** [Step 5](plan.md#step-5-the-overview-in-a-window-of-its-own). On 2026-11-09 the user asked for the
overview in a window of its own, choosing a browser window over a dialog covering the whole page.

**Plan impact:** Proposal 1 is replaced. The overview stays open after a choice, unlike the overlay.

**Validation:** `TestTerminalPanel` covers opening and claiming, a blocked window, answering `-ask`, choosing in the
page's project, in another, with a link off the server, with the panel hidden, from a page whose panel never opened,
and in the terminal's window; and on the overview's page, the grouping, cards, keys, choosing, Refresh keeping the
focused card, a new owner, a shell ended before its preview, an owner gone, Escape, no opener, and a refused list.
`TestTerminalPages` checks both pages' nonce, `TestTerminalShellRoutes`, `TestTerminalShellRoutesAreGuarded`,
`TestTerminalIsAbsentWithoutTheOption`, and `TestTerminalIsOfferedAndGuarded` the route.
`TestTerminalPanelInBrowser` now opens the window, reads `/` on a card, and chooses another card; it skipped here for
want of Chrome.

**Remaining:** None; step 6 replaced it.

## Step 6: the overview as the page's dialog

**Commits:** 2026-11-09 20:05:10 `a642e843`, which holds steps 1 to 3 and 6 to 11 together.

**Change:** [web_shell.js](../../internal/webui/assets/web_shell.js), [web_shell.css](../../internal/webui/assets/web_shell.css),
[terminal_panel_test.go](../../internal/webui/terminal_panel_test.go), and
[terminal_panel_browser_test.go](../../conformance/05-web/terminal_panel_browser_test.go) were restored from the jj
snapshot `46e8367820a4`, which held steps 2 and 3, and `/_terminal/overview`, its page, `composeTerminalPage`, and
their tests were removed. `toggleOverview` now appends `.output-modal.terminal-overview-modal` (`role="dialog"`,
`aria-modal`, labelled by its title) to `<body>`, with `.output-panel.terminal-overview` inside holding a header of
the log dialog's kind (title, then **Refresh** and **Close** in `.modal-actions`) and the cards. A click on the
backdrop closes it. The CSS sizes the panel to its cards, puts the dialog above the panel's bar (`z-index: 60`),
and gives it the whole of the terminal's own window.

**Reason:** [Step 6](plan.md#step-6-the-overview-as-the-pages-dialog). On 2026-11-09 the user said they had meant a
popup like the log's, which closes when a shell is chosen.

**Plan impact:** Step 5's window, claims, and owner are gone, and with them its remaining note about a page whose
panel is in its window.

**Validation:** `TestTerminalPanel` checks the dialog is `<body>`'s with the log dialog's classes, labelled, and
modal, that a click inside leaves it and one beside it closes it, and the cases of step 2.
`TestTerminalPanelInBrowser` now checks the dialog is over the page above the panel and closes on a choice; it skipped
here for want of Chrome.

**Remaining:** None.

## Step 7: the running command

**Commits:** 2026-11-09 20:05:10 `a642e843`, which holds steps 1 to 3 and 6 to 11 together.

**Change:** In [internal/terminal](../../internal/terminal), `dir_*.go` became `process_*.go`, which add
`processCommand`: `/proc/<pid>/cmdline` with its arguments joined by spaces on Linux, and `/bin/ps -o command=` with
the same 5 second limit as `lsof` on macOS. `Session.Foreground` reads the terminal's foreground process group with
`TIOCGPGRP` through `SyscallConn`, as `Resize` reaches the terminal, and returns "" for the shell's own group;
`Keeper.Foreground` finds the shell as `Keeper.Dir` does. The preview adds `command` and `command_error`, and a card
shows the command under the directory, marked with a run triangle, or "running program not known" with the reason in
its title.

**Reason:** [Step 7](plan.md#step-7-the-running-command). The user asked for it on 2026-11-09, while step 6 was
being built.

**Plan impact:** The plan's non-goal of the program running in a shell is taken back; that program's directory stays
a non-goal.

**Validation:** `TestForegroundIsTheProgramRunning` (nothing at the prompt, `sleep 30` while it runs, nothing after
Ctrl-C, an unknown shell), `TestShellExitIsReported` (an exited shell's is an error),
`TestTerminalPreviewSaysWhereTheShellIs` (`sleep 30` in the preview), and `TestTerminalPanel` (a command, one not
known, and none). `go vet` passed for darwin, and the package builds for freebsd; the macOS path was not run.

**Remaining:** None.

## Step 8: renaming and ending from a card

**Commits:** 2026-11-09 20:05:10 `a642e843`, which holds steps 1 to 3 and 6 to 11 together.

**Change:** In [web_shell.js](../../internal/webui/assets/web_shell.js), `renameShell` and `endShell` take how to tell
a refusal (the pane with the keys by default, `paneRefusal`) and return whether they did it; `endShell`'s question
names the project of a shell not in the page's. A card is now `.terminal-overview-item`, holding the choosing button
and `.terminal-overview-actions` with **Rename** (a pencil) and **End** (a cross), labelled with the shell's number
and name. `renameFromOverview` and `endFromOverview` call them with `overviewRefusal`, which writes into
`.terminal-overview-problem` at the dialog's top, and draw the overview again on success, `drawOverview` taking the
card to focus. **F2** and **Delete** on a card do the same. [web_shell.css](../../internal/webui/assets/web_shell.css)
places the actions in the card's corner.

**Reason:** [Step 8](plan.md#step-8-renaming-and-ending-from-a-card). The user asked for it on 2026-11-09.

**Plan impact:** None.

**Validation:** `TestTerminalPanel`: renaming by button (the prompt, the request, the new name, the dialog open, the
keys kept) and F2 cancelled (no request), ending by button (the question naming the project, the request, the card
gone, the keys on the next) and Delete declined (nothing ended), and both refused, told in the dialog and not in the
pane. The tab's own rename and close tests pass unchanged.

**Remaining:** None.

## Step 9: starting shells from the overview

**Commits:** 2026-11-09 20:05:10 `a642e843`, which holds steps 1 to 3 and 6 to 11 together.

**Change:** In [web_shell.js](../../internal/webui/assets/web_shell.js), `pagePlace` reads its query through
`placeOfPath`, which a shell's `link` can be given too. Each project's heading has a **+**
(`.terminal-overview-add`, "New shell in" and its name), off with the limit in its title at the project's or the
server's limit, which `startFromOverview` uses to POST a shell at the focused terminal's size, or 24 by 80, and then
draws the overview with the keys on its card; a refusal is told in the dialog. `drawOverviewNew` adds
`.terminal-overview-new` under the cards: a select of every basedir's projects with no shell, from `/api/basedirs`
and `/api/projects`, in an optgroup per basedir when there are several, with "No project (home directory)" first
when no shell has no project, and **Start**; a basedir whose projects cannot be listed is named in its note, and
the basedirs not being listed says why. [web_shell.css](../../internal/webui/assets/web_shell.css) styles both.

**Reason:** [Step 9](plan.md#step-9-starting-shells-from-the-overview). The user asked for it on 2026-11-09, and
chose the dropdown.

**Plan impact:** None.

**Validation:** `TestTerminalPanel`: + in the current basedir and in another (the query, no pane connected, the keys
on the new card, off at the project's limit with the reason), the dropdown's options across three basedirs (no
project, a project with shells left out, an optgroup per basedir, an encoded name, a basedir that cannot be listed),
Start in another basedir, the basedirs not listed, and a start refused. The test page's fake server answers
`/api/basedirs` and `/api/projects`.

**Remaining:** None.

## Step 10: the icons' titles

**Commits:** 2026-11-09 20:05:10 `a642e843`, which holds steps 1 to 3 and 6 to 11 together.

**Change:** [web_shell.css](../../internal/webui/assets/web_shell.css) gives `.terminal-icon svg`
`pointer-events: none`, so that the pointer over a picture is over its button and the button's title shows. This
covers the bar's icons and the overview's Rename and End.

**Reason:** [Step 10](plan.md#step-10-the-icons-titles). The user noticed on 2026-11-09 that the icons no longer said
what they do; they became pictures in `88fddf21`.

**Plan impact:** None.

**Validation:** `TestTerminalPanel` checks every picture button has a title and a label, and the rule.
`TestTerminalPanelInBrowser` now checks that the point at the middle of a split icon's picture is its button and
that the button has a title; it skipped here for want of Chrome, so the tooltip itself has not been seen.

**Remaining:** Trying the dialog and the tooltips by hand, on Linux and on a Mac, and deciding on step 4. A disabled
button, such as Split right in a full panel, may show no tooltip in some browsers, which this does not change.

## Step 11: ending a project's shells, or every shell

**Commits:** 2026-11-09 20:05:10 `a642e843`, which holds steps 1 to 3 and 6 to 11 together.

**Change:** In [web_shell.js](../../internal/webui/assets/web_shell.js), `endShells` takes a question and how to tell a
refusal, and returns whether it ended them; `projectEndQuestion` and `everyEndQuestion` word the question. The tab
menu gains **Close all shells of every project**, off when every shell is the page's project's. In the overview, each
project's heading gains **End all** (`.terminal-overview-end-all`), and the header **End all shells**, off with no
shell; both draw the overview again after ending, and a refusal is told in it.

**Reason:** [Step 11](plan.md#step-11-ending-a-projects-shells-or-every-shell). The user asked for it on 2026-11-09.

**Plan impact:** None.

**Validation:** `TestTerminalPanel`: the menu item (the question, every shell ended, the shown one last, off with
only the project's shells), the overview's End all (the question, that project's shells only, the dialog open), End
all shells (refused and told in the dialog, then the question for one shell, none left, off with none), and the
existing Close all tabs tests, unchanged.

**Remaining:** Trying it by hand.

## Step 12: the dialogs on the Recent jobs page

**Commits:** Not yet committed.

**Change:** The user found All shells on the Recent jobs page drawn under the jobs rather than as a dialog. The
overview and the launch dialog took their frame from `.output-modal` and `.output-panel`, whose rules are in
`web_styles.css`, which only the application page loads; the Recent jobs page and the panel's own window load
`web_shell.css` and the info styles. [web_shell.css](../../internal/webui/assets/web_shell.css) now gives
`.terminal-overview-modal` and `.terminal-launch-modal` their own fixed, full-window frame and overlay, and
`.terminal-overview` and `.terminal-launch` their own box and header, keeping the classes that leave the panel's room.

**Reason:** A bug in `a642e843` and `4424402f`, reported on 2026-11-09.

**Plan impact:** None.

**Validation:** `TestTerminalPanel` checks the rule. `TestTerminalPanelInBrowser` now opens All shells on `/jobs/` and
checks the dialog is fixed and in view; it runs only on CI, as Chrome is not here.

**Remaining:** Seeing it on the Recent jobs page and in the panel's own window.
