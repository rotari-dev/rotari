"""Capture material for the overview: every run on the cluster, on the record.

Run through `capture-web.sh WORK ROTARI SAMPLES capture_overview.py` (no
display needed) with PROMO_SLURM_SB naming the Slurm sandbox from
`setup.sh --with-slurm`. slurm-up.sh starts a one-node Slurm cluster
(gpu-node01) sharing /home/me. Off camera, thirty jobs are submitted to it by
hand with sbatch, as last week's work, leaving thirty slurm-N.out files in
~/resnet, and another project, bert, has a finished run. On camera: those
files, then a ResNet sweep (three learning rates by two batch sizes) handed
to rotari for Slurm, where batch 128 runs out of GPU memory at fp32; every
project and the sweep's jobs on the Web UI; the failed job's log; the fix to
bf16 and a retry of only the failed jobs; and the retried run. A headless
Chrome draws the pages at 900x645 CSS px, and the log at 680x487, at scale 2.

    capture_overview.py FIXTURE OUT ROTARI SAMPLES

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
HERE = os.path.dirname(os.path.abspath(__file__))
SANDBOX = os.environ["PROMO_SLURM_SB"]
HOME = "/home/me"
EXP = f"{HOME}/resnet"
PORT = 18812
BASE = f"http://127.0.0.1:{PORT}"
VIEW = {"width": 900, "height": 645}
CLOSE_VIEW = {"width": 680, "height": 487}
HAND_JOBS = 30

ENV = {
    "PATH": f"{HOME}/.local/bin:{HOME}/sbin:/usr/bin:/bin",
    "HOME": HOME,
    "USER": "me",
    "LOGNAME": "me",
    "TERM": "xterm-256color",
    "COLUMNS": "70",
    "LC_ALL": "C",
    "TZ": "UTC",
    "ROTARI_MASTERDIR": f"{HOME}/.rotari-master",
}
S = Session(ENV, HOME, OUT, workdir=EXP)

# Show only these columns of the run page's jobs table: the Columns menu
# keeps its choices in localStorage, and every choice re-renders the menu.
COLUMNS = """[...document.querySelectorAll('input[data-column]')].map((box) => box.dataset.column)
  .forEach((key) => {
    const choices = JSON.parse(localStorage.getItem('rotari-run-columns') || '{}');
    choices[key] = ['hosts', 'executor', 'duration'].includes(key);
    localStorage.setItem('rotari-run-columns', JSON.stringify(choices));
  })"""
# Scroll the jobs table near the top of the window, and to its first column.
TO_JOBS = """(() => {
  document.querySelectorAll('*').forEach((e) => { if (e.scrollLeft) e.scrollLeft = 0; });
  const t = document.querySelector('table.runs');
  window.scrollTo(0, t.getBoundingClientRect().top + scrollY - 110);
})()"""


def setup():
    os.makedirs(f"{HOME}/.local/bin", exist_ok=True)
    shutil.copy(ROTARI, f"{HOME}/.local/bin/rotari")
    os.makedirs(EXP, exist_ok=True)
    os.makedirs(f"{HOME}/bert", exist_ok=True)
    for f in ("train.sh", "train.conf"):
        shutil.copy(f"{FIXTURE}/overview/{f}", EXP)
    shutil.copy(f"{FIXTURE}/overview/old.sh", f"{EXP}/train_v1.sh")
    shutil.rmtree(OUT, ignore_errors=True)
    os.makedirs(f"{OUT}/web")
    with open(f"{OUT}/slurm-up.log", "w") as log:
        subprocess.run(
            [f"{HERE}/slurm-up.sh", SANDBOX, f"{HOME}/sbin"],
            stdout=log,
            stderr=subprocess.STDOUT,
            check=True,
        )
    # Last week, by hand: thirty sbatch submissions and their output files.
    for _ in range(HAND_JOBS):
        S.sh("sbatch train_v1.sh")
    for _ in range(120):
        if not S.sh("squeue -h").stdout.strip():
            break
        time.sleep(1)
    # Another project with a finished run, so the Web UI lists more than one.
    S.sh(
        "rotari add -p bert -e slurm --job-name finetune"
        " -- sh -c 'echo finetune bert on $(hostname); sleep 2; echo val_f1=0.91'"
        " && rotari run -p bert --quiet",
        cwd=f"{HOME}/bert",
    )


def main():
    setup()
    terms, frames, logs = [], {}, {"view": VIEW}

    # The problem: a directory full of slurm-N.out, and which one was which?
    t = S.term("before")
    t.run("ls", real="ls -C", line_s=0.03)
    t.run("grep -l Error slurm-*.out")
    terms.append(t)

    web = subprocess.Popen(
        [f"{HOME}/.local/bin/rotari", "web", "--port", str(PORT)],
        cwd=HOME,
        env=ENV,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    time.sleep(1.5)
    try:
        with sync_playwright() as p:
            b = p.chromium.launch(args=["--no-sandbox"])
            page = b.new_page(viewport=VIEW, device_scale_factor=2, color_scheme="dark")

            # With rotari: the same script, handed to it for Slurm.
            t = S.term("submit")
            t.run(
                "rotari add -p resnet -e slurm --job-name train"
                " --matrix LR=1e-3,3e-4,1e-4 --matrix BS=64,128 ./train.sh",
                cps=40,
                keep=only("added"),
            )
            t.run(
                "rotari run -p resnet",
                keep=only("Submitted", "progress", "Summary"),
                line_s=0.08,
            )
            terms.append(t)
            first = S.runs_of("resnet")[-1]

            # Every project, then the sweep's jobs.
            page.goto(f"{BASE}/")
            page.wait_for_timeout(1500)
            S.record(page, frames, "projects", 0.5, fps=4)
            page.goto(f"{BASE}/project/resnet/run/{first}")
            page.wait_for_selector("table.runs")
            page.evaluate(COLUMNS)
            page.reload()
            page.wait_for_selector("table.runs")
            page.wait_for_timeout(1200)
            page.evaluate(TO_JOBS)
            page.wait_for_timeout(300)
            S.record(page, frames, "jobs", 0.5, fps=4)

            # A failed job's log, in a smaller window so that it reads.
            close = b.new_page(
                viewport=CLOSE_VIEW, device_scale_factor=2, color_scheme="dark"
            )
            close.goto(f"{BASE}/project/resnet/run/{first}")
            close.wait_for_selector("table.runs")
            close.wait_for_timeout(1200)
            failed = (
                close.locator("table.runs tbody tr").filter(has_text="failed").first
            )
            failed.get_by_role("button", name="Log", exact=True).click()
            close.wait_for_timeout(1500)
            S.record(close, frames, "log", 0.5, fps=4)
            close.close()

            # The fix, and a retry of only the failed jobs.
            t = S.term("retry")
            t.run("cat train.conf")
            t.run("sed -i 's/fp32/bf16/' train.conf")
            t.run(
                "rotari retry -p resnet",
                keep=only("Submitted", "Excluded", "progress", "Summary", "Carried"),
                line_s=0.08,
            )
            terms.append(t)
            second = S.runs_of("resnet")[-1]
            page.goto(f"{BASE}/project/resnet/run/{second}")
            page.wait_for_selector("table.runs")
            page.wait_for_timeout(1500)
            page.evaluate(TO_JOBS)
            page.wait_for_timeout(300)
            S.record(page, frames, "retried", 0.5, fps=4)
            b.close()
    finally:
        web.terminate()

    Path(f"{OUT}/terminals.json").write_text(
        json.dumps([t.dump() for t in terms], indent=1)
    )
    Path(f"{OUT}/web.json").write_text(json.dumps(frames, indent=1))
    Path(f"{OUT}/logs.json").write_text(json.dumps(logs, indent=1))


main()
