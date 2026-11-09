# Work log: Launching from where a shell is

## Steps 1 and 2: the launch on a shell, and the dialog

**Commits:** 2026-11-09 20:07:50 `4424402f`.

**Change:** In [launches.go](../../internal/webui/launches.go), `webLaunchRequest` takes `shell`, and the preview
reads it from the query; `launchStart` hands such a request to `shellLaunchStart`, which refuses any artifact field
as well, a server without the terminal, a shell not running, and a directory not found or not a directory, then
checks the command accepts that directory by kind and name and labels the launch with the shell's number, basedir,
and project. In [web_shell.js](../../internal/webui/assets/web_shell.js), `openLaunchDialog` opens a page dialog of
the overview's kind: it asks the terminal's preview where the shell is, lists the commands `/api/launchers` gives for
a directory of that name (unavailable ones off, with the reason), shows `/api/launch-preview` for the one chosen,
launches it, and follows a service until its Open link, or says why not; "Show in Launches" appears where the page
has the Launches list. **Launch here** in the bar (off with no shell shown) and a Launch icon on each All shells card
open it. WEB-38 and its row, [WEB.md](../../docs/WEB.md), and [SECURITY.md](../../docs/SECURITY.md) describe it.

**Reason:** [The plan](plan.md#decided).

**Plan impact:** No listing of the shell's directory, and no `child`, as decided. The dialog is the terminal's own,
since the artifact dialog is not on every page with the terminal.

**Validation:** `TestLaunchOnAShellsDirectory` (unit: the preview's target, working directory, and labels, the
launch's output, the refusals; conformance: the built binary, a directory with a space, `pwd -P` in the launched
command, a shell not running) and `TestTerminalPanel` (launching off, explained; the command list with one
unavailable; the preview's details; the launch request; a service followed to its Open link; a refusal; a card's
launch closing the overview; off with no shell).

**Remaining:** Trying it by hand with TensorBoard.
