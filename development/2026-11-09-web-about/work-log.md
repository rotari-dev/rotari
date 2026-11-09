# Work log: Which rotari serves the Web page

## Step 1: the Settings menu

**Commits:** Not yet committed.

**Change:** `webui.Options` has `About` (`Version`, `Executable`, `ExecutableError`), which `webAbout` in
[web.go](../../cmd/rotari/web.go) fills from `versionString()` and `os.Executable` for `rotari web` only, so a static
export has none. `livePage.about` carries it to the live application and Recent jobs pages, and `aboutSection` in
[assets.go](../../internal/webui/assets.go) writes it at the foot of both Settings menus through `__ROTARI_ABOUT__`:
the version, then the path, cut at its start when long, with `writeJobsCopyButton`, or the reason it is not known.
`copyJobsValue` moved from the Recent jobs template into [web_shell.js](../../internal/webui/assets/web_shell.js), so
the application page has it too, and `addJobsSendButtons` gives the copy button its send button on both pages, the
path going as one word. [web_shell.css](../../internal/webui/assets/web_shell.css) draws the section and its buttons,
since the application page lacks the Recent jobs page's styles. WEB-37 and its row, the page frame paragraph, and
[WEB.md](../../docs/WEB.md) describe it.

**Reason:** [The plan](plan.md#decided). The user asked on 2026-11-09.

**Plan impact:** Proposals 3 and 4 dropped.

**Validation:** `TestSettingsSayWhichRotariServes` (unit: both pages, escaping, inside the menu, an unknown path,
nothing without `About`, and the live handlers; conformance: the built binary's `rotari version` and path on `/`
and `/jobs/`), `TestTerminalPanel` (copy and send beside the path on both pages), and
`TestJobsPageCopyButtonShowsFeedback` with the moved function. `go test ./conformance -run Contract` passed. Not
looked at in a browser.

**Remaining:** Trying it by hand.
