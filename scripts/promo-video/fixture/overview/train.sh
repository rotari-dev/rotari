#!/bin/sh
# One point of the ResNet sweep, submitted by rotari to Slurm. LR and BS come
# from the matrix; train.conf sets the precision. At fp32, batch 128 does not
# fit in GPU memory.
. ./train.conf
echo "train resnet50 lr=$LR bs=$BS precision=$precision on $(hostname)"
for epoch in 1 2 3; do
    sleep 1
    if [ "$BS" = 128 ] && [ "$precision" = fp32 ]; then
        echo "torch.OutOfMemoryError: CUDA out of memory. Tried to allocate 6.00 GiB" >&2
        exit 1
    fi
    echo "epoch $epoch/3 loss=0.$((70 - epoch * 9))"
done
case "$LR" in 1e-3) acc=74 ;; 3e-4) acc=77 ;; *) acc=75 ;; esac
[ "$BS" = 128 ] && acc=$((acc + 1))
echo "val_top1=$acc.$((epoch * 3))%"
