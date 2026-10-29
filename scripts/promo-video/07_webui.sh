#!/usr/bin/env bash
# Make the Web UI episode (landscape) into out/07_webui.mp4.
#
#   scripts/promo-video/07_webui.sh [RENDER_OPTIONS...]
#
# Accompanies docs/WEB.md: four projects in their own directories and two
# state directories, seen through All projects, Activity, Recent jobs, and
# History search. Prepares the tools on first use, then builds rotari,
# captures (a headless browser is enough), and renders. PROMO_TOOLS and
# PROMO_WORK as for 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/defaults.sh"

"$here/setup.sh" "$PROMO_TOOLS"
"$here/make-episode.sh" "$PROMO_TOOLS" "$PROMO_WORK/07_webui" \
    "$here/out/07_webui.mp4" "$here/capture_webui.py" "$here/render_webui.py" "$@"
echo "wrote $here/out/07_webui.mp4 and .srt"
