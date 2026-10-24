"""Capture material for the executors episode: local, SSH, and Slurm in one run.

Run through `capture-web.sh WORK ROTARI SAMPLES capture_executors.py` (no
display needed) with PROMO_SLURM_SB naming the Slurm sandbox from
`setup.sh --with-slurm`. In the namespace the workstation is "workstation";
slurm-up.sh starts a one-node Slurm cluster (gpu-node01) and ssh-up.sh an SSH
server for a lab GPU box (gpu-box), both sharing /home/me. The terminal first
runs a pipeline's three steps by hand, each its own way, then submits them to
rotari as one run with -e local, -e slurm, and -e ssh; a headless Chrome
records the run page (900x645 CSS px at scale 2) while the steps move
across the machines.

    capture_executors.py FIXTURE OUT ROTARI SAMPLES

Writes OUT/terminals.json, OUT/web.json, OUT/logs.json, and
OUT/web/<scene>/*.png.
"""

import glob
import json
import os
import shutil
import subprocess
import sys
import time
from datetime import datetime
from pathlib import Path

from common import Session, only
from playwright.sync_api import sync_playwright

FIXTURE, OUT, ROTARI, SAMPLES = sys.argv[1:5]
HERE = os.path.dirname(os.path.abspath(__file__))
SANDBOX = os.environ["PROMO_SLURM_SB"]
HOME = "/home/me"
EXP = f"{HOME}/pipeline"
PORT = 18797
BASE = f"http://127.0.0.1:{PORT}"
# Smaller than the other episodes' 1100x782 window, with the frame's shape,
# so that the jobs table comes out larger.
VIEW = {"width": 900, "height": 645}
SSH_HOST = "gpu-box"

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
    "SQUEUE_FORMAT": "%.5i %.9P %.10j %.5u %.2t %.5M %R",
}
S = Session(ENV, HOME, OUT, workdir=EXP)

# Show only these columns of the run page's jobs table: the Columns menu
# keeps its choices in localStorage, and every choice re-renders the menu.
COLUMNS = """[...document.querySelectorAll('input[data-column]')].map((box) => box.dataset.column)
  .forEach((key) => {
    const choices = JSON.parse(localStorage.getItem('rotari-run-columns') || '{}');
    choices[key] = ['hosts', 'executor'].includes(key);
    localStorage.setItem('rotari-run-columns', JSON.stringify(choices));
  })"""
# Scroll the jobs table to the top of the window.
TO_JOBS = """(() => {
  const t = document.querySelector('table.runs');
  window.scrollTo(0, t.getBoundingClientRect().top + scrollY - 150);
})()"""


def setup():
    os.makedirs(f"{HOME}/.local/bin", exist_ok=True)
    shutil.copy(ROTARI, f"{HOME}/.local/bin/rotari")
    os.makedirs(EXP, exist_ok=True)
    for f in ("prep.sh", "train-gpu.sh", "eval.sh"):
        shutil.copy(f"{FIXTURE}/{f}", EXP)
    shutil.rmtree(OUT, ignore_errors=True)
    os.makedirs(f"{OUT}/web")
    with open(f"{OUT}/machines.log", "w") as log:
        subprocess.run(
            [f"{HERE}/slurm-up.sh", SANDBOX, f"{HOME}/sbin"],
            stdout=log,
            stderr=subprocess.STDOUT,
            check=True,
        )
        subprocess.run(
            [
                f"{HERE}/ssh-up.sh",
                f"{SANDBOX}/usr/local/lib/nogroups.so",
                SSH_HOST,
                "127.0.0.2",
                f"{os.path.dirname(OUT)}/ssh",
            ],
            env={**os.environ, "HOME": HOME},
            stdout=log,
            stderr=subprocess.STDOUT,
            check=True,
        )


def main():
    setup()
    terms, frames, logs = [], {}, {"view": VIEW}

    # By hand: each machine has its own way to start a job and to check on it.
    t = S.term("byhand")
    t.run("nohup ./prep.sh > prep.log 2>&1 &")
    t.run("sbatch train-gpu.sh")
    t.run(f"ssh {SSH_HOST} 'cd pipeline && nohup ./eval.sh > eval.log 2>&1 &'")
    time.sleep(2.5)
    t.run("tail -n 1 prep.log")
    t.run("squeue", real="squeue | sed 's/ root / me   /'")
    t.run(f"ssh {SSH_HOST} tail -n 1 pipeline/eval.log")
    terms.append(t)
    time.sleep(5)
    S.sh("rm -f prep.log eval.log slurm-*.out nohup.out")

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

            # With rotari: one pipeline, three executors, one command to run it.
            t = S.term("pipeline")
            t.run(
                "rotari add -p pipeline -e local --job-name prep ./prep.sh",
                cps=40,
                keep=only("added"),
            )
            t.run(
                "rotari add -p pipeline -e slurm --job-name train"
                " --depends-on prep ./train-gpu.sh",
                cps=40,
                keep=only("added"),
            )
            t.run(
                f"rotari add -p pipeline -e ssh --executor-option {SSH_HOST}"
                " --job-name eval --depends-on train ./eval.sh",
                cps=40,
                keep=only("added"),
            )
            t.run(
                "rotari run -p pipeline --async",
                keep=only("=== Run started", "Project:"),
            )
            terms.append(t)
            rid = S.runs_of("pipeline")[-1]
            page.goto(f"{BASE}/project/pipeline/run/{rid}")
            page.wait_for_selector("table.runs")
            page.wait_for_timeout(800)
            logs["columns"] = page.evaluate(
                "[...document.querySelectorAll('input[data-column]')].map(b => b.dataset.column)"
            )
            page.evaluate(COLUMNS)
            page.reload()
            page.wait_for_selector("table.runs")
            page.wait_for_timeout(800)
            # By executor (local, slurm, ssh), the pipeline reads in order.
            page.locator('table.runs th[data-sort="executor"]').click()
            page.wait_for_timeout(300)
            page.evaluate(TO_JOBS)
            logs["live_started"] = time.time()
            S.record(page, frames, "live", 24, fps=4)
            S.wait_idle("pipeline")
            # When the SSH step was submitted: until it finishes, the run
            # page shows it as unknown (see development/ISSUES.md), so the
            # video's live scene ends before then.
            run_dir = f"{HOME}/.local/state/rotari/projects/pipeline/runs/{rid}"
            for path in glob.glob(f"{run_dir}/*/attempts/*/job.json"):
                job = json.loads(Path(path).read_text())
                if job["executor"] == "ssh":
                    at = datetime.fromisoformat(job["submitted_at"].replace("Z", "+00:00"))
                    logs["ssh_submitted"] = at.timestamp() - logs["live_started"]
            page.reload()
            page.wait_for_selector("table.runs")
            page.wait_for_timeout(1500)
            page.locator('table.runs th[data-sort="executor"]').click()
            page.wait_for_timeout(300)
            page.evaluate(TO_JOBS)
            S.record(page, frames, "done", 0.5, fps=4)
            logs["show"] = S.sh("rotari show -p pipeline").stdout
            b.close()
    finally:
        web.terminate()

    Path(f"{OUT}/terminals.json").write_text(
        json.dumps([t.dump() for t in terms], indent=1)
    )
    Path(f"{OUT}/web.json").write_text(json.dumps(frames, indent=1))
    Path(f"{OUT}/logs.json").write_text(json.dumps(logs, indent=1))


main()
