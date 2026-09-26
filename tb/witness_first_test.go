// FIRST WITNESS — machine-check of mos6502-logos cycle twin against hand programs.
// Soft≠Physical. No Digilent flash. Not a silicon claim.
package tb_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/xhris57/mos6502-logos/twin/cpu"
)

type mem [65536]byte

func (m *mem) Read(a uint16) uint8     { return m[a] }
func (m *mem) Write(a uint16, v uint8) { m[a] = v }

type check struct {
	name string
	ok   bool
	detail string
}

type receipt struct {
	Suite      string            `json:"suite"`
	Result     string            `json:"result"`
	ErrCount   int               `json:"err"`
	PassCount  int               `json:"pass"`
	CycleTotal uint64            `json:"cycle_total"`
	Checks     []map[string]any  `json:"checks"`
	Notes      []string          `json:"notes"`
}

func resetAt(m *mem, pc uint16) *cpu.CPU {
	m[0xFFFC] = byte(pc)
	m[0xFFFD] = byte(pc >> 8)
	c := cpu.New(m)
	c.Reset()
	return c
}

func TestFirstWitness(t *testing.T) {
	var checks []check
	var cycleTotal uint64
	add := func(name string, ok bool, detail string) {
		checks = append(checks, check{name, ok, detail})
		if !ok {
			t.Errorf("FAIL %s: %s", name, detail)
		}
	}

	// --- 1. Reset vector fetch ---
	{
		var m mem
		m[0xFFFC] = 0x00
		m[0xFFFD] = 0x80
		c := cpu.New(&m)
		before := c.Cycles
		c.Reset()
		cyc := c.Cycles - before
		cycleTotal += cyc
		add("reset_vector_pc", c.PC == 0x8000, fmt.Sprintf("PC=%04X want 8000", c.PC))
		add("reset_vector_cycles", cyc == 7, fmt.Sprintf("cycles=%d want 7", cyc))
		add("reset_I_set", c.P&cpu.FlagI != 0, fmt.Sprintf("P=%02X", c.P))
	}

	// --- 2. LDA/STA imm + zp ---
	{
		var m mem
		c := resetAt(&m, 0x8000)
		cycleTotal += 7 // reset
		// LDA #$42; STA $10; LDA #$00; LDA $10
		copy(m[0x8000:], []byte{0xA9, 0x42, 0x85, 0x10, 0xA9, 0x00, 0xA5, 0x10})
		cycleTotal += uint64(c.Step())
		add("lda_imm", c.A == 0x42 && c.P&cpu.FlagZ == 0, fmt.Sprintf("A=%02X P=%02X", c.A, c.P))
		cycleTotal += uint64(c.Step())
		add("sta_zp", m[0x10] == 0x42, fmt.Sprintf("mem10=%02X", m[0x10]))
		cycleTotal += uint64(c.Step())
		add("lda_zero_flag", c.A == 0 && c.P&cpu.FlagZ != 0, fmt.Sprintf("A=%02X P=%02X", c.A, c.P))
		cycleTotal += uint64(c.Step())
		add("lda_zp", c.A == 0x42, fmt.Sprintf("A=%02X", c.A))
	}

	// --- 3. ADC/SBC ±C ---
	{
		var m mem
		c := resetAt(&m, 0x8000)
		cycleTotal += 7
		// CLC; LDA #$7F; ADC #$01  → A=80, V=1, N=1, C=0
		copy(m[0x8000:], []byte{0x18, 0xA9, 0x7F, 0x69, 0x01})
		cycleTotal += uint64(c.Step())
		cycleTotal += uint64(c.Step())
		cycleTotal += uint64(c.Step())
		add("adc_overflow", c.A == 0x80 && c.P&cpu.FlagV != 0 && c.P&cpu.FlagN != 0 && c.P&cpu.FlagC == 0,
			fmt.Sprintf("A=%02X P=%02X", c.A, c.P))
	}
	{
		var m mem
		c := resetAt(&m, 0x8000)
		cycleTotal += 7
		// SEC; LDA #$05; SBC #$01 → A=04, C=1
		copy(m[0x8000:], []byte{0x38, 0xA9, 0x05, 0xE9, 0x01})
		cycleTotal += uint64(c.Step())
		cycleTotal += uint64(c.Step())
		cycleTotal += uint64(c.Step())
		add("sbc_with_c", c.A == 0x04 && c.P&cpu.FlagC != 0, fmt.Sprintf("A=%02X P=%02X", c.A, c.P))
	}
	{
		var m mem
		c := resetAt(&m, 0x8000)
		cycleTotal += 7
		// CLC; LDA #$05; SBC #$01 → A=03, C=1? wait: C clear means borrow, so 5-1-1=3, C set if no borrow from 8-bit
		// SBC: A - M - (1-C). CLC → borrow 1 → 5-1-1=3; C set if result >=0 in unsigned sense (bin < 0x100)
		copy(m[0x8000:], []byte{0x18, 0xA9, 0x05, 0xE9, 0x01})
		cycleTotal += uint64(c.Step())
		cycleTotal += uint64(c.Step())
		cycleTotal += uint64(c.Step())
		add("sbc_borrow", c.A == 0x03 && c.P&cpu.FlagC != 0, fmt.Sprintf("A=%02X P=%02X", c.A, c.P))
	}
	{
		var m mem
		c := resetAt(&m, 0x8000)
		cycleTotal += 7
		// SED; CLC; LDA #$09; ADC #$01 → A=10 (BCD)
		copy(m[0x8000:], []byte{0xF8, 0x18, 0xA9, 0x09, 0x69, 0x01})
		for i := 0; i < 4; i++ {
			cycleTotal += uint64(c.Step())
		}
		add("adc_bcd_9plus1", c.A == 0x10, fmt.Sprintf("A=%02X", c.A))
	}

	// --- 4. Branches (taken + page cross) ---
	{
		var m mem
		c := resetAt(&m, 0x80F0)
		cycleTotal += 7
		// LDA #$00; BEQ +$11 → $8105; cycles: LDA=2, BEQ=2+1taken+1page=4
		copy(m[0x80F0:], []byte{0xA9, 0x00, 0xF0, 0x11})
		cycleTotal += uint64(c.Step())
		beq := c.Step()
		cycleTotal += uint64(beq)
		add("beq_target", c.PC == 0x8105, fmt.Sprintf("PC=%04X", c.PC))
		add("beq_page_cycles", beq == 4, fmt.Sprintf("cycles=%d want 4", beq))
	}
	{
		var m mem
		c := resetAt(&m, 0x8000)
		cycleTotal += 7
		// LDA #$01; BNE +2; LDA #$FF; NOP  — branch taken over LDA #$FF
		// 8000: A9 01 / D0 02 / A9 FF / EA
		copy(m[0x8000:], []byte{0xA9, 0x01, 0xD0, 0x02, 0xA9, 0xFF, 0xEA})
		cycleTotal += uint64(c.Step())
		bne := c.Step()
		cycleTotal += uint64(bne)
		add("bne_taken_pc", c.PC == 0x8006, fmt.Sprintf("PC=%04X", c.PC))
		add("bne_taken_cyc", bne == 3, fmt.Sprintf("cycles=%d want 3", bne))
		cycleTotal += uint64(c.Step()) // NOP
		add("bne_skipped_lda", c.A == 0x01, fmt.Sprintf("A=%02X", c.A))
	}

	// --- 5. JSR/RTS ---
	{
		var m mem
		c := resetAt(&m, 0x8000)
		cycleTotal += 7
		// JSR $9000; LDA #$AA
		// $9000: LDA #$55; RTS
		copy(m[0x8000:], []byte{0x20, 0x00, 0x90, 0xA9, 0xAA})
		copy(m[0x9000:], []byte{0xA9, 0x55, 0x60})
		jsr := c.Step()
		cycleTotal += uint64(jsr)
		add("jsr_pc", c.PC == 0x9000, fmt.Sprintf("PC=%04X", c.PC))
		add("jsr_cycles", jsr == 6, fmt.Sprintf("cycles=%d", jsr))
		cycleTotal += uint64(c.Step())
		add("jsr_sub_lda", c.A == 0x55, fmt.Sprintf("A=%02X", c.A))
		rts := c.Step()
		cycleTotal += uint64(rts)
		add("rts_pc", c.PC == 0x8003, fmt.Sprintf("PC=%04X", c.PC))
		add("rts_cycles", rts == 6, fmt.Sprintf("cycles=%d", rts))
		cycleTotal += uint64(c.Step())
		add("after_rts_lda", c.A == 0xAA, fmt.Sprintf("A=%02X", c.A))
	}

	// --- 6. Stack PHP/PLA ---
	{
		var m mem
		c := resetAt(&m, 0x8000)
		cycleTotal += 7
		copy(m[0x8000:], []byte{0x08, 0x68}) // PHP PLA
		cycleTotal += uint64(c.Step())
		cycleTotal += uint64(c.Step())
		add("php_pla_BU", c.A&cpu.FlagB != 0 && c.A&cpu.FlagU != 0, fmt.Sprintf("A=%02X", c.A))
	}
	{
		var m mem
		c := resetAt(&m, 0x8000)
		cycleTotal += 7
		// LDA #$99; PHA; LDA #$00; PLA
		copy(m[0x8000:], []byte{0xA9, 0x99, 0x48, 0xA9, 0x00, 0x68})
		for i := 0; i < 5; i++ {
			cycleTotal += uint64(c.Step())
		}
		add("pha_pla", c.A == 0x99, fmt.Sprintf("A=%02X", c.A))
	}

	// --- 7. Integrated hand program: reset → load/store → add → branch → jsr/rts → halt via BRK vector ---
	{
		var m mem
		// Program at $8000 (assembled by hand; see programs/first_witness.asm)
		prog := []byte{
			0xA9, 0x10,       // LDA #$10
			0x85, 0x00,       // STA $00
			0xA9, 0x20,       // LDA #$20
			0x18,             // CLC
			0x65, 0x00,       // ADC $00   → A=$30
			0x85, 0x01,       // STA $01
			0xC9, 0x30,       // CMP #$30
			0xF0, 0x02,       // BEQ ok
			0xA9, 0xEE,       // LDA #$EE  (fail path)
			0x20, 0x20, 0x80, // JSR $8020
			0xA9, 0x55,       // LDA #$55  (after RTS marker intent: overwritten by sub)
			0x00,             // BRK
			// pad to $8020
		}
		for len(prog) < 0x20 {
			prog = append(prog, 0xEA)
		}
		sub := []byte{
			0xA9, 0xAA, // LDA #$AA
			0x85, 0x02, // STA $02
			0x60,       // RTS
		}
		copy(m[0x8000:], prog)
		copy(m[0x8020:], sub)
		m[0xFFFE] = 0x00
		m[0xFFFF] = 0x90 // BRK → $9000
		m[0x9000] = 0xEA // NOP landing
		c := resetAt(&m, 0x8000)
		cycleTotal += 7
		// run until BRK consumed (PC lands at IRQ vector)
		for steps := 0; steps < 40 && c.PC != 0x9000; steps++ {
			cycleTotal += uint64(c.Step())
		}
		add("hand_sum_zp", m[0x01] == 0x30, fmt.Sprintf("zp01=%02X", m[0x01]))
		add("hand_sub_store", m[0x02] == 0xAA, fmt.Sprintf("zp02=%02X", m[0x02]))
		add("hand_brk_vector", c.PC == 0x9000, fmt.Sprintf("PC=%04X", c.PC))
		var buf bytes.Buffer
		_ = c.DumpLine(&buf)
		add("hand_dump_emitted", buf.Len() > 0, buf.String())
	}

	errCount := 0
	passCount := 0
	outChecks := make([]map[string]any, 0, len(checks))
	for _, ch := range checks {
		if ch.ok {
			passCount++
		} else {
			errCount++
		}
		outChecks = append(outChecks, map[string]any{
			"name": ch.name, "ok": ch.ok, "detail": ch.detail,
		})
	}
	result := "PASS"
	if errCount > 0 {
		result = "FAIL"
	}
	rec := receipt{
		Suite:      "first_witness",
		Result:     result,
		ErrCount:   errCount,
		PassCount:  passCount,
		CycleTotal: cycleTotal,
		Checks:     outChecks,
		Notes: []string{
			"Soft≠Physical — soft twin only; no Digilent flash; no silicon claim",
			"Cycle totals include reset(7) per scenario; not Φ1/Φ2 half-cycles",
			"Harvest: pi6502-go 3505eafe… · rpi-6502 58914ac2… · Visual6502 observation-cite only",
		},
	}
	raw, _ := json.MarshalIndent(rec, "", "  ")
	artDir := findArtifacts()
	if artDir != "" {
		path := filepath.Join(artDir, "witness-first.json")
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Logf("could not write receipt: %v", err)
		}
		txt := fmt.Sprintf("suite=first_witness result=%s err=%d pass=%d cycle_total=%d\n",
			result, errCount, passCount, cycleTotal)
		_ = os.WriteFile(filepath.Join(artDir, "witness-first.txt"), []byte(txt), 0o644)
	}
	t.Logf("%s", string(raw))
	if errCount > 0 {
		t.Fatalf("first_witness FAIL err=%d", errCount)
	}
}

func findArtifacts() string {
	candidates := []string{
		"artifacts",
		"../artifacts",
		"../../artifacts",
	}
	// also walk up from cwd
	wd, _ := os.Getwd()
	for d := wd; d != "/" && d != ""; d = filepath.Dir(d) {
		candidates = append(candidates, filepath.Join(d, "artifacts"))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}
	return ""
}
