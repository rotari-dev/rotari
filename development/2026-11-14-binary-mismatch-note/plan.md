# Plan: Say when a running server is another rotari binary

Created: 2026-11-14
Status: Proposed. Follows [shells last as long as the server by default](../2026-11-13-terminal-idle-off/plan.md),
as the user ordered on 2026-11-13.

## Goal

When a command attaches to something rotari left running, and that process runs a different rotari binary than the
command, the command says so in one line. This catches "I upgraded, but the old server or run is still going",
whose behavior follows the old binary. It is a note, never a refusal: watching a run started by another binary is
normal and useful.

## Existing implementation

- **Runs.** `rotari run` always starts its own supervisor from its own binary (`server.Start` with
  `os.Executable()`), so no mismatch can arise there. Later commands attach to a running run without the
  supervisor's protocol: `rotari wait` (and `run --async` followed by `wait`) follows its state files and
  attachment sessions. The run's durable context (`state.LoadContext`) holds `model.RunCoordinator`: host, PID,
  PID namespace, process start, and record times. `projectrun/context.go` writes it when the run starts. Nothing
  records the binary.
- **Detached Web servers.** `webdetach.Record` holds `Version` and `Executable` (a path), but no content hash, so a
  rebuild at the same path with the same version string (`rotari dev`) looks the same. `rotari web --detach`
  (reporting a running server), `--shutdown`, the attached client's refused Ctrl-D, and the foreground note all
  read the record.
- `currentExecutableIdentity` (`cmd/rotari/executable_identity.go`) already computes the running binary's SHA-256
  and modification time for `weblog` and `bug-report`.

## Proposals

1. **Record the binary.** `model.RunCoordinator` gains `version`, `executable`, and `executable_sha256`, all
   optional, which the supervisor writes with the rest of its identity. `webdetach.Record` gains
   `executable_sha256`. Old records lack the fields and are treated as "unknown", which prints nothing.
2. **Compare.** A binary differs when the SHA-256 of both is known and they differ. Then the note names the other
   binary's version and path, and this one's:
   `Note: run R is managed by rotari v0.1.15 (/opt/old/rotari), not this rotari v0.1.16 (/usr/local/bin/rotari).`
   When the versions are equal (as for two `rotari dev` builds), it says `a different build of rotari v…`. The
   current binary's hash is computed only when the record has one to compare with, so commands with nothing to
   compare pay nothing.
3. **Where it is said:**
   - `rotari wait` attaching to a run whose coordinator is still running: once, on stderr, before the progress. With
     `--json` it goes on stderr too, so the JSON stays as it is.
   - `rotari web --detach` reporting a running server, `rotari web --shutdown`, and the foreground note about a
     detached server: one line after theirs.
   - `rotari show` and the Web UI: not in this plan. They describe a run rather than attach to it.
4. **Never an error, never silent about a failure.** When this binary's hash cannot be computed (it was replaced
   or deleted under the process), the note says the comparison could not be made and why, and the command goes on.
   A record without the fields, written by an older rotari, says nothing: there is nothing to compare, which is
   not a failure.

## Interfaces and documents

- **CLI:** the notes above; no new flags.
- **Persistent state:** the new optional fields of the run context and of the detached-server record. Read
  `contracts/README.md` for the run-context rules before changing it.
- **Python client and MCP:** they read the run context through the CLI or shared functions. The fields are additive,
  and no output changes beyond the stderr notes.
- **Docs:** RUNNING.md (`wait`) and WEB.md ("Keeping shells through a disconnect") mention the note. FAQ: "I upgraded
  rotari; why does my run behave the old way?".
- **Contracts:** a note and conformance tests for the recorded fields and the notes.

## Steps

1. Record the fields: the supervisor's coordinator identity, and `webdetach.Record`. Unit tests check that old
   records without them still load.
2. The comparison and note in one place (`cmd/rotari`), used by `wait` and the web commands.
3. Conformance tests: build a second binary that differs only by a link-time version, start a run (or a detached
   server) with one, then attach with the other and expect the note. Attaching with the same binary, and to an old
   record without the fields, expects none.
4. Docs, contracts, work log.

## Questions

- **Hash cost:** hashing the binary (tens of MB) takes a few tens of milliseconds on each `wait` that attaches to a
  run with a recorded hash. Is that acceptable, or should the comparison use path, size, and modification time
  instead? That is cheaper, but it misses a binary copied over with the same size and time.
- **`run` and `retry` reattaching:** when `rotari run` finds the project's run already active and reports it, should
  it say this too? The proposal covers `wait` only.
- **The Web UI:** the Settings menu already shows which rotari serves the page. Should it also say when the
  project's running run is managed by another binary? Left out for now.

## Risks

- A run context written by a new binary and read by an old one: the extra fields are ignored by Go's JSON decoding,
  so old binaries are unaffected.
