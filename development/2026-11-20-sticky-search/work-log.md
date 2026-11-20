# Work log: Search Sticky Notes

Plan: [plan.md](plan.md)

## 2026-11-20

- Read `internal/stickies`, `cmd/rotari/stickies.go`, the `/api/stickies`
  handler, `web_app_stickies.js`, and WEB-62. The menu already listed every
  note, so search became a filter on that list, matched on the server.
- Added `stickies.Search` (all words, case-insensitive; an empty query is an
  error). Wired it into `GET /api/stickies` (`q`, with or without `page`) and
  `rotari stickies --search`.
- Added a search box to the **Stickies** menu (`assets.go`, `web_app_stickies.js`,
  `web_shell.css`). A search does not update the sidebar marks.
- Regenerated `python/rotari/generated_cli.py` (then `ruff format`, to keep the
  diff small), `docs/CLI_REFERENCE.md`, and the `schema.json` golden.
- `TestCLIFlagPairInventory` failed ("schema changed") because `stickies` gained
  a flag; updated its pinned count (4 to 5) and fingerprint in
  `conformance/03-interfaces/flag_pairs_test.go`. The sample values are
  generic, so no new sample was needed.
- Updated WEB-62 (contract and README table), `docs/WEB.md`, and
  `docs/ARCHITECTURE.md`.

## Tests

- Passed: `internal/stickies`, `internal/archtest`, `conformance/05-web`
  (`TestWebStickiesAreSharedWithTheCLI`, `TestWebStickiesAreSearchable`),
  `conformance/03-interfaces`, `go vet`, `gofmt`,
  `generate_cli_reference.py --check`.
- Could not run: the Node-based tests in `internal/webui` and two in
  `cmd/rotari`, and `npm run typecheck`, because the machine has Node v12.
  `scripts/check.sh` and pre-commit were not run.

## Remaining

- CI browser check that typing in the search box narrows the list and clearing
  it restores the list.
- Commit with `jj` once CI checks are acceptable.
