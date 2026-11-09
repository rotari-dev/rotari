# Work History: The Web terminal and launch options in config and the environment

See [plan.md](plan.md) for current scope and status.

## Read the options from trusted configs

**Commits:** 2026-11-09 15:27:19 `eb18a1b8`.

**Change:** `allow-terminal`, `allow-exec`, `allowed-host`, and the five `terminal-*` options left `CommandLineOnly` and got `ROTARI_WEB_*` variables and config keys. New `cmd/rotari/web_config.go` reads them strictly, keeps each config file as a layer (`cliConfigLayers`), reads the first three (and `host` and `auth-token` with the terminal or launchers on) only from the command line, the environment, the global config, or `--config FILE`, and warns of each local value it ignores. Local `rotari config` templates leave out the three keys. `keepsWebAuthToken` in `internal/webui` refuses Web UI config saves that add, change, or remove `auth-token`. Contracts: WEB-3 and WEB-34 updated, WEB-36 added, config-save note in `01-resolution-and-config.md`. Docs: WEB, SECURITY, CONFIGURATION; regenerated CLI reference, environment variables, and the Python generated CLI. `ISSUES.md` records that other options ignore bad configured values.

**Reason:** The user wanted these options settable in config, with options a local config may not set ignored with a message rather than rejected. The command-line-only rule had been about bad values being silently ignored, not safety.

**Plan impact:** Proposals 1–7 implemented as planned. Proposal 8 added: the Web UI in control mode can save the global config with no token on loopback, so another local user could have chosen the next server's token. I proposed refusing saves of all trusted keys; the user pointed out that refusing only `auth-token` changes is enough, since the terminal and launchers always require a token, which such a user then cannot learn from the generated one.

**Validation:** `go vet ./...` and `gofmt` clean; `go test ./...` passed in every package (jsdom tests with the VS Code Server node and `NODE_PATH`); `go test -race` on `cmd/rotari` and `conformance/05-web` passed before proposal 8. Generators' `--check` modes, README sync, and ruff 0.17.1 on the generated Python passed. Not run: pytest (not installed) and `scripts/check.sh` as a whole (npm broken; no TypeScript changed). Checked that Web UI config generation passes no defaults, so it cannot write a token.

**Remaining:** An `auth-token` already stored in a config file is shown by the Web UI's config view, as before this change; prefer `ROTARI_WEB_AUTH_TOKEN`. Not pushed.
