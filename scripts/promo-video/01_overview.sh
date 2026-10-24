#!/usr/bin/env bash
# Make the overview ("You don't need rotari for one job"), landscape, into
# out/01_overview.mp4.
#
#   scripts/promo-video/01_overview.sh [RENDER_OPTIONS...]
#
# The same story as 01_short.sh, captured and rendered for 1920x1080: the main
# version, for YouTube and the documentation. RENDER_OPTIONS and the
# directories are as for 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/defaults.sh"

slurm=()
for option in "$@"; do
    [[ "$option" == --with-slurm ]] && slurm=(--with-slurm)
done
"$here/setup.sh" "${slurm[@]}" "$PROMO_TOOLS"
"$here/make-video.sh" --landscape "$PROMO_TOOLS" "$PROMO_WORK/01_overview" \
    "$here/out/01_overview.mp4" "$@"
echo "wrote $here/out/01_overview.mp4 and .srt"
