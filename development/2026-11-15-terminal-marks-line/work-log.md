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
