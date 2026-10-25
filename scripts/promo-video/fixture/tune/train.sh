#!/bin/sh
# One seed of the sweep the agent tunes, configured by config.sh (BATCH, LR).
# Seeds 2 and 5 run out of GPU memory while BATCH is above 64; seed 4
# diverges at LR 1e-3; the evaluation loss depends on LR and BATCH.
. ./config.sh
seed=${ROTARI_ARRAY_TASK_ID}
echo "seed $seed: batch=$BATCH lr=$LR"
sleep "1.$seed"
case " 2 5 " in *" $seed "*)
    if [ "$BATCH" -gt 64 ]; then
        echo "torch.OutOfMemoryError: CUDA out of memory. Tried to allocate 3.00 GiB" >&2
        exit 1
    fi
    ;;
esac
if [ "$seed" = 4 ] && [ "$LR" = 1e-3 ]; then
    echo "ValueError: loss is NaN at step 812" >&2
    exit 1
fi
case "$LR" in 1e-3) loss=480 ;; 3e-4) loss=410 ;; 1e-4) loss=440 ;; *) loss=500 ;; esac
[ "$BATCH" -le 32 ] && loss=$((loss + 15))
loss=$((loss + seed * 3))
echo "seed $seed: eval_loss=0.$loss"
