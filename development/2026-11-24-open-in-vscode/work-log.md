# Work log: Open in VS Code

Plan: [plan.md](plan.md)

## 2026-11-24 17:24:54 +0900 - `633b5454` - Rename the built-in launcher

- **Change:** `internal/launcher` names the built-in that starts VS Code's Web
  version **VS Code (Web)**, in the registry and in the template
  `rotari config --launchers` writes. Expectations in `internal/launcher`,
  `internal/webui`, and `conformance/05-web`; contract WEB-15; CONFIGURATION,
  WEB, SECURITY, and FAQ.
- **Reason:** Phase 2. The old name beside **Open in VS Code** would not say
  which of the two opens the installed VS Code.
- **Plan impact:** Phase 2 is implemented. No alias for the old name is kept
  (decision 2); CONFIGURATION says to rename an entry that replaced it.
- **Validation:** `go vet ./...`, the Web UI type check, and `go test` on every
  package whose name does not contain `pairedits` passed, with Go 1.25.8 and
  Node 24. With the machine's default Node v12, the browser and JSDOM tests fail
  on syntax it does not know. Not run: `pairedits`, `go test -race`, and
  pre-commit (not installed). `scripts/check.sh` stops here because the system
  `npm` does not start under Node 24, so its steps were run one by one.
- **Remaining:** The release notes of the next version should mention the rename.

## 2026-11-24 17:49:23 +0900 - `54f4f1f4` - Open without a line

- **Change:** `web_vscode.js` (the dialog, the kept choice, and the URL), part of
  the page frame's script; `internal/webui/vscode.go` (the element that offers the
  action, and `GET /api/vscode-target`); Explorer rows, the path menu, Artifacts,
  **Files**, and Settings. Contract WEB-64 with
  `TestWebOpensArtifactsInVSCode`, unit and JSDOM tests, WEB, SECURITY, and
  ARCHITECTURE.
- **Reason:** Phase 3.
- **Plan impact:** The planned `GET /api/editor` became an element of live pages. A
  remote file gets `:1` and a directory `?windowId=_blank`, after the user saw VS
  Code close a window's folder for a file without a line.
- **Validation:** `go vet ./...`, the Web UI type check, Prettier 3.9.8 on the
  changed assets it accepts, and `go test` on every package but `pairedits`,
  `pairjobcontrol`, and `pairruns` passed, with Go 1.25.8 and Node 24. Not run:
  those three, `go test -race`, and pre-commit (not installed). The feature has not
  been tried in a real browser; the URLs were, by hand (plan, phase 1).
- **Remaining:** The unchecked cases of phase 1, on the finished feature. Two
  processes of one session edited this change for a while; the older one was
  stopped, and all of it was read and tested after that.

## 2026-11-24 18:06:23 +0900 - `f5f9fd38` - Lines in shell output, and shells' directories

- **Change:** `terminal_links.go` answers a file's line and column, and takes a name
  without a slash that has one; `web_shell.js` finds `PATH:LINE` and Python
  tracebacks, opens them from a path's menu, and opens a shell's directory from the
  bar, All shells cards, and a pane's menu. WEB-46, WEB-51, WEB-64, WEB, and
  ARCHITECTURE; unit, JSDOM, and conformance tests.
- **Reason:** Phase 4, and the user's request for the terminal's own menus.
- **Plan impact:** Phase 4 is implemented; decision 12 was added. The user had tried
  phase 3 in a browser and confirmed it before this.
- **Validation:** `go vet ./...`, the Web UI type check, and `go test` on every
  package but `pairedits`, `pairjobcontrol`, and `pairruns` passed, with Go 1.25.8
  and Node 24. Not run: those three, `go test -race`, and pre-commit (not installed).
  Not tried in a real browser.
- **Remaining:** Phase 5, and the unchecked cases of phase 1.

## 2026-11-24 18:43:40 +0900 - `6d28cc8f` - Lines in job logs

- **Change:** `log_links.go` holds how a printed path is read, now shared with shells'
  output, and `POST /api/log-links`; `web_app_logs.js` links the lines in view of the
  log dialog. Contract WEB-65 with `TestWebJobLogLinks` (a plain, a failed, a rerun,
  and a carried job, array tasks, read-only, and with the terminal), unit and JSDOM
  tests, WEB, SECURITY, and ARCHITECTURE.
- **Reason:** Phase 5.
- **Plan impact:** All phases are implemented. Only files are links in a log.
- **Validation:** `go vet ./...`, the Web UI type check, and `go test` on every
  package but `pairedits`, `pairjobcontrol`, and `pairruns` passed, with Go 1.25.8
  and Node 24. Not run: those three, `go test -race`, and pre-commit (not installed).
  Not tried in a real browser.
- **Remaining:** The unchecked cases of phase 1, the links in a browser, and the
  release notes' word on the renamed launcher.

## 2026-11-24 18:44:16 +0900 - `d13ba8f1` - What the dialog takes as an SSH host

- **Change:** The dialog's hint and WEB say that `USER@HOST` is taken and that VS Code
  asks for a password; a test refuses `USER:PASSWORD@HOST`, as the check already did.
- **Reason:** The user asked whether a user name may be given, and about passwords.
- **Plan impact:** None.
- **Validation:** The VS Code tests of `internal/webui` and `conformance/05-web`, and
  Prettier, passed. The full suite was not run again for this change. Opening with
  `USER@HOST` has not been tried.
- **Remaining:** None.
