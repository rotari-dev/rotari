# Work log: Choose the editor that files open in

Plan: [plan.md](plan.md)

## 2026-11-25 14:03:23 +0900 - `a8f338eb` - Both phases

- **Change:** `web_editor.js` (was `web_vscode.js`) lists five editors and builds each
  one's URL; Settings and the dialog choose one; labels name it. `editor.go` (was
  `vscode.go`), `GET /api/editor-target`, and `#editor-connection` are named for an
  editor. WEB-42, WEB-46, WEB-51, WEB-64, WEB-65, WEB, SECURITY, and ARCHITECTURE;
  unit, JSDOM, and conformance tests.
- **Reason:** The plan, whose two phases touch the same lines and went in as one change.
- **Plan impact:** Both phases are implemented. Settings has the list itself, as the
  user asked, and the storage keys are new (decision 4).
- **Validation:** `go vet ./...`, the Web UI type check, Prettier on the changed
  assets it accepts, and `go test` on every package but `pairedits`,
  `pairjobcontrol`, and `pairruns` passed, with Go 1.25.8 and Node 24; WEB.md's word
  on Zed was changed after. Not run: those three, `go test -race`, and pre-commit.
- **Remaining:** A file opened in Zed, at its line; the other editors, when someone has
  them; and the whole in a browser.

## 2026-11-25 14:32:40 +0900 - `0bfe8e79` - Without VS Code Insiders

- **Change:** VS Code Insiders is out of the list, its tests, WEB-64, WEB, and SECURITY;
  a browser that kept it is asked again, as for any editor not known.
- **Reason:** The user found it too rarely used for a place in the list.
- **Plan impact:** Decision 1 lists four editors.
- **Validation:** The editor and log link tests of `internal/webui`, `conformance`, and
  the editor tests of `conformance/05-web` passed. The full suite was not run again.
- **Remaining:** As above.

## 2026-11-25 16:10:46 +0900 - `c0e4e4ef` - None in place of the check box

- **Change:** Settings' **Editor** list has **None**, kept as the editor, which hides the
  action as **Show Open in editor** did; that check box and its key are gone. WEB-64 and
  WEB; the page test.
- **Reason:** The user found it odd that the editor's icon alone could be switched off.
- **Plan impact:** Decision 9 was added.
- **Validation:** `go vet ./...`, the Web UI type check, Prettier, and `go test` on
  `internal/webui`, `internal/archtest`, `cmd/rotari`, `conformance`, and
  `conformance/05-web` passed before the commit was rebased onto `9cb3be47`; after it,
  the editor, log link, and terminal panel tests of `internal/webui`, `conformance`, and
  the editor tests of `conformance/05-web` passed. The page test failed, as it should,
  with the kept None ignored. Not run: the other packages, `go test -race`, and
  pre-commit. Not tried in a real browser.
- **Remaining:** As above.
