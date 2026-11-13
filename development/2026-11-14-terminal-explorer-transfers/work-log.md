# Work log: Downloading and uploading through the terminal's Explorer

## 2026-11-14 01:31 +09:00 — `c20e153e` Download and upload through the terminal's Explorer

**Change:**

- `internal/fileops/transfer.go` (new):
  - `WriteNew` checks the name and refuses a taken one up front. It writes to `.rotari-upload-<random>` in the same
    folder (`O_EXCL`, mode 0666 before the umask, unlike `os.CreateTemp`'s 0600), renames it into place with
    `renameNoReplace`, and removes the temporary file on any failure.
  - `WriteZip` walks the folder with `filepath.WalkDir`. It writes entries under the folder's name, with folders as
    entries (stored), files deflated, and non-UTF-8 names marked `NonUTF8`. It skips links and special files,
    stops on the first error or when the context ends, and closes the archive only on success.
  - `ZipName` names the download (`root.zip` for `/`).
- `internal/webui/terminal_files.go`:
  - `serveTerminalDownload` (GET/HEAD) serves a regular file with `http.ServeContent`, so ranges work, as
    `application/octet-stream`. It streams a folder as `application/zip`, with `attachmentDisposition` from the
    Table viewer for both names. A zip error after the status logs and panics with `http.ErrAbortHandler`, as the
    Table viewer's export does.
  - `serveTerminalUpload` (POST) needs `application/octet-stream` (415) and writes with `WriteNew`.
  - `writeFileChange` now holds the error-to-status mapping shared with the other changes. Both routes are under
    `/_terminal/files/`, so they sit behind the terminal's checks.
- `internal/webui/assets/web_shell.js`:
  - A row's menu has **Download** and **Upload...**, and so does the empty tree's menu (**Upload...**).
    `downloadExplorerItem` clicks a temporary `<a download>`, and `chooseUploads` a temporary file input.
  - `uploadFiles` posts each file in turn with the footer naming it, then shows one note for what was uploaded and
    why the rest was not.
  - `takesDrops` also takes files from outside the page (`carriesFiles`, the `Files` type when no row is being
    dragged), with the same 700 ms open. `uploadDropped` leaves out folders (`webkitGetAsEntry().isDirectory`)
    and says so.
  - `dragstart` sets `DownloadURL` unless the saved name holds `:`.
  - The panel's body takes files dropped beside the tree, so the browser does not open them in place of the
    page, and says where to drop.
- Tests:
  - `TestWriteNew`, `TestWriteZip`, and `TestWriteZipFailsRatherThanSkipping`.
  - `TestTerminalFilesDownloadAndUpload`: names, ranges, zip entries, refusals, the abort, and upload refusals.
    The guard and absence tests were extended.
  - `TestTerminalPanel`: the download link, `DownloadURL`, chosen files with one refused, a drop with a folder
    left out, and a drop on a pane caught.
  - `TestTerminalExplorerDownloadsAndUploads` (WEB-44).
- Contract WEB-44 note and row, `docs/WEB.md`, `docs/SECURITY.md`, and `docs/ARCHITECTURE.md`.

**Reason:** The user asked for download and upload from the menu and by drag and drop, and for folders as zips.
They agreed on 2026-11-14 that uploads never replace, folders are not uploaded, and the zip is streamed with links
left out and a failure aborting the download.

**Plan impact:** All phases are done. Catching files dropped on the panes was added beyond the plan, since the
Explorer now invites dragging files in, and a miss would have left the page.

**Validation:**

- `go test ./internal/fileops/`: pass.
- `TestTerminalFilesDownloadAndUpload`: pass, once its expected conflict text matched `fs.ErrExist`'s "file already
  exists".
- `TestTerminalPanel`: pass. A deliberately wrong last assertion of the new section failed as expected.
- `TestTerminalExplorerDownloadsAndUploads`: pass.
- Full run: `go vet ./...`, `tsc`, `go test -timeout 15m ./...`, and `go test -race -timeout 15m ./...` (under
  `taskset -c 0-11`, go1.25.8, Node v24): all passed.
- Prettier 3.9.8: the CSS passes; the JS shows only the two `confirmEndingShells` differences that are on `main` too.
- Not run: a real browser. Dragging out (`DownloadURL`) and dropping from the desktop were checked only with
  synthetic events.

**Remaining:** Uploading folders, and progress for one large upload (`fetch` reports none), if wanted.
