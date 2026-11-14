# Work log: Blocks in the Web terminal

## 2026-11-14 12:40 +09:00 — Parsing and the block model (`90d4ac41`, rebased from `fada514399eb`)

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

## 2026-11-14 14:10 +09:00 — Display

- **Change:** each pane's terminal is made with `allowProposedApi: true`. A block gets one decoration on its
  prompt's line (`decorateBlock`), spanning the line (`width: 100%`, no pointer events) so that it can draw the
  separator, with a `.terminal-block-mark` child in the gutter that alone takes the pointer. `paintBlock` sets the
  mark's `data-state` (`prompt`, hidden; `running`, grey; `ok`, green; `failed`, red; `unknown`, a hollow ring) and
  its title: the command line, `Running`, `Exit N`, `Ended without an exit code`, or `No exit code: another prompt
  came first`, with `after 1.5 s` / `1 min 15 s` / `1 h 2 min` for commands seen running. A failure remakes the
  decoration with `overviewRulerOptions` in `--terminal-red`, since xterm.js takes a ruler colour only when a
  decoration is made, and turns on `overviewRulerWidth` (10) the first time; a theme change remakes failures' marks.
  The viewport gets the 16px gutter (`terminal-viewport-blocks`) with its first block; a replay that brings no
  blocks removes it, so a shell that sends no marks keeps today's layout, and a marking shell's gutter does not flick
  off and on across its own replay. Colours are the terminal tokens, so both themes follow. The fake terminal gains
  `registerDecoration`, which throws without `allowProposedApi` as xterm.js does.
- **Reason:** plan phase 2.
- **Plan impact:** decision 3 now says that `D` without a code is hollow too, that a prompt with no command yet has
  no mark, and that separators follow output, not every prompt.
- **Validation:** `tsc` clean; prettier unchanged. `go test ./internal/webui ./conformance/05-web` (see the commit).
  Not seen in a browser: none here; CI's browser step does not look at blocks until phase 4.
- **Remaining:** phases 3 (moving and acting) and 4 (WEB-47, docs, the browser step).
