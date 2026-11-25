# Web UI

<iframe src="https://www.youtube-nocookie.com/embed/UjoVz2AfFSw" title="Every project and past run in one Web UI" style="width: 100%; aspect-ratio: 16 / 9; border: 0" allow="encrypted-media; picture-in-picture; fullscreen" allowfullscreen></iframe>

`rotari web` serves a UI for your runs: job state, logs, history across
projects, artifacts, and config. It is a separate process from `rotari run`:
stopping it never affects runs, only its [terminal](#terminal)'s shells and the
commands it [launched](#launching-commands).

```sh
rotari web                  # http://127.0.0.1:<default port>, with job control
rotari web --allow-exec     # also launch TensorBoard and other viewers
rotari web --allow-terminal # also a terminal panel
```

On a remote machine, forward the port:

```sh
# on the remote machine
rotari web --allow-exec --allow-terminal
# on your machine
ssh -L 8787:127.0.0.1:8787 user@remote
```

The [read-only demo](https://rotari-dev.github.io/rotari/) needs no
installation.

## Choosing a mode

Each mode adds capability, and with it, risk. Pick the smallest one that does
what you need.

| Mode | Command | What the browser can do | Token |
| --- | --- | --- | --- |
| Static export | `rotari web --static-dir DIR` | Browse a snapshot as plain files; nothing is served live. | n/a |
| Read-only | `rotari web --allow-control=false` | View everything the live server serves; control and file-write APIs return `403`. | Recommended beyond loopback |
| Control (default) | `rotari web` | Also copy, change, remove, cancel, and clear jobs, edit queues, and save config files. | Required beyond loopback |
| Terminal | `rotari web --allow-terminal` | Also open a shell on this host, as you. | Always required |
| Launching | `rotari web --allow-exec` | Also run registered commands and built-in services against artifacts. | Always required |

Job control does not start runs; only `rotari run` does. `--allow-exec` and
`--allow-terminal` can be combined, but not with `--allow-control=false` or
`--static-dir`.

## Options

See the [CLI reference](CLI_REFERENCE.md#rotari-web) and
[Environment variables](ENVIRONMENT_VARIABLES.md).

### Stopping and detaching

At a terminal:

- **Ctrl-C** stops the server.
- **Ctrl-D** detaches it: it keeps running in the background, as with
  `--detach`. Stop it with `rotari web --shutdown`.
- **Closing the terminal**, SIGHUP, SIGTERM, or killing `rotari web` follows
  `--disconnect-action`: `stop` (default) or `detach`. In a config, set it under
  `[web]`; the top-level `disconnect-action` and `ROTARI_DISCONNECT_ACTION` are
  for runs.

There is one detached server per user and host. Running `rotari web` again at a
terminal with the same options attaches to it and shows its log; Ctrl-D or
closing the terminal then leaves it running, and Ctrl-C stops it. With different
options, it starts a second server.

Without a terminal, as from a script or a service, and with the default `stop`,
it serves in its own process and SIGTERM stops it.

### Setting them in a config or the environment

Each option except `--artifact-root` can also go in a config file, at the top
level or under `[web]`, or in its environment variable. For example, in
`~/.config/rotari/config.toml`:

```toml
[web]
allow-terminal = true
terminal-max-shells = 16
```

`allow-terminal`, `allow-exec`, and `allowed-host` are read only from the
command line, the environment, the global config, or a file named with
`--config`; so are `host` and `auth-token` when the terminal or launchers are
on. A workspace (`.rotari.toml`), basedir, or project config may have been
written by someone else, so its values for them are ignored with a warning.

With `allow-terminal` or `allow-exec` in the global config or the environment,
`--static-dir` is rejected, as it is for `host` there; use `--config FILE` with
a file without them for the export.

Settings in the Web UI cannot change `auth-token` in a config file; edit the
file directly.

## Reaching the UI

- **Same machine:** the default `127.0.0.1` binding needs no token unless you
  use `--allow-exec` or `--allow-terminal`.
- **Remote machine:** keep the loopback binding and forward the printed port
  (8787 unless it is taken) with `ssh -L`. Launched services use the same port.
- **Shared network:** set a token and put an HTTPS reverse proxy in front;
  sign-in is refused over plain HTTP except on a direct loopback connection.
  With `--allow-exec` or `--allow-terminal`, allow its `Host` name with
  `--allowed-host`. The terminal needs WebSocket connections to
  `/_terminal/ws`.

Sign in with the user name `rotari` and the token as the password; the session
lasts 12 hours. API clients send `Authorization: Bearer TOKEN` or
`X-Rotari-Token: TOKEN`. Where a token is always required, one is generated and
printed unless you set it. Prefer `ROTARI_WEB_AUTH_TOKEN` to `--auth-token`,
which shows in the process list.

The token is authentication, not encryption. See
[Security model](SECURITY.md) before exposing the server, and
the [FAQ](FAQ.md#is-rotari-web-safe-to-expose-beyond-127001).

## What you can do

- **Runs and jobs:** state, attempts, logs, and reports. A report is never
  [sent anywhere](FAQ.md#does-the-web-ui-send-run-details-to-an-ai-service).
- **Several projects and state directories:** switch from the sidebar, or look
  across them with **Activity**, **Recent jobs**, and **History search** (live
  server only). `rotari gc` unregisters a state directory marked **missing**.
- **Control:** change jobs, and create or append to a queue, which a separate
  `rotari run` executes. **Recover…** on an interrupted run does what
  `rotari unlock` does; see
  [Operations](OPERATIONS.md#shared-filesystem-use).
- **Files:** [artifacts](#artifacts), [tables](#viewing-tables), and
  [your editor](#opening-files-in-your-editor).
- **Config:** see [Configuration](CONFIGURATION.md).
- **Notifications:** see
  [Notifications](NOTIFICATIONS.md#browser-notifications).
- **Stickies:** notes on a page, kept in the state directory, so every browser
  sees them. They are separate from run notes and change nothing about runs.
  `rotari stickies` lists them from the command line.
- **Settings:** theme, editor, **Stop server** (with job control), and the
  version and path of the running `rotari`.

## Terminal

<iframe src="https://www.youtube-nocookie.com/embed/nObEULGw63k" title="Shells that belong to your experiments" style="width: 100%; aspect-ratio: 16 / 9; border: 0" allow="encrypted-media; picture-in-picture; fullscreen" allowfullscreen></iframe>

`rotari web --allow-terminal` adds a **Terminal** panel: tabs, up to four split
panes, a file Explorer, and, from its **Panel position** menu, a separate
browser window.

Anyone who can sign in gets a shell as you, on the host running `rotari web`.
It opens only on a direct `localhost` connection, such as an SSH forward, or
over HTTPS. See the [security model](SECURITY.md#the-web-terminal).

### Shells and projects

Shells belong to projects: a project's pages show that project's tabs. A state
directory also has shells of its own, for work across its projects, which its
projects' pages show under **Shared across projects**. Turn off **Link terminal
and project** to keep the terminal's project while you visit other pages.

| Where you are | Where a new shell starts |
|---|---|
| A project's page | Where its latest run started |
| A run's page | Where that run started |
| A state directory's All projects or Recent jobs | The workspace it belongs to, otherwise your home directory |
| Activity or History search | Your home directory |

A state directory belongs to a workspace when it is named `.rotari-state` or
when the `.rotari.toml` beside it names it as `basedir`.

`ROTARI_BASEDIR` and `ROTARI_PROJECT_NAME` are set in a project's shells, so
`rotari show` needs no `-p`, wherever you `cd`. A state directory's own shells
have only `ROTARI_BASEDIR`.

**All shells** lists every running shell, with its directory and command. Drag a
card to another project to move the shell there; a command already running keeps
the old project. Only bash and zsh with
[shell integration](#command-blocks-and-shell-integration) can move.

### Typing into a shell

Send buttons, **Recent directories**, **Commands**, and the Explorer type into
the focused pane without pressing Enter. Values are quoted if needed. Send
buttons type into whatever runs there, such as `vim`; the two lists type only at
a prompt. Text of several lines (up to 256 KiB) is sent only to a program that
takes pasted text as such, since elsewhere each line would run.

**Recent directories** and **Commands** are kept in memory only, until every
shell ends.

### Explorer

The tree follows the shell's directory while **Follow** is on.

- Nothing overwrites a file or folder of the same name.
- Delete cannot be undone.
- A move to another file system is refused, and folders cannot be copied; use
  `mv` and `cp -r` in the shell.
- A folder downloads as a zip without its symbolic links. Folders cannot be
  uploaded.
- A name that is not valid UTF-8 offers no actions, and one with a control
  character cannot be sent to the terminal.
- A folder shows its first 2,000 entries.

### Links in output

Run IDs, job IDs, and paths that exist become links, as do `file://` (OSC 8)
links and web addresses. A name without a slash is a link only with a line
number, such as `train.py:12`. Job IDs are looked up in the project's last 20
runs and its queue. Right-click a path for more actions.

A relative path is taken from the directory of the prompt (OSC 7) its command
ran from, so after `cd sub && ls`, paths in `sub` are not found. Paths printed
by another host over `ssh` are not links when its shell reports its directory.

### Command blocks and shell integration

In bash and zsh, each command's output is a block, with a dot for how it ended:
point at a command, click its dot, or right-click to copy or fold it, or
type it again.
**Open transcript** exports commands and output as Markdown, HTML, or a shell
script. Blocks and transcripts cover the last 8 MiB the page received.

A command that ends out of sight marks its tab, and one of at least 30 seconds
also sends a [desktop notification](NOTIFICATIONS.md#terminal-commands).

rotari starts bash and zsh with its own startup files, which read yours and then
add marks (OSC 133). Pass `--terminal-shell-integration=false` for plain login
shells. Other shells have blocks if they send the marks themselves, as fish 4
does.

- **bash before 4.4** has no marks if your files set a `DEBUG` trap.
- **Shells started inside a tab:** bash and zsh are marked too, but not inside
  `tmux`, `screen`, or `ssh`, whose directory changes are not followed either.
  For this, rotari exports `PROMPT_COMMAND` and `ZDOTDIR` naming its files in
  `~/.cache/rotari/shell-integration`, so programs that read `$ZDOTDIR` see that
  directory.
- **A `.bashrc` that replaces `PROMPT_COMMAND`** leaves an inner bash unmarked.
  Add this at its end (for zsh, `marks.zsh` in `~/.zshrc`):

  ```bash
  [ -r "$ROTARI_SHELL_INTEGRATION/marks.bash" ] && . "$ROTARI_SHELL_INTEGRATION/marks.bash"
  ```

  One that appends a hook after rotari's can leave the first prompt unmarked.
- Any program can print the marks, so a file shown with `cat` can draw false
  ones. They change only what the page shows.

### Keys, fonts, and clipboard

- Some keys, such as Ctrl-W, stay with the browser. On a Mac, **Options** >
  **Use Option as Meta** gives `M-` keys.
- **Options** > **Font...** uses a font installed on the browser's computer,
  such as a Nerd Font.
- Right-click opens the terminal's menu; Shift+right-click the browser's.
- **Options** > **Let programs copy** lets `tmux`, Neovim, and other programs
  set your clipboard (OSC 52, up to 1 MiB), through `ssh` too. It is off by
  default: with it on, anything printed, including a file or a log, can replace
  the clipboard. Programs cannot read it.

For programs inside `tmux`, add `set -s set-clipboard on` to `~/.tmux.conf`. In
Neovim, `vim.g.clipboard = 'osc52'` alone makes putting from `+` wait 10 seconds
for an answer that does not come; use this in `init.lua`, with which putting
gives what Neovim last copied:

```lua
local osc52 = require('vim.ui.clipboard.osc52').copy('+')
local last = { { '' }, 'v' }
local function copy(lines, regtype)
  last = { lines, regtype }
  osc52(lines)
end
local function paste()
  return last
end
vim.g.clipboard = {
  name = 'OSC 52, copy only',
  copy = { ['+'] = copy, ['*'] = copy },
  paste = { ['+'] = paste, ['*'] = paste },
}
```

### How long shells last

Shells run on the server, so they carry on as you move between pages, close the
panel, or disconnect. They end when you close their tab, sign out, or stop
`rotari web`.

- A page that reconnects, or a tab you return to, shows only recent output.
- A page that cannot keep up lets go of the shell at 8 MiB of pending output and
  reconnects once in sight. The program is not slowed.
- Two browser tabs cannot show the same shell; the second takes it, and the
  first offers to take it back.

| Flag | Default | |
| --- | --- | --- |
| `--terminal-idle-timeout` | `0` (none) | End the shells this long after the last page closes; at least `2m`. A sleeping computer or an expired sign-in counts as closed. |
| `--terminal-max-shells` | `32` | Shells at once. |
| `--terminal-max-shells-per-project` | `8` | Shells of one project, or of one state directory's own pages, at once. |
| `--terminal-replay-size` | `256KiB` | Recent output kept in memory per shell, up to `64MiB`. A pane shows at most its last 10,000 lines. |
| `--terminal-shell` | `$SHELL`, or `/bin/sh` | The shell to run. |
| `--terminal-shell-integration` | `true` | Mark commands in bash and zsh. |

### Keeping shells through a disconnect

Shells last as long as the `rotari web` process. To keep them through a closed
laptop or a dropped connection, press Ctrl-D before you go, or start it detached
or so that a disconnect detaches it:

```sh
rotari web --detach --allow-terminal
rotari web --disconnect-action detach --allow-terminal
```

Leave `--terminal-idle-timeout` unset, or longer than you will be away.

**Finding it again:** run the same command. `--detach` prints the running
server's URL, or fails naming the options that differ. After an upgrade, it
notes that the server runs another binary; stop it and start it again to use the
new one. A generated token is not shown again, so keep it or set
`ROTARI_WEB_AUTH_TOKEN`.

**Stopping it:** `rotari web --shutdown`, or **Stop server** in Settings. Both
end its shells and ask first when anything runs in them; without a terminal,
`--shutdown` then needs `--yes`.

Signing out, stopping `rotari web`, or restarting the host ends every shell. For
work that must survive signing out or a stopped server, use `rotari run`, or
`tmux` inside the shell.

## Artifacts

<iframe src="https://www.youtube-nocookie.com/embed/XlSJfi8HCCM" title="See what your runs wrote" style="width: 100%; aspect-ratio: 16 / 9; border: 0" allow="encrypted-media; picture-in-picture; fullscreen" allowfullscreen></iframe>

Rotari records the paths each job's definition names (arguments, `--env` and
matrix values, redirections, and paths in the config files and scripts it
reads), without running the job. Paths built at runtime are missed; declare
them with `rotari add --artifact PATH`, which may use
`$ROTARI_ARRAY_TASK_ID`, `$ROTARI_JOB_DIR`, and the job's variables.

A job's **Artifacts** button lists them, also while the job runs, as does
`rotari show -j JOB --artifacts`. The first row, `.`, is the job's working
directory. The run toolbar's **Files** opens the experiment directory around
`.rotari-state`, where jobs can share one TensorBoard log directory or tracker
store. Markdown rendering and syntax highlighting stop at 2 MiB.

Only files under the job's working directory or the directory holding the
basedir are served, and only when no other user can write the project's
state; `--artifact-root DIR` allows more. See
[Security model](SECURITY.md#artifact-files-and-static-exports).

### Launching commands

With `rotari web --allow-exec`, each row of Artifacts and **Files** has a button
per command that accepts it: TensorBoard, Jupyter Lab, MLflow, Aim, Streamlit,
Datasette, marimo, Panel, Gradio, VS Code for the Web, and the commands in your
global `launchers.toml` (`rotari config --launchers` writes a template).
Services open through the Web UI's own port. With `--allow-terminal` too,
**Launch here** in the terminal runs one on the shell's directory.

Closing the page does not stop a launch; stopping it under **Launches** or
stopping `rotari web` does.

- The browser only names a registered command and an artifact; the command
  text comes from `launchers.toml`, which rotari refuses unless no other user
  can write it.
- Commands run as you, with all your file permissions.

See [Launcher commands](CONFIGURATION.md#launcher-commands) and
[Security model](SECURITY.md#launching-services-and-commands).

### Opening files in your editor

**Open in VS Code** opens a server's file or directory in the editor on the
machine your browser runs on. It is offered for artifacts and files, in the
terminal, and for paths in a job's **Log**, at their line. The browser hands the
editor a URL such as `vscode://` and the server runs nothing, so it works on any
live server, read-only or not. It is not the **VS Code for the Web** launcher.

Choose another editor, or **None** to hide it, in **Settings** > **Editor**:

| Editor | Checked |
| --- | --- |
| VS Code | opened over Remote SSH, a file at its line and a directory |
| VSCodium | not tried; it takes VS Code's URLs under a scheme of its own, and needs an SSH extension such as Open Remote - SSH |
| Cursor | not tried; its users report that a file over SSH opens another window each time |
| Zed | starts and connects over SSH; opening a file there, and its line, not seen yet |

For **Connect over SSH**, give the host name your editor already connects with;
VS Code may open another window for another name of the same host, and needs its
Remote - SSH extension. For another port, use a `Host` alias in `~/.ssh/config`.

- The page is not told whether the editor opened anything, so a wrong name
  shows as an error in the editor only.
- Without `--allow-terminal`, only Log paths whose files **Artifacts** may show
  are links. They are looked up on the server's host.
- A path with a number after a colon, such as `a:12`, is refused, since the
  editor would take it for a line.

### Experiment trackers

Rotari does not read tracker metrics; it lets you open a tracker on what the
job wrote. Choose **Aim** on the `.` row, or open `mlflow.db` in it and choose
**MLflow (SQLite)**. For a store shared by the run's jobs, use **Files**.

To link a tracker run back to rotari, tag it with `ROTARI_RUN_ID`,
`ROTARI_JOB_ID`, and `ROTARI_ATTEMPT_ID` from the job's environment. Avoid one
SQLite `mlflow.db` written by many parallel jobs on NFS; use a file store,
one store per job, or a tracking server.

### Viewing tables

The **Table viewer** opens tables (`.csv`, `.tsv`, `.jsonl`, `.scp`, and
similar), Markdown, scripts, PDFs, and directories, without `--allow-exec`. A
cell that names an image, audio, video, or PDF file opens it in a pane. Try the
[demo](https://rotari-dev.github.io/rotari/tableview/), a static snapshot.

- **Filters** must all match. The regex ones use
  [Go's syntax](https://pkg.go.dev/regexp/syntax), without lookaround or
  backreferences, and see the stored value: a duration shown as `1:30` is `90`.
- **Describe** covers every row the filters select. Quartiles and histograms are
  left out past 10,000,000 numbers, and top values past 10,000 distinct ones.
- **Export rows** writes the selected rows in the file's own format, or a
  directory as CSV. Cells are written as they are, so a spreadsheet may read one
  starting with `=` as a formula.
- **Group by stem** in a directory puts `ref/u1.wav`, `hyp/u1.wav`, and
  `plots/u1.png` in one row, to compare outputs and find missing ones.
- **JSON Lines:** object keys become sorted columns; arrays and scalars go in a
  `value` column. Malformed records are shown as errors.
- **Audio and video:** **Channel** picks one channel to draw and play; **Mix**
  draws their mean and plays the file as it is.
- **Sharing:** the URL keeps the view.
- **Another screen:** drag a tab out of the window; allow pop-ups if the browser
  blocks it.

| Key | |
| --- | --- |
| `↑` `↓` | Move between rows; the active pane changes to the new row's file |
| `←` `→` | Move between the row's cells |
| `Enter` | Open the framed file, or the directory |
| `Shift+Enter` | Open all of the row's files side by side |
| `Space` | Play or pause |
| `Alt+↑` | Go up one directory |

Arrow keys stay with a player once you click its controls; click elsewhere to
get them back.

`rotari tableview` opens the same viewer outside any project, and with
`--static-dir` writes a snapshot of one delimited or JSON Lines file:

```sh
rotari tableview metadata.csv
rotari tableview eval/ --port 8090
rotari tableview --static-dir site/eval eval/utterances.csv
```

A snapshot holds at most 10,000 rows and copies the media files its cells name
(up to 10 MiB each, 100 MiB in all), so share it only with readers who may see
them. Its filters are fixed and it has no export. `DIR` is replaced.

## Static export

`rotari web --static-dir DIR` writes a snapshot you can publish or open
offline. It contains the raw config sources and saved config snapshots of the
runs, so share it only with readers you trust. It holds artifact paths only,
unless `--static-artifact-contents` copies the previewable files in. History
search, launching, and control are not available in it. See the
[Security model](SECURITY.md).

Options of the live server are refused with `--static-dir`, except that an
`auth-token` from config or `ROTARI_WEB_AUTH_TOKEN` is ignored with a warning.
The same holds for `rotari tableview --static-dir`.

## Related

- [FAQ: Web UI](FAQ.md#web-ui)
- [CLI reference: `rotari web`](CLI_REFERENCE.md#rotari-web)
- [Security model](SECURITY.md)
