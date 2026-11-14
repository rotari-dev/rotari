# Plan: Marks from shells started inside the Web terminal's shells

Created: 2026-11-15
Status: Rewritten on 2026-11-15 after the user asked to avoid a line in one's own startup file, and accepted with its
side effects and the open questions answered as proposed; phases 1 and 2 are done and phase 3 (contracts and docs) is next. See
[work-log.md](work-log.md).

## Goal

With `--allow-terminal` and `--terminal-shell-integration`, a bash or zsh started by hand inside a panel's shell (`zsh`
typed at a bash prompt, `bash` inside zsh) marks its prompts and commands as the shells rotari starts do, with nothing to
set up, so that blocks, Commands, Copy output, click to move, and the right-click Cut work there too. A line for one's
own startup file stays in the docs for the cases the automatic way misses.

This is the second of two plans for inner shells proposed on 2026-11-15, after [following
them](../2026-11-15-terminal-nested-shells/plan.md), which made the Explorer and the prompt checks follow them.

## Existing implementation

- **Shell integration** (WEB-49, [Blocks plan](../2026-11-14-terminal-blocks/plan.md) decision 1). The server writes
  `rotari.bash` and zsh's `.zshenv`, `.zprofile`, `.zshrc`, and `.zlogin` to a private temporary directory, removed when
  it stops, and starts bash with `--init-file` and zsh with `ZDOTDIR` there. Each reads the user's files and then adds
  the hooks. zsh puts `ZDOTDIR` back in `.zlogin`, which only a login zsh reads.
- **An inner shell** reads only the user's files: `zsh` (interactive, not a login shell) `.zshenv` and `.zshrc` from
  `ZDOTDIR` or `$HOME`; `bash` `~/.bashrc`. It inherits its parent's environment, and bash runs an inherited
  `PROMPT_COMMAND` unless `~/.bashrc` replaces it. bash ignores `ZDOTDIR`; zsh ignores `PROMPT_COMMAND`.
- **The panel** reads marks from the stream whatever process prints them.

## Decisions

1. **Files that outlive the server.** Something started in a panel's shell can outlive the server (a `tmux` server, an
   editor) and keep the environment below; files that vanished would leave a later zsh there reading none of the
   user's files. So the files go to a lasting directory, `rotari/shell-integration/<hash of their content>` under the
   user's cache directory (`$XDG_CACHE_HOME` or `~/.cache`; `~/Library/Caches` on macOS), made 0700, its files 0600 and
   rewritten when they differ from the server's own, and never removed; a new rotari with other files uses another
   directory beside it. The server refuses a directory or file it does not own or that others can write, says why in
   its log, and falls back to a private temporary directory as today, where inner shells get no marks.
2. **The hooks on their own.** The hooks move into `marks.bash` and `marks.zsh`, which only add them, and load once per
   shell: a shell variable (not exported) makes a second load do nothing. rotari's startup files source them after the
   user's files. `ROTARI_SHELL_INTEGRATION`, the directory, is in every panel shell's environment.
3. **zsh started inside.** rotari's zsh files put `ZDOTDIR` back at the end of `.zshrc` (so that a zsh that is not a
   login shell does too, and a login zsh then reads the user's own `.zlogin`), and rotari's `.zlogin` goes.
   - In **bash**, which ignores `ZDOTDIR`, rotari's files export it for the whole session as rotari's `zsh` directory,
     with `ROTARI_USER_ZDOTDIR` when the user had one, so that a zsh started anywhere below reads rotari's files, and
     through them the user's.
   - In **zsh**, which reads `ZDOTDIR` itself (and a command line expanding `$ZDOTDIR` should see the user's), rotari's
     `preexec` exports it only for a command line whose first word is `zsh` or a path ending in `/zsh` (after `exec`,
     `command`, or assignments), and `precmd` puts the user's back.
4. **bash started inside.** rotari's shells export `PROMPT_COMMAND` holding a loader,
   `[ -r "$ROTARI_SHELL_INTEGRATION/marks.bash" ] && . "$ROTARI_SHELL_INTEGRATION/marks.bash"`, which zsh ignores and an
   inner bash runs at its first prompt. In rotari's bash the loader comes first in its own `PROMPT_COMMAND` and does
   nothing after the first load. Loaded, `marks.bash` rebuilds `PROMPT_COMMAND` (string or array): its own pieces (the
   loader and its hooks) taken out of whatever `~/.bashrc` left, `__rotari_keep` first and `__rotari_prompt` last, so
   that an inherited copy and a `.bashrc` that appends to it do not run them twice. A `.bashrc` that replaces
   `PROMPT_COMMAND` drops the loader, and that bash gets no marks.
5. **The line**, for what the automatic way misses (a `.bashrc` that replaces `PROMPT_COMMAND`, a shell started with a
   cleaned environment), stays in docs/WEB.md:
   `[ -r "$ROTARI_SHELL_INTEGRATION/marks.bash" ] && . "$ROTARI_SHELL_INTEGRATION/marks.bash"` in `~/.bashrc`, and
   `[[ -r $ROTARI_SHELL_INTEGRATION/marks.zsh ]] && . "$ROTARI_SHELL_INTEGRATION/marks.zsh"` in `~/.zshrc`.
6. **Side effects, accepted and documented.** `ZDOTDIR`, `ROTARI_USER_ZDOTDIR`, `PROMPT_COMMAND`, and
   `ROTARI_SHELL_INTEGRATION` are in the environment of what bash runs (and, for `PROMPT_COMMAND`, what zsh runs), so a
   program started from a panel's bash that reads `$ZDOTDIR` sees rotari's directory; a zsh in a `tmux` started from a
   panel shell keeps reading the user's files through rotari's, whose marks tmux does not pass on. Not through `ssh`.
   With `--terminal-shell-integration=false`, none of this.
7. **No route or option.** WEB-49 says where the files are and what is exported; a new WEB-53 promises the inner shells'
   marks.

## Phases

1. **Lasting files.** The cache directory by content hash, its checks, the fallback, and the Keeper no longer removing
   it. Go tests: written once and reused, rewritten when changed, a directory others can write refused with the
   fallback.
2. **Marks inside.** `marks.bash` and `marks.zsh` with the once-per-shell guard; rotari's files sourcing them; zsh's
   `ZDOTDIR` put back in `.zshrc`; the exports of decisions 3 and 4; `marks.bash` rebuilding `PROMPT_COMMAND`. Tests with
   real shells: zsh inside bash, zsh inside zsh, bash inside zsh, and bash inside bash marking (`D`, `A`, `B`, `C`), each
   reading the user's files once; `$ZDOTDIR` in an ordinary command line of a zsh being the user's; a `.bashrc` that
   appends to `PROMPT_COMMAND` running the hooks once; the shells rotari starts marking as before.
3. **Contracts and docs.** WEB-49's addition and WEB-53, conformance through the binary (a zsh started inside a bash
   panel shell marking its prompts), docs/WEB.md (inner shells marking, the side effects, the line), and docs/FAQ.md.

## Tests and validation

- Go: as in phases 1 and 2, in `internal/terminal` (bash, and zsh where installed).
- Conformance: as in phase 3.
- `go test ./internal/terminal ./internal/webui ./conformance/05-web`.

## Non-goals

- fish and other shells (fish 4 marks by itself).
- Marks through `ssh`, `tmux`, or `screen`, or from a shell started with a cleaned environment (`env -i`, `sudo`).
- Removing old directories of files; each is a few kilobytes.

## Settled questions

Both were accepted as proposed on 2026-11-15.


1. **zsh inside zsh.** Proposed: `ZDOTDIR` exported only for command lines starting with `zsh` (decision 3). The
   alternative exports it for every command, which also reaches a zsh started by a script or a program, at the cost of
   `$ZDOTDIR` typed in a command line meaning rotari's directory.
2. **Where the files live.** Proposed: the user's cache directory. The alternative is rotari's own state directory
   under the basedir, which a basedir on NFS shares between hosts.

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace and is rebased onto `main` before integrating.
