#!/bin/sh
# The commands Claude Code ran on 2026-10-25 when asked to tune this sweep
# ("make all six seeds pass and find the batch size and learning rate with the
# lowest eval loss; use rotari and read `rotari guide` first"), in the order
# it ran them, with its notes as written. Only what changes state is kept: its
# read-only commands (guide, check, show --logs, lineage) are left out, and
# the run IDs it read from their output come from the state directory here.
# Run in the sweep directory with ROTARI_ACTOR=claude-code, as it was.
last_run() { ls "$HOME/.local/state/rotari/projects/sweep/runs" | tail -n 1; }

git init -q && git add . && git commit -qm "sweep: batch 128, lr 1e-3"
rotari add -p sweep --job-name train --array 1-6 -- ./train.sh
rotari run -p sweep --async --note "Baseline: batch 128, lr 1e-3, as committed. Expect some seeds to fail; find out why before tuning."
rotari wait sweep --quiet
rotari note "$(last_run)" "3 of 6 failed: seeds 2 and 5 ran out of GPU memory at batch 128; seed 4 diverged (NaN at step 812) at lr 1e-3. Passing seeds: eval_loss 0.483-0.498. Next: batch 64 alone, to separate the two causes."

sed -i "s/BATCH=128/BATCH=64/" config.sh && git commit -qam "batch 64"
rotari add -p sweep --job-name train --array 1-6 -- ./train.sh
rotari run -p sweep --async --note "Batch 64 to fit in GPU memory; lr unchanged, so seed 4 may still diverge."
rotari wait sweep --quiet
rotari note "$(last_run)" "Batch 64 fixed the OOM on seeds 2 and 5 without changing eval_loss (0.483-0.498). Seed 4 still diverges, so the NaN comes from lr 1e-3. Next: lr 3e-4."

sed -i "s/LR=1e-3/LR=3e-4/" config.sh && git commit -qam "lr 3e-4"
rotari add -p sweep --job-name train --array 1-6 -- ./train.sh
rotari run -p sweep --async --note "lr 3e-4 to stop seed 4 diverging; batch 64 kept."
rotari wait sweep --quiet
rotari note "$(last_run)" "All 6 seeds pass at batch 64, lr 3e-4; eval_loss 0.413-0.428, down from 0.483-0.498. Next: lr 1e-4, to see whether lower is better still."

sed -i "s/LR=3e-4/LR=1e-4/" config.sh && git commit -qam "lr 1e-4"
rotari add -p sweep --job-name train --array 1-6 -- ./train.sh
rotari run -p sweep --async --note "lr 1e-4: does a lower learning rate help further?"
rotari wait sweep --quiet

cat > results.md <<'TABLE'
Best: **batch 64, lr 3e-4** (eval_loss 0.413-0.428, all seeds pass). Config set back to it.

| job | b128 lr1e-3 | b64 lr1e-3 | b64 lr3e-4 | b64 lr1e-4 |
| --- | --- | --- | --- | --- |
| train[1] | 0.483 | 0.483 | **0.413** | 0.443 |
| train[2] | OOM | 0.486 | **0.416** | 0.446 |
| train[3] | 0.489 | 0.489 | **0.419** | 0.449 |
| train[4] | NaN | NaN | **0.422** | 0.452 |
| train[5] | OOM | 0.495 | **0.425** | 0.455 |
| train[6] | 0.498 | 0.498 | **0.428** | 0.458 |
TABLE
sed -i "s/LR=1e-4/LR=3e-4/" config.sh && git commit -qam "Use batch 64, lr 3e-4: best eval_loss in the sweep"
rotari note "$(last_run)" "$(cat results.md)"
