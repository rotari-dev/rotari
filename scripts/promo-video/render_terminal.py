"""Render the terminal episode (landscape): inspecting runs without a browser.

    render_terminal.py CAPTURE_DIR REPO_DIR OUT.mp4
                       [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                       [--preview SECONDS...]

Builds the scene timeline from capture_terminal.py's material and renders it
with renderer.py. It accompanies docs/INSPECT.md. `rotari jobs` and `rotari
show` print wider than the frame column, so their scenes are wide terminals.
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

    def term(name, captions, extra=1.2, **more):
        t = terms[name]
        return {
            "kind": "term",
            "title": "me@login01: ~/sweep",
            "events": t["events"],
            "dur": t["duration"] + extra,
            "captions": captions,
            **more,
        }

    return [
        {
            "kind": "title",
            "dur": 3.0,
            "lines": [["No browser?", "", 0], ["Just a terminal.", "soft", 0.35]],
            "say": "Logged in to the cluster, with no browser? Everything is in the terminal too.",
        },
        term(
            "jobs",
            [[0, "What's running,<br><em>in every project.</em>"]],
            wide=True,
            # 93 columns wide.
            fontSize=28,
            say="rotari jobs lists what's running and what just finished, in every project.",
        ),
        term(
            "show",
            [[0, "One run:<br><em>every job, and why some failed.</em>"]],
            wide=True,
            # Its fix lines run to 154 columns.
            fontSize=19,
            extra=2.0,
            say="rotari show gives one run: every job's status, how long the running one has been "
            "quiet, and the failures grouped by cause.",
        ),
        term(
            "metrics",
            [[0, "Every job's last line:<br><em>your metrics.</em>"]],
            extra=1.6,
            say="Every job's last log line, which for a sweep is its result.",
        ),
        term(
            "failed",
            [[0, "Only the failures'<br><em>logs.</em>"]],
            extra=1.4,
            say="Only the failed jobs' logs.",
        ),
        term(
            "follow",
            [[0, "Follow one,<br><em>live.</em>"]],
            extra=0.8,
            say="Or follow a running job, live.",
        ),
        {
            "kind": "logo",
            "dur": 4.0,
            "tagline": "Inspecting runs: jobs, show, logs",
            "url": "rotari-dev.github.io/rotari/docs/INSPECT/",
            "say": "Rotari.",
        },
    ]


renderer.main(ARGS, timeline(), layout="landscape")
