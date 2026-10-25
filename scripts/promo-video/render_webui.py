"""Render the Web UI episode (landscape): every project and state directory.

    render_webui.py CAPTURE_DIR REPO_DIR OUT.mp4
                    [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                    [--preview SECONDS...]

Builds the scene timeline from capture_webui.py's material and renders it
with renderer.py. It accompanies docs/WEB.md.
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

    def webscene(name, dur, captions, **more):
        return {"kind": "web", "frames": web[name], "dur": dur, "captions": captions, **more}

    start = terms["start"]
    return [
        {
            "kind": "title",
            "dur": 2.8,
            "lines": [["Many projects.", "", 0], ["Where did that run go?", "soft", 0.35]],
            "say": "Many projects, in many directories. Where did that run go?",
        },
        {
            "kind": "term",
            "title": "~",
            "events": start["events"],
            "dur": start["duration"] + 0.6,
            "captions": [[0, "One command."]],
            "say": "One command opens all of them.",
        },
        webscene(
            "home",
            3.4,
            [[0, "Every project,<br><em>every state directory.</em>"]],
            say="Every project, from every state directory.",
        ),
        webscene(
            "activity",
            3.2,
            [[0, "Activity:<br><em>your runs by day.</em>"]],
            say="Activity puts your runs on a calendar,",
        ),
        webscene(
            "where",
            3.6,
            [[0, "…and where<br><em>you worked.</em>"]],
            say="and shows where you worked.",
        ),
        webscene(
            "jobs",
            3.4,
            [[0, "What's running,<br><em>what just finished.</em>"]],
            say="Job activity shows what's running, and what just finished.",
        ),
        webscene(
            "search_query",
            3.4,
            [[0, "Which jobs ran out<br><em>of GPU memory?</em>"]],
            say="Which jobs ran out of GPU memory, in any project?",
        ),
        webscene(
            "search_results",
            3.6,
            [[0, "History search<br><em>finds them all.</em>"]],
            say="History search finds them, across state directories.",
        ),
        {
            "kind": "logo",
            "dur": 4.0,
            "tagline": "The Web UI",
            "url": "rotari-dev.github.io/rotari/docs/WEB/",
            "say": "Rotari.",
        },
    ]


renderer.main(ARGS, timeline(), layout="landscape")
