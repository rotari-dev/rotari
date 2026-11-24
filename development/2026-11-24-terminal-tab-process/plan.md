# Plan: The foreground program's name in terminal tabs

Created: 2026-11-24
Status: Agreed on 2026-11-24, with the user confirming decisions 1 to 3. Both phases
are implemented; the tabs have not been seen in a real browser.
Related: [web terminal](../2026-11-05-web-terminal/plan.md),
[command bell](../2026-11-09-command-bell/plan.md),
[nested shells](../2026-11-15-terminal-nested-shells/plan.md)

## Purpose

A tab shows a number, or the name someone gave it, and since `e6653cb4` a triangle
while a command runs in its shell. With several commands running, the triangles do
not say which tab is which: finding the one with `vim` means opening **All shells**
or each tab. The tab should say what runs in it.

## Existing implementation

- `GET /_terminal/shells` gives `running` for each shell: something other than an
  interactive shell leads the terminal's foreground process group
  (`Session.atPrompt`). To tell, it has already read that leader's arguments
  (`processArgs`), and throws them away.
- `GET /_terminal/shells/ID/preview` gives `command`, the whole command line, to
  the All shells cards, one request per shell.
- A page refreshes the list every five seconds (`shellsRefresh`), and after actions
  on shells. A tab is drawn again only when something it shows changes.
- A tab's label is cut at 160 pixels.

## Scope

- The program's name beside a tab's number or name while a command runs in the
  shell's foreground, in the panel and in its own window, for a project's own tabs
  and the shared ones.
- The same name in the tab's tooltip and accessible label.

## Non-goals

- A title a program sets (OSC 0 and 2), which this panel does not read.
- Background jobs, and what runs inside `ssh`, `tmux`, or `screen`.
- The sidebar, the page's title, and All shells cards, which show the whole command
  already.
- Renaming a shell by what runs in it. A given name stays as it is.

## Decisions

1. **What the tab says** (confirmed):

   | Shell | At its prompt | Running |
   | --- | --- | --- |
   | Unnamed, number 1 | `1` | `▶ 1 python` |
   | Named `train` | `train` | `▶ train python` |

   The program is in the muted colour after the number or name. The number stays,
   since messages and dialogs call the shell by it. The triangle stays, since a
   program whose name is not known still runs.
2. **The program's name, not its command line** (confirmed): the last path element
   of its first argument, so `/usr/bin/python3 train.py` is `python3`. Arguments
   make a tab long, and may hold secrets that a list read every five seconds should
   not carry. For the whole command there is All shells.
3. **A script's interpreter is the program**: `./train.py` run by its `#!` line is
   `python3`, as the process table has it. Showing the script instead needs each
   interpreter's options to find it, which is guessing. Two tabs running Python look
   the same, and a given name tells them apart (confirmed).
4. **A pipeline shows its first program**, as the command on All shells does. A
   shell script run by hand shows its shell, such as `bash`; an interactive shell
   typed at a prompt is a prompt, as it is today.
5. **When it changes.** At the list's refresh, as the triangle does, so up to five
   seconds late, and a shorter command may never show. Refreshing at a command's
   start (OSC 133 `C`) would change the tabs at each `ls`; not proposed.
6. **Width.** The program is cut with an ellipsis after 12 characters' width and
   the label keeps its 160-pixel limit, the name giving way first, so that a long
   name does not hide the program. The bar wraps when full, as it does today.
7. **Unknown names.** When the arguments cannot be read or are empty, the list has
   no program and the tab keeps its triangle alone. The name is cut to 64 bytes at a
   character's edge, and what is not valid UTF-8 or is a control or bidirectional
   formatting character becomes U+FFFD: a program chooses its own first argument.
   The page sets it as text.
8. **Login shells.** A leading `-` (`-zsh`) is left off.
9. **No setting.** The name appears only while something runs.

## Design

- `internal/terminal`: `atPrompt` also gives the arguments it read;
  `ShellInfo.Program` is the name by decisions 2, 7, and 8, set where `Running` is,
  in `Keeper.Shells`. No further read of the process table is needed.
- `internal/webui/terminal.go`: `program` beside `running`, left out when empty.
- `web_shell.js`: `drawTab` adds a `terminal-tab-program` span, and `program` joins
  the values that decide whether tabs are drawn again. The tooltip says "Running
  python3." and the label ", running python3", in place of "A command is running."
  and ", command running".
- Dragging a tab, renaming it, and `shellName` keep using the name and number only.

## Variants to cover

| Dimension | Values |
| --- | --- |
| Shell | at its prompt, running, stopped job, interactive shell inside, shell script, pipeline |
| Program | with a path, a `#!` script, a login shell's, long, empty arguments, control characters, not UTF-8 |
| Tab | named, unnamed, shown, in another pane, not shown with a bell or a dot, shared, being dragged |
| Place | panel at the bottom and at a side, its own window, another project's page |
| System | Linux, macOS, one whose processes are not known |

## Phases

1. The name in `internal/terminal` and the list, with tests.
2. The tab, its tooltip and label, and the styles.

Both are one change to ship.

## Contracts and documentation

WEB-34 (the list's `program`), WEB-33 (what a tab shows), and
[docs/WEB.md](../../docs/WEB.md), where the triangle is described. The Open in VS
Code work in progress edits the same files; start after it is committed.

## Tests and validation

- `internal/terminal`: the name from each kind of arguments; a running command, a
  pipeline, a script, and an interactive shell inside, in a real shell.
- `TestTerminalPanel`: each tab variant, redrawing only on a change, and a name that
  looks like markup.
- Conformance through the binary: `program` while a command runs and none at the
  prompt.
- By hand: tabs at the bottom and at a side with long names, and how far tabs move
  when commands start and end.
