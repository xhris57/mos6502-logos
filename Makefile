.PHONY: witness twin-test asm

# FIRST WITNESS — Soft≠Physical. Writes artifacts/witness-first.{json,txt}
witness:
	go test ./tb/ -count=1 -v

twin-test:
	go test ./twin/cpu/ ./tb/ -count=1 -v

asm:
	python3 tools/asm.py programs/first_witness.asm 0x8000
