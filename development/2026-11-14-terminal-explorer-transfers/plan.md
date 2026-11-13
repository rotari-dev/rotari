# Plan: Downloading and uploading through the terminal's Explorer

Created: 2026-11-14
Status: Implemented; see [work-log.md](work-log.md). Dragging out to the desktop and dropping from it are checked only with synthetic events; there is no browser here.

## Goal

From the terminal panel's Explorer ([its plan](../2026-11-13-terminal-explorer/plan.md), [its changes to
files](../2026-11-14-terminal-explorer-edits/plan.md)), download a file, or a folder as a zip, and upload files,
both from the menu and by dragging between the tree and the desktop.

## Decisions

1. **Download.** A row's menu has **Download**: a file as it is, a folder as `name.zip`. The browser's own download
   shows progress and can cancel it. Dragging a row out of the browser downloads it too where the browser supports
   it (`DownloadURL`, Chrome and Edge); elsewhere nothing happens. A name holding `:` cannot go in `DownloadURL` and
   is left out of it.
2. **The zip is streamed** (agreed 2026-11-14): written while the folder is read, so no size is needed first. Its
   entries sit under the folder's name, folders included so empty ones survive. Symbolic links are not followed
   or stored; other special files are left out. A file that cannot be read, or any other failure partway, aborts
   the connection, so the browser reports a failed download rather than keeping a short zip; nothing is skipped
   silently.
3. **Upload.** **Upload...** in the menu of a folder, a file (into its folder), and the tree's empty part opens the
   browser's file chooser, for one or more files. Files dropped from the desktop onto a folder, a file's row, or
   the tree's empty part upload there, and a closed folder held over opens after 700 ms, as when moving a row.
4. **Upload never replaces** (agreed 2026-11-14). A name taken is refused for that file, with the reason. Each file
   is written under a temporary name in the same folder and renamed into place without replacing, so a broken
   upload leaves no partial file under the real name; the temporary file is removed on failure.
5. **Files only for upload** (agreed 2026-11-14). A dropped folder is refused with a note; the files dropped with
   it still upload.
6. **Routes.** `GET /_terminal/files/download?path=/ABS` and `POST /_terminal/files/upload?dir=/ABS&name=NAME`
   with the file as an `application/octet-stream` body (415 otherwise, which a form from another site cannot
   send), both behind the terminal's checks. `internal/fileops` gains `WriteNew` and `WriteZip`.
7. **Feedback.** Uploads go one at a time; the footer says which (`Uploading a.csv, 2 of 3...`), and a note at the
   end says how many uploaded and why any did not.

## Phases

1. `internal/fileops`: `WriteNew` and `WriteZip`, with tests.
2. The routes in `internal/webui`, with tests.
3. The Explorer: menu items, the file chooser, drops from the desktop, dragging out, and JSDOM tests.
4. WEB-44 and its conformance test, [docs/WEB.md](../../docs/WEB.md), [docs/SECURITY.md](../../docs/SECURITY.md),
   and [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md).

## Tests and validation

- `WriteNew`: written whole; a taken name refused; a failed read leaves neither the file nor the temporary one; a
  bad name refused. `WriteZip`: the entries, an empty folder, a link left out, a non-UTF-8 name kept as bytes, and
  an unreadable file failing.
- Routes: a file and a folder downloaded with their names; refusals; the abort when a file cannot be read; upload
  success, conflict, bad name, wrong content type; absent without the terminal.
- JSDOM: the menu items, the download link, the chooser's files uploaded in order with the footer, a refused file
  in the note, a dropped folder refused, `DownloadURL` set when dragging.
- Conformance under WEB-44 through the binary.
