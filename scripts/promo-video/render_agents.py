"""Render the agents episode (landscape): what your agent did, and why.

    render_agents.py CAPTURE_DIR REPO_DIR OUT.mp4
                     [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                     [--preview SECONDS...]

Builds the scene timeline from capture_agents.py's material and renders it
with renderer.py: the runs Claude Code made while tuning a sweep, as the
person who asked it reads them. It accompanies `rotari guide`, docs/MCP.md,
and docs/INSPECT.md (Notes on runs). The lineage tables are wider than the
frame column, so their scenes are wide terminals.
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

    def term(name, captions, speed=1.0, extra=1.6, **more):
        t = terms[name]
        return {
            "kind": "term",
            "title": "~/sweep",
            "events": t["events"],
            "speed": speed,
            "dur": t["duration"] / speed + extra,
            "captions": captions,
            **more,
        }

    return [
        {
            "kind": "title",
            "dur": 3.4,
            "lines": [
                ["Your agent ran", "", 0],
                ["four experiments.", "", 0.3],
                ["What did it do?", "soft", 0.8],
            ],
            "say": "You asked your AI coding agent to tune a sweep. It ran four experiments. "
            "What did it do, and why?",
        },
        term(
            "history",
            [[0, "Every run it made,<br><em>and why it made it.</em>"]],
            wide=True,
            # One history row per line: with its NOTE column it is 192 columns.
            fontSize=15,
            say="rotari keeps every run it made, with the note it left on why: "
            "the baseline, a smaller batch, a lower learning rate, "
            "and a check that lower still isn't better.",
        ),
        term(
            "why",
            [[0, "Its reasoning,<br><em>from one run to the next.</em>"]],
            wide=True,
            # Larger than a wide scene's default: the notes wrap as prose.
            fontSize=20,
            extra=2.4,
            say="Compare two runs to follow its reasoning: what failed, what it concluded, "
            "what it tried next, and what that fixed.",
        ),
        {
            "kind": "web",
            "frames": web["run"],
            "dur": 3.6,
            "captions": [[0, "Started by the agent,<br><em>noted by the agent.</em>"]],
            "say": "Each run is marked as started by the agent,",
        },
        {
            "kind": "web",
            "frames": web["report"],
            "dur": 4.0,
            "captions": [[0, "…down to<br><em>what it found.</em>"]],
            "say": "and the last one carries what it found, as a table.",
        },
        term(
            "agents",
            [[0, "All it took:<br><em>one line in AGENTS.md.</em>"]],
            say="All it took was one line in your agent's instructions.",
        ),
        {
            "kind": "logo",
            "dur": 4.0,
            "tagline": "Your agent's experiments, on the record",
            "url": "github.com/rotari-dev/rotari",
            "say": "Rotari.",
        },
    ]


renderer.main(ARGS, timeline(), layout="landscape")
