# Work log: Changing files from the terminal's Explorer

## 2026-11-14 01:10 +09:00 — `32f2c52c` Make, rename, move, copy, and delete files from the terminal's Explorer

**Change:**

- `internal/fileops` (new; registered in `internal/archtest` as knowing nothing of HTTP):
  - `CheckName` refuses empty names, `.`, `..`, `/`, `\`, and control characters.
  - `Mkdir` and `CreateFile` (`O_EXCL`) make a folder or an empty file.
  - `Move` refuses a move into itself (`ErrBadName`) or to where it already is (`ErrAlreadyThere`). It renames
    without replacing: `renameat2` with `RENAME_NOREPLACE` on Linux (`rename_linux.go`), falling back to
    `Lstat` then `os.Rename` on `EINVAL`/`ENOSYS` and on other systems (`rename_other.go`). `EXDEV` becomes a
    refusal that says to use `mv`.
  - `CopyFile` refuses folders and anything but regular files and links. A link is copied as the link, and a file
    with its permission bits (`O_EXCL`); what a failed copy wrote is removed. With `unique`, names go through
    `CopyName` (`name copy.ext`, `name copy 2.ext`).
  - `Remove` removes a folder with everything in it and a link without its target. It refuses `/`, and a missing
    path is an error.
- `internal/webui/terminal_files.go`: `serveTerminalFileChange` serves `POST /_terminal/files/{mkdir,create,
  move,copy,delete}`, dispatched in `serveTerminal`, so it sits behind the terminal's checks. It needs a JSON body
  (415 otherwise) and refuses unknown fields. Paths go through `checkTerminalDir`. Errors map to 400 (bad name),
  409 (exists, already there), and 422 (anything else); success answers `{"path"}`.
- `internal/webui/assets/web_shell.js`:
  - `changeFiles` and `explorerChange` show the busy state in the footer, a refusal as a note, then list again
    and select what was made.
  - **New File** and **New Folder** go in the head and in the menu of a row or of the tree's empty part. A row's
    menu also has Cut, Copy (disabled for folders), Paste, Rename..., and Delete....
  - In the tree, F2, Delete, and Ctrl/Cmd+X/C/V do the same; Ctrl+C on a folder says to use `cp -r`.
  - Names are asked with `prompt`. Deletions are confirmed with `confirm`, a folder's naming its direct entries
    from one listing.
  - Dragging a row onto a folder, a file's row, or the empty tree moves it after `confirm`. `takesDrops` with
    `movableInto` refuses its own folder and itself or below. A closed folder held over for 700 ms opens.
  - A cut row is dimmed.
- `web_shell.css`: the drop mark and the cut row.
- Tests:
  - `internal/fileops` tests. `TestMoveDoesNotCrossFileSystems` ran here against `/dev/shm`, and it skips where
    there is no second file system.
  - `TestTerminalFilesChange` (routes, refusals, 415, 405, unknown op), and the guard and absence tests extended.
  - `TestTerminalPanel`: a fake for the change routes and a section for each menu item and key, the questions
    asked, a refused change's note, a drag onto a folder, and the 700 ms open.
  - Conformance: `TestTerminalExplorerChangesFiles` (WEB-43).
- Contract WEB-43 note and row; `docs/WEB.md`, `docs/SECURITY.md`, and `docs/ARCHITECTURE.md` (the package and the
  routes).

**Reason:** The user asked for the file operations a terminal application's file pane usually has, and agreed on
2026-11-14 to: permanent deletion after confirming, confirming moves by dragging, and copying files only, since a
folder's size takes as long to find as the copy.

**Plan impact:** All four phases are done. The delete question's count is of direct entries only, as agreed, not
the folder's whole size.

**Validation:**

- `go test ./internal/fileops/ ./internal/archtest/`: pass.
- `go test ./internal/webui/ -run 'TestTerminalFiles|TestTerminalShellRoutesAreGuarded|TestTerminalIsAbsentWithoutTheOption'`: pass.
- `TestTerminalPanel`: pass. A deliberately wrong last assertion of the new section failed as expected.
- `TestTerminalExplorerChangesFiles`: pass. Its permission check was loosened afterwards to allow any umask, and
  the full run below included that.
- Full run: `go vet ./...`, `tsc`, `go test -timeout 15m ./...`, and `go test -race -timeout 15m ./...` (under
  `taskset -c 0-11`, go1.25.8, Node v24): all passed.
- Prettier 3.9.8: only the two `confirmEndingShells` differences that are on `main` too.
- Not run: a real browser.

**Remaining:** Folder copies, undo, and a trash are out of this plan. Downloading and uploading follow in their own
plan.
