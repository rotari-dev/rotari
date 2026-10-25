#!/bin/sh
# One training run of the grid: LR and BS from the matrix, the seed from the
# array task (or SEED when started by hand). Writes its final loss to results/.
seed=${ROTARI_ARRAY_TASK_ID:-$SEED}
mkdir -p results
echo "train lr=$LR bs=$BS seed=$seed"
case "$LR" in 1e-3) base=48 ;; *) base=41 ;; esac
case "$BS" in 16) base=$((base + 3)) ;; esac
sleep "$((seed + 1)).$((BS / 4))"
loss="0.$((base + seed))"
echo "final loss=$loss"
echo "$loss lr=$LR bs=$BS seed=$seed" > "results/lr$LR-bs$BS-seed$seed.txt"
