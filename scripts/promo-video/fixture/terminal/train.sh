#!/bin/sh
# One seed of an eight-seed sweep for the terminal episode. Seed 3 runs out
# of GPU memory, seed 6 diverges, and seed 8 trains for longer than the rest,
# so it is still running while the others are inspected.
seed=${ROTARI_ARRAY_TASK_ID}
epochs=3
[ "$seed" = 8 ] && epochs=40
echo "seed $seed: batch=64 lr=3e-4"
for epoch in $(seq 1 "$epochs"); do
    sleep 1
    echo "seed $seed: epoch $epoch/$epochs loss=0.$((90 - epoch * 2 - seed))"
    if [ "$seed" = 3 ] && [ "$epoch" = 2 ]; then
        echo "torch.OutOfMemoryError: CUDA out of memory. Tried to allocate 2.00 GiB" >&2
        exit 1
    fi
    if [ "$seed" = 6 ] && [ "$epoch" = 2 ]; then
        echo "ValueError: loss is NaN at step 412" >&2
        exit 1
    fi
done
echo "seed $seed: eval_loss=0.$((40 + seed))"
