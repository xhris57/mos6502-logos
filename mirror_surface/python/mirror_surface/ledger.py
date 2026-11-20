"""Append-only, SHA-256 hash-chained JSONL curvature ledger.

prev_tx_hash = sha256(canonical_json(previous full record)), the same rule used by
/workspace/ledger-current.jsonl (seven-symbol market). Integrity != truth: the chain proves
records were not altered, not that the sorting is correct."""
from __future__ import annotations

import datetime as _dt
import json
import os

from .canon import canonical_json, sha256_hex

SCHEMA = "mirror.surface.decision.v1"


def _now() -> str:
    return _dt.datetime.now(_dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


class Ledger:
    def __init__(self, path: str):
        self.path = path
        self.prev, self.n = None, 0
        if os.path.exists(path):
            with open(path, encoding="utf-8") as f:
                for line in f:
                    if line.strip():
                        self.prev = sha256_hex(canonical_json(json.loads(line)))
                        self.n += 1

    def _append(self, rec: dict) -> dict:
        rec["prev_tx_hash"] = self.prev
        line = canonical_json(rec)
        with open(self.path, "a", encoding="utf-8") as f:
            f.write(line + "\n")
        self.prev = sha256_hex(line)
        self.n += 1
        return rec

    def ensure_genesis(self, engine, timestamp: str | None = None):
        if self.n == 0:
            self._append({"schema": SCHEMA, "tx_id": "mirror-genesis", "timestamp": timestamp or _now(),
                          "type": "genesis", "rules_sha256": engine.rules_sha256,
                          "meanings_sha256": engine.meanings_sha256,
                          "note": "Mirror surface opened. Zero thickness: this ledger is the only state."})

    def commit(self, engine, decision: dict, timestamp: str | None = None, wall_ms: int | None = None) -> dict:
        """The sentinel lets the fuse burn out: the decision becomes irreversible curvature."""
        self.ensure_genesis(engine, timestamp)
        c = decision["core"]
        rec = {
            "schema": SCHEMA,
            "tx_id": f"mirror-{self.n:06d}",
            "timestamp": timestamp or _now(),
            "type": "sort",
            "symbol": decision["symbol"],
            "symbol_sha256": sha256_hex(canonical_json(decision["symbol"])),
            "decision": c,
            "core_sha256": decision["core_sha256"],
            "stream": c["stream"],
            "rules_fired": c["rules_fired"],
            "irreversible": True,
            "entered_bulk": c["entered_bulk"],
            "rules_sha256": engine.rules_sha256,
            "meaning": decision["meaning"],
        }
        if wall_ms is not None:
            rec["wall_ms"] = int(wall_ms)
        return self._append(rec)


def verify_chain(path: str) -> tuple[bool, int, str]:
    """Returns (ok, records_checked, message)."""
    prev, n = None, 0
    with open(path, encoding="utf-8") as f:
        for lineno, line in enumerate(f, 1):
            if not line.strip():
                continue
            try:
                rec = json.loads(line)
            except json.JSONDecodeError as e:
                return False, n, f"line {lineno}: invalid JSON ({e})"
            if rec.get("prev_tx_hash") != prev:
                return False, n, f"line {lineno} ({rec.get('tx_id')}): prev_tx_hash mismatch"
            if rec.get("type") == "sort":
                if sha256_hex(canonical_json(rec["decision"])) != rec.get("core_sha256"):
                    return False, n, f"line {lineno} ({rec.get('tx_id')}): core_sha256 mismatch"
                if rec.get("irreversible") is not True:
                    return False, n, f"line {lineno}: committed record not marked irreversible"
            prev = sha256_hex(canonical_json(rec))
            n += 1
    return True, n, f"chain ok: {n} records, head {prev}"


def curvature(path: str) -> dict:
    """Aggregate irreversible decisions. RED is counted but never enters the bulk."""
    counts = {"GOLD": 0, "CYAN": 0, "RED": 0}
    sums, bulk = {}, 0
    with open(path, encoding="utf-8") as f:
        for line in f:
            if not line.strip():
                continue
            rec = json.loads(line)
            if rec.get("type") != "sort":
                continue
            counts[rec["stream"]] += 1
            if rec["entered_bulk"]:
                bulk += 1
                for k, v in rec["decision"]["axes_centi"].items():
                    sums[k] = sums.get(k, 0) + v
    mean = {k: (round(v / bulk) if bulk else 0) for k, v in sorted(sums.items())}
    return {"counts": counts, "bulk_records": bulk, "mean_axes_centi_bulk": mean}
