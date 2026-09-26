// UNDOC / illegal NMOS 6502 matrix witness — Soft≠Physical. No Digilent flash.
// Compares twin behavior to published silicon-behavior tables (Graham oxyron,
// masswerk demystified, Visual6502 observation-cite only — no geometry vendor).
package tb_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/xhris57/mos6502-logos/twin/cpu"
)

type undocRow struct {
	Op       uint8  `json:"op"`
	Family   string `json:"family"`
	Mode     string `json:"mode"`
	Status   string `json:"status"` // MATCH | DIFF | UNCLAIMED
	TwinNote string `json:"twin_note"`
	Expect   string `json:"expect"`
	Cycles   uint   `json:"cycles_got"`
	CyclesOK bool   `json:"cycles_ok"`
	Detail   string `json:"detail,omitempty"`
}

type undocReceipt struct {
	Suite      string     `json:"suite"`
	Result     string     `json:"result"`
	Matched    int        `json:"matched"`
	Differed   int        `json:"differed"`
	Unclaimed  int        `json:"unclaimed"`
	Rows       int        `json:"rows"`
	Families   map[string]familyCount `json:"families"`
	Notes      []string   `json:"notes"`
	SoftOnly   bool       `json:"soft_only"`
	Matrix     []undocRow `json:"matrix"`
}

type familyCount struct {
	Match     int `json:"match"`
	Diff      int `json:"diff"`
	Unclaimed int `json:"unclaimed"`
}

func undocMem() (*mem, *cpu.CPU) {
	var m mem
	m[0xFFFC] = 0x00
	m[0xFFFD] = 0x80
	c := cpu.New(&m)
	c.Reset()
	return &m, c
}

func runOp(m *mem, c *cpu.CPU, bytes []byte, a, x, y, p uint8) uint {
	c.A, c.X, c.Y, c.P = a, x, y, p|cpu.FlagU
	c.PC = 0x8000
	copy(m[0x8000:], bytes)
	before := c.Cycles
	c.Step()
	return uint(c.Cycles - before)
}

func TestUndocMatrix(t *testing.T) {
	var rows []undocRow
	fam := map[string]*familyCount{}
	bump := func(family, status string) {
		fc := fam[family]
		if fc == nil {
			fc = &familyCount{}
			fam[family] = fc
		}
		switch status {
		case "MATCH":
			fc.Match++
		case "DIFF":
			fc.Diff++
		default:
			fc.Unclaimed++
		}
	}
	add := func(r undocRow) {
		rows = append(rows, r)
		bump(r.Family, r.Status)
		if r.Status == "DIFF" {
			t.Errorf("DIFF $%02X %s %s: %s", r.Op, r.Family, r.Mode, r.Detail)
		}
	}
	match := func(op uint8, family, mode, expect string, cyc uint, cycWant uint, ok bool, detail string) {
		st := "MATCH"
		if !ok {
			st = "DIFF"
		}
		add(undocRow{Op: op, Family: family, Mode: mode, Status: st, Expect: expect,
			Cycles: cyc, CyclesOK: cyc == cycWant, Detail: detail,
			TwinNote: "implemented composite"})
	}
	unclaimed := func(op uint8, family, mode, why string) {
		add(undocRow{Op: op, Family: family, Mode: mode, Status: "UNCLAIMED",
			Expect: why, TwinNote: "residual — no deterministic silicon model claimed"})
	}

	// ===== NOP family (already present; verify) =====
	{
		m, c := undocMem()
		cyc := runOp(m, c, []byte{0x1A}, 0, 0, 0, 0)
		match(0x1A, "NOP", "impl", "2-cycle NOP", cyc, 2, cyc == 2 && c.PC == 0x8001, fmt.Sprintf("cyc=%d PC=%04X", cyc, c.PC))
	}
	for _, op := range []uint8{0x3A, 0x5A, 0x7A, 0xDA, 0xFA} {
		m, c := undocMem()
		cyc := runOp(m, c, []byte{op}, 0, 0, 0, 0)
		match(op, "NOP", "impl", "2-cycle NOP", cyc, 2, cyc == 2, fmt.Sprintf("cyc=%d", cyc))
	}
	for _, op := range []uint8{0x80, 0x82, 0x89, 0xC2, 0xE2} {
		m, c := undocMem()
		cyc := runOp(m, c, []byte{op, 0x5A}, 0, 0, 0, 0)
		match(op, "NOP", "imm", "2-cycle NOP #imm", cyc, 2, cyc == 2 && c.PC == 0x8002, fmt.Sprintf("cyc=%d PC=%04X", cyc, c.PC))
	}
	for _, op := range []uint8{0x04, 0x44, 0x64} {
		m, c := undocMem()
		cyc := runOp(m, c, []byte{op, 0x10}, 0, 0, 0, 0)
		match(op, "NOP", "zp", "3-cycle NOP zp", cyc, 3, cyc == 3 && c.PC == 0x8002, fmt.Sprintf("cyc=%d", cyc))
	}
	for _, op := range []uint8{0x14, 0x34, 0x54, 0x74, 0xD4, 0xF4} {
		m, c := undocMem()
		cyc := runOp(m, c, []byte{op, 0x10}, 0, 0, 0, 0)
		match(op, "NOP", "zpx", "4-cycle NOP zp,X", cyc, 4, cyc == 4, fmt.Sprintf("cyc=%d", cyc))
	}
	{
		m, c := undocMem()
		cyc := runOp(m, c, []byte{0x0C, 0x00, 0x10}, 0, 0, 0, 0)
		match(0x0C, "NOP", "abs", "4-cycle NOP abs", cyc, 4, cyc == 4, fmt.Sprintf("cyc=%d", cyc))
	}
	for _, op := range []uint8{0x1C, 0x3C, 0x5C, 0x7C, 0xDC, 0xFC} {
		m, c := undocMem()
		cyc := runOp(m, c, []byte{op, 0x00, 0x10}, 0, 0, 0, 0) // no page cross
		match(op, "NOP", "abx", "4(+1) NOP abs,X", cyc, 4, cyc == 4, fmt.Sprintf("cyc=%d", cyc))
	}

	// ===== JAM / KIL =====
	for _, op := range []uint8{0x02, 0x12, 0x22, 0x32, 0x42, 0x52, 0x62, 0x72, 0x92, 0xB2, 0xD2, 0xF2} {
		m, c := undocMem()
		_ = runOp(m, c, []byte{op}, 0, 0, 0, 0)
		pc1 := c.PC
		c.Step()
		pc2 := c.PC
		ok := c.Jammed && pc1 == pc2
		match(op, "JAM", "impl", "halt until reset", 0, 0, ok, fmt.Sprintf("halt=%v PC=%04X/%04X", c.Jammed, pc1, pc2))
	}

	// ===== LAX (non-imm) =====
	{
		m, c := undocMem()
		m[0x10] = 0xC3
		cyc := runOp(m, c, []byte{0xA7, 0x10}, 0, 0, 0, 0)
		ok := c.A == 0xC3 && c.X == 0xC3 && c.P&cpu.FlagN != 0 && cyc == 3
		match(0xA7, "LAX", "zp", "A,X←M NZ", cyc, 3, ok, fmt.Sprintf("A=%02X X=%02X P=%02X cyc=%d", c.A, c.X, c.P, cyc))
	}
	{
		m, c := undocMem()
		m[0x15] = 0x00
		cyc := runOp(m, c, []byte{0xB7, 0x10}, 0, 0, 0x05, 0)
		ok := c.A == 0 && c.X == 0 && c.P&cpu.FlagZ != 0 && cyc == 4
		match(0xB7, "LAX", "zpy", "A,X←M NZ", cyc, 4, ok, fmt.Sprintf("A=%02X X=%02X cyc=%d", c.A, c.X, cyc))
	}
	{
		m, c := undocMem()
		m[0x1234] = 0x7E
		cyc := runOp(m, c, []byte{0xAF, 0x34, 0x12}, 0, 0, 0, 0)
		ok := c.A == 0x7E && c.X == 0x7E && cyc == 4
		match(0xAF, "LAX", "abs", "A,X←M", cyc, 4, ok, fmt.Sprintf("A=%02X cyc=%d", c.A, cyc))
	}
	{
		m, c := undocMem()
		m[0x1234] = 0x11
		cyc := runOp(m, c, []byte{0xBF, 0x30, 0x12}, 0, 0, 0x04, 0) // abs,Y no cross
		ok := c.A == 0x11 && c.X == 0x11 && cyc == 4
		match(0xBF, "LAX", "aby", "A,X←M", cyc, 4, ok, fmt.Sprintf("A=%02X cyc=%d", c.A, cyc))
	}
	{
		m, c := undocMem()
		m[0x20] = 0x00
		m[0x21] = 0x20
		m[0x2000] = 0x55
		cyc := runOp(m, c, []byte{0xA3, 0x20}, 0, 0, 0, 0) // (zp,X) X=0
		ok := c.A == 0x55 && c.X == 0x55 && cyc == 6
		match(0xA3, "LAX", "izx", "A,X←M", cyc, 6, ok, fmt.Sprintf("A=%02X cyc=%d", c.A, cyc))
	}
	{
		m, c := undocMem()
		m[0x20] = 0x00
		m[0x21] = 0x20
		m[0x2005] = 0x66
		cyc := runOp(m, c, []byte{0xB3, 0x20}, 0, 0, 0x05, 0)
		ok := c.A == 0x66 && c.X == 0x66 && cyc == 5
		match(0xB3, "LAX", "izy", "A,X←M", cyc, 5, ok, fmt.Sprintf("A=%02X cyc=%d", c.A, cyc))
	}

	// ===== SAX =====
	{
		m, c := undocMem()
		cyc := runOp(m, c, []byte{0x87, 0x10}, 0xF0, 0x0F, 0, 0)
		ok := m[0x10] == 0x00 && cyc == 3
		match(0x87, "SAX", "zp", "M←A&X", cyc, 3, ok, fmt.Sprintf("M=%02X cyc=%d", m[0x10], cyc))
	}
	{
		m, c := undocMem()
		cyc := runOp(m, c, []byte{0x97, 0x10}, 0xAA, 0x0F, 0x05, 0)
		ok := m[0x15] == 0x0A && cyc == 4
		match(0x97, "SAX", "zpy", "M←A&X", cyc, 4, ok, fmt.Sprintf("M=%02X cyc=%d", m[0x15], cyc))
	}
	{
		m, c := undocMem()
		cyc := runOp(m, c, []byte{0x8F, 0x00, 0x20}, 0xF0, 0x33, 0, 0)
		ok := m[0x2000] == 0x30 && cyc == 4
		match(0x8F, "SAX", "abs", "M←A&X", cyc, 4, ok, fmt.Sprintf("M=%02X cyc=%d", m[0x2000], cyc))
	}
	{
		m, c := undocMem()
		m[0x24] = 0x00
		m[0x25] = 0x30
		cyc := runOp(m, c, []byte{0x83, 0x20}, 0xFF, 0x04, 0, 0) // X=4 → ptr $24
		ok := m[0x3000] == 0x04 && cyc == 6
		match(0x83, "SAX", "izx", "M←A&X", cyc, 6, ok, fmt.Sprintf("M=%02X cyc=%d", m[0x3000], cyc))
	}

	// ===== DCP =====
	{
		m, c := undocMem()
		m[0x10] = 0x05
		cyc := runOp(m, c, []byte{0xC7, 0x10}, 0x04, 0, 0, 0)
		// DEC → 04; CMP A=04 → Z=1 C=1
		ok := m[0x10] == 0x04 && c.P&cpu.FlagZ != 0 && c.P&cpu.FlagC != 0 && cyc == 5
		match(0xC7, "DCP", "zp", "DEC+CMP", cyc, 5, ok, fmt.Sprintf("M=%02X P=%02X cyc=%d", m[0x10], c.P, cyc))
	}
	{
		m, c := undocMem()
		m[0x2000] = 0x10
		cyc := runOp(m, c, []byte{0xCF, 0x00, 0x20}, 0x0F, 0, 0, 0)
		ok := m[0x2000] == 0x0F && c.P&cpu.FlagZ != 0 && cyc == 6
		match(0xCF, "DCP", "abs", "DEC+CMP", cyc, 6, ok, fmt.Sprintf("M=%02X P=%02X", m[0x2000], c.P))
	}
	{
		m, c := undocMem()
		m[0x15] = 0x02
		cyc := runOp(m, c, []byte{0xD7, 0x10}, 0x01, 0x05, 0, 0)
		ok := m[0x15] == 0x01 && c.P&cpu.FlagZ != 0 && cyc == 6
		match(0xD7, "DCP", "zpx", "DEC+CMP", cyc, 6, ok, fmt.Sprintf("M=%02X cyc=%d", m[0x15], cyc))
	}
	{
		m, c := undocMem()
		m[0x2010] = 0x08
		cyc := runOp(m, c, []byte{0xDF, 0x00, 0x20}, 0x07, 0x10, 0, 0)
		ok := m[0x2010] == 0x07 && cyc == 7
		match(0xDF, "DCP", "abx", "DEC+CMP 7c", cyc, 7, ok, fmt.Sprintf("M=%02X cyc=%d", m[0x2010], cyc))
	}
	{
		m, c := undocMem()
		m[0x2010] = 0x08
		cyc := runOp(m, c, []byte{0xDB, 0x00, 0x20}, 0x07, 0, 0x10, 0)
		ok := m[0x2010] == 0x07 && cyc == 7
		match(0xDB, "DCP", "aby", "DEC+CMP 7c", cyc, 7, ok, fmt.Sprintf("M=%02X cyc=%d", m[0x2010], cyc))
	}
	{
		m, c := undocMem()
		m[0x20] = 0x00
		m[0x21] = 0x40
		m[0x4000] = 0x03
		cyc := runOp(m, c, []byte{0xC3, 0x20}, 0x02, 0, 0, 0)
		ok := m[0x4000] == 0x02 && cyc == 8
		match(0xC3, "DCP", "izx", "DEC+CMP 8c", cyc, 8, ok, fmt.Sprintf("M=%02X cyc=%d", m[0x4000], cyc))
	}
	{
		m, c := undocMem()
		m[0x20] = 0x00
		m[0x21] = 0x40
		m[0x4003] = 0x03
		cyc := runOp(m, c, []byte{0xD3, 0x20}, 0x02, 0, 0x03, 0)
		ok := m[0x4003] == 0x02 && cyc == 8
		match(0xD3, "DCP", "izy", "DEC+CMP 8c", cyc, 8, ok, fmt.Sprintf("M=%02X cyc=%d", m[0x4003], cyc))
	}

	// ===== ISC =====
	{
		m, c := undocMem()
		m[0x10] = 0x01
		// SEC; then ISC: INC→02; SBC 02 from A=05 → 03, C=1
		cyc := runOp(m, c, []byte{0xE7, 0x10}, 0x05, 0, 0, cpu.FlagC)
		ok := m[0x10] == 0x02 && c.A == 0x03 && c.P&cpu.FlagC != 0 && cyc == 5
		match(0xE7, "ISC", "zp", "INC+SBC", cyc, 5, ok, fmt.Sprintf("M=%02X A=%02X P=%02X cyc=%d", m[0x10], c.A, c.P, cyc))
	}
	{
		m, c := undocMem()
		m[0x2000] = 0x00
		cyc := runOp(m, c, []byte{0xEF, 0x00, 0x20}, 0x10, 0, 0, cpu.FlagC)
		ok := m[0x2000] == 0x01 && c.A == 0x0F && cyc == 6
		match(0xEF, "ISC", "abs", "INC+SBC", cyc, 6, ok, fmt.Sprintf("M=%02X A=%02X cyc=%d", m[0x2000], c.A, cyc))
	}
	for _, tc := range []struct {
		op uint8
		mode string
		setup func(*mem, *cpu.CPU) []byte
		wantCyc uint
	}{
		{0xF7, "zpx", func(m *mem, c *cpu.CPU) []byte { m[0x15] = 0; return []byte{0xF7, 0x10} }, 6},
		{0xFF, "abx", func(m *mem, c *cpu.CPU) []byte { m[0x2010] = 0; return []byte{0xFF, 0x00, 0x20} }, 7},
		{0xFB, "aby", func(m *mem, c *cpu.CPU) []byte { m[0x2010] = 0; return []byte{0xFB, 0x00, 0x20} }, 7},
		{0xE3, "izx", func(m *mem, c *cpu.CPU) []byte { m[0x20]=0; m[0x21]=0x50; m[0x5000]=0; return []byte{0xE3, 0x20} }, 8},
		{0xF3, "izy", func(m *mem, c *cpu.CPU) []byte { m[0x20]=0; m[0x21]=0x50; m[0x5002]=0; return []byte{0xF3, 0x20} }, 8},
	} {
		m, c := undocMem()
		bytes := tc.setup(m, c)
		x, y := uint8(0x05), uint8(0x02)
		if tc.mode == "abx" || tc.mode == "zpx" {
			x = 0x10
			y = 0
		}
		if tc.mode == "aby" {
			x = 0
			y = 0x10
		}
		if tc.mode == "izx" {
			x, y = 0, 0
		}
		cyc := runOp(m, c, bytes, 0x20, x, y, cpu.FlagC)
		ok := cyc == tc.wantCyc && c.A == 0x1F // INC 0→1; 0x20-1=0x1F
		match(tc.op, "ISC", tc.mode, "INC+SBC", cyc, tc.wantCyc, ok, fmt.Sprintf("A=%02X cyc=%d", c.A, cyc))
	}

	// ===== SLO =====
	{
		m, c := undocMem()
		m[0x10] = 0x80
		cyc := runOp(m, c, []byte{0x07, 0x10}, 0x01, 0, 0, 0)
		// ASL 80→00 C=1; ORA 00 → A=01
		ok := m[0x10] == 0x00 && c.A == 0x01 && c.P&cpu.FlagC != 0 && cyc == 5
		match(0x07, "SLO", "zp", "ASL+ORA", cyc, 5, ok, fmt.Sprintf("M=%02X A=%02X P=%02X cyc=%d", m[0x10], c.A, c.P, cyc))
	}
	{
		m, c := undocMem()
		m[0x2000] = 0x41
		cyc := runOp(m, c, []byte{0x0F, 0x00, 0x20}, 0x00, 0, 0, 0)
		ok := m[0x2000] == 0x82 && c.A == 0x82 && cyc == 6
		match(0x0F, "SLO", "abs", "ASL+ORA", cyc, 6, ok, fmt.Sprintf("M=%02X A=%02X cyc=%d", m[0x2000], c.A, cyc))
	}
	for _, tc := range []struct {
		op, mode string
		opc uint8
		setup func(*mem) []byte
		x, y uint8
		wantCyc uint
		wantM, wantA uint8
	}{
		{"SLO", "zpx", 0x17, func(m *mem) []byte { m[0x15] = 0x01; return []byte{0x17, 0x10} }, 0x05, 0, 6, 0x02, 0x02},
		{"SLO", "abx", 0x1F, func(m *mem) []byte { m[0x2010] = 0x01; return []byte{0x1F, 0x00, 0x20} }, 0x10, 0, 7, 0x02, 0x02},
		{"SLO", "aby", 0x1B, func(m *mem) []byte { m[0x2010] = 0x01; return []byte{0x1B, 0x00, 0x20} }, 0, 0x10, 7, 0x02, 0x02},
		{"SLO", "izx", 0x03, func(m *mem) []byte { m[0x20]=0; m[0x21]=0x60; m[0x6000]=0x01; return []byte{0x03, 0x20} }, 0, 0, 8, 0x02, 0x02},
		{"SLO", "izy", 0x13, func(m *mem) []byte { m[0x20]=0; m[0x21]=0x60; m[0x6003]=0x01; return []byte{0x13, 0x20} }, 0, 0x03, 8, 0x02, 0x02},
	} {
		m, c := undocMem()
		bytes := tc.setup(m)
		cyc := runOp(m, c, bytes, 0, tc.x, tc.y, 0)
		var gotM uint8
		switch tc.mode {
		case "zpx":
			gotM = m[0x15]
		case "abx", "aby":
			gotM = m[0x2010]
		case "izx":
			gotM = m[0x6000]
		case "izy":
			gotM = m[0x6003]
		}
		ok := gotM == tc.wantM && c.A == tc.wantA && cyc == tc.wantCyc
		match(tc.opc, "SLO", tc.mode, "ASL+ORA", cyc, tc.wantCyc, ok, fmt.Sprintf("M=%02X A=%02X cyc=%d", gotM, c.A, cyc))
	}

	// ===== RLA =====
	{
		m, c := undocMem()
		m[0x10] = 0x80
		cyc := runOp(m, c, []byte{0x27, 0x10}, 0xFF, 0, 0, 0) // C=0
		// ROL 80→00 C=1; AND FF → 00
		ok := m[0x10] == 0x00 && c.A == 0x00 && c.P&cpu.FlagC != 0 && cyc == 5
		match(0x27, "RLA", "zp", "ROL+AND", cyc, 5, ok, fmt.Sprintf("M=%02X A=%02X P=%02X", m[0x10], c.A, c.P))
	}
	{
		m, c := undocMem()
		m[0x2000] = 0x01
		cyc := runOp(m, c, []byte{0x2F, 0x00, 0x20}, 0x03, 0, 0, cpu.FlagC)
		// ROL 01 C=1 → 03; AND 03 → 03
		ok := m[0x2000] == 0x03 && c.A == 0x03 && cyc == 6
		match(0x2F, "RLA", "abs", "ROL+AND", cyc, 6, ok, fmt.Sprintf("M=%02X A=%02X", m[0x2000], c.A))
	}
	for _, opc := range []struct {
		op uint8
		mode string
		cyc uint
		setup func(*mem) []byte
		x, y uint8
		addr uint16
	}{
		{0x37, "zpx", 6, func(m *mem) []byte { m[0x15] = 0x40; return []byte{0x37, 0x10} }, 5, 0, 0x15},
		{0x3F, "abx", 7, func(m *mem) []byte { m[0x2010] = 0x40; return []byte{0x3F, 0x00, 0x20} }, 0x10, 0, 0x2010},
		{0x3B, "aby", 7, func(m *mem) []byte { m[0x2010] = 0x40; return []byte{0x3B, 0x00, 0x20} }, 0, 0x10, 0x2010},
		{0x23, "izx", 8, func(m *mem) []byte { m[0x20]=0; m[0x21]=0x70; m[0x7000]=0x40; return []byte{0x23, 0x20} }, 0, 0, 0x7000},
		{0x33, "izy", 8, func(m *mem) []byte { m[0x20]=0; m[0x21]=0x70; m[0x7002]=0x40; return []byte{0x33, 0x20} }, 0, 2, 0x7002},
	} {
		m, c := undocMem()
		bytes := opc.setup(m)
		cyc := runOp(m, c, bytes, 0xFF, opc.x, opc.y, 0)
		// ROL 40→80; AND FF→80
		ok := m[opc.addr] == 0x80 && c.A == 0x80 && cyc == opc.cyc
		match(opc.op, "RLA", opc.mode, "ROL+AND", cyc, opc.cyc, ok, fmt.Sprintf("M=%02X A=%02X cyc=%d", m[opc.addr], c.A, cyc))
	}

	// ===== SRE =====
	{
		m, c := undocMem()
		m[0x10] = 0x03
		cyc := runOp(m, c, []byte{0x47, 0x10}, 0x01, 0, 0, 0)
		// LSR 03→01 C=1; EOR 01 → A=00
		ok := m[0x10] == 0x01 && c.A == 0x00 && c.P&cpu.FlagC != 0 && cyc == 5
		match(0x47, "SRE", "zp", "LSR+EOR", cyc, 5, ok, fmt.Sprintf("M=%02X A=%02X P=%02X", m[0x10], c.A, c.P))
	}
	{
		m, c := undocMem()
		m[0x2000] = 0xFE
		cyc := runOp(m, c, []byte{0x4F, 0x00, 0x20}, 0x00, 0, 0, 0)
		ok := m[0x2000] == 0x7F && c.A == 0x7F && cyc == 6
		match(0x4F, "SRE", "abs", "LSR+EOR", cyc, 6, ok, fmt.Sprintf("M=%02X A=%02X", m[0x2000], c.A))
	}
	for _, opc := range []struct {
		op uint8
		mode string
		cyc uint
		setup func(*mem) []byte
		x, y uint8
		addr uint16
	}{
		{0x57, "zpx", 6, func(m *mem) []byte { m[0x15] = 0x02; return []byte{0x57, 0x10} }, 5, 0, 0x15},
		{0x5F, "abx", 7, func(m *mem) []byte { m[0x2010] = 0x02; return []byte{0x5F, 0x00, 0x20} }, 0x10, 0, 0x2010},
		{0x5B, "aby", 7, func(m *mem) []byte { m[0x2010] = 0x02; return []byte{0x5B, 0x00, 0x20} }, 0, 0x10, 0x2010},
		{0x43, "izx", 8, func(m *mem) []byte { m[0x20]=0; m[0x21]=0x91; m[0x9100]=0x02; return []byte{0x43, 0x20} }, 0, 0, 0x9100},
		{0x53, "izy", 8, func(m *mem) []byte { m[0x20]=0; m[0x21]=0x81; m[0x8102]=0x02; return []byte{0x53, 0x20} }, 0, 2, 0x8102},
	} {
		m, c := undocMem()
		bytes := opc.setup(m)
		// careful: program at 0x8000 — izx target 0x8000 overlaps! use different pages
		cyc := runOp(m, c, bytes, 0x01, opc.x, opc.y, 0)
		ok := m[opc.addr] == 0x01 && c.A == 0x00 && cyc == opc.cyc
		match(opc.op, "SRE", opc.mode, "LSR+EOR", cyc, opc.cyc, ok, fmt.Sprintf("M=%02X A=%02X cyc=%d", m[opc.addr], c.A, cyc))
	}

	// ===== RRA =====
	{
		m, c := undocMem()
		m[0x10] = 0x01
		cyc := runOp(m, c, []byte{0x67, 0x10}, 0x00, 0, 0, 0) // C=0
		// ROR 01→00 C=1; ADC 00 + C → A=01
		ok := m[0x10] == 0x00 && c.A == 0x01 && cyc == 5
		match(0x67, "RRA", "zp", "ROR+ADC", cyc, 5, ok, fmt.Sprintf("M=%02X A=%02X P=%02X cyc=%d", m[0x10], c.A, c.P, cyc))
	}
	{
		m, c := undocMem()
		m[0x2000] = 0x00
		cyc := runOp(m, c, []byte{0x6F, 0x00, 0x20}, 0x10, 0, 0, cpu.FlagC)
		// ROR 00 C=1 → 80; ADC: 10+80+0 = 90
		ok := m[0x2000] == 0x80 && c.A == 0x90 && cyc == 6
		match(0x6F, "RRA", "abs", "ROR+ADC", cyc, 6, ok, fmt.Sprintf("M=%02X A=%02X cyc=%d", m[0x2000], c.A, cyc))
	}
	for _, opc := range []struct {
		op uint8
		mode string
		cyc uint
		setup func(*mem) []byte
		x, y uint8
		addr uint16
	}{
		{0x77, "zpx", 6, func(m *mem) []byte { m[0x15] = 0x00; return []byte{0x77, 0x10} }, 5, 0, 0x15},
		{0x7F, "abx", 7, func(m *mem) []byte { m[0x2010] = 0x00; return []byte{0x7F, 0x00, 0x20} }, 0x10, 0, 0x2010},
		{0x7B, "aby", 7, func(m *mem) []byte { m[0x2010] = 0x00; return []byte{0x7B, 0x00, 0x20} }, 0, 0x10, 0x2010},
		{0x63, "izx", 8, func(m *mem) []byte { m[0x20]=0; m[0x21]=0x90; m[0x9000]=0x00; return []byte{0x63, 0x20} }, 0, 0, 0x9000},
		{0x73, "izy", 8, func(m *mem) []byte { m[0x20]=0; m[0x21]=0x90; m[0x9002]=0x00; return []byte{0x73, 0x20} }, 0, 2, 0x9002},
	} {
		m, c := undocMem()
		bytes := opc.setup(m)
		cyc := runOp(m, c, bytes, 0x01, opc.x, opc.y, cpu.FlagC)
		// ROR 00+C → 80; ADC 01+80 = 81
		ok := m[opc.addr] == 0x80 && c.A == 0x81 && cyc == opc.cyc
		match(opc.op, "RRA", opc.mode, "ROR+ADC", cyc, opc.cyc, ok, fmt.Sprintf("M=%02X A=%02X cyc=%d", m[opc.addr], c.A, cyc))
	}

	// ===== ANC / ALR / ARR / AXS / USBC =====
	{
		m, c := undocMem()
		cyc := runOp(m, c, []byte{0x0B, 0x80}, 0xFF, 0, 0, 0)
		ok := c.A == 0x80 && c.P&cpu.FlagN != 0 && c.P&cpu.FlagC != 0 && cyc == 2
		match(0x0B, "ANC", "imm", "AND; C←N", cyc, 2, ok, fmt.Sprintf("A=%02X P=%02X", c.A, c.P))
	}
	{
		m, c := undocMem()
		cyc := runOp(m, c, []byte{0x2B, 0x7F}, 0xFF, 0, 0, 0)
		ok := c.A == 0x7F && c.P&cpu.FlagC == 0 && cyc == 2
		match(0x2B, "ANC", "imm", "AND; C←N", cyc, 2, ok, fmt.Sprintf("A=%02X P=%02X", c.A, c.P))
	}
	{
		m, c := undocMem()
		cyc := runOp(m, c, []byte{0x4B, 0xFE}, 0xFF, 0, 0, 0)
		// ALR: FF&FE=FE; LSR → 7F C=0
		ok := c.A == 0x7F && c.P&cpu.FlagC == 0 && cyc == 2
		match(0x4B, "ALR", "imm", "AND+LSR", cyc, 2, ok, fmt.Sprintf("A=%02X P=%02X", c.A, c.P))
	}
	{
		m, c := undocMem()
		cyc := runOp(m, c, []byte{0x6B, 0xFF}, 0xC0, 0, 0, 0) // D=0
		// ARR: C0&FF=C0; ROR C=0 → 60; C←bit6=1; V←bit6⊕bit5=1⊕1=0
		ok := c.A == 0x60 && c.P&cpu.FlagC != 0 && c.P&cpu.FlagV == 0 && cyc == 2
		match(0x6B, "ARR", "imm", "AND+ROR; C=b6 V=b6⊕b5 (D=0)", cyc, 2, ok, fmt.Sprintf("A=%02X P=%02X", c.A, c.P))
	}
	{
		m, c := undocMem()
		cyc := runOp(m, c, []byte{0xCB, 0x01}, 0xFF, 0x0F, 0, 0)
		// AXS: (FF&0F)-01 = 0E; C=1
		ok := c.X == 0x0E && c.P&cpu.FlagC != 0 && cyc == 2
		match(0xCB, "AXS", "imm", "X:=(A&X)-#", cyc, 2, ok, fmt.Sprintf("X=%02X P=%02X", c.X, c.P))
	}
	{
		m, c := undocMem()
		cyc := runOp(m, c, []byte{0xEB, 0x01}, 0x05, 0, 0, cpu.FlagC)
		ok := c.A == 0x04 && c.P&cpu.FlagC != 0 && cyc == 2
		match(0xEB, "USBC", "imm", "SBC #imm alias", cyc, 2, ok, fmt.Sprintf("A=%02X P=%02X", c.A, c.P))
	}

	// ===== UNCLAIMED unstable =====
	unclaimed(0x8B, "XAA", "imm", "highly unstable (Graham highly-unstable); magic constant; no deterministic claim")
	unclaimed(0xAB, "LAX", "imm", "highly unstable (Graham highly-unstable); magic constant; no deterministic claim")
	unclaimed(0x93, "AHX", "izy", "unstable (Graham unstable); H+1 bus race; residual")
	unclaimed(0x9F, "AHX", "aby", "unstable (Graham unstable); H+1 bus race; residual")
	unclaimed(0x9C, "SHY", "abx", "unstable (Graham unstable); H+1 bus race; residual")
	unclaimed(0x9E, "SHX", "aby", "unstable (Graham unstable); H+1 bus race; residual")
	unclaimed(0x9B, "TAS", "aby", "unstable (Graham unstable); S + H+1 race; residual")
	unclaimed(0xBB, "LAS", "aby", "Graham: 'probably unreliable'; residual UNCLAIMED")

	// tally
	matched, differed, uncl := 0, 0, 0
	for _, r := range rows {
		switch r.Status {
		case "MATCH":
			matched++
		case "DIFF":
			differed++
		default:
			uncl++
		}
	}
	famOut := map[string]familyCount{}
	for k, v := range fam {
		famOut[k] = *v
	}
	result := "PASS"
	if differed > 0 {
		result = "FAIL"
	}
	rec := undocReceipt{
		Suite:     "nmos_6502_undoc_illegal_matrix",
		Result:    result,
		Matched:   matched,
		Differed:  differed,
		Unclaimed: uncl,
		Rows:      len(rows),
		Families:  famOut,
		Notes: []string{
			"Soft≠Physical — Digilent mutex Operator; NO flash",
			"Sources: Graham oxyron opcodes02.html; masswerk.at illegal demystified; Visual6502 observation-cite only (no geometry)",
			"Stable composites implemented: NOP/JAM/LAX/SAX/DCP/ISC/SLO/RLA/SRE/RRA/ANC/ALR/ARR(D=0)/AXS/USBC",
			"UNCLAIMED residual: XAA $8B, LAX# $AB, AHX $93/$9F, SHY $9C, SHX $9E, TAS $9B, LAS $BB",
			"ARR decimal-mode residual; Soft≠Physical",
		},
		SoftOnly: true,
		Matrix:   rows,
	}

	root := findRoot(t)
	art := filepath.Join(root, "artifacts")
	_ = os.MkdirAll(art, 0o755)
	jb, _ := json.MarshalIndent(rec, "", "  ")
	_ = os.WriteFile(filepath.Join(art, "witness-undoc.json"), jb, 0o644)
	txt := fmt.Sprintf("suite=nmos_6502_undoc_illegal_matrix result=%s matched=%d differed=%d unclaimed=%d rows=%d soft_only=true Soft≠Physical\n",
		result, matched, differed, uncl, len(rows))
	_ = os.WriteFile(filepath.Join(art, "witness-undoc.txt"), []byte(txt), 0o644)

	t.Logf("undoc matrix: matched=%d differed=%d unclaimed=%d rows=%d", matched, differed, uncl, len(rows))
	if differed > 0 {
		t.Fatalf("undoc matrix DIFF count=%d", differed)
	}
}

func findRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// tb/ → repo root
	return filepath.Clean(filepath.Join(wd, ".."))
}
