"""Capture material for the Web UI episode: every project and state directory.

Run through `capture-web.sh WORK ROTARI SAMPLES capture_webui.py` (no display
needed). Four projects live in their own directories: asr, vision, and nlp
keep their state in the default state directory, and tts is a rotari
workspace with its own (`rotari init`). Their history is made off camera:
asr and tts each ran out of GPU memory once and then succeeded, vision is
missing a data file, and nlp's evaluation finished while its training is
still running. The terminal starts `rotari web`; a headless Chrome (960x688
CSS px at scale 2) records All projects, Activity, Job activity, and a
History search for the jobs that ran out of memory in either state
directory.

    capture_webui.py FIXTURE OUT ROTARI SAMPLES

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

from common import Session
from playwright.sync_api import sync_playwright

FIXTURE, OUT, ROTARI, SAMPLES = sys.argv[1:5]
HOME = "/home/me"
PORT = 18802
BASE = f"http://127.0.0.1:{PORT}"
VIEW = {"width": 960, "height": 688}
PROJECTS = ["asr", "nlp", "tts", "vision"]

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
}
S = Session(ENV, HOME, OUT)


def add_run(project, outcome, *extra):
    """Add one training job with OUTCOME to project and run it."""
    cwd = f"{HOME}/{project}"
    options = " ".join(extra)
    r = S.sh(
        f"rotari add -p {project} --job-name train --env OUTCOME={outcome}"
        f" {options} ./train.sh && rotari run -p {project} --quiet",
        cwd=cwd,
    )
    return r


def setup():
    os.makedirs(f"{HOME}/.local/bin", exist_ok=True)
    shutil.copy(ROTARI, f"{HOME}/.local/bin/rotari")
    for project in PROJECTS:
        os.makedirs(f"{HOME}/{project}", exist_ok=True)
        shutil.copy(f"{FIXTURE}/webui/train.sh", f"{HOME}/{project}")
    S.sh("rotari init", cwd=f"{HOME}/tts")
    shutil.rmtree(OUT, ignore_errors=True)
    os.makedirs(f"{OUT}/web")

    # The history, off camera.
    add_run("asr", "oom")
    add_run("vision", "missing")
    add_run("tts", "oom")
    S.sh(
        "rotari add -p nlp --job-name eval --array 1-4 --env OUTCOME=ok ./train.sh"
        " && rotari run -p nlp --quiet",
        cwd=f"{HOME}/nlp",
    )
    add_run("asr", "ok", "--env BATCH=32")
    add_run("tts", "ok", "--env BATCH=32")
    S.sh(
        "rotari add -p nlp --job-name train --env OUTCOME=long ./train.sh"
        " && rotari run -p nlp --async --quiet",
        cwd=f"{HOME}/nlp",
    )
    time.sleep(3)


def main():
    setup()
    terms, frames, logs = [], {}, {"view": VIEW}

    t = S.term("start")
    t.run("ls", real="ls -C")
    # The terminal shows rotari web's own start-up lines on its default
    # port; the server the browser uses below runs on PORT for the capture.
    t.run("rotari web", real="timeout 3 rotari web")
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

            page.goto(f"{BASE}/")
            page.wait_for_timeout(1500)
            # Open the workspace's state directory in the sidebar too.
            page.get_by_text("tts", exact=True).first.click()
            page.wait_for_timeout(800)
            page.goto(f"{BASE}/")
            page.wait_for_timeout(1500)
            S.record(page, frames, "home", 0.5, fps=4)

            page.get_by_text("Activity", exact=True).first.click()
            page.wait_for_timeout(2000)
            S.record(page, frames, "activity", 0.5, fps=4)
            page.get_by_text("Where you worked", exact=True).first.evaluate(
                "e => window.scrollTo(0, e.getBoundingClientRect().top + scrollY - 80)"
            )
            page.wait_for_timeout(500)
            S.record(page, frames, "where", 0.5, fps=4)

            page.goto(f"{BASE}/")
            page.wait_for_timeout(1200)
            page.get_by_text("Job activity", exact=True).first.click()
            page.wait_for_timeout(2000)
            S.record(page, frames, "jobs", 0.5, fps=4)

            # History search: the jobs that ran out of GPU memory, in both
            # state directories.
            page.goto(f"{BASE}/search/")
            page.wait_for_timeout(1500)
            S.record(page, frames, "search_form", 0.3, fps=4)
            page.get_by_role("button", name="Add search range").click()
            page.wait_for_timeout(500)
            page.locator("select.history-search-scope-basedir").last.select_option(
                label="tts/.rotari-state"
            )
            page.wait_for_timeout(500)
            page.locator("select:has(option:text-is('Jobs'))").select_option(
                label="Jobs"
            )
            page.locator("select.history-search-field").first.select_option(
                label="Job · Diagnosis"
            )
            # For a diagnosis the word is chosen from the diagnosis names.
            page.locator("select.history-search-word").first.select_option(
                label="CUDA/GPU memory exhausted"
            )
            page.locator("select:has(option:text-is('Last 7 days'))").select_option(
                label="Last 7 days"
            )
            page.wait_for_timeout(500)
            S.record(page, frames, "search_query", 0.5, fps=4)
            page.get_by_role("button", name="Search", exact=True).click()
            page.wait_for_timeout(2500)
            results = page.get_by_text("Results", exact=False).last
            if results.count():
                results.evaluate(
                    "e => window.scrollTo(0, e.getBoundingClientRect().top + scrollY - 80)"
                )
            page.wait_for_timeout(500)
            S.record(page, frames, "search_results", 0.5, fps=4)
            page.screenshot(path=f"{OUT}/search_full.png", full_page=True)
            b.close()
    finally:
        web.terminate()
        S.sh("rotari cancel -p nlp --yes --wait", cwd=f"{HOME}/nlp")

    Path(f"{OUT}/terminals.json").write_text(
        json.dumps([t.dump() for t in terms], indent=1)
    )
    Path(f"{OUT}/web.json").write_text(json.dumps(frames, indent=1))
    Path(f"{OUT}/logs.json").write_text(json.dumps(logs, indent=1))


main()
