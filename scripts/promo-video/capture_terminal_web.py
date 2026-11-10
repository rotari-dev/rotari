"""Capture material for the Web terminal episode: shells that belong to projects.

Run through `capture-web.sh WORK ROTARI SAMPLES capture_terminal_web.py` (no
display needed). Three projects live in their own directories: asr's last run
finished, vision's failed, and tts's eight-seed sweep is still training. Off
camera, the browser starts shells in tts (one training with worker processes,
one in an experiment's output directory) and in vision (an evaluation that
ends while nobody looks). On camera, a headless Chrome (1400x996 CSS px at
scale 2) opens the panel on asr's page, follows the sidebar to tts, opens All
shells, follows the finished evaluation to vision, and launches TensorBoard
from the shell in tts's output directory.

The opening scene, a terminal whose tabs are all called bash, runs the same
programs in plain terminal sessions.

    capture_terminal_web.py FIXTURE OUT ROTARI SAMPLES

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
REPO = str(Path(__file__).resolve().parents[2])
PORT = 18810
BASE = f"http://127.0.0.1:{PORT}"
# The frame column's shape, larger than other episodes' so that All shells,
# which keeps above the panel, shows every project.
VIEW = {"width": 1400, "height": 996}
PROJECTS = ["asr", "tts", "vision"]
RUN_OUT = "outputs/exp0415"

ENV = {
    "PATH": f"{HOME}/.local/bin:{REPO}/.venv/bin:/usr/bin:/bin",
    "HOME": HOME,
    "USER": "me",
    "LOGNAME": "me",
    "SHELL": "/bin/bash",
    "TERM": "xterm-256color",
    "COLUMNS": "70",
    "LC_ALL": "C",
    "TZ": "UTC",
    "ROTARI_MASTERDIR": f"{HOME}/.rotari-master",
    "TTS_SAMPLES": f"{HOME}/.cache/tts-samples",
    "MPLCONFIGDIR": f"{HOME}/.cache/matplotlib",
    # The launcher's runtime directory must be under a directory the user
    # owns; the host's /tmp looks owned by nobody in the namespace.
    "TMPDIR": f"{HOME}/tmp",
    # --allow-terminal and --allow-exec need a token; this one never leaves
    # the namespace.
    "ROTARI_WEB_AUTH_TOKEN": "promo-capture-token-0123456789",
}
S = Session(ENV, HOME, OUT)


class Rec:
    """Frames of one web scene, taken as actions happen, on the wall clock."""

    def __init__(self, page, frames, name):
        self.page, self.frames, self.name = page, frames, name
        self.start = time.time()
        self.n = 0
        self.shots = []
        os.makedirs(f"{OUT}/web/{name}", exist_ok=True)
        frames[name] = self.shots

    def shot(self, page=None):
        path = f"web/{self.name}/{self.n:04d}.png"
        (page or self.page).screenshot(path=f"{OUT}/{path}")
        self.shots.append([round(time.time() - self.start, 3), path])
        self.n += 1

    def hold(self, seconds, fps=5, page=None):
        end = time.time() + seconds
        while time.time() < end:
            self.shot(page)
            time.sleep(max(0, 1 / fps - 0.05))

    def type(self, text, cps=14):
        """Type into the focused shell a key at a time, a frame after each."""
        for ch in text:
            if ch == "\n":
                self.page.keyboard.press("Enter")
            else:
                self.page.keyboard.type(ch)
            time.sleep(1 / cps)
            self.shot()


def setup():
    os.makedirs(f"{HOME}/.local/bin", exist_ok=True)
    os.makedirs(f"{HOME}/tmp", exist_ok=True)
    shutil.copy(ROTARI, f"{HOME}/.local/bin/rotari")
    # `python` as people type it, for the shell's training command.
    os.symlink("/usr/bin/python3", f"{HOME}/.local/bin/python")
    shutil.copytree(SAMPLES, f"{HOME}/.cache/tts-samples", dirs_exist_ok=True)
    Path(f"{HOME}/.bash_profile").write_text(
        "PS1='\\[\\e[32m\\]me@workstation\\[\\e[0m\\]:\\[\\e[34m\\]\\w\\[\\e[0m\\]$ '\n"
    )
    for project in PROJECTS:
        os.makedirs(f"{HOME}/{project}", exist_ok=True)
        shutil.copy(f"{FIXTURE}/terminal_web/train.sh", f"{HOME}/{project}")
    shutil.copy(f"{FIXTURE}/terminal_web/eval.sh", f"{HOME}/vision")
    shutil.copy(f"{FIXTURE}/terminal_web/workers.py", f"{HOME}/tts/train.py")
    shutil.rmtree(OUT, ignore_errors=True)
    os.makedirs(f"{OUT}/web")

    # An earlier experiment's outputs in tts, with TensorBoard scalars.
    S.sh(
        f"{REPO}/.venv/bin/python {FIXTURE}/tts_train.py --epochs 1 --out {RUN_OUT}",
        cwd=f"{HOME}/tts",
    )
    # The history, off camera.
    for project, outcome, extra in [
        ("asr", "ok", ""),
        ("vision", "ok", ""),
        ("vision", "missing", ""),
    ]:
        S.sh(
            f"rotari add -p {project} --job-name train --env OUTCOME={outcome} {extra}"
            f" ./train.sh && rotari run -p {project} --quiet",
            cwd=f"{HOME}/{project}",
        )
    S.sh(
        "rotari add -p tts --job-name sweep --array 1-8 --env OUTCOME=long ./train.sh"
        " && rotari run -p tts --async --quiet",
        cwd=f"{HOME}/tts",
    )
    time.sleep(3)


def opening():
    """The plain terminal: every tab is bash, and each runs one experiment."""
    tabs = []
    for project, line, real in [
        ("asr", "./train.sh", None),
        ("tts", "python train.py --workers 3", "timeout 7 python train.py --workers 3"),
        ("vision", "./eval.sh", None),
        ("tts", f"ls {RUN_OUT}", None),
    ]:
        t = S.term(f"tab_{len(tabs)}", cwd=f"{HOME}/{project}")
        t.run(line, real=real)
        tabs.append(t)
    return tabs


def wait_for_port(web):
    import socket

    for _ in range(100):
        if web.poll() is not None:
            raise SystemExit(
                "rotari web stopped: " + Path(f"{OUT}/web.log").read_text()
            )
        with socket.socket() as sock:
            if sock.connect_ex(("127.0.0.1", PORT)) == 0:
                return
        time.sleep(0.1)
    raise SystemExit("rotari web did not answer: " + Path(f"{OUT}/web.log").read_text())


def sign_in(page):
    page.goto(f"{BASE}/_auth/login?next=/")
    page.fill('input[name="password"]', ENV["ROTARI_WEB_AUTH_TOKEN"])
    page.locator("form button, form input[type=submit]").first.click()
    page.wait_for_load_state()
    if "/_auth/login" in page.url:
        raise SystemExit(f"sign-in failed: {page.url}")


def bar_button(page, label):
    return page.locator(f'.terminal-bar button[aria-label="{label}"]').first


def open_panel(page):
    if not page.locator(".terminal-bar").first.is_visible():
        page.locator("#terminal-button").click()
    page.wait_for_timeout(800)
    # Following a link with the panel open starts nothing; the panel offers to.
    start = page.get_by_role("button", name="Start a shell")
    if start.count() and start.first.is_visible():
        start.first.click()
    page.locator(".xterm:visible").first.wait_for(timeout=15000)
    page.wait_for_timeout(1200)


def focus_shell(page):
    page.locator(".xterm:visible").first.click()
    page.wait_for_timeout(200)


def shell_line(page, text, wait=800):
    focus_shell(page)
    page.keyboard.type(text)
    page.keyboard.press("Enter")
    page.wait_for_timeout(wait)


def main():
    setup()
    tabs = opening()
    frames, logs = {}, {"view": VIEW}

    with open(f"{OUT}/web.log", "w") as log:
        web = subprocess.Popen(
            [
                f"{HOME}/.local/bin/rotari",
                "web",
                "--port",
                str(PORT),
                "--allow-terminal",
                "--allow-exec",
            ],
            cwd=HOME,
            env=ENV,
            stdout=subprocess.DEVNULL,
            stderr=log,
        )
        wait_for_port(web)
        try:
            with sync_playwright() as p:
                b = p.chromium.launch(args=["--no-sandbox"])
                ctx = b.new_context(
                    viewport=VIEW, device_scale_factor=2, color_scheme="dark"
                )
                # A low panel leaves the page, and All shells over it, the room.
                ctx.add_init_script(
                    "try { localStorage.setItem('rotari-terminal-height', '230') } catch {}"
                )
                page = ctx.new_page()
                sign_in(page)
                try:
                    scenes(page, ctx, frames)
                except Exception:
                    for n, open_page in enumerate(ctx.pages):
                        open_page.screenshot(path=f"{OUT}/failure-{n}.png")
                    raise
                b.close()
        finally:
            web.terminate()
            S.sh("rotari cancel -p tts --yes --wait", cwd=f"{HOME}/tts")

    Path(f"{OUT}/terminals.json").write_text(
        json.dumps([t.dump() for t in tabs], indent=1)
    )
    Path(f"{OUT}/web.json").write_text(json.dumps(frames, indent=1))
    Path(f"{OUT}/logs.json").write_text(json.dumps(logs, indent=1))


def scenes(page, ctx, frames):
    """The browser's part: shells off camera, then each recorded scene."""
    # Off camera: tts gets a shell training with workers and one in an
    # earlier experiment's outputs; vision one evaluating.
    page.goto(f"{BASE}/project/tts")
    open_panel(page)
    shell_line(page, "clear; python train.py --workers 3", wait=1500)
    bar_button(page, "New shell").click()
    page.wait_for_timeout(1500)
    shell_line(page, f"clear; cd {RUN_OUT}; ls")
    # tts's page shows its training shell when the camera reaches it.
    page.locator('.terminal-bar [role="tab"]').first.click()
    page.wait_for_timeout(800)
    page.goto(f"{BASE}/project/vision")
    open_panel(page)
    shell_line(page, "clear; ./eval.sh", wait=300)
    # The panel starts closed on asr's page, which has no shell.
    page.locator("#terminal-button").click()
    page.wait_for_timeout(500)
    page.goto(f"{BASE}/project/asr")
    page.wait_for_timeout(1500)

    # On camera: asr's page, then its panel, which starts a shell where
    # asr's last run started.
    r = Rec(page, frames, "asr_open")
    r.hold(1.2)
    page.locator("#terminal-button").click()
    page.locator(".xterm:visible").first.wait_for(timeout=15000)
    r.hold(1.6)
    focus_shell(page)
    r.type("pwd\n")
    r.hold(0.6)
    r.type("ls\n")
    r.hold(1.6)

    # The sidebar marks the projects with shells; tts's page shows
    # tts's shells.
    r = Rec(page, frames, "tts_page")
    page.locator("a.sidebar-project-link").filter(has_text="tts").first.click()
    page.wait_for_load_state()
    page.locator(".xterm:visible").first.wait_for(timeout=15000)
    r.hold(3.5)

    # All shells.
    r = Rec(page, frames, "all_shells")
    bar_button(page, "All shells").click()
    page.locator(".terminal-overview-card").first.wait_for(timeout=15000)
    page.wait_for_timeout(1500)
    r.hold(4.5)
    r = Rec(page, frames, "processes")
    tts = page.locator('.terminal-overview-group[aria-label="tts"]')
    tts.locator(".terminal-overview-processes:visible").first.click()
    page.wait_for_timeout(300)
    r.hold(2.5)

    # The finished evaluation: its card takes the page to vision.
    r = Rec(page, frames, "to_vision")
    r.hold(0.8)
    page.locator(".terminal-overview-finished").first.click()
    page.wait_for_load_state()
    page.locator(".xterm:visible").first.wait_for(timeout=15000)
    page.wait_for_timeout(800)
    r.hold(3.0)

    # TensorBoard where tts's second shell is, from its card.
    r = Rec(page, frames, "launch")
    bar_button(page, "All shells").click()
    page.locator(".terminal-overview-card").first.wait_for(timeout=15000)
    page.wait_for_timeout(1200)
    r.hold(0.8)
    tts = page.locator('.terminal-overview-group[aria-label="tts"]')
    tts.locator('[aria-label="Launch in shell 2"]').click()
    page.locator(".terminal-launch-commands button").first.wait_for(timeout=15000)
    page.wait_for_timeout(500)
    r.hold(1.5)
    page.locator(".terminal-launch-commands button").filter(
        has_text="TensorBoard"
    ).first.click()
    page.locator("button.terminal-launch-confirm").wait_for(timeout=15000)
    page.wait_for_timeout(400)
    r.hold(1.8)
    page.locator("button.terminal-launch-confirm").click()
    page.locator("a.terminal-launch-open").wait_for(timeout=90000)
    page.wait_for_timeout(400)
    r.hold(1.2)
    with ctx.expect_page() as opened:
        page.locator("a.terminal-launch-open").click()
    board = opened.value
    board.wait_for_load_state()
    # TensorBoard draws its charts after loading the event files.
    board.wait_for_timeout(9000)
    close = board.locator('button[aria-label="Close side pane"]')
    if close.count():
        close.first.click()
        board.wait_for_timeout(800)
    r = Rec(board, frames, "board")
    r.hold(1.0)
    board.close()
    page.screenshot(path=f"{OUT}/last.png")


main()
