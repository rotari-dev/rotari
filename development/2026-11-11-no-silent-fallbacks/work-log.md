# Work log: no silent fallbacks

## Plan and decisions

Commit: `1b6b6efa` (2026-11-11 15:48:42 +0900)

- **Change:** Added [plan.md](plan.md). It covers three open ISSUES.md items:
  bad config and environment values are ignored, a removed basedir reads as an
  empty `default` project, and a standing `auth-token` blocks static exports.
- **Reason:** This is the first of three stabilization plans agreed with the
  user on 2026-11-11, after a review of the open issues. The other two are
  shared-rule violations and test-suite flakiness.
- **Plan impact:** The user settled D3, D4, D6, and D7, each with the
  recommended option. A bad configured value is an error even when the command
  line overrides it. An empty environment variable means unset for typed
  options. A missing basedir is reported as missing by read-only commands and
  the Web UI. A static export ignores `auth-token` from config and the
  environment.
- **Validation:** Documentation only; no tests run.
- **Remaining:** Phases 1 to 5.

## Phase 1: survey and reproduce

Commit: `c9462df6` (2026-11-11 15:54:06 +0900)

- **Change:** Updated the findings in [plan.md](plan.md) with reproduced
  behavior, the list of project-listing implementations, and the owning
  package for the shared listing (`internal/state`). Phases 2 and 3 were
  refined to match.
- **Reason:** Phase 1 of the plan: confirm each case with the binary before
  changing behavior.
- **Plan impact:**
  - Bool, int, and duration values from config and the environment are
    dropped or truncated silently, as ISSUES.md said.
  - Choices and repeated values are not silent: the command rejects them
    later, but the error names `--option` and not the variable or file. A
    table given for a repeated option is dropped silently.
  - `configString` turns a list or table into its `fmt.Sprint` text.
  - No reader outside the helpers parses leniently; notification settings are
    already strict.
  - A missing basedir and an empty one give identical output from `check`,
    `projects`, `jobs`, `runs`, `notifications`, `wait`, `show`, `export`, and
    `lineage`. The cause is ten separate readers of `<basedir>/projects`.
- **Validation:** Built `rotari dev (1b6b6efa77e3)` with go1.25.8 into the
  session scratchpad. Ran it with `env -i`, an isolated `HOME`,
  `XDG_CONFIG_HOME`, and basedir, using `--help` defaults and real `add` /
  `run` / `config` / `export` invocations for each helper kind. Then ran the
  read-only commands above against a nonexistent and an empty basedir. The
  `/api/projects` behavior was not rerun; it is taken from the ISSUES.md entry
  of 2026-11-04.
- **Remaining:** Phases 2 to 5.

## Phase 2: configured option values

Commit: `23bdc89a` (2026-11-11 16:22:45 +0900)

- **Change:**
  - New [cmd/rotari/configured_value.go](../../cmd/rotari/configured_value.go)
    holds the value type that `web_config.go` used (`webValue`, now
    `configuredValue`), its parsers, and a collector of problems per flag set.
  - `cliBool`, `cliInt`, `cliDuration`, `cliString` (through
    `configuredString`), and `cliValue` (through `configuredRepeated`) in
    `cli_spec.go` read config and then the environment through it.
    `parseCLIFlagSet` reports the problems after a successful parse, or warns
    under `--help`. `configBool`, `configInt`, and `configStrings` were
    removed; tests keep test-only `configInt`/`configBool` accessors that
    panic on a bad value.
  - `loadCLIConfig` reads `basedir` and `project-name` through
    `configLocation`, so a list or table is a config error (CLI-17) before
    any path is made from it.
  - `readWebConfigured` takes the flag set and reports through the
    collector. It now checks the config value and the variable separately,
    so a bad config value is reported even when the environment replaces it.
  - Contract CLI-30 (note and status row, CLI-17 row lists the new test),
    [docs/CONFIGURATION.md](../../docs/CONFIGURATION.md),
    [docs/ENVIRONMENT_VARIABLES.md](../../docs/ENVIRONMENT_VARIABLES.md), and
    the ISSUES.md entry moved to Resolved.
- **Reason:** Phase 2, implementing D1 to D5.
- **Plan impact:**
  - Item 1 is done. Phase 5's contract work for item 1 was done here as
    CLI-30 rather than as an extension of CLI-3.
  - Found while testing: a list for `basedir` made `add` create a directory
    named `[a b]`, because location values are read before any flag set
    exists. Hence `configLocation`.
  - `jobs` ignores a configured `basedir` by design, so it does not report
    one, which follows D2.
  - Repeated values whose content is validated by the command later, such as
    `--env` without `=`, still name the flag and not the source: their flag
    values accept any text in `Set`.
- **Validation:**
  - New `TestBadConfiguredValuesAreErrors` (11 cases across bool, integer,
    duration, choice, table, and basedir, from variables, global, and
    workspace configs, each with `--help`; plus D2 and D4 checks) passes.
  - Added a WEB-36 case for a bad config value under a valid variable.
  - `go test ./conformance` (contract status table) passes.
  - Full run of the `scripts/check.sh` steps: `go vet ./...`, `tsc -p
    internal/webui/tsconfig.json` (run directly, as `npm` is broken on this
    machine), `go test ./...`, and `go test -race ./...`, all passing with
    go1.25.8 and Node v24. pre-commit is not installed; `gofmt -l` is clean.
  - Rebuilt the scratch binary and reran the Phase 1 reproductions; each now
    fails with the source named.
- **Remaining:** Phases 3 to 5. The source of later `--env`-style
  validation errors is not named (see above).

## Phase 3: missing basedir

Commit: `28a3f1cb` (2026-11-11 18:47:12 +0900)

- **Change:**
  - New [internal/state/project_names.go](../../internal/state/project_names.go):
    `ProjectNames` is the one listing of `<basedir>/projects`. It returns
    `BaseDirMissingError` (matching `ErrBaseDirMissing`) for a basedir that
    does not exist, nil when there is no projects directory, and an empty
    list when the directory is empty. `ProjectNamesOrNone` reads a missing
    basedir as having no projects, and `RequireBaseDir` checks existence.
  - All sixteen readers use it: `internal/joblist`, `internal/project`
    (`Overviews`), `internal/config`, `internal/resolve`, `internal/runlist`
    (twice), `internal/webui` (index and static state), and in `cmd/rotari`
    `config.go` (twice), `server.go`, `completion.go` (twice), `info.go`,
    `show.go`, and `wait.go` (twice).
  - `loadCLIConfig` records the basedir an invocation was given and where it
    came from. `warnMissingGivenBaseDir` in `main` warns for read-only
    commands, except `wait`.
  - `jobsBaseDirs` leaves out registered basedirs that are gone and returns
    them for `warnMissingBaseDirs`. `projects` now uses it instead of its own
    copy of the registry walk.
  - Web: `webBaseDir.Missing` is set in `basedirEntries` for registered
    basedirs other than the server's own. Both `basedirLabelHTML` versions
    (Go and `web_app_core.js`) add a `missing` mark, styled in
    `web_shell.css`. `site.projectNames` gives the server's own missing
    basedir its default project and is an error for any other, which
    `/api/projects` answers with 404. The history search and `runwatch` use
    the shared error instead of their own `os.Stat`.
  - Contract RES-28 and status row;
    [docs/CONCEPTS.md](../../docs/CONCEPTS.md),
    [docs/OPERATIONS.md](../../docs/OPERATIONS.md),
    [docs/WEB.md](../../docs/WEB.md), and
    [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md); the ISSUES.md entry
    moved to Resolved.
- **Reason:** Phase 3, implementing D6.
- **Plan impact:**
  - D6a: before writing code, the user chose to split the missing cases into
    a given basedir, the implicit default, and a registered one.
  - D6b: the first full test run failed nine existing conformance tests
    (RES-3, RES-16, RES-24, the registry-mismatch and "project is in another
    basedir" messages) and two CLI-30 cases, because a given basedir that
    does not exist was an error. A basedir named in `ROTARI_BASEDIR` or by
    `rotari init` exists only after its first run. The user chose a warning
    instead, which keeps output and exit status. `wait` was dropped from the
    warning for RES-16.
  - The Phase 1 count of ten readers was wrong; there were sixteen (the
    search output had been cut short). The plan records the correction.
  - Item 2 is done.
- **Validation:**
  - New `TestMissingGivenBasedirIsSaid` and
    `TestRemovedRegisteredBasedirIsReported` pass. The first covers each of
    the eight read-only commands with a missing and an existing empty
    basedir, comparing exit status, plus the variable and config sources,
    `--help`, silent `wait`, the implicit default, and `add` creating its
    basedir. The second covers the warning in `jobs`, `projects`, `runs`, and
    `notifications`, `/api/basedirs` `missing`, `/api/projects` 404 and the
    server's own default, and the sidebar mark on the jobs page.
  - `go test ./conformance` (contract status table) passes.
  - After the D6b change, a full run of the `scripts/check.sh` steps passed:
    `go vet ./...`, `tsc -p internal/webui/tsconfig.json` (run directly),
    `go test ./...`, and `go test -race ./...`. The earlier run with the
    error behavior failed as described above. `gofmt -l` is clean;
    pre-commit is not installed.
  - With a scratch binary, ran each read-only command against a missing and
    an empty basedir, and the aggregate commands with a removed registered
    basedir, before the D6b change; the conformance tests cover the final
    behavior.
- **Remaining:** Phases 4 (static export and `auth-token`) and 5 (remaining
  contract and documentation checks).

## Phases 4 and 5: static export token, contracts, and documentation

Commit: `95ebe62e` (2026-11-11 19:05:42 +0900)

- **Change:**
  - `staticServerOptions` in [cmd/rotari/web.go](../../cmd/rotari/web.go),
    which both `rotari web --static-dir` and `rotari tableview --static-dir`
    use, no longer counts an `auth-token` from the environment or config.
    `warnIgnoredStaticToken` warns, naming the variable or config file but
    never the token. `--auth-token` on the command line is still refused.
  - New `TestStaticExportsIgnoreConfiguredToken` (both commands, both
    sources, and the command-line refusal). The env-token rejection
    assertions in `TestCLIFlagPairWebStaticServerOptions` and
    `TestTableViewCommandWritesAStaticSnapshot` were removed: they encoded
    the old WEB-3/WEB-22 rule that D7 changes.
  - WEB-3 and WEB-22 notes and status rows, the static export section of
    [docs/WEB.md](../../docs/WEB.md), the ISSUES.md entry moved to
    Resolved, and the plan marked done.
- **Reason:** Phase 4, implementing D7. Phase 5 had been done along the way:
  CLI-30, RES-28, WEB-3/WEB-22, CONFIGURATION.md, ENVIRONMENT_VARIABLES.md,
  CONCEPTS.md, OPERATIONS.md, WEB.md, ARCHITECTURE.md, and all three ISSUES.md
  items.
- **Plan impact:**
  - The plan is complete.
  - Scope kept to `auth-token`, as D7 says. A `host`, `port`, or
    `allow-terminal` from config still refuses a static export: WEB-3 and
    WEB-36 decide that on purpose, with their own test
    (`TestTerminalOptionsRejectBadConfiguredValues`).
- **Validation:**
  - The new and changed tests pass.
  - `go test ./conformance` (contract status table) passes.
  - A full run of the `scripts/check.sh` steps passed: `go vet ./...`,
    `tsc -p internal/webui/tsconfig.json` (run directly), `go test ./...`, and
    `go test -race ./...`. `gofmt -l` is clean; pre-commit is not installed.
- **Remaining:** None for this plan. Two follow-ups are recorded above: the
  source of later `--env`-style validation errors is not named (Phase 2),
  and other live-server options from config still refuse a static export by
  design.
