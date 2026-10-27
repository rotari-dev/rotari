#!/bin/sh
# One of last week's hand-submitted jobs: some setting of some model, which
# its output only half says. A few of them failed.
echo "start $(date -u +%H:%M:%S) on $(hostname)"
sleep 1
case $((SLURM_JOB_ID % 7)) in
0) echo "RuntimeError: CUDA error: an illegal memory access was encountered" >&2 ;;
3) echo "Error: data shard 17 not found" >&2 ;;
*) echo "epoch 10/10 val_loss=0.$((40 + SLURM_JOB_ID % 13))" ;;
esac
