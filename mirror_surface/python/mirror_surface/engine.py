"""Deterministic, rule-table-driven sorter. No model, no randomness, no wall clock in the decision."""
from __future__ import annotations

import json
import os
from dataclasses import dataclass, field

from .canon import canonical_json, sha256_hex

PKG_DIR = os.path.dirname(os.path.abspath(__file__))
STREAMS = ("GOLD", "CYAN", "RED")


def find_home(start: str | None = None) -> str:
    """Directory containing rules/rules.json: $MIRROR_SURFACE_HOME, else walk up from the package."""
    env = os.environ.get("MIRROR_SURFACE_HOME")
    if env:
        return env
    d = os.path.abspath(start or PKG_DIR)
    while True:
        if os.path.exists(os.path.join(d, "rules", "rules.json")):
            return d
        parent = os.path.dirname(d)
        if parent == d:
            raise FileNotFoundError("rules/rules.json not found (set MIRROR_SURFACE_HOME)")
        d = parent


def _default_dir(name: str) -> str:
    return os.path.join(find_home(), "rules", name)


def tokenize(text: str) -> list[str]:
    """ASCII-lowercase; split on any ASCII char outside [a-z0-9]; non-ASCII chars stay in tokens."""
    out, cur = [], []
    for ch in text:
        if "A" <= ch <= "Z":
            ch = chr(ord(ch) + 32)
        if ord(ch) < 128 and not ("a" <= ch <= "z" or "0" <= ch <= "9"):
            if cur:
                out.append("".join(cur))
                cur = []
        else:
            cur.append(ch)
    if cur:
        out.append("".join(cur))
    return out


def _phrase_in(phrase: list[str], toks: list[str]) -> bool:
    n = len(phrase)
    if n == 0:
        return False
    return any(toks[i:i + n] == phrase for i in range(len(toks) - n + 1))


def normalize_symbol(sym) -> dict:
    if isinstance(sym, str):
        sym = {"text": sym}
    text = str(sym.get("text", ""))
    sid = sym.get("id") or "sym-" + sha256_hex(text)[:12]
    tags = sorted({str(t) for t in sym.get("tags", [])})
    return {"id": str(sid), "text": text, "tags": tags}


@dataclass
class Engine:
    rules: dict
    meanings: dict
    rules_sha256: str
    meanings_sha256: str
    _kw: dict = field(default_factory=dict, repr=False)

    def __post_init__(self):
        self._kw = {f["id"]: [tokenize(k) for k in f["keywords"]] for f in self.rules["features"]}
        self._feature_ids = [f["id"] for f in self.rules["features"]]
        self._ordered_rules = sorted(enumerate(self.rules["rules"]), key=lambda p: (-p[1]["priority"], p[0]))

    # ---- measured state -------------------------------------------------
    def sort(self, symbol, budget_ticks: int | None = None) -> dict:
        sym = normalize_symbol(symbol)
        budget = self.rules["fuse"]["budget_ticks"] if budget_ticks is None else int(budget_ticks)
        toks = tokenize(sym["text"])
        declared = set(sym["tags"])
        known = set(self._feature_ids)
        spent, exhausted = 0, False

        present = set()
        for fid in self._feature_ids:
            if spent >= budget:
                exhausted = True
                break
            spent += 1
            if fid in declared or any(_phrase_in(p, toks) for p in self._kw[fid]):
                present.add(fid)

        axes = {}
        for ax in self.rules["axes"]:
            s = sum(w for f, w in ax["weights"].items() if f in present)
            axes[ax["id"]] = max(-100, min(100, s))

        fired = []
        if not exhausted:
            for _, rule in self._ordered_rules:
                if spent >= budget:
                    exhausted = True
                    break
                spent += 1
                if self._rule_fires(rule, present, axes):
                    fired.append(rule)

        if fired:
            deciding = fired[0]
            stream, deciding_id, reason = deciding["stream"], deciding["id"], deciding["reason"]
        else:
            d = self.rules["default"]
            stream, deciding_id, reason = d["stream"], d["rule_id"], d["reason"]

        sconf = self.rules["streams"][stream]
        core = {
            "id": sym["id"],
            "features": sorted(present),
            "unknown_tags": sorted(declared - known),
            "axes_centi": axes,
            "stream": stream,
            "rules_fired": [r["id"] for r in fired],
            "deciding_rule": deciding_id,
            "fuse": {"budget_ticks": budget, "spent_ticks": spent, "exhausted": exhausted,
                     "heart_cost": sconf["heart_cost"]},
            "entered_bulk": bool(sconf["enters_bulk"]),
        }
        core_sha = sha256_hex(canonical_json(core))
        return {"symbol": sym, "core": core, "core_sha256": core_sha, "reason": reason,
                "meaning": self.meaning(stream)}

    @staticmethod
    def _rule_fires(rule, present, axes) -> bool:
        if not all(f in present for f in rule.get("all", [])):
            return False
        anyl = rule.get("any", [])
        if anyl and not any(f in present for f in anyl):
            return False
        if any(f in present for f in rule.get("none", [])):
            return False
        for ax, cond in rule.get("axes", {}).items():
            v = axes[ax]
            if "gte" in cond and v < cond["gte"]:
                return False
            if "lte" in cond and v > cond["lte"]:
                return False
        return True

    # ---- assigned meaning (separate layer) ------------------------------
    def meaning(self, stream: str) -> dict:
        m = self.meanings["streams"][stream]
        return {"label": m["label"], "surface_role": m["surface_role"],
                "reflectivity_permille": m["reflectivity_permille"], "sentinel": m["sentinel"],
                "meanings_sha256": self.meanings_sha256}


def reflect(decision: dict) -> dict:
    """Every emitted output is itself a new symbol fed back into the sorter."""
    c = decision["core"]
    return {"id": f"{c['id']}~r", "text": f"emitted {c['stream']} decision on {c['id']}",
            "tags": ["emitted"]}


def load_engine(rules_path: str | None = None, meanings_path: str | None = None) -> Engine:
    rules_path = rules_path or _default_dir("rules.json")
    meanings_path = meanings_path or _default_dir("meanings.json")
    with open(rules_path, "rb") as f:
        rb = f.read()
    with open(meanings_path, "rb") as f:
        mb = f.read()
    rules, meanings = json.loads(rb), json.loads(mb)
    validate_rules(rules, meanings)
    return Engine(rules, meanings, sha256_hex(rb), sha256_hex(mb))


def validate_rules(rules: dict, meanings: dict) -> None:
    fids = [f["id"] for f in rules["features"]]
    if len(set(fids)) != len(fids):
        raise ValueError("duplicate feature ids")
    fset, axids = set(fids), {a["id"] for a in rules["axes"]}
    if len(rules["axes"]) != 5:
        raise ValueError("exactly five axes required")
    for a in rules["axes"]:
        for f, w in a["weights"].items():
            if f not in fset or not isinstance(w, int):
                raise ValueError(f"axis {a['id']}: bad weight {f}={w!r}")
    seen = set()
    for r in rules["rules"]:
        if r["id"] in seen:
            raise ValueError(f"duplicate rule id {r['id']}")
        seen.add(r["id"])
        if r["stream"] not in STREAMS:
            raise ValueError(f"rule {r['id']}: unknown stream")
        for k in ("all", "any", "none"):
            for f in r.get(k, []):
                if f not in fset:
                    raise ValueError(f"rule {r['id']}: unknown feature {f}")
        for ax in r.get("axes", {}):
            if ax not in axids:
                raise ValueError(f"rule {r['id']}: unknown axis {ax}")
    for s in STREAMS:
        if s not in rules["streams"] or s not in meanings["streams"]:
            raise ValueError(f"stream {s} missing from rules or meanings")
