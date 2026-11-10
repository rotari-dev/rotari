"""Training with worker processes, for the tts shell's foreground program.

    python train.py --workers N

Starts N worker processes and prints the epoch until it is stopped, so the
All shells card shows the command and how many processes it started.
"""

import argparse
import multiprocessing
import time


def work():
    while True:
        time.sleep(1)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--workers", type=int, default=3)
    args = parser.parse_args()
    for _ in range(args.workers):
        multiprocessing.Process(target=work, daemon=True).start()
    for epoch in range(1, 10000):
        print(f"epoch {epoch}  loss={1.9 / (1 + epoch / 4) + 0.3:.3f}", flush=True)
        time.sleep(2)
