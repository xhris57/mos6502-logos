// Package cpu is the mos6502-logos cycle twin: a teaching, instruction-cycle
// clock of the NMOS 6502 law (official opcodes, BCD, JMP ($xxFF) wrap).
//
// Reduced from xhris57/pi6502-go @ 3505eafe13ab90b579f12586d0ae7ad129ea9c4b
// (internal/cpu/). Soft twin only — Soft≠Physical. Not a silicon claim.
package cpu

const (
	FlagC uint8 = 0x01 // carry
	FlagZ uint8 = 0x02 // zero
	FlagI uint8 = 0x04 // IRQ disable
	FlagD uint8 = 0x08 // decimal (BCD ADC/SBC)
	FlagB uint8 = 0x10 // break (set in PHP/BRK image, not in P)
	FlagU uint8 = 0x20 // unused, always pushed as 1
	FlagV uint8 = 0x40 // overflow
	FlagN uint8 = 0x80 // negative
)

type Bus interface {
	Read(addr uint16) uint8
	Write(addr uint16, value uint8)
}

type CPU struct {
	A, X, Y, S, P uint8 // accumulator, index, stack pointer, status
	PC            uint16
	Cycles        uint64
	NMIEdge       bool // one-shot NMI, sampled before the next opcode
	IRQLine       bool // level-triggered IRQ, masked by FlagI
	Jammed        bool // KIL/JAM: PC frozen
	Bus           Bus
}

func (c *CPU) rd(a uint16) uint8    { return c.Bus.Read(a) }
func (c *CPU) wr(a uint16, v uint8) { c.Bus.Write(a, v) }
func (c *CPU) fetch() uint8 {
	v := c.rd(c.PC)
	c.PC++
	return v
}
func (c *CPU) rd16(a uint16) uint16 { // little-endian, no wrap
	return uint16(c.rd(a)) | uint16(c.rd(a+1))<<8
}
func (c *CPU) rd16Wrap(a uint16) uint16 {
	// NMOS: the high byte is read from the same page. JMP ($xxFF) therefore
	// takes the high byte from $xx00, not $xx00+100. Also used by (zp,X)/(zp),Y.
	a2 := (a & 0xFF00) | ((a + 1) & 0x00FF)
	return uint16(c.rd(a)) | uint16(c.rd(a2))<<8
}
func (c *CPU) push(v uint8) { // stack is $0100+S, then S--
	c.wr(0x0100|uint16(c.S), v)
	c.S--
}
func (c *CPU) pull() uint8 {
	c.S++
	return c.rd(0x0100 | uint16(c.S))
}
func (c *CPU) setZN(v uint8) {
	c.P = (c.P &^ (FlagZ | FlagN)) | (v & FlagN)
	if v == 0 {
		c.P |= FlagZ
	}
}
func (c *CPU) setC(on bool) {
	if on {
		c.P |= FlagC
	} else {
		c.P &^= FlagC
	}
}
func (c *CPU) setV(on bool) {
	if on {
		c.P |= FlagV
	} else {
		c.P &^= FlagV
	}
}

func (c *CPU) addrZP() uint16  { return uint16(c.fetch()) }                    // $00nn
func (c *CPU) addrZPX() uint16 { return uint16(uint8(c.fetch() + c.X)) }        // zp+X, wrap in page 0
func (c *CPU) addrZPY() uint16 { return uint16(uint8(c.fetch() + c.Y)) }        // zp+Y, wrap in page 0
func (c *CPU) addrAbs() uint16 {
	lo := c.fetch()
	return uint16(lo) | uint16(c.fetch())<<8
}
func (c *CPU) addrAbsX() (uint16, int) { // extra cycle if the index crosses a page
	base := c.addrAbs()
	a := base + uint16(c.X)
	cross := 0
	if (base^a)&0xFF00 != 0 {
		cross = 1
	}
	return a, cross
}
func (c *CPU) addrAbsY() (uint16, int) {
	base := c.addrAbs()
	a := base + uint16(c.Y)
	cross := 0
	if (base^a)&0xFF00 != 0 {
		cross = 1
	}
	return a, cross
}
func (c *CPU) addrIndX() uint16 { // (zp+X) — pointer wraps in zero page
	z := uint8(c.fetch() + c.X)
	return c.rd16Wrap(uint16(z))
}
func (c *CPU) addrIndY() (uint16, int) { // (zp),Y — pointer wraps; +1 cycle on page cross
	z := c.fetch()
	base := c.rd16Wrap(uint16(z))
	a := base + uint16(c.Y)
	cross := 0
	if (base^a)&0xFF00 != 0 {
		cross = 1
	}
	return a, cross
}

func (c *CPU) branch(take bool, cyc *uint) {
	off := int8(c.fetch()) // signed relative
	if !take {
		return
	}
	old := c.PC
	c.PC = uint16(int32(c.PC) + int32(off))
	*cyc++ // taken branch: +1
	if (old^c.PC)&0xFF00 != 0 {
		*cyc++ // page cross: +1 more
	}
}

func (c *CPU) cmp(r, v uint8) { // C set if r>=v; Z/N from r-v
	t := uint16(r) - uint16(v)
	c.setC(r >= v)
	c.setZN(uint8(t))
}

func (c *CPU) adc(v uint8) {
	a := c.A
	bin := uint16(a) + uint16(v) + uint16(c.P&FlagC)
	// V: signed overflow of the binary sum, even in decimal mode (NMOS).
	c.setV((^(a ^ v))&(a^uint8(bin))&0x80 != 0)
	if c.P&FlagD != 0 {
		// NMOS BCD: each nibble corrected if >9. Z/N from the BCD result.
		l := int(a&0x0F) + int(v&0x0F) + int(c.P&FlagC)
		h := int(a>>4) + int(v>>4)
		if l > 9 {
			l -= 10
			h++
		}
		if h > 9 {
			h -= 10
			c.setC(true)
		} else {
			c.setC(false)
		}
		c.A = uint8(((h & 0x0F) << 4) | (l & 0x0F))
		c.setZN(c.A)
		return
	}
	c.setC(bin > 0xFF)
	c.A = uint8(bin)
	c.setZN(c.A)
}

func (c *CPU) sbc(v uint8) {
	a := c.A
	borrow := uint16(0)
	if c.P&FlagC == 0 {
		borrow = 1
	}
	bin := uint16(a) - uint16(v) - borrow
	c.setV((a^v)&(a^uint8(bin))&0x80 != 0)
	if c.P&FlagD != 0 {
		l := int(a&0x0F) - int(v&0x0F) - int(borrow)
		h := int(a>>4) - int(v>>4)
		if l < 0 {
			l += 10
			h--
		}
		if h < 0 {
			h += 10
			c.setC(false)
		} else {
			c.setC(true)
		}
		c.A = uint8(((h & 0x0F) << 4) | (l & 0x0F))
		c.setZN(c.A)
		return
	}
	c.setC(bin < 0x100) // C set if no borrow
	c.A = uint8(bin)
	c.setZN(c.A)
}

func (c *CPU) asl(v uint8) uint8 { // C ← bit7, then <<1
	c.setC(v&0x80 != 0)
	v <<= 1
	c.setZN(v)
	return v
}
func (c *CPU) lsr(v uint8) uint8 { // C ← bit0, then >>1
	c.setC(v&0x01 != 0)
	v >>= 1
	c.setZN(v)
	return v
}
func (c *CPU) rol(v uint8) uint8 { // 9-bit rotate through C
	carry := c.P & FlagC
	c.setC(v&0x80 != 0)
	v = (v << 1) | carry
	c.setZN(v)
	return v
}
func (c *CPU) ror(v uint8) uint8 {
	carry := c.P & FlagC
	c.setC(v&0x01 != 0)
	v >>= 1
	if carry != 0 {
		v |= 0x80
	}
	c.setZN(v)
	return v
}
func (c *CPU) bitOp(v uint8) { // Z from A&M; N,V copied from M bits 7,6
	c.P = (c.P &^ (FlagZ | FlagV | FlagN)) | (v & (FlagV | FlagN))
	if c.A&v == 0 {
		c.P |= FlagZ
	}
}

func (c *CPU) doNMI() { // vector $FFFA; B clear in the pushed P
	c.push(uint8(c.PC >> 8))
	c.push(uint8(c.PC))
	c.push((c.P | FlagU) &^ FlagB)
	c.P |= FlagI
	c.PC = c.rd16(0xFFFA)
	c.Cycles += 7
}
func (c *CPU) doIRQ() { // vector $FFFE; B clear in the pushed P
	c.push(uint8(c.PC >> 8))
	c.push(uint8(c.PC))
	c.push((c.P | FlagU) &^ FlagB)
	c.P |= FlagI
	c.PC = c.rd16(0xFFFE)
	c.Cycles += 7
}

func (c *CPU) Reset() { // vector $FFFC; S -= 3 as on a real chip (stack not written)
	c.S -= 3
	c.P |= FlagI | FlagU
	c.P &^= FlagB
	c.PC = c.rd16(0xFFFC)
	c.Cycles += 7
	c.Jammed = false
}

func New(bus Bus) *CPU {
	return &CPU{S: 0xFD, P: FlagU | FlagI, Bus: bus}
}


// --- undocumented / illegal NMOS helpers (stable composites) ---
// Soft≠Physical. Sources: Graham oxyron matrix; masswerk illegal demystified;
// Visual6502 observation-cite only (not geometry). See artifacts/SOURCE-undoc.txt.

func (c *CPU) rmwSLO(a uint16) {
	t := c.asl(c.rd(a))
	c.wr(a, t)
	c.A |= t
	c.setZN(c.A)
}
func (c *CPU) rmwRLA(a uint16) {
	t := c.rol(c.rd(a))
	c.wr(a, t)
	c.A &= t
	c.setZN(c.A)
}
func (c *CPU) rmwSRE(a uint16) {
	t := c.lsr(c.rd(a))
	c.wr(a, t)
	c.A ^= t
	c.setZN(c.A)
}
func (c *CPU) rmwRRA(a uint16) {
	t := c.ror(c.rd(a))
	c.wr(a, t)
	c.adc(t)
}
func (c *CPU) rmwDCP(a uint16) {
	t := c.rd(a) - 1
	c.wr(a, t)
	c.cmp(c.A, t)
}
func (c *CPU) rmwISC(a uint16) {
	t := c.rd(a) + 1
	c.wr(a, t)
	c.sbc(t)
}
func (c *CPU) lax(v uint8) {
	c.A, c.X = v, v
	c.setZN(v)
}
func (c *CPU) sax(a uint16) { c.wr(a, c.A&c.X) }

func (c *CPU) anc(v uint8) {
	c.A &= v
	c.setZN(c.A)
	c.setC(c.A&0x80 != 0) // C ← N (ASL/ROL side-effect)
}
func (c *CPU) alr(v uint8) {
	c.A &= v
	c.A = c.lsr(c.A)
}
func (c *CPU) arr(v uint8) {
	// Binary-mode ARR (D=0): AND then ROR; C←bit6, V←bit6⊕bit5.
	// Decimal ARR residual (not claimed). Soft≠Physical.
	c.A &= v
	c.A = c.ror(c.A)
	c.setC(c.A&0x40 != 0)
	c.setV(((c.A>>6)^(c.A>>5))&1 != 0)
}
func (c *CPU) axs(v uint8) {
	// AXS/SBX: X := (A&X) - imm; flags like CMP (not SBC — C ignores prior C).
	t := uint16(c.A&c.X) - uint16(v)
	c.X = uint8(t)
	c.setC(t < 0x100)
	c.setZN(c.X)
}

func (c *CPU) Step() uint {
	if c.Jammed {
		return 0
	}
	if c.NMIEdge {
		c.NMIEdge = false
		before := c.Cycles
		c.doNMI()
		return uint(c.Cycles - before)
	}
	if c.IRQLine && c.P&FlagI == 0 {
		before := c.Cycles
		c.doIRQ()
		return uint(c.Cycles - before)
	}
	start := c.Cycles
	op := c.fetch()
	cyc := uint(2)
	switch op {
	case 0x02, 0x12, 0x22, 0x32, 0x42, 0x52, 0x62, 0x72, 0x92, 0xB2, 0xD2, 0xF2: // JAM / KIL  (halt)
		c.Jammed = true
		c.PC--
		cyc = 2
	case 0x00: // BRK  (push PC+1, P|B; PC=$FFFE)
		c.fetch()
		c.push(uint8(c.PC >> 8))
		c.push(uint8(c.PC))
		c.push(c.P | FlagB | FlagU)
		c.P |= FlagI
		c.PC = c.rd16(0xFFFE)
		cyc = 7
	case 0x09: // ORA #imm
		c.A |= c.fetch()
		c.setZN(c.A)
		cyc = 2
	case 0x05: // ORA zp
		c.A |= c.rd(c.addrZP())
		c.setZN(c.A)
		cyc = 3
	case 0x15: // ORA zp,X
		c.A |= c.rd(c.addrZPX())
		c.setZN(c.A)
		cyc = 4
	case 0x0D: // ORA abs
		c.A |= c.rd(c.addrAbs())
		c.setZN(c.A)
		cyc = 4
	case 0x1D: // ORA abs,X
		a, x := c.addrAbsX()
		c.A |= c.rd(a)
		c.setZN(c.A)
		cyc = 4 + uint(x)
	case 0x19: // ORA abs,Y
		a, x := c.addrAbsY()
		c.A |= c.rd(a)
		c.setZN(c.A)
		cyc = 4 + uint(x)
	case 0x01: // ORA (zp,X)
		c.A |= c.rd(c.addrIndX())
		c.setZN(c.A)
		cyc = 6
	case 0x11: // ORA (zp),Y
		a, x := c.addrIndY()
		c.A |= c.rd(a)
		c.setZN(c.A)
		cyc = 5 + uint(x)
	case 0x29: // AND #imm
		c.A &= c.fetch()
		c.setZN(c.A)
		cyc = 2
	case 0x25: // AND zp
		c.A &= c.rd(c.addrZP())
		c.setZN(c.A)
		cyc = 3
	case 0x35: // AND zp,X
		c.A &= c.rd(c.addrZPX())
		c.setZN(c.A)
		cyc = 4
	case 0x2D: // AND abs
		c.A &= c.rd(c.addrAbs())
		c.setZN(c.A)
		cyc = 4
	case 0x3D: // AND abs,X
		a, x := c.addrAbsX()
		c.A &= c.rd(a)
		c.setZN(c.A)
		cyc = 4 + uint(x)
	case 0x39: // AND abs,Y
		a, x := c.addrAbsY()
		c.A &= c.rd(a)
		c.setZN(c.A)
		cyc = 4 + uint(x)
	case 0x21: // AND (zp,X)
		c.A &= c.rd(c.addrIndX())
		c.setZN(c.A)
		cyc = 6
	case 0x31: // AND (zp),Y
		a, x := c.addrIndY()
		c.A &= c.rd(a)
		c.setZN(c.A)
		cyc = 5 + uint(x)
	case 0x49: // EOR #imm
		c.A ^= c.fetch()
		c.setZN(c.A)
		cyc = 2
	case 0x45: // EOR zp
		c.A ^= c.rd(c.addrZP())
		c.setZN(c.A)
		cyc = 3
	case 0x55: // EOR zp,X
		c.A ^= c.rd(c.addrZPX())
		c.setZN(c.A)
		cyc = 4
	case 0x4D: // EOR abs
		c.A ^= c.rd(c.addrAbs())
		c.setZN(c.A)
		cyc = 4
	case 0x5D: // EOR abs,X
		a, x := c.addrAbsX()
		c.A ^= c.rd(a)
		c.setZN(c.A)
		cyc = 4 + uint(x)
	case 0x59: // EOR abs,Y
		a, x := c.addrAbsY()
		c.A ^= c.rd(a)
		c.setZN(c.A)
		cyc = 4 + uint(x)
	case 0x41: // EOR (zp,X)
		c.A ^= c.rd(c.addrIndX())
		c.setZN(c.A)
		cyc = 6
	case 0x51: // EOR (zp),Y
		a, x := c.addrIndY()
		c.A ^= c.rd(a)
		c.setZN(c.A)
		cyc = 5 + uint(x)
	case 0x69: // ADC #imm
		c.adc(c.fetch())
		cyc = 2
	case 0x65: // ADC zp
		c.adc(c.rd(c.addrZP()))
		cyc = 3
	case 0x75: // ADC zp,X
		c.adc(c.rd(c.addrZPX()))
		cyc = 4
	case 0x6D: // ADC abs
		c.adc(c.rd(c.addrAbs()))
		cyc = 4
	case 0x7D: // ADC abs,X
		a, x := c.addrAbsX()
		c.adc(c.rd(a))
		cyc = 4 + uint(x)
	case 0x79: // ADC abs,Y
		a, x := c.addrAbsY()
		c.adc(c.rd(a))
		cyc = 4 + uint(x)
	case 0x61: // ADC (zp,X)
		c.adc(c.rd(c.addrIndX()))
		cyc = 6
	case 0x71: // ADC (zp),Y
		a, x := c.addrIndY()
		c.adc(c.rd(a))
		cyc = 5 + uint(x)
	case 0xE9, 0xEB: // SBC #imm
		c.sbc(c.fetch())
		cyc = 2
	case 0xE5: // SBC zp
		c.sbc(c.rd(c.addrZP()))
		cyc = 3
	case 0xF5: // SBC zp,X
		c.sbc(c.rd(c.addrZPX()))
		cyc = 4
	case 0xED: // SBC abs
		c.sbc(c.rd(c.addrAbs()))
		cyc = 4
	case 0xFD: // SBC abs,X
		a, x := c.addrAbsX()
		c.sbc(c.rd(a))
		cyc = 4 + uint(x)
	case 0xF9: // SBC abs,Y
		a, x := c.addrAbsY()
		c.sbc(c.rd(a))
		cyc = 4 + uint(x)
	case 0xE1: // SBC (zp,X)
		c.sbc(c.rd(c.addrIndX()))
		cyc = 6
	case 0xF1: // SBC (zp),Y
		a, x := c.addrIndY()
		c.sbc(c.rd(a))
		cyc = 5 + uint(x)
	case 0x85: // STA zp
		c.wr(c.addrZP(), c.A)
		cyc = 3
	case 0x95: // STA zp,X
		c.wr(c.addrZPX(), c.A)
		cyc = 4
	case 0x8D: // STA abs
		c.wr(c.addrAbs(), c.A)
		cyc = 4
	case 0x9D: // STA abs,X
		a, _ := c.addrAbsX()
		c.wr(a, c.A)
		cyc = 5
	case 0x99: // STA abs,Y
		a, _ := c.addrAbsY()
		c.wr(a, c.A)
		cyc = 5
	case 0x81: // STA (zp,X)
		c.wr(c.addrIndX(), c.A)
		cyc = 6
	case 0x91: // STA (zp),Y
		a, _ := c.addrIndY()
		c.wr(a, c.A)
		cyc = 6
	case 0x86: // STX zp
		c.wr(c.addrZP(), c.X)
		cyc = 3
	case 0x96: // STX zp,Y
		c.wr(c.addrZPY(), c.X)
		cyc = 4
	case 0x8E: // STX abs
		c.wr(c.addrAbs(), c.X)
		cyc = 4
	case 0x84: // STY zp
		c.wr(c.addrZP(), c.Y)
		cyc = 3
	case 0x94: // STY zp,X
		c.wr(c.addrZPX(), c.Y)
		cyc = 4
	case 0x8C: // STY abs
		c.wr(c.addrAbs(), c.Y)
		cyc = 4
	case 0xA9: // LDA #imm
		c.A = c.fetch()
		c.setZN(c.A)
		cyc = 2
	case 0xA5: // LDA zp
		c.A = c.rd(c.addrZP())
		c.setZN(c.A)
		cyc = 3
	case 0xB5: // LDA zp,X
		c.A = c.rd(c.addrZPX())
		c.setZN(c.A)
		cyc = 4
	case 0xAD: // LDA abs
		c.A = c.rd(c.addrAbs())
		c.setZN(c.A)
		cyc = 4
	case 0xBD: // LDA abs,X
		a, x := c.addrAbsX()
		c.A = c.rd(a)
		c.setZN(c.A)
		cyc = 4 + uint(x)
	case 0xB9: // LDA abs,Y
		a, x := c.addrAbsY()
		c.A = c.rd(a)
		c.setZN(c.A)
		cyc = 4 + uint(x)
	case 0xA1: // LDA (zp,X)
		c.A = c.rd(c.addrIndX())
		c.setZN(c.A)
		cyc = 6
	case 0xB1: // LDA (zp),Y
		a, x := c.addrIndY()
		c.A = c.rd(a)
		c.setZN(c.A)
		cyc = 5 + uint(x)
	case 0xA2: // LDX #imm
		c.X = c.fetch()
		c.setZN(c.X)
		cyc = 2
	case 0xA6: // LDX zp
		c.X = c.rd(c.addrZP())
		c.setZN(c.X)
		cyc = 3
	case 0xB6: // LDX zp,Y
		c.X = c.rd(c.addrZPY())
		c.setZN(c.X)
		cyc = 4
	case 0xAE: // LDX abs
		c.X = c.rd(c.addrAbs())
		c.setZN(c.X)
		cyc = 4
	case 0xBE: // LDX abs,Y
		a, x := c.addrAbsY()
		c.X = c.rd(a)
		c.setZN(c.X)
		cyc = 4 + uint(x)
	case 0xA0: // LDY #imm
		c.Y = c.fetch()
		c.setZN(c.Y)
		cyc = 2
	case 0xA4: // LDY zp
		c.Y = c.rd(c.addrZP())
		c.setZN(c.Y)
		cyc = 3
	case 0xB4: // LDY zp,X
		c.Y = c.rd(c.addrZPX())
		c.setZN(c.Y)
		cyc = 4
	case 0xAC: // LDY abs
		c.Y = c.rd(c.addrAbs())
		c.setZN(c.Y)
		cyc = 4
	case 0xBC: // LDY abs,X
		a, x := c.addrAbsX()
		c.Y = c.rd(a)
		c.setZN(c.Y)
		cyc = 4 + uint(x)
	case 0xC9: // CMP #imm
		c.cmp(c.A, c.fetch())
		cyc = 2
	case 0xC5: // CMP zp
		c.cmp(c.A, c.rd(c.addrZP()))
		cyc = 3
	case 0xD5: // CMP zp,X
		c.cmp(c.A, c.rd(c.addrZPX()))
		cyc = 4
	case 0xCD: // CMP abs
		c.cmp(c.A, c.rd(c.addrAbs()))
		cyc = 4
	case 0xDD: // CMP abs,X
		a, x := c.addrAbsX()
		c.cmp(c.A, c.rd(a))
		cyc = 4 + uint(x)
	case 0xD9: // CMP abs,Y
		a, x := c.addrAbsY()
		c.cmp(c.A, c.rd(a))
		cyc = 4 + uint(x)
	case 0xC1: // CMP (zp,X)
		c.cmp(c.A, c.rd(c.addrIndX()))
		cyc = 6
	case 0xD1: // CMP (zp),Y
		a, x := c.addrIndY()
		c.cmp(c.A, c.rd(a))
		cyc = 5 + uint(x)
	case 0xE0: // CPX #imm
		c.cmp(c.X, c.fetch())
		cyc = 2
	case 0xE4: // CPX zp
		c.cmp(c.X, c.rd(c.addrZP()))
		cyc = 3
	case 0xEC: // CPX abs
		c.cmp(c.X, c.rd(c.addrAbs()))
		cyc = 4
	case 0xC0: // CPY #imm
		c.cmp(c.Y, c.fetch())
		cyc = 2
	case 0xC4: // CPY zp
		c.cmp(c.Y, c.rd(c.addrZP()))
		cyc = 3
	case 0xCC: // CPY abs
		c.cmp(c.Y, c.rd(c.addrAbs()))
		cyc = 4
	case 0xC6: // DEC zp
		a := c.addrZP()
		t := c.rd(a) - 1
		c.wr(a, t)
		c.setZN(t)
		cyc = 5
	case 0xD6: // DEC zp,X
		a := c.addrZPX()
		t := c.rd(a) - 1
		c.wr(a, t)
		c.setZN(t)
		cyc = 6
	case 0xCE: // DEC abs
		a := c.addrAbs()
		t := c.rd(a) - 1
		c.wr(a, t)
		c.setZN(t)
		cyc = 6
	case 0xDE: // DEC abs,X
		a, _ := c.addrAbsX()
		t := c.rd(a) - 1
		c.wr(a, t)
		c.setZN(t)
		cyc = 7
	case 0xCA: // DEX
		c.X--
		c.setZN(c.X)
		cyc = 2
	case 0x88: // DEY
		c.Y--
		c.setZN(c.Y)
		cyc = 2
	case 0xE6: // INC zp
		a := c.addrZP()
		t := c.rd(a) + 1
		c.wr(a, t)
		c.setZN(t)
		cyc = 5
	case 0xF6: // INC zp,X
		a := c.addrZPX()
		t := c.rd(a) + 1
		c.wr(a, t)
		c.setZN(t)
		cyc = 6
	case 0xEE: // INC abs
		a := c.addrAbs()
		t := c.rd(a) + 1
		c.wr(a, t)
		c.setZN(t)
		cyc = 6
	case 0xFE: // INC abs,X
		a, _ := c.addrAbsX()
		t := c.rd(a) + 1
		c.wr(a, t)
		c.setZN(t)
		cyc = 7
	case 0xE8: // INX
		c.X++
		c.setZN(c.X)
		cyc = 2
	case 0xC8: // INY
		c.Y++
		c.setZN(c.Y)
		cyc = 2
	case 0x0A: // ASL A
		c.A = c.asl(c.A)
		cyc = 2
	case 0x06: // ASL zp
		a := c.addrZP()
		c.wr(a, c.asl(c.rd(a)))
		cyc = 5
	case 0x16: // ASL zp,X
		a := c.addrZPX()
		c.wr(a, c.asl(c.rd(a)))
		cyc = 6
	case 0x0E: // ASL abs
		a := c.addrAbs()
		c.wr(a, c.asl(c.rd(a)))
		cyc = 6
	case 0x1E: // ASL abs,X
		a, _ := c.addrAbsX()
		c.wr(a, c.asl(c.rd(a)))
		cyc = 7
	case 0x4A: // LSR A
		c.A = c.lsr(c.A)
		cyc = 2
	case 0x46: // LSR zp
		a := c.addrZP()
		c.wr(a, c.lsr(c.rd(a)))
		cyc = 5
	case 0x56: // LSR zp,X
		a := c.addrZPX()
		c.wr(a, c.lsr(c.rd(a)))
		cyc = 6
	case 0x4E: // LSR abs
		a := c.addrAbs()
		c.wr(a, c.lsr(c.rd(a)))
		cyc = 6
	case 0x5E: // LSR abs,X
		a, _ := c.addrAbsX()
		c.wr(a, c.lsr(c.rd(a)))
		cyc = 7
	case 0x2A: // ROL A
		c.A = c.rol(c.A)
		cyc = 2
	case 0x26: // ROL zp
		a := c.addrZP()
		c.wr(a, c.rol(c.rd(a)))
		cyc = 5
	case 0x36: // ROL zp,X
		a := c.addrZPX()
		c.wr(a, c.rol(c.rd(a)))
		cyc = 6
	case 0x2E: // ROL abs
		a := c.addrAbs()
		c.wr(a, c.rol(c.rd(a)))
		cyc = 6
	case 0x3E: // ROL abs,X
		a, _ := c.addrAbsX()
		c.wr(a, c.rol(c.rd(a)))
		cyc = 7
	case 0x6A: // ROR A
		c.A = c.ror(c.A)
		cyc = 2
	case 0x66: // ROR zp
		a := c.addrZP()
		c.wr(a, c.ror(c.rd(a)))
		cyc = 5
	case 0x76: // ROR zp,X
		a := c.addrZPX()
		c.wr(a, c.ror(c.rd(a)))
		cyc = 6
	case 0x6E: // ROR abs
		a := c.addrAbs()
		c.wr(a, c.ror(c.rd(a)))
		cyc = 6
	case 0x7E: // ROR abs,X
		a, _ := c.addrAbsX()
		c.wr(a, c.ror(c.rd(a)))
		cyc = 7
	case 0x24: // BIT zp
		c.bitOp(c.rd(c.addrZP()))
		cyc = 3
	case 0x2C: // BIT abs
		c.bitOp(c.rd(c.addrAbs()))
		cyc = 4
	case 0x10: // BPL rel
		c.branch(c.P&FlagN == 0, &cyc)
	case 0x30: // BMI rel
		c.branch(c.P&FlagN != 0, &cyc)
	case 0x50: // BVC rel
		c.branch(c.P&FlagV == 0, &cyc)
	case 0x70: // BVS rel
		c.branch(c.P&FlagV != 0, &cyc)
	case 0x90: // BCC rel
		c.branch(c.P&FlagC == 0, &cyc)
	case 0xB0: // BCS rel
		c.branch(c.P&FlagC != 0, &cyc)
	case 0xD0: // BNE rel
		c.branch(c.P&FlagZ == 0, &cyc)
	case 0xF0: // BEQ rel
		c.branch(c.P&FlagZ != 0, &cyc)
	case 0x4C: // JMP abs
		c.PC = c.addrAbs()
		cyc = 3
	case 0x6C: // JMP (abs)  NMOS: ($xxFF) wraps in-page
		a := c.addrAbs()
		c.PC = c.rd16Wrap(a)
		cyc = 5
	case 0x20: // JSR abs
		lo := c.fetch()
		c.push(uint8(c.PC >> 8))
		c.push(uint8(c.PC))
		c.PC = uint16(lo) | uint16(c.fetch())<<8
		cyc = 6
	case 0x60: // RTS
		t := c.pull()
		c.PC = uint16(t) | uint16(c.pull())<<8
		c.PC++
		cyc = 6
	case 0x40: // RTI
		c.P = (c.pull() | FlagU) &^ FlagB
		t := c.pull()
		c.PC = uint16(t) | uint16(c.pull())<<8
		cyc = 6
	case 0x48: // PHA
		c.push(c.A)
		cyc = 3
	case 0x68: // PLA
		c.A = c.pull()
		c.setZN(c.A)
		cyc = 4
	case 0x08: // PHP
		c.push(c.P | FlagB | FlagU)
		cyc = 3
	case 0x28: // PLP
		c.P = (c.pull() | FlagU) &^ FlagB
		cyc = 4
	case 0x18: // CLC
		c.P &^= FlagC
		cyc = 2
	case 0x38: // SEC
		c.P |= FlagC
		cyc = 2
	case 0x58: // CLI
		c.P &^= FlagI
		cyc = 2
	case 0x78: // SEI
		c.P |= FlagI
		cyc = 2
	case 0xD8: // CLD
		c.P &^= FlagD
		cyc = 2
	case 0xF8: // SED
		c.P |= FlagD
		cyc = 2
	case 0xB8: // CLV
		c.P &^= FlagV
		cyc = 2
	case 0xAA: // TAX
		c.X = c.A
		c.setZN(c.X)
		cyc = 2
	case 0xA8: // TAY
		c.Y = c.A
		c.setZN(c.Y)
		cyc = 2
	case 0x8A: // TXA
		c.A = c.X
		c.setZN(c.A)
		cyc = 2
	case 0x98: // TYA
		c.A = c.Y
		c.setZN(c.A)
		cyc = 2
	case 0x9A: // TXS
		c.S = c.X
		cyc = 2
	case 0xBA: // TSX
		c.X = c.S
		c.setZN(c.X)
		cyc = 2
	// ========== undocumented / illegal NMOS (stable composites) ==========
	// Twin previously NOP/JAM or default-stubbed many of these. Soft≠Physical.

	// --- SLO / ASO = ASL + ORA ---
	case 0x07: // SLO zp
		c.rmwSLO(c.addrZP()); cyc = 5
	case 0x17: // SLO zp,X
		c.rmwSLO(c.addrZPX()); cyc = 6
	case 0x0F: // SLO abs
		c.rmwSLO(c.addrAbs()); cyc = 6
	case 0x1F: // SLO abs,X
		a, _ := c.addrAbsX(); c.rmwSLO(a); cyc = 7
	case 0x1B: // SLO abs,Y
		a, _ := c.addrAbsY(); c.rmwSLO(a); cyc = 7
	case 0x03: // SLO (zp,X)
		c.rmwSLO(c.addrIndX()); cyc = 8
	case 0x13: // SLO (zp),Y
		a, _ := c.addrIndY(); c.rmwSLO(a); cyc = 8

	// --- RLA = ROL + AND ---
	case 0x27:
		c.rmwRLA(c.addrZP()); cyc = 5
	case 0x37:
		c.rmwRLA(c.addrZPX()); cyc = 6
	case 0x2F:
		c.rmwRLA(c.addrAbs()); cyc = 6
	case 0x3F:
		a, _ := c.addrAbsX(); c.rmwRLA(a); cyc = 7
	case 0x3B:
		a, _ := c.addrAbsY(); c.rmwRLA(a); cyc = 7
	case 0x23:
		c.rmwRLA(c.addrIndX()); cyc = 8
	case 0x33:
		a, _ := c.addrIndY(); c.rmwRLA(a); cyc = 8

	// --- SRE / LSE = LSR + EOR ---
	case 0x47:
		c.rmwSRE(c.addrZP()); cyc = 5
	case 0x57:
		c.rmwSRE(c.addrZPX()); cyc = 6
	case 0x4F:
		c.rmwSRE(c.addrAbs()); cyc = 6
	case 0x5F:
		a, _ := c.addrAbsX(); c.rmwSRE(a); cyc = 7
	case 0x5B:
		a, _ := c.addrAbsY(); c.rmwSRE(a); cyc = 7
	case 0x43:
		c.rmwSRE(c.addrIndX()); cyc = 8
	case 0x53:
		a, _ := c.addrIndY(); c.rmwSRE(a); cyc = 8

	// --- RRA = ROR + ADC ---
	case 0x67:
		c.rmwRRA(c.addrZP()); cyc = 5
	case 0x77:
		c.rmwRRA(c.addrZPX()); cyc = 6
	case 0x6F:
		c.rmwRRA(c.addrAbs()); cyc = 6
	case 0x7F:
		a, _ := c.addrAbsX(); c.rmwRRA(a); cyc = 7
	case 0x7B:
		a, _ := c.addrAbsY(); c.rmwRRA(a); cyc = 7
	case 0x63:
		c.rmwRRA(c.addrIndX()); cyc = 8
	case 0x73:
		a, _ := c.addrIndY(); c.rmwRRA(a); cyc = 8

	// --- SAX / AXS / AAX = store A&X ---
	case 0x87: // SAX zp
		c.sax(c.addrZP()); cyc = 3
	case 0x97: // SAX zp,Y
		c.sax(c.addrZPY()); cyc = 4
	case 0x8F: // SAX abs
		c.sax(c.addrAbs()); cyc = 4
	case 0x83: // SAX (zp,X)
		c.sax(c.addrIndX()); cyc = 6

	// --- LAX = LDA + LDX (non-immediate; imm $AB is unstable) ---
	case 0xA7: // LAX zp
		c.lax(c.rd(c.addrZP())); cyc = 3
	case 0xB7: // LAX zp,Y
		c.lax(c.rd(c.addrZPY())); cyc = 4
	case 0xAF: // LAX abs
		c.lax(c.rd(c.addrAbs())); cyc = 4
	case 0xBF: // LAX abs,Y
		a, x := c.addrAbsY(); c.lax(c.rd(a)); cyc = 4 + uint(x)
	case 0xA3: // LAX (zp,X)
		c.lax(c.rd(c.addrIndX())); cyc = 6
	case 0xB3: // LAX (zp),Y
		a, x := c.addrIndY(); c.lax(c.rd(a)); cyc = 5 + uint(x)

	// --- DCP / DCM = DEC + CMP ---
	case 0xC7:
		c.rmwDCP(c.addrZP()); cyc = 5
	case 0xD7:
		c.rmwDCP(c.addrZPX()); cyc = 6
	case 0xCF:
		c.rmwDCP(c.addrAbs()); cyc = 6
	case 0xDF:
		a, _ := c.addrAbsX(); c.rmwDCP(a); cyc = 7
	case 0xDB:
		a, _ := c.addrAbsY(); c.rmwDCP(a); cyc = 7
	case 0xC3:
		c.rmwDCP(c.addrIndX()); cyc = 8
	case 0xD3:
		a, _ := c.addrIndY(); c.rmwDCP(a); cyc = 8

	// --- ISC / ISB / INS = INC + SBC ---
	case 0xE7:
		c.rmwISC(c.addrZP()); cyc = 5
	case 0xF7:
		c.rmwISC(c.addrZPX()); cyc = 6
	case 0xEF:
		c.rmwISC(c.addrAbs()); cyc = 6
	case 0xFF:
		a, _ := c.addrAbsX(); c.rmwISC(a); cyc = 7
	case 0xFB:
		a, _ := c.addrAbsY(); c.rmwISC(a); cyc = 7
	case 0xE3:
		c.rmwISC(c.addrIndX()); cyc = 8
	case 0xF3:
		a, _ := c.addrIndY(); c.rmwISC(a); cyc = 8

	// --- immediate composites (stable) ---
	case 0x0B, 0x2B: // ANC #imm
		c.anc(c.fetch()); cyc = 2
	case 0x4B: // ALR #imm
		c.alr(c.fetch()); cyc = 2
	case 0x6B: // ARR #imm (binary D=0 claimed)
		c.arr(c.fetch()); cyc = 2
	case 0xCB: // AXS/SBX #imm — deterministic; Graham ≠ highly-unstable
		c.axs(c.fetch()); cyc = 2
	// $EB USBC already aliased with SBC #imm above.

	// UNCLAIMED unstable (no silicon-deterministic model claimed here):
	// $8B XAA/ANE, $AB LAX#, $93/$9F AHX, $9C SHY, $9E SHX, $9B TAS, $BB LAS
	// fall through to default stub (operand not invented). Soft≠Physical.

	case 0xEA: // NOP
		cyc = 2
	case 0x1A, 0x3A, 0x5A, 0x7A, 0xDA, 0xFA: // NOP  (undocumented)
		cyc = 2
	case 0x80, 0x82, 0x89, 0xC2, 0xE2: // NOP #imm  (undocumented)
		c.fetch()
		cyc = 2
	case 0x04, 0x44, 0x64:
		c.addrZP()
		cyc = 3
	case 0x14, 0x34, 0x54, 0x74, 0xD4, 0xF4: // NOP zp,X  (undocumented)
		c.addrZPX()
		cyc = 4
	case 0x0C: // NOP abs  (undocumented)
		c.addrAbs()
		cyc = 4
	case 0x1C, 0x3C, 0x5C, 0x7C, 0xDC, 0xFC: // NOP abs,X  (undocumented)
		_, x := c.addrAbsX()
		cyc = 4 + uint(x)
	default:
		// residual stub: unstable or unclaimed illegal; Soft≠Physical
		cyc = 2
	}
	c.Cycles += uint64(cyc)
	return uint(c.Cycles - start)
}

func (c *CPU) Run(budget uint) {
	used := uint(0)
	for used < budget && !c.Jammed {
		used += c.Step()
	}
}
