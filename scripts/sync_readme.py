#!/usr/bin/env python3
"""Synchronize the Getting started guide into the GitHub README.

GitHub strips iframes, so a YouTube player the guide embeds for the docs site
becomes the video's thumbnail, linked to the video, in the README.
"""

from __future__ import annotations

import argparse
import pathlib
import re

BEGIN = "<!-- BEGIN GETTING STARTED -->"
END = "<!-- END GETTING STARTED -->"
RELATIVE_MARKDOWN_LINK = re.compile(r"\]\((?!https?://)([^)#]+)")
README_LINK_REPLACEMENT = r"](docs/\1"
YOUTUBE_EMBED = re.compile(
    r'<iframe src="https://www\.youtube-nocookie\.com/embed/([\w-]+)" title="([^"]*)"[^>]*></iframe>'
)
YOUTUBE_THUMBNAIL = (
    r"[![\2](https://img.youtube.com/vi/\1/maxresdefault.jpg)]"
    r"(https://www.youtube.com/watch?v=\1)"
)


def readme_content(repo_dir: pathlib.Path) -> str:
    guide = (repo_dir / "docs" / "GETTING_STARTED.md").read_text(encoding="utf-8")
    guide = re.sub(r"^# Getting started\s*\n", "", guide, count=1)
    guide = RELATIVE_MARKDOWN_LINK.sub(README_LINK_REPLACEMENT, guide)
    guide = YOUTUBE_EMBED.sub(YOUTUBE_THUMBNAIL, guide)
    if "<iframe" in guide:
        raise SystemExit(
            "docs/GETTING_STARTED.md has an iframe the README cannot show; "
            "embed YouTube videos as https://www.youtube-nocookie.com/embed/ID with a title"
        )
    return f"{BEGIN}\n\n{guide.rstrip()}\n\n{END}"


def update_readme(readme: str, replacement: str) -> str:
    begin = readme.find(BEGIN)
    end = readme.find(END, begin)
    if begin < 0 or end < 0:
        raise SystemExit("README must contain exactly one Getting started marker pair")

    # Migrate the former standalone value-proposition block into the single
    # generated Getting started section, keeping the edit warning before it.
    pitch_begin = readme.find("<!-- BEGIN ROTARI VALUE PROPOSITION -->")
    pitch_end_marker = "<!-- END ROTARI VALUE PROPOSITION -->"
    if pitch_begin >= 0:
        pitch_end = readme.find(pitch_end_marker, pitch_begin)
        if pitch_end < 0 or pitch_begin > begin:
            raise SystemExit("README contains malformed value-proposition markers")
        warning_start = readme.find("<!-- Do not edit `README.md` directly.", pitch_end)
        warning_end = readme.find("-->", warning_start)
        if warning_start < 0 or warning_end < 0 or warning_start > begin:
            raise SystemExit("README is missing the generated-content warning")
        prefix = readme[:pitch_begin].rstrip()
        warning = readme[warning_start : warning_end + 3]
        suffix = readme[end + len(END) :]
        return f"{prefix}\n\n{warning}\n\n{replacement}{suffix}"

    end += len(END)
    return readme[:begin] + replacement + readme[end:]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    repo_dir = pathlib.Path(__file__).resolve().parents[1]
    readme_path = repo_dir / "README.md"
    readme = readme_path.read_text(encoding="utf-8")
    replacement = readme_content(repo_dir)
    updated = update_readme(readme, replacement)
    if args.check:
        if updated != readme:
            raise SystemExit("README is out of sync; run scripts/sync_readme.py")
    else:
        readme_path.write_text(updated, encoding="utf-8")


if __name__ == "__main__":
    main()
