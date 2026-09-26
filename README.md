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

```bash
make witness    # go test ./tb/  — writes artifacts/witness-first.{json,txt}
```

## RESIDUAL (named honestly)

1. **Soft≠Physical** — no Digilent flash; no pin / Φ1Φ2 half-cycle bus timing; instruction-cycle twin only.
2. **Not silicon-proven** — twin↔law agreement is soft; Visual6502 is observation cite, not seed / not transistor netlist.
3. **Undocumented opcodes** — twin stubs NOP/JAM only; full illegal matrix residual.
4. **RDY / SO / mid-instruction IRQ edge cases** — not modeled.
5. **Klaus Dormann functional test** — not fetched/run in v1 (hand FIRST WITNESS instead).
6. **Opcode×mode×cycle scale** — ~151 official encodings × variable cycles ≫ SN76489 LFSR witness; LAW tables start with documented 56 mnemonics + modes; many rows still **assumed** until witnessed.
7. **BCD N/Z/V** — NMOS teaching rule as in harvested twins; corner vs every die residual.

## Proven vs assumed

**Proven in this repo (soft):** FIRST WITNESS checks above — `result=PASS err=0` in `artifacts/witness-first.txt`.

**Assumed (not silicon-proven here):** remaining LAW rows; half-cycle bus; analog/electrical; any FPGA bitstream; Visual6502 geometry.

## Layout

- `TRANSFORMERS.md` — catalog (`cpu.mos6502`, twin, asm) matching linguistic-fabric form
- `artifacts/LAW.md` — law tables
- `artifacts/witness-first.{json,txt}` — receipt
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
```
