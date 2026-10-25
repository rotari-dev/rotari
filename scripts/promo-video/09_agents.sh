#!/usr/bin/env bash
# Make the agents episode (landscape) into out/09_agents.mp4.
#
#   scripts/promo-video/09_agents.sh [RENDER_OPTIONS...]
#
# Accompanies `rotari guide`, docs/MCP.md, and docs/INSPECT.md (Notes on
# runs): the runs Claude Code made while tuning a sweep with rotari
# (fixture/tune/session.sh, replayed), as the person who asked reads them:
# the history with its notes, its reasoning between two runs, the last run's
# page and report, and the line in AGENTS.md that set it up. Prepares the
# tools on first use, then builds rotari, captures (a headless browser is
# enough), and renders. PROMO_TOOLS and PROMO_WORK as for 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/defaults.sh"

"$here/setup.sh" "$PROMO_TOOLS"
"$here/make-episode.sh" "$PROMO_TOOLS" "$PROMO_WORK/09_agents" \
    "$here/out/09_agents.mp4" "$here/capture_agents.py" "$here/render_agents.py" "$@"
echo "wrote $here/out/09_agents.mp4 and .srt"
