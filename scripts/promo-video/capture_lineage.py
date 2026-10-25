"""Capture material for the lineage episode: run history, comparison, notes.

Run through `capture-web.sh WORK ROTARI SAMPLES capture_lineage.py` (no
display needed). A six-run sweep lives in a git repository. Off camera it
ran twice, each run from a commit and with a note: batch 128 (two tasks run
out of GPU memory), then batch 64 (memory fits, but another task diverges).
On camera `rotari lineage` lists the history with its notes and code
changes, compares the two runs, and, after a third commit fixes the learning
rate, runs again with a note, records the conclusion with `rotari note`, and
lists the history once more. A headless Chrome (900x645 CSS px at scale 2)
records the last run's page with its notes.

    capture_lineage.py FIXTURE OUT ROTARI SAMPLES

Writes OUT/terminals.json, OUT/web.json, OUT/logs.json, and
OUT/web/<scene>/*.png.
"""

import json
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path

from common import Session, only, section
from playwright.sync_api import sync_playwright

FIXTURE, OUT, ROTARI, SAMPLES = sys.argv[1:5]
HOME = "/home/me"
EXP = f"{HOME}/sweep"
PORT = 18806
BASE = f"http://127.0.0.1:{PORT}"
VIEW = {"width": 900, "height": 645}

ENV = {
    "PATH": f"{HOME}/.local/bin:/usr/bin:/bin",
    "HOME": HOME,
    "USER": "me",
    "LOGNAME": "me",
    "TERM": "xterm-256color",
    "COLUMNS": "70",
    "LC_ALL": "C",
    "TZ": "UTC",
    "ROTARI_MASTERDIR": f"{HOME}/.rotari-master",
    "ROTARI_RUN_LOCAL_CONCURRENCY": "6",
    "GIT_AUTHOR_NAME": "me",
    "GIT_AUTHOR_EMAIL": "me@example.com",
    "GIT_COMMITTER_NAME": "me",
    "GIT_COMMITTER_EMAIL": "me@example.com",
}
S = Session(ENV, HOME, OUT, workdir=EXP)
SWEEP = "rotari add -p sweep --job-name train --array 1-6 ./train.sh"
# The history table and the hint after it; the project header above it
# (state directory, queue, runner server) is left out.
LINEAGE = section("Runs (oldest first)")


def setup():
    os.makedirs(f"{HOME}/.local/bin", exist_ok=True)
    shutil.copy(ROTARI, f"{HOME}/.local/bin/rotari")
    os.makedirs(EXP, exist_ok=True)
    for f in ("train.sh", "config.sh"):
        shutil.copy(f"{FIXTURE}/lineage/{f}", EXP)
    shutil.rmtree(OUT, ignore_errors=True)
    os.makedirs(f"{OUT}/web")
    S.sh("git init -q && git add . && git commit -qm 'sweep: batch 128, lr 1e-3'")
    S.sh(f"{SWEEP} && rotari run -p sweep --quiet --note 'baseline: batch 128, lr 1e-3'")
    S.sh("sed -i 's/BATCH=128/BATCH=64/' config.sh && git commit -qam 'batch 64'")
    S.sh(f"{SWEEP} && rotari run -p sweep --quiet --note 'batch 64 to fit in GPU memory'")


def main():
    setup()
    terms, frames, logs = [], {}, {"view": VIEW}
    first, second = S.runs_of("sweep")

    t = S.term("history")
    t.run("rotari lineage -p sweep", keep=LINEAGE, line_s=0.06)
    terms.append(t)
    t = S.term("compare")
    logs["compare"] = t.run(f"rotari lineage -p sweep {first} {second}", cps=60, line_s=0.06)
    terms.append(t)

    # Fix the learning rate, run with a note, then note what it showed.
    t = S.term("fix")
    t.run("sed -i 's/LR=1e-3/LR=3e-4/' config.sh && git commit -qam 'lr 3e-4'")
    t.run(SWEEP, keep=only("added"))
    t.run(
        "rotari run -p sweep --note 'lr 3e-4 for the NaN in task 4'",
        keep=only("Summary"),
        line_s=0.08,
    )
    third = S.runs_of("sweep")[-1]
    t.run(f"rotari note {third} 'batch 64 + lr 3e-4: all six pass'", cps=60)
    t.run("rotari lineage -p sweep", keep=LINEAGE, line_s=0.06)
    terms.append(t)

    web = subprocess.Popen(
        [f"{HOME}/.local/bin/rotari", "web", "--port", str(PORT)],
        cwd=EXP,
        env=ENV,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    time.sleep(1.5)
    try:
        with sync_playwright() as p:
            b = p.chromium.launch(args=["--no-sandbox"])
            page = b.new_page(viewport=VIEW, device_scale_factor=2, color_scheme="dark")
            page.goto(f"{BASE}/project/sweep/run/{third}")
            page.wait_for_selector("table.runs")
            page.wait_for_timeout(1200)
            S.record(page, frames, "run", 0.5, fps=4)
            b.close()
    finally:
        web.terminate()

    Path(f"{OUT}/terminals.json").write_text(
        json.dumps([t.dump() for t in terms], indent=1)
    )
    Path(f"{OUT}/web.json").write_text(json.dumps(frames, indent=1))
    Path(f"{OUT}/logs.json").write_text(json.dumps(logs, indent=1))


main()
