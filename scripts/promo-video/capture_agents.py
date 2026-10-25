"""Capture material for the agents episode: what your agent did, and why.

Run through `capture-web.sh WORK ROTARI SAMPLES capture_agents.py` (no
display needed). fixture/tune/session.sh holds the commands Claude Code ran
on 2026-10-25 when asked to tune a sweep with rotari after reading `rotari
guide`: four runs, each started with a note on why and noted afterwards with
what it showed, ending with a table of results. The capture replays them in
~/sweep with ROTARI_ACTOR=claude-code, off camera, then shows them as the
person who asked would read them: `rotari lineage` (history with notes),
a comparison of the first two runs (the agent's reasoning between them), the
last run's page and report (Started by, notes, the results table), and the
one line in AGENTS.md that set it up. A headless Chrome draws the Web UI at
680x487 CSS px (scale 2) so the notes read in the frame.

    capture_agents.py FIXTURE OUT ROTARI SAMPLES

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

from common import Session, section
from playwright.sync_api import sync_playwright

FIXTURE, OUT, ROTARI, SAMPLES = sys.argv[1:5]
HOME = "/home/me"
EXP = f"{HOME}/sweep"
PORT = 18807
BASE = f"http://127.0.0.1:{PORT}"
VIEW = {"width": 680, "height": 487}

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
# The history table; the project header above it is left out.
LINEAGE = section("Runs (oldest first)")


def setup():
    os.makedirs(f"{HOME}/.local/bin", exist_ok=True)
    shutil.copy(ROTARI, f"{HOME}/.local/bin/rotari")
    os.makedirs(EXP, exist_ok=True)
    for f in ("train.sh", "config.sh", "AGENTS.md"):
        shutil.copy(f"{FIXTURE}/tune/{f}", EXP)
    shutil.rmtree(OUT, ignore_errors=True)
    os.makedirs(f"{OUT}/web")
    # The agent's session, as it ran it.
    r = subprocess.run(
        ["sh", f"{FIXTURE}/tune/session.sh"],
        cwd=EXP,
        env={**ENV, "ROTARI_ACTOR": "claude-code"},
        capture_output=True,
        text=True,
    )
    Path(f"{OUT}/session.log").write_text(r.stdout + r.stderr)


def main():
    setup()
    terms, frames, logs = [], {}, {"view": VIEW}
    runs = S.runs_of("sweep")
    if len(runs) != 4:
        raise SystemExit(f"the agent's session made {len(runs)} runs, not 4")

    t = S.term("history")
    t.run("rotari lineage -p sweep", keep=LINEAGE, line_s=0.06)
    terms.append(t)
    t = S.term("why")
    t.run(f"rotari lineage -p sweep {runs[0]} {runs[1]}", cps=60, line_s=0.06)
    terms.append(t)
    t = S.term("agents")
    t.run("cat AGENTS.md")
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
            page.goto(f"{BASE}/project/sweep/run/{runs[-1]}")
            page.wait_for_selector("table.runs")
            page.wait_for_timeout(1200)
            S.record(page, frames, "run", 0.5, fps=4)
            # The run report covers the selected jobs; its notes come first.
            page.get_by_role("button", name="Select all").first.click()
            page.wait_for_timeout(300)
            page.get_by_role("button", name="Report").first.click()
            page.wait_for_timeout(1500)
            page.locator("#output-modal h2", has_text="Notes").first.evaluate(
                "e => e.scrollIntoView({block: 'start'})"
            )
            page.wait_for_timeout(500)
            S.record(page, frames, "report", 0.5, fps=4)
            b.close()
    finally:
        web.terminate()

    Path(f"{OUT}/terminals.json").write_text(
        json.dumps([t.dump() for t in terms], indent=1)
    )
    Path(f"{OUT}/web.json").write_text(json.dumps(frames, indent=1))
    Path(f"{OUT}/logs.json").write_text(json.dumps(logs, indent=1))


main()
