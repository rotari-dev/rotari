# Promo video pipeline

These scripts make rotari's videos from real runs: rotari, its Web UI, and a
one-node Slurm cluster run in Linux namespaces, and a headless browser
renders the material into frames for ffmpeg. Every command, log line, and Web
UI screen in a video comes from those runs. They are a production aid, not
part of rotari or its tests.

All videos have English narration by Kokoro TTS (voice `af_heart`), burned-in
subtitles, and an `.srt` beside the video, and no music. Every video's main
version is landscape (1920×1080): the overview for the README and Getting
started, and one feature video per documentation page, linked from that page.
Vertical (1080×1920) versions are only for YouTube Shorts, made from a
landscape video's story; so far the overview has one, `01_short.sh`. A Short's
description links are not clickable, so set its Related video to the landscape
version.

## Episodes and documentation

| # | Video | Page it accompanies | Script | State (2026-10-25) |
| --- | --- | --- | --- | --- |
| 01 | Overview: one job needs no rotari, many do | README, Getting started | `01_overview.sh` (Short: `01_short.sh`) | done, 78 s (96 s with the Slurm act), landscape and vertical |
| 02 | Notifications: desktop, Inbox, Slack | `docs/NOTIFICATIONS.md` | `02_notifications.sh` | done, 35 s |
| 03 | Outputs: Artifacts, previews, TensorBoard, Files | `docs/OUTPUTS.md` | `03_outputs.sh` | done, 53 s |
| 04 | Rerunning failures and diagnosis | `docs/RECOVERING.md`, `docs/LOCAL_DIAGNOSIS.md` | `04_recovering.sh` | done, 47 s |
| 05 | Local and Slurm | `docs/EXECUTORS.md` | `05_executors.sh` | done, 37 s |
| 06 | Arrays, matrices, dependencies | `docs/RUNNING.md` | `06_grid.sh` | done, 44 s |
| 07 | The Web UI across projects, Activity | `docs/WEB.md` | `07_webui.sh` | done, 34 s |
| 08 | Inspecting runs from a terminal: jobs, show, logs | `docs/INSPECT.md` | `08_terminal.sh` | done, 44 s |
| 09 | Coding agents: what yours did, and why | `rotari guide`, `docs/MCP.md`, `docs/INSPECT.md` | `09_agents.sh` | done, 39 s |

`docs/OUTPUTS.md` was split out of `docs/INSPECT.md` so that video 03 has one
page to accompany.

## The Short's story (01_short.sh)

The vertical Short, and the overview before it became a tour (see
[Overview (01)](#overview-01)), tell this story.

### Message

Rotari makes the experiment cycle (write code, submit jobs, check how they
ended, fix and resubmit failures, inspect results) less tedious. With one job a
terminal is enough, so the video opens by saying so: rotari pays off when
something becomes plural.

- **Places:** projects in different directories. Without rotari, one terminal
  tab each and `cd` to check; with rotari, every project of a state directory
  is on one Web UI page.
- **Numbers:** a 48-run sweep where 8 runs fail for two reasons (a GPU fault
  and CUDA out of memory). Rotari groups failures by diagnosed cause with a fix
  each; after the batch size is fixed, a plain `rotari retry` reruns only the
  8 failures while the 40 successful runs carry forward. Retrying one cause
  (`--filter-diagnosis`) was tried and dropped as too specialized for a first
  video.
- **Machines (optional act):** Slurm and local jobs are submitted and watched
  differently by hand; with rotari only `-e local` / `-e slurm` differs, and
  one run can hold both.

Claims stay within what rotari does: diagnosis covers known log patterns only,
the Web UI reads one basedir at a time (other basedirs are in the sidebar), and
rotari is not a workflow engine. The retry succeeds because the fixture's
failures really are fixed: the GPU fault happens on the first attempt only, and
the OOM depends on `max_batch` in `fixture/sweep.conf`.

### Storyboard

Each scene's on-screen caption and narration line (`say`, with `sayAt` when
it should not start right away) are in `render.py` (`timeline()`); that file
is the source of truth. The lines were translated from a reviewed Japanese
draft, summarized here:

| Time | Scene | Narration |
| --- | --- | --- |
| 0:00 | `python train.py` in one terminal: "One job?" | One job? Watching it in a terminal is enough. |
| 0:06 | Title: "You don't need rotari. / But research isn't one job." | You don't need rotari. But research is never just one job. |
| 0:11 | Four tabs (`asr`, `tts`, `vision`, `nlp`) with `tail`; vision has crashed | Each project lives in its own directory… something broke. |
| 0:18 | `cd` + `rotari add -p NAME` + `rotari run --async` in each directory | With rotari, you submit from any directory, |
| 0:26 | Web UI All projects, live: vision turns failed | and every project shows up on one page… |
| 0:34 | `rotari add --array 1-48 ./sweep.sh`, `rotari run --async` | Now a parameter sweep. Forty-eight runs. |
| 0:41 | Run page live: failures appear | Eight of them fail. |
| 0:49 | Two log lines: GPU fault, CUDA OOM | For different reasons. Do you open every log? |
| 0:53 | Run page Failure causes: three groups, fix and retry command each | Rotari groups the failures by cause, and suggests a fix for each. |
| 0:58 | `sed -i 's/max_batch=128/max_batch=64/' sweep.conf`, `rotari retry -p llm-sweep` | Fix the batch size, and rerun only the failed jobs… |
| 1:06 | Retry run page: 8 rerun, 40 carried, 48 succeeded | All forty-eight done. |
| (opt.) | Slurm (`sbatch`/`squeue`/`cat`) above local (`nohup`/`ps`/`tail`) | Preprocessing runs locally, training on Slurm… |
| (opt.) | `rotari add -e local …` and `-e slurm --depends-on …`; run page jobs table with workstation and gpu-node01 | …Only the executor changes. One run, two machines. |
| 1:10 | All projects again | More jobs. Still one place to look. |
| 1:13 | Logo and GitHub URL | Rotari. Spend your time on research, not on babysitting jobs. |

## Narration

`render.py --narration KOKORO_DIR` (which `make-video.sh` passes) speaks each
line with [Kokoro-82M](https://huggingface.co/hexgrad/Kokoro-82M) through
`kokoro-onnx` on the CPU, about 2 s per line. `narration.py` caches each clip
in `WORK/capture/narration/` by text, voice, and speed, so editing one line
synthesizes only that line. A scene shorter than its line is lengthened; the
clips are mixed into one track normalized to -1 dBFS and muxed as AAC. Each
line is also burned in below the frame and written to `OUT.srt`.

`--voice` picks another Kokoro voice (`af_heart` is the default; `am_michael`,
`bf_emma`, and `bm_george` are among the others) and `--speed` changes the
pace. Nobody has listened to the result in this pipeline's checks: review the
pronunciation of "rotari" and the CLI terms by ear before publishing.

## Files

| File | Role |
| --- | --- |
| `01_overview.sh`, `01_short.sh`, `02_notifications.sh`, `03_outputs.sh`, `04_recovering.sh`, `05_executors.sh`, `06_grid.sh`, `07_webui.sh`, `08_terminal.sh`, `09_agents.sh` | Make one video each into `out/`; the usual entry points (`defaults.sh` sets their directories) |
| `setup.sh TOOLS` | Install Playwright and kokoro-onnx into `.venv`; fetch Chrome, the Kokoro model, and the Slurm image into `TOOLS` |
| `make-video.sh [--landscape] TOOLS WORK OUT.mp4 [opts]` | Build rotari, capture, and render the overview (vertical without `--landscape`) |
| `capture.sh` | Enter the namespaces, start Slurm, run `capture.py` |
| `capture.py` | Run the scenario; record terminal sessions and Web UI screenshots |
| `slurm-up.sh` | Start slurmdbd, slurmctld, and slurmd from the image; write client wrappers |
| `render.py`, `player.html` | The overview's scene timeline (layout from the capture), and the HTML player every video renders through (portrait or landscape) |
| `renderer.py` | Shared rendering: options, narration, player page, frames into ffmpeg |
| `common.py` | Shared capture helpers: commands, terminal sessions (`Term`), Web UI frames |
| `narration.py` | Kokoro TTS clips, scene fitting, the audio track, and subtitles |
| `fixture/` | Fake jobs: `train.py`, per-project `train.sh`, `sweep.sh`, `prep.sh`, `train-gpu.sh`, `eval.sh`, and per-episode `grid/`, `webui/`, `terminal/`, `tune/` (with the agent's session) |
| `fullpage.py` | Framing aid: full-page screenshots of a captured state, run in a namespace |
| `make-episode.sh [--display] TOOLS WORK OUT.mp4 CAPTURE.py RENDER.py` | Build rotari, capture, and render one feature video |
| `capture-web.sh [--display]` | Enter the namespaces and a namespace-owned `/`, with Xvfb, D-Bus, and dunst when asked, and run a capture script |
| `capture_notifications.py`, `render_notifications.py` | Video 02 (the capture needs Xvfb, dunst, google-chrome) |
| `capture_outputs.py`, `render_outputs.py` | Video 03 (a headless browser is enough) |
| `capture_recovering.py`, `render_recovering.py` | Video 04 (a headless browser is enough) |
| `capture_grid.py`, `render_grid.py` | Video 06 (a headless browser is enough) |
| `capture_webui.py`, `render_webui.py` | Video 07 (a headless browser is enough) |
| `capture_terminal.py`, `render_terminal.py` | Video 08 (no browser) |
| `capture_agents.py`, `render_agents.py` | Video 09 (a headless browser is enough); it replays `fixture/tune/session.sh` |
| `capture_executors.py`, `render_executors.py` | Video 05 (needs the Slurm image) |
| `assets/slack/` | Slack screenshots of one capture's webhook message, for video 02 |
| `newroot.sh`, `dunstrc` | A namespace-owned `/` for `--allow-exec`; the desktop notification style |
| `tts_samples.py`, `fixture/tts_train.py` | Kokoro clips and the fake TTS job that "produces" them |
| `send-webhook.py` | Post the Slack payloads a capture recorded to your real webhook |

## Making a video

Each video has a numbered script that does everything (fetch the tools on
first use, build rotari, capture, narrate, render) and writes
`scripts/promo-video/out/NN_name.mp4` with an `.srt` beside it:

```sh
scripts/promo-video/01_overview.sh        # out/01_overview.mp4
scripts/promo-video/01_short.sh           # out/01_short.mp4 (vertical, for Shorts)
scripts/promo-video/02_notifications.sh   # out/02_notifications.mp4 (needs Xvfb)
scripts/promo-video/03_outputs.sh         # out/03_outputs.mp4
scripts/promo-video/04_recovering.sh      # out/04_recovering.mp4
scripts/promo-video/05_executors.sh       # out/05_executors.mp4 (needs singularity once)
scripts/promo-video/06_grid.sh            # out/06_grid.mp4
scripts/promo-video/07_webui.sh           # out/07_webui.mp4
scripts/promo-video/08_terminal.sh        # out/08_terminal.mp4
scripts/promo-video/09_agents.sh          # out/09_agents.mp4
scripts/promo-video/01_short.sh --with-slurm          # include the Slurm act
scripts/promo-video/01_short.sh --voice am_michael    # another narrator
scripts/promo-video/03_outputs.sh --preview 20 45     # PNGs in the work directory
```

The tools go to `PROMO_TOOLS` (default `~/.cache/rotari-promo`, about 600 MB)
and captures to `PROMO_WORK` (default `$TMPDIR/rotari-promo-$USER`); set `GO`
when `go` is not on `PATH` or in `/usr/local/go/bin`. Options after the
script name go to the episode's renderer. `out/` is not tracked.

Underneath, `setup.sh TOOLS` prepares the tools, and `make-video.sh` (the
overview, `--landscape` or vertical) and `make-episode.sh` (feature videos) capture and render one video
into a given path:

```sh
scripts/promo-video/setup.sh "$TMPDIR/promo-tools"
scripts/promo-video/make-video.sh "$TMPDIR/promo-tools" "$TMPDIR/promo-work" out.mp4
scripts/promo-video/setup.sh --with-slurm "$TMPDIR/promo-tools"   # also fetch the Slurm image
```

Capturing takes about 2 minutes and rendering about 5. To change only
timing, captions, narration, or layout, render the existing capture again
(leave out `--narration` for a silent cut):

```sh
PLAYWRIGHT_BROWSERS_PATH="$TOOLS/pw" .venv/bin/python \
  scripts/promo-video/render.py "$WORK/capture" . out.mp4 --narration "$TOOLS/kokoro"
```

For a feature video, run its `render_*.py` on `$PROMO_WORK/NN_name/capture`
the same way (video 02 also takes `--slack scripts/promo-video/assets/slack
--slack-font "$PROMO_TOOLS/fonts/Lato-Regular.ttf"`).

Requirements: Linux with unprivileged user namespaces, `ffmpeg` with libx264,
`curl`, `unzip`, Go, and the repository's `.venv` (created with uv; `setup.sh`
adds Playwright, kokoro-onnx, and soundfile to it with `uv pip install`). Set
`PYTHON` to use another interpreter. The Slurm act also needs `singularity`
or `apptainer`; only then do `setup.sh --with-slurm` and `make-video.sh
--with-slurm` start Slurm, so hosts without singularity can make the default
cut. The Kokoro model is 350 MB, Chrome about 270 MB unpacked, and the Slurm
image 1.5 GB unpacked.

`.venv/bin/python` is a symlink to the system interpreter. The repository is
shared between hosts, so point it at a path every host has
(`/usr/bin/python3.10`); a link to `/usr/bin/python` broke on a host without
that name.

## How the capture is isolated

`capture.sh` runs everything in user, UTS, mount, network, and PID namespaces,
so nothing outside `WORK` changes and nothing private reaches the screen:

- The host name is `workstation`; Slurm jobs see `gpu-node01`. Real host
  names, which rotari records and the Web UI shows, never appear.
- `/home` is a tmpfs with `WORK/home` mounted at `/home/me`, so paths read
  `/home/me/asr`. Rotari's default basedir and `ROTARI_MASTERDIR` are inside
  it. Do not run the scenario's rotari commands outside the namespace: a
  global `~/.config/rotari/config.toml` or `ROTARI_*` variable would send jobs
  to the user's real state directory, as happened once while this was built.
- The network namespace has only loopback, plus a dummy interface with an IPv4
  address. Without that, `getaddrinfo` with `AI_ADDRCONFIG` resolves no IPv4
  names and Slurm cannot find its controller. The daemons' ports cannot clash
  with the host's.
- Exiting `capture.py` ends the PID namespace and every daemon in it.

## Pitfalls already solved

- **No Docker:** the docker socket needs the docker group. `setup.sh` unpacks
  the image `scheduler-integration.sh` uses with singularity instead, and
  `slurm-up.sh` runs it by chroot as namespace root.
- **setgroups:** an unprivileged user namespace refuses `setgroups`, so slurmd
  exits. `setup.sh` preloads a shim in the image that makes it succeed.
- **MariaDB 5.5:** no `CREATE USER IF NOT EXISTS`; the grant creates the user.
  It runs with `--skip-networking`, since the host may already use port 3306.
- **Static rotari:** batch jobs run inside the CentOS 7 image, whose glibc is
  too old for a cgo build, so `make-video.sh` builds with `CGO_ENABLED=0`.
- **TLS-inspecting proxy:** `playwright install` fails on it, and uv needs
  `--native-tls`; `setup.sh` downloads the matching Chrome Headless Shell and
  the Kokoro model files with curl.
- **Readable frames:** the Web UI is captured 600 CSS px wide at scale 2.
  Full-page shots pan between page positions that `capture.py` records
  (`logs.json` `offsets_*`). `SQUEUE_FORMAT` narrows `squeue` for the split
  screen. The pipeline run's jobs table is narrowed through its Columns menu.
- **Layouts:** `capture.py` takes the layout and records it with the
  viewport in `logs.json`, and `render.py` renders whichever the capture was
  made for: the vertical capture uses a 600×810 CSS px browser and 52 terminal
  columns, the landscape one 1100×782 and 70. Terminals show their latest
  lines when the output is taller than the frame, as `submit4` is in
  landscape; the optional Slurm act stacks its two terminals in the frame's
  column there.
- **Fonts:** the player's IBM Plex faces are embedded as data URIs, and
  `render.py` stops if any face fails to load instead of rendering in a
  fallback font. Chrome's `FontFace` API refuses `file://` font URLs.

## Feature videos

Feature videos are landscape: the player puts the caption on the left, the
frame on the right, and the narration subtitle under it. The Web UI is
captured 1100×782 CSS px at scale 2, which keeps Files and Artifacts tables
whole. Each capture runs through `capture-web.sh`, so `newroot.sh` gives the
namespace its own `/`: `rotari web --allow-exec` checks that every directory
from `/` to the launcher file is owned by the user or root, and in a user
namespace the real `/` looks owned by nobody. Web UIs started with
`--allow-exec` use a fixed token and are signed in to over
`http://127.0.0.1` before recording (loopback HTTP sign-in is allowed since
2026-10-23). The fake TTS job writes its outputs with f-strings
(`f"{out}/loss.png"`), as training code commonly does: artifact discovery
skips f-strings, so only `--out outputs/exp0415` is listed, while plain
literals such as `"loss.png"` were listed as missing paths under the working
directory.

### Notifications (02)

A TTS run is checked by hand ("Is it done yet?"), then submitted with rotari;
the moment it ends the desktop shows the Web UI's notification, the Inbox
lists it, and Slack gets the webhook message ("Away from your desk?").

```sh
scripts/promo-video/02_notifications.sh
```

- **Desktop notification:** `capture-web.sh --display` starts Xvfb
  (2200×1564), a D-Bus session, and dunst (1.5 syntax in `dunstrc`) inside the
  namespace. A headed `/usr/bin/google-chrome` at scale 2 opens the run page,
  turns on the Web UI's notifications, and ffmpeg x11grab records the screen
  (`-draw_mouse 0`) while the run ends. Chrome runs as the namespace root, so
  it needs `--no-sandbox`; `--test-type` hides the warning bar, `--lang=en-US`
  with Translate disabled keeps a translation prompt out of the frame, and
  the profile turns the password manager off. Xvfb runs with `-nolock` (its
  lock file would land in the host's shared `/tmp`) and `-nocursor`.
  `render_notifications.py` finds the notification's arrival by comparing the
  top right of each frame with the first.
- **Slack:** the namespace has no network, so the capture's
  `notifications.toml` points the webhook at a local recorder
  (`WORK/capture/webhook/NNN.json`), with the same fields as the user's own
  settings. Afterwards `send-webhook.py` posts those exact payloads to the
  `[webhook] url` of `~/.config/rotari/notifications.toml` without printing
  it. Screenshots taken in Slack then (the desktop notification, without the
  pointer over it, and the message, cropped above the input box) replace
  `assets/slack/`; the current ones show run `20261023-085534-04f4dc00`.
  Every capture makes a new run ID, so `render_notifications.py` redraws the
  run ID in its copy of `message.png` with the one the capture's webhook
  sent, in Lato (Slack's typeface, fetched by `setup.sh`) at the size and
  place in `RUN_ID`, measured by fitting the old ID; nothing else in the
  shots changes. New screenshots need `RUN_ID` measured again; the renderer
  stops when the image size differs from the measured one.

### Outputs (03)

A training run writes its outputs; the job's Artifacts list them, a plot is
previewed and a sample played (briefly, in the Table viewer with its
waveform and spectrogram, which the video does not name), TensorBoard is
launched on the run's `tb/` row, and then the run's Files view opens the
experiment directory, where TensorBoard on `outputs/` compares every
experiment.

```sh
scripts/promo-video/03_outputs.sh
```

- **Workspace:** the capture runs `rotari init` in `~/tts`, so the basedir is
  `~/tts/.rotari-state` and Files opens `~/tts`. Three earlier experiments
  (`exp0412_lr1e-3`, `exp0413_lr3e-4`, `exp0414_bs32`) are made with the fake
  job's `--floor`, so their curves differ in TensorBoard.
- **Launching:** Artifacts and Files rows have their own launch buttons. The
  capture presses the `tb` row's **TensorBoard**, and in Files the first
  **TensorBoard** in the view, the one beside the directory's path. The run
  toolbar has launch buttons too, so lookups stay inside `#output-modal`.
  TensorBoard comes from `.venv` on `PATH`, and its settings side pane is
  closed before the shot. The confirmation shows "Runs as root@workstation":
  the namespace's user.

### Recovering (04)

The overview's 48-run sweep (`fixture/sweep.sh`) fails 8 tasks for two
causes. `rotari show` groups them by cause (its `show:` and `retry:` command
lines are left out of the terminal; the run page shows them), the run page's
Failure causes card shows the same, and one job's Diagnosis dialog gives the
evidence and next step, worked out locally. Then `sed` fixes the batch limit,
`retry --dry-run` lists the 8 jobs that would run, and `retry` reruns them
with the 40 successes carried forward; the new run's jobs table shows
`success (carried)` beside the two reruns' plain `success`.

- The Diagnosis dialog is captured in an 820×588 CSS px window instead of
  1100×782, so its text is larger in the same frame.
- Caption changes inside a terminal scene are timed to the line they describe
  (`shown()` in `render_recovering.py`), not to fixed seconds.

### Executors (05)

A pipeline's three steps run by hand first, each waiting for the one before
(`./prep.sh` here, `sbatch` and `squeue` until training runs on Slurm,
`./eval.sh` here), then go to rotari as one run with `-e local` and
`-e slurm` and dependencies; the run page's jobs table, narrowed to hosts,
executor, and start time and sorted by start time, shows workstation,
gpu-node01, and workstation. SSH is left out: it is rarely used where Slurm
is (the user's call, 2026-10-25).

- **Machines:** `capture_executors.py` starts `slurm-up.sh` (gpu-node01),
  which shares `/home/me` with the workstation as a cluster's home would.
- **Run page:** captured at 900×645 CSS px so the jobs table is larger. Its
  columns are set through the Columns menu's `localStorage` choices, since
  each menu click re-renders the menu; sorting by a header scrolls the table
  sideways, so the capture scrolls it back.
- An earlier version also ran a step over SSH, against an sshd started in the
  namespace; capturing it found that every SSH job in a run failed as "job is
  not running" (fixed in `ffb8c62b`).

### Grid (06)

Twelve training runs (two learning rates × two batch sizes × three seeds)
after one preparation step, then a report (`fixture/grid/`): first by hand
with nested loops, polling `ls results | wc -l`, and running the report;
then as `--job-name prep`, a `--matrix LR=… --matrix BS=… --array 1-3`
job that `--depends-on prep`, and a report that `--depends-on-finished
train`. The run page's job timeline (900×645 CSS px) shows prep, the twelve
runs at once, then the report; the matrix card, opened by its toggle and
captured at 600×430 so its table reads, shows 3/3 in every LR × BS cell, and
`show --job-name report --tail 4` shows the report's output.

### Web UI (07)

Four projects in their own directories (`fixture/webui/train.sh`, whose
`OUTCOME` picks how a job ends): asr, vision, and nlp in the default state
directory, tts in a workspace with its own (`rotari init`). Their history is
made off camera; nlp's training is still running. The terminal runs `ls` and
`rotari web` (its own start-up lines, on the default port); the browser
(960×688) shows All projects, Activity's calendar and Where you worked, Job
activity, and a History search for jobs whose diagnosis is "CUDA/GPU memory
exhausted" in both state directories, which finds asr's and tts's.

- Runs cannot be dated in the past (rotari reads the clock itself), so
  Activity shows one busy day; its per-project time lanes are too narrow to
  read and stay out of the video.
- History search's controls are found by class (`history-search-scope-basedir`,
  `history-search-field`, `history-search-word`) or exact option text;
  Playwright's `has_text` matches case-insensitive substrings, so "Projects"
  also matched "All projects". A diagnosis condition offers the diagnosis
  names in a select instead of a text box.

### Terminal (08)

The other feature videos lean on the Web UI; this one shows what is there on
a cluster's login node with no browser (the user's call, 2026-10-25). An
eight-seed sweep (`fixture/terminal/train.sh`: seed 3 runs out of GPU
memory, seed 6 diverges, seed 8 trains for 40 s) runs in the background
after a finished run of another project. `rotari jobs` (every project),
`rotari show -p sweep` from `Jobs:` to the failures by cause, `show --logs
--tail 1` (each job's last line, its result), `show --failed-logs --tail
2`, and `show --job-name 'train[8]' --follow` (recorded with
`Term.live`, under `timeout`). The log views keep each job's `=== Job:`
line and its log lines, without the working directory, command, and log
path under it. `jobs` (93 columns) and `show` (154) are wide scenes at
28 px and 19 px.

An earlier 08 showed `rotari lineage` comparing a sweep's runs; 09 covers
that through the agent's notes, so it was dropped.

### Overview (01)

`01_overview.sh` makes a quick tour: 01's own opening terminal ("One job? A
terminal is enough."), then a few seconds borrowed from each feature video's
capture in `PROMO_WORK` (07 All projects, 06 the grid's timeline, 05 local
and Slurm, 02 the desktop notification, 04 failure causes and the retried
run, 03 TensorBoard, 09 the agent's run page), ending on the GitHub link
(`render_tour.py`). Make those episodes first; 02's capture needs a display,
so it may come from another host (copy `PROMO_WORK/02_notifications`). The
old story (projects, the 48-run sweep, causes, retry) remains the vertical
Short's, `01_short.sh`, until the Short is cut from the tour.

### Agents (09)

The video takes the side of the person who asked an agent to do the work:
what did it run, why, and what did it find? Its material is a real session.
On 2026-10-25 Claude Code (the agent maintaining these scripts) was asked to
tune the sweep in `fixture/tune/` ("make all six seeds pass and find the
batch size and learning rate with the lowest eval loss; use rotari and read
`rotari guide` first"). It read the guide and made four runs, each with
`--note` on why and `rotari note` afterwards on what it showed: the
baseline (two OOMs, one NaN), batch 64 alone (fixes the OOMs), lr 3e-4
(fixes the NaN, lowers the loss), and lr 1e-4 (worse), ending with a
Markdown table of every seed under every setting. `fixture/tune/session.sh`
keeps the commands that changed state, in order and with the notes as
written; the run IDs it read from output are looked up instead, and its
read-only commands are left out. The capture replays it with
`ROTARI_ACTOR=claude-code`, so the run page says "Started by: cli ·
claude-code".

The video shows `rotari lineage -p sweep` (a wide scene at 15 px, since
its rows with the NOTE column are 192 columns), the first two runs compared
(20 px, where the notes show the agent's reasoning), the last run's page
and report (680×487 CSS px), and `cat AGENTS.md`, the one line that asks an
agent to use rotari this way. An earlier version showed `rotari guide` and
the commands an agent would run; it read as a list of commands any agent
could already run (the user's feedback, 2026-10-25).

## Next steps

- Rework 01 once the feature videos are done: it goes into failure causes and
  retry in as much detail as 04 (the user's feedback, 2026-10-25); the
  overview should instead touch many features briefly and point to the
  feature videos.
- Listen to the narration and adjust wording, voice, or speed; consider
  quiet background music.
- The run page job timeline's axis labels overlap at narrow widths and are
  visible in the video; see `development/ISSUES.md`.
