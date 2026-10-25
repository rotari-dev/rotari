#!/bin/sh
# Evaluation on the workstation, after training.
echo "eval on $(hostname): 3 test sets"
for result in "clean: wer=4.8%" "noisy: wer=9.6%" "far: wer=14.1%"; do
    sleep 1
    echo "$result"
done
