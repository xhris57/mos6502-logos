# mirror_surface: symbol sorter (Crystal feature)

A self-contained, additive copy of the mirror-surface sorter. Nothing outside this folder is touched.

* **Five axes**, each an integer score in [-100, 100]: Binary↔Continuous, Symbolic↔Material, Tested↔Novelty,
  Personal Logos↔Impersonal Form, Fire-tested↔Aesthetic.
* **Three streams**: GOLD (Christ-aligned, fire-tested invariants → coating), CYAN (computable / formal / testable →
  lattice), RED (discarded novelty → reflected away, never enters the bulk).
* **Sentinel fuse**: every decision spends logical ticks against a budget and carries a heart cost. A committed decision is
  irreversible.
* **Ledger**: receipts go into an append-only JSONL file chained with sha256 (`prev_tx_hash` = sha256 of the previous
  record's canonical JSON).
* **Transparent and deterministic**: `rules/rules.json` holds the features, weights and rules. `rules/meanings.json` holds
  the meaning layer, kept apart from the measured state. Every receipt names the rule ids that fired.

| Path | Role |
|---|---|
| `rules/` | Shared, editable rule table and meaning layer (same format in every language) |
| `fixtures/seed_symbols.json` | The 24 seed symbols plus contrast cases |
| `vectors/golden.json` | Cross-language golden vectors (Python, Swift, TypeScript and C all pass them) |
| `python/ (reference implementation, stdlib only)` | Implementation for this repo's primary language |

Run from the repo root:

```bash
cd mirror_surface/python && python -m pytest -q tests && python -m mirror_surface seed --brief
```

Canonical source: `mirror-sorter` (Python reference, Swift, TypeScript and C ports; `./test-all.sh`).
