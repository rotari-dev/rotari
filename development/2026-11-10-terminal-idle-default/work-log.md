# Work log: Shells last 3 hours without a page

## Step 1: the default and the tip

**Commits:** None yet.

**Change:** `webTerminalIdleLimit` is `3 * time.Hour`; the flag's description says `default 3h`, regenerated into
[CLI_REFERENCE.md](../../docs/CLI_REFERENCE.md) and [generated_cli.py](../../python/rotari/generated_cli.py) by the
recipe in [scripts/README.md](../../scripts/README.md) (the generator's output was reformatted with `ruff format`,
leaving one changed line). WEB-34 says three hours. [WEB.md](../../docs/WEB.md) says 3 hours, its table `3h`, and
its new section "Keeping shells through a disconnect" gives the tmux and timeout tip.

**Reason:** [The plan](plan.md).

**Plan impact:** None.

**Validation:** `go test ./cmd/rotari` (the settings tests use explicit limits and the default's validity),
`./conformance/03-interfaces/pairweb`, and `./conformance` passed. The Python tests were not run: pytest is not
installed here.

**Remaining:** None.
