# Plan: Fast output in the Web terminal's page

Created: 2026-11-24
Status: Phase 1 is done. After its results the user chose, on 2026-11-24, a change to the page alone over the
acknowledgements first planned. Phases 2 to 4 are implemented; see [work-log.md](work-log.md). Not done: the browser
test, and the checks by hand.
Related: [Web terminal](../2026-11-05-web-terminal/plan.md),
[folding output](../2026-11-16-terminal-fold-output/plan.md), and "A very fast program can make the terminal panel lose
output" in [ISSUES.md](../ISSUES.md)

## Purpose

The terminal's page must never lose output part way and go on as if nothing were missing. When it cannot keep up, it
drops the connection, and comes back with the replay, saying so.

## Problem

`onmessage` gives xterm.js each message at once. With more than 50 MB unparsed, `term.write` throws `write data
discarded`, and that message is gone. The page cannot pause a WebSocket, and goes on receiving.

1. **A hidden page.** Browsers slow its timers, in which xterm.js parses. Output piles up and is lost.
2. **A large replay.** `--terminal-replay-size` may be up to 64 MiB, sent as one message. xterm.js takes it, since it
   checks its limit before adding, and throws on what follows until enough is parsed.
3. **Output during a rewrite** (from reading the code). While folding rewrites the screen, output waits in the pane's 8
   MiB record. What falls out of it is gone: `recordSlice` returns null, and `feed` writes nothing, silently.

In 1 and 2, `recordBytes` and `scanMarks` have already taken the message when `write` throws, so folding and blocks can
disagree with the screen as well.

## Measured on 2026-11-24

On the shared development machine, with few CPUs; single runs, so only the order of magnitude matters.

| What | Rate |
| --- | --- |
| Vendored xterm.js 5.5.0 parsing 100-character lines, 200 columns (Node 24, no DOM, no drawing) | 16 MB/s plain, 15 MB/s with colors |
| `Keeper` to a Go reader, without a WebSocket, 100-character lines | 105 MB/s |
| The same, 4-character lines | 18 MB/s |

xterm.js alone threw once 50.0 MB was queued. A Node 24 client with the vendored xterm.js, connected to a real `rotari
web` over its WebSocket, lost 92.8 MB of 200 MB, and after Ctrl-C lost the prompt too. That turned out **not** to
predict what a browser does.

## Phase 1 results

Headless Chrome on GitHub's `ubuntu-latest` and `macos-latest`, on 2026-11-24, from a temporary branch since deleted
(runs [45359759648](https://github.com/rotari-dev/rotari/actions/runs/45359759648),
[45360358427](https://github.com/rotari-dev/rotari/actions/runs/45360358427),
[45361645995](https://github.com/rotari-dev/rotari/actions/runs/45361645995), and
[45362574593](https://github.com/rotari-dev/rotari/actions/runs/45362574593)). "Queued" is what xterm.js has been given
and not yet parsed.

| Scenario | Ubuntu | macOS |
| --- | --- | --- |
| Visible page, 200 MB of 100-character lines | Nothing lost; at most 1.2 MB queued | Nothing lost; at most 2.9 MB queued |
| Visible page, `yes` for 4 s, then Ctrl-C | Prompt after 0.11 s | Prompt after 0.11 s |
| Hidden page, `yes` for 15 s and more (two runs) | Nothing lost; parsed at 27 to 28 MB/s | **Lost** 78.8 MB and 0.5 MB; parsed at 0.27 and 0.013 MB/s |
| A full 64 MiB replay (first message 67.1 MB), then output | **Threw** once | **Threw** twice |
| A replay of at most 8.3 MB, then output | Nothing lost | Nothing lost |

- **A burst loses output only in a hidden page.** A visible page in Chrome kept up, for reasons not established; it
  seems that receiving does not get ahead of parsing there.
- **A late Ctrl-C**, which the first design was also for, was not seen.
- **The large replay is confirmed** on both systems. A first test missed it by coming back before the replay filled.
- **Output during a rewrite** was not tested.
- The second macOS hidden run ended soon after the queue reached 50 MB, so its loss says little about the rate. A hidden
  page on Ubuntu was not slowed there, which may be only headless Chrome.
- Firefox, Safari, and a visible page on a busy machine were not tested.

## Scope

The page's path from `onmessage` to `term.write`, in `web_shell.js`.

## Non-goals

- The server and the WebSocket's messages, which do not change.
- Making a program wait for a slow page. A slow connection still does (WEB-34).
- Faster parsing or drawing, and updating xterm.js or synchronized output, which would be plans of their own.
- A setting, CLI option, or Python interface.

## Decisions

1. **The page keeps a queue of its own per pane.** A message goes into it, and is given to xterm.js only while less than
   2 MiB given is still unparsed. A message that finds the queue empty and room is given at once, as today, so ordinary
   output is no later.
2. **Large messages are given in pieces** of at most 256 KiB. xterm.js and `scanMarks` both already take sequences and
   characters split anywhere.
3. **Output enters the record when it is given, not when it arrives.** The record then holds exactly what xterm.js was
   given. Otherwise a rewrite would write what is still queued, and the queue would write it again.
4. **Nothing is given during a rewrite.** Output waits in the queue, which replaces the rewrite's own list of waiting
   callbacks (`rewriting.after`) and the output it takes back from the record.
5. **More than 8 MiB queued, the replay aside, ends the connection.** The queue is dropped. A hidden page reconnects
   when it is visible, as `reconnect` already does; a visible one does so after about a second, without taking the shell
   from another tab. The program goes on meanwhile, printing to the server's ring. Chosen by the user over slowing the
   program, which could hold up a long job whose tab is merely out of sight.
6. **The page says so.** After such a reconnect a note says that the terminal fell behind and shows only recent output.
7. **The queue outlives the connection when the shell ends**, so that its last output is still shown. A new connection
   drops it, since the replay brings that output back.

### Rejected

- **Acknowledgements from the page, with a window in the server.** The first design. It would keep WEB-34's promise for
  a slow visible page, but phase 1 found no such page in Chrome, and a hidden page must not hold a program anyway.
- **Only catching the exception and reconnecting.** It needs 50 MB of backlog first, and the record and marks are wrong
  by then.

## Variants to cover

| Dimension | Values |
| --- | --- |
| Page | visible, hidden, hidden then visible, panel closed, own window, up to four panes |
| Output | typing, a burst, one message over 8 MiB, sequences and UTF-8 split across pieces, marks across pieces |
| Replay | empty, 256 KiB, 64 MiB, with output following; folds applied after it |
| Event with output queued | shell exit, `DELETE`, taken over, connection lost, sign-out, server stop, tab or group switched, pane closed, **Clear**, a line sent to the shell |
| Screen | normal, alternate, during a fold's rewrite |

What **Send to terminal** waits for (`pendingSend`) and what follows a replay (`blocksLive`, `refold`) still happen once
that message is parsed, not once it is queued.

## Phases

1. **Reproduce.** Done; see the results above.
2. **The queue**: decisions 1 to 4 and 7, with their tests.
3. **The limit**: decisions 5 and 6, with their tests.
4. **Contracts and docs**, and the ISSUES.md entry moved to Resolved.

## Contracts and documentation

- WEB-33, which covers the panel, gains decisions 5 and 6; WEB-34 says its order without loss is the server's, to a
  reader that keeps reading. Their rows in [contracts/README.md](../../contracts/README.md) follow. No new ID.
- [docs/WEB.md](../../docs/WEB.md): a tab that falls far behind, usually a hidden one, comes back with only its recent
  output, and the program is not slowed.

## Tests and validation

- `TestTerminalPanel` (JSDOM), with a stand-in that parses only when told and throws over 50 MB as xterm.js does: what
  is given and when, pieces, a rewrite with nearly 8 MiB arriving, the limit for a hidden and a visible page, the
  note, and each event in the table. Each must fail before the change where it covers a fault.
- `TestTerminalFastOutputInBrowser`, from the reproducer (local commit `4abd039c`, whose replay scenario goes to the
  wrong page and must wait for the replay to fill): the full 64 MiB replay, which failed on both systems, and a hidden
  page, which failed on macOS only.
- **Time in CI.** `conformance/05-web` took 11 minutes of a 15-minute limit on Ubuntu on 2026-11-24. The new tests must
  add at most about a minute, waits that run out included.
- By hand: `vim`, `less`, `tmux`, and an AI agent's TUI behave as before, and a tab hidden during `yes` comes back with
  its note.

## Risks

- **Output stuck in the queue.** If a piece's callback never comes, such as after `term.reset` at the wrong time,
  nothing more is shown. Connecting, switching, and rewriting each need a test that output goes on afterwards.
- **A large replay with fast output.** If 8 MiB arrives while a replay of tens of MiB is parsed, the page lets go again
  and starts the replay over, until the program slows. The shell still takes keys between, such as Ctrl-C. Only a raised
  `--terminal-replay-size` can reach this.
- **`web_shell.js` is changed by other work** at the same time. Phases 2 and 3 are done in a workspace of their own from
  `main`.
