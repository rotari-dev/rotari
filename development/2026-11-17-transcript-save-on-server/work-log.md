# Work log: Saving a transcript on the server, where the shell is

## 2026-11-17 00:13 +09:00 — Replacing a file on upload

- **Change:** `fileops.Replace` (beside `WriteNew`, sharing `writeFile`): writes beside the target and renames over a
  regular file, keeping its permissions, or writes a new file without replacing; anything else under the name, a folder
  or a symbolic link among them, is `ErrNotRegular`, and a failure partway leaves what was there. The upload route takes
  `replace=1` (or `0`, the default) for it; another value is 400; refusing a non-regular file is 422.
- **Reason:** plan phase 1, after the user said "ok" to the UI.
- **Plan impact:** none.
- **Validation:** `TestReplace` (a file replaced with its mode kept, a new file, a folder and a symbolic link refused
  with the linked file untouched, a broken write leaving the old content, nothing left beside) and the upload test's
  replace cases (replaced, a folder refused, `replace=yes` refused) pass; `go test ./internal/fileops` passes.
- **Remaining:** phases 2 (the page) and 3 (contracts and docs).
