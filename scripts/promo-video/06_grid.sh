#!/usr/bin/env bash
# Make the grid episode (landscape) into out/06_grid.mp4.
#
#   scripts/promo-video/06_grid.sh [RENDER_OPTIONS...]
#
# Accompanies docs/RUNNING.md (Array and matrix jobs, Dependencies and
# stages): a grid of 12 training runs after a preparation step, then a
# report, first by hand with nested loops and then as a job, a matrix with an
# array, and a job depending on the matrix finishing. Prepares the tools on
# first use, then builds rotari, captures (a headless browser is enough), and
# renders. PROMO_TOOLS and PROMO_WORK as for 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/defaults.sh"

"$here/setup.sh" "$PROMO_TOOLS"
"$here/make-episode.sh" "$PROMO_TOOLS" "$PROMO_WORK/06_grid" \
    "$here/out/06_grid.mp4" "$here/capture_grid.py" "$here/render_grid.py" "$@"
echo "wrote $here/out/06_grid.mp4 and .srt"
