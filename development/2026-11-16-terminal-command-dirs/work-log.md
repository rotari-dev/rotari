# Work log: Relative paths in the Web terminal's output, from where each command ran

## 2026-11-16 01:57 +09:00 — Integration and server

- **Change:** `marks.bash` and `marks.zsh` print OSC 7 (`file://HOST/PATH`, `$HOSTNAME` / `$HOST`, the path
  percent-encoded byte by byte with / kept) before `D`/`A` at each prompt (`__rotari_dir`, encoding a directory
  once); the encoder `cmdline_url=` used becomes `__rotari_encode`, shared by both. `POST
  /_terminal/shells/ID/links` takes `dir` (absolute, at most 4 KiB, else 400), against which relative paths are
  resolved instead of the shell's directory now, and `host` (at most 4 KiB): one that is not this host
  (`terminalLocalHost`: equal ignoring case, equal up to the first dot, empty, or localhost) links no paths, IDs
  still; a host name the server cannot read is said in `problems`. The body limit leaves room for both.
- **Reason:** plan phase 1, after the user accepted the plan with the open questions as proposed.
- **Plan impact:** none.
- **Validation:** `TestIntegrationSaysTheDirectory` (real bash and zsh, a directory with a space and a non-ASCII name,
  typed through printf since zsh's line editor without a UTF-8 locale takes the bytes apart; the host this one, and
  the directory before the cd too) and `TestTerminalLinksAreWhatExists` (a file found under `dir` and not under the
  shell's directory; this host by several names; another host linking only the run; a relative or long `dir` and a
  long `host` refused) pass; `go test ./internal/terminal ./internal/webui ./conformance/05-web` pass.
- **Remaining:** phases 2 (the panel) and 3 (contracts and docs).

## 2026-11-16 02:12 +09:00 — Panel

- **Change:** `web_shell.js`: each pane reads OSC 7 (`readDirMark`, `registerOscHandler(7)`) into `pane.place`:
  `{dir, host}` when the path percent-decodes to UTF-8 (strictly, `TextDecoder` fatal) and is absolute, else
  `{unknown: true}`; a new connection drops it with the blocks, and the replay brings it back. A block takes the
  pane's place at its `A`. The link provider asks with the place of the block holding the hovered line
  (`placeAt`, null for a line in no block): `dir` and `host` in the body; an unknown place asks about no
  relative path (`relativePath`: not a run or job ID, holding / and not starting with it) and sends no `dir`;
  answers are cached per shell, place, and text.
- **Reason:** plan phase 2.
- **Plan impact:** none.
- **Validation:** `TestTerminalPanel`: a block from the replay in `/w/a b/日本` (percent-encoded) linking `src/x.csv`
  there with `dir` and `host` sent; the block after `cd /tmp` asking with `/tmp`; a block whose directory does not
  decode asking only about the absolute path, with no `dir`; another host's block sending its host; output with no
  OSC 7 asking with no directory. `tsc` and prettier clean.
- **Remaining:** phase 3 (contracts and docs).

## 2026-11-16 02:22 +09:00 — Contracts and docs

- **Change:** WEB-46 says the links request's `dir` and `host`, paths resolved in `dir`, none from another host, and
  how the panel asks with a block's OSC 7 directory (or none), with decision 6's limit; WEB-49 that bash and zsh
  print OSC 7 before each prompt; their status table rows too. Conformance: `TestTerminalShellIntegration` checks
  OSC 7 naming `a b` (percent-encoded) from integrated bash and zsh and none from plain ones;
  `TestTerminalOutputLinks` a relative path found under `dir`, another host linking only the run, and a relative
  `dir` refused; `TestTerminalCommandDirsInBrowser` (new, its own file, as the shared shells' browser test) has bash
  print `sub/y.txt` in one directory, `cd /`, and the printed path still link to that file. docs/WEB.md replaces
  the caveat; docs/SECURITY.md says `dir` and that a false OSC 7 at worst links wrongly.
- **Reason:** plan phase 3.
- **Plan impact:** the browser step is a test of its own (`TestTerminalCommandDirsInBrowser`, bash with the
  integration) rather than a step in `TestTerminalPanelInBrowser`, whose shell is `/bin/sh` with a hand-made prompt
  and which another thread also changes.
- **Validation:** `go vet ./...`; `go test ./conformance ./conformance/05-web ./internal/terminal ./internal/webui`
  pass. The browser test skips here (no Chrome); its script passes `node --check`. `scripts/check.sh` not run (npm
  broken here; the full and race runs are too heavy for the shared machine).
- **Remaining:** CI's browser run.
