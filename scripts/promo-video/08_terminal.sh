#!/usr/bin/env bash
# Make the terminal episode (landscape) into out/08_terminal.mp4.
#
#   scripts/promo-video/08_terminal.sh [RENDER_OPTIONS...]
#
# Accompanies docs/INSPECT.md: a sweep still running, inspected from a
# terminal with no browser: rotari jobs, rotari show, every job's last log
# line, the failed jobs' logs, and following the running job. Prepares the
# tools on first use, then builds rotari, captures (no browser is used), and
# renders. PROMO_TOOLS and PROMO_WORK as for 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/defaults.sh"

"$here/setup.sh" "$PROMO_TOOLS"
"$here/make-episode.sh" "$PROMO_TOOLS" "$PROMO_WORK/08_terminal" \
    "$here/out/08_terminal.mp4" "$here/capture_terminal.py" "$here/render_terminal.py" "$@"
echo "wrote $here/out/08_terminal.mp4 and .srt"
