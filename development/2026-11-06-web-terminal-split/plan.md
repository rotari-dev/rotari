# Plan: Split the Web terminal panel

Created: 2026-11-06
Status: Done: the user tried the panes in a browser, and `TestTerminalPanelInBrowser`'s split checks passed on CI. **Split**
and **Unsplit** have since become **Split right**, **Split down**, and **Close pane**, under
[the columns plan](../2026-11-06-web-terminal-columns/plan.md). See the [work log](work-log.md). Follows [the Web terminal plan](../2026-11-05-web-terminal/plan.md).

## Terms


```text
On the server (lasts after a page closes)
rotari web
└─ basedir
   └─ project
      └─ shell: a running process

In the browser (what one page shows)
browser tab: one page
└─ panel: the terminal area along the bottom of the page
   ├─ bar: the row of controls along the panel's top
   │   ├─ project list
   │   ├─ tab row, and +
   │   └─ buttons: A-, A+, Clear, Split, Maximize, Hide
   └─ pane: a window onto a shell (one, or two when split)
```

| Term | Meaning | How many |
|---|---|---|
| **project** | What groups shells; a page's panel shows its project's shells | |
| **shell** | The process itself, on the server | Up to 8 per project and 32 in all, by default |
| **browser tab** | A tab of the browser, holding one page | Any number |
| **panel** | The terminal area at the bottom of a page | One per page |
| **bar** | The row along the panel's top | One per panel |
| **tab** | A shell's label in the bar. One tab is one shell | One per shell |
| **pane** | Where a shell is shown, with one terminal screen (xterm.js) and one connection | One, or two when split |

**"Tab" alone always means a panel's tab,** and the browser's is always "browser tab". A tab says which shell, and a
pane says where it shows: with five tabs, only the shells shown in panes are connected. **Closing a tab ends its
shell, while closing a pane does not.** The Table viewer's panes each have their own tabs; these share one row.

| What | Kept by | Why |
|---|---|---|
| Shells, and tabs' names and order | The server | So that every browser tab sees the same |
| Which tab shows where, and the split | The browser tab | So that browser tabs can differ |
| The panel's being open, height, maximizing, and text size | The browser | Preferences of how it looks |

## Goal

See two of a project's shells side by side, such as `rotari wait` beside an editor, and keep what the panel already
does: tabs, sending from the page, marks on background tabs, and following links. `tmux` already splits a terminal,
but cannot do those across its panes from the page.

## Existing implementation

- **The server needs no change.** Each shell has its own connection and take-over, and connections to different
  shells do not disturb each other (WEB-34). Unseen output and the idle limit already go by what is attached.
- **The panel assumes one of everything.** Inside [web_shell.js](../../internal/webui/assets/web_shell.js), the
  terminal panel is about 1,100 lines, and about 80 lines use `term`, `socket`, or `shown`. Also single: `fit`,
  `viewport`, `retryTimer`, `retryDelay`, `whenVisible`, `pendingSend`, `connectSerial`, `waitingToOpen`, and the status
  and action in the bar.
- **The Table viewer splits already** ([table_view_panes.js](../../internal/webui/assets/table_view_panes.js)), with a
  tree of panes and drag handles. Its panes are redrawn from URLs. A terminal cannot be: redrawing an xterm.js element
  loses its screen, so the layout code cannot be reused as it is. Its ideas and styles (`.tv-split`, `.tv-split-handle`)
  can.
- **`TestTerminalPanel`** counts one list of sockets by index, so splitting changes many of its expectations.

## Decisions

The user was asked to review these, said to go on, and then tried the result.

1. **Two panes, side by side, at first.** No nesting. A tree of splits, as in the Table viewer, can follow once panes
   are objects, if anyone wants it.
2. **One row of tabs for the project,** not one per pane. A click shows that tab in the focused pane, and tabs mark
   which pane shows them.
3. **The same shell is never in both panes.** A second connection would take it from the first and close it. Choosing a
   tab shown in the other pane swaps the two.
4. **Only shown shells connect,** now up to two.
5. **The layout belongs to the browser tab and project,** in `sessionStorage` beside the shown tab today, so following a
   link restores both panes. Another browser tab keeps its own.
6. **Sending and Clear act on the focused pane.** Text size stays one value for the panel, and Maximize still covers
   the window with both panes.

### What happens when

| Event | Result |
|---|---|
| **Split** with another shell in the project | It shows in the new pane, which takes focus |
| **Split** with none | A new shell starts, as **+** does, and starts nothing if at the limit, saying so |
| **+** | A new shell in the focused pane |
| A pane's shell ends, and another shell not shown exists | That pane moves to it, without taking it from another browser tab |
| A pane's shell ends, and every other shell is in the other pane | The split closes |
| The last shell ends | As today: the panel says how and offers **New shell** |
| **Close tabs to the right** or **all** | As today, each pane then following the rows above. Focused pane's shell ends last |
| A pane's shell is taken over by another browser tab | That pane says so and offers to take it back; the other is unaffected |
| Following a link to another project | That project's layout, or one pane offering to start a shell |
| **Unsplit** | The other pane closes and its shell keeps running. A command waiting for that pane is told |
| A narrow window | Panes stack, under 720 pixels wide |

## Steps

### Step 1: one pane object, no change in behavior

Move `term`, `fit`, `viewport`, `socket`, `shown`, the retry state, `pendingSend`, and the connect state into one pane
object, and have `connect`, `open`, `show`, `shellGone`, `reconnect`, and `sendToTerminal` take a pane. The panel still
makes exactly one.

**The check is that `TestTerminalPanel`, `TestTerminalThemeTokens`, and the conformance tests pass unchanged.** A
change to them in this step means behavior changed. Look at the diff before going on.

### Step 2: focus and status per pane

A focused pane, following xterm.js's input element. The status and action move from the bar into each pane, over the
screen so that a message coming and going does not resize the shell, since two panes can need different messages, such
as one taken over and one reconnecting. This was chosen without seeing it, and the user's trial raised no objection.

### Step 3: two panes

**Split** in the bar, a handle between the panes with its share kept in `sessionStorage`, a fit per pane on resize, and
the swap and the table above. Theme and text size changes reach both terminals. Tabs mark their pane. The swap changes
only the panes' `order`: connecting again would replay both, and moving an element in the page would lose its scrolling.

### Step 4: promise and document

Contract WEB-33, `docs/WEB.md`, and the Web terminal plan. WEB-34 should not change; if it must, the server did.

### Later, if wanted

Stacked panes, more than two, and a tree of splits. Up to four panes, in columns of rows, are planned in
[the columns plan](../2026-11-06-web-terminal-columns/plan.md).

## Non-goals

- Server changes, and a layout kept on the server. Two browser tabs may want different layouts, and a layout is not
  state the server needs.
- Showing one shell twice. It would need the server to allow two connections to one shell, which sizes make hard.
- Splits across projects.

## Risks

- **Regressions in what took several fixes to get right:** waiting for parsing before connecting, dropping a closed
  connection's output, not taking a shell by reconnecting, and background browser tabs not connecting. Each has a test,
  which step 1 keeps running unchanged.
- **Two connections on page load.** Earlier, many old tabs delayed connecting. Two per tab is unlikely to matter, but a
  hidden tab still must not connect at all.
- **Sizes.** A resize must reach both shells, and hidden or just-created panes measure as zero, which is an error on
  the server's side.
- **Test size.** `TestTerminalPanel` is long and indexes sockets. Splitting needs new helpers, and may take as much work
  as the panel.
- **Focus.** Keys must go where the person thinks they go. The focused pane must be clear at a glance, in both themes.

## Tests and validation

- Step 1: the existing tests, unchanged, and `tsc`.
- Each row of the table above, in `TestTerminalPanel` with stand-ins.
- A pane's connection closing or being taken over leaves the other's state alone, including its retry timer and a
  command waiting to be sent.
- Sending goes to the focused pane only. Clear skips a pane on the alternate screen and leaves the other.
- Fast output in one pane does not reach the other when switching tabs.
- `TestTerminalPanelInBrowser`: split, type in each, follow a link and find both, and check by measuring that the two
  panes fill the panel without overlapping and that each terminal's size matches its share, under the page's policy.
- By hand, since layout and focus are what this is for: `vim` in one pane and `yes` in the other, dragging the handle,
  both themes, and Maximize.
