#!/bin/sh
# A project's training job, configured by train.conf when the project has one.
# vision's dataset path is wrong until train.conf is fixed; once vision
# finishes, it plots its loss to outputs/loss.png (with PLOT_PYTHON).
name=$(basename "$PWD")
data=data
[ -f train.conf ] && . ./train.conf
echo "[$name] start on $(hostname)"
for epoch in 1 2 3 4 5 6; do
    sleep 2
    if [ "$name" = vision ] && [ "$epoch" = 2 ] && [ ! -d "$data" ]; then
        echo "Traceback (most recent call last):" >&2
        echo "  File \"train.py\", line 31, in <module>" >&2
        echo "FileNotFoundError: [Errno 2] No such file or directory: '$data'" >&2
        exit 1
    fi
    echo "[$name] epoch $epoch/6  loss=0.$((80 - epoch * 9))"
done
if [ "$name" = vision ] && [ -n "$PLOT_PYTHON" ]; then
    "$PLOT_PYTHON" plot.py
fi
