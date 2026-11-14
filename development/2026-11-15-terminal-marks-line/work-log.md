# Work log: Marks from shells started inside the Web terminal's shells

## 2026-11-15 08:00 +09:00 — Files that outlive the server

- **Change:** `WriteIntegration` now returns the directory, whether it lasts, and a note. It first writes the files
  under the user's cache (`os.UserCacheDir()`), in `rotari/shell-integration/HASH`, HASH the first 16 hex digits of a
  SHA-256 over each file's place and content (`integrationHash`, over `integrationLayout`, the list of embedded files
  and their places): a new directory is filled beside its place and renamed into it, accepting one another server put
  there first; `rotari`, `shell-integration`, the directory, and its `zsh` directory must each be this user's, not a
  symbolic link, and not writable by others (`ownedPrivately`); each file is kept when it is this user's regular file,
  not writable by others, and equal to the server's, and otherwise replaced by renaming a new 0600 file over it. When
  any of that fails, it writes a private temporary directory as before, and the note says why. The Keeper logs the note
  and removes only a temporary directory on Close.
- **Reason:** plan phase 1, after the user accepted the rewritten plan with the open questions answered as proposed
  (`ZDOTDIR` only for command lines starting with `zsh`; the user's cache directory).
- **Plan impact:** none.
- **Validation:** `TestLastingIntegration` (the directory by hash, 0700 with 0600 files; the same directory again; a
  changed file put back; a file others could write replaced; a directory others can write refused; a linked
  `shell-integration` refused), `TestKeeperKeepsTheIntegrationInTheCache` (in the cache and kept after Close; with a
  `rotari` directory others can write, a temporary one, logged once and removed on Close) passed; the integration tests
  now write to a test's own cache directory.
- **Remaining:** phases 2 (marks inside) and 3 (contracts and docs).

## 2026-11-15 09:00 +09:00 — Marks inside

- **Change:** the hooks moved into `marks.bash` and `marks.zsh` (embedded and written with the rest), each loading once
  per shell by an unexported variable, `__rotari_marks`. `rotari.bash` reads the login files and sources `marks.bash`;
  zsh's `.zshrc` sources the user's `.zshrc` and `marks.zsh`, then puts `ZDOTDIR` back, which `.zlogin` used to do (it
  is gone, so a login zsh reads the user's `.zlogin` itself); `.zshenv` puts it back for a zsh that is not
  interactive, and takes a `ROTARI_USER_ZDOTDIR` naming rotari's own directory for none. `shellCommand` gives every
  shell `ROTARI_SHELL_INTEGRATION`, and a zsh started by a server that itself runs inside a panel's shell takes the
  user's `ZDOTDIR` from `ROTARI_USER_ZDOTDIR`. In `marks.bash`, the loader comes first in `PROMPT_COMMAND`: it keeps
  `$?` in `__rotari_status` (so `__rotari_keep` now only gives it back) and, in a bash that has not loaded the marks,
  sources `marks.bash`; `PROMPT_COMMAND` is rebuilt from what the user's files left, rotari's pieces taken out
  wherever they are, and exported when it is a string; and `ZDOTDIR` is exported as rotari's `zsh` directory for the
  session, the user's in `ROTARI_USER_ZDOTDIR`, unless it already is rotari's. The old bash's `DEBUG` trap now knows a
  prompt by the loader's first command. In `marks.zsh`, `__rotari_zsh_inside` (a `preexec` hook) exports `ZDOTDIR` as
  rotari's only for a command line whose first word, after assignments, `exec`, `command`, `builtin`, `nocorrect`,
  `noglob`, or `-`, is `zsh` or ends in `/zsh`, and `__rotari_keep` puts the user's back; and `PROMPT_COMMAND` is
  exported as the loader.
- **Reason:** plan phase 2.
- **Plan impact:** none.
- **Validation:** `TestInnerShellsMark` (zsh inside bash, zsh inside zsh, bash inside zsh, bash inside bash, with a
  `.bashrc` that adds to `PROMPT_COMMAND`: the inner shell's prompt marks `C`, `D;1`, its own `A`, and `B`; it reads
  the user's startup file once; an inner bash's `PROMPT_COMMAND` holds `__rotari_prompt` once; an inner zsh's command
  line sees `ZDOTDIR` and `ROTARI_USER_ZDOTDIR` unset) passed on its first run. Not exporting `PROMPT_COMMAND` fails
  bash inside bash, and not exporting `ZDOTDIR` fails zsh inside zsh. `TestShellCommandWithIntegration` has the
  variable and a server started inside a panel's shell. The integration tests pass three times in a row; see the
  commit for the package runs. Not tried with the user's own setup, which would write rotari's files to their cache
  and the tests' lines to their history.
- **Remaining:** phase 3 (contracts and docs).
