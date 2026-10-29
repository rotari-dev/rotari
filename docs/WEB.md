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
| Launching | `rotari web --allow-exec` | Also run registered commands and built-in services against artifacts. | Always required |

- Job control does not start runs; execution starts only through `rotari run`.
- `--allow-exec` needs job control, so it cannot be combined with
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
| `--allowed-host NAME` | With `--allow-exec`, accept one more `Host` header name (repeatable). |
| `--artifact-root DIR` | Also serve recorded artifact files under `DIR` (repeatable). |
| `--static-dir DIR` | Write a static export instead of serving. |
| `--static-artifact-contents` | With `--static-dir`, copy previewable artifact files into the export. |
| `--notifications` | Default state of the browser notification toggle. |

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
  `rotari run` executes.
- **Artifacts:** preview and download what jobs wrote, open tables, and
  launch TensorBoard and other viewers on it; see [Artifacts](#artifacts).
- **Config:** **Generate config** and **View config** edit one config source at
  a time; see [Configuration](CONFIGURATION.md).
- **Notifications:** the in-page inbox and optional desktop popups; see
  [Notifications](NOTIFICATIONS.md#browser-notifications).
- **Theme:** `Settings` selects Light, Dark, or System.

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
too. Files can be previewed (images, audio, video, NumPy arrays, tables,
text) or downloaded, and directories opened. A file of another type, such
as one without an extension, has **Open as text**. The first row, `.`, is the
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
Streamlit, and the commands in your global `launchers.toml`
(`rotari config --launchers` writes a template). The UI shows what will run
before **Launch**; a service such as TensorBoard then opens through the Web
UI's own port, so it needs no extra SSH forward. **Launches** in the top bar
lists and stops them. Closing the page does not stop a launch; stopping
`rotari web` does.

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

A table file (`.csv`, `.tsv`, `.scp`, and similar) or a directory has a
**Table viewer** button, available without `--allow-exec`. It pages through
files with millions of rows and offers filters, sorting, and per-column
statistics (**Describe**) over the whole file. A cell holding the path of an
image, audio, or video file opens it in a pane above the table (the table keeps
its height, and the page scrolls), with a
waveform and spectrogram for audio; **Channel** picks one channel of a
multichannel file to draw and to play from both speakers, or **Mix** to
draw their mean and play the file as it is. For a directory, **Group by stem** puts
`ref/u1.wav`, `hyp/u1.wav`, and `plots/u1.png` in one row to compare outputs
and find missing ones. In a directory, clicking any other file shows it as
text in a pane, and a table file also has **Open table**, which opens it in
a Table viewer of its own in a new tab. The URL keeps the view, so a link can be shared. Try
it in the [Table viewer demo](https://rotari-dev.github.io/rotari/tableview/),
an evaluation table whose cells name WAV files; it is a static snapshot
(below), so filters are off there.

`rotari tableview PATH` opens the same viewer outside any project:

```sh
rotari tableview metadata.csv
rotari tableview eval/ --port 8090
```

`rotari tableview --static-dir DIR FILE` writes a snapshot of one table file
that you can publish or open without a server, as in the
[demo](https://rotari-dev.github.io/rotari/tableview/). It holds every row
(at most 10,000) and copies the media files the cells name (up to 10 MiB
each, 100 MiB in all), so share it only with readers who may see them.
Paging, sorting, **Describe** over the whole file, and the panes work;
filters, the separator, and the header row can be changed only in the live
viewer. `DIR` is replaced.

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
