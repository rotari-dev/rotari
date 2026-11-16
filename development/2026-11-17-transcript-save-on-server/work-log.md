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

## 2026-11-17 00:19 +09:00 — The page

- **Change:** `web_transcript.js`: the three Save buttons become a format choice (Markdown, HTML, Shell script, kept as
  `rotari-transcript-format`), **Download**, and **Save on the server…**, which opens a form under the toolbar with the
  shell's directory now (its preview's `cwd`, else where it started with a note saying why) and the download's name,
  the name selected; Save checks the path is absolute, uploads with `application/octet-stream`, asks `confirm()` on a
  409 and uploads again with `replace=1`, says when the file was kept, and once saved says where with **Show in the
  Explorer**, which posts `rotari-transcript-show` to the opener; Escape and Cancel close the form. The panel gives the
  transcript the shell's ID and starting directory, and takes `rotari-transcript-show` (an absolute path) only from a
  transcript page it opened, opening the panel and showing the file's folder in the Explorer.
- **Reason:** plan phase 2.
- **Plan impact:** none.
- **Validation:** `TestTerminalTranscript` (the format choice and its memory, Download in each format, the form's path
  from the preview, a taken name asked about and replaced, kept when refused, a relative path refused, Escape, the
  starting directory with its note when the preview fails, Show in the Explorer's message) and `TestTerminalPanel` (the
  shell's ID and directory in the transcript; the show message from its page, not another window, showing the folder)
  pass; `tsc` and prettier clean.
- **Remaining:** phase 3 (contracts and docs).

## 2026-11-17 00:19 +09:00 — Contracts and docs

- **Change:** WEB-60's paragraph and row describe the format choice, Download, Save on the server, replacing after
  asking, and Show in the Explorer; WEB-44's describe `replace=1`. `TestTerminalTranscriptInBrowser` saves the shell
  script by Download with the format chosen, then on the server, and Go reads the file from the shell's home.
  docs/WEB.md says how to save on the server.
- **Reason:** plan phase 3.
- **Plan impact:** none.
- **Validation:** `go test ./conformance -run 'TestContractStatus|TestConformanceLayout'` passes and `go vet
  ./conformance/05-web` is clean; the browser test runs only in CI; its script passes `node --check`.
- **Remaining:** CI's run of `TestTerminalTranscriptInBrowser`.
