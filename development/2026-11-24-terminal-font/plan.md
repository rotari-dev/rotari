# Plan: Choosing the Web terminal's font

Created: 2026-11-24
Status: Proposed; not started.
Related: [Web terminal](../2026-11-05-web-terminal/plan.md), and the two plans proposed with this one: [Option as
Meta](../2026-11-24-terminal-option-meta/plan.md) and [OSC 52 copy](../2026-11-24-terminal-osc52-copy/plan.md)

## Purpose

People who use `tmux`, Neovim, or a decorated prompt often depend on a font of their own, such as a Nerd Font, whose
icons otherwise show as boxes. The terminal should be able to use a font installed on the browser's machine.

## Today

Each pane takes the page's `--font-mono` (`fontFamily` in `web_shell.js`): the embedded IBM Plex Mono, which is cut to
Latin-1, then the system's monospace fonts. Only the text size can be changed, with **A-** and **A+**, and the browser
remembers it (`rotari-terminal-font-size`).

## Scope

The font of the terminal's panes and of what is drawn to match them (the sticky command line).

## Non-goals

- Embedding a Nerd Font or any other font, or uploading one.
- The fonts of the rest of the Web UI, transcripts, and logs.
- Ligatures, which need another xterm.js addon.
- Listing the installed fonts: the API is Chrome's alone and asks for a permission.
- A rotari config or CLI option. A font is on the browser's machine, not the server's, as the text size already shows.

## Decisions

1. **A font name, kept by the browser.** **Font...** asks for one family name, with a preview line of text, box
   drawing, and Nerd Font icons.
2. **Where: an Options button in the panel's bar**, whose menu holds **Font...**. The pane's right-click menu does not
   open while a program takes the mouse, as `tmux` and Neovim usually do, and **Settings** is not on the panel's own
   window. The other two plans add to this menu; whichever is done first makes the button.
3. **The default stack follows the name**, so a missing font or glyph falls back as today. Empty means the default.
4. **Checked whenever it is read**, since stored values can be edited: at most 100 characters, no control characters,
   quoted as a CSS string. A bad stored value is not used, and the dialog says why.
5. **A missing font is said, as far as a page can tell.** Browsers do not say whether a font is installed. The dialog
   compares the width of sample text in the font with each generic family, and says "This browser does not seem to have
   FONT" when they are all equal. It does not refuse it.
6. **A proportional font is said too**, when `i` and `W` differ in width, since the cells would not fit.
7. **All panes change at once** and are fitted again, as with the text size, and the shells get the resize.
8. **When storage is refused**, the font lasts for the page, and a note says so.

## Design

`fontFamily()` is the one place that makes the stack. It is used where a pane opens, when the theme changes, and by the
sticky line, so each follows. xterm.js measures cells when the option is set; the fit comes after the font is loaded
(`document.fonts.load`), so a font still loading does not leave the old cell size.

## Variants to cover

| Dimension | Values |
| --- | --- |
| Name | none, with spaces, with quotes or a comma, non-ASCII, too long, missing, proportional |
| Page | panel, own window, narrow bar, a program taking the mouse, up to four panes, a pane opened later, theme changed |
| Storage | kept, refused, edited to something bad, changed in another tab (used on the next load) |

## Contracts and documentation

- A new contract ID, the next free when implemented. [docs/WEB.md](../../docs/WEB.md) beside **A-** and **A+**.

## Tests and validation

- `TestTerminalPanel` (JSDOM): the stack for each name above, bad stored values, every pane changed and fitted, the
  sticky line.
- A browser test only if it can be a step of an existing one: CI's `conformance/05-web` has little time left.
- By hand: a Nerd Font on Linux, macOS, and Windows, with `tmux` and Neovim showing icons and box drawing.

## Open questions

1. **One name or a list.** Proposed: one, with the default stack after it. A list would allow an icon font apart from
   the text font.
