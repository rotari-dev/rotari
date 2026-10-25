"""Render the executors episode (landscape): local and Slurm in one run.

    render_executors.py CAPTURE_DIR REPO_DIR OUT.mp4
                        [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                        [--preview SECONDS...]

Builds the scene timeline from capture_executors.py's material and renders it
with renderer.py. It accompanies docs/EXECUTORS.md.
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
            elif line == prompt + command:
                return prompt_at
        raise SystemExit(f"{name}: never typed {command!r}")

    def term(name, title, captions, speed=1.0, extra=0.6, **more):
        t = terms[name]
        return {
            "kind": "term",
            "title": title,
            "events": t["events"],
            "speed": speed,
            "dur": t["duration"] / speed + extra,
            "captions": [[at / speed, text] for at, text in captions],
            **more,
        }

    live_speed = 1.6
    return [
        term(
            "byhand",
            "~/pipeline",
            [
                [0, "Prepare here,<br><em>train on Slurm…</em>"],
                [typed("byhand", "squeue"), "…and wait<br><em>for each step.</em>"],
            ],
            speed=1.3,
            say="Prepare the data here, train on the Slurm cluster, evaluate here again. "
            "By hand, you start each step, and wait for it, before the next.",
        ),
        term(
            "pipeline",
            "~/pipeline",
            [
                [0, "With rotari,<br><em>add each step the same way.</em>"],
                [typed("pipeline", "rotari run -p pipeline --async"), "One command<br><em>runs them all.</em>"],
            ],
            speed=1.3,
            say="With rotari, each step is added the same way; only the executor changes. "
            "One command runs the whole pipeline, in order.",
        ),
        {
            "kind": "web",
            "frames": web["live"],
            "speed": live_speed,
            "dur": web["live"][-1][0] / live_speed + 0.4,
            "captions": [[0, "One run<br><em>across machines.</em>"]],
            "say": "One run, across machines,",
        },
        {
            "kind": "web",
            "frames": web["done"],
            "dur": 4.0,
            "captions": [[0, "Each job,<br><em>and where it ran.</em>"]],
            "say": "and one page shows each job, and where it ran.",
        },
        {
            "kind": "logo",
            "dur": 4.0,
            "tagline": "Executors: local, Slurm, PBS, LSF, SGE",
            "url": "rotari-dev.github.io/rotari/docs/EXECUTORS/",
            "say": "Rotari.",
        },
    ]


renderer.main(ARGS, timeline(), layout="landscape")
