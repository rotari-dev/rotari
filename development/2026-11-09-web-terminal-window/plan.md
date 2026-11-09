# Plan: The Web terminal panel in a window of its own

Created: 2026-11-09
Status: Steps 1 to 4 implemented, not yet committed. The user tried step 1 in a browser; steps 2 and 3 are untried by hand, and the browser test runs only on CI. The user asked for the panel to pop out as one more choice of its position, taking the
whole panel, tabs and project list included, into the window, and chose the recommended answers below.
Follows [the side plan](../2026-11-06-web-terminal-side/plan.md), which listed "one in a window of its own" as a
non-goal. This plan takes that back.

## Goal

**Window** joins Bottom, Left, and Right in the **Panel position** menu. It moves the panel into a browser window of
its own: the panes, the row of tabs, and the project list, so that switching shells and projects happens there. That
window can go on a second screen or beside the browser, and the page keeps all of its room.

## Existing implementation

- **One connection per shell.** In [terminal.go](../../internal/webui/terminal.go), a new WebSocket for a shell
  takes it from the one before (close code 4001, `closeTakenOver`), unless it asks with `takeover=0`. Today a second
  page shows "Another tab has this terminal." A window that takes over the shells must not leave that message in the page.
- **The panes belong to the browser tab.** In [web_shell.js](../../internal/webui/assets/web_shell.js), the layout and
  the shell shown are kept in `sessionStorage` under keys for each project. A window opened with `window.open` gets
  its own `sessionStorage`, so it does not see the page's panes unless they are passed to it.
- **The panel follows the page.** `pagePlace` reads the project and run from the page's path. The project chooses the
  shells, and the run chooses the directory a new shell starts in (`openTerminalHere`). The project list (`groups`)
  changes project by going to another page.
- **Send buttons call `window.sendToTerminal`** in the same page. They open the panel if it is closed.
- **The place is kept per browser** in `localStorage` (`rotari-terminal-place`), applied before the page is drawn, and
  shared by every tab. Under 1000 pixels the page uses the bottom.
- **The panel is built only in the application page** (`composeWebPage`), and only when `terminalControl` is present.
  There is no page with nothing but the terminal.

## Proposals

1. **A page with only the panel,** served by a new route such as `/_terminal/window?project=...`, with the same assets
   and theme. The panel fills the window, as Maximize does now. It has the bar as it is, tabs, **+**, splits, Clear, and
   the project list, but no resizer or Maximize, and its Panel position menu offers only **Back to page** (see 5).
2. **The window follows the page's project, and has its own list.** It opens on the project of the page it came from.
   When the page moves to another project's page, the window switches to that project; a page of no project leaves
   it. Its project list, as the panel's, takes the page to that project's page, and the window switches with it;
   with the page gone, only the window switches. (First proposed as a window that neither followed nor moved the
   page; after trying it, the user asked for the page and the window to behave as the panel does.)
3. **The panes and shells move with it.** The page hands its panes for each project to the window, which keeps them in its
   own `sessionStorage`, and the page closes its panel, letting go of its connections first, so that neither side shows
   "Another tab has this terminal."
4. **The page knows it has a window.** While the window is open, the page's top bar **Terminal** button brings the window to
   the front, and the page's panel stays closed. The page and the window talk through `window.opener` / `postMessage`.
5. **Closing the window, or Back to page, puts the panel back** in the page, at the place it had before (bottom, left, or
   right), showing the window's panes for the page's project. Shells keep running either way, as when the panel is
   hidden.
6. **Window is not kept per browser, unlike the other places.** A popup has to be opened from a click, so a page loaded
   later cannot open one. Following a link in the page keeps the window and the page's link to it. A page reloaded, or
   opened in a new tab, starts with its panel where it was before, and may take over shells the window shows, as another
   tab does today.
7. **The window's size and position are left to the browser.** The first opening asks for about 900 by 600.

### What happens when

| Event | Result |
|---|---|
| Choosing **Window** | A window opens on the page's project, with the same panes and shells. The page's panel closes without a message |
| The browser blocks the popup | The panel stays where it was, and says that the browser blocked the window |
| Choosing a project in the window | The page goes to that project's page, and the window shows its shells and panes |
| Following a link in the page | The window stays. On another project's page it switches to that project |
| A send button in the page | Types into the window's focused pane, brings the window to the front, and names that pane's project when it is not the page's |
| **Terminal** in the page's top bar | Brings the window to the front |
| Closing the window, or **Back to page** | The panel comes back in the page at its previous place, with the window's panes for the page's project |
| Closing or reloading the page | The window keeps working as a terminal; only Back to page has nowhere to go, and it says so |
| A narrow window | The panes stack as they do in a narrow panel |

## Decided

The user chose the recommended answer to each question, on 2026-11-09:

- **A send button in the page** types into the window's focused pane and brings the window to the front, saying which
  project that pane is in when it is not the page's.
- **When the page goes away,** the window stays as a terminal of its own, and **Back to page** says it has nowhere to go.
- **The menu calls it "Window".**
- **"Terminal here"** while the window is open starts the shell in the window, switching the window to the page's
  project.

## Steps

### Step 1: the window's page

The route and a page with only the panel, which shows the shells of the project named in its query, and whose project
list switches in place. No link from the page yet. A test loads it.

### Step 2: opening and closing

**Window** in the menu, handing the panes over, the page's panel closing without a message, **Back to page**, closing
the window, and a blocked popup.

### Step 3: the page and the window together

Send buttons, "Terminal here", the **Terminal** button, and the page going away.

### Step 4: promise and document

Contract WEB-33 and WEB-34 if needed, `docs/WEB.md`, and the earlier plans' non-goals.

## Non-goals

- A window shared by several tabs, or several windows for one page.
- Opening the window again by itself after a reload.
- Remembering the window's size and position ourselves.
- The static export, which has no terminal.

## Risks

- **Popup blockers.** `window.open` must be called directly from the click, before any `await`.
- **Takeover order.** If the window connects before the page lets go, the page shows "Another tab has this terminal" for a
  moment. The page must close its connections first, without the message.
- **`window.opener` across a link.** The window keeps its page as opener when the page follows a link in the same origin,
  but loses it if the page goes to another site. Each message needs to check its sender is the same origin.
- **Two places to keep in step:** the font size and theme are in `localStorage`, so both see them, but a change in one
  needs a `storage` event in the other.
- **The test stand-ins** in `TestTerminalPanel` have one window. Opening another needs a stand-in for `window.open`.

## Tests and validation

- Each row of the table above, in `TestTerminalPanel` with stand-ins for `window.open` and `postMessage`.
- Handing over keeps every shell and its place in the layout, and no pane in either shows the takeover message.
- `TestTerminalPanelInBrowser`: choose **Window**, type in the window, switch its project, send from the page, close the
  window, and find the panel back with the same shells.
- By hand: a second screen, both themes, a blocked popup, following links, and closing each side first.
