"""Render the Web terminal episode (landscape): shells that belong to projects.

    render_terminal_web.py CAPTURE_DIR REPO_DIR OUT.mp4
                           [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                           [--preview SECONDS...]

Builds the scene timeline from capture_terminal_web.py's material and renders
it with renderer.py. It accompanies the Terminal section of docs/WEB.md.
"""

import json
from pathlib import Path

import renderer

ARGS = renderer.options()
CAP = ARGS.capture_dir


def timeline():
    terms = json.loads(Path(f"{CAP}/terminals.json").read_text())
    web = json.loads(Path(f"{CAP}/web.json").read_text())

    def webscene(name, captions, extra=0.0, **more):
        frames = web[name]
        return {
            "kind": "web",
            "frames": frames,
            "dur": round(frames[-1][0] + 0.3 + extra, 3),
            "captions": captions,
            **more,
        }

    return [
        {
            "kind": "tabs",
            "dur": 6.0,
            "tabs": [{"title": "bash", "events": t["events"]} for t in terms],
            "captions": [
                [0, "Experiments in parallel.<br><em>Which tab was which?</em>"]
            ],
            "say": "Run experiments in parallel, and the terminal tabs pile up. "
            "Which one was the TTS sweep?",
        },
        {
            "kind": "title",
            "dur": 3.0,
            "lines": [
                ["Shells that belong", "", 0],
                ["to your experiments.", "soft", 0.35],
            ],
            "say": "In rotari, shells belong to your projects.",
        },
        webscene(
            "asr_open",
            [
                [0, "Open a project's terminal…"],
                [2.8, "…where its<br><em>last run started.</em>"],
            ],
            say="Open the terminal on a project's page, and the shell starts where its last run started.",
        ),
        webscene(
            "tts_page",
            [[0, "Another project,<br><em>its own shells.</em>"]],
            say="Go to another project, and the panel shows that project's shells.",
        ),
        webscene(
            "all_shells",
            [[0, "All shells,<br><em>at a glance.</em>"]],
            extra=1.0,
            say="All shells shows every shell of every project: where it is, what it runs, "
            "which command finished, and how each project's last run is doing.",
        ),
        webscene(
            "processes",
            [[0, "What each shell<br><em>started.</em>"]],
            say="Even the processes each shell started.",
        ),
        webscene(
            "to_vision",
            [[0, "Pick one,<br><em>and you're there.</em>"]],
            say="Pick one, and you're on that project's page, with that shell.",
        ),
        webscene(
            "launch",
            [[0, "Launch where<br><em>the shell is.</em>"]],
            say="And launch TensorBoard right where a shell is.",
        ),
        webscene(
            "board",
            [[0, "TensorBoard,<br><em>one click away.</em>"]],
            extra=2.0,
        ),
        {
            "kind": "logo",
            "dur": 4.0,
            "tagline": "The Web terminal",
            "install": "rotari web --allow-terminal",
            "url": "rotari-dev.github.io/rotari/docs/WEB/#terminal",
            "say": "Rotari.",
        },
    ]


renderer.main(ARGS, timeline(), layout="landscape")
