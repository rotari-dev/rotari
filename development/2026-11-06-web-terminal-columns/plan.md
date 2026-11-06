# Plan: Up to four panes in the Web terminal panel

Created: 2026-11-06
Status: Steps 0 to 4 are implemented. What remains is the user's look at four panes, and the browser test's first run with
them on CI. See the [work log](work-log.md). Follows
[the split plan](../2026-11-06-web-terminal-split/plan.md), whose terms this uses. The limit of four, the single level of
nesting, and closing the focused pane are the user's choices. They accepted decisions 3, 6, and 7 as proposed, decision 3
tentatively: it is an implementation choice to judge by what step 1 does to the panel. Decisions 5 and 8 were not raised
separately.

## Goal

Show three or four of a project's shells at once, such as an editor on the left with `rotari wait` and a prompt stacked
on the right. The panel keeps what it does with two panes: one row of tabs, sending from the page, marks on background
tabs, and following links.

## Existing implementation

- **The server needs no change.** Each shell has its own connection, whatever the number of panes (WEB-34).
- **Connections are ready for it.** Each pane already holds its own terminal, connection, status, and retry state, and
  `fitPanes`, `paneShowing`, the theme, text size, and visibility checks go over `panes`.
- **The layout assumes two.** In [web_shell.js](../../internal/webui/assets/web_shell.js), these do: `splitShare` and
  the one `splitHandle`, `applyLayout`, `swapInto` (`panes.reverse()`), `toggleSplit`, the saved `{shell, focused, share}`,
  and `begin` restoring one pane beside another.
- **Flex `order` is not enough.** It reorders panes within one parent. Rows inside columns would need a parent per
  column, and moving a pane to another column would move its element, which makes browsers forget a screen's scrolling.
  The Table viewer's tree ([table_view_panes.js](../../internal/webui/assets/table_view_panes.js)) redraws from URLs, which
  a terminal cannot do.

## Decisions

1. **At most four panes.** Each shown shell holds a connection, and more screens than this are too small to read.
2. **One level: columns, each with rows.** A layout is a list of columns, and a column a list of panes. **Split right**
   adds a column beside the focused pane's column, at the column's full height, so it cannot split a single row sideways.
   This is the whole of the nesting; a general tree stays out.
3. **Panes never move in the page.** Every pane and handle is a direct child of the panes' box, placed by position and
   size worked out from the layout. Splitting, closing, and swapping change numbers only. This replaces `order` and
   `flexGrow`, and carries the reason for them further.
4. **Bar: Split right, Split down, and Close pane,** replacing Split and Unsplit. Both splits are off at four panes.
   **Close pane** closes the focused one, where Unsplit closed the other, since with more than two "the other" means
   nothing, and the user preferred it from the start. Its shell keeps running, as before.
5. **Tabs stay one row.** A dashed tab is in another pane, and choosing it exchanges that pane's place with the focused
   one's, connecting nothing, as now. Tabs do not say which pane; the screens do.
6. **Saved under a new key,** `rotari-terminal-layout`, per browser tab and project. A browser tab open across the upgrade
   loses its split once, which is not worth code to read the old form. `rotari-terminal-shell` still holds the first
   pane's shell.
7. **A smallest pane.** A handle stops at it, and a split that would go under it is refused, saying so. Start from 200 by
   100 pixels and adjust by looking.
8. **Narrow windows stack everything,** in reading order, in one column, under 720 pixels as now. This is only how it
   is drawn: the saved layout keeps its columns for a wider window. Being worked out in the script, it needs `matchMedia`
   in place of the CSS rule.

### What happens when

| Event | Result |
|---|---|
| **Split right** or **Split down** with a shell not shown | It shows in the new pane, which takes focus and half of the focused pane's room |
| The same with none | A new shell starts, and nothing starts if at the limit, saying so |
| The same at four panes, or with too little room | The buttons are off at four; too little room says so |
| **+** | A new shell in the focused pane |
| Choosing a tab not shown | It shows in the focused pane |
| Choosing a tab in another pane | The two panes exchange places, and focus stays in the same place |
| **Close pane** | Its shell keeps running. Its room goes to its neighbor in the column, or the neighboring column if it was alone. Focus goes to that neighbor |
| A pane's shell ends, and a shell not shown exists | That pane moves to it, without taking it from another browser tab, and focus stays |
| A pane's shell ends, and every other shell is in a pane | The pane closes, as **Close pane** does |
| The last shell ends | As today: the panel says how and offers **New shell** |
| **Close tabs to the right** or **all** | As today: those not shown first, then those in other panes, the focused pane's last |
| A pane's shell is taken over by another browser tab | That pane says so and offers to take it back; the others are unaffected |
| Following a link | The layout comes back, with shells that have gone dropped as if their panes had closed |
| Following a link to another project | That project's layout, or one pane offering to start a shell |
| A browser tab out of sight | No pane connects until it is looked at |

## Steps

### Step 0: the first run on CI

Run `TestTerminalPanelInBrowser` on CI, as its split checks have never run, and fix what it finds. This work changes the
same test, and a failure afterwards should not be ambiguous.

### Step 1: place panes by position, still two

Replace `order` and `flexGrow` with positions worked out from a layout of columns and rows, holding only what two panes
can make today. Handles become positioned elements. The panel behaves as it does now.

**The check is `TestTerminalPanel` and the browser test,** with changes only where they read `order`, `flexGrow`, or the
720-pixel CSS rule. Look at the diff before going on, as the split plan's step 1 did.

### Step 2: columns and rows

The layout operations: split right, split down, close, and exchange, with the table above, the handles between columns
and between a column's rows, and the smallest pane. `begin` restores a whole layout, fitting every pane before any
connects. The new saved form, ignoring anything that makes no sense.

### Step 3: the bar

**Split right**, **Split down**, and **Close pane**, and the dashed mark as "in another pane". Look at the bar's room at
ordinary widths: it already holds many buttons, and these may need icons.

### Step 4: promise and document

Contract WEB-33, `docs/WEB.md`, and the two earlier plans. WEB-34 should not change; if it must, the server did.

## Non-goals

- A general tree, rows split into columns, and more than four panes. If wanted later, the layout's form is what changes.
- Dragging tabs onto panes, and moving between panes by keyboard. Both are worth their own decision.
- Server changes, and a layout kept on the server.
- Showing one shell twice, for the reasons in [the split plan](../2026-11-06-web-terminal-split/plan.md#non-goals).

## Risks

- **Positioning by script.** Rounding can leave gaps or overlaps, and every resize of the window or the panel must place
  things again. Sharing one function for all of it, and measuring in the browser test, is the guard.
- **Focus marks.** A colored top edge has to read clearly among four panes, in both themes, and a pane under another
  now has an edge next to it.
- **Four connections on page load.** Earlier, many old tabs delayed connecting. Four per browser tab may matter where
  two did not. A hidden tab still must not connect at all. Watch for it when trying this.
- **Resizes.** Dragging a handle resizes two shells, and a window resize all four, each sending a message per change.
- **Small panes.** Status messages sit over the screen and could cover most of a small pane.
- **Test size.** `TestTerminalPanel` grows again, and its split section assumes two panes throughout.

## Tests and validation

- Step 1: the existing tests, with changes only as above.
- Each row of the table above, in `TestTerminalPanel` with stand-ins, for columns and for rows.
- The layout operations on their own, since their cases multiply: every shape of up to four panes, closing each pane of
  each, and that shares still add up.
- A pane's connection closing or being taken over leaves the other three's state alone, including retries and a waiting
  command.
- What is saved: restoring each shape, shells that have gone, the same shell named twice, more than four, and values
  someone edited.
- `TestTerminalPanelInBrowser`: three and four panes, typing in each, following a link and finding them all, and measuring
  that the panes fill the panel without overlapping and that each terminal's size matches its room.
- By hand, since layout and focus are what this is for: `vim` and `yes` among four panes, dragging each handle, both
  themes, Maximize, and a narrow window.
