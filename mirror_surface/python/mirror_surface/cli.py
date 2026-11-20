"""mirror-sort CLI.

  mirror-sort "E=mc^2" "The cross"                  # sort text symbols (dry run: not committed)
  mirror-sort sort --json symbols.json --ledger curvature.jsonl --feedback 1
  echo '{"id":"x","text":"if(condition)"}' | mirror-sort sort --json -
  mirror-sort verify-chain curvature.jsonl
  mirror-sort curvature curvature.jsonl
  mirror-sort seed [--ledger L]                     # sort the seed fixture
  mirror-sort golden --out vectors/golden.json      # regenerate cross-language vectors
"""
from __future__ import annotations

import argparse
import json
import os
import sys
import time

from .engine import find_home, load_engine, reflect
from .ledger import Ledger, curvature, verify_chain
from .canon import canonical_json

SUBCOMMANDS = {"sort", "verify-chain", "curvature", "seed", "golden"}


def _load_symbols(arg: str):
    data = json.load(sys.stdin if arg == "-" else open(arg, encoding="utf-8"))
    if isinstance(data, dict) and "symbols" in data:
        return list(data["symbols"])
    if isinstance(data, dict):
        return [data]
    return list(data)


def _run_sort(engine, symbols, args) -> int:
    ledger = Ledger(args.ledger) if args.ledger and not args.no_commit else None
    for sym in symbols:
        depth = 0
        while True:
            t0 = time.perf_counter()
            d = engine.sort(sym, budget_ticks=args.budget)
            wall = int((time.perf_counter() - t0) * 1000)
            out = {"stream": d["core"]["stream"], "reason": d["reason"], "symbol": d["symbol"],
                   "core": d["core"], "core_sha256": d["core_sha256"], "meaning": d["meaning"],
                   "irreversible": False}
            if ledger:
                rec = ledger.commit(engine, d, timestamp=args.timestamp, wall_ms=wall)
                out.update(irreversible=True, tx_id=rec["tx_id"], prev_tx_hash=rec["prev_tx_hash"])
            if args.brief:
                c = d["core"]
                print(f"{c['stream']:4}  {c['id']:32} {c['deciding_rule']:38} {','.join(c['features'])}")
            else:
                print(json.dumps(out, indent=2 if args.pretty else None, ensure_ascii=False))
            if depth >= args.feedback:
                break
            sym, depth = reflect(d), depth + 1
    return 0


def main(argv=None) -> int:
    argv = list(sys.argv[1:] if argv is None else argv)
    if not argv or (argv[0] not in SUBCOMMANDS and argv[0] not in ("-h", "--help")):
        argv = ["sort"] + argv
    p = argparse.ArgumentParser(prog="mirror-sort", description="Five-axis, three-stream mirror-surface symbol sorter.")
    p.add_argument("--rules", help="rules.json path")
    p.add_argument("--meanings", help="meanings.json path")
    sub = p.add_subparsers(dest="cmd", required=True)

    def sort_opts(sp):
        sp.add_argument("--ledger", help="append irreversible receipts to this hash-chained JSONL")
        sp.add_argument("--no-commit", action="store_true", help="dry run even if --ledger is given")
        sp.add_argument("--feedback", type=int, default=0, help="re-sort each emitted decision N times")
        sp.add_argument("--budget", type=int, default=None, help="override fuse budget (ticks)")
        sp.add_argument("--timestamp", help="fixed ISO timestamp (tests/reproducibility)")
        sp.add_argument("--pretty", action="store_true")
        sp.add_argument("--brief", action="store_true", help="one line per decision")

    s = sub.add_parser("sort", help="sort symbols given as text args, or --json FILE|-")
    s.add_argument("text", nargs="*")
    s.add_argument("--json", dest="json_path")
    sort_opts(s)
    sd = sub.add_parser("seed", help="sort the seed fixture (and contrast cases with --contrast)")
    sd.add_argument("--contrast", action="store_true")
    sort_opts(sd)
    v = sub.add_parser("verify-chain", help="verify a curvature ledger")
    v.add_argument("path")
    c = sub.add_parser("curvature", help="summarize a curvature ledger")
    c.add_argument("path")
    g = sub.add_parser("golden", help="regenerate cross-language golden vectors")
    g.add_argument("--out", default=None)
    args = p.parse_args(argv)

    if args.cmd == "verify-chain":
        ok, n, msg = verify_chain(args.path)
        print(msg)
        return 0 if ok else 1
    if args.cmd == "curvature":
        print(json.dumps(curvature(args.path), indent=2))
        return 0

    engine = load_engine(args.rules, args.meanings)
    home = os.path.dirname(os.path.dirname(os.path.abspath(args.rules))) if args.rules else find_home()
    if args.cmd == "golden":
        vec = build_golden(engine, os.path.join(home, "fixtures", "seed_symbols.json"))
        out = args.out or os.path.join(home, "vectors", "golden.json")
        with open(out, "w", encoding="utf-8") as f:
            f.write(json.dumps(vec, indent=2, ensure_ascii=True) + "\n")
        print(f"wrote {len(vec['cases'])} cases to {out}")
        return 0
    if args.cmd == "seed":
        fx = json.load(open(os.path.join(home, "fixtures", "seed_symbols.json"), encoding="utf-8"))
        syms = fx["symbols"] + (fx["contrast"] if args.contrast else [])
        return _run_sort(engine, syms, args)
    symbols = list(args.text)
    if args.json_path:
        symbols += _load_symbols(args.json_path)
    if not symbols:
        p.error("no symbols given")
    return _run_sort(engine, symbols, args)


def build_golden(engine, fixture_path: str) -> dict:
    fx = json.load(open(fixture_path, encoding="utf-8"))
    syms = fx["symbols"] + fx["contrast"]
    cases = []
    for sym in syms:
        d = engine.sort(sym)
        cases.append({"input": d["symbol"], "budget_ticks": None, "expect": d["core"], "core_sha256": d["core_sha256"]})
        r = reflect(d)
        dr = engine.sort(r)
        cases.append({"input": dr["symbol"], "budget_ticks": None, "expect": dr["core"], "core_sha256": dr["core_sha256"]})
    for b in (5, 30):  # fuse exhaustion cases
        d = engine.sort(syms[9], budget_ticks=b)
        cases.append({"input": d["symbol"], "budget_ticks": b, "expect": d["core"], "core_sha256": d["core_sha256"]})
    canon_probe = {"k": "\u03c8 caf\u00e9 \U0001f525 \"q\" \\ \n\t\x01", "b": True, "n": None, "i": -7, "a": []}
    return {
        "schema": "mirror.surface.golden.v1",
        "rules_sha256": engine.rules_sha256,
        "meanings_sha256": engine.meanings_sha256,
        "tokenize_cases": [{"text": t, "tokens": __import__("mirror_surface").tokenize(t)} for t in
                           ["if(condition)", "E=mc^2", "Maxwell E_parallel=0", "We are God (participatory, not autonomous)",
                            "\u03c8=0 caf\u00e9", "KV488", ""]],
        "canonical_probe": {"value": canon_probe, "canonical": canonical_json(canon_probe)},
        "cases": cases,
    }


if __name__ == "__main__":
    sys.exit(main())
