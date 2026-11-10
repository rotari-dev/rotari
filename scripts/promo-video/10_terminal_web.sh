#!/usr/bin/env bash
# Make the Web terminal episode (landscape) into out/10_terminal_web.mp4.
#
#   scripts/promo-video/10_terminal_web.sh [RENDER_OPTIONS...]
#
# Accompanies the Terminal section of docs/WEB.md: shells that belong to
# projects, All shells, and launching TensorBoard where a shell is. Prepares
# the tools on first use, then builds rotari, captures (a headless browser is
# enough), and renders. PROMO_TOOLS and PROMO_WORK as for 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/defaults.sh"

"$here/setup.sh" "$PROMO_TOOLS"
"$here/make-episode.sh" "$PROMO_TOOLS" "$PROMO_WORK/10_terminal_web" \
    "$here/out/10_terminal_web.mp4" "$here/capture_terminal_web.py" "$here/render_terminal_web.py" "$@"
echo "wrote $here/out/10_terminal_web.mp4 and .srt"
