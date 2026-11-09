# Plan: The Web terminal and launch options in config and the environment

Created: 2026-11-09
Status: Implemented. The user agreed to the direction and asked that options a local config may not set be ignored with a
message when `rotari web` starts, rather than rejected; they later chose to refuse only `auth-token` changes in Web UI
config saves (proposal 8).

## Goal

`--allow-terminal`, the `--terminal-*` limits, `--allow-exec`, and `--allowed-host` can be set in a config file or an
environment variable, as other `rotari web` options can. Options that let other people reach this host are read only
from places the user controls: the command line, the environment, the global config, or a file named with `--config`.
A config in the working directory, the basedir, or a project can be written by someone else or arrive with a cloned
repository, so its values for those options are ignored, and `rotari web` says so when it starts.

## Existing implementation

- **The options are command-line only.** In [cli_spec.go](../../cmd/rotari/cli_spec.go), `CommandLineOnly` keeps
  config files and environment variables from supplying `allow-exec`, `allow-terminal`, the five `terminal-*` flags,
  `allowed-host`, and `artifact-root`. A config value for them is ignored with no message, and there is no
  `ROTARI_WEB_ALLOW_TERMINAL`.
- **That was for bad values, not for safety.** The [terminal work log](../2026-11-05-web-terminal/work-log.md) made the
  `terminal-*` flags command-line only because `cliInt`, `cliDuration`, `cliBool`, and `configInt` ignore a value
  they cannot parse and keep the default.
- **`host` and `auth-token` are read from every config scope.** Tried on 2026-11-06: a `.rotari.toml` in the working
  directory with `host` and `auth-token` set makes `rotari web --allow-terminal` listen on that host behind that token.
  Because shells need HTTPS or a direct loopback connection (WEB-34), plain HTTP from elsewhere cannot open one, but
  job control and launchers can be reached.
- **Config merges global → workspace → basedir → project** in `loadCLIConfig` ([config.go](../../cmd/rotari/config.go)),
  and `cliFileConfig.Sources` names each file and its scope. `configValue` reads the merged map, the command's section
  first. A file named with `--config` replaces discovery and has the scope `explicit`.
- **`--static-dir` rejects live-server options** supplied by flag, environment, or config (WEB-3), through
  `staticServerOptions` in [web.go](../../cmd/rotari/web.go), which skips command-line-only options when it looks at
  the environment and config.
- **Launchers are already global only:** `--allow-exec` runs only commands registered in the global `launchers.toml`.

## Proposals

1. **The options leave `CommandLineOnly`.** `allow-terminal`, `allow-exec`, `allowed-host`, and the five `terminal-*`
   options get config keys in `[web]` (or at the top level, as other keys) and environment variables named by the
   usual rule: `ROTARI_WEB_ALLOW_TERMINAL`, `ROTARI_WEB_ALLOW_EXEC`, `ROTARI_WEB_ALLOWED_HOST`,
   `ROTARI_WEB_TERMINAL_IDLE_TIMEOUT`, `ROTARI_WEB_TERMINAL_MAX_SHELLS`, `ROTARI_WEB_TERMINAL_MAX_SHELLS_PER_PROJECT`,
   `ROTARI_WEB_TERMINAL_REPLAY_SIZE`, and `ROTARI_WEB_TERMINAL_SHELL`. `artifact-root` stays command-line only; it is
   not part of this request.
2. **Bad values are errors.** A value for these options from config or the environment that does not parse, or is out
   of range, stops `rotari web` before the server starts, naming the option, the value, and where it came from (the
   variable, or the config file). Range checks are the ones `newWebTerminalSettings` already makes. The general case,
   every option whose bad config or environment value is ignored, is recorded in `ISSUES.md` rather than changed here.
3. **Trusted and local scopes.** Global and explicit (`--config FILE`) configs are trusted. Workspace, basedir, and
   project configs are local.
   - `allow-terminal`, `allow-exec`, and `allowed-host` are always read only from the command line, the environment,
     and trusted configs.
   - `host` and `auth-token` are read that way too whenever the server ends up with the terminal or launchers on.
     Without them, they keep reading every scope, as now.
   - The `terminal-*` limits and `terminal-shell` are read from every scope. The limits only narrow what the terminal
     does, and a local config can already name commands that `rotari run` executes, so `terminal-shell` adds no new
     trust.
   With a local value ignored, the value comes from the next place in the order: a trusted config, then the built-in
   default.
4. **The message.** When a local config's value for one of those options is ignored, and no command-line or
   environment value would have replaced it anyway, `rotari web` prints one warning per option on stderr before the
   server starts, for example:

   ```text
   WARNING: ignoring allow-terminal = true in workspace config /work/.rotari.toml; rotari web reads it only from the command line, ROTARI_WEB_ALLOW_TERMINAL, the global config, or --config FILE
   ```

   The server still starts. The warning names the scope, the file, the key, and the value, except for `auth-token`,
   whose value is never printed. A key in both the top level and `[web]` of one file gives one warning, for the value
   that would have applied.
5. **`--static-dir` keeps WEB-3.** A trusted config or environment value that turns on the terminal or launchers, or
   sets a `terminal-*` option, is a live-server option supplied by config, so `--static-dir` rejects it, as it does
   `host` in a config now. A local value that is ignored under 3 is not supplied, so it is not rejected, but it still
   gets the warning. (Alternative: let `--static-dir` ignore config-supplied live-server options with a warning.
   That changes WEB-3 for `host`, `port`, and `allow-control` too, so it is left out unless the user asks for it.)
6. **Command-line checks stay command-line checks.** `--terminal-max-shells` and the others on the command line
   without the terminal on are still errors. The same keys in a config, with the terminal off, are unused defaults
   and say nothing, as other config keys for other commands do.
7. **`rotari config` templates.** The global template lists all the new keys. Workspace, basedir, and project
   templates leave out `allow-terminal`, `allow-exec`, and `allowed-host`, and say beside `host` and `auth-token` that
   they are ignored with the terminal or launchers on. `--help` shows defaults with ignored local values left out.
8. **Web UI config saves keep `auth-token`.** The Web UI in control mode can save the global config, with no token on
   loopback. A save that adds, changes, or removes `auth-token` (top level or `[web]`, any scope) is refused, naming
   the file to edit directly. The terminal and launchers always require a token, so a page that cannot choose it
   cannot open a shell by writing `allow-terminal`, `host`, or `allowed-host`; those keys stay savable.

## Interfaces

- **CLI:** `rotari web` and its help; `rotari env` and `rotari config` templates; the CLI schema (`command_line_only`
  goes away for these options) and its golden file.
- **Web UI/API:** the server's behavior does not change once started. The `server_start` event's `auth` field is
  checked for whether a config-supplied token needs its own value. The Web UI's config view shows the raw files, which
  is unchanged.
- **Python client:** the generated CLI follows the schema through `scripts/generate_python_cli.py`. Its API is
  unchanged.
- **Server/MCP:** unaffected.

## Steps

1. **Trusted values.** In `loadCLIConfig`, keep the merged values of the trusted layers beside `cliFileConfig`, and the
   local layers' values with their sources, so that one lookup can give the trusted value and the local value it hid.
   One function in `cmd/rotari` owns the rule in 3, called once from `parseWebFlags`; `staticServerOptions` uses it.
2. **Strict reading** for the options in 1: a reader that returns an error for a bad value with its source, used for
   the environment and config values of these options.
3. **`parseWebFlags`** decides the terminal and launchers first, then reads `host` and `auth-token` under 3, prints the
   warnings of 4, and keeps the existing checks.
4. **Specs and generated files:** `cli_spec.go`, the schema golden file, `docs/CLI_REFERENCE.md`,
   `docs/ENVIRONMENT_VARIABLES.md`, the Python generated CLI, through their generators.
5. **Contracts:** CLI-20 (the options are no longer exceptions), WEB-3 (as 5 says), WEB-34 (the options may come from
   config and the environment), and a new WEB-36 for trusted scopes and the warning, with the status table in
   `contracts/README.md`.
6. **Documents:** [WEB.md](../../docs/WEB.md), [SECURITY.md](../../docs/SECURITY.md) (which config files can open the
   server to others), and [CONFIGURATION.md](../../docs/CONFIGURATION.md) (keys read only from trusted scopes).
   `README.md` only through `docs/GETTING_STARTED.md` and `scripts/sync_readme.py`, if Getting Started changes.
7. **`ISSUES.md`:** bad config and environment values ignored for other options.

## Validation

- Unit tests in `cmd/rotari` for the rule in 3: each option from each scope, CLI over environment over trusted config,
  a local value hidden by a trusted one, `host` and `auth-token` with the terminal or launchers on and off, the
  top-level and `[web]` spellings, and `--config FILE` as trusted.
- Conformance through the binary (WEB-36): a workspace `.rotari.toml` with `allow-terminal = true` leaves the terminal
  routes at 404 and prints the warning; the same key in the global config turns them on; a workspace `host` and
  `auth-token` with `--allow-terminal` are ignored with a warning and the global or generated token applies; bad
  environment and config values are rejected before the server starts; `--static-dir` with a global `allow-terminal`
  is rejected and with a workspace one is not. Test servers listen only on loopback.
- `TestCLIFlagEnvironmentCoverage` (CLI-20) passes with the new variables; existing WEB-3 and WEB-34 tests pass.
- `go test ./...`, the generators' `--check` modes, pre-commit on changed files, and `scripts/check.sh`.

## Non-goals

- `artifact-root` in config.
- Strict parsing for every other option's config and environment values (recorded in `ISSUES.md` instead).
- Changing what `rotari run` trusts in local configs.
