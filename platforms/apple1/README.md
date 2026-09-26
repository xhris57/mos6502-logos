# Soft Apple-1-ish platform (WOZMON)

**Soft≠Physical.** Soft twin only. No Digilent flash. No silicon claim.
No real 6820 PIA timing, no NTSC video, no physical Apple-1.

Minimal Soft platform for the mos6502-logos cycle twin:

- NMOS 6502 Soft twin (`twin/cpu`)
- 8 KiB Soft RAM (`$0000–$1FFF`) + zero-filled remainder
- Soft PIA-ish keyboard/display stubs at `$D010–$D013`
- Classic WOZMON ROM at `$FF00` (`wozmon.bin`)

## Provenance

See `SOURCE.txt` (URLs, SHA256 `e5af0d1c…`, license note).

Assemble (optional; must match `wozmon.bin`):

```bash
python3 tools/asm.py programs/wozmon/wozmon.asm 0xFF00
```

## Witness

```bash
make witness-wozmon
cat artifacts/witness-wozmon.txt
```

Soft-scripted keyboard injects examine/deposit hex; Soft console + cycle
receipt land in `artifacts/witness-wozmon.{txt,json}`.

## Residual (named)

- Soft≠Physical — no Φ1/Φ2, no real PIA handshake latency
- Display always-ready Soft stub (no DA bit delay)
- Keyboard is a Soft byte queue, not scanned matrix
- Not a claim about any physical Apple-1 or replica board
