# Plan: Table Viewer Files in Their Own Windows

Created: 2026-11-05
Status: Implemented, and tried by the user in Chrome; see [work-log.md](work-log.md).

## Purpose

Show a file on another screen while stepping through the table, by dragging its tab
out of the window and back, as browsers do with their own tabs.

## Scope

- A tab dragged past the window's edge, or **Open in a new window**, moves the file to a
  window of its own.
- A tab dragged back to any window, or **Back to the table**, returns it.
- Row moves change the file in the window last active, and arrow keys in that window
  move the table.

## Non-goals

- Linked time and zoom across windows. They stay within a window.
- Closing a window returning its files. It closes them, as closing a tab does; a reload
  fires the same event, so telling them apart would be fragile.
- Static snapshots as a tested case: they may work, but `file://` pages are not checked.

## Decisions

- **Dragging first, with a button as a fallback.** A prototype showed Chrome allows a
  window from the end of a drag, as the user confirmed on 2026-11-05. Another browser may
  not, so a blocked window says so and leaves the tab.
- **Windows have names and a home.** `window.name` outlives a reload. Each window of its
  own reports to its home, the window with the table, over a `BroadcastChannel`.
- **One active window.** It is the window last clicked in or moved to. The table's window
  sends it the row's files, so that it needs no table of its own.
- **A new file interrupts playback.** Media elements cannot move between documents and
  keep playing, so the new window starts paused at the same position.

## Tests and validation

jsdom has no windows or native drag, so tests use stand-ins for `window.open`, the drag
events, and the channel. The real thing needs a person:

- drag out, back, and between two windows;
- arrow keys from each window, and `Space` in each;
- close a window, then move rows;
- the other browsers the user will try at the end.

## Remaining work

Trying the other browsers, which the user plans to do at the end.
