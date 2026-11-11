# No silent fallbacks for configured values and missing basedirs

Created 2026-11-11. Status: planned; decisions settled with the user on
2026-11-11. Phase 1 is next.

## Purpose

AGENTS.md says: "Surface errors explicitly. Never silently ignore an option or
return a success-shaped fallback for an error." Three open items in
[ISSUES.md](../ISSUES.md) break that rule, or decide it inconsistently:

1. **Bad config and environment values are ignored for most options.** A value
   that does not parse is dropped and the built-in default is used, with no
   message.
2. **Readers take a removed basedir for an empty `default` project.**
   `/api/projects` answers `["default"]` and `rotari check` reports
   `state: empty` for a basedir whose directory is gone.
3. **A standing `auth-token` blocks every static export.** This is the reverse
   case: a configured default is refused as if it had been given explicitly.
   It is included because it is the same question, which configured values
   count and how strictly, and the answer should be written once.

This is the first of three stabilization plans agreed on 2026-11-11. The
others, not yet written, are shared-rule violations (Web index run order,
array origin lookup, PID namespaces) and test-suite flakiness.

## Findings

### Option defaults (item 1)

Option defaults are read while each command defines its flag set, in
[cmd/rotari/cli_spec.go](../../cmd/rotari/cli_spec.go) and
[cmd/rotari/config.go](../../cmd/rotari/config.go). These helpers return a
pointer and have no error path, so every parse failure falls back silently:

| Helper | Config value | Environment value |
| --- | --- | --- |
| `cliBool` / `configBool` | a non-bool, or a string `ParseBool` rejects, is dropped | `ParseBool` failure is dropped (including `ROTARI_QUIET`) |
| `cliInt` / `configInt` | a non-number or non-integer string is dropped; a float such as `1.5` is truncated to `1` | `Atoi` failure is dropped |
| `cliDuration` | `ParseDuration` failure is dropped | `ParseDuration` failure is dropped |
| `cliString` with `Values` (choices) | assigned to the target without checking the choices | assigned without checking |
| `cliValue` (repeated and custom values) | `Set` errors are discarded; a value that is neither a string nor a list (for example a table) is dropped by `configStrings` | `Set` error is discarded |

The flags themselves already reject bad command-line values through the flag
package. Only the environment and config paths are silent.

`rotari web`'s terminal and launch options already follow the rule (WEB-36):
`readWebConfigured` in
[cmd/rotari/web_config.go](../../cmd/rotari/web_config.go) collects
problems while the flag set is defined. `runWebCommand` reports them after
`cliParse`, so `--help` still works, with messages such as
`invalid terminal-max-shells "lots" in global config`. This plan makes that
the behavior of every option and keeps one implementation.

Other readers of `configValue` / `cliConfig` (11 call sites outside the
helpers, including webhook and launcher sections) are not option defaults.
Phase 1 lists them and states which ones parse leniently. Any that do are
fixed in this plan if they are option-like, or recorded in ISSUES.md if they
need their own decision.

### Missing basedir (item 2)

`joblist.Projects` returns `["default"]` when `<basedir>/projects` does not
exist. That is correct for a basedir that has no state yet. It is wrong for a
basedir that does not exist at all, or for a registered basedir whose
directory was removed. Since 2026-11-09, the history search and `rotari web`'s
interrupted-run pass check for absence themselves (NOT-4), so the rule is now
decided in more than one caller instead of once.

### Static export and `auth-token` (item 3)

`staticServerOptions` in [cmd/rotari/web.go](../../cmd/rotari/web.go)
refuses `--auth-token` with `--static-dir` from any source (WEB-3, WEB-22).
A user who keeps a token in config cannot write any static export without
editing config or pointing `XDG_CONFIG_HOME` elsewhere.
`scripts/generate-static-web.sh` does neither, so it fails for such a user.

## Decisions

D3, D4, D6, and D7 were chosen by the user on 2026-11-11; each took the
recommended option.

- **D1. One collector for configured-value problems.** The typed helpers
  record a problem (option, value, source: variable name or config scope and
  path) instead of dropping the value. `cliParse` reports all of them after a
  successful parse and returns an error, so `--help` keeps CLI-17 behavior and
  normal execution fails. `readWebConfigured` is moved onto the same
  collector instead of keeping its own.
- **D2. Problems are reported only for options the command reads.** A root
  `retry = "lots"` fails `run` and `retry`, not `show`. This follows from
  collecting at flag definition.
- **D3. A bad configured value is an error even when the command line gives
  the option.** This matches WEB-36 and is simpler to explain. Rejected:
  reporting it only when it would have been used, which lets a wrong setting
  stay unnoticed.
- **D4. An empty environment variable means unset for typed options**
  (bool, int, duration, choice). `cliValue` already treats an empty value as
  unset, and `VAR=` is a common way to clear a variable in a shell. Plain
  strings keep the empty value, since an empty string can be meaningful.
- **D5. Integers must be integers.** A config number with a fraction, or a
  number out of range, is an error, not truncated.
- **D6. Missing basedir.** Separate "the basedir directory does not exist"
  from "it exists but has no projects", once, in the package that owns project
  listing. Read-only commands (`check`, `show`,
  `jobs`, listing commands, `/api/projects`, the Web index for registered
  basedirs) report the basedir as missing: an error from the CLI, and a
  visible missing state in the Web UI. Commands that create state (`add`,
  `run`, `init`) keep creating it. The two NOT-4 special cases then use the
  shared rule.
- **D7. Static export and `auth-token`.** A static export ignores
  `auth-token` from config and the environment, and still refuses it on the
  command line; the contract says so. A token in config means "protect my
  live server" and has no meaning for files on disk. Both
  `rotari web --static-dir` and `rotari tableview --static-dir` use one helper
  for this. Rejected: refusing from every source and documenting a
  workaround.

## Phases

1. **Survey and reproduce.** Use the built binary to confirm each row of the
   findings table with one example per helper. List the other `configValue`
   readers and their parsing. Check which commands treat a missing basedir as
   empty. Record anything outside scope in ISSUES.md.
2. **Collector and typed helpers (item 1).** Implement D1 to D5 in `cmd/rotari`. Move `readWebConfigured` onto the
   collector without changing WEB-36's messages. Error format:
   `invalid OPTION "VALUE" in SOURCE: REASON`, as WEB-36 uses now.
3. **Missing basedir (item 2).** Implement D6 in the owning package
   (`internal/joblist` or `internal/project`; Phase 1 decides), and switch
   `check`, the listing commands, `/api/projects`, the Web index, and the
   NOT-4 callers to it. Python client: check that `rotari` errors surface as
   exceptions and are not swallowed.
4. **Static export (item 3).** Implement D7 in one helper used by both
   static-export paths.
5. **Contracts and documentation.** Extend CLI-3 (or add a CLI rule) to say
   that bad environment and config values are errors naming their source.
   Update RES rules for the missing basedir, WEB-3/WEB-22 for D7, and
   [docs/CONFIGURATION.md](../../docs/CONFIGURATION.md) and
   [docs/ENVIRONMENT_VARIABLES.md](../../docs/ENVIRONMENT_VARIABLES.md).
   Move the three ISSUES.md items to Resolved.

## Tests and validation

- Conformance: one table test across commands and helper kinds: bool, int,
  duration, choice, repeated value. Each case uses a bad value from the
  environment and from each config scope, and expects a nonzero exit and the
  source in stderr. The same cases with `--help` expect exit 0 and the CLI-17
  warning. Add a case for D3, and a case showing that a root value fails only
  the commands that read it (D2).
- Conformance for the missing basedir: a removed registered basedir and a
  nonexistent `-b` for each read-only command and `/api/projects`, and a fresh
  basedir with no state, which must stay valid.
- Conformance for D7 on both static-export commands.
- Existing WEB-36, CLI-3, CLI-17, and NOT-4 tests keep passing unchanged.
- `go test ./cmd/rotari ./internal/...`, `go test ./conformance`, pre-commit
  on changed files, then `scripts/check.sh`.

## Non-goals

- Validating option values beyond parsing, such as ranges or combinations,
  except where a command already does so for the command line.
- Unknown config keys. They are a separate question and are not changed here.
- The config file format and merge rules (RES-23).
