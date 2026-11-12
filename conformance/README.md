# Conformance tests

The tests in this directory exercise the built `rotari` binary and Web API from
outside the implementation packages. Contract coverage is checked by
`TestContractStatus` in `contracts_test.go`.

Maintainer notes on test strategy and coverage are indexed in
[docs/internal](../docs/internal/README.md). Package-specific run instructions
remain beside their tests.

## Golden outputs

`golden_test.go` compares stable representative CLI output with the files in
`testdata/golden/`:

- `help.txt` covers `rotari --help`.
- `schema.json` covers the command and flag metadata from `rotari schema --json`.
- `show.json` covers the deterministic queue projection from `rotari show --queue --json`.

Run the tests normally to detect an intentional or accidental output change:

```sh
go test ./conformance -run TestGoldenOutputs
```

When a deliberate CLI metadata or output change is made, regenerate the files,
inspect the diff, and commit the updated golden files with the implementation:

```sh
go test ./conformance -run TestGoldenOutputs -update
```

The schema golden keeps command and flag names, short names, value names, and
fixed value choices. Descriptions and other derived metadata are covered by
help output or focused tests rather than duplicated in the schema snapshot.
Dynamic paths, IDs, and timestamps in JSON output are normalized before
comparison.

## Evidence from a failing test

When a test that uses `support.NewEnv` (or `newEnv` in this directory)
fails, its state directory is moved to
`$TMPDIR/rotari-conformance-failed/TEST-NNNN/state`, and
`commands.log` beside it lists every rotari command the test ran: when it
started, where, its exit code, and the full output of the commands run to
completion. The failure output names the directory. A passing test keeps
nothing, and nothing removes these directories, so delete them when done.

To reproduce an intermittent failure, run the test many times under CPU
contention with [scripts/flake-repeat.sh](../scripts/flake-repeat.sh), which
keeps the `TMPDIR` of each failing process.
