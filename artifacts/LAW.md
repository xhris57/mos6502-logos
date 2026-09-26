# cpu.mos6502 law

**Soft≠Physical.** This is original teaching prose reduced from public opcode tables and from the harvested soft twins (`xhris57/pi6502-go` @ `3505eafe…`, `xhris57/rpi-6502` @ `58914ac2…`). Not a paste of MOS datasheets. Visual6502 is an **observation cite** only — not a seed.

**Pocket universe / coordinate system:** the law (opcode×mode×cycle transitions) is the Fibonacci *source* basin — what the machine *is*. The Go twin is a Galois *image* coordinate system that clocks that law. Matching basins under the twin does **not** prove silicon.

## Type (named once)

```
T : I × P → O
I = instruction stream + bus cycles
P = clock, decimal-mode flag D, undocumented-opcode policy, reset vector
O = register/bus state per cycle
```

## One-line / FSM

```
RESET → (fetch opcode @ PC) → decode(mode) → execute(bus + ALU + flags) → advance PC/cycles → …
```

Interrupt edges (NMI one-shot, IRQ level∧¬I) sample before opcode fetch. Stack lives at `$0100|S`.

## Registers

| name | width | role |
|------|-------|------|
| A | 8 | accumulator |
| X Y | 8 | index |
| S | 8 | stack pointer (page `$01`) |
| P | 8 | NV-BDIZC (B not live; U forced on push) |
| PC | 16 | program counter |
| cycles | counter | supervisor budget (not on-die) |

## Address modes (cycle base, before page-cross penalties)

| mode | bytes | typical read cycles | notes |
|------|-------|---------------------|-------|
| implied / accum | 1 | 2 | |
| immediate | 2 | 2 | |
| zero page | 2 | 3 | |
| zp,X / zp,Y | 2 | 4 | wrap in page 0 |
| absolute | 3 | 4 | |
| abs,X / abs,Y | 3 | 4(+1 cross) | stores often 5 fixed |
| (zp,X) | 2 | 6 | pointer wraps in zp |
| (zp),Y | 2 | 5(+1 cross) | stores 6 fixed |
| relative | 2 | 2(+1 taken)(+1 cross) | |
| (abs) JMP | 3 | 5 | NMOS: `$xxFF` hi from `$xx00` |

## Documented official set (56 mnemonics × modes) — transitions

Law tables below state **what changes** (register / bus / flags / cycles). Undocumented opcodes: stable NMOS composites (SLO/RLA/SRE/RRA/SAX/LAX/DCP/ISC/ANC/ALR/ARR(D=0)/AXS/USBC) + NOP/JAM claimed soft per artifacts/SOURCE-undoc.txt; unstable XAA/LAX#/AHX/SHY/SHX/TAS/LAS remain UNCLAIMED residual. Soft≠Physical.

### Loads / stores

| op | modes | O effect | flags | cycles |
|----|-------|----------|-------|--------|
| LDA | imm zp zpx abs absx absy indx indy | A←M | N Z | 2/3/4/4+/4+/6/5+ |
| LDX | imm zp zpy abs absy | X←M | N Z | 2/3/4/4/4+ |
| LDY | imm zp zpx abs absx | Y←M | N Z | 2/3/4/4/4+ |
| STA | zp zpx abs absx absy indx indy | M←A | — | 3/4/4/5/5/6/6 |
| STX | zp zpy abs | M←X | — | 3/4/4 |
| STY | zp zpx abs | M←Y | — | 3/4/4 |

### ALU

| op | modes | O effect | flags | notes |
|----|-------|----------|-------|-------|
| ADC | imm…indy | A←A+M+C | N V Z C | if D: BCD nibble correct; V from binary sum (NMOS teaching rule) |
| SBC | imm…indy | A←A−M−(1−C) | N V Z C | D: BCD borrow correct |
| AND/ORA/EOR | imm…indy | A←A∘M | N Z | |
| CMP/CPX/CPY | (as loads) | — | N Z C | C if reg≥M; NZ from reg−M |
| BIT | zp abs | — | N V Z | N V←M765; Z←A∧M |

### Shifts / RMW

| op | modes | O effect | flags | cycles |
|----|-------|----------|-------|--------|
| ASL | A zp zpx abs absx | ≪1, C←bit7 | N Z C | 2/5/6/6/7 |
| LSR | A zp zpx abs absx | ≫1, C←bit0 | N Z C | same |
| ROL/ROR | A zp zpx abs absx | 9-bit through C | N Z C | same |
| INC/DEC | zp zpx abs absx | M±1 | N Z | 5/6/6/7 |
| INX/INY/DEX/DEY | impl | index±1 | N Z | 2 |

### Control / stack

| op | O effect | cycles |
|----|----------|--------|
| JMP abs | PC←addr | 3 |
| JMP (abs) | PC←rd16_wrap(addr) | 5 |
| JSR abs | push PC−1; PC←addr | 6 |
| RTS | PC←pull16+1 | 6 |
| BRK | push PC+1, P\|B\|U; I←1; PC←$FFFE | 7 |
| RTI | P←pull; PC←pull16 | 6 |
| PHA/PHP | push A / P\|B\|U | 3 |
| PLA/PLP | A←pull (+NZ) / P←pull | 4 |
| Bcc rel | if cond: PC+=off; +1; +1 if page | 2/3/4 |
| CLC/SEC/CLI/SEI/CLD/SED/CLV | flag set/clear | 2 |
| TAX/TAY/TXA/TYA/TXS/TSX | transfer (+NZ except TXS) | 2 |
| NOP | — | 2 |
| RESET | S←S−3; I U set; PC←$FFFC | 7 |

## Profile (essential P)

- `clock` / cycle budget (soft)
- `D` decimal mode
- undocumented policy (`official` | `nop_jam_stubs`)
- reset vector `$FFFC/$FFFD`

Host FPGA, Digilent cable, Vivado — **not** this `T`.

## Proven vs assumed (law layer)

See README / `artifacts/RESIDUAL.md`.

**Soft-witnessed (FIRST WITNESS):** reset, LDA/STA imm+zp, ADC/SBC±C, BCD 9+1, branches, JSR/RTS, stack, BRK vector — `artifacts/witness-first.*`.

**Soft-witnessed (Klaus Dormann NMOS functional):** official opcode×mode×flag behavior through suite sections `$00`–`$2B` (completion marker `$F0`, success trap `$3469`, cycles=96241388) — `artifacts/witness-klaus.*`. Functional correctness ≠ per-cycle φ bus geometry.

**Still assumed / residual:** 65C02 extras; undocumented illegals; RDY/SO; half-cycle bus; silicon; BCD N/V/Z die corners.
