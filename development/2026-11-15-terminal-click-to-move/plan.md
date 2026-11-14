# Plan: Click to move the cursor in the Web terminal's command line

Created: 2026-11-15
Status: Proposed; waiting for the user's decision on the open questions.

## Goal

With `--allow-terminal`, clicking inside the command line being typed at a shell's prompt moves the cursor there,
as in an editor, instead of only through the arrow keys. A click anywhere else, a drag that selects, a link, and
everything inside `vim` or `less` behave as they do today.

This is the first of two features proposed on 2026-11-15; a right-click menu (copy, paste, cut within the command
line) is the second and gets a plan of its own.

## Existing implementation

- **Blocks** ([plan](../2026-11-14-terminal-blocks/plan.md)). The panel knows, from OSC 133, whether a pane's shell is
  at its prompt (its last block has `B` and no `C`) and where the command line starts (the `B` marker and column).
  rotari's bash and zsh startup files send the marks by default (WEB-49); other shells only if they send them.
- **xterm.js 5.5** has `altClickMovesCursor` (on by default): Alt+click sends left or right arrow keys for the number
  of *cells* between the cursor and the click, only while the view is at the bottom. A character two cells wide
  (Japanese, Chinese, many emoji) counts twice, so it overshoots; it knows nothing of where the command line starts.
- **Line editors.** bash's readline, zsh's zle, and fish move one *character* per arrow key (`ESC [ D` / `ESC [ C`, or
  `ESC O D` / `ESC O C` in application cursor mode). zsh-autosuggestions, which the user has, shows a suggestion in
  grey after the typed text, and accepts it when the cursor moves right at the end of the buffer (`forward-char` and
  friends are its accept widgets), so arrows sent past the typed text would accept a suggestion the user never chose.
  The screen alone tells typed text from the suggestion only by its colour, which users can change.

## Decisions

1. **Where a click moves the cursor.** Only a plain left click (no modifier, no drag, not on a link) in a pane whose
   shell is at its prompt, on a row from the `B` marker's row to the cursor's row (or below it on the input's wrapped
   rows), outside the alternate screen and with the view at the bottom. The target is counted in characters, not
   cells: the cells from `B` to the click, skipping the second cell of a wide character, with a row that ends without
   wrapping counted as one character (a newline in a multi-line buffer). Before `B`, the click is ignored; past the end
   of the text, it goes to the end. Alt+click keeps xterm.js's own behaviour.
2. **How the cursor gets there.**
   - In **zsh with rotari's integration**, exactly: rotari's `.zshrc` binds a private key sequence to a widget that
     reads the target index typed after it and sets `CURSOR` to it, clamped to the buffer. zle counts characters as
     the panel does, a wide character is one, and the suggestion is never accepted, since `CURSOR` never passes the
     buffer's end. The zsh announces that it can by a parameter on its `A` mark (`A;rotari_goto=1`), which other
     shells and older rotari files never send.
   - **Elsewhere** (bash, fish, zsh without the integration), by arrow keys: the difference in characters between the
     cursor and the target, left or right, in the terminal's cursor key mode. In zsh without the integration, a click
     past the typed text may accept an autosuggestion, as an arrow key there would; docs say so.
3. **Nothing is typed but cursor keys.** The panel sends only the sequences above, through the pane's connection as
   keystrokes, never Enter and nothing that edits the line; they are what pressing the keys would send. With no
   blocks (a shell that sends no marks), clicks do nothing new.
4. **No server change** beyond the zsh startup file. The contract is a new WEB-50 (the click) and an addition to WEB-49
   (the widget and its announcement).

## Phases

1. **Clicks by arrow keys.** In `web_shell.js`: the click test (left button, no modifier, no movement between press
   and release, no selection made, no link under it), the input rows from the `B` marker to the cursor, the character
   count across wrapped rows and wide cells, and the arrow keys in the right mode. JSDOM tests with the fake terminal
   (cells with widths, `isWrapped`, cursor key mode).
2. **zsh exactly.** The widget and its key binding in rotari's `.zshrc`, the `A;rotari_goto=1` announcement, and the
   panel choosing it when announced. Tests with a real zsh: a target past a suggestion-like `POSTDISPLAY` does not
   accept it, a wide character counts once, and a multi-line buffer.
3. **Contracts and docs.** WEB-50 and the WEB-49 addition, conformance where the binary can show it (the zsh widget
   moving `CURSOR` through a shell on `/_terminal/ws`), a `TestTerminalPanelInBrowser` step (a bash or zsh tab, typing
   a line and clicking into it), and the Terminal section of [docs/WEB.md](../../docs/WEB.md).

## Tests and validation

- JSDOM: a click on the input row moves by the right number of arrows, left and right; a wide character counts once;
  a click on a wrapped row; a click before `B` and on an output row does nothing; a drag, a click on a link, a click
  with Alt, and a click in the alternate screen do nothing new; application cursor mode sends `ESC O`; a pane with no
  marks does nothing new; a zsh that announced the widget gets it instead of arrows.
- Go: the zsh widget with a real zsh, as above, and the announcement only with rotari's files.
- Browser (CI only): the step in phase 3.
- `go test ./internal/terminal ./internal/webui ./conformance/05-web` with Node 24, `tsc`, and prettier.

## Non-goals

- The right-click menu (its own plan).
- Moving the cursor in programs other than the shell's line editor (`vim` has its own mouse support).
- Selecting text in the command line with the line editor's own region.
- A widget for bash: readline gives `bind -x` functions `READLINE_POINT`, but no way to pass them a number in the
  same keystrokes without reading the terminal behind readline's back; arrows are exact enough there, as bash has no
  autosuggestions of its own.

## Open questions

1. **zsh's exact way.** Proposed: the widget (decision 2), so that a click never accepts an autosuggestion and wide
   characters are exact. The alternative is arrows everywhere, which is simpler and leaves the autosuggestion problem
   in zsh.
2. **Plain click or a modifier.** Proposed: a plain click, as in an editor, with Alt+click left to xterm.js. The
   alternative is to keep it on Alt+click only, as VS Code does, and fix xterm.js's count for wide characters.
3. **A click past the end of the text.** Proposed: move to the end (zsh clamps; bash and fish stop at the end).

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace and is rebased onto `main` before integrating.
