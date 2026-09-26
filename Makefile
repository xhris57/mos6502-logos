.PHONY: witness witness-klaus witness-undoc witness-wozmon twin-test asm

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

# Undoc/illegal NMOS matrix — Soft≠Physical. Writes artifacts/witness-undoc.{json,txt}
# Digilent mutex Operator — NO flash. Unstable opcodes stay UNCLAIMED.
witness-undoc:
	go test ./tb/ -run TestUndocMatrix -count=1 -v

# Soft Apple-1 WOZMON platform — Soft≠Physical. Writes artifacts/witness-wozmon.{json,txt}
# Digilent mutex Operator — NO flash. No real PIA / NTSC / physical Apple-1.
witness-wozmon:
	go test ./tb/ -run TestWozmonSoftPlatform -count=1 -v
