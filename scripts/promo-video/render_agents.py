"""Render the agents episode (landscape): the CLI guide, notes, and MCP.

    render_agents.py CAPTURE_DIR REPO_DIR OUT.mp4
                     [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                     [--preview SECONDS...]

Builds the scene timeline from capture_agents.py's material and renders it
with renderer.py. It accompanies docs/MCP.md and `rotari guide`. No agent
appears: the captions say what an agent reads and which commands the guide
has it use, which ran for real.
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

    def term(name, title, captions, speed=1.0, extra=0.8, **more):
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

    return [
        {
            "kind": "title",
            "dur": 3.0,
            "lines": [["Your coding agent", "", 0], ["runs experiments too.", "soft", 0.35]],
            "say": "Your coding agent runs experiments too.",
        },
        term(
            "guide",
            "~/sweep",
            [[0, "One command<br><em>teaches it the rules.</em>"]],
            extra=2.0,
            # The guide is written 80 columns wide.
            wide=True,
            fontSize=24,
            say="One command gives it the rules: start runs in the background, "
            "wait for them, leave notes, and read summaries before logs.",
        ),
        term(
            "run",
            "~/sweep",
            [
                [0, "Run with a note,<br><em>in the background.</em>"],
                [typed("run", "rotari wait"), "Back at the<br><em>first failure.</em>"],
                [typed("run", "rotari lineage"), "A summary,<br><em>not a wall of logs.</em>"],
            ],
            speed=1.3,
            say="Run in the background with a note, and come back at the first failure, "
            "with its cause and a fix. The summary stays short, however many jobs fail.",
        ),
        term(
            "fix",
            "~/sweep",
            [
                [0, "Fix, retry,<br><em>with a note.</em>"],
                [typed("fix", "cat results.md"), "Leave the results<br><em>for the next reader.</em>"],
            ],
            speed=1.3,
            say="Fix it, retry with a note, and leave the results as a table "
            "for whoever reads the run next, person or agent.",
        ),
        {
            "kind": "web",
            "frames": web["report"],
            "dur": 3.6,
            "captions": [[0, "In the run report<br><em>for you, too.</em>"]],
            "say": "You'll find it in the run report, too.",
        },
        term(
            "mcp",
            "~/sweep",
            [[0, "No shell?<br><em>An MCP server.</em>"]],
            extra=1.4,
            say="And for agents without a shell, rotari has an MCP server.",
        ),
        {
            "kind": "logo",
            "dur": 4.0,
            "tagline": "For coding agents: rotari guide",
            "url": "rotari-dev.github.io/rotari/docs/MCP/",
            "say": "Rotari.",
        },
    ]


renderer.main(ARGS, timeline(), layout="landscape")
