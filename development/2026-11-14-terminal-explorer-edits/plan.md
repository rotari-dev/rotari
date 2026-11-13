# Plan: Changing files from the terminal's Explorer

Created: 2026-11-14
Status: Implemented; see [work-log.md](work-log.md). Not seen in a real browser (none here); `TestTerminalPanel` covers the panel.

## Goal

The terminal panel's Explorer ([its plan](../2026-11-13-terminal-explorer/plan.md)) can do what a terminal
application's file pane usually does: make a folder or an empty file, rename, delete, move by dragging or by Cut and
Paste, and copy a file. It stays short of an IDE: no editor, no search, no large copies.

## Decisions

1. **Operations.** **New File** and **New Folder** (in the Explorer's head and the menu), **Rename** (menu, F2),
   **Delete** (menu, Delete key), **Cut**, **Copy**, and **Paste** (menu, Ctrl+X/C/V in the tree), and moving by
   dragging a row onto a folder. A new item goes into the selected folder, the selected file's folder, or the root.
2. **Names.** A new name is refused when empty, `.` or `..`, or when it holds `/`, `\`, NUL, or another control
   character. `\` is allowed in Linux names but refused here, as AGENTS.md asks where separators are not allowed,
   and control characters could not be sent to the shell anyway.
3. **Nothing is replaced.** Making, renaming, moving, and copying refuse a destination that exists, with the
   reason. A move uses `renameat2` with `RENAME_NOREPLACE` on Linux; where that is not supported (another system,
   or a file system that answers `EINVAL`), it checks with `Lstat` and then renames, which can race but still
   never replaces knowingly. A move into itself or below itself is refused; a move to where it already is does
   nothing and says so.
4. **No copying between file systems.** A move that `rename` cannot do (`EXDEV`) is refused, saying to use `mv` in
   the shell, rather than copying and deleting, which could leave half of it behind.
5. **Copy is for files only** (agreed 2026-11-14). A folder is refused, saying to use `cp -r` in the shell: copying
   one would need its size, which takes as long to find as the copy, or a background job with progress, a later plan
   if wanted. A symbolic link is copied as a link. A file is copied with its permission bits; a failure removes what
   was written. Pasted into its own folder, a copy is named `name copy.ext`, then `name copy 2.ext`, and so on;
   pasted elsewhere over an existing name, it is refused.
6. **Delete is permanent, after asking** (agreed 2026-11-14). The question names the item, and for a folder how many
   entries it holds directly (one listing; its depth is not walked) and that everything in it goes. A symbolic link
   is removed, not what it points to. `/` is refused. A large folder takes as long as it takes; the Explorer says
   it is deleting until the answer.
7. **Moving by dragging asks first** (agreed 2026-11-14), naming what moves and where. Dropping a row on a folder
   moves it into that folder, on a file into the file's folder, and on the tree's empty part into the root. A drop on
   a pane still types the path. Pausing over a closed folder while dragging opens it after 700 ms.
8. **Where the rules live.** The file system rules (names, no replacing, the copy's name, removing) go in a new
   package, `internal/fileops`, which knows nothing of HTTP. `internal/webui` serves them under `/_terminal/files/`
   (`mkdir`, `create`, `move`, `copy`, `delete`) as `POST` with a JSON body, behind the terminal's checks, so a
   server without the terminal has none of them. Each answers the path it made, or an error with the reason.
9. **Dialogs.** Names are asked with the browser's `prompt` and confirmations with `confirm`, as the panel already
   does for renaming and ending shells.
10. **Not in this plan:** copying folders, undo, a trash, and editing file contents.

## Phases

1. `internal/fileops` and its tests.
2. The routes in `internal/webui` with tests.
3. The Explorer: head buttons, menu items, keys, drag and drop with the pause, the deleting state, and the JSDOM
   tests.
4. A new contract, WEB-43, its conformance test, [docs/WEB.md](../../docs/WEB.md),
   [docs/SECURITY.md](../../docs/SECURITY.md), and [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md).

## Tests and validation

- `internal/fileops`: each operation, each refused name, an existing destination for every operation, a move into
  itself, a move to the same place, `EXDEV` (when a second file system is at hand; skipped otherwise), a folder
  copy refused, a link copied as a link, copy names in the same folder, a failed copy leaving nothing, removing a
  link leaves its target, and `/` refused.
- Routes: success and refusal codes, JSON content type required, and absent without the terminal.
- JSDOM: each menu item and key, the questions asked, a drag onto a folder after confirming, the pause opening a
  folder, and that a refusal is shown.
- Conformance under WEB-43 through the binary.
