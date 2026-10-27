"""Write a sample evaluation table for the static Table viewer demo.

utterances.csv lists synthetic utterances with their scores and a WAV file
each, under wav/: tones, chirps, and noise, some in stereo with a different
sound per channel. Uses only the standard library, so the demo needs no
numpy or audio packages, and is seeded, so every build writes the same
files.
"""

from __future__ import annotations

import argparse
import csv
import math
import random
import struct
import wave
from pathlib import Path

RATE = 16000


def signal(
    kind: str, frequency: float, seconds: float, rng: random.Random
) -> list[float]:
    """Return samples in [-1, 1] of a tone, a rising chirp, or noise."""
    samples = []
    count = int(RATE * seconds)
    for index in range(count):
        time = index / RATE
        if kind == "tone":
            value = math.sin(2 * math.pi * frequency * time)
        elif kind == "chirp":
            # Rises from frequency to three times it over the clip.
            rate = 2 * frequency / seconds
            value = math.sin(2 * math.pi * (frequency * time + rate * time * time / 2))
        else:
            value = rng.uniform(-1, 1)
        # Fade in and out so clips do not click.
        envelope = min(1.0, time / 0.05, (seconds - time) / 0.05)
        samples.append(0.5 * envelope * value)
    return samples


def write_wav(
    path: Path, channels: list[list[float]], snr_db: float, rng: random.Random
) -> None:
    """Write 16-bit channels with noise at snr_db."""
    noise = 0.5 * 10 ** (-snr_db / 20)
    with wave.open(str(path), "wb") as output:
        output.setnchannels(len(channels))
        output.setsampwidth(2)
        output.setframerate(RATE)
        frames = bytearray()
        for frame in zip(*channels, strict=True):
            for value in frame:
                value = max(-1.0, min(1.0, value + rng.gauss(0, noise)))
                frames += struct.pack("<h", int(32767 * value))
        output.writeframes(bytes(frames))


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    args = parser.parse_args()
    rng = random.Random(7)
    (args.directory / "wav").mkdir(parents=True, exist_ok=True)
    kinds = ["tone", "chirp", "noise"]
    splits = ["dev", "dev", "test"]
    speakers = ["spk01", "spk02", "spk03", "spk04"]
    rows = []
    for number in range(1, 37):
        kind = kinds[number % len(kinds)]
        frequency = rng.choice([220.0, 330.0, 440.0, 660.0, 880.0])
        seconds = round(rng.uniform(0.8, 2.5), 2)
        snr_db = round(rng.uniform(0, 30), 1)
        channels = [signal(kind, frequency, seconds, rng)]
        if number % 6 == 0:
            # Stereo: the right channel an octave up, so the Channel
            # choice draws and plays something different.
            channels.append(
                signal(kinds[(number + 1) % len(kinds)], frequency * 2, seconds, rng)
            )
        name = f"wav/utt{number:03d}.wav"
        write_wav(args.directory / name, channels, snr_db, rng)
        # Lower SNR, more errors: a score that sorts and describes sensibly.
        wer = round(max(0.0, min(1.0, 0.45 - snr_db / 80 + rng.gauss(0, 0.05))), 3)
        rows.append(
            {
                "id": f"utt{number:03d}",
                "audio": name,
                "split": splits[number % len(splits)],
                "speaker": speakers[number % len(speakers)],
                "signal": kind,
                "channels": len(channels),
                "snr_db": snr_db,
                "wer": wer,
            }
        )
    with open(args.directory / "utterances.csv", "w", newline="") as output:
        writer = csv.DictWriter(output, fieldnames=list(rows[0]))
        writer.writeheader()
        writer.writerows(rows)


if __name__ == "__main__":
    main()
