# Work log: Shells of a basedir

## 2026-11-12 22:57 +09:00 — `4f130421` Shells of a basedir's own pages

**Change:**

- `internal/webui/terminal.go`
  - `terminalPlaceFor` takes `basedir` with no project as the place of that basedir's own pages, with an empty ID
    meaning the server's basedir. `run` with no project is still refused.
  - `terminalBaseDirStart` picks the basedir's parent when the basedir is named `.rotari-state`, or when the
    parent's `.rotari.toml` (`config.LoadScope("workspace", …)`) names it as `basedir`. Otherwise the shell starts
    in home without a first line. A missing parent or an unreadable `.rotari.toml` gives a first line.
  - `terminalGroup` keys such a shell `basedir + "\x00"`. The new `terminalGroupPlace` reads every group back, and
    the shell list, `/_terminal/projects`, and launch labels (`launches.go`) now use it instead of splitting the
    group each on their own.
  - A basedir's shell links to its All projects. A shell of no project now links to `/activity`, since `/` is the
    current basedir's page.
  - The group-limit refusal says "basedir" for a basedir's own shells.
- `internal/terminal/keeper.go`: `StartOptions.Unset` drops variables from the keeper's environment. A basedir's
  shell gets `ROTARI_BASEDIR` and loses `ROTARI_PROJECT_NAME`.
- `internal/webui/assets/web_shell.js`
  - `pageOfPath` classifies a page as the application script routes it: project, global (Activity, History
    search), or the basedir's. `placeOfPath` and the window's `projectPath` and `followPage` use it.
  - The sidebar marks All projects for a basedir's own shells.
  - All shells names a basedir group by its path. New shell in offers "All projects (the basedir)" under each
    basedir, and no project as `/activity`.
- `cmd/rotari/cli_spec.go`: the `--terminal-max-shells-per-project` help mentions a basedir's own pages.
  `docs/CLI_REFERENCE.md` and `python/rotari/generated_cli.py` were regenerated, the latter with `ruff format` as
  CI does.
- Docs and contracts: the Terminal section of `docs/WEB.md` (placement table, workspace rule, environment,
  limits), WEB-33 and WEB-34 notes and rows in `contracts/`, and `docs/ARCHITECTURE.md`.
- Tests
  - New conformance test `TestTerminalBasedirShellsStartInTheirWorkspace` (WEB-34) covers `.rotari-state`, a
    workspace-named basedir, one its parent does not name, a missing workspace, the server's basedir,
    environment, links, separate groups, and refusals.
  - `TestTerminalShellsFollowTheirProject` expects `/activity` as the link of a shell of no project.
  - `TestStartLeavesOutWhatItUnsets` covers the keeper.
  - `TestTerminalPanel` covers the page-to-group mapping, the All projects mark, New shell in, and window
    following. Its harness now defaults to `/activity` and models basedir shells, and Recent jobs now uses a basedir
    shell.

**Reason:** The user wanted shells for a basedir besides per-project ones, not per run. They chose the workspace rule
for the start directory, the basedir's own pages for showing them, and the mark to the right of All projects.

**Plan impact:** All three phases are done. Two decisions were added during implementation: a shell of no project
links to `/activity`, and a project page of the current basedir still omits `basedir`. Both are recorded in
[plan.md](plan.md).

**Validation:**

- `go vet ./...` passed.
- `tsc -p internal/webui/tsconfig.json`, with the VS Code server's node v24 since npm is broken here, passed.
- `go test -timeout 15m ./...` and `go test -race -timeout 15m ./...` passed.
- After two later edits (prettier formatting of `web_shell.js`, and the "basedir" refusal wording applied only to
  basedir groups), `go test ./internal/webui/ ./internal/terminal/` and `go test ./conformance/05-web/ -run
  Terminal` passed again.
- Prettier 3.9.8 (pre-commit pins 4.0 alpha) reports `web_shell.js` clean, as it does `main`'s.
- `gofmt` is clean. `pre-commit` itself is not installed.

**Remaining:**

- `TestTerminalPanelInBrowser` (WEB-33) was not extended: it needs Chrome, which this host lacks, so it runs only in
  CI.
- The basedir group-limit wording has no test.
- Not looked at in a browser.
