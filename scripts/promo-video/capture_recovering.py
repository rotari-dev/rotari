"""Capture material for the recovering episode: failure causes, diagnosis, retry.

Run through `capture-web.sh WORK ROTARI SAMPLES capture_recovering.py` (no
display needed). A 48-run sweep fails 8 tasks for two causes (fixture/sweep.sh:
a GPU fault on the first attempt of three tasks, CUDA out of memory on five
while sweep.conf's max_batch is above 64). The terminal shows the failures
grouped by cause, the Web UI the run's Failure causes and one job's
Diagnosis, then the fix, `retry --dry-run`, and `retry`. A headless Chrome
draws the Web UI 1100x782 CSS px at scale 2 for the landscape frame, and the
Diagnosis dialog 820x588 so that it reads at that size.

    capture_recovering.py FIXTURE OUT ROTARI SAMPLES

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
EXP = f"{HOME}/llm-sweep"
PORT = 18796
BASE = f"http://127.0.0.1:{PORT}"
VIEW = {"width": 1100, "height": 782}
# The Diagnosis dialog is drawn in a smaller window with the frame's shape,
# so its text comes out larger.
CLOSE_VIEW = {"width": 820, "height": 588}

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

# Page positions (CSS px from the top) that the renderer pans between.
OFFSETS = """(() => {
  const top = (el) => (el ? el.getBoundingClientRect().top + scrollY : null);
  const byText = (t) => [...document.querySelectorAll('*')]
    .find((e) => e.children.length === 0 && e.textContent.trim() === t);
  return {
    cards: top(document.querySelector('.run-cards')),
    causes: top(byText('Failure causes')),
    timeline: top(byText('Job timeline')),
    jobs: top(document.querySelector('table.runs')),
    height: document.documentElement.scrollHeight,
  };
})()"""


def setup():
    os.makedirs(f"{HOME}/.local/bin", exist_ok=True)
    shutil.copy(ROTARI, f"{HOME}/.local/bin/rotari")
    os.makedirs(EXP, exist_ok=True)
    for f in ("sweep.sh", "sweep.conf"):
        shutil.copy(f"{FIXTURE}/{f}", EXP)
    shutil.rmtree(OUT, ignore_errors=True)
    os.makedirs(f"{OUT}/web")


def page_shot(page, frames, logs, name):
    """A full-page shot of page, with its pan positions in logs."""
    page.wait_for_timeout(1500)
    logs[f"offsets_{name}"] = page.evaluate(OFFSETS)
    S.record(page, frames, name, 0.1, full=True)


def main():
    setup()
    terms, frames, logs = [], {}, {"view": VIEW}

    # The sweep, run to the end.
    t = S.term("sweep")
    t.run("rotari add -p llm-sweep --array 1-48 ./sweep.sh", keep=only("added"))
    t.run("rotari run -p llm-sweep", keep=only("Summary", "Status"), line_s=0.08)
    terms.append(t)
    logs["run_full"] = S.sh("rotari show -p llm-sweep").stdout
    failed = S.sh("rotari show -p llm-sweep --failed-logs").stdout
    for key, marker in (("gpu", "fallen off the bus"), ("oom", "OutOfMemoryError")):
        logs[key] = next(ln for ln in failed.splitlines() if marker in ln)

    # The failures, grouped by cause.
    t = S.term("causes")
    causes = section("Failures by cause", ("Inspect", "To show"))
    t.run(
        "rotari show -p llm-sweep",
        # The show: and retry: lines are long commands; the Web UI shows them.
        keep=lambda lines: [
            ln
            for ln in causes(lines)
            if ln.strip() and not ln.lstrip().startswith(("show:", "retry:"))
        ],
        line_s=0.06,
    )
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
            rid = S.runs_of("llm-sweep")[-1]
            page.goto(f"{BASE}/project/llm-sweep/run/{rid}")
            page_shot(page, frames, logs, "failed_run")

            # One failed job's Diagnosis: its evidence and next step.
            close = b.new_page(
                viewport=CLOSE_VIEW, device_scale_factor=2, color_scheme="dark"
            )
            close.goto(f"{BASE}/project/llm-sweep/run/{rid}")
            close.wait_for_timeout(1500)
            close.get_by_role("button", name="Diagnosis").and_(
                close.locator(":enabled")
            ).first.click()
            close.wait_for_timeout(1200)
            S.record(close, frames, "diagnosis", 0.5, fps=4)
            close.close()

            # Fix the batch limit, check what would rerun, then rerun it.
            t = S.term("retry")
            t.run("sed -i 's/max_batch=128/max_batch=64/' sweep.conf")
            logs["dry_run"] = t.run(
                "rotari retry -p llm-sweep --dry-run",
                keep=lambda lines: [ln for ln in lines if not ln.startswith("revision=")],
            )
            terms.append(t)
            t = S.term("retry_run")
            t.run(
                "rotari retry -p llm-sweep",
                keep=only(
                    "Submitted", "Excluded", "progress", "Summary", "Carried", "Status"
                ),
                line_s=0.08,
            )
            terms.append(t)
            rid2 = S.runs_of("llm-sweep")[-1]
            page.goto(f"{BASE}/project/llm-sweep/run/{rid2}")
            page_shot(page, frames, logs, "retry_run")
            b.close()
    finally:
        web.terminate()

    Path(f"{OUT}/terminals.json").write_text(
        json.dumps([t.dump() for t in terms], indent=1)
    )
    Path(f"{OUT}/web.json").write_text(json.dumps(frames, indent=1))
    Path(f"{OUT}/logs.json").write_text(json.dumps(logs, indent=1))


main()
