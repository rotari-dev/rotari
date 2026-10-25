#!/bin/sh
# One point of a six-run sweep, configured by config.sh. Tasks 2 and 5 run
# out of GPU memory while BATCH is above 64; task 4 diverges with BATCH 64
# and LR 1e-3.
. ./config.sh
t=${ROTARI_ARRAY_TASK_ID}
echo "task $t: batch=$BATCH lr=$LR"
sleep "1.$t"
case " 2 5 " in *" $t "*)
    if [ "$BATCH" -gt 64 ]; then
        echo "torch.OutOfMemoryError: CUDA out of memory. Tried to allocate 3.00 GiB" >&2
        exit 1
    fi
    ;;
esac
if [ "$t" = 4 ] && [ "$BATCH" -le 64 ] && [ "$LR" = 1e-3 ]; then
    echo "ValueError: loss is NaN at step 812" >&2
    exit 1
fi
echo "task $t: eval_loss=0.4$t"
