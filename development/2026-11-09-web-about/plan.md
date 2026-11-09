# Plan: Which rotari serves the Web page

Created: 2026-11-09
Status: Step 1 done in `875abb62`, not yet tried by hand; proposals 3 and 4 dropped. See the
[work log](work-log.md). The user asked for the path of the rotari running `rotari web`, and its version, to be visible
somewhere in rotari, without knowing where it would fit best.

## Goal

Someone looking at a Web page can tell which rotari binary serves it and which version that is, for instance after
upgrading or with several installations side by side.

## Existing implementation

- **The CLI says it:** `rotari version` prints `versionString()` from [version.go](../../cmd/rotari/version.go):
  the release version, or for a dev build `dev` and the VCS commit, `-dirty` if modified.
- **The Web UI says nothing** of either. [webui.Options](../../internal/webui/options.go) has no version or path.
- **The top bar's Settings menu** is on the application and Recent jobs pages
  ([web_template.html](../../internal/webui/assets/web_template.html),
  [jobs_template.html](../../internal/webui/assets/jobs_template.html)): Theme, then the config buttons that
  [web_app_core.js](../../internal/webui/assets/web_app_core.js) adds.
- `os.Executable` is already used to find the running binary (`executable_identity.go`, `server.go`,
  `web_launch.go`). On Linux, a binary replaced since it started reads with ` (deleted)` after its path.

## Proposals

1. **At the bottom of the Settings menu,** under a line: `rotari v0.1.15` (or `rotari dev (abc123def456-dirty)`)
   and the binary's path in monospace, with a copy button. The menu is where a page's own settings and configs
   already are, on both pages, and it keeps the top bar as it is.
2. **The server passes them to the page** as `rotari web` starts: `cmd/rotari` sets `Options.Version`
   (`versionString()`) and `Options.Executable` (`os.Executable`, or the error saying why it is not known), and
   the pages get them with their other values. No request is needed.
3. **A binary replaced since it started** is said: when the path no longer holds the file the server started from,
   the menu says "replaced since this server started; restart rotari web to use the new one". This needs the
   identity taken at start (as `executableIdentity`), compared when the page loads.
4. **The API:** `GET /api/about` returns `{"version", "executable"}` for scripts and the Python client, behind the
   same token as the rest.
5. **The static export** gets neither: it has no server, and its pages say which rotari wrote them nowhere today.

## Decided

On 2026-11-09 the user agreed to the Settings menu (proposal 1), asked for the path to have a send button beside its
copy button, and dropped proposal 3 (few users rebuild rotari under a running server) and proposal 4 (asked what it
was for: scripts and the Python client checking a server's version, which nothing needs now, and `rotari version`
answers locally).

## Steps

1. The values in `Options`, the menu's section on both pages with copy and send buttons, the contract (WEB-37),
   `docs/WEB.md`, and tests.
