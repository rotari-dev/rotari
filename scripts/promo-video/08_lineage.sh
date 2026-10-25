#!/usr/bin/env bash
# Make the lineage episode (landscape) into out/08_lineage.mp4.
#
#   scripts/promo-video/08_lineage.sh [RENDER_OPTIONS...]
#
# Accompanies docs/INSPECT.md (Run lineage): a sweep's runs from three
# commits, each with a note, listed and compared with `rotari lineage`, and a
# conclusion recorded with `rotari note`. Prepares the tools on first use,
# then builds rotari, captures (a headless browser is enough), and renders.
# PROMO_TOOLS and PROMO_WORK as for 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/defaults.sh"

"$here/setup.sh" "$PROMO_TOOLS"
"$here/make-episode.sh" "$PROMO_TOOLS" "$PROMO_WORK/08_lineage" \
    "$here/out/08_lineage.mp4" "$here/capture_lineage.py" "$here/render_lineage.py" "$@"
echo "wrote $here/out/08_lineage.mp4 and .srt"
