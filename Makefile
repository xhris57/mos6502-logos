.PHONY: witness witness-klaus twin-test asm

# FIRST WITNESS — Soft≠Physical. Writes artifacts/witness-first.{json,txt}
witness:
	go test ./tb/ -run TestFirstWitness -count=1 -v

# Klaus Dormann NMOS functional — Soft≠Physical. Writes artifacts/witness-klaus.{json,txt}
# Digilent mutex Operator — NO flash.
witness-klaus:
	go test ./tb/ -run TestKlausDormannFunctional -count=1 -v -timeout 10m

twin-test:
	go test ./twin/cpu/ ./tb/ -count=1 -v -timeout 10m

asm:
	python3 tools/asm.py programs/first_witness.asm 0x8000
