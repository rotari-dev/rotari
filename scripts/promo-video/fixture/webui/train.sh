#!/bin/sh
# A training job for the Web UI episode; OUTCOME picks how it ends:
# ok, oom (CUDA out of memory), missing (a missing data file), or long.
echo "train in $(basename "$PWD"): batch=${BATCH:-64}"
case "${OUTCOME:-ok}" in
ok)
    sleep 2
    echo "final loss=0.41"
    ;;
oom)
    sleep 1
    echo "torch.OutOfMemoryError: CUDA out of memory. Tried to allocate 2.00 GiB" >&2
    exit 1
    ;;
missing)
    sleep 1
    echo "FileNotFoundError: [Errno 2] No such file or directory: 'data/val.jsonl'" >&2
    exit 1
    ;;
long) for epoch in $(seq 1 60); do
    sleep 2
    echo "epoch $epoch/60"
done ;;
esac
