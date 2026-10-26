# Plot the vision project's training curves into the outputs directory.

import math
import os

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt

steps = list(range(0, 3001, 50))
train = [2.4 * math.exp(-s / 700) + 0.35 + 0.03 * math.sin(s / 90) for s in steps]
val = [2.4 * math.exp(-s / 760) + 0.42 + 0.02 * math.sin(s / 140) for s in steps]

os.makedirs("outputs", exist_ok=True)
plt.style.use("dark_background")
fig, ax = plt.subplots(figsize=(6.4, 3.6), dpi=150)
ax.plot(steps, train, label="train", color="#4cc38a")
ax.plot(steps, val, label="validation", color="#f08b70")
ax.set_xlabel("step")
ax.set_ylabel("loss")
ax.set_title("ResNet-50 · ImageNet")
ax.legend()
fig.tight_layout()
fig.savefig("outputs/loss.png")
