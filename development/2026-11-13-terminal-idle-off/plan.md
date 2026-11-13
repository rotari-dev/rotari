# Plan: Shells last as long as the server by default

Created: 2026-11-13
Status: Proposed. To follow [a run-style client for rotari web](../2026-11-13-web-client/plan.md), which makes the
server's lifetime the user's choice.

## Goal

By default, a Web terminal shell runs until it is ended, the user signs out, or `rotari web` stops. No timer ends
it. `--terminal-idle-timeout` stays for anyone who wants one.

## Decisions (2026-11-13)

- **No idle limit by default.** The limit made up for `rotari web` running for days with no clear owner, under tmux.
  With the run-style client, the server stops on Ctrl-C or a closed terminal and stays only when Ctrl-D or
  `--detach` asks for it. `--shutdown` and the Settings button stop it, and sign-out still ends every shell. A timer
  on top of that adds a second lifetime to explain, and it has a real cost: a laptop back from sleep finds vim or a
  long command gone. That cost is why the default went from 30 minutes to 3 hours in
  [2026-11-10](../2026-11-10-terminal-idle-default/plan.md).
- **The risk of a long-lived privileged shell is accepted.** A shell that ran `sudo -s` or holds an unlocked
  ssh-agent is worth more to someone with the token than a new shell would be. But a 3-hour limit does not
  materially reduce that, and sign-out and Stop server end such shells at once.
- **The flag stays,** so configs and environments that set it keep working, and a limit can still be chosen.

## Existing implementation

- `webTerminalIdleLimit` is 3 h and `webTerminalMinIdleLimit` is 2 min ([web_terminal.go](../../cmd/rotari/web_terminal.go)).
  `newWebTerminalSettings` rejects values below the minimum.
- `terminal.NewKeeper` rejects `IdleLimit <= 0`. `armIdleTimer` starts the timer when nothing is attached to any
  shell, and `/_terminal/alive`, posted each minute by every open page, holds it off.
- WEB-34, WEB.md (terminal section, the limits table, "Keeping shells through a disconnect"), FAQ, SECURITY.md, and
  the flag's description all state the 3-hour default.

## Proposals

1. **`--terminal-idle-timeout` defaults to `0`, meaning no limit.** Any other value must still be at least 2 minutes;
   negative values are errors. The same holds for `terminal-idle-timeout` in a config and for
   `ROTARI_WEB_TERMINAL_IDLE_TIMEOUT`. `0` given explicitly also means no limit.
2. **`terminal.KeeperOptions.IdleLimit` of 0 means no timer.** `armIdleTimer` does nothing then. `/_terminal/alive`
   keeps answering, so pages need no change.
3. **Startup message** ([web-client](../2026-11-13-web-client/plan.md) proposal 7). With no limit, the line reads
   `Shells end when the server stops or you sign out.` With a limit, it gives the duration as that proposal says.
4. **Docs and contracts.** WEB-34 says shells end on sign-out, server stop, or the idle limit when one is set. WEB.md's
   "Keeping shells through a disconnect" no longer asks for a longer timeout. The limits table shows `0` (none) as
   the default. The FAQ and SECURITY.md describe the default as no limit, and the flag's description and the
   generated views follow.

## Steps

1. Keeper: `IdleLimit` 0 disables the timer; unit tests that shells outlive a period with nothing attached, and that
   a positive limit still applies.
2. CLI settings: the default and validation; unit tests for 0, 1m (refused), 2m, and negative values, from the flag,
   the environment, and a config.
3. Conformance: WEB-34's idle-limit test runs with an explicit limit; a new case checks that with the default, a shell
   with nothing attached is still running after a period. Its length is chosen so the test is short while still
   showing that no short default exists.
4. Startup message, docs, contracts, generated views, work log.

## Risks

- Someone relying on the 3-hour default to clean up forgotten shells loses it silently on upgrade. The release notes
  and the startup message say how to set a limit.
