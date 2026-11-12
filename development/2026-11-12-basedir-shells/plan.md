# Plan: Shells of a basedir

Created: 2026-11-12
Status: Implemented (all three phases); see [work-log.md](work-log.md). The browser test
`TestTerminalPanelInBrowser` (WEB-33) was not extended, since it runs only in CI and could not be tried here; the
JSDOM test `TestTerminalPanel` covers the panel's side.

## Goal

With `--allow-terminal`, a basedir's own pages have shells of their own, as a project's pages do, for work across
the projects of one basedir. Today shells belong to a project or to no project, and the basedir's pages show the
shells of no project.

Shells of a run are not part of this plan; the user does not need them for now.

## Existing implementation

- **Groups.** `terminalGroup` in [terminal.go](../../internal/webui/terminal.go) keys a shell by
  `basedir + "\x00" + project`, or `""` for no project. `terminalShellResponse` splits the key back into the shell's
  basedir, project, and `link`, which is the project's page.
- **Place.** `terminalPlaceFor` reads `basedir`, `project`, and `run` from the query. With no project it returns the
  home directory and **ignores `basedir`**. With a project it starts where the latest or named run started, falling
  back to home with a `note` that becomes the shell's first line.
- **Environment.** A project's shell gets `ROTARI_BASEDIR` and `ROTARI_PROJECT_NAME` (`start.Env`, which
  `terminal.Keeper` appends to the server's own environment). Other shells keep `rotari web`'s environment without
  its token (`webChildEnvironment`). An empty `ROTARI_PROJECT_NAME` counts as unset (`internal/state/project.go`).
- **The basedir's pages.** In the sidebar, **All projects** links to `basedirURL(id, "/")` and **Recent jobs** to
  `basedirURL(id, "/jobs/")`: both are pages of one basedir. **Activity** and **History search** cover every basedir.
  `placeOfPath` in [web_shell.js](../../internal/webui/assets/web_shell.js) sends `basedir=ID` only for a mounted
  basedir (`/_basedir/ID/...`), and nothing for the current one.
- **Sidebar marks.** `markSidebarProjects` marks every `.sidebar-project-row` whose link matches a shell's `link`,
  skipping the basedir row and shells of no project ([sidebar shell mark plan](../2026-11-09-sidebar-shell-mark/plan.md)).
- **Workspace defaults.** `rotari init` makes `./.rotari-state`, and `defaultBaseDir` picks `./.rotari-state` in the
  current directory. A `.rotari.toml` (`config.WorkspaceFile`) may set `basedir`. Its value is used as written,
  resolved against the directory a command runs in.

## Decisions

1. **Three kinds of shell:** a project's, a basedir's, and no project's. A basedir's group key is
   `basedir + "\x00"`, which matches neither of the other two, since a project name is never empty. The limit per
   project (eight by default, `--terminal-max-shells-per-project`) applies to each basedir group too. The flag keeps
   its name; its help and the docs say that it covers a basedir's shells as well.
2. **Which pages show which shells:**

   | Page | Shells |
   |---|---|
   | A project's or a run's page | That project's (unchanged) |
   | **All projects** and **Recent jobs** of a basedir | That basedir's (new) |
   | **Activity**, **History search**, and anything else | No project's (unchanged) |

   This changes what opening the terminal on `/` or `/jobs/` does today: the shell becomes the basedir's.
   `placeOfPath` therefore has to name the current basedir too. The page asks for `basedir=` with no ID for the
   current basedir, which the server takes as `s.BaseDir`, as it already does for a project. A project's page of
   the current basedir still leaves `basedir` out. The panel decides a page's kind as the application script
   routes it (`pageOfPath`): `project/...` is a project's, `activity` and `search` are of no project, and anything
   else is the basedir's.

   Since `/` is now the current basedir's page, a shell of no project links to `/activity` instead of `/`, and
   **New shell in** offers no project as `/activity`.
3. **Where a basedir's shell starts:** in the basedir's parent directory, when that parent is a workspace;
   otherwise in the home directory. The parent is a workspace when either:
   - the basedir's name is `.rotari-state`; or
   - the parent holds a `.rotari.toml` whose `basedir`, resolved against the parent, is this basedir (compared after
     `filepath.Abs` and `Clean`).

   The default location (`~/.local/state/rotari`) and shared locations therefore open in home. Opening in home here
   is not a fallback, so no first line is printed. A parent that does not exist, or a `.rotari.toml` that does not
   parse, still opens in home, with a first line that says why, as a project's shell does.
4. **Environment:** `ROTARI_BASEDIR` is set to the basedir, and `ROTARI_PROJECT_NAME` is removed, so that a value
   in `rotari web`'s own environment does not pick a project. `terminal.StartOptions` gets a way to remove variables
   (such as `Unset []string`), rather than relying on an empty value counting as unset.
5. **Sidebar mark:** to the right of **All projects** in that basedir's section, the same icon and title
   ("2 shells in the terminal") as a project's row. The shell's `link` is `basedirURL(id, "/")`, so the existing
   match by link marks it once `markSidebarProjects` stops skipping shells with no project but a basedir. The
   basedir row itself stays unmarked.
6. **Everywhere else that groups shells:** All shells shows a basedir group (named by its path, with **+** and
   **End all**). **New shell in** offers each basedir besides its projects and no project. `/_terminal/projects`
   reports no last run for a basedir group. The window follows a basedir's page as it follows a project's page,
   so "pages of no project leave the window as it is" now means only Activity, History search, and similar pages.
   Tab layouts and the shown tab are keyed by basedir and project already (`":" + basedir + ":" + project`), so an
   empty project with a basedir gets keys of its own.

## Phases

1. **Server:** the place and group of a basedir (`terminalPlaceFor`, `terminalGroup`, `terminalShellResponse`),
   the workspace check, and the environment, with `Unset` in `internal/terminal`. Unit tests in `internal/webui` and
   `internal/terminal`.
2. **Panel:** `placeOfPath` for both kinds of basedir pages, the sidebar mark beside All projects, All shells,
   New shell in, and the window following basedir pages.
3. **Contracts and docs:** WEB-33 and WEB-34 in [contracts/README.md](../../contracts/README.md), the table and
   text of the Terminal section of [docs/WEB.md](../../docs/WEB.md), the flag's help, and
   [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md) ("groups them by basedir and project").

## Tests and validation

- Place: a basedir named `.rotari-state` starts in its parent. A basedir named in its parent's `.rotari.toml`, as an
  absolute path and as a relative one, starts in its parent. The default location and a `.rotari.toml` that names
  another basedir start in home without a first line. A missing parent starts in home with a first line.
- Environment: `ROTARI_BASEDIR` is the basedir, and `ROTARI_PROJECT_NAME` is absent even when `rotari web` has it.
- Groups: a basedir's shells count against their own group limit, apart from its projects' and no project's shells.
- Conformance: extend `TestTerminalShellsFollowTheirProject` (or add a test under WEB-34) through the binary.
  Extend `TestTerminalPanelInBrowser` (WEB-33) for All projects and Recent jobs showing the basedir's tabs, the mark
  beside All projects, and Activity still showing no project's shells. The browser test runs in CI only: there is no
  Chrome here.
- `go test ./internal/webui/ ./internal/terminal/ ./cmd/rotari/` and the conformance tests for WEB-33 and WEB-34.

## Coordination

Other workspaces touch the terminal (`terminal-split`, `web-terminal-cli`, `web-terminal-config`). Before
implementing, check which of them hold work in progress on `terminal.go` or `web_shell.js`, and base this work on
`main` after they land, or agree on an order.
