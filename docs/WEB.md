# Web UI

<iframe src="https://www.youtube-nocookie.com/embed/UjoVz2AfFSw" title="Every project and past run in one Web UI" style="width: 100%; aspect-ratio: 16 / 9; border: 0" allow="encrypted-media; picture-in-picture; fullscreen" allowfullscreen></iframe>

`rotari web` serves a status UI for your runs: job state, logs, history across
projects, artifacts, and config. It is a separate process from `rotari run`.
Closing or stopping it never affects runs; it only stops the commands it
launched itself (see [Launching commands](#launching-commands)).

```sh
rotari web                  # http://127.0.0.1:<default port>, with job control
```

A [read-only demo](https://rotari-dev.github.io/rotari/) built from generated
data shows the UI without installing anything.

## Choosing a mode

Each mode adds capability, and with it, risk. Pick the smallest one that does
what you need.

| Mode | Command | What the browser can do | Token |
| --- | --- | --- | --- |
| Static export | `rotari web --static-dir DIR` | Browse a snapshot as plain files; nothing is served live. | n/a |
| Read-only | `rotari web --allow-control=false` | View everything the live server serves; control and file-write APIs return `403`. | Recommended beyond loopback |
| Control (default) | `rotari web` | Also copy, change, remove, cancel, and clear jobs, edit queues, and save config files. | Required beyond loopback |
| Terminal | `rotari web --allow-terminal` | Also open a shell on this host, as you, in a panel beside the jobs. | Always required |
| Launching | `rotari web --allow-exec` | Also run registered commands and built-in services against artifacts. | Always required |

- Job control does not start runs; execution starts only through `rotari run`, which the
  terminal can of course run.
- `--allow-exec` and `--allow-terminal` need job control, so neither can be combined with
  `--allow-control=false` or `--static-dir`.
- Options that only make sense for one mode, such as `--host` with
  `--static-dir`, are rejected rather than ignored.

## Options

The full list is in the [CLI reference](CLI_REFERENCE.md#rotari-web); the
environment variables are in
[Environment variables](ENVIRONMENT_VARIABLES.md). In short:

| Option | Purpose |
| --- | --- |
| `--host`, `--port` | Listen address; the default host is `127.0.0.1`. |
| `--auth-token` | Require a token. Prefer `ROTARI_WEB_AUTH_TOKEN` so it stays out of the process list. |
| `--allow-control` | Job control on (default) or `=false` for read-only. |
| `--allow-exec` | Let the UI launch registered commands; see below. |
| `--allow-terminal` | Add a terminal panel; see below. |
| `--terminal-idle-timeout`, `--terminal-max-shells`, `--terminal-max-shells-per-project`, `--terminal-replay-size`, `--terminal-shell` | With `--allow-terminal`, change its limits; see below. |
| `--allowed-host NAME` | With `--allow-exec` or `--allow-terminal`, accept one more `Host` header name (repeatable). |
| `--artifact-root DIR` | Also serve recorded artifact files under `DIR` (repeatable). |
| `--static-dir DIR` | Write a static export instead of serving. |
| `--static-artifact-contents` | With `--static-dir`, copy previewable artifact files into the export. |
| `--notifications` | Default state of the browser notification toggle. |

### Setting them in a config or the environment

Each option above can also go in a config file, at the top level or under `[web]`, or in its
environment variable, except `--artifact-root`. For example, in the global config
(`~/.config/rotari/config.toml`):

```toml
[web]
allow-terminal = true
terminal-max-shells = 16
```

`--allow-terminal`, `--allow-exec`, and `--allowed-host` let other people reach this host, so
`rotari web` reads them only from the command line, the environment, the global config, or a file
named with `--config`. With the terminal or launchers on, the same goes for `--host` and
`--auth-token`. A workspace (`.rotari.toml`), basedir, or project config may have been written by
someone else, or come with a cloned repository, so `rotari web` ignores its values for them and says
so when it starts:

```text
WARNING: ignoring allow-terminal = true in workspace config /work/.rotari.toml; rotari web reads it only from the command line, ROTARI_WEB_ALLOW_TERMINAL, the global config, or --config FILE
```

A value from a config or variable that does not parse, such as `terminal-max-shells = "lots"`, stops
`rotari web` with an error naming where it came from. With `allow-terminal` or `allow-exec` in the
global config or the environment, `--static-dir` is rejected, as it is for `host` there; use
`--config FILE` with a file without them for the export.

Settings in the Web UI cannot add, change, or remove `auth-token` in any config file; edit the file
directly for that.

## Reaching the UI

- **Same machine:** the default `127.0.0.1` binding needs no token unless you
  use `--allow-exec`. With one, sign in at `http://127.0.0.1:PORT` or
  `http://localhost:PORT` directly; plain HTTP is accepted for a browser on
  the same host.
- **Remote machine:** forward the port with `ssh -L PORT:127.0.0.1:PORT host`
  and keep the loopback binding. Launched services open through the same port,
  so no further forward is needed.
- **Shared network:** set a token and put an HTTPS reverse proxy in front.
  Browser sign-in is refused over plain HTTP that is not a direct loopback
  connection. Sign in with the user name
  `rotari` and the token as the password; the session lasts 12 hours. API
  clients send `Authorization: Bearer TOKEN` or `X-Rotari-Token: TOKEN`.

The token is authentication, not encryption. See
[Security model](SECURITY.md) before exposing the server, and
the [FAQ](FAQ.md#is-rotari-web-safe-to-expose-beyond-127001).

## What you can do

- **Runs and jobs:** state, attempts, logs, and reports for a run or selected
  jobs. A report is never sent anywhere; see the
  [FAQ](FAQ.md#does-the-web-ui-send-run-details-to-an-ai-service).
- **Several projects and state directories:** switch from the sidebar.
  **Activity** shows runs across every state directory on a calendar;
  **Recent jobs** lists unfinished jobs and jobs finished in the last hours or
  in a range you drag on a timeline of runs;
  **History search** searches project, run, and job history (live server only).
- **Control** (not in read-only mode): copy, change, remove, cancel, and clear
  jobs, and **Create queue** or **Append to queue**, which a separate
  `rotari run` executes. **Recover…** on an interrupted run does what
  `rotari unlock` does; see
  [Operations](OPERATIONS.md#shared-filesystem-use).
- **Artifacts:** preview and download what jobs wrote, open tables, and
  launch TensorBoard and other viewers on it; see [Artifacts](#artifacts).
- **Config:** **Generate config** and **View config** edit one config source at
  a time; see [Configuration](CONFIGURATION.md).
- **Notifications:** the in-page inbox and optional desktop popups; see
  [Notifications](NOTIFICATIONS.md#browser-notifications).
- **Theme:** `Settings` selects Light, Dark, or System.
- **Which rotari:** the foot of `Settings` shows the version of the `rotari web` serving the page and the path of
  its binary, with buttons to copy the path or send it to the terminal.

## Terminal

`rotari web --allow-terminal` adds a **Terminal** button to the top bar, which opens a shell in
a panel along the bottom of the page, so you can type while watching jobs or reading a log. Drag
the panel's top edge to change its height. The **Panel position** icon in its bar moves it down the left or
right of the window, for a tall shell beside the jobs; the browser remembers the choice, and in a window
under 1000 pixels wide the panel stays at the bottom. Panes stack when the panel is too narrow for columns.

**Window** in the same menu moves the whole panel, with its tabs, panes, and project list, into a browser
window of its own, for a second screen. The page gets all of its room back. The page and the window keep to one
project, as with the panel: moving to another project's page switches the window, and choosing a project in the
window takes the page there. Pages of no project, such as Recent jobs, leave the window as it is. While the window is open, the **Terminal** button brings it
to the front, the send buttons type into it, and **Open a terminal here** starts the shell there. Following
links in the page keeps the window. Close the window, or choose **Back to page** in its **Panel position** menu,
to put the panel back where it was. If the browser blocks pop-ups, the panel stays and says so. Reloading the
page while the window is open keeps the window, but a page opened in another browser tab takes its shells, as
another tab does today. Closing the browser tab closes its window too; the shells keep running for the next page
that opens the terminal.
The button shows how many shells are running in all
projects, as **Terminal (3)**, so you can see what is left running with the panel closed, and each project
in the sidebar with a shell running has a terminal icon; point at it to see how many.

The **All shells** icon beside **+** opens a dialog over the page, as a log does, listing every running shell,
grouped by project with the page's first. Under each project's name, a line says what its last run (the one
`rotari show` shows) is doing, such as `Last run running, started 12m ago · 3 in progress · 5 pending · 0 failed`,
counted as the run page's Jobs card counts them; click it to open that run. Each card shows its tab name, the directory its shell is in now, and
the command running in it, if any, and when it started and how long it has been idle (no output and no
typing); a tab's tooltip says the same. Next to the command, the card says how many processes the shell has
started, as `▶ make · 3 processes`, or `2 background processes` when nothing runs in front; click it to see each
one's process ID and command, indented under its parent. A process that left the shell, such as one started with
`nohup` or `setsid`, is not counted. Choose one with a click, or with the arrow keys and Enter, to close the dialog
and show that shell in the pane with the keys; one of another project takes the page there, as the project list
does. The pencil on a card renames its tab and the cross ends its shell, after asking; **F2** and **Delete** do
the same for the card with the keys, and the dialog stays open for the next one. **End all** beside a project's
name ends its shells, and **End all shells** at the top every shell, after asking. The **+** beside a project's name
starts another shell there, and **New shell in** under the cards starts one in a project that has none, or in no
project; each starts where that project's page would, and the new shell appears as a card without being shown
in a pane. The list is taken when the dialog
opens, and **Refresh** takes it again. Escape, **Close**, or a click
beside the dialog closes it. The directory is the shell's own, not that of a program running in it, and when the
server cannot find it the card says where the shell started instead. The command is the one in the foreground,
the first of a pipeline; a shell with no job control runs everything in its own process group, so its cards
show no command.

Shells belong to projects, as an editor's terminals belong to its window. A project's pages show
that project's tabs, and the list beside them goes to other projects that have shells. Other pages,
such as Recent jobs, show the shells of no project.

| Where you are | Where a new shell starts |
|---|---|
| A project's page | Where its latest run started |
| A run's page | Where that run started |
| Any other page | Your home directory |

A project's shells have `ROTARI_BASEDIR` and `ROTARI_PROJECT_NAME` set to it, so `rotari show` and the
like need no `-p`, wherever you `cd`. Give `-p` to reach another project. Other shells keep what
`rotari web` itself has.

**+** opens another shell in a tab of its own, up to eight per project and 32 in all, and each tab's
**×** ends its shell. A run's page also has **Open a terminal here** beside its working directory.
If the directory is gone or the project has no run, the shell opens in your home directory and its
first line says why. A shell that has started stays where it is when newer runs start.

Copy buttons have a send button beside them that types the same thing into the tab being shown, in the
focused pane if the panel is split, without running it: press Enter in the terminal to run it. A suggested command, such as how to run the queue,
goes in as one line with its comments and alternatives left out. A value such as a run ID or a path
goes in as one word, quoted if it needs to be, so it can finish a command you have started typing. It
goes to whatever is running there, such as `vim`. If the project has no shell, the button says so and
starts none. A log or other text of several lines goes in as pasted text, without colors and up to
256 KiB, and only where the program takes it as such, such as a recent `bash` or `zsh` prompt or an
editor; elsewhere each line would run, so the button says so and sends nothing.

The **Clear** icon empties the tab's screen and scrollback, and what would be shown again after following a
link. It sends nothing to the shell, so the prompt may not show again until you press Enter. It does nothing
while a full-screen program such as `vim` runs, and says so.

The panel shows up to four shells at once, such as an editor beside `rotari wait` and a prompt.
In the bar, the **Split right** icon adds a column of the whole height beside the focused pane's, and the
**Split down** icon a pane under it. Each shows a shell not yet in a pane, or a new one if there is none, and refuses if the panes
would get too small. Drag the handles between the panes to change their shares; in a narrow panel they all stack. The
pane with a colored top edge has the keys, and **Clear** and the send buttons act on it, while **A-**, **A+**, and
**Maximize** apply to the whole panel.

There is one row of tabs. Clicking a tab shows it in the focused pane, and a dashed tab is in another
pane: clicking it makes the two panes change places, since a shell cannot be in two. **+** starts a
shell in the focused pane. The **Close pane** icon closes the focused pane, and its shell keeps running in its tab;
closing a tab ends its shell, wherever it shows. When a pane's shell ends, the pane moves to a tab not
in a pane, or closes if there is none. Each browser tab keeps its own panes for each
project, and gets them back after following a link.

A tab you are not looking at is marked when it prints something, until you show it, and the list marks
other projects the same way. When a command finishes in a tab you are not looking at, such as a build or a
training run, the tab and its All shells card show a bell instead, until you show it; a command left running in
the background with `&` does not count. When a shell in another tab ends, a note says so. Signing out ends every shell, so
when anything runs in one, a command in front or processes left in the background, **Sign out** first lists those
shells and what they run, and asks. Closing a tab, or ending shells from All shells or a tab's menu, says how many
processes run in them and names a few before asking. These update every few
seconds while the panel is open and the page is in view.

Drag a tab to move it, or right-click it to move it, rename it, close the tabs to its right, close all of
this project's tabs, or close every shell of every project. All shells offers the last two as well: **End all**
beside a project's name and **End all shells** at the top. A new tab goes at the right end, and keeps the lowest number free. Names and places last
as long as their shells.

A web address in the output opens in a new tab when clicked. A program can print any address, so look
before you click, as in any terminal.

**A-** and **A+** change the text size, which the browser remembers.

The **Maximize** icon gives the panel the whole window, for programs such as `vim`, until you press it
again, now **Restore**. Each icon's name shows when you point at it.
The browser remembers it from page to page. Keep the panel small to reach the page's menus and dialogs.

Opening the panel starts a shell if the project has none. Following a link to another project with
the panel open starts nothing, so browsing leaves no shells behind; the panel offers to start one.

```sh
export ROTARI_WEB_AUTH_TOKEN=...      # optional; otherwise one is generated and printed
rotari web --allow-terminal
```

- **Who can use it:** anyone who can sign in gets a shell as you, on the host running
  `rotari web`. A token is always required, and it opens only on a direct `localhost` connection,
  such as an SSH forward, or over HTTPS. See the
  [security model](SECURITY.md#the-web-terminal).
- **Following links:** shells live on the server, so they carry on as you move between pages, and
  the panel shows recent output again. This includes full-screen programs such as `vim`.
- **Tabs you are not looking at:** their programs keep running. Switching to one shows up to its
  last 256 KiB of output again, as following a link does, not its whole scrollback.
- **Several browser tabs:** each browser tab shows one shell and remembers which. Two can use
  different shells at once. If one opens a shell another is using, it takes it, and the other says
  so and offers to take it back.
- **When a shell ends:** with its tab's **×** or `exit`, which moves to a neighboring tab of the
  project.
- **When all of them end:** when you sign out, when `rotari web` stops, or **3 hours after the
  last `rotari web` page closes**, which also ends what is running in their foregrounds. Any open page
  keeps them, even in the background with its panel closed. A computer that sleeps counts as closing
  its pages, and so does a sign-in that has expired. Use `rotari run` or `tmux` for long work: a run
  started in the terminal carries on regardless. To keep shells through a disconnect, see
  [below](#keeping-shells-through-a-disconnect).
- **Changing the limits,** on the command line, in a config, or in the environment (see
  [above](#setting-them-in-a-config-or-the-environment)):

  | Flag | Default | |
  | --- | --- | --- |
  | `--terminal-idle-timeout` | `3h` | How long the shells last after the last page closes. At least `2m`. |
  | `--terminal-max-shells` | `32` | Shells at once. |
  | `--terminal-max-shells-per-project` | `8` | Shells of one project at once. |
  | `--terminal-replay-size` | `256KiB` | Recent output kept per shell, up to `64MiB`. More uses more memory. |
  | `--terminal-shell` | `$SHELL`, or `/bin/sh` | The shell to run. |

- **Keys:** a browser keeps some keys for itself, such as Ctrl-W, which do not reach the shell.
- **Behind a reverse proxy:** the proxy must pass WebSocket connections to `/_terminal/ws`.

### Keeping shells through a disconnect

The shells belong to the `rotari web` process, not to your browser. To have them, and what runs in them,
outlast a closed laptop, a dropped connection, or a night away, keep that process alive and give it time:

1. **Run `rotari web` where it outlives your login,** such as in `tmux` or `screen` on the host. If you
   start it in an SSH session, it stops when that session ends, and every shell ends with it.

   ```sh
   tmux new -s rotari-web            # on the host; later, tmux attach -t rotari-web
   rotari web --allow-terminal --terminal-idle-timeout 24h
   ```

   Detach with Ctrl-B D and close the SSH session; the server, and its shells, keep running. Reach it
   again through your usual SSH forward.
2. **Make `--terminal-idle-timeout` longer than you will be away.** It counts from the last page closing,
   so a laptop that sleeps overnight needs more than the 3-hour default. It can also go under `[web]` in a
   config as `terminal-idle-timeout = "24h"`, or in `ROTARI_WEB_TERMINAL_IDLE_TIMEOUT` (see
   [above](#setting-them-in-a-config-or-the-environment)).

Even so, signing out, stopping `rotari web`, or restarting the host ends every shell, and a shell shows only
its last 256 KiB of output (`--terminal-replay-size`) when you come back. For work that must survive those,
run it with `rotari run`, or in `tmux` inside the shell.

## Artifacts

<iframe src="https://www.youtube-nocookie.com/embed/XlSJfi8HCCM" title="See what your runs wrote" style="width: 100%; aspect-ratio: 16 / 9; border: 0" allow="encrypted-media; picture-in-picture; fullscreen" allowfullscreen></iframe>

Rotari records the paths each job's definition names (arguments, `--env` and
matrix values, redirections, and paths in the config files and scripts it
reads), without running the job. A sentence that mentions a path, such as a
docstring or log message (spaces inside and a final period), is not taken
for one. Paths built at runtime are missed; declare
them with `rotari add --artifact PATH`, which may use
`$ROTARI_ARRAY_TASK_ID`, `$ROTARI_JOB_DIR`, and the job's variables.
`rotari show -j JOB --artifacts` lists them in a terminal.

In the Web UI, a job's **Artifacts** button lists them, while the job runs
too. Files can be previewed (images, audio, video, PDF, NumPy arrays, tables,
Markdown, common source/config formats, and other text) or downloaded, and
directories opened. Markdown rendering, syntax highlighting, and PDF previews
use the same local renderers in the artifact preview and Table viewer.
Markdown rendering and syntax highlighting stop at 2 MiB and show the source.
A file of another type, such as one without an extension, has **Open as text**.
The first row, `.`, is the
job's working directory, where trackers such as MLflow and Aim write by
default. The run toolbar's **Files** opens the experiment directory around
`.rotari-state`, where jobs can share one TensorBoard log directory or
tracker store.

Only files under the job's working directory or the directory holding the
basedir are served, and only when no other user can write the project's
state; `--artifact-root DIR` allows more. See
[Security model](SECURITY.md#artifact-files-and-static-exports).

### Launching commands

With `rotari web --allow-exec`, each row of Artifacts and **Files** has a
button per command that accepts it: TensorBoard, Jupyter Lab, MLflow, Aim,
Streamlit, Datasette, marimo, Panel, Gradio, VS Code, and the commands in your global `launchers.toml`
(`rotari config --launchers` writes a template). The UI shows what will run
before **Launch**; a service such as TensorBoard then opens through the Web
UI's own port, so it needs no extra SSH forward. **Launches** in the top bar
shows how many are starting, running, or stopping, and lists and stops them.
Closing the page does not stop a launch; stopping `rotari web` does.

```sh
export ROTARI_WEB_AUTH_TOKEN=...      # optional; otherwise one is generated and printed
rotari web --allow-exec
```

- A token is required even on `127.0.0.1`; sign in with the user name
  `rotari`.
- The browser only names a registered command and an artifact; the command
  text comes from `launchers.toml`, which rotari refuses unless no other user
  can write it.
- Commands run as you, with all your file permissions.

With `--allow-terminal` too, **Launch here** (the play icon) in the terminal's bar runs a command on the directory
the shell in the pane with the keys is in now, and each All shells card has the same button for its shell: pick a
command, check what will run, and **Launch**. It is the same as launching on an artifact, with the shell's
directory as the target.

Without `--allow-exec`, Artifacts says which commands a restart with it could
run on the artifact, including built-ins whose program is not installed, or
why the launcher file would stop it from starting.

See [Launcher commands](CONFIGURATION.md#launcher-commands) and
[Security model](SECURITY.md#launching-services-and-commands).

### Experiment trackers

Rotari does not read tracker metrics; it lets you open a tracker on what the
job wrote. Choose **Aim** on the `.` row, or open `mlflow.db` in it and choose
**MLflow (SQLite)**. For a store shared by the run's jobs, use **Files**.

To link a tracker run back to rotari, tag it with `ROTARI_RUN_ID`,
`ROTARI_JOB_ID`, and `ROTARI_ATTEMPT_ID` from the job's environment. Avoid one
SQLite `mlflow.db` written by many parallel jobs on NFS; use a file store,
one store per job, or a tracking server.

### Viewing tables

A table file (`.csv`, `.tsv`, `.jsonl`, `.scp`, and similar), a Markdown or
script file, a PDF, or a directory has a **Table viewer** button, available without
`--allow-exec`. It pages through
delimited and JSON Lines files and offers filters, sorting, and per-column
statistics (**Describe**) over the whole file. JSON Lines records are one JSON
value per line: object keys become sorted columns, and arrays or scalar values
go in a `value` column. Blank lines are skipped; malformed records are shown
as errors. A cell holding the path of an image, audio, video, or PDF file opens
it in a pane above the table (the table keeps its height, and the page scrolls),
with a waveform and spectrogram for audio, with grids and scales of time, amplitude,
and frequency (drag the bar between them to resize the waveform); **Channel** picks one channel of a
multichannel file to draw and to play from both speakers, or **Mix** to draw
their mean and play the file as it is. Markdown files use the same safe
renderer as run reports, with a **Show source** toggle (rendering is limited
to the first 2 MiB); source and config files supported by the bundled
Highlight.js build get syntax highlighting, including Python, shell,
JavaScript, TypeScript, Go, Rust, Java, C/C++, Ruby, JSON, and YAML. PDFs use
the browser's built-in viewer and can also
be opened in a new tab. For a
directory, **Group by stem** puts
`ref/u1.wav`, `hyp/u1.wav`, and `plots/u1.png` in one row to compare outputs
and find missing ones. In a directory, clicking any other file shows it in a
pane, rendering Markdown and highlighting supported source files; PDFs open
in the browser viewer. A table file also has **Open table**, which opens it in
its own viewer in a new tab. A directory row's **→** shows that directory in
the same window (Ctrl+click opens a new tab); the path above the table links
back to each directory up to the one the viewer was opened on, **↑** (or
Alt+↑) goes up one level, and the browser's Back returns. Files open in panes
stay open while you move, so `ref/u1.wav` and `hyp/u1.wav` can be compared
from their own directories. The URL keeps the view, so a link can be shared.

Filters include **matches regex** and **does not match regex**, which search a cell
with [Go's syntax](https://pkg.go.dev/regexp/syntax): `^(alpha|beta)$` selects either
value of a column, and `(?i)` ignores case. Lookaround and backreferences are not
available. They see the stored value rather than the formatted one, so a duration
shown as `1:30` is matched as `90`. All conditions must match.

**Describe** also shows each column's quartiles and most frequent values, and a
**Histogram** button that opens its distribution in a pane, where you can choose
10 to 200 bars. Numbers beyond 10,000,000 in the selected rows leave out quartiles
and histograms, and past 10,000 distinct values a column lists no top values; a filter
narrows both.

**Export rows** downloads every row the current filters select, across all pages and
in the current order, in the file's own format: a delimited file in its separator, a
JSON Lines file as its original lines, and a directory listing as CSV of what is
listed. Cells are written as they are, so a spreadsheet may read one starting with
`=` as a formula, as it would in the source file. A static snapshot has no export.

Each pane has a download button for the file showing.

To use another screen, drag a tab out of the window, or press the pane's **Open in a
new window**; if the browser blocks it, allow pop-ups for the page. Drag the tab back,
or press **Back to the table**, to return it. Closing that window closes its files. Row
moves change the file in the window you last clicked in, and arrow keys work from
either window.

To go through a table from the keyboard, press `↓` or `↑` to move between rows, in
the order shown and across pages, and `←` or `→` to move a frame between the row's
cells. Moving opens nothing. `Enter` opens the framed file, `Shift+Enter` opens all of
the row's files side by side, and `Space` plays or pauses the framed file, opening it
if needed. Nothing plays on its own.

The file showing in the active pane changes to its column of each new row, and says
so when that row has no file there. Other panes and hidden tabs stay, so one can be
kept to compare against; click a pane to make it the one that changes. Click a
player's controls and the arrow keys stay with it until you click elsewhere.

Try
it in the [Table viewer demo](https://rotari-dev.github.io/rotari/tableview/),
an evaluation table whose cells name WAV files; it is a static snapshot
(below), so filters are off there.

`rotari tableview PATH` opens the same viewer outside any project for a
delimited or JSON Lines table, a Markdown or script file, a PDF, or a directory:

```sh
rotari tableview metadata.csv
rotari tableview eval/ --port 8090
```

`rotari tableview --static-dir DIR FILE` writes a snapshot of one delimited or
JSON Lines table file
that you can publish or open without a server, as in the
[demo](https://rotari-dev.github.io/rotari/tableview/). It holds every row
(at most 10,000) and copies the image, audio, video, and PDF files the cells name (up to 10 MiB
each, 100 MiB in all), so share it only with readers who may see them.
Paging, sorting, **Describe** over the whole file, and the panes work;
filters can be changed only in the live viewer. Delimited tables also keep
their separator and header row fixed in the snapshot. Markdown, script, and
PDF previews are available in the live viewer, not as table snapshots. `DIR` is
replaced.

```sh
rotari tableview --static-dir site/eval eval/utterances.csv
```

## Static export

`rotari web --static-dir DIR` writes a snapshot you can publish or open
offline. It contains the raw config sources and saved config snapshots of the
runs, so share it only with readers you trust, and it holds artifact paths
only, unless `--static-artifact-contents` copies the files in. History search,
launching, and control are not available in it. See the
[Security model](SECURITY.md).

## Related

- [FAQ: Web UI](FAQ.md#web-ui)
- [CLI reference: `rotari web`](CLI_REFERENCE.md#rotari-web)
- [Security model](SECURITY.md)
