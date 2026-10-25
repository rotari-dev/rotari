"""Render the overview (landscape): a quick tour of rotari, one scene per feature.

    render_tour.py WORK_DIR REPO_DIR OUT.mp4
                   [--narration KOKORO_DIR [--voice VOICE] [--speed X]]
                   [--preview SECONDS...]

WORK_DIR is PROMO_WORK, holding the episodes' captures as
WORK_DIR/NN_name/capture: the overview borrows a few seconds from each
feature video (02 to 07, and 09) and its opening terminal from 01's own
capture, so those episodes are captured first. The player page is written to
WORK_DIR/tour; it reaches the episodes' frames by absolute file URL.
"""

import json
import os
from pathlib import Path

from PIL import Image, ImageChops, ImageStat

import renderer

ARGS = renderer.options()
WORK = os.path.abspath(ARGS.capture_dir)
EPISODES = {
    "one": "01_overview",
    "notify": "02_notifications",
    "outputs": "03_outputs",
    "recover": "04_recovering",
    "executors": "05_executors",
    "grid": "06_grid",
    "webui": "07_webui",
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
    # 06: the grid's live timeline, faster.
    grid_speed = 2.0
    recover = c["recover"].logs

    return [
        term(
            "one",
            "one",
            [[0, "One job?<br><em>A terminal is enough.</em>"]],
            title="~/llm",
            extra=0.8,
            say="One job? A terminal is enough.",
            sayAt=1.4,
        ),
        {
            "kind": "title",
            "dur": 2.6,
            "lines": [["But research", "", 0], ["isn't one job.", "soft", 0.35]],
            "say": "But research is never one job.",
        },
        web(
            "webui",
            "home",
            3.6,
            [[0, "Every project,<br><em>one page.</em>"]],
            say="Every project, on one page.",
        ),
        web(
            "grid",
            "live",
            c["grid"].web["live"][-1][0] / grid_speed + 0.3,
            [[0, "A whole grid,<br><em>in one run.</em>"]],
            speed=grid_speed,
            say="A whole grid of runs, in order, in one command.",
        ),
        web(
            "executors",
            "done",
            3.6,
            [[0, "Here or on Slurm,<br><em>the same way.</em>"]],
            say="Here or on Slurm, the same way.",
        ),
        web(
            "notify",
            "finish",
            3.8,
            [[0, "Know the moment<br><em>it ends.</em>"]],
            frames=finish,
            say="Know the moment it ends.",
        ),
        page(
            "recover",
            "failed_run",
            3.6,
            [[0, "Failures,<br><em>grouped by cause.</em>"]],
            recover["offsets_failed_run"]["cards"] - 16,
            say="Failures, grouped by cause,",
        ),
        page(
            "recover",
            "retry_run",
            3.4,
            [[0, "Rerun only<br><em>what failed.</em>"]],
            recover["offsets_retry_run"]["jobs"] - 60,
            say="and rerun only what failed.",
        ),
        web(
            "outputs",
            "tb_board",
            3.6,
            [[0, "Outputs,<br><em>one click away.</em>"]],
            say="Your outputs, one click away.",
        ),
        web(
            "agents",
            "run",
            3.8,
            [[0, "What your agent did,<br><em>and why.</em>"]],
            say="And when your coding agent runs the experiments, you can see what it did, and why.",
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
