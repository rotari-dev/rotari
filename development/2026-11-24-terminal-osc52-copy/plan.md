# Plan: Copying from programs with OSC 52 in the Web terminal

Created: 2026-11-24
Status: Proposed. Phase 1 was done on 2026-11-25 except that Neovim was read about, not run; see [Phase 1
results](#phase-1-results). Nothing is implemented.
Related: [Web terminal](../2026-11-05-web-terminal/plan.md),
[fast output](../2026-11-24-terminal-flow-control/plan.md), and the two plans proposed with this one:
[font](../2026-11-24-terminal-font/plan.md) and [Option as Meta](../2026-11-24-terminal-option-meta/plan.md)

## Purpose

`tmux` and Neovim can copy to the clipboard of the machine in front of the user by printing OSC 52, also through `ssh`.
The panel ignores it, so a yank in Neovim or `tmux`'s copy mode stays on the server. Selecting with the mouse does not
replace it: with mouse reporting on, Neovim's default and an option of `tmux`, the program gets the mouse, and a
selection across `tmux` panes takes in both.

## Today

Nothing handles OSC 52: xterm.js 5.5.0 leaves it to an addon that rotari does not include. The page writes the clipboard
only when someone chooses **Copy** or the like, and [docs/SECURITY.md](../../docs/SECURITY.md) relies on that: a false
mark "can at worst put text in front of you".

## Risk

Anything that can print in a terminal could then replace the clipboard: a file shown with `cat`, a job's log, a remote
host over `ssh`. Text waiting there can be pasted elsewhere and run. So this is off unless chosen.

## Phase 1 results

tmux 3.2a, the one on the development machine, on 2026-11-25: run with no configuration on a pty with
`TERM=xterm-256color`, answered as xterm.js 5.5.0 answers (`ESC [ ? 1 ; 2 c`, `ESC [ > 0 ; 276 ; 0 c`).

| What | OSC 52 |
| --- | --- |
| Copy mode, and a drag with `mouse on`, with ASCII and UTF-8 | Printed, with the selection empty, ended by BEL |
| `set-buffer -w` | The same |
| A program inside `tmux` prints it | Swallowed by default (`set-clipboard external`); printed with `set -s set-clipboard on` |
| A program inside wraps it for `tmux` | Passed on as it is |
| `load-buffer -w` of 10,000 bytes | Printed |
| `load-buffer -w` of 50,000 bytes or more | Not seen. Whether `tmux` drops it, or this harness missed it, is not known |

So `tmux`'s own copying needs no configuration, and one line lets programs inside it copy.

Neovim is not on the machine. From its [documentation](https://neovim.io/doc/user/provider/) and its `osc52.lua` files
on `master`, of 2026-11-24, with no version stated:

- It prints `ESC ] 52 ; c ; BASE64 ESC \` for `+` and `p` for `*`, ended by ST.
- It uses OSC 52 by itself when nothing else provides a clipboard and the terminal says it can: `52` in its answer to
  `ESC [ c`, or an answer to a query for `Ms`. xterm.js gives neither. `vim.g.clipboard = 'osc52'` makes it, in one
  line.
- With it, `p` from `+` or `*` **asks the terminal** (`?`), and waits about 10 seconds, less with Ctrl-C, when nothing
  answers. That is what decision "never answered" gives a user who adds that line.

tmux and Neovim both print it with at most a line of configuration, so the plan goes on, once open question 2 is
settled.

## Scope

Writing the clipboard from OSC 52, for the panes of the panel and its own window.

## Non-goals

- **Reading the clipboard** (`OSC 52 ; c ; ?`). It is never answered, whatever the setting: any program, local or
  remote, could take what is there.
- The primary selection, which browsers do not have.
- Clipboard content other than text.
- A CLI, config, or server setting. The clipboard is the browser's, and a server must not turn this on for it.

## Decisions

1. **Off by default, chosen per browser**: **Let programs copy** in the bar's Options menu of the [font
   plan](../2026-11-24-terminal-font/plan.md), marked when on. The pane's right-click menu does not open while a program
   takes the mouse. Turning it on says what it allows. Only `"1"` under `rotari-terminal-osc52`
   is on. When storage is refused, it lasts for the page, and a note says so.
2. **Every copy is shown**: a note says how many characters a program copied.
3. **Only for live output.** A replay and a fold's rewrite print old bytes again, and must not copy again: nothing is
   copied until the replay is parsed, nor during a rewrite. A note does not appear either.
4. **Only where the user is**: the pane is the focused one, and the page is visible and has focus. Otherwise nothing is
   copied, and a note with a **Copy** button offers it when the user comes back; only the latest is kept, in memory.
5. **When the browser refuses**, as those that want a key press or click first do, the same note with **Copy** is shown.
   A success is never reported before the browser says so.
6. **What is accepted**: the selection empty or containing `c`; standard base64 of valid UTF-8; at most 1 MiB decoded.
   Anything else copies nothing and a note says why. An empty text does not clear the clipboard.
7. **While it is off**, the first OSC 52 seen live in a page is told once, as a note naming the setting, so that it is
   not a silent failure. Later ones are not.
8. **No new dependency.** The handler is a few lines with `registerOscHandler`, and xterm.js's clipboard addon would
   also bring reading.

### Rejected

- **On by default**, as some terminals have it. The page is also where job logs from anywhere are shown.
- **Asking at each copy.** With Neovim copying each yank it would be turned off at once.

## Design

- `openTerminal` registers the handler beside those for OSC 133 and OSC 7. `pane.blocksLive` already says that a replay
  is over and no rewrite runs; it is renamed for both uses, or given a function of its own, so that one rule decides.
- The base64 is decoded by one function that returns the text or a reason, with the limit applied before decoding.
- The note with a button is shared with the panel's other notes.

## Variants to cover

| Dimension | Values |
| --- | --- |
| Setting | off, on, changed while a program takes the mouse, stored badly, storage refused, changed in another tab |
| Sequence | BEL and ST ends, split across messages, empty selection, `c`, `p`, query, bad base64, bad UTF-8, empty, too long |
| Moment | live, replay, fold's rewrite, reconnect, hidden page, other pane focused, panel closed, alternate screen |
| Browser | allowed, refused, no clipboard API |

## Contracts and documentation

- A new contract ID, the next free when implemented.
- [docs/WEB.md](../../docs/WEB.md): the setting, and how to turn it on in `tmux` and Neovim, checked by hand first.
  [docs/SECURITY.md](../../docs/SECURITY.md): the risk above, that reading is never answered, and what it now says of
  the clipboard.

## Phases

1. **Check by hand** what `tmux` and Neovim print under `TERM=xterm-256color`, and what they need to be told, in a
   terminal that shows it. If the usual versions do not print OSC 52 here without more than a line of their
   configuration, decide again.
2. **The setting and the handler**, with their tests.
3. **Contracts and docs.**

## Tests and validation

- `TestTerminalPanel` (JSDOM): each row above, with a clipboard that records, refuses, or is missing. A replay that
  holds OSC 52 and a fold over one must copy nothing, and a query must send nothing to the shell.
- By hand: a yank in Neovim and a copy in `tmux`, directly and through `ssh`, in Chrome, Firefox, and Safari.

## Open questions

1. **Telling the user while it is off** (decision 7). Proposed: once per page. The alternative says nothing, and leaves
   it to the docs.
2. **What a query gets.** Never answering keeps a Neovim set to `osc52` waiting 10 seconds at each `p` from `+`.
   - **Proposed: answered as empty**, only while the setting is on, and only where a copy would be taken (decision 3 and
     4). It tells nothing of the clipboard, and Neovim goes on at once, pasting nothing. Like any answer, it is typed
     into whatever runs, so never for a replay.
   - The alternative never answers, and the docs give a Neovim configuration of several lines whose paste does not ask.
3. **Saying that it can.** Answering Neovim's `Ms` query while the setting is on would make it copy with no
   configuration. Proposed: not at first. It is one more answer, and it depends on how Neovim asks.
