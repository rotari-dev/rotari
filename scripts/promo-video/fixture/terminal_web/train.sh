#!/bin/sh
# A training job for the Web terminal episode; OUTCOME picks how it ends:
# ok, missing (a missing data file), or long (still training when the
# capture ends).
echo "train in $(basename "$PWD"): seed=${ROTARI_ARRAY_TASK_ID:-1}"
case "${OUTCOME:-ok}" in
ok)
    sleep 2
    echo "final loss=0.41"
    ;;
missing)
    sleep 1
    echo "FileNotFoundError: [Errno 2] No such file or directory: 'data/val.jsonl'" >&2
    exit 1
    ;;
long) for epoch in $(seq 1 300); do
    sleep 2
    echo "epoch $epoch/300 loss=$(awk "BEGIN { printf \"%.3f\", 1.9 / (1 + $epoch / 20) + 0.3 }")"
done ;;
esac
