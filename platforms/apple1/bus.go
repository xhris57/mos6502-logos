// Package apple1 is a Soft Apple-1-ish bus for the mos6502-logos cycle twin.
// Soft≠Physical. No Digilent flash. No silicon / PIA-timing / NTSC claim.
package apple1

import "strings"

const (
	KBD   = uint16(0xD010)
	KBDCR = uint16(0xD011)
	DSP   = uint16(0xD012)
	DSPCR = uint16(0xD013)

	ROMBase = uint16(0xFF00)
	RAMSize = 0x2000 // 8 KiB Soft RAM $0000–$1FFF
)

// Bus is Soft RAM + Soft PIA-ish KBD/DSP stubs + WOZMON ROM window.
type Bus struct {
	RAM [65536]byte
	ROM [256]byte // mirrored at $FF00; writes ignored

	kbdQ         []byte // Soft keyboard queue (Apple-1: bit7 set)
	dspCR        byte
	kbdCR        byte // Soft: bit7 set when key ready
	dspReg       byte // Soft: bit7 clear = display ready (always Soft-ready)
	softDataMode bool // Soft: ignore DSP echo until DSPCR written (DDR residual)

	Console strings.Builder // Soft TTY capture (ASCII, bit7 stripped)
}

func New(rom []byte) *Bus {
	b := &Bus{}
	if len(rom) > 256 {
		rom = rom[:256]
	}
	copy(b.ROM[:], rom)
	copy(b.RAM[ROMBase:], b.ROM[:])
	// Soft display ready: bit7 clear
	b.dspReg = 0x00
	return b
}

// LoadROM installs 256-byte WOZMON at $FF00 and refreshes the Soft mirror.
func (b *Bus) LoadROM(rom []byte) {
	copy(b.ROM[:], rom)
	copy(b.RAM[ROMBase:], b.ROM[:])
}

// InjectKeys enqueues Soft keyboard bytes. Hex/ASCII input is OR'd with $80
// (Apple-1 keyboard sense). CR may be given as '\r' or $8D.
func (b *Bus) InjectKeys(s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\n':
			c = 0x0D
		}
		b.kbdQ = append(b.kbdQ, c|0x80)
	}
	b.refreshKBDCR()
}

func (b *Bus) refreshKBDCR() {
	if len(b.kbdQ) > 0 {
		b.kbdCR = 0x80
	} else {
		b.kbdCR = 0x00
	}
}

func (b *Bus) Read(addr uint16) uint8 {
	switch addr {
	case KBD:
		if len(b.kbdQ) == 0 {
			return 0x00
		}
		v := b.kbdQ[0]
		b.kbdQ = b.kbdQ[1:]
		b.refreshKBDCR()
		return v
	case KBDCR:
		b.refreshKBDCR()
		return b.kbdCR
	case DSP:
		// Soft always ready (bit7 clear). Residual: no real DA timing.
		return b.dspReg & 0x7F
	case DSPCR:
		return b.dspCR
	}
	if addr >= ROMBase {
		return b.ROM[addr-ROMBase]
	}
	return b.RAM[addr]
}

func (b *Bus) Write(addr uint16, value uint8) {
	switch addr {
	case KBD:
		return // Soft: keyboard is input-only
	case KBDCR:
		b.kbdCR = value
		return
	case DSP:
		b.dspReg = value & 0x7F // Soft: drop DA; always ready next read
		if !b.softDataMode {
			return // Soft residual: DDR-phase write (WOZMON STY #$7F) not echoed
		}
		ch := value & 0x7F
		switch ch {
		case 0x0D: // CR
			b.Console.WriteByte('\n')
		case 0x00:
			// ignore NUL
		default:
			if ch >= 0x20 && ch < 0x7F {
				b.Console.WriteByte(ch)
			} else {
				// Soft: keep other control as hex escape for receipts
				b.Console.WriteByte('?')
			}
		}
		return
	case DSPCR:
		b.dspCR = value
		b.softDataMode = true // Soft: treat as leaving DDR; residual ≠ real 6820
		return
	}
	if addr >= ROMBase {
		return // Soft ROM window: ignore writes
	}
	if int(addr) < RAMSize {
		b.RAM[addr] = value
		return
	}
	// Soft: allow writes elsewhere for deposit/examine demos (open bus Soft)
	b.RAM[addr] = value
}

// KBDQueue returns a copy of the Soft keyboard pending bytes (Apple-1 bit7 form).
func (b *Bus) KBDQueue() []byte {
	out := make([]byte, len(b.kbdQ))
	copy(out, b.kbdQ)
	return out
}

// ConsoleASCII returns Soft TTY text captured from DSP writes.
func (b *Bus) ConsoleASCII() string { return b.Console.String() }
