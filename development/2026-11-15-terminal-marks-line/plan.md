# Plan: Marks from shells started inside the Web terminal, by a line in one's own startup file

Created: 2026-11-15
Status: Proposed; waiting for the user's decision on the open questions.

## Goal

With `--allow-terminal`, a bash or zsh started by hand inside a panel's shell (`zsh` typed at a bash prompt) can mark its
prompts and commands as the shells rotari starts do, so that blocks, Commands, Copy output, click to move, and the
right-click Cut work there too. It takes one line in one's own `~/.bashrc` or `~/.zshrc`, which does nothing outside a
rotari terminal; VS Code offers the same for its sub-shells.

This is the second of two plans for inner shells proposed on 2026-11-15, after [following
them](../2026-11-15-terminal-nested-shells/plan.md), which made the Explorer and the prompt checks follow them.

## Existing implementation

- **Shell integration** (WEB-49, [Blocks plan](../2026-11-14-terminal-blocks/plan.md) decision 1). With
  `--terminal-shell-integration`, on by default, the server writes `rotari.bash` and zsh's startup files to a private
  temporary directory and starts bash with `--init-file` and zsh with `ZDOTDIR` there. Each reads the user's files and
  then adds the hooks: bash's `PROMPT_COMMAND`, `PS0` or a `DEBUG` trap, and `__rotari_c`; zsh's `precmd`, `preexec`,
  `line-init`, `line-pre-redraw`, and the `ESC [ 9998 ~` widget. The hooks and the startup work are in the same files.
- **An inner shell** reads only the user's files: `zsh` reads `~/.zshenv` and `~/.zshrc` (rotari's `ZDOTDIR` is put back
  before the first prompt), and `bash` reads `~/.bashrc`. It inherits the environment of the shell it was started in.
- **The panel** reads marks from the stream whatever process prints them, so an inner shell's marks need nothing new
  there.

## Decisions

1. **The hooks on their own.** The hooks move into `marks.bash` and `marks.zsh` in the same directory, which only add the
   hooks and change nothing else of the shell. rotari's startup files source them after the user's files, as now. Each
   loads once per shell: a shell variable (not exported) set on loading makes a second sourcing do nothing, so the line
   in a user's file, read by a shell rotari started too, does not add the hooks twice.
2. **The environment variable.** Every shell the server starts has `ROTARI_SHELL_INTEGRATION` set to the directory, and
   a shell started inside inherits it. Without `--terminal-shell-integration` the server writes no files and sets no
   variable, so the line does nothing.
3. **The line.** docs/WEB.md gives, for `~/.bashrc`:
   ```bash
   [ -r "$ROTARI_SHELL_INTEGRATION/marks.bash" ] && . "$ROTARI_SHELL_INTEGRATION/marks.bash"
   ```
   and for `~/.zshrc`:
   ```zsh
   [[ -r $ROTARI_SHELL_INTEGRATION/marks.zsh ]] && . "$ROTARI_SHELL_INTEGRATION/marks.zsh"
   ```
   Outside a rotari terminal the variable is unset, and after the server stops its directory is gone; either way the
   test fails and nothing is read.
4. **Where it does not help.** `ssh` does not pass the variable, so remote shells are unchanged. A shell inside `tmux` or
   `screen` loads the hooks, but its marks go to tmux's or screen's terminal, which does not pass them on, so the panel
   sees none: harmless, and the same as today.
5. **No server route or option.** The contract adds the variable and the marks files to WEB-49, and a new WEB-53 for
   the line's promise (inner shells marking, once per shell).

## Phases

1. **The hooks on their own.** `marks.bash` and `marks.zsh` split out of `rotari.bash` and zsh's `.zshrc`, with the
   once-per-shell guard; the startup files sourcing them; `ROTARI_SHELL_INTEGRATION` in the shells' environment. Tests
   with real shells: the shells rotari starts mark as before; a bash and a zsh started inside one, with the line in
   their startup file, mark too (`D`, `A`, `B`, `C;cmdline_url=`), zsh with its widget; a second sourcing adds no second
   set of marks; without the integration the variable is unset.
2. **Contracts and docs.** WEB-49's addition and WEB-53, conformance through the binary (an inner shell with the line in
   its startup file marking its prompts), and the Terminal section of [docs/WEB.md](../../docs/WEB.md) with the two
   lines, and [docs/FAQ.md](../../docs/FAQ.md).

## Tests and validation

- Go: as in phase 1, in `internal/terminal` (bash, and zsh where installed).
- Conformance: as in phase 2.
- `go test ./internal/terminal ./conformance/05-web`.

## Non-goals

- Adding the line to anyone's startup file, or offering to.
- fish and other shells (fish 4 marks by itself).
- Marks through `ssh`, `tmux`, or `screen`.

## Open questions

1. **How the line finds the files.** Proposed: the environment variable, which needs nothing on `PATH` and costs nothing
   per shell. The alternative, as VS Code's `code --locate-shell-integration-path zsh`, is a `rotari` command that
   prints the path, run by the line each time a shell starts.
2. **With `--terminal-shell-integration=false`.** Proposed: no files and no variable, so turning it off turns everything
   off. The alternative keeps the files and the variable, for someone who wants marks only where they add the line.

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace and is rebased onto `main` before integrating.
