"""Capture material for the grid episode: arrays, matrices, and dependencies.

Run through `capture-web.sh WORK ROTARI SAMPLES capture_grid.py` (no display
needed). A grid of 12 training runs (two learning rates, two batch sizes,
three seeds) after one preparation step, then a report, first by hand with
nested loops and then as three `rotari add` commands: a job, a matrix with an
array, and a job that depends on the matrix finishing. A headless Chrome
records the run page's job timeline (900x645 CSS px at scale 2, so it reads
in the frame) while the run goes, and the matrix card at 600x430.

    capture_grid.py FIXTURE OUT ROTARI SAMPLES

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

from common import Session, only
from playwright.sync_api import sync_playwright

FIXTURE, OUT, ROTARI, SAMPLES = sys.argv[1:5]
HOME = "/home/me"
EXP = f"{HOME}/grid"
PORT = 18798
BASE = f"http://127.0.0.1:{PORT}"
VIEW = {"width": 900, "height": 645}
CLOSE_VIEW = {"width": 600, "height": 430}

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
    "ROTARI_RUN_LOCAL_CONCURRENCY": "12",
}
S = Session(ENV, HOME, OUT, workdir=EXP)

# Scroll the job timeline to the top of the window.
TO_TIMELINE = """(() => {
  const t = [...document.querySelectorAll('*')]
    .find((e) => e.children.length === 0 && e.textContent.trim() === 'Job timeline');
  window.scrollTo(0, t.getBoundingClientRect().top + scrollY - 20);
})()"""


def setup():
    os.makedirs(f"{HOME}/.local/bin", exist_ok=True)
    shutil.copy(ROTARI, f"{HOME}/.local/bin/rotari")
    os.makedirs(EXP, exist_ok=True)
    for f in ("prepare.sh", "train.sh", "report.sh"):
        shutil.copy(f"{FIXTURE}/grid/{f}", EXP)
    shutil.rmtree(OUT, ignore_errors=True)
    os.makedirs(f"{OUT}/web")


def main():
    setup()
    terms, frames, logs = [], {}, {"view": VIEW}

    # By hand: prepare, start 12 runs from nested loops, poll, then report.
    t = S.term("byhand")
    t.run("./prepare.sh > prepare.log")
    t.run(
        "for lr in 1e-3 3e-4; do for bs in 16 32; do for s in 1 2 3; do"
        " LR=$lr BS=$bs SEED=$s nohup ./train.sh > log.$lr.$bs.$s 2>&1 &"
        " done; done; done",
        cps=60,
    )
    time.sleep(1.5)
    t.run("ls results | wc -l")
    time.sleep(1.0)
    t.run("ls results | wc -l")
    time.sleep(2.5)
    t.run("ls results | wc -l")
    t.run("./report.sh")
    terms.append(t)
    S.sh("rm -rf results log.* prepare.log nohup.out")

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

            # With rotari: the same grid as three adds and one run.
            t = S.term("grid")
            t.run(
                "rotari add -p grid --job-name prep ./prepare.sh",
                cps=40,
                keep=only("added"),
            )
            t.run(
                "rotari add -p grid --job-name train --depends-on prep"
                " --matrix LR=1e-3,3e-4 --matrix BS=16,32 --array 1-3 ./train.sh",
                cps=40,
                keep=only("added"),
            )
            t.run(
                "rotari add -p grid --job-name report"
                " --depends-on-finished train ./report.sh",
                cps=40,
                keep=only("added"),
            )
            t.run(
                "rotari run -p grid --async", keep=only("=== Run started", "Project:")
            )
            terms.append(t)
            rid = S.runs_of("grid")[-1]
            page.goto(f"{BASE}/project/grid/run/{rid}")
            page.wait_for_selector("table.runs")
            page.wait_for_timeout(500)
            page.evaluate(TO_TIMELINE)
            S.record(page, frames, "live", 14, fps=4)
            S.wait_idle("grid")
            page.reload()
            page.wait_for_selector("table.runs")
            page.wait_for_timeout(1500)
            page.evaluate(TO_TIMELINE)
            S.record(page, frames, "done", 0.5, fps=4)
            # The matrix card, every combination's result, in a smaller window
            # with the frame's shape so that its table comes out larger.
            close = b.new_page(
                viewport=CLOSE_VIEW, device_scale_factor=2, color_scheme="dark"
            )
            close.goto(page.url)
            close.wait_for_selector(".matrix-panel")
            close.wait_for_timeout(1000)
            panel = close.locator(".matrix-panel").first
            panel.locator(".matrix-toggle").click()
            close.wait_for_timeout(800)
            # Below the sticky header bar.
            panel.evaluate(
                "e => window.scrollTo(0, e.getBoundingClientRect().top + scrollY - 70)"
            )
            close.wait_for_timeout(300)
            S.record(close, frames, "matrix", 0.5, fps=4)
            b.close()
    finally:
        web.terminate()

    # The report job's output.
    t = S.term("report")
    t.run(
        "rotari show -p grid --job-name report --tail 4",
        keep=lambda lines: lines[
            lines.index(next(ln for ln in lines if ln.startswith("report:"))) :
        ],
    )
    terms.append(t)
    logs["show"] = S.sh("rotari show -p grid").stdout

    Path(f"{OUT}/terminals.json").write_text(
        json.dumps([t.dump() for t in terms], indent=1)
    )
    Path(f"{OUT}/web.json").write_text(json.dumps(frames, indent=1))
    Path(f"{OUT}/logs.json").write_text(json.dumps(logs, indent=1))


main()
