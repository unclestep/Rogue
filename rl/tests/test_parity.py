"""
Byte-exact Go↔Python parity on BuildObservation.

Each scenario in rl/fixtures/parity/*.json carries the full BuildObservation
input (topology path, pursuer/player positions, memory, other monsters). The
Go utility cmd/dump-observation rebuilds the same state server-side and writes
a reference 859-float vector; the Python env runs the mirror implementation
here and np.allclose-compares.

Tests are skipped when the `go` toolchain is unavailable (fresh Python-only
Colab install, for example). CI always has Go.
"""

from __future__ import annotations

import json
import math
import os
import shutil
import subprocess
import tempfile
from pathlib import Path

import numpy as np
import pytest

from rl.pursuer_env import (
    OBSERVATION_SIZE,
    PursuerMemory,
    Topology,
    build_observation,
    flashlight_mask,
)


REPO_ROOT = Path(__file__).resolve().parents[2]
PARITY_DIR = REPO_ROOT / "rl" / "fixtures" / "parity"
ATOL = 1e-6


def _go_available() -> bool:
    return shutil.which("go") is not None


def _scenarios() -> list[Path]:
    return sorted(p for p in PARITY_DIR.glob("*.json") if p.is_file())


def _resolve_topology(scenario_path: Path, topology: str) -> Path:
    if os.path.isabs(topology):
        return Path(topology)
    return (scenario_path.parent / topology).resolve()


def _dump_reference(scenario_path: Path) -> np.ndarray:
    out_path = Path(tempfile.mkstemp(suffix=".json", prefix="obs_")[1])
    try:
        subprocess.run(
            [
                "go",
                "run",
                "./cmd/dump-observation",
                "-scenario",
                str(scenario_path),
                "-out",
                str(out_path),
            ],
            cwd=REPO_ROOT,
            check=True,
            capture_output=True,
        )
        with out_path.open("r", encoding="utf-8") as fh:
            data = json.load(fh)
    finally:
        out_path.unlink(missing_ok=True)

    arr = np.asarray(data["observation"], dtype=np.float32)
    assert arr.shape == (OBSERVATION_SIZE,), (
        f"go dump produced {arr.shape}, want ({OBSERVATION_SIZE},)"
    )
    return arr


def _python_observation(scenario_path: Path) -> np.ndarray:
    with scenario_path.open("r", encoding="utf-8") as fh:
        scenario = json.load(fh)

    topo_path = _resolve_topology(scenario_path, scenario["topology"])
    topology = Topology.from_json(str(topo_path))

    pursuer = scenario["pursuer"]
    pursuer_pos = (pursuer["pos"]["x"], pursuer["pos"]["y"])

    player = scenario.get("player")
    if player is not None:
        player_pos = (player["pos"]["x"], player["pos"]["y"])
        player_angle = float(player["aim_angle_rad"])
        cone = flashlight_mask(topology, player_pos, player_angle)
    else:
        player_pos = None
        player_angle = 0.0
        cone = None

    memory = PursuerMemory()
    mem_spec = scenario["memory"]
    ls = mem_spec["last_seen"]
    if ls["x"] >= 0 and ls["y"] >= 0:
        memory.last_seen = (ls["x"], ls["y"])
    memory.turns_since_los = int(mem_spec["turns_since_los"])
    for p in mem_spec.get("trail", []):
        memory.push_trail((p["x"], p["y"]))

    others = scenario.get("other_monsters") or []
    other_pos = (others[0]["x"], others[0]["y"]) if others else None

    from rl.pursuer_env import chase_scent_map

    scent_map = None
    if player_pos:
        scent_map = chase_scent_map(topology, player_pos)

    return build_observation(
        topology=topology,
        pursuer_pos=pursuer_pos,
        pursuer_hp_frac=float(pursuer["hp_frac"]),
        pursuer_stamina_frac=float(pursuer["stamina_frac"]),
        memory=memory,
        player_pos=player_pos,
        player_angle_rad=player_angle,
        cone_mask=cone,
        other_pursuer_pos=other_pos,
        scent_map=scent_map,
    )


@pytest.mark.skipif(not _go_available(), reason="go toolchain not available")
@pytest.mark.parametrize("scenario", _scenarios(), ids=lambda p: p.name)
def test_parity_with_go(scenario: Path):
    want = _dump_reference(scenario)
    got = _python_observation(scenario)

    if not np.allclose(got, want, atol=ATOL):
        diffs = np.where(~np.isclose(got, want, atol=ATOL))[0]
        samples = [(int(i), float(got[i]), float(want[i])) for i in diffs[:10]]
        max_abs = float(np.max(np.abs(got - want)))
        pytest.fail(
            f"observation mismatch for {scenario.name}: "
            f"max|diff|={max_abs:.3g}, first divergences "
            f"(idx, py, go)={samples}"
        )
