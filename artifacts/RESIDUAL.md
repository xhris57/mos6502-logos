# residual(cpu.mos6502)

Named after FIRST WITNESS + Klaus Dormann soft run + undoc matrix. Soft≠Physical.

1. Soft≠Physical — no Digilent flash; no Φ1/Φ2 half-cycle claim.
2. Soft twin agreement ≠ silicon proof.
3. Visual6502 observation-cite only — not seeded; geometry not vendored.
4. Undocumented opcode matrix: stable composites matched soft (see artifacts/witness-undoc.*);
   UNCLAIMED residual: $8B XAA, $AB LAX#, $93/$9F AHX, $9C SHY, $9E SHX, $9B TAS, $BB LAS;
   ARR D=1 residual. Sources: artifacts/SOURCE-undoc.txt.
5. RDY/SO / exotic interrupt timing absent.
6. **65C02 extras** — `65C02_extended_opcodes_test` not run; no claim on Rockwell/WDC extras.
7. LAW cycle-table rows: Klaus witnesses **functional** correctness (result + flags + modes) for the official NMOS set through `test_case` $00–$2B → $F0 → success @$3469; per-cycle bus φ timing and exact Visual6502 half-cycle geometry remain **assumed**.
8. BCD N/V/Z corners — Klaus decimal tests document valid-BCD C primarily; N/V/Z vs every die residual (NMOS teaching rule).
9. Interrupt mid-instruction / rare NMI edge cases — suite traps unexpected IRQ/NMI but does not exhaustively witness every edge.

Minimum surgery after residual: extend witness rows, do not rewrite law to match a broken twin.
Do not invent silicon PASS for unstable illegals.
