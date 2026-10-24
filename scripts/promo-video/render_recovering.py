"""Render the recovering episode (landscape): failure causes, diagnosis, retry.

    render_recovering.py CAPTURE_DIR REPO_DIR OUT.mp4
                         [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                         [--preview SECONDS...]

Builds the scene timeline from capture_recovering.py's material and renders
it with renderer.py. It accompanies docs/RECOVERING.md and
docs/LOCAL_DIAGNOSIS.md.
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
    logs = json.loads(Path(f"{CAP}/logs.json").read_text())
    view = logs["view"]

    def shown(name, prefix):
        """Seconds into terminal name when its line starting with prefix appears."""
        text = ""
        for at, piece in terms[name]["events"]:
            text = text[:-1] if piece == "\b" else text + piece
            if text.rstrip("\n").split("\n")[-1].startswith(prefix):
                return at
        raise SystemExit(f"{name}: no line starting with {prefix!r}")

    def term(name, title, captions, speed=1.0, extra=0.6, **more):
        t = terms[name]
        return {
            "kind": "term",
            "title": title,
            "events": t["events"],
            "speed": speed,
            "dur": t["duration"] / speed + extra,
            "captions": captions,
            **more,
        }

    def page(name, dur, captions, pan, **more):
        """A full-page shot, panned between page positions (CSS px)."""
        return {
            "kind": "web",
            "frames": web[name],
            "dur": dur,
            "captions": captions,
            "pan": pan,
            "panCss": True,
            "cssWidth": view["width"],
            **more,
        }

    failed, retried = logs["offsets_failed_run"], logs["offsets_retry_run"]
    return [
        term(
            "sweep",
            "~/llm-sweep",
            [
                [0, "A 48-run sweep."],
                [shown("sweep", "  Status"), "48 runs.<br><em>8 failed.</em>"],
            ],
            say="A forty-eight-run sweep. Eight of them failed.",
        ),
        {
            "kind": "logs",
            "dur": 4.4,
            "captions": [[0, "…for different reasons."]],
            "cards": [["task 7", logs["gpu"]], ["task 4", logs["oom"]]],
            "say": "For different reasons. Do you open every log?",
        },
        term(
            "causes",
            "~/llm-sweep",
            [[0, "Rotari groups them<br><em>by cause.</em>"]],
            extra=1.2,
            say="Rotari groups the failures by cause, "
            "with the evidence and a suggested fix for each.",
        ),
        page(
            "failed_run",
            4.0,
            [[0, "The run page<br><em>shows the same.</em>"]],
            pan=[[0, failed["cards"] - 16]],
            say="The run page shows the same.",
        ),
        {
            "kind": "web",
            "frames": web["diagnosis"],
            "dur": 5.0,
            "captions": [
                [0, "Each failed job:<br><em>evidence, next step.</em>"],
                [2.6, "Diagnosed locally.<br><em>No log leaves your machine.</em>"],
            ],
            "say": "Each failed job gets a diagnosis, worked out on your machine. "
            "No log leaves it.",
        },
        term(
            "retry",
            "~/llm-sweep",
            [
                [0, "Fix the batch size."],
                [shown("retry", "dry run"), "Preview the retry:<br><em>only the 8 run.</em>"],
            ],
            extra=1.0,
            say="Fix the batch size, and preview the retry. "
            "Only the eight failures would run.",
        ),
        term(
            "retry_run",
            "~/llm-sweep",
            [
                [0, "Retry."],
                [shown("retry_run", "  Submitted"), "8 rerun.<br><em>40 carried forward.</em>"],
            ],
            extra=0.8,
            say="Retry. The forty that succeeded carry forward; they don't run again.",
        ),
        page(
            "retry_run",
            4.4,
            [[0, "All 48 done,<br><em>in one run.</em>"]],
            pan=[[0.3, retried["cards"] - 16], [2.2, retried["jobs"] - 60]],
            say="All forty-eight done, in one run.",
        ),
        {
            "kind": "logo",
            "dur": 4.0,
            "tagline": "Recovering failed runs",
            "url": "rotari-dev.github.io/rotari/docs/RECOVERING/",
            "say": "Rotari.",
        },
    ]


renderer.main(ARGS, timeline(), layout="landscape")
