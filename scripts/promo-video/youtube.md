# YouTube titles and descriptions

One section per video in `out/`: the heading is the video's file name, the
`Title:` line its title, and the rest its description, pasted as is.
`youtube_text.py` writes each section to `out/NAME.txt`. Chapters are listed
only where YouTube shows them (at least three, each at least 10 seconds), from
the scene times the renderer prints; check them again after a video changes.

## 01_overview

Title: Research isn't one job: all your runs in one place | rotari

One job? Watching it in a terminal is enough. But research is many projects in
many directories, and somewhere, something breaks. rotari puts every project
on one page: see which run failed, open its log, fix it, rerun just that
project, and look at what it produced, right in the browser.

0:00 One job, or many?
0:11 Every project on one page
0:34 Why it failed, and the fix
0:47 What it made

rotari is an open-source tool for managing and running jobs: submit commands
from any directory, run them here or on Slurm, and see every project's runs,
logs, and outputs in one place.

GitHub: https://github.com/rotari-dev/rotari
Documentation: https://rotari-dev.github.io/rotari/docs/
Getting started: https://rotari-dev.github.io/rotari/docs/GETTING_STARTED/

Everything on screen comes from real runs of rotari. Narration is synthesized
with Kokoro TTS.

#MachineLearning #MLOps #rotari

## 01_short

Title: One job? You don't need rotari. But research isn't one job. #Shorts

Four projects, four terminal tabs, and somewhere something broke. rotari puts
every project on one page, groups a sweep's failures by cause, and reruns
only what failed.

GitHub: https://github.com/rotari-dev/rotari

#Shorts #MachineLearning #rotari

## 02_notifications

Title: Know the moment a run ends: desktop and Slack notifications | rotari

Stop checking whether your run is done. Submit it with rotari and get on with
your day: when it ends, a desktop notification pops up, the result lands in
the Web UI's Inbox, and, if you're away from your desk, in Slack.

Notifications: https://rotari-dev.github.io/rotari/docs/NOTIFICATIONS/
GitHub: https://github.com/rotari-dev/rotari

Everything on screen comes from real runs of rotari. Narration is synthesized
with Kokoro TTS.

#MachineLearning #MLOps #rotari

## 03_outputs

Title: See what your runs wrote: plots, audio, TensorBoard in the browser | rotari

A training run writes plots, audio samples, and TensorBoard logs. rotari lists
each job's outputs, previews a plot or plays a sample in the browser, starts
TensorBoard from the row after showing you what it will run, and opens the
whole experiment directory to compare every run in one TensorBoard.

0:00 A job's outputs, previewed
0:26 TensorBoard from the row
0:39 Compare every experiment

Outputs and artifacts: https://rotari-dev.github.io/rotari/docs/OUTPUTS/
GitHub: https://github.com/rotari-dev/rotari

Everything on screen comes from real runs of rotari. Narration is synthesized
with Kokoro TTS.

#MachineLearning #TensorBoard #rotari

## 04_recovering

Title: Failed runs: see why, and rerun only what failed | rotari

Eight of 48 runs failed, for different reasons. rotari groups the failures by
cause, each with an example line and a suggested fix, and diagnoses each job
on your machine: no log leaves it. Fix the cause, preview the retry, and rerun
only the eight. The 40 that succeeded carry forward.

0:00 48 runs, 8 failed
0:16 Diagnosis, done locally
0:26 Fix and retry only the failures

Recovering failed runs: https://rotari-dev.github.io/rotari/docs/RECOVERING/
Local diagnosis: https://rotari-dev.github.io/rotari/docs/LOCAL_DIAGNOSIS/
GitHub: https://github.com/rotari-dev/rotari

Everything on screen comes from real runs of rotari. Narration is synthesized
with Kokoro TTS.

#MachineLearning #MLOps #rotari

## 05_executors

Title: One pipeline, here and on Slurm, the same commands | rotari

Prepare data on your workstation, train on the Slurm cluster, evaluate back
on your workstation. By hand, you start each step and wait for it before the
next. With rotari, every step is added the same way, only the executor
changes, and one command runs the whole pipeline in order. The run page shows
each job and where it ran.

rotari runs jobs locally, over SSH, or on Slurm, PBS, LSF, and Sun Grid
Engine.

Executors and schedulers: https://rotari-dev.github.io/rotari/docs/EXECUTORS/
GitHub: https://github.com/rotari-dev/rotari

Everything on screen comes from real runs of rotari, with a one-node Slurm
cluster. Narration is synthesized with Kokoro TTS.

#Slurm #HPC #rotari

## 06_grid

Title: A 12-run grid in three lines: arrays, matrices, dependencies | rotari

Two learning rates, two batch sizes, three seeds: twelve runs after one
preparation step, then a report. By hand, that's nested loops, checking
whether they're done, and remembering the report. With rotari it's a job, a
matrix with an array that depends on it, and a report that waits for the
matrix to finish: one run, in order, with every combination's result at a
glance.

0:00 By hand: nested loops
0:11 With rotari: three lines
0:22 The run, and every combination

Arrays, matrices, and dependencies: https://rotari-dev.github.io/rotari/docs/RUNNING/
GitHub: https://github.com/rotari-dev/rotari

Everything on screen comes from real runs of rotari. Narration is synthesized
with Kokoro TTS.

#MachineLearning #HyperparameterTuning #rotari

## 07_webui

Title: Every project and past run in one Web UI: calendar and search | rotari

Many projects in many directories, some with state directories of their own.
One rotari web shows them all: every project, your runs on a calendar and
where you worked, what's running and what just finished, and a search through
your whole history, such as every job that ran out of GPU memory, in any
project.

The Web UI: https://rotari-dev.github.io/rotari/docs/WEB/
GitHub: https://github.com/rotari-dev/rotari

Everything on screen comes from real runs of rotari. Narration is synthesized
with Kokoro TTS.

#MachineLearning #MLOps #rotari

## 08_terminal

Title: No browser? Inspect your runs from the terminal: jobs, show, logs | rotari

Logged in to the cluster with no browser? rotari jobs lists what's running and
what just finished in every project; rotari show gives one run, with every
job's status, how long the running one has been quiet, and its failures
grouped by cause. Read every job's last log line, which for a sweep is its
result, only the failed jobs' logs, or follow a running job live.

0:00 What's running, and one run at a glance
0:18 Logs: every result, only the failures
0:29 Follow a running job live

Inspecting runs and jobs: https://rotari-dev.github.io/rotari/docs/INSPECT/
GitHub: https://github.com/rotari-dev/rotari

Everything on screen comes from real runs of rotari. Narration is synthesized
with Kokoro TTS.

#HPC #CommandLine #rotari

## 09_agents

Title: What did your AI coding agent run, and why? | rotari

You asked your AI coding agent to tune a sweep, and it ran four experiments.
With rotari you can follow what it did: every run with the note it left on
why, its reasoning from one run to the next, each run marked as started by
the agent, and its conclusion as a table of results. All it took was one line
in AGENTS.md.

The runs in this video were made by Claude Code, asked to tune the sweep after
reading `rotari guide`; its commands and notes are shown as it wrote them.

Notes on runs: https://rotari-dev.github.io/rotari/docs/INSPECT/#notes-on-runs
MCP server: https://rotari-dev.github.io/rotari/docs/MCP/
GitHub: https://github.com/rotari-dev/rotari

Narration is synthesized with Kokoro TTS.

#AIAgents #MachineLearning #rotari
