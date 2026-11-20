import json
import os
import subprocess
import sys

import pytest

_PY = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, _PY)  # prefer this package even if a same-named folder sits higher up

from mirror_surface import load_engine, tokenize, reflect, canonical_json, sha256_hex, Ledger, verify_chain
from mirror_surface.engine import validate_rules
from mirror_surface.ledger import curvature
from mirror_surface.cli import build_golden, main

from mirror_surface.engine import find_home
HOME = find_home(_PY)
FIX = os.path.join(HOME, "fixtures", "seed_symbols.json")
GOLD = os.path.join(HOME, "vectors", "golden.json")


@pytest.fixture(scope="module")
def eng():
    return load_engine()


@pytest.fixture(scope="module")
def fixture():
    return json.load(open(FIX))


EXPECTED = {
    "S01-binary-predicates": "CYAN", "S02-if-condition": "CYAN", "S03-e-mc2": "CYAN", "S04-psi-0": "CYAN",
    "S05-maxwell": "CYAN", "S06-ads-cft": "CYAN", "S07-it-from-bit": "RED", "S08-dna": "CYAN",
    "S09-mozart-kv488": "CYAN", "S10-in-christ": "GOLD", "S11-the-cross": "GOLD", "S12-we-are-god": "GOLD",
    "S13-green-fish-people": "CYAN", "S14-holographic-theorem-image": "CYAN", "S15-parmenides-being": "CYAN",
    "S16-plato-forms": "CYAN", "S17-heraclitus-logos": "CYAN", "S18-aristotle-categories": "CYAN",
    "S19-socratic-elenchus": "CYAN", "S20-stoic-citadel": "CYAN", "S21-recursive-questioning": "CYAN",
    "S22-mirror-for-saints": "GOLD", "S23-kingdom-invariants": "GOLD", "S24-fire-testing-language": "CYAN",
    "X01-autonomous-divinization": "RED", "X02-spectacle": "RED", "X03-medium-above-word": "RED",
    "X04-orchard-fruit": "GOLD", "X05-unsorted": "RED", "X06-bare-we-are-god": "RED", "X07-non-ascii": "CYAN",
}


def test_seed_fixture_has_all_24_symbols(fixture):
    assert len(fixture["symbols"]) == 24


def test_expected_streams(eng, fixture):
    for s in fixture["symbols"] + fixture["contrast"]:
        assert eng.sort(s)["core"]["stream"] == EXPECTED[s["id"]], s["id"]


def test_every_decision_is_explained(eng, fixture):
    rule_ids = {r["id"] for r in eng.rules["rules"]} | {eng.rules["default"]["rule_id"]}
    for s in fixture["symbols"] + fixture["contrast"]:
        c = eng.sort(s)["core"]
        assert c["deciding_rule"] in rule_ids
        if c["rules_fired"]:
            assert c["deciding_rule"] == c["rules_fired"][0]
        else:
            assert c["deciding_rule"] == "RED-00-UNSORTED-DEFAULT"


def test_axes_bounded_and_five(eng, fixture):
    for s in fixture["symbols"]:
        ax = eng.sort(s)["core"]["axes_centi"]
        assert len(ax) == 5 and all(-100 <= v <= 100 and isinstance(v, int) for v in ax.values())


def test_deterministic(eng):
    a = eng.sort({"id": "a", "text": "The cross"})
    b = eng.sort({"id": "a", "text": "The cross"})
    assert a == b


def test_red_veto_beats_gold(eng):
    c = eng.sort("We are God, the cross is irrelevant, autonomous")["core"]
    assert c["stream"] == "RED" and "GOLD-01-CHRIST-INVARIANT" in c["rules_fired"]


def test_participatory_marker_flips_red_to_gold(eng):
    assert eng.sort("We are God")["core"]["stream"] == "RED"
    assert eng.sort({"text": "We are God", "tags": ["participatory"]})["core"]["stream"] == "GOLD"


def test_unknown_tags_reported(eng):
    c = eng.sort({"text": "x", "tags": ["not_a_feature"]})["core"]
    assert c["unknown_tags"] == ["not_a_feature"]


def test_tokenizer():
    assert tokenize("if(condition)") == ["if", "condition"]
    assert tokenize("E=mc^2") == ["e", "mc", "2"]
    assert tokenize("\u03c8=0 Caf\u00e9") == ["\u03c8", "0", "caf\u00e9"]
    assert tokenize("orbit") == ["orbit"]


def test_whole_token_matching(eng):
    assert "binary" not in eng.sort("orbit habit")["core"]["features"]


def test_meaning_layer_is_separate(eng, tmp_path):
    m = json.load(open(os.path.join(HOME, "rules", "meanings.json")))
    m["streams"]["GOLD"]["label"] = "renamed"
    m["streams"]["GOLD"]["reflectivity_permille"] = 1
    mp = tmp_path / "m.json"
    mp.write_text(json.dumps(m))
    e2 = load_engine(meanings_path=str(mp))
    d1, d2 = eng.sort("The cross"), e2.sort("The cross")
    assert d1["core"] == d2["core"] and d1["core_sha256"] == d2["core_sha256"]
    assert d2["meaning"]["label"] == "renamed"


def test_rule_table_is_editable(tmp_path):
    r = json.load(open(os.path.join(HOME, "rules", "rules.json")))
    for f in r["features"]:
        if f["id"] == "lens":
            f["keywords"] = []
    rp = tmp_path / "r.json"
    rp.write_text(json.dumps(r))
    e2 = load_engine(rules_path=str(rp), meanings_path=os.path.join(HOME, "rules", "meanings.json"))
    assert e2.sort({"text": "Plato's Forms", "tags": ["metaphysical_thesis"]})["core"]["stream"] == "RED"


def test_validate_rejects_bad_table():
    r = json.load(open(os.path.join(HOME, "rules", "rules.json")))
    m = json.load(open(os.path.join(HOME, "rules", "meanings.json")))
    r["rules"][0]["all"] = ["nope"]
    with pytest.raises(ValueError):
        validate_rules(r, m)


def test_fuse_exhaustion(eng):
    full = eng.sort("The cross")["core"]["fuse"]
    assert not full["exhausted"] and full["spent_ticks"] <= full["budget_ticks"]
    c = eng.sort("The cross", budget_ticks=5)["core"]
    assert c["fuse"]["exhausted"] and c["fuse"]["spent_ticks"] == 5
    assert c["stream"] == "RED" and c["deciding_rule"] == "RED-00-UNSORTED-DEFAULT"
    c = eng.sort("The cross", budget_ticks=30)["core"]
    assert c["fuse"]["exhausted"] and c["fuse"]["spent_ticks"] == 30


def test_feedback_reflection(eng):
    d = eng.sort("E=mc^2")
    r = eng.sort(reflect(d))
    assert r["symbol"]["id"].endswith("~r") and r["core"]["stream"] == "CYAN"


def test_ledger_chain_and_tamper(eng, tmp_path):
    p = str(tmp_path / "ledger.jsonl")
    L = Ledger(p)
    for t in ["The cross", "E=mc^2", "lorem ipsum"]:
        rec = L.commit(eng, eng.sort(t), timestamp="2026-11-19T00:00:00Z")
        assert rec["irreversible"] is True
    ok, n, _ = verify_chain(p)
    assert ok and n == 4  # genesis + 3
    # reopen and continue the chain
    Ledger(p).commit(eng, eng.sort("DNA double helix"), timestamp="2026-11-19T00:00:01Z")
    assert verify_chain(p)[0]
    cv = curvature(p)
    assert cv["counts"] == {"GOLD": 1, "CYAN": 2, "RED": 1} and cv["bulk_records"] == 3
    lines = open(p).read().splitlines()
    rec = json.loads(lines[2])
    rec["decision"]["stream"] = "GOLD"
    lines[2] = canonical_json(rec)
    open(p, "w").write("\n".join(lines) + "\n")
    ok, _, msg = verify_chain(p)
    assert not ok


def test_ledger_hash_matches_workspace_convention():
    # prev_tx_hash = sha256(canonical_json(prev record)), as in /workspace/ledger-current.jsonl
    a = {"tx_id": "g", "prev_tx_hash": None, "note": "\u2014"}
    assert canonical_json(a) == '{"note":"\\u2014","prev_tx_hash":null,"tx_id":"g"}'


def test_golden_vectors_current(eng):
    vec = json.load(open(GOLD))
    fresh = build_golden(eng, FIX)
    assert vec == json.loads(json.dumps(fresh)), "run `mirror-sort golden` after editing rules"
    for case in vec["cases"]:
        d = eng.sort(case["input"], budget_ticks=case["budget_ticks"])
        assert d["core"] == case["expect"] and d["core_sha256"] == case["core_sha256"]


def test_cli_end_to_end(tmp_path, capsys):
    p = str(tmp_path / "l.jsonl")
    assert main(["The cross", "E=mc^2", "--ledger", p, "--feedback", "1", "--timestamp", "2026-11-19T00:00:00Z"]) == 0
    out = [json.loads(l) for l in capsys.readouterr().out.splitlines()]
    assert [o["stream"] for o in out] == ["GOLD", "CYAN", "CYAN", "CYAN"]
    assert all(o["irreversible"] for o in out)
    assert main(["verify-chain", p]) == 0
    assert main(["The cross"]) == 0
    assert json.loads(capsys.readouterr().out.splitlines()[-1])["irreversible"] is False


def test_cli_stdin_json(tmp_path):
    r = subprocess.run([sys.executable, "-m", "mirror_surface", "sort", "--json", "-"], cwd=_PY,
                       input='[{"id":"q","text":"if(condition)"}]', capture_output=True, text=True)
    assert r.returncode == 0 and json.loads(r.stdout)["stream"] == "CYAN"
