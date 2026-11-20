"""Canonical JSON: sorted keys, compact separators, ASCII-escaped (== json.dumps(sort_keys=True,
separators=(',', ':'))). Same convention as /workspace/ledger-current.jsonl prev_tx_hash.
Receipts contain no floats, so every port can reproduce the bytes exactly."""
import hashlib
import json


def canonical_json(obj) -> str:
    return json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=True, allow_nan=False)


def sha256_hex(data) -> str:
    if isinstance(data, str):
        data = data.encode("utf-8")
    return hashlib.sha256(data).hexdigest()
