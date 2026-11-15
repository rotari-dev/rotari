# Plan: Relative paths in the Web terminal's output, from where each command ran

Created: 2026-11-16
Status: Accepted by the user on 2026-11-16, with the open questions answered as proposed; phase 1 is done. See
[work-log.md](work-log.md).

## Goal

With `--allow-terminal`, a relative path printed by a command links to what it named where that command ran, not
where the shell is when the pointer reaches it. Today `ls -d src/*`, then `cd /tmp`, leaves `src/main.go` linking to
`/tmp/src/main.go`, or to nothing. Shells say their directory at each prompt (OSC 7, as VS Code, kitty, WezTerm, and
GNOME Terminal read it), the panel keeps it with each block, and the server resolves a block's relative paths
against it.

This is the caveat left in the [output links plan](../2026-11-14-terminal-output-links/plan.md) (decision 4:
"Blocks, with the directory each command ran in, could fix that later"), chosen on 2026-11-16 from the remaining
terminal candidates.

## Existing implementation

- **Links** (WEB-46). The panel picks out candidates on the hovered line (`linkCandidates`) and asks
  `POST /_terminal/shells/ID/links` (`askLinks`), caching answers per shell and text, paths for 5 seconds. The server
  (`terminalLinkFinder.path` in `internal/webui/terminal_links.go`) joins a relative path to `Keeper.Dir`, the
  shell's directory now (WEB-52's current shell).
- **Blocks** (WEB-48). The panel reads OSC 133 into blocks: a prompt (`A`), its command line (`B`), the command (`C`),
  and its end (`D;exit`); `blockAt(pane, line)` finds a buffer line's block. Replay rebuilds them from the last
  256 KiB the server keeps.
- **Shell integration** (WEB-49, WEB-53). rotari's `marks.bash` and `marks.zsh` print `D` and `A` before each prompt
  and `C` with a percent-encoded `cmdline_url=`; inner bash and zsh load them too. fish 4 prints OSC 133 itself.
- **OSC 7** is `ESC ] 7 ; file://HOST/PATH BEL`, with the path percent-encoded. Many prompts and distributions
  already print it (vte.sh, fish, starship's option); nothing in rotari reads it.

## Decisions

1. **The integration says the directory.** `marks.bash` and `marks.zsh` print OSC 7 just before `D`/`A` at each
   prompt: `file://` and the host (`$HOSTNAME`, `$HOST`), then `$PWD` percent-encoded byte by byte with the encoder
   `cmdline_url=` already uses. Inner shells get it as they get the marks. A shell or prompt that prints OSC 7 itself
   is read the same way.
2. **The panel keeps it with each block.** Each pane reads OSC 7 (`registerOscHandler(7)`) as it passes, with replay
   too, into the pane's directory now; a block takes the directory the pane had at its `A`. The directory is the
   percent-decoded path when it decodes to UTF-8 and is absolute; one that does not is kept as *unknown*, not
   dropped, so a block after it does not take an older directory.
3. **Links ask with the block's directory.** For a line in a block with a directory, the panel sends
   `{"candidates": [...], "dir": "/abs/path"}`; the server resolves that request's relative paths against `dir`
   instead of the shell's directory now, and absolute ones as before. A line in a block whose directory is unknown
   links no relative path (absolute ones still link). A line in no block, or a block from a shell that says no
   directory, keeps today's behavior: the shell's directory now. Answers are cached per shell, directory, and text.
4. **Another host's directory.** A block whose OSC 7 names a host other than the server's (after `ssh`, which passes
   OSC 7 through when the far shell prints it) links no paths at all, relative or absolute, since they are the far
   host's; run and job IDs still link. The panel sends `host`; the server compares it with `os.Hostname()`: equal
   ignoring case, equal up to the first dot, empty, or `localhost` counts as this host. (See open question 2.)
5. **The server checks `dir`.** It must be absolute, at most 4 KiB, and is only joined and stat'ed, as candidates
   are: the terminal's user already runs a shell as that user, so this reads nothing new. A relative `dir` is a 400.
6. **Commands that move.** A command line that changes directory itself (`cd sub && ls -d */`) prints relative to
   where it went, which no mark says; its block keeps the prompt's directory, and the docs say so.
7. **Contracts.** WEB-49 adds OSC 7 to what the integration prints, and WEB-46 how a relative path is resolved; no
   new ID, since both rules already have theirs.

## Phases

1. **Integration and server.** OSC 7 in `marks.bash` and `marks.zsh`; `dir` and `host` on the links request. Tests:
   a real bash and zsh printing OSC 7 with a directory holding a space and a non-ASCII name; the links handler with
   `dir` (a relative file found under it and not under the shell's directory), a relative `dir` (400), another host
   (no paths, IDs still), and without `dir` as before.
2. **Panel.** The OSC 7 handler, each block's directory (unknown when undecodable), `askLinks` with the block's
   directory and host, and the cache key. JSDOM tests: a path printed, then `cd`, linking where it was printed; a block
   with an unknown directory linking only absolute paths; a block from another host linking no paths; output without
   OSC 7 as before; replay rebuilding the directories.
3. **Contracts and docs.** WEB-46 and WEB-49 wording, conformance through the binary (`TestTerminalIntegration...`
   for OSC 7 and the links route with `dir`), a `TestTerminalPanelInBrowser` step (print `sub/file`, `cd /`, the link
   still opening the file), docs/WEB.md (the caveat replaced, decision 6), and docs/SECURITY.md (`dir`).

## Tests and validation

- Go: as in phase 1, in `internal/terminal` and `internal/webui`.
- JSDOM: as in phase 2, in `TestTerminalPanel`.
- Conformance and browser (CI only) as in phase 3.
- `go test ./internal/terminal ./internal/webui ./conformance/05-web` with Node 24, `tsc`, and prettier.

## Non-goals

- Following directory changes inside one command line (decision 6).
- Using OSC 7 for the Explorer's Follow or Recent directories, which read the current shell's directory from the
  host (WEB-45, WEB-52).
- Opening a remote host's paths (decision 4).
- fish and other shells beyond reading the OSC 7 they print.

## Settled questions

All three were accepted as proposed on 2026-11-16.


1. **Unknown directory.** Proposed: a block whose OSC 7 does not decode links no relative path (decision 2). The
   alternative is to fall back to the shell's directory now, as today, at the risk of a wrong link.
2. **Another host.** Proposed: no path links in a block from another host (decision 4). The alternative is to keep
   linking absolute paths there as today, which open this host's file of that name.
3. **Lines in no block.** Proposed: today's behavior, the shell's directory now (decision 3). The alternative is to
   use the latest OSC 7 seen before the line even without blocks, for a prompt that prints OSC 7 but no OSC 133.

## Coordination

Implementation starts from `main` in the `terminal-blocks` workspace and is rebased onto `main` before integrating.
