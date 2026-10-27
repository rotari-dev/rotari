"""Render the overview (landscape): every run on the cluster, on the record.

    render_overview.py CAPTURE_DIR REPO_DIR OUT.mp4
                       [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                       [--preview SECONDS...] [--scale N]

Builds the scene timeline from capture_overview.py's material and renders it
with renderer.py. It opens on one job, which needs no rotari, and research,
which is never one job. Then one message: hand rotari your usual commands,
and every run is recorded and seen in one place. Thirty slurm-N.out files nobody can tell
apart; the same script handed to rotari for Slurm; every project and the
sweep's jobs in the Web UI; a failed job's log; the fix and a retry of only
the failed jobs; and the end card naming the other features.
"""

import json
from pathlib import Path

import renderer

ARGS = renderer.options()
CAP = ARGS.capture_dir


def timeline():
    terms = {
        t["name"]: t for t in json.loads(Path(f"{CAP}/terminals.json").read_text())
    }
    web = json.loads(Path(f"{CAP}/web.json").read_text())

    def typed(name, command):
        """Seconds into terminal name when the prompt for command appears."""
        prompt = "\x01$ \x02"
        text, prompt_at = "", None
        for at, piece in terms[name]["events"]:
            text = text[:-1] if piece == "\b" else text + piece
            line = text.split("\n")[-1]
            if line == prompt:
                prompt_at = at
            elif line.startswith(prompt + command):
                return prompt_at
        raise SystemExit(f"{name}: never typed {command!r}")

    def term(name, captions, extra=1.0, **more):
        t = terms[name]
        return {
            "kind": "term",
            "title": more.pop("title", "me@login01: ~/resnet"),
            "events": t["events"],
            "dur": t["duration"] + extra,
            "captions": captions,
            **more,
        }

    def webscene(name, dur, captions, **more):
        return {
            "kind": "web",
            "frames": web[name],
            "dur": dur,
            "captions": captions,
            **more,
        }

    return [
        # One job needs no rotari; research is never one job.
        term(
            "one",
            [[0, "One job?"]],
            title="me@login01: ~/llm",
            say="One job? Watching it in a terminal is enough.",
            sayAt=1.6,
        ),
        {
            "kind": "title",
            "dur": 2.6,
            "lines": [["You don't", "", 0], ["need rotari.", "hl", 0.35]],
            "say": "You don't need rotari.",
        },
        {
            "kind": "title",
            "dur": 2.6,
            "lines": [["But research", "", 0], ["isn't one job.", "soft", 0.35]],
            "say": "But research is never just one job.",
        },
        term(
            "before",
            [
                [0, "Dozens of jobs."],
                [typed("before", "grep"), "Which one was which?"],
            ],
            say="On a cluster, you submit dozens of jobs. A week later: which one "
            "was which, which one failed, and where are their logs?",
        ),
        term(
            "submit",
            [[0, "Same commands.<br><em>rotari submits them.</em>"]],
            say="With rotari, you hand it the same commands, "
            "and it submits them to Slurm for you.",
        ),
        webscene(
            "projects",
            3.6,
            [[0, "Every run,<br><em>in one place.</em>"]],
            say="Every run is recorded, across all your projects, in one place.",
        ),
        webscene(
            "jobs",
            3.6,
            [[0, "What ran, where,<br><em>and how it ended.</em>"]],
            say="Open any run: what ran, where, and how it ended,",
        ),
        webscene(
            "log",
            4.0,
            [[0, "And each job's log,<br><em>one click away.</em>"]],
            say="and each job's log. Here, batch 128 ran out of GPU memory.",
        ),
        term(
            "retry",
            [
                [0, "Fix it…"],
                [
                    typed("retry", "rotari retry"),
                    "…then rerun<br><em>only what failed.</em>",
                ],
            ],
            say="Fix it, and one command reruns only what failed.",
        ),
        webscene(
            "retried",
            3.6,
            [[0, "The rest<br><em>stay as they are.</em>"]],
            say="The jobs that succeeded stay as they are.",
        ),
        {
            "kind": "logo",
            "dur": 6.0,
            "tagline": "Let it keep track of your runs.<br>Spend your time on research.",
            "url": "github.com/rotari-dev/rotari",
            "also": "Also: notifications · outputs · the terminal · AI coding agents",
            "say": "Rotari. Let it keep track of your runs, and spend your time on research.",
        },
    ]


renderer.main(ARGS, timeline(), layout="landscape")
