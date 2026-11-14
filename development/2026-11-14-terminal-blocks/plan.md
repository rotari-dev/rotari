# Plan: Blocks in the Web terminal

Created: 2026-11-14
Status: Phases 1–7 implemented on 2026-11-14. After them the user found Ctrl+Shift+ArrowUp / ArrowDown little
better than scrolling, and accepted a **Commands** dropdown and copy buttons in their place (phases 8 and 9); all nine phases
are done; the browser steps run only in CI. See [work-log.md](work-log.md).

## Goal

With `--allow-terminal`, a shell that marks its prompts and commands (OSC 133, as VS Code, iTerm2, WezTerm, Ghostty,
and Windows Terminal read them) has its output shown as one block per command: a mark beside each prompt that says
whether the command succeeded, a separator between commands, and the failures on the scroll bar. From the keyboard
or a mark, you go to the previous or next command, copy a command's output or the command, select the output, or
type the command again. A shell that sends no marks looks and behaves as it does today.

This is the fourth of the terminal features proposed on 2026-11-14, after the scrollback, [recent
directories](../2026-11-14-terminal-directory-history/plan.md), and [output links](../2026-11-14-terminal-output-links/plan.md).
Grouping blocks by directory, discussed with recent directories, would build on this and is left to a plan of its own.

## Existing implementation

- **xterm.js 5.5** (vendored in `internal/webui/assets/vendor/xterm.js`) has `term.parser.registerOscHandler`, which
  sees an OSC sequence as it is parsed, in order with the text around it; `registerMarker`, a line that moves with
  the buffer and is disposed when it scrolls out of the scrollback or the buffer is reset; and `registerDecoration`,
  an element or a scroll bar mark attached to a marker. `registerDecoration` is a proposed API in this build: it
  throws unless the terminal is made with `allowProposedApi: true`. `overviewRulerWidth` turns on the ruler that
  shows decorations' scroll bar marks.
- **Panel** ([web_shell.js](../../internal/webui/assets/web_shell.js)). `openTerminal` makes each pane's terminal;
  `open` calls `term.reset()` and replays the server's ring on every connection, including a tab switch, a link
  followed, and a reload. The ring keeps raw bytes (WEB-33), so the marks in the last 256 KiB come back with the
  text. **Clear** clears the screen and scrollback here and on the server, and is off in the alternate screen.
  `sendToTerminalAndSay` types text without Enter under the send buttons' rules (the shell's preview must show it at
  its prompt).
- **Shells** are started with the environment rotari gives them and nothing added to their startup files
  (`internal/terminal`); FAQ.md says rotari does not log what is typed.
- **Activity.** The server already tells when a command ends by the terminal's foreground process group
  (`CommandFinished`, the bell); it knows no exit codes.

## Decisions

1. **The shell marks; rotari reads, and starts bash and zsh so that they mark.** The panel interprets OSC 133 `A`
   (prompt starts), `B` (command line starts), `C` (command output starts), and `D[;EXIT]` (command ended) when a
   shell or prompt sends them. *Revised 2026-11-14:* with `--terminal-shell-integration`, on by default, rotari starts
   bash and zsh as VS Code does, without touching the user's files: bash with `--init-file` naming a script that reads
   `/etc/profile` and the first of `~/.bash_profile`, `~/.bash_login`, `~/.profile`, as a login shell would, then
   adds the marks; zsh with `ZDOTDIR` pointing at rotari's files, each reading the user's own from where zsh would
   have (following a `ZDOTDIR` the user's `.zshenv` sets) at top level, the last adding the marks and putting
   `ZDOTDIR` back. The marks come from a first prompt hook that keeps `$?` and a last one that prints `D` and `A` and
   puts `B` back at the end of a prompt a theme rebuilt (zsh prints `B` from its `line-init` hook instead, and both say the command line in `C` as kitty does, bash only when its history took it); bash 4.4's `PS0`, a `DEBUG` trap in earlier bash (unless the
   user has one), and zsh's `preexec` print `C`. The scripts
   are written to a private temporary directory when the server starts and removed when it stops. Any other shell,
   and every shell with `--terminal-shell-integration=false`, starts with `-l` as before; fish 4 marks by itself.
2. **Blocks live in the page, from the stream.** A block starts at `A`; its command line is the text from `B` to
   `C`, its output from `C` to `D`, and its exit code comes from `D`. Nothing is parsed or stored on the server, which
   keeps "records nothing that is typed" true: the command line is only what is already on the screen. Because the
   ring keeps the marks, blocks come back after a reload, a tab switch, or a link, for as much output as the ring
   holds; a block whose start fell out of the ring or the scrollback is gone. Marks inside the alternate screen
   (`vim`, `less`) are ignored.
3. **What a block shows.** A mark in a narrow gutter left of the first line of each prompt: green for exit 0, red
   for any other exit code, grey while running, and hollow when the command ended without an exit code (`D` with
   none, or another prompt came first, as after Ctrl-C in some setups). A prompt where no command has started (`C`)
   has no mark. A thin separator above each prompt that follows a command's output, so empty prompts in a row get
   none. Failed commands are
   red marks on the scroll bar, so that a failure far up is easy to find. Hovering a mark says the command line, the
   exit code, and, for a command seen running in this page, how long it took; replayed blocks have no times, since
   the ring keeps none. The replay is a connection's first message (`Keeper.Attach` puts it first in what `Next`
   returns, and each `Next` is one WebSocket message), so marks parsed before that message is done get no times.
4. **Moving and acting.** *Revised 2026-11-14:* the bar's **Commands** dropdown lists the pane's commands, newest
   first, with their state, line, and end, and a filter; choosing one scrolls to its prompt and lights it up, and each
   row copies its output or line. A copy button shows at the right of a prompt's line while the pointer is over the
   command's prompt or output (phase 9).
   No key is taken from the shell; Ctrl+Shift+ArrowUp / ArrowDown, first chosen, did little more than scrolling. Clicking a mark opens a small menu: **Copy output**, **Copy
   command**, **Select output** (so the browser's own copy and the existing actions work on it), and **Type command
   again**, which goes through `sendToTerminalAndSay` and so never presses Enter, and, as choosing a recent
   directory does, only once the shell's preview says nothing runs at its prompt. A command on lines of its own is
   not typed again, since its text holds the shell's continuation prompts. A running command's output is what it has
   printed so far.
5. **Marks are only output.** Any program can print OSC 133, so a file shown with `cat` can draw false marks. They
   change nothing but the page's display; **Type command again** types the text shown, without Enter, under the same
   rules as the send buttons, so a false mark can at worst put text in front of you. Copy writes to the clipboard
   only when you choose it. SECURITY.md says so.
6. **No server change.** No route, option, or Keeper change. The contract is about what the panel does with the
   marks (a new WEB-48; WEB-47 went to another thread's work first), and the ring's keeping of raw bytes (WEB-33) is what brings them back.

## Phases

1. **Parsing and the block model.** OSC 133 handlers on each pane's terminal, markers for `A`, `B`, `C`, `D`, a
   block list that drops blocks whose marker is disposed, nothing in the alternate screen, and a reset with the
   terminal's. JSDOM tests in `TestTerminalPanel` with the fake terminal taught OSC handlers and markers.
2. **Display.** `allowProposedApi`, the gutter (the viewport padded on the left so marks sit outside the text), the
   marks and their colors, separators, the scroll bar marks for failures, hover titles, both themes. JSDOM tests of
   the decorations asked for; a look in a real browser is only possible in CI's browser step.
3. **Moving and acting.** The keys through `attachCustomKeyEventHandler`, the mark's menu, copy and select, and
   typing the command again through `sendToTerminalAndSay`. JSDOM tests.
4. **Contracts and docs.** WEB-48 and its row; a `TestTerminalPanelInBrowser` step whose `/bin/sh` prompt sends the
   marks (`PS1` holding `A`, `B`, and `D;$?`, with `C` printed by the command, since `sh` has no hook before a
   command) and checks a red mark for `false`, a green one for `true`, and Ctrl+Shift+ArrowUp; the Terminal section
   of [docs/WEB.md](../../docs/WEB.md) with the bash and zsh lines, [docs/FAQ.md](../../docs/FAQ.md), and
   [docs/SECURITY.md](../../docs/SECURITY.md).

5. **Shell integration.** The scripts embedded in `internal/terminal`; `Options.Integration`, the directory, used by
   `Start` for bash (`--init-file`) and zsh (`ZDOTDIR`, the user's in `ROTARI_USER_ZDOTDIR`, cleared at start);
   `KeeperOptions.ShellIntegration` making the directory in `NewKeeper` and removing it in `Close`. Tests with a real
   bash, and zsh where installed: the user's login files read (and a `ZDOTDIR` set in `.zshenv` followed), `ZDOTDIR`
   as it was afterwards, `$?` kept for a `PROMPT_COMMAND` / `precmd` of the user's, `B` put back after a theme sets
   `PS1`, and the marks `D;1 A B C` for `false`; `/bin/sh` unchanged.
6. **The option.** `--terminal-shell-integration` (default true), `terminal-shell-integration` under `[web]`, and
   `ROTARI_WEB_TERMINAL_SHELL_INTEGRATION`, as the other terminal options are read; in the detached server's settings
   and their differences.
7. **Contracts and docs.** A contract (WEB-49) for the option and what it starts, conformance through the binary
   with bash; WEB.md (blocks with nothing to set up, the option, what it does to bash and zsh), FAQ.md ("ordinary
   shells"), SECURITY.md, and the CLI reference.

8. **Commands.** The dropdown, sharing the Recent directories dropdown's code (moved into one `openDropdown`); the
   jump and its highlight; the row's copy buttons; the keys removed; WEB-48, docs/WEB.md, and the browser step
   changed to it.
9. **Copy on the prompt's line.** A **Copy output** button at the right end of a prompt's line, shown while the
   pointer is over the command's prompt or output; WEB-48, docs, and the browser step.

## Tests and validation

- JSDOM: blocks from a stream of marks, split across writes; exit codes 0, 1, and none; a block without `B` or `C`;
  marks ignored in the alternate screen; blocks dropped when their markers are disposed and on reset; decorations
  and scroll bar marks asked for with the right colors; the keys moving to the previous and next prompt and not
  reaching the shell; copy and select taking the right lines, including wrapped ones; typing again refused while a
  command runs; a shell sending no marks getting no gutter, decorations, or key handling.
- Browser (CI only): the step in phase 4.
- `go test ./internal/webui ./conformance/05-web` with Node 24, `tsc`, and prettier.

## Non-goals

- Changing the user's startup files. (rotari's own startup files for bash and zsh, with an option, are decision 1.)
- Integration for shells other than bash and zsh.
- Grouping blocks by directory, folding output, sticky command lines, or re-running a block with Enter.
- Exit codes on the server: in **All shells**, the bell, or notifications. These could use the marks later.
- VS Code's OSC 633, iTerm2's OSC 1337 marks, and OSC 7 directories in the blocks.
- Keeping blocks beyond what the ring and the scrollback hold.

## Settled questions

All three were accepted as proposed on 2026-11-14.

1. **How do shells get the marks?** Proposed: docs only, with lines for bash and zsh to paste into one's own startup
   file. The other choices are a `rotari` command that prints the lines, or an opt-in flag with which rotari adds
   them to the shells it starts; both change the "ordinary shells" promise more than docs do. *Revisited
   2026-11-14:* after phase 4 the user found pasting lines more than anyone would do, and chose an option with which
   rotari adds them, on by default (decision 1).
2. **Keys.** Proposed: Ctrl+Shift+ArrowUp / ArrowDown. VS Code uses Ctrl+ArrowUp / ArrowDown, which shells and
   editors in the terminal use for moving by word or paragraph, and which this panel would then take from them.
3. **Directory grouping.** Proposed: left to a later plan, which would need blocks' directories (OSC 7) and,
   to outlive the ring, a small block index on the server.

## Coordination

Implementation starts from `main` at `b7d74f9d` in the `terminal-blocks` workspace and is rebased onto `main` before
integrating.
