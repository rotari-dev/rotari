# Work log: Blocks in the Web terminal

## 2026-11-14 12:40 +09:00 — Parsing and the block model (`fada514399eb`)

- **Change:** `web_shell.js` registers an OSC 133 handler on each pane's terminal (`readBlockMark`). `A` starts a
  block with a marker where the prompt is; `B` and `C` record where the command line and the output start (marker
  and column); `D;EXIT` ends it with the exit code, `null` when it is missing or not an integer. A prompt with no `D`
  before it ends the last block without an exit code. Marks after a block's end, or with no block, add nothing, and
  marks in the alternate screen are ignored. When a prompt's marker is disposed (scrolled out, or **Clear**, which
  clears all markers), its block and its other markers go. `open` drops every block before `term.reset()`, which
  does not dispose markers itself. `pane.blocksLive` turns on when the connection's first message has been parsed,
  and only then do `C` and `D` stamp `startedAt` and `endedAt`. `blockText` and a block's `command()` give the text
  from `B` to `C`, wrapped lines joined and other line ends kept. The pane's state is reachable as
  `.terminal-pane`'s `terminalPane`, as rows carry `dirEntry`. The JSDOM fake terminal gains `parser`,
  `registerMarker`, an `osc` helper, cursor positions, and `translateToString`; the test is at the end of
  `TestTerminalPanel`.
- **Reason:** plan phase 1.
- **Plan impact:** decision 3 now says how the replay is told apart. `registerMarker` is not a proposed API in this
  build, so `allowProposedApi` waits for phase 2's decorations.
- **Validation:** `tsc` clean; prettier unchanged. `go test ./internal/webui ./conformance/05-web` passed with Node
  24 and `NODE_PATH` (under `taskset -c 0-7`). Nothing is shown yet, so there is nothing to look at in a browser.
- **Remaining:** phases 2 (display), 3 (moving and acting), and 4 (WEB-47, docs, the browser step).
