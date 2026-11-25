# Plan: Choose the editor that files open in

Created: 2026-11-25
Status: Agreed on 2026-11-25, with the user confirming decisions 1 and 2. Both phases are
implemented, and have not been tried in a browser.
Related: [Open in VS Code](../2026-11-24-open-in-vscode/plan.md)

## Purpose

**Open in VS Code** serves one editor, under an icon (`<>`) that means any. Other
editors open a file, at a line, over SSH, by a URL of the same kind. Let each
browser choose its editor; the action and its icon then say what they do.

Nothing of Open in VS Code has been released, so names may still change freely.

## Scope

- One editor per browser, VS Code unless another is chosen, used wherever Open in
  VS Code is today: the Explorer, printed paths, the terminal's bar, All shells
  cards and pane menu, Artifacts, **Files**, and job logs.
- The editors below, with the same positions, connection, and checks.

## Non-goals

- A command, or a URL a user writes (decision 3).
- Several editors offered at once.
- Detecting installed editors, which a browser does not tell a page.
- JetBrains IDEs: their URL (`jetbrains://IDE/navigate/reference?project=&path=`)
  needs the name of a project open in the IDE, and no documented one opens a file
  over SSH.
- The built-in launchers, which run editors in the browser.

## Editors

| Editor | This machine | Over SSH | Evidence, read on 2026-11-25 |
| --- | --- | --- | --- |
| VS Code | `vscode://file/PATH:L:C` | `vscode://vscode-remote/ssh-remote+NAME/PATH:L:C` | its source; SSH tried by the user |
| VSCodium | `vscodium://`, as VS Code | same | its `prepare_vscode.sh` sets the scheme; SSH needs an extension such as Open Remote - SSH |
| Cursor | `cursor://`, as VS Code | same | its forum only; reports that a remote link opens another window each time |
| Zed | `zed://file/PATH:L:C` | `zed://ssh/NAME/PATH:L:C` | its source (`open_listener.rs`); whether a line is kept over SSH is not certain |

VS Code's source (`electron-main/app.ts`) confirms what Open in VS Code relies on:
a remote path that ends in `:NUMBER` is a file and any other a folder, and
`?windowId=_blank` asks for a window of its own. It also asks before opening a file
through a URL until its setting `security.promptForRemoteFileProtocolHandling`
says not to.

The user tried Zed on 2026-11-25: the URL started it and it went to connect to the
server, then could not fetch its remote server program. That request is made by the
Zed on the user's machine, and the host answers from this network only through a
proxy, so it is taken for a setting of Zed's and not of the URL.

Windsurf is left out: nothing read confirms its scheme. It, or another, is one row
to add once its URL is known.

## Decisions

1. **Which editors** (confirmed): the four above. VS Code Insiders was among them and
   went in first; the user then took it out as too rarely used. Only VS Code can be tried by the
   user today. Documentation says which were tried and which follow the editor's
   source or reports, and an editor found not to work is removed, not worked around.
2. **Where it is chosen** (confirmed): in the dialog that asks for the connection,
   as a list over it, and, as the user asked, in Settings itself, as a list that
   takes effect at once. The first open still asks once.

   ```text
   Open in editor
     Editor  [ VS Code       v ]
     ( ) It runs on the same machine as rotari web
     (•) Connect over SSH
         SSH host: [ ks000        ]

                            [Cancel]  [Open]
   ```

3. **A fixed list, no custom URL.** The choice is kept in the browser's storage,
   which a launched service on the same origin can write; a URL template there
   could hold `javascript:` or another site's address. An unknown editor in the
   storage opens the dialog with the reason.
4. **What is kept.** The editor per browser, since it is of the browser's machine;
   the connection per server host name, as today. The keys are new, `rotari-editor`
   and `rotari-editor-host-HOST`, which no host's name can make alike, so a
   connection kept before this is asked for once more; none was released.
5. **Labels name the editor:** **Open in Cursor**, **Open in Zed**, and for VS Code
   **Open in VS Code**, the wording of Microsoft's rules. Before one is chosen it is
   VS Code. Buttons already drawn change their labels when the choice does, in other
   tabs too. Settings says **Editor** and **Editor connection...**.
6. **The icon stays `<>`** for all of them; no editor's own mark is used.
7. **Positions and refusals.** A VS Code family path with `:NUMBER` in it is refused,
   as today. Zed reads a position from the end of a path too, so the same rule holds.
   A directory takes `?windowId=_blank` in the VS Code family only; a remote file
   without a line takes `:1` there only.
8. **Names in the code** become the editor's: `web_editor.js`,
   `GET /api/editor-target`, `#editor-connection`, `openInEditor`. WEB-64 is
   reworded, WEB-65 and the others that name it follow.

9. **None** (asked for by the user on 2026-11-25) is in the list of Settings, not of the
   dialog, and hides the action everywhere in the browser. It takes the place of the
   **Show Open in editor** check box, which read as a setting for one icon alone and not
   as having no editor to use. Its key, `rotari-editor-hidden`, is no longer read; it was
   never released. Choosing which icons are shown is another matter, and not this plan.

## Variants to cover

| Dimension | Values |
| --- | --- |
| Editor | each of the four, None, nothing kept, unknown in the storage |
| Target | file with and without a position, directory, each refused name |
| Connection | same machine, SSH, `USER@HOST`, refused name, storage refused |
| Place | every place that offers it, drawn before the choice changed, hidden |
| Page | application page, Recent jobs, terminal window, another tab, static export |

## Phases

1. **Rename** in the code, contracts, and documents, with one editor.
2. **The list:** the table of editors and their URLs, the dialog, labels that
   follow the choice, and the documents.

## Contracts and documentation

WEB-64, WEB-65, WEB-46, WEB-51, WEB-42; WEB, SECURITY, ARCHITECTURE, and the plan of
Open in VS Code, which keeps its name as a record.

## Tests and validation

- Each editor's URL for each target and connection; the kept choice old, new, and
  unknown; labels before and after a change, and in another tab.
- Conformance: the renamed route and element.
- By hand: VS Code again. Each other editor when someone has it, and Zed's line over
  SSH first.
