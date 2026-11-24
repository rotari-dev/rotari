# Work log: The foreground program's name in terminal tabs

Plan: [plan.md](plan.md)

## 2026-11-24 18:16:40 +0900 - `4d21ebc7` - Both phases

- **Change:** `internal/terminal` names the program that leads a shell's foreground
  group from the arguments it already read (`programName`); `GET /_terminal/shells`
  gives it as `program`; a tab shows it after its number or name, in its tooltip,
  and in its accessible label. WEB-33, WEB-34, and WEB; unit, JSDOM, and conformance
  tests, and a step of the browser test.
- **Reason:** The plan's two phases, shipped as one change.
- **Plan impact:** Both phases are implemented as agreed.
- **Validation:** `go vet ./...`, `go vet` of `internal/terminal` for macOS, the Web
  UI type check, and `go test` on every package but `pairedits`, `pairjobcontrol`,
  and `pairruns` passed, with Go 1.25.8 and Node 24. Not run: those three,
  `go test -race`, pre-commit (not installed), and the browser tests (no Chrome).
- **Remaining:** Seeing the tabs in a real browser, with long names and at a side.
