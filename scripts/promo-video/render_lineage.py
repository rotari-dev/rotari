"""Render the lineage episode (landscape): run history, comparison, notes.

    render_lineage.py CAPTURE_DIR REPO_DIR OUT.mp4
                      [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                      [--preview SECONDS...]

Builds the scene timeline from capture_lineage.py's material and renders it
with renderer.py. It accompanies docs/INSPECT.md (Run lineage). The lineage
tables are wider than the frame column, so their scenes are wide terminals.
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

    def wide(name, captions, speed=1.0, extra=1.4, **more):
        t = terms[name]
        return {
            "kind": "term",
            "wide": True,
            "title": "~/sweep",
            "events": t["events"],
            "speed": speed,
            "dur": t["duration"] / speed + extra,
            "captions": [[at / speed, text] for at, text in captions],
            **more,
        }

    return [
        {
            "kind": "title",
            "dur": 3.0,
            "lines": [["One fix.", "", 0], ["One new failure.", "hl", 0.35]],
            "say": "Your last change fixed one thing, and broke another. What changed?",
        },
        wide(
            "history",
            [[0, "Every run, with its note<br><em>and whether the code changed.</em>"]],
            say="rotari lineage lists every run, with the note you gave it, "
            "and whether the code changed since the run before.",
        ),
        wide(
            "compare",
            [[0, "Compare two runs:<br><em>fixed, newly failing, and why.</em>"]],
            speed=1.2,
            say="Compare two runs: two jobs fixed, one newly failing, "
            "each with its cause, and the commit between them.",
        ),
        wide(
            "fix",
            [
                [0, "Fix it, with a note."],
                [typed("fix", "rotari note"), "Write down<br><em>what it showed.</em>"],
                [typed("fix", "rotari lineage"), "The history<br><em>tells the story.</em>"],
            ],
            speed=1.3,
            say="Fix the learning rate, run with a note, and write down what it showed. "
            "The history now tells the whole story.",
        ),
        {
            "kind": "web",
            "frames": web["run"],
            "dur": 3.6,
            "captions": [[0, "Notes on<br><em>the run page, too.</em>"]],
            "say": "The notes are on the run page, too.",
        },
        {
            "kind": "logo",
            "dur": 4.0,
            "tagline": "Run history and comparison",
            "url": "rotari-dev.github.io/rotari/docs/INSPECT/",
            "say": "Rotari.",
        },
    ]


renderer.main(ARGS, timeline(), layout="landscape")
