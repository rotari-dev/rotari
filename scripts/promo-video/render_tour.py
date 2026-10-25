"""Render the overview (landscape): the story, then a quick tour of the features.

    render_tour.py WORK_DIR REPO_DIR OUT.mp4
                   [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                   [--preview SECONDS...]

Act 1 tells the story from 01's own capture: one job needs no rotari, but
research is many projects in many terminal tabs, and with rotari they all
show on one page. Act 2 passes through the features: a couple of seconds of
each feature video's capture (06, 05, 02, 04, 03, 08, 09), each with a short
caption and line. WORK_DIR is PROMO_WORK, holding the captures as
WORK_DIR/NN_name/capture, so those episodes are captured first. The player
page is written to WORK_DIR/tour; it reaches the frames by absolute file URL.
"""

import json
import os
from pathlib import Path

from PIL import Image, ImageChops, ImageStat

import renderer

# Seconds per shot of the tour.
SHOT = 2.0

ARGS = renderer.options()
WORK = os.path.abspath(ARGS.capture_dir)
EPISODES = {
    "one": "01_overview",
    "notify": "02_notifications",
    "outputs": "03_outputs",
    "recover": "04_recovering",
    "executors": "05_executors",
    "grid": "06_grid",
    "terminal": "08_terminal",
    "agents": "09_agents",
}


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


def notified_at(cap, frames):
    """Seconds into 02's recording when the desktop notification appears."""
    path = lambda url: url[len("file://") :]
    first = Image.open(path(frames[0][1])).convert("L")
    width, height = first.size
    box = (width // 3, 0, width, height // 3)
    first = first.crop(box)
    for t, url in frames:
        frame = Image.open(path(url)).convert("L").crop(box)
        if ImageStat.Stat(ImageChops.difference(first, frame)).mean[0] > 12:
            return t
    raise SystemExit("no desktop notification in 02's recording")


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

    def web(cap, scene, dur, captions, frames=None, **more):
        return {
            "kind": "web",
            "frames": frames or c[cap].web[scene],
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

    # 02: from a second before the desktop notification arrives.
    finish = c["notify"].web["finish"]
    at = notified_at(c["notify"], finish)
    finish = [[t - (at - 1.0), u] for t, u in finish if t >= at - 1.0]
    recover = c["recover"].logs
    one = c["one"].terms

    def shot(scene, caption, say):
        """A tour shot: scene for SHOT seconds, or as long as its short line."""
        return {**scene, "dur": SHOT, "captions": [[0, caption]], "say": say}

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
        # Act 2, the tour: each feature in passing, a short line each.
        shot(web("grid", "live", 0, [], speed=4.0), "Sweeps and pipelines", "From there: sweeps and pipelines."),
        shot(web("executors", "done", 0, []), "Here or on Slurm", "Here, or on Slurm."),
        shot(web("notify", "finish", 0, [], frames=finish), "Told when it ends", "Told when it ends."),
        shot(
            page("recover", "failed_run", 0, [], recover["offsets_failed_run"]["cards"] - 16),
            "Failures, by cause",
            "Failures, grouped by cause.",
        ),
        shot(
            page("recover", "retry_run", 0, [], recover["offsets_retry_run"]["jobs"] - 60),
            "Rerun only what failed",
            "Rerun only what failed.",
        ),
        shot(web("outputs", "tb_board", 0, []), "Outputs, one click away", "Outputs, one click away."),
        shot(
            term("terminal", "jobs", [], title="me@login01: ~/sweep", speed=2.5, wide=True, fontSize=28),
            "All of it in the terminal, too",
            "All of it in the terminal, too.",
        ),
        shot(
            web("agents", "run", 0, []),
            "Your agent's runs, on the record",
            "And your coding agent's runs, on the record.",
        ),
        {
            "kind": "logo",
            "dur": 5.0,
            "tagline": "Spend your time on research,<br>not on babysitting jobs.",
            "url": "github.com/rotari-dev/rotari",
            "say": "Rotari. Spend your time on research, not on babysitting jobs.",
        },
    ]


os.makedirs(f"{WORK}/tour", exist_ok=True)
ARGS.capture_dir = f"{WORK}/tour"
renderer.main(ARGS, timeline(), layout="landscape")
