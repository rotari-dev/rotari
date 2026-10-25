#!/usr/bin/env bash
# Make the overview (landscape) into out/01_overview.mp4.
#
#   scripts/promo-video/01_overview.sh [RENDER_OPTIONS...]
#
# One job needs no rotari; research is many projects in many tabs, and with
# rotari they are on one page, with the broken job's log a click away; then
# past runs on the calendar and in search (07's capture), outputs previewed
# in the browser (03's), and an end card naming the other features. It
# reuses 03's and 07's captures in PROMO_WORK, so make those first; this
# script captures only 01's own material, then renders. RENDER_OPTIONS go to
# render_tour.py. PROMO_TOOLS and PROMO_WORK as for 01_short.sh.
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
