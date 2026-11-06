#!/usr/bin/env python3
"""Wait for a commit's GitHub Actions runs to finish, and say how they ended.

Usage: watch_ci.py SHA [--timeout MINUTES]

Reads GitHub's public API without signing in, so it cannot read job logs or
start a run again. Exits 0 if every run succeeded, 1 if any did not, and 2 if
it could not tell before the timeout.
"""

import argparse
import json
import sys
import time
import urllib.request

REPO = "rotari-dev/rotari"
POLL_SECONDS = 150
# Workflows can register late, so finished runs are looked at again before
# being believed. A commit whose runs finished long ago costs only this wait.
SETTLE_SECONDS = 30
HEADERS = {"Accept": "application/vnd.github+json", "User-Agent": "rotari-watch-ci"}


def get(url):
    request = urllib.request.Request(url, headers=HEADERS)
    with urllib.request.urlopen(request, timeout=30) as response:
        if "api.github.com" not in response.geturl():
            raise OSError("redirected to " + response.geturl().split("?")[0])
        return json.load(response)


def runs_of(sha):
    url = f"https://api.github.com/repos/{REPO}/actions/runs?head_sha={sha}&per_page=100"
    return get(url)["workflow_runs"]


def report(runs):
    failed = False
    for run in sorted(runs, key=lambda item: item["name"]):
        print(run["name"], run["conclusion"], f"attempt {run['run_attempt']}", run["html_url"])
        failed = failed or run["conclusion"] != "success"
        for job in get(run["jobs_url"])["jobs"]:
            print("  job", job["name"], job["conclusion"])
            if job["conclusion"] == "success":
                continue
            for step in job.get("steps", []):
                if step["conclusion"] not in ("success", "skipped"):
                    print("    step", step["name"], step["conclusion"])
    return failed


def main():
    parser = argparse.ArgumentParser(description="Wait for a commit's CI to finish.")
    parser.add_argument("sha", help="the full commit ID, as pushed")
    parser.add_argument("--timeout", type=int, default=50, help="minutes to wait (default 50)")
    options = parser.parse_args()
    if len(options.sha) != 40 or not all(c in "0123456789abcdef" for c in options.sha):
        parser.error("SHA must be a full commit ID: git rev-parse <revision>")

    deadline = time.time() + options.timeout * 60
    finished = None
    problem = "no runs were found"
    while time.time() < deadline:
        try:
            runs = runs_of(options.sha)
            names = sorted(run["id"] for run in runs)
            if runs and all(run["status"] == "completed" for run in runs):
                if finished == names:
                    sys.exit(1 if report(runs) else 0)
                finished = names
                time.sleep(SETTLE_SECONDS)
                continue
            finished = None
            problem = f"{sum(run['status'] != 'completed' for run in runs)} of {len(runs)} runs unfinished"
        except (OSError, ValueError, KeyError) as error:
            # A network's sign-in page answers with something that is not the API.
            finished = None
            problem = f"cannot read the API: {error}"
            print(problem, file=sys.stderr, flush=True)
        time.sleep(POLL_SECONDS)
    print(f"timed out: {problem}", file=sys.stderr)
    sys.exit(2)


if __name__ == "__main__":
    main()
