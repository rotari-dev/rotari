"""Capture material for the terminal episode: inspecting runs without a browser.

Run through `capture-web.sh WORK ROTARI SAMPLES capture_terminal.py` (no
display needed; no browser is used). An eight-seed sweep runs in the
background (fixture/terminal/train.sh: seed 3 runs out of GPU memory, seed 6
diverges, seed 8 is still training) after a finished run of another project.
While it runs, the terminal shows what you would type on a cluster's login
node: `rotari jobs`, `rotari show`, every job's last log line, the failed
jobs' logs, and following the running job's log.

    capture_terminal.py FIXTURE OUT ROTARI SAMPLES

Writes OUT/terminals.json, OUT/web.json (empty), and OUT/logs.json.
"""

import json
import os
import shutil
import sys
import time
from pathlib import Path

from common import Session, section

FIXTURE, OUT, ROTARI, SAMPLES = sys.argv[1:5]
HOME = "/home/me"
EXP = f"{HOME}/sweep"

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
    "ROTARI_RUN_LOCAL_CONCURRENCY": "8",
}
S = Session(ENV, HOME, OUT, workdir=EXP)

# Log views: each job's header line and its log lines, without the working
# directory, command, and log path printed under each header, or the project
# header above the first job.
DETAIL = ("Working directory:", "Command:", "OUTPUT:", "STDOUT:", "STDERR:")


def logs_only(lines):
    out, on = [], False
    for ln in lines:
        on = on or ln.startswith("=== Job:")
        if on and ln.strip() and not ln.startswith(DETAIL):
            out.append(ln)
    return out


def setup():
    os.makedirs(f"{HOME}/.local/bin", exist_ok=True)
    shutil.copy(ROTARI, f"{HOME}/.local/bin/rotari")
    for d in ("sweep", "asr"):
        os.makedirs(f"{HOME}/{d}", exist_ok=True)
        shutil.copy(f"{FIXTURE}/terminal/train.sh", f"{HOME}/{d}")
    shutil.rmtree(OUT, ignore_errors=True)
    os.makedirs(OUT)
    S.sh(
        "rotari add -p asr --job-name train --array 1-2 ./train.sh"
        " && rotari run -p asr --quiet",
        cwd=f"{HOME}/asr",
    )
    S.sh("rotari add -p sweep --job-name train --array 1-8 ./train.sh")
    S.sh("rotari run -p sweep --async --quiet")
    # Every seed but the eighth has finished after about four seconds.
    time.sleep(6)


def main():
    setup()
    terms, logs = [], {}

    t = S.term("jobs")
    t.run("rotari jobs", line_s=0.05)
    terms.append(t)

    t = S.term("show")
    t.run(
        "rotari show -p sweep",
        keep=section("Jobs:", ("To show a job",)),
        line_s=0.04,
    )
    terms.append(t)

    t = S.term("metrics")
    t.run("rotari show -p sweep --logs --tail 1", keep=logs_only, line_s=0.05)
    terms.append(t)

    t = S.term("failed")
    t.run("rotari show -p sweep --failed-logs --tail 2", keep=logs_only, line_s=0.05)
    terms.append(t)

    t = S.term("follow")
    t.live(
        "rotari show -p sweep --job-name 'train[8]' --follow",
        real="timeout 6 rotari show -p sweep --job-name 'train[8]' --follow --no-pager",
    )
    terms.append(t)
    S.sh("rotari cancel -p sweep --yes --wait")

    Path(f"{OUT}/terminals.json").write_text(
        json.dumps([t.dump() for t in terms], indent=1)
    )
    Path(f"{OUT}/web.json").write_text("{}")
    Path(f"{OUT}/logs.json").write_text(json.dumps(logs, indent=1))


main()
