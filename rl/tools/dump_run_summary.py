"""
Dump TensorBoard scalars from one or more runs to compact CSV summaries.

Usage:
    python rl/tools/dump_run_summary.py rl/runs/QRDQN_5
    python rl/tools/dump_run_summary.py rl/runs/QRDQN_5 -o rl/runs_summary
    python rl/tools/dump_run_summary.py --all                 # process every run dir under rl/runs

Output: one CSV per run with columns
    step, ep_rew_mean, catch_rate, ambush_ratio, in_cone_ratio,
    ep_len_mean, hit_per_ep, curriculum_stage, loss, lr, fps

Loss/lr/fps are downsampled (one row per ~50K steps by default) to keep file
size small. The notebook reads these CSVs to build the run-history view; full
TB events stay in rl/runs/ (gitignored) for full-fidelity local inspection.
"""

from __future__ import annotations

import argparse
import csv
from pathlib import Path

import numpy as np
from tensorboard.backend.event_processing.event_accumulator import EventAccumulator


SCALAR_TAGS = [
    ("metrics/ep_rew_mean",      "ep_rew_mean"),
    ("metrics/catch_rate",       "catch_rate"),
    ("metrics/ambush_ratio",     "ambush_ratio"),
    ("metrics/in_cone_ratio",    "in_cone_ratio"),
    ("metrics/ep_len_mean",      "ep_len_mean"),
    ("metrics/hit_per_ep",       "hit_per_ep"),
    ("metrics/curriculum_stage", "curriculum_stage"),
    ("train/loss",               "loss"),
    ("train/learning_rate",      "lr"),
    ("time/fps",                 "fps"),
]


def _downsample(steps: list[int], values: list[float], bin_size: int) -> dict[int, float]:
    """Average values within each bin_size-step bucket. Returns {bin_step: mean_value}."""
    if not steps:
        return {}
    arr_s = np.asarray(steps)
    arr_v = np.asarray(values, dtype=np.float64)
    bins = (arr_s // bin_size) * bin_size
    out: dict[int, float] = {}
    for b in np.unique(bins):
        out[int(b)] = float(arr_v[bins == b].mean())
    return out


def dump_run(run_dir: Path, out_dir: Path, bin_size: int = 50_000) -> Path:
    """Read one TB run dir and write a single CSV summary. Returns the CSV path."""
    if not any(run_dir.glob("events.out.tfevents.*")):
        raise FileNotFoundError(f"no TB events in {run_dir}")

    ea = EventAccumulator(str(run_dir), size_guidance={"scalars": 0})
    ea.Reload()
    available = set(ea.Tags().get("scalars", []))

    series: dict[str, dict[int, float]] = {}
    all_steps: set[int] = set()
    for tb_tag, col in SCALAR_TAGS:
        if tb_tag not in available:
            series[col] = {}
            continue
        ev = ea.Scalars(tb_tag)
        steps = [e.step for e in ev]
        values = [e.value for e in ev]
        # high-frequency tags (loss, lr, fps) get downsampled; per-eval tags
        # already fire at low cadence and pass through unchanged.
        if len(steps) > 200:
            binned = _downsample(steps, values, bin_size)
        else:
            binned = {int(s): float(v) for s, v in zip(steps, values)}
        series[col] = binned
        all_steps.update(binned.keys())

    out_dir.mkdir(parents=True, exist_ok=True)
    out_path = out_dir / f"{run_dir.name}.csv"
    cols = ["step"] + [c for _, c in SCALAR_TAGS]
    with out_path.open("w", newline="") as fh:
        w = csv.writer(fh)
        w.writerow(cols)
        for step in sorted(all_steps):
            row = [step]
            for _, col in SCALAR_TAGS:
                v = series[col].get(step)
                row.append("" if v is None else f"{v:.6g}")
            w.writerow(row)
    return out_path


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("run_dir", nargs="?", help="path to a single TB run directory")
    ap.add_argument("--all", action="store_true", help="process every subdir under rl/runs")
    ap.add_argument("-o", "--out", default="rl/runs_summary", help="output dir for CSVs")
    ap.add_argument("--bin", type=int, default=50_000, help="downsample bucket size in env steps")
    args = ap.parse_args()

    out_dir = Path(args.out)

    if args.all:
        runs_root = Path("rl/runs")
        targets = sorted(p for p in runs_root.iterdir() if p.is_dir())
    elif args.run_dir:
        targets = [Path(args.run_dir)]
    else:
        ap.error("pass a run dir or --all")

    for run_dir in targets:
        try:
            csv_path = dump_run(run_dir, out_dir, bin_size=args.bin)
            print(f"wrote {csv_path}")
        except FileNotFoundError as e:
            print(f"skip {run_dir.name}: {e}")


if __name__ == "__main__":
    main()
