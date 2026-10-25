"""Render the grid episode (landscape): arrays, matrices, and dependencies.

    render_grid.py CAPTURE_DIR REPO_DIR OUT.mp4
                   [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                   [--preview SECONDS...]

Builds the scene timeline from capture_grid.py's material and renders it with
renderer.py. It accompanies docs/RUNNING.md (Array and matrix jobs,
Dependencies and stages).
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
            "~/grid",
            [
                [0, "2 learning rates × 2 batch sizes<br><em>× 3 seeds, by hand.</em>"],
                [typed("byhand", "ls results | wc -l"), "Done yet?"],
                [typed("byhand", "./report.sh"), "Then remember<br><em>the report.</em>"],
            ],
            speed=1.3,
            say="Two learning rates, two batch sizes, three seeds: twelve runs, "
            "after one preparation step. By hand, that's nested loops, "
            "checking whether they're done, and remembering the report.",
        ),
        term(
            "grid",
            "~/grid",
            [
                [0, "With rotari:<br><em>the whole grid in three lines.</em>"],
                [typed("grid", "rotari run -p grid --async"), "One run."],
            ],
            speed=1.3,
            say="With rotari, the grid is a matrix with an array, "
            "after the preparation step. The report waits until training is finished.",
        ),
        {
            "kind": "web",
            "frames": web["live"],
            "speed": live_speed,
            "dur": web["live"][-1][0] / live_speed + 0.4,
            "captions": [[0, "Prepare, then 12 runs,<br><em>then the report.</em>"]],
            "say": "Prepare first, then twelve runs at once, then the report.",
        },
        {
            "kind": "web",
            "frames": web["matrix"],
            "dur": 3.6,
            "captions": [[0, "Every combination<br><em>at a glance.</em>"]],
            "say": "Every combination, at a glance.",
        },
        term(
            "report",
            "~/grid",
            [[0, "And the report<br><em>ran by itself.</em>"]],
            extra=1.4,
            say="And the report ran by itself.",
        ),
        {
            "kind": "logo",
            "dur": 4.0,
            "tagline": "Arrays, matrices, dependencies",
            "url": "rotari-dev.github.io/rotari/docs/RUNNING/",
            "say": "Rotari.",
        },
    ]


renderer.main(ARGS, timeline(), layout="landscape")
