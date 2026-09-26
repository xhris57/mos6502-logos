# Generic transformers — mos6502-logos

Type:

```
T : I × P → O
witness(T, run)
residual(T)
```

Host machines and boards are `P.host`, not names of `T`.
Compose when `O1 ⊆ I2`.
Profile shrinks by deleting knobs that do not change `O`.

**Framing:** law is the pocket universe (Fibonacci *source* basin). The cycle twin is a coordinate system on that law (Galois *image* basin). Soft≠Physical. Do not claim silicon proof. Visual6502 is an observation cite, not a seed.

## Records

### cpu.mos6502

- I: instruction stream (opcode bytes in memory) + bus cycles (reads/writes the twin emits while clocking)
- P: clock / cycle budget; decimal-mode flag `D`; undocumented-opcode policy (official-only vs NOP/JAM stubs); reset vector at `$FFFC`
- O: register/bus state per cycle (`A X Y S P PC`, mem writes, cycle count)
- law: see `artifacts/LAW.md`
- witness: `make witness` → `artifacts/witness-first.*`; `make witness-klaus` → `artifacts/witness-klaus.*` (Klaus Dormann NMOS functional PASS, cycles=96241388, PC=$3469, test_case=$F0, max=$2B)
- residual: Φ1/Φ2 half-cycle bus timing collapsed; RDY/SO pins absent; full illegal-opcode matrix incomplete; 65C02 extended suite not run; Soft≠Physical (no Digilent flash / no pin timing)

### twin.cycle.go

- I: flat 64 KiB bus (`Read`/`Write`) + reset vector
- P: same as `cpu.mos6502` (soft Go coordinate system)
- O: `cpu.Snapshot` / dump line / JSON
- law: instruction-cycle FSM in `twin/cpu/`
- witness: FIRST WITNESS + Klaus Dormann soft harness (`tb/witness_klaus_test.go`)
- residual: not transistor-level; not bitstream; coordinate system ≠ silicon; 65C02/undoc/RDY/SO residual

### asm.6502.tools

- I: assembly source (labels, `.org`, `.byte`, `.word`)
- P: origin, mnemonic table (official set)
- O: byte image
- law: two-pass assemble (`tools/asm.py`)
- witness: `programs/first_witness.asm` bytes match embedded tb image
- residual: no linker / no reloc; zp vs abs heuristics only

Chain used as “law → soft prove”:

```
asm.6502.tools → cpu.mos6502 (via twin.cycle.go) → witness receipt
```

Flash / Digilent / kitchen program is **out of scope** for this repo (would be a different `T`, e.g. `fpga.program`). Soft≠Physical.

## Reduction rules

1. Same `(I,O)` ⇒ same `T`, different `P`.
2. If sweeping a field in `P` never changes `O`, drop it.
3. Law must fit one equation or one small state machine.
4. Flash / JTAG / kitchen program is not this transformer.
5. Do not mint a new house for a new `P`.
6. Fibonacci source / Galois image: changing coordinate system must not silently rewrite the pocket-universe law.
