# Plan: Marking the sidebar's projects that have shells

Created: 2026-11-09
Status: Done in `fe15da58`. The user asked for an icon in the left sidebar by each project with a shell
open, and left the details to this plan.

## Goal

With `--allow-terminal`, each project in the sidebar that has a shell running shows a small terminal icon, so that
shells left running are visible without opening the panel, from any page.

## Existing implementation

- **The sidebar is drawn by the application script** (`renderSidebar` in
  [web_app_core.js](../../internal/webui/assets/web_app_core.js)), again with every refresh, by `innerHTML`. A
  project's row is `.sidebar-project-row`, holding a link to `basedirURL(id, "/project/" + name)`.
- **The panel's script knows every shell** ([web_shell.js](../../internal/webui/assets/web_shell.js)), listed
  every five seconds on a page in view, with the panel open or not, for the count on the Terminal button. Each shell's
  `link` is its project's page, made on the server the same way as the sidebar's link: no prefix for the current
  basedir, `/_basedir/<id>` for another.
- **Long project names are cut with an ellipsis** inside the link, so a mark inside it would be cut too.

## Decisions

1. **The panel's script marks the rows,** after each list of shells and whenever the sidebar is drawn again (a
   `MutationObserver` on its child list). The application script needs no change, and a page with no terminal has no
   marks.
2. **Matching by link:** a row is marked when its link's decoded path is a shell's decoded `link`. Shells of no project
   mark nothing.
3. **The icon is the one of Open a terminal here,** drawn by `web_shell.css` after the link on the row, so it is never cut.
   Its title says how many shells, as "2 shells in the terminal".
4. **Only project rows.** A collapsed basedir does not show its projects' marks; the Terminal button's count still
   tells of them.

## Tests and validation

- `TestTerminalPanel`: rows of projects with shells are marked with their count, others not; a row drawn again is
  marked again; a shell ending removes the mark.
- By hand: both themes, a long project name, the active project's row.

## Done

`markSidebarProjects` in web_shell.js and the row's `::after` in web_shell.css; WEB-33 and docs/WEB.md say so.
`go test ./internal/webui/` passed. Not looked at in a browser: there is no Chrome here.
