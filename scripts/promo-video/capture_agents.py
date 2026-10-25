"""Capture material for the agents episode: the CLI guide, notes, and MCP.

Run through `capture-web.sh WORK ROTARI SAMPLES capture_agents.py` (no
display needed). No agent runs here: the terminal shows what a coding agent
reads (`rotari guide`) and the commands the guide has it use, run for real
on the lineage episode's sweep in a git repository: `run --async --note`,
`wait --until-failure`, `lineage RUN_ID`, then a fix, `retry --note`, and a
concluding note with a Markdown table of the results. A headless Chrome
(680x487 CSS px at scale 2) records the run's report with that note, and the
terminal ends on the VS Code entry that starts `rotari mcp`.

    capture_agents.py FIXTURE OUT ROTARI SAMPLES

Writes OUT/terminals.json, OUT/web.json, OUT/logs.json, and
OUT/web/<scene>/*.png.
"""

import json
import os
import re
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
PORT = 18807
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

# Leave out the long show: and retry: command lines under each failure cause.
def BRIEF(lines):
    return [ln for ln in lines if not ln.lstrip().startswith(("show:", "retry:"))]


MCP_JSON = """{
  "servers": {
    "rotari": {
      "type": "stdio",
      "command": "rotari",
      "args": ["mcp"]
    }
  }
}
"""


def setup():
    os.makedirs(f"{HOME}/.local/bin", exist_ok=True)
    shutil.copy(ROTARI, f"{HOME}/.local/bin/rotari")
    os.makedirs(f"{EXP}/.vscode", exist_ok=True)
    for f in ("train.sh", "config.sh"):
        shutil.copy(f"{FIXTURE}/lineage/{f}", EXP)
    Path(f"{EXP}/.vscode/mcp.json").write_text(MCP_JSON)
    shutil.rmtree(OUT, ignore_errors=True)
    os.makedirs(f"{OUT}/web")
    S.sh("git init -q && git add . && git commit -qm 'sweep: batch 128, lr 1e-3'")


def results_table(run_id):
    """A Markdown table of each task's final loss, from the run's logs."""
    logs = S.sh(f"rotari show -r {run_id} --logs --tail 1 --no-pager").stdout
    rows = sorted(set(re.findall(r"task (\d+): eval_loss=([0-9.]+)", logs)), key=lambda r: int(r[0]))
    return "| task | eval loss |\n| --- | --- |\n" + "".join(
        f"| train[{task}] | {loss} |\n" for task, loss in rows
    )


def main():
    setup()
    terms, frames, logs = [], {}, {"view": VIEW}

    t = S.term("guide")
    t.run("rotari guide | head -n 30", line_s=0.03)
    terms.append(t)

    t = S.term("run")
    t.run("rotari add -p sweep --job-name train --array 1-6 ./train.sh", keep=only("added"))
    t.run(
        "rotari run -p sweep --async --note 'baseline: batch 128'",
        keep=only("=== Run started", "Project:"),
    )
    # From where wait returns: the run's failures so far, grouped by cause.
    logs["wait"] = t.run(
        "rotari wait sweep --until-failure",
        keep=lambda lines: BRIEF(section("Run ", ("To keep waiting",))(lines)),
        line_s=0.06,
    )
    first = S.runs_of("sweep")[-1]
    S.wait_idle("sweep")
    logs["lineage"] = t.run(
        f"rotari lineage {first}",
        keep=lambda lines: BRIEF(section("Summary", ("Origins",))(lines)),
        cps=60,
        line_s=0.05,
    )
    terms.append(t)

    t = S.term("fix")
    t.run("sed -i 's/BATCH=128/BATCH=64/' config.sh && git commit -qam 'batch 64'")
    t.run(
        "rotari retry -p sweep --note 'batch 64 to fit in GPU memory'",
        keep=only("Submitted", "Excluded", "Summary", "Carried"),
        line_s=0.08,
    )
    second = S.runs_of("sweep")[-1]
    Path(f"{EXP}/results.md").write_text(results_table(second))
    t.run("cat results.md", line_s=0.05)
    t.run(f'rotari note {second} "$(cat results.md)"', cps=60)
    terms.append(t)

    t = S.term("mcp")
    t.run("cat .vscode/mcp.json", line_s=0.04)
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
            # The report is drawn in a smaller window with the frame's shape,
            # so its notes table comes out larger.
            page = b.new_page(
                viewport={"width": 680, "height": 487},
                device_scale_factor=2,
                color_scheme="dark",
            )
            page.goto(f"{BASE}/project/sweep/run/{second}")
            page.wait_for_selector("table.runs")
            page.wait_for_timeout(1200)
            # The run report covers the selected jobs.
            page.get_by_role("button", name="Select all").first.click()
            page.wait_for_timeout(300)
            page.get_by_role("button", name="Report").first.click()
            page.wait_for_timeout(1500)
            # The notes, with the results table, at the top of the report.
            page.locator("#output-modal h2", has_text="Notes").first.evaluate(
                "e => e.scrollIntoView({block: 'start'})"
            )
            page.wait_for_timeout(500)
            S.record(page, frames, "report", 0.5, fps=4)
            page.screenshot(path=f"{OUT}/report_full.png", full_page=True)
            b.close()
    finally:
        web.terminate()

    Path(f"{OUT}/terminals.json").write_text(
        json.dumps([t.dump() for t in terms], indent=1)
    )
    Path(f"{OUT}/web.json").write_text(json.dumps(frames, indent=1))
    Path(f"{OUT}/logs.json").write_text(json.dumps(logs, indent=1))


main()
