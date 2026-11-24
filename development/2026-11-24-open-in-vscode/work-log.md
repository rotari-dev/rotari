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
