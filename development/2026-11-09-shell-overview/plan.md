# Plan: An overview of every shell, to choose one from

Created: 2026-11-09
Status: Steps 1 to 3 done; step 5 (a window of its own) built and then replaced by step 6 (the page's dialog);
steps 7 (the running command), 8 (renaming and ending from a card), and 9 (starting shells from the overview)
done, with the icons' titles fixed on the way (10), and 11 (ending a project's shells, or all) done. Not yet committed or tried by hand; step 4 waits for the user. See the [work log](work-log.md). The user asked for something like macOS Mission Control: all shells shown at once, to pick one.
They chose a still view over a live one, and every project's shells over the current project's. What matters most
to them is seeing each shell's project, tab name, and current directory at a glance; a picture of the screen is
welcome but not needed.

## Goal

A button in the terminal bar opens an **overview** in a dialog over the page, as a log's is (first built over the
panel's panes, then in a window of its own; see steps 5 and 6): every running shell of the server, grouped by project, each showing its project, tab name, and current directory. Choosing one shows it in
the focused pane, moving the page to its project when it is another, as the project list does.

## Existing implementation

- **The shell list has most of it.** `GET /_terminal/shells` in [terminal.go](../../internal/webui/terminal.go) returns
  every shell of the server with `name`, `number`, `basedir`, `project`, `dir`, and `link` (the page of its project).
  The panel already fetches it for the tabs, the project list, and the sidebar marks (`refreshShells`).
- **`dir` is where the shell started,** not where it is now. Nothing in
  [keeper.go](../../internal/terminal/keeper.go) or [session.go](../../internal/terminal/session.go) knows the shell's
  current directory; the session holds the shell's process.
- **Recent output is kept** per shell in an `outputRing` (`RingSize`), which an Attach replays. There is no way to read
  it without attaching, and attaching takes the shell from another connection (close code 4001).
- **Choosing a tab** shows that shell in the focused pane; the project list goes to another project's page, and in
  the window also moves the page (`rotari-terminal-go`). Which shell a project's focused pane shows is kept in
  `sessionStorage` (`rotari-terminal-shell:<basedir>:<project>`).
- rotari is released for Linux and macOS.

## Proposals

1. **A grid button in the bar,** next to **+**, opens the overview over the panel's panes. It is in the window too.
   Esc, the button again, or choosing a shell closes it.
2. **One card per shell, grouped under each project's name** (with the base directory when there are several), the
   page's project first. A card shows the tab name (or number), the current directory, and a mark when it has output
   not yet seen, as its tab does. The shell in the focused pane is marked as current.
3. **The current directory comes from the server,** fetched when the overview opens, not with each refresh of the
   list: `GET /_terminal/shells/<id>/preview` returns `{cwd}`, or `{cwd_error}` (and the screen with its rows and
   columns, see 6).
   - Linux: the link `/proc/<pid>/cwd` of the shell's process.
   - macOS: `lsof -a -p <pid> -d cwd -Fn`, which comes with the system.
   - When it cannot be found (the process ended, `lsof` failed), the response says so with the reason, and the card
     shows the directory it started in, marked "started in", not passed off as the current one.
   It is the shell's directory, not that of a program running in it.
4. **Choosing a card** in the page's project shows that shell in the focused pane, as choosing its tab does. In another
   project it records that shell for that project's focused pane and goes to the shell's `link`; in the window it does
   both, as the window's project list does. Arrow keys move between cards and Enter chooses.
5. **The overview is a still view,** taken when it opens. A **Refresh** button takes it again. Shells started or
   ended meanwhile appear or go at the next refresh of the list, as tabs do.
6. **A picture of the screen, if wanted after trying 1–5:** the preview also returns the kept output, which each card
   writes into a small xterm of the shell's rows and columns, scaled down. Reading it does not attach, so it takes no
   shell from another tab, and does not clear the unseen-output mark. Full-screen programs whose start has left the
   ring look broken, as they do on replay today.

## Steps

### Step 1: the current directory

`Keeper.Dir(id)` (the process's directory, without attaching), the route
`GET /_terminal/shells/<id>/preview` under the same guards as the other terminal routes, and tests on Linux, with the
macOS path behind a build tag.

### Step 2: the overview

The bar button, the cards, choosing (same project, another project, in the window), keys, and Refresh. Tests in
`TestTerminalPanel`.

### Step 3: promise and document

WEB-33 (the overview) and WEB-34 (the route and its guards), `docs/WEB.md`, and a scenario in
`TestTerminalPanelInBrowser`.

### Step 4 (to decide after trying): the picture

Proposal 6.

### Step 5: the overview in a window of its own (replaced by step 6)

After steps 1 to 3, the user asked for the overview in "a window of its own", and chose a browser window over a
dialog covering the whole page when asked. This replaced proposal 1, and kept proposals 2 to 5 for the window's
contents:

- **All shells** opens `/_terminal/overview` in a popup named `rotari-terminal-overview`, from the click, so a second
  click, from any page or the terminal's window, brings back the same window and draws it again. A blocked popup
  leaves things as they were and says so in the pane with the keys.
- **The window has an owner,** the panel that last opened it: the page, or the terminal's window. The owner tells it
  its project and which shells its panes show, which order the groups and mark the cards, and does what a card asks.
  A window reused by another panel is claimed by it, since a browser does not change a reused window's opener.
  Following a link in the owner keeps the owner, as the window's terminal keeps its page.
- **Choosing a card** brings the owner to the front and has it show the shell, as proposal 4 says, opening the panel
  if it was hidden. The overview stays open, marking the chosen card; Escape closes it.
- **With no owner,** or one that has gone, the window says so and its cards choose nothing.
- **It is drawn again** on Refresh and when it comes to the front.

### Step 6: the overview as the page's dialog

On seeing step 5, the user said they had meant a popup like the one that shows a log, closing once a shell is
chosen. The overview goes back to the code of step 2, with the browser window, its page, its route, and the owner's
messages taken out, and is mounted as the log's dialog is: `.output-modal` over the page, with the panel's room left
to it, and `.output-panel` inside, as tall as its cards. Choosing a card, Escape, **Close**, a click beside it, or
hiding the panel closes it. In the terminal's own window it covers the whole window.

### Step 7: the running command

While step 6 was being built, the user asked whether a card could show the command a shell is running. The preview
adds `command`, the command line of the terminal's foreground process group's leader, found with `TIOCGPGRP` on
the terminal and then `/proc/<pid>/cmdline` on Linux or `ps -o command=` on macOS; it is absent when that group is
the shell's own, at its prompt, and `command_error` says why it could not be found. A card shows it under the
directory. A pipeline gives its first program, and a shell without job control runs everything in its own group, so
shows none.

### Step 8: renaming and ending from a card

The user asked for a card to rename its tab and to end its shell. Each card gets a pencil and a cross in its top
corner, beside the button that chooses it, which run the tab's own `renameShell` and `endShell`: the same prompt, the
same question (naming the project of a shell not in the page's), and the same requests. **F2** and **Delete** do the
same for the card with the keys. The dialog stays open and is drawn again, the keys on the renamed card or the one
after the ended one. A refusal is told at the top of the dialog rather than in a pane under it.

### Step 9: starting shells from the overview

The user asked for the overview to start shells, without showing them in a pane. A project with shells gets a **+**
beside its name, which starts another where its page would (the place from the shells' `link`, as a page's own path
gives it), off at the project's or the server's limit. For a project with none yet, the user chose a dropdown, over
a searchable list or listing every project, when asked: **New shell in** under the cards lists, from
`/api/basedirs` and `/api/projects`, every basedir's projects with no shell, grouped by basedir when there are
several, and "No project (home directory)" when no shell has none, with **Start**. Either way the dialog stays open
and draws the new shell as a card with the keys.

### Step 11: ending a project's shells, or every shell

The user asked for the overview and the panel each to end a project's shells and every shell. The overview gets
**End all** beside each project's name and **End all shells** in its header; the panel's tab menu, which already has
**Close all tabs** for the page's project, gets **Close all shells of every project**, off when only that project
has shells. All go through `endShells`, with one question saying how many and whose.

### Step 10: the icons' titles

The user noticed that pointing at the bar's icons no longer said what they do. Each still has its title, but since
they became pictures (`88fddf21`) the pointer is over the `<svg>` inside, and the browser shows no title for it. The
picture now lets the pointer through to its button.

## Non-goals

- A live view of every shell, which would need the Keeper to have readers that watch without taking a shell.
- The directory of the program running in a shell, which can differ from the shell's.
- Shells of another server.

## Risks

- **`lsof` on macOS** is slow with many shells; the previews are fetched in parallel, and only when the overview opens.
- **The shell's directory can lag** behind what the screen shows when a program in it has changed directory; the card
  says it is the shell's.
- **Many cards with pictures** (step 4) means many xterm instances; drawing them only when visible, or a limit, may
  be needed.
