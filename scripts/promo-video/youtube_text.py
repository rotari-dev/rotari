"""Write each video's YouTube title and description to out/NAME.txt.

    youtube_text.py [OUT_DIR]

Reads youtube.md beside this script, whose `## NAME` sections hold a
`Title:` line and the description, and writes OUT_DIR/NAME.txt (default
out/) with the title, a blank line, and the description, ready to paste.
Prose paragraphs, wrapped in youtube.md, become one line each, since YouTube
keeps line breaks; chapter, link, and hashtag lines stay as they are.
Stops if a section has no title, or a title is longer than YouTube's 100
characters.
"""

import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
# Lines kept on their own: chapters, "Label: URL" links, and hashtags.
KEEP = re.compile(r"^(\d+:\d\d |[^:]+: https?://|#)")


def unwrap(description):
    """Join each prose paragraph's lines; keep listed lines one per line."""
    paragraphs = []
    for paragraph in description.split("\n\n"):
        lines = paragraph.split("\n")
        if all(KEEP.match(line) for line in lines):
            paragraphs.append(paragraph)
        else:
            paragraphs.append(" ".join(line.strip() for line in lines))
    return "\n\n".join(paragraphs)


out = Path(sys.argv[1]) if len(sys.argv) > 1 else HERE / "out"
out.mkdir(exist_ok=True)

sections = re.split(r"^## ", (HERE / "youtube.md").read_text(), flags=re.M)[1:]
for section in sections:
    name, body = section.split("\n", 1)
    found = re.match(r"\s*Title: (.+)\n", body)
    if not found:
        sys.exit(f"youtube.md: {name} has no Title: line")
    title = found.group(1).strip()
    if len(title) > 100:
        sys.exit(f"youtube.md: {name}'s title is {len(title)} characters, over 100")
    description = unwrap(body[found.end() :].strip())
    (out / f"{name}.txt").write_text(f"{title}\n\n{description}\n")
    print(f"{name}.txt  {title}")
