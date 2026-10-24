#!/usr/bin/env bash
# Make the executors episode (landscape) into out/05_executors.mp4.
#
#   scripts/promo-video/05_executors.sh [RENDER_OPTIONS...]
#
# Accompanies docs/EXECUTORS.md: a pipeline's steps run by hand on three
# machines, then as one rotari run with -e local, -e slurm, and -e ssh.
# Prepares the tools on first use (with the Slurm image, which needs
# singularity or apptainer), then builds rotari, captures (a headless browser
# is enough, plus openssh-server for the SSH host), and renders. PROMO_TOOLS
# and PROMO_WORK as for 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/defaults.sh"

"$here/setup.sh" --with-slurm "$PROMO_TOOLS"
export PROMO_SLURM_SB="$PROMO_TOOLS/slurm-sb"
"$here/make-episode.sh" "$PROMO_TOOLS" "$PROMO_WORK/05_executors" \
    "$here/out/05_executors.mp4" "$here/capture_executors.py" "$here/render_executors.py" "$@"
echo "wrote $here/out/05_executors.mp4 and .srt"
