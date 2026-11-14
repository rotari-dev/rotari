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

## 2026-11-14 15:00 +09:00 — Moving and acting

- **Change:** `term.attachCustomKeyEventHandler` takes Ctrl+Shift+ArrowUp / ArrowDown on keydown, only with no Alt or
  Meta, outside the alternate screen, and only when the pane has a block with a command; they scroll to the prompt
  of the command before or after the top line (`moveToPrompt`), and down past the last goes to the bottom. Any other
  key, and those keys in a shell without marks, reach the shell as before. Clicking a mark focuses its pane and opens
  the panel's menu (`openPanelMenu`) with **Copy output**, **Copy command**, **Select output**, and **Type command
  again**; the mark stops `mousedown` and `mouseup` so a click on it does not start a selection. A block's output
  runs from `C` to `D`, or to the next prompt, or to the cursor while it runs (`blockOutputEnd`); **Select output**
  uses `term.select`. **Type command again** goes through `typeAtPrompt`, the "only at the prompt" check that
  `typeCd` had, now shared (`typeCd` calls it with "cd", so its messages are unchanged), and is off for a command of
  more than one line. The Explorer's `copyExplorerText` became `copyText(text, said)` and is used by both.
- **Reason:** plan phase 3.
- **Plan impact:** decision 4 now says that typing again waits for the preview as recent directories does, that
  multi-line commands are not typed again, and what a running command's output is.
- **Validation:** `tsc` clean; prettier unchanged. `TestTerminalPanel` passed, then `go test ./internal/webui
  ./conformance/05-web`.
- **Remaining:** phase 4 (WEB-47, docs, the browser step).

## 2026-11-14 15:40 +09:00 — Contracts and docs

- **Change:** contract **WEB-48** (the note before WEB-47 in `contracts/05-web-assets-and-static-export.md`, and its
  row), covered by `TestTerminalPanelInBrowser`, which now names WEB-33 and WEB-48. Its new step, on the project
  page after the run link: a `/bin/sh` `PS1` holding `D;$?`, `A`, and `B`, and a `c` function printing `C`; `c false`,
  `c long` (60 lines), and `c true`, one at a time so that none is echoed before its prompt; red and green marks with
  different, non-transparent colours, the green one left of `.xterm-screen` and the element under the pointer there;
  the overview ruler present; Ctrl+Shift+ArrowUp twice and ArrowDown once putting `$ c long`, `$ c false`, and
  `$ c long` on the top row; a real click on the green mark opening the four-item menu, and **Type command again**
  typing `c true`, which ` && echo "again-$((5+5))"` then completes and runs; and after a reload, the blocks back
  with `c true\nExit 0` and no time. docs/WEB.md's Terminal section describes blocks and gives lines for bash (4.4
  or later: `PROMPT_COMMAND` for `D` and `A`, keeping `$?` for whatever follows; `PS1` for `B`; `PS0` for `C`) and
  zsh (`precmd` first, `preexec`, `PS1`), and notes fish 4 and oh-my-posh; docs/FAQ.md and docs/SECURITY.md say what
  blocks are and that false marks change only the display.
- **Reason:** plan phase 4.
- **Plan impact:** the contract is WEB-48, since WEB-47 was taken by the `rotari web` client in the meantime.
- **Validation:** the `sh` line from the step, and the bash and zsh lines from WEB.md, were run in a pty here (dash,
  bash 5.1, zsh): `D;0 A B C D;1 A B C D;0 A B` for `false` then `true`, and no `C` for an empty line. The step's
  script passes `node --check`, and its string is the `sh` line tested. `TestContractStatus` passed, and so did
  `go test ./internal/webui ./conformance/05-web ./conformance`. The browser step itself has not run: there is no
  Chrome here, so its first run is CI's.
- **Remaining:** nothing in this plan. Directory grouping, exit codes on the server, and the other non-goals are left
  to plans of their own.

## 2026-11-14 18:10 +09:00 — Revision: marks without editing startup files

- **Change:** none in code; the plan. After phase 4 the user found adding lines to `~/.bashrc` or `~/.zshrc` more
  than anyone would do, and asked for it to be an option, on by default. Decision 1 now has rotari start bash and
  zsh with files of its own that read the user's, as VS Code does; settled question 1 records the change; phases 5
  (the integration in `internal/terminal`), 6 (the option), and 7 (contract and docs) are added.
- **Reason:** the user's request.
- **Plan impact:** as above. The bash and zsh lines in docs/WEB.md become what the option does for you.

## 2026-11-14 18:40 +09:00 — Shell integration

- **Change:** `internal/terminal/integration/` holds `rotari.bash` and zsh's `zshenv`, `zprofile`, `zshrc`, and
  `zlogin`, embedded (the zsh ones written as dot files, which go:embed would skip). `WriteIntegration` writes them
  into a new 0700 directory under the temporary directory. `Options.Integration` names it; `shellCommand` then
  starts a shell whose program is `bash` with `--init-file DIR/rotari.bash` (not `-l`, which `--init-file` would
  ignore), and `zsh` with `-l`, `ZDOTDIR=DIR/zsh`, and the `ZDOTDIR` it was to have in `ROTARI_USER_ZDOTDIR` (a
  `ROTARI_USER_ZDOTDIR` already in the environment is dropped); any other shell, and every shell without it, starts
  with `-l` as before. `rotari.bash` reads `/etc/profile` and the first of `~/.bash_profile`, `~/.bash_login`,
  `~/.profile`, then puts `__rotari_keep` first in `PROMPT_COMMAND` (string, joined by lines since the user's may end
  with `;`, or array) and `__rotari_prompt` last, which prints `D;$status` and `A` and puts `B` back at the end of
  `PS1` when missing; `PS0` gets `C`. The zsh files each set `ZDOTDIR` to the user's directory, source the user's file
  of their name at top level (not in a function, where `typeset` would make the user's variables local), take any
  `ZDOTDIR` it set, and point back; `.zshrc` adds `__rotari_keep` first and `__rotari_prompt` last in
  `precmd_functions` and `__rotari_preexec` (`C`) in `preexec_functions`; `.zlogin` leaves `ZDOTDIR` as the user had
  it, unset when it was and nothing set it. `KeeperOptions.ShellIntegration` makes `NewKeeper` write the directory
  and `Close` remove it, logging a failure. The option is not wired to `rotari web` yet (phase 6).
- **Reason:** plan phase 5.
- **Plan impact:** none.
- **Validation:** `TestBashIntegrationReadsTheLoginFilesAndMarks` (`.bash_profile` read and not `.profile`, `$?` kept
  for the user's `PROMPT_COMMAND`, which also rebuilds `PS1`, and marks `C D;1 A B` for `false`; `D;1 A B` for bash
  before 4.4, as macOS has), `TestZshIntegrationReadsTheUsersFilesAndMarks` (files from `$HOME`, from a given
  `ZDOTDIR`, and from one `.zshenv` sets; all four in order; `$?` kept for a `precmd_functions` theme; `ZDOTDIR` as it
  was; `ROTARI_USER_ZDOTDIR` gone), `TestShellCommandWithIntegration`, and `TestKeeperWritesAndRemovesTheIntegration`,
  with bash 5.1 and zsh here; three runs in a row. Taking `return` out of `__rotari_keep`, or the line that puts `B`
  back, fails the bash test. `go test ./internal/terminal` passed; `go build ./...` and `go vet` clean.
- **Remaining:** phases 6 (the option) and 7 (contract and docs).

## 2026-11-14 19:20 +09:00 — The option

- **Change:** `rotari web --terminal-shell-integration` (a boolean, default true), `terminal-shell-integration` under
  `[web]` in any config, and `ROTARI_WEB_TERMINAL_SHELL_INTEGRATION`, read as the other terminal options are
  (`readWebConfigured`; a value that is not a boolean is an error naming its source). It is one of
  `webTerminalOptions`, so it applies only with `--allow-terminal` and is refused with `--static-dir`.
  `webTerminalSettings.integration` reaches `KeeperOptions.ShellIntegration`. A detached server's record keeps
  `terminal_no_shell_integration` when it is off, so a record written before this means the default, and a second
  `--detach` that asks otherwise reports `terminal-shell-integration` as a difference. The CLI schema, the Python
  client's `generated_cli.py` (formatted with the pre-commit pin, ruff 0.17.1), docs/CLI_REFERENCE.md,
  docs/ENVIRONMENT_VARIABLES.md, and the golden schema were regenerated.
- **Reason:** plan phase 6.
- **Plan impact:** none.
- **Validation:** new cases in `TestWebTrustedOptionsComeOnlyFromTrustedConfigs` (on by default; off from a project
  config, from the environment over a config, and from the command line), `TestWebConfiguredBadValuesAreErrors`,
  `TestDifferences`, and the static-pair tests (`--terminal-shell-integration=false` refused with `--static-dir`).
  `generate_cli_reference.py --check` passed, and so did `go test ./cmd/rotari ./internal/webdetach
  ./internal/terminal ./conformance ./conformance/03-interfaces/pairweb ./conformance/05-web`.
- **Remaining:** phase 7 (contract, conformance through the binary, docs).

## 2026-11-14 20:10 +09:00 — Contract and docs for shell integration

- **Change:** contract **WEB-49** (note before WEB-48, and its row): what `--terminal-shell-integration` starts, which
  files are read from where, `ZDOTDIR` afterwards, the marks and their order with the user's hooks, the directory's
  life, and the ways to turn it off; WEB-34's "runs as `$SHELL -l`" and WEB-36's list of options read from every
  config, and WEB-48's opening, point to it. `TestTerminalShellIntegration` covers it through the binary: bash, and
  zsh where installed, each with the option on (`D;1`, `A`, `B` after `false`, the user's files read) and off (the
  files read, no OSC 133 at all), the user's files unchanged after. docs/WEB.md now says bash and zsh need nothing set
  up, what rotari does to them, and how to turn it off, in place of the lines to paste, and lists the flag in both
  option tables; docs/FAQ.md's "ordinary shells" and docs/SECURITY.md say what the files do.
  While writing the docs, bash before 4.4 turned out worse off than phase 5 assumed: with no `PS0` there is no `C`,
  and a block without `C` has no mark at all, so macOS's bash 3.2 would show nothing. `rotari.bash` now, for bash
  before 4.4 and when the user's files set no `DEBUG` trap, sets one that prints `C` before the first command after a
  prompt (`__rotari_prompt` raises the flag; `__rotari_keep`, first after a line with no command, lowers it);
  `ROTARI_BASH_DEBUG_TRAP`, unset by the script, lets the test take that way with bash 5.1.
- **Reason:** plan phase 7, and the gap found in it.
- **Plan impact:** decision 1 says how earlier bash prints `C`; the non-goal of a `DEBUG` trap is dropped.
- **Validation:** `TestTerminalShellIntegration` (bash 5.1 and zsh, on and off) passed through the binary.
  `TestOldBashIntegrationMarksCommandsByTheDebugTrap` gives exactly `D;0 A B` for an empty line and `C D;1 A B` for
  `true | false`, with `$?` kept for the user's `PROMPT_COMMAND`; five runs and three more in a row passed, and
  lowering the flag nowhere makes it fail with a `C` for the empty line. No bash 3.2 here: macOS CI's is the first.
  `TestContractStatus` passed; see the commit for the package runs.
- **Remaining:** nothing in this plan.

## 2026-11-14 20:40 +09:00 — CI: the flag-pair inventory

- **Change:** `pairInventory` in `conformance/03-interfaces/flag_pairs_test.go` records `web` with 23 flags
  (fingerprint `9203714407001852`), counting `--terminal-shell-integration`. The pair samples already had it (phase 6),
  and the coverage document's 66 static-export pairs are unchanged, since the flag is refused with `--static-dir`.
- **Reason:** CI's `TestCLIFlagPairInventory` failed with `web: schema changed`. Phase 6 ran `pairweb` but not
  `./conformance/03-interfaces` itself, which holds the inventory.
- **Plan impact:** none.
- **Validation:** the failure reproduced here with the same fingerprint; `go test ./conformance/03-interfaces/...`
  then passed.
