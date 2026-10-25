"""Render the overview (landscape): one story, from many jobs to one place.

    render_tour.py WORK_DIR REPO_DIR OUT.mp4
                   [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                   [--preview SECONDS...]

From 01's own capture: one job needs no rotari, but research is many
projects in many terminal tabs where something breaks, and with rotari they
all show on one page. Then one payoff from 04's capture: the failures grouped
by cause, and a retry that reruns only them. The end card names the other
features without narration; each has its own video. WORK_DIR is PROMO_WORK,
holding the captures as WORK_DIR/NN_name/capture, so 01's and 04's are made
first. The player page is written to WORK_DIR/tour; it reaches the frames by
absolute file URL.
"""

import json
import os
from pathlib import Path

import renderer

ARGS = renderer.options()
WORK = os.path.abspath(ARGS.capture_dir)
EPISODES = {"one": "01_overview", "recover": "04_recovering"}


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

    def page(cap, scene, dur, captions, top, **more):
        """A full-page shot from cap, shown from page position top (CSS px)."""
        return web(
            cap,
            scene,
            dur,
            captions,
            pan=[[0, top]],
            panCss=True,
            cssWidth=c[cap].logs.get("view", {"width": 1100})["width"],
            **more,
        )

    recover = c["recover"].logs
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
        # Act 3, one payoff: something broke, so rerun only that.
        page(
            "recover",
            "failed_run",
            4.0,
            [[0, "Failures,<br><em>grouped by cause.</em>"]],
            recover["offsets_failed_run"]["cards"] - 16,
            say="When something fails, rotari groups the failures by cause, "
            "with a fix for each.",
        ),
        term(
            "recover",
            "retry_run",
            [[0, "Rerun only<br><em>what failed.</em>"]],
            title="~/llm-sweep",
            extra=0.8,
            say="Fix it, and rerun only what failed. Everything that succeeded stays as it is.",
        ),
        {
            "kind": "logo",
            "dur": 6.0,
            "tagline": "Spend your time on research,<br>not on babysitting jobs.",
            "url": "github.com/rotari-dev/rotari",
            "also": "Also: Slurm · notifications · outputs · the terminal · coding agents",
            "say": "Rotari. Spend your time on research, not on babysitting jobs.",
        },
    ]


os.makedirs(f"{WORK}/tour", exist_ok=True)
ARGS.capture_dir = f"{WORK}/tour"
renderer.main(ARGS, timeline(), layout="landscape")
