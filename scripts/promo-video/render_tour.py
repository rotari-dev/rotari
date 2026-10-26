"""Render the overview (landscape): one story, from many jobs to one place.

    render_tour.py WORK_DIR REPO_DIR OUT.mp4
                   [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                   [--preview SECONDS...]

All from 01's own capture, following one failure through: one job needs no
rotari, but research is many projects in many terminal tabs where something
breaks; with rotari they all show on one page, where the broken project
stands out; its log says why, a fixed path and a rerun of just that project
mend it, and the plot it then writes opens in the browser. The end card
names the other features without narration; each has its own video.
WORK_DIR is PROMO_WORK, holding the capture as
WORK_DIR/01_overview/capture. The player page is written to WORK_DIR/tour; it
reaches the frames by absolute file URL.
"""

import json
import os
from pathlib import Path

import renderer

ARGS = renderer.options()
WORK = os.path.abspath(ARGS.capture_dir)
EPISODES = {"one": "01_overview"}


class Capture:
    """One episode's capture: its terminals, Web UI frames, and logs."""

    def __init__(self, name):
        self.dir = f"{WORK}/{EPISODES[name]}/capture"
        if not os.path.isdir(self.dir):
            raise SystemExit(
                f"{self.dir} is missing: capture {EPISODES[name]} first "
                f"(scripts/promo-video/{EPISODES[name]}.sh)"
            )
        self.terms = {
            t["name"]: t for t in json.loads(Path(f"{self.dir}/terminals.json").read_text())
        }
        self.logs = json.loads(Path(f"{self.dir}/logs.json").read_text())
        web = json.loads(Path(f"{self.dir}/web.json").read_text())
        # Frames by absolute URL, since the player page lives elsewhere.
        self.web = {
            scene: [[t, f"file://{self.dir}/{path}"] for t, path in shots]
            for scene, shots in web.items()
        }


def timeline():
    c = {name: Capture(name) for name in EPISODES}

    def term(cap, name, captions, speed=1.0, extra=0.6, **more):
        t = c[cap].terms[name]
        return {
            "kind": "term",
            "title": more.pop("title", "~"),
            "events": t["events"],
            "speed": speed,
            "dur": t["duration"] / speed + extra,
            "captions": captions,
            **more,
        }

    def web(cap, scene, dur, captions, **more):
        return {
            "kind": "web",
            "frames": c[cap].web[scene],
            "dur": dur,
            "captions": captions,
            **more,
        }

    one = c["one"].terms

    return [
        # Act 1, the story: one job needs no rotari; research is many jobs.
        term(
            "one",
            "one",
            [[0, "One job?"]],
            title="~/llm",
            extra=1.0,
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
        {
            "kind": "tabs",
            "dur": 7.2,
            "speed": 1.6,
            "tabs": [
                {"title": d, "events": one[f"tab-{d}"]["events"]}
                for d in ("asr", "tts", "vision", "nlp")
            ],
            "captions": [
                [0, "4 projects.<br>4 directories. 4 tabs."],
                [3.6, "Something broke.<br><em>Which tab was it?</em>"],
            ],
            "say": "Each project lives in its own directory. So you hop between "
            "terminal tabs, and somewhere, something broke.",
        },
        term(
            "one",
            "submit4",
            [[0, "With rotari, submit<br>from any directory…"]],
            speed=2.6,
            extra=0.4,
            say="With rotari, you submit from any directory,",
            sayAt=4.6,
        ),
        web(
            "one",
            "projects",
            8.0,
            [
                [0, "…and see every project<br>on one page."],
                [3.8, "Vision failed.<br><em>You see it at a glance.</em>"],
            ],
            speed=15 / 8.0,
            say="and every project shows up on one page. "
            "You see what failed at a glance.",
        ),
        # Then follow the failure through: why, the fix, and what it made.
        web(
            "one",
            "vision_log",
            4.0,
            [[0, "Why?<br><em>Its log, right there.</em>"]],
            say="Why? Its log is right there: the dataset path is wrong.",
        ),
        term(
            "one",
            "vision_fix",
            [[0, "Fix it,<br><em>rerun just that.</em>"]],
            title="~/vision",
            extra=0.8,
            say="Fix the path, and rerun just that project.",
        ),
        web(
            "one",
            "vision_plot",
            4.0,
            [[0, "Then see<br><em>what it made.</em>"]],
            say="Then see what it made, right in the browser.",
        ),
        {
            "kind": "logo",
            "dur": 6.0,
            "tagline": "Spend your time on research,<br>not on babysitting jobs.",
            "url": "github.com/rotari-dev/rotari",
            "also": "Also: calendar & history search · sweeps · Slurm · notifications · AI coding agents",
            "say": "Rotari. Spend your time on research, not on babysitting jobs.",
        },
    ]


os.makedirs(f"{WORK}/tour", exist_ok=True)
ARGS.capture_dir = f"{WORK}/tour"
renderer.main(ARGS, timeline(), layout="landscape")
