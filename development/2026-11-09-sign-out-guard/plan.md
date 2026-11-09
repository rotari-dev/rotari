# Plan: Asking before signing out ends running commands

Created: 2026-11-09
Status: Done, not yet committed or tried by hand. See the [work log](work-log.md).

## Goal

Signing out ends every shell (WEB-34). When a command is running in one, the page asks first, naming the shells
and what they run, so that a training run or a build is not ended by a click meant only to sign out.

## Proposals

1. **The shell list says what runs now:** `running`, read from the terminal's foreground process group as the list
   is made, which costs one `ioctl` per shell.
2. **The Sign out form asks first** on a page with the terminal: it lists the shells, and when any is running, names
   up to five with the command each runs (from the preview), and asks; declining keeps the session. With nothing
   running it signs out at once. When the shells cannot be listed it says so and asks, rather than signing out
   unasked.
