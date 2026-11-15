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
