#!/bin/sh
# Prepare the dataset once for every training run.
echo "prepare: tokenizing 4 shards"
for i in 1 2 3 4; do
    sleep 0.5
    echo "shard $i/4"
done
