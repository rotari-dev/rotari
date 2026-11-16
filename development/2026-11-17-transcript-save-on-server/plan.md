# Plan: Saving a transcript on the server, where the shell is

Created: 2026-11-17
Status: Accepted by the user on 2026-11-17 ("ok" to the UI proposed in conversation). See [work-log.md](work-log.md).

## Goal

The [transcript page](../2026-11-16-terminal-transcript/plan.md) (WEB-60) saves only by downloading. The user wants to
write the file where the shell is too. The page chooses a format once, then **Download** or **Save on the server…**,
which offers the shell's current directory and a file name, asks before replacing a file, and can show the saved file
in the opening tab's Explorer.

## Existing implementation

- **Upload** (WEB-44): `POST /_terminal/files/upload?dir=&name=`, `application/octet-stream`, writes a new file with
  `fileops.WriteNew` and never replaces one (409 when it exists).
- **The shell's directory now**: `GET /_terminal/shells/ID/preview` answers `cwd` (or `cwd_error`).
- **The transcript page** gets the transcript from its opener by `postMessage`; it is same-origin, so its own requests
  pass the terminal's checks as the panel's do.

## Decisions

1. **Format, then destination.** The three Save buttons become a format choice (Markdown, HTML, Shell script,
   remembered by the browser) and two buttons: **Download** (as now) and **Save on the server…**.
2. **The dialog.** A path field filled with the shell's current directory (the preview's `cwd`, asked when the dialog
   opens; the shell's starting directory with a note when it cannot be told) joined with the file name the download
   would have. **Save** writes there; the path must be absolute. Saved, the page says where, with **Show in the
   Explorer**, which asks the opening tab to show the file's folder in its Explorer.
3. **Replacing.** A file of that name is replaced only after `confirm()` asks; the upload route takes `replace=1`, which
   replaces a regular file atomically (written beside it, then renamed over it) and refuses a directory, a symbolic
   link, or anything else that is not a regular file. Without `replace=1` the route keeps WEB-44's behavior.
4. **The transcript carries the shell's ID**, for the preview, and the opening tab takes `rotari-transcript-show`
   (a path) only from a transcript page it opened.
5. **Contracts.** WEB-60 and WEB-44 change; no new ID.

## Phases

1. **Server.** `fileops.Replace` and `replace=1` on the upload route. Go tests: replacing a file, refusing a directory
   and a symbolic link, a new file with `replace=1`, and without it as before.
2. **Page.** The format choice, the two buttons, the dialog, the confirmation, Show in the Explorer, and the panel's
   side (the ID, the show message). JSDOM tests in `TestTerminalTranscript` and `TestTerminalPanel`.
3. **Contracts and docs.** WEB-60, WEB-44, docs/WEB.md, and the browser test saving on the server.

## Non-goals

- Choosing the folder by browsing; the Explorer already does that, and the path is editable.
- Saving anywhere but this server.
