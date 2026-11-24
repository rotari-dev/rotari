# Plan: Flow control between the Web terminal's page and server

Created: 2026-11-24
Status: Design accepted by the user on 2026-11-24, with both open questions as proposed; not started. Phase 1 decides
whether phases 2 to 4 are done.
Related: [Web terminal](../2026-11-05-web-terminal/plan.md),
[folding output](../2026-11-16-terminal-fold-output/plan.md), and "A very fast program can make the terminal panel lose
output" in [ISSUES.md](../ISSUES.md)

## Purpose

A program that prints faster than the page can parse must neither lose output in the page nor leave Ctrl-C's effect
waiting behind seconds of old output. The page tells the server how much it has parsed, and the server sends only a
limited amount beyond that.

## Problem

Output passes through these queues:

| Queue | Limit today | When full |
| --- | --- | --- |
| `Attachment.pending` in the server | 1 MiB (`pendingLimit`) | The program waits; after 30 s without a read, the reader is detached |
| Network and the browser's WebSocket | Their buffers | The server's write waits |
| xterm.js's write queue | 50 MB | `term.write` throws `write data discarded` |

A page cannot pause a WebSocket. `onmessage` only queues bytes in xterm.js, so the browser goes on receiving while
xterm.js is behind, and the server never sees the delay. Contract WEB-34 says a slow reader makes the program wait; that
holds for a slow connection, but not for a slow page.

Consequences:

1. **Lost output.** With more than 50 MB waiting, `term.write` throws from `onmessage`. The chunk's marks have already
   been queued by `scanMarks`, so blocks can disagree with what is on screen as well.
2. **Late Ctrl-C.** What is queued is still shown after the program stops: up to 50 MB, about 3 s at the rate below.
3. **A large replay** (from reading the code). `--terminal-replay-size` may be up to 64 MiB, sent as one message.
   xterm.js checks its limit before adding to the queue, so it takes the replay, then throws on the next output until
   enough is parsed.
4. **Output during a rewrite** (from reading the code). While folding rewrites the screen, output waits in the page's
   8 MiB record. Beyond that its oldest bytes are dropped, `recordSlice` returns null, and `feed` writes nothing,
   silently.

### Measured on 2026-11-24

On the shared development machine, with few CPUs; single runs, so only the order of magnitude matters.

| What | Rate |
| --- | --- |
| Vendored xterm.js 5.5.0 parsing 100-character lines, 200 columns (Node 24, no DOM, no drawing) | 16 MB/s plain, 15 MB/s with colors |
| `Keeper` to a Go reader, without a WebSocket, 100-character lines | 105 MB/s |
| The same, 4-character lines | 18 MB/s |

xterm.js alone threw once 50.0 MB was queued. The loss has **not** been reproduced through `rotari web` and a browser;
phase 1 does that.

## Scope

- Flow control for the WebSocket at `/_terminal/ws`, from the page's parsing back to the program.
- The four consequences above.

## Non-goals

- Faster parsing or drawing: the WebGL addon, or parsing in a worker.
- Synchronized output (DEC mode 2026) and updating xterm.js, which would be a plan of its own.
- Compressing messages, or skipping output nobody will see.
- A CLI, config, or Python interface. Nothing here is for the user to set.

## Decisions

1. **The page asks for it.** It connects with `ack=1`. Without `ack`, a connection behaves as today, so an open page
   older than the server, or another client, is not held up by acknowledgements it never sends. Any other value is
   refused with 400.
2. **The message.** `{"type": "ack", "bytes": N}`, where `N` is the total output bytes of this connection, replay
   included, that the page is done with. A total cannot drift as increments can. An `N` that goes down or exceeds what
   was sent, or an `ack` without `ack=1`, closes the connection as a policy violation, as an unknown message does.
3. **The window is 2 MiB.** The server sends at most that beyond the last `N`. It is far below xterm.js's 50 MB, about
   0.15 s of parsing, and allows 20 MB/s at a 100 ms round trip, more than xterm.js parses. The first message is sent
   whole whatever its size, since the page takes it as the replay; nothing follows it until the window has room.
4. **The page acknowledges every 256 KiB parsed**, from `term.write`'s callback. Ordinary typing therefore sends
   nothing. The remainder needs no acknowledgement, being far below the window.
5. **The wait is in `Attachment.Next`**, not in the WebSocket handler, so that being taken over, the shell ending, and
   the server stopping still end it at once. Once the attachment has ended, its last output is sent regardless of the
   window, as today.
6. **The existing limits stay.** While `Next` waits, `pending` fills to 1 MiB and the program waits, as for a slow
   connection. A page that frees no room for 30 s while the program waits is detached (4003) and reconnects to a
   replay.
7. **A hidden page must not hold a program.** Browsers throttle timers in hidden pages, and xterm.js parses in timers,
   so acknowledging only what is parsed would slow a program whose tab is merely out of sight, which does not happen
   today. While `document.hidden`, the page acknowledges what it has *received*. If more than 8 MiB then waits to be
   parsed, it closes the connection, and reconnects when visible, as `reconnect` already does. The program goes on
   meanwhile, printing to the ring. Such a tab comes back with only the replay. Chosen by the user over slowing the
   program, which would keep its output whole but could hold up a long job whose tab is merely out of sight.
8. **Nothing is silently dropped in the page.** If `feed` gets no bytes because the record no longer holds them, the
   page reports the error and reconnects to a replay. With the window this should not happen.

### Rejected

- **Limiting the send rate.** The page's speed varies with the machine, the width, and what is printed.
- **Stopping WebSocket reads.** Browsers have no such API in common use; `WebSocketStream` is not in all of them.
- **Dropping output when behind.** It would break WEB-34's order without loss, and can cut escape sequences in half.
- **Enlarging xterm.js's queue.** It is not an option of xterm.js, and would make Ctrl-C later still.

## Design

- **`internal/terminal`**: `Attachment` counts the bytes `Next` has returned and those acknowledged. A method turns
  the window on, and `Acknowledge(total)` checks `total` and wakes `Next`. With the window on, `Next` returns at most
  the remaining room (after the first message). The package still knows nothing of WebSockets.
- **`internal/webui/terminal.go`**: reads `ack`, turns the window on, and handles the `ack` message in
  `readTerminalInput`.
- **`web_shell.js`**: `received`, `parsed`, and `acknowledged` totals live on the connection, not the pane, so an old
  connection's callbacks cannot count for a new one. The `done` callback in `onmessage` counts. For output held during
  a rewrite, `done` must run after its bytes are parsed; today `rewrite` runs it when the write is queued.

## Variants to cover

| Dimension | Values |
| --- | --- |
| Page | visible, hidden, hidden then visible, panel closed, in its own window, up to four panes |
| Client | with `ack=1`, without, a wrong total, an `ack` without `ack=1` |
| Event while waiting | taken over, shell exit, `DELETE`, sign-out, server stop, connection lost, tab switched |
| Screen | normal, alternate, during a fold's rewrite, after **Clear** |
| Replay | 256 KiB, 64 MiB, empty |

## Phases

1. **Reproduce.** A browser test prints at least 200 MB of 100-character lines, ends with a marker, and checks that
   `term.write` never throws and the marker appears. It also records how long Ctrl-C takes to show the prompt, what a
   hidden page parses per second, and the 64 MiB replay. If neither the loss nor a Ctrl-C delay over 1 s appears in a
   real browser, stop here, record the figures in ISSUES.md, and fix only consequences 3 and 4 (agreed with the user
   over going on for the sake of WEB-34's promise).
2. **Server.** The window, `ack`, and their tests. No page uses them yet.
3. **Page.** Acknowledgements, the hidden rule, and the rewrite's callbacks.
4. **Contracts and docs**, and the ISSUES.md entry moved to Resolved.

## Contracts and documentation

- WEB-34 is extended: what a reader "keeps reading" means for a page, the `ack` message, and the hidden rule. Its row
  in [contracts/README.md](../../contracts/README.md) and its tests follow. No new ID.
- [docs/WEB.md](../../docs/WEB.md): a program that prints faster than the page shows is slowed, and a hidden tab that
  falls far behind comes back with only its recent output.

## Tests and validation

- `internal/terminal`: `Next` stops at the window and goes on after `Acknowledge`; the program waits meanwhile; the
  first message is whole; every ending in the table wakes `Next`; a bad total is refused; without the window, the
  existing tests pass unchanged.
- Conformance through the binary: a client with `ack=1` that stops acknowledging gets at most the window beyond the
  replay, and then every byte in order once it goes on. `TestTerminalStreamsOutputBurst`, which sends no `ack`, stays
  as it is.
- `TestTerminalPanel` (JSDOM): when acknowledgements are sent, the hidden rule and its limit, a rewrite, and a tab
  switch.
- The phase 1 browser test in CI, as there is no local Chrome.
- By hand, over a forwarded SSH port: Ctrl-C during `yes` shows the prompt within about 1 s, and `vim`, `less`, and an
  AI agent's TUI behave as before.

## Risks

- **A missing acknowledgement stops output.** A path that forgets to call `done` would freeze the terminal for up to
  30 s before the reconnect. The variants above exist for this.
- **An older server under an open page.** A server downgraded while a page stays open closes the connection on `ack`,
  and the page reconnects repeatedly until reloaded. Accepted, as the page is served by the server itself.
