#!/usr/bin/env bash
# Make the recovering episode (landscape) into out/04_recovering.mp4.
#
#   scripts/promo-video/04_recovering.sh [RENDER_OPTIONS...]
#
# Accompanies docs/RECOVERING.md and docs/LOCAL_DIAGNOSIS.md: a sweep's
# failures grouped by cause, one job's Diagnosis, then the fix,
# `retry --dry-run`, and `retry`, with the successes carried forward.
# Prepares the tools on first use, then builds rotari, captures (a headless
# browser is enough), and renders. PROMO_TOOLS and PROMO_WORK as for
# 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/defaults.sh"

"$here/setup.sh" "$PROMO_TOOLS"
"$here/make-episode.sh" "$PROMO_TOOLS" "$PROMO_WORK/04_recovering" \
    "$here/out/04_recovering.mp4" "$here/capture_recovering.py" "$here/render_recovering.py" "$@"
echo "wrote $here/out/04_recovering.mp4 and .srt"
