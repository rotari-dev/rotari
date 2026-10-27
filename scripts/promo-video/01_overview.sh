#!/usr/bin/env bash
# Make the overview (landscape) into out/01_overview.mp4.
#
#   scripts/promo-video/01_overview.sh [RENDER_OPTIONS...]
#
# One message for cluster users: hand rotari your usual commands, and every
# run is recorded and seen in one place. Thirty slurm-N.out files nobody can
# tell apart, the same script handed to rotari for Slurm, every project and
# the sweep's jobs in the Web UI, a failed job's log, and a retry of only the
# failed jobs. Prepares the tools on first use (with the Slurm image, which
# needs singularity or apptainer), then builds rotari, captures (a headless
# browser is enough), and renders. PROMO_TOOLS and PROMO_WORK as for
# 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/defaults.sh"

"$here/setup.sh" --with-slurm "$PROMO_TOOLS"
export PROMO_SLURM_SB="$PROMO_TOOLS/slurm-sb"
"$here/make-episode.sh" "$PROMO_TOOLS" "$PROMO_WORK/01_overview" \
    "$here/out/01_overview.mp4" "$here/capture_overview.py" "$here/render_overview.py" "$@"
echo "wrote $here/out/01_overview.mp4 and .srt"
