# mos6502-logos

MOS 6502 logos: **Transformer law + cycle twin + witness + residual**.

Soft≠Physical. No Digilent flash. Soft twin ≠ Physical silicon. Do not claim silicon accuracy unwitnessed.

## TYPE

```
T : I × P → O
witness(T, run)
residual(T)
```

Named once:

| | |
|--|--|
| **I** | instruction stream + bus cycles |
| **P** | clock, decimal-mode flag `D`, undocumented-opcode policy, reset vector |
| **O** | register/bus state per cycle |

Law is the pocket universe (**Fibonacci source**). The Go cycle twin is a coordinate system on that law (**Galois image**). Changing coordinates must not silently rewrite the law. Catalog: `TRANSFORMERS.md`.

## LAW

One-line FSM: `RESET → fetch → decode(mode) → execute(bus/ALU/flags) → …`  
Tables: `artifacts/LAW.md` (opcode×mode×cycle register/bus/flag transitions for the documented official set).

## WITNESS

| Claim | Evidence |
|-------|----------|
| Reset vector fetch, 7 cycles, I set | `tb/witness_first_test.go` → `artifacts/witness-first.*` |
| LDA/STA imm + zp + Z/N | same |
| ADC overflow ±C; SBC ±C; BCD 9+1→10 | same |
| BEQ page-cross cycles; BNE taken | same |
| JSR/RTS PC + 6/6 cycles | same |
| PHP/PLA B+U; PHA/PLA | same |
| Hand program sum/JSR/BRK vector | same |
| **Klaus Dormann NMOS functional (all official opcode×mode sections $00–$2B → $F0)** | `tb/witness_klaus_test.go` → `artifacts/witness-klaus.*` — soft PASS, err=0, cycles=96241388, PC=$3469 |
| **Undoc/illegal NMOS matrix (stable composites)** | `tb/witness_undoc_test.go` → `artifacts/witness-undoc.*` — matched=97 differed=0 unclaimed=8; Soft≠Physical |

```bash
make witness         # FIRST WITNESS → artifacts/witness-first.{json,txt}
make witness-klaus   # Klaus Dormann → artifacts/witness-klaus.{json,txt}
make witness-undoc   # illegal/undoc matrix → artifacts/witness-undoc.{json,txt}
```

ROM provenance: `testdata/SOURCE.txt` (Klaus2m5 @ `7954e2db…`, SHA256 `fa12bfc7…`).

## RESIDUAL (named honestly)

1. **Soft≠Physical** — no Digilent flash; no pin / Φ1Φ2 half-cycle bus timing; instruction-cycle twin only.
2. **Not silicon-proven** — twin↔law agreement is soft; Visual6502 is observation cite, not seed / not transistor netlist.
3. **Undocumented opcodes** — stable composites soft-matched (`witness-undoc`); UNCLAIMED residual: XAA/LAX#/AHX/SHY/SHX/TAS/LAS (+ ARR D=1). Soft≠Physical.
4. **RDY / SO / mid-instruction IRQ edge cases** — not modeled.
5. **65C02 extras** — extended-opcode suite not run; not claimed.
6. **Opcode×mode×cycle φ timing** — Klaus witnesses **functional** result/flags/modes for the official NMOS set; per-cycle bus geometry / half-cycle still **assumed**.
7. **BCD N/Z/V** — NMOS teaching rule; Klaus decimal focuses valid-BCD carry; corner vs every die residual.

## Proven vs assumed

**Proven in this repo (soft):**
- FIRST WITNESS — `artifacts/witness-first.txt` (`result=PASS err=0`).
- Klaus Dormann NMOS functional — `artifacts/witness-klaus.txt` (`result=PASS err=0 cycles=96241388 pc_end=3469 max_test_case=2B`). Soft≠Physical.

**Newly witnessed (soft, via Klaus):** official NMOS opcode×mode×flag matrices exercised by sections through `$2B` (loads/stores/ALU/shifts/RMW/branches/stack/JMP/JSR/BRK/decimal ADC·SBC, etc.).

**Still residual / assumed:** 65C02 extras; unstable illegals UNCLAIMED; RDY/SO; Φ1/Φ2 half-cycle bus; silicon / FPGA bitstream; Visual6502 geometry; BCD N/V/Z die corners; ARR D=1.

## Layout

- `TRANSFORMERS.md` — catalog (`cpu.mos6502`, twin, asm) matching linguistic-fabric form
- `artifacts/LAW.md` — law tables
- `artifacts/witness-first.{json,txt}` — FIRST WITNESS receipt
- `artifacts/witness-klaus.{json,txt}` — Klaus Dormann soft receipt
- `artifacts/witness-undoc.{json,txt}` + `SOURCE-undoc.txt` — illegal/undoc soft matrix
- `testdata/6502_functional_test.bin` + `SOURCE.txt` — Klaus ROM + provenance
- `twin/cpu/` — Go cycle twin (reduced from pi6502-go) + dump format
- `tb/` — FIRST WITNESS
- `tools/asm.py` — harvested assembler
- `programs/first_witness.asm` — hand program source

## Harvest / cites

| source | SHA | role |
|--------|-----|------|
| `xhris57/pi6502-go` | `3505eafe13ab90b579f12586d0ae7ad129ea9c4b` | twin seed (`internal/cpu/`) |
| `xhris57/rpi-6502` | `58914ac24cac0f62e8b4a40523e7baa98e2ce4e8` | SYMBOLS / asm / C tests (pattern) |
| `xhris57/sn76489-logos` | `9d715ed7` / tree `9d715edef68c913e8df8681000dc9d2e428c1d27` | logos structure pattern |
| Visual6502 | observation cite only | **not** seed |

Ignore `*-private` duplicates.

## Quick check

```bash
make witness
cat artifacts/witness-first.txt
make witness-klaus
cat artifacts/witness-klaus.txt
make witness-undoc
cat artifacts/witness-undoc.txt
```
