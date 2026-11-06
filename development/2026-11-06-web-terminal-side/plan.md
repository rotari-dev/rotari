# Plan: The Web terminal panel on the left or right

Created: 2026-11-06
Status: Steps 1 to 4 are implemented, and wait for the user's look. Follows
[the columns plan](../2026-11-06-web-terminal-columns/plan.md), whose terms and panes this uses. Offering left and right,
and leaving out the top, are the user's choices, and they said to go ahead with the rest as proposed. See the
[work log](work-log.md).

## Goal

Choose where the panel sits: along the bottom as now, or down the left or right of the window. On a wide screen, a panel
to the side leaves the jobs their full height beside a tall shell.

The top is left out. It gives about what the bottom does, and it competes with the sticky top bar and sidebar, and with
the rule that the top bar's menus open over the panel.

## Existing implementation

- **The server needs no change.** Where the panel sits is a preference of how it looks, which the
  [split plan](../2026-11-06-web-terminal-split/plan.md#terms) gives to the browser.
- **The panes are ready.** Inside the panel everything is placed by share, so a differently shaped panel needs nothing
  new there.
- **The page assumes the bottom.** In [web_shell.css](../../internal/webui/assets/web_shell.css), `--terminal-height`
  sets the panel's height, `body`'s bottom padding, the height of `main` and the sidebar, the bottom of `.output-modal`,
  and the place of `.terminal-send-note`.
- **So does the script.** In [web_shell.js](../../internal/webui/assets/web_shell.js), `clampHeight`, `applyHeight`,
  `savedHeight`, `currentHeight`, `saveHeight`, and `buildResizer` deal in one direction, and `heightKey` holds one size.
- **The bar is one row** of many buttons, which needs a full window's width.
- **Stacking goes by the window,** under 720 pixels (`narrowWindow`). With the panel at the bottom, its width is the
  window's.
- **The page's layouts go by the window too:** media queries at 760, 650, 640, and 520 pixels, and under 760 the sidebar
  becomes a drawer. They cannot see a panel taking room.

## Decisions

1. **Bottom, left, or right, kept per browser,** in `localStorage` with the panel's other looks, and applied before the
   page is drawn, so that it does not jump. Bottom stays the default.
2. **The whole height of the window.** A side panel runs from the window's top to its bottom, and the page, top bar
   included, takes what is left.
3. **On the left, outside the sidebar:** panel, sidebar, content, by `body`'s padding as at the bottom. Between the
   sidebar and the content would keep the sidebar at the window's edge, but would mean changing how the page is laid out.
4. **Width and height are kept apart.** A new `rotari-terminal-width` holds the side's size, from 480 pixels, so that
   moving the panel and back restores each. Left and right share it. It is at least 320 pixels and leaves the page 480.
5. **Narrow windows use the bottom.** Under 1000 pixels the panel is at the bottom whatever is chosen, and the choice
   stays for a wider window: a page narrower than its layouts expect is the main danger of this work, and the drawer
   sidebar assumes the left edge is free.
6. **One button in the bar opens a menu** of Bottom, Left, and Right, marking the current one, by the code of the tab
   menu. Three more buttons would crowd the bar. Its icon follows the Table viewer's style, with no frame.
7. **The bar wraps in a side panel:** tabs get a row of their own under the buttons, without hiding anything in a menu.
   Look at it before deciding more.
8. **Stacking goes by the panel's width,** still under 720 pixels, so most side panels stack their panes. At the bottom
   this is the same as today. The columns stay saved, and come back when the panel is moved or widened.
9. **Maximize works the same from any place,** and Restore returns to that place.

### What happens when

| Event | Result |
|---|---|
| Choosing a place | The panel moves at once, with no connection changed and each screen fitted to its room |
| Dragging the panel's edge | At the bottom its top edge, as now. At the right its left edge, and at the left its right edge. Arrow keys follow |
| A window narrowed under 1000 pixels | The panel goes to the bottom until the window is wide again |
| Opening a dialog | It fits the room beside the panel, as it does above it now |
| A note from sending | It stays over the page, clear of the panel |
| The panel closed | The page has all the room, whatever place is chosen |
| Following a link, or another browser tab | The same place: it belongs to the browser |
| **Split right** in a stacked panel | The new pane is drawn below, and stands in a column when there is room |

## Steps

### Step 1: name the room the panel takes, still at the bottom

CSS custom properties for the room taken on each side, used by the page, dialogs, and notes, and a size and resizer
written for either direction. The panel behaves as it does now.

**The check is `TestTerminalPanel` and the browser test,** with changes only where they read the CSS rules this renames.
`--terminal-height` kept its name as the room at the bottom, so none needed to change.

### Step 2: left and right

The choice in `localStorage`, placing before the page is drawn, the width and its limits, the fallback under 1000 pixels,
and stacking by the panel's width. No control yet: a test sets the choice.

### Step 3: the bar

The menu, and the bar wrapping in a side panel. Look at it in both places and both themes.

### Step 4: promise and document

Contract WEB-33, `docs/WEB.md`, and the earlier plans. WEB-34 should not change; if it must, the server did.

## Non-goals

- The top. If wanted later, the room named in step 1 is where it would start.
- A floating panel, or one in a window of its own.
- A place per project or per browser tab.
- Making the page's layouts respond to their own width rather than the window's. If step 2 shows that a page breaks, record
  it in [ISSUES.md](../ISSUES.md), and raise the 1000 pixels or fix that page.

## Risks

- **The page in less room.** Media queries go by the window, so with a 1280-pixel window and a 480-pixel panel, a page laid
  out for 1280 has 800 and a sidebar. Wide tables, the Recent jobs page, and dialogs are the places to look.
- **The sidebar on the left.** It is sticky, with its own resizer. That resizer goes by how far the pointer moved, so a panel
  to its left does not offset it, as was feared when this was written.
- **Anything else positioned against the window,** such as the inbox panel and the tab menu. These need a look in each place.
- **The bar in 320 pixels.** Even wrapped, it holds many buttons, and the minimum may need to rise.
- **A resize when moving.** Moving changes every screen's size at once, which each shell hears of. Full-screen programs
  redraw, and that should be all.
- **Saved values.** A saved place someone edited must fall back to the bottom, and a width must stay in its limits.

## Tests and validation

- Step 1: the existing tests, with changes only as above.
- Each row of the table above, in `TestTerminalPanel` with stand-ins, for both sides.
- Moving keeps every connection and focus, and sends nothing but resizes.
- What is saved: a place or width that makes no sense, and the two sizes staying apart.
- `TestTerminalPanelInBrowser`: at each place, measuring that the panel takes the room it should, that the page, the top
  bar, and a dialog lie clear of it, and that each terminal fits its pane. Under 1000 pixels, that it is at the bottom.
- By hand, since looks are what this is for: each place and theme, with a wide table, Recent jobs, a log in a dialog,
  dragging the panel's edge and the sidebar's, four panes, Maximize, and a window crossing 1000 pixels.
