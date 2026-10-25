#!/usr/bin/env bash
# Make the agents episode (landscape) into out/09_agents.mp4.
#
#   scripts/promo-video/09_agents.sh [RENDER_OPTIONS...]
#
# Accompanies docs/MCP.md and `rotari guide`: what a coding agent reads, the
# commands the guide has it use (run with a note, wait for the first failure,
# read the summary, retry, note the results), the run report with that note,
# and the VS Code entry for `rotari mcp`. Prepares the tools on first use,
# then builds rotari, captures (a headless browser is enough), and renders.
# PROMO_TOOLS and PROMO_WORK as for 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/defaults.sh"

"$here/setup.sh" "$PROMO_TOOLS"
"$here/make-episode.sh" "$PROMO_TOOLS" "$PROMO_WORK/09_agents" \
    "$here/out/09_agents.mp4" "$here/capture_agents.py" "$here/render_agents.py" "$@"
echo "wrote $here/out/09_agents.mp4 and .srt"
