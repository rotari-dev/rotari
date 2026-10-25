#!/usr/bin/env bash
# Make the overview, a quick tour of rotari (landscape), into
# out/01_overview.mp4.
#
#   scripts/promo-video/01_overview.sh [RENDER_OPTIONS...]
#
# One job needs no rotari; research is many. Then a few seconds of each
# feature video: projects on one page (07), a grid (06), local and Slurm
# (05), notifications (02), failures and retry (04), outputs (03), and a
# coding agent's runs (09). The tour reuses those videos' captures in
# PROMO_WORK, so make them first (02 needs a display; 05 the Slurm image);
# this script captures only 01's own opening terminal, then renders.
# RENDER_OPTIONS go to render_tour.py. PROMO_TOOLS and PROMO_WORK as for
# 01_short.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)
. "$here/defaults.sh"

"$here/setup.sh" "$PROMO_TOOLS"
work="$PROMO_WORK/01_overview"
mkdir -p "$work"
(cd "$repo" && CGO_ENABLED=0 "$GO" build -o "$work/rotari" ./cmd/rotari)
rm -rf "$work/home" "$work/capture"
export PYTHON=${PYTHON:-$repo/.venv/bin/python} PLAYWRIGHT_BROWSERS_PATH="$PROMO_TOOLS/pw"
"$here/capture.sh" --landscape "$work" "$work/rotari" </dev/null
"$PYTHON" "$here/render_tour.py" "$PROMO_WORK" "$repo" "$here/out/01_overview.mp4" \
    --narration "$PROMO_TOOLS/kokoro" "$@"
echo "wrote $here/out/01_overview.mp4 and .srt"
