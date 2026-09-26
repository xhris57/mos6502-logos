// Klaus Dormann 6502 functional test harness — Soft≠Physical.
// Loads testdata/6502_functional_test.bin, runs until success trap $3469
// or failure/timeout. Digilent mutex Operator — NO flash.
package tb_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xhris57/mos6502-logos/twin/cpu"
)

const (
	klausBinRel     = "testdata/6502_functional_test.bin"
	klausStartPC    = uint16(0x0400)
	klausSuccessPC  = uint16(0x3469) // jmp * ; test passed (listing)
	klausTestCaseZP = uint16(0x0200) // test_case byte
	klausCycleCap   = uint64(200_000_000)
	klausStuckN     = 8 // consecutive identical PC after a Step → trap
)

// known section labels from Klaus listing (test_case values). Incomplete map OK.
var klausSectionNames = map[uint8]string{
	0x00: "initialize",
	0x01: "ram_integrity_prep",
	0x02: "branch_wrap",
	0x03: "flags_partial",
	0x04: "immediate",
	0x05: "zero_page",
	0x06: "zp_xy",
	0x07: "absolute",
	0x08: "abs_xy",
	0x09: "indirect_x",
	0x0a: "indirect_y",
	0x0b: "transfers",
	0x0c: "stack_ops",
	0x0d: "shifts_rmw",
	0x0e: "stack_wrap",
	0x0f: "jmp_jsr",
	0x10: "brk_rti",
	0x11: "interrupt_traps",
	0x12: "decimal_adc",
	0x13: "decimal_sbc",
	// 0x14..0x2B: full official opcode×mode flag matrices (Klaus numbering)
	0x2B: "last_opcode_matrix",
	0xF0: "opcode_testing_complete", // Klaus marks F0 then success at $3469
}

type klausReceipt struct {
	Suite       string         `json:"suite"`
	Result      string         `json:"result"`
	Err         int            `json:"err"`
	Pass        int            `json:"pass"`
	Cycles      uint64         `json:"cycles"`
	PCEnd       string         `json:"pc_end"`
	FailAddress string         `json:"fail_address,omitempty"`
	TestCase    uint8          `json:"test_case"`
	MaxTestCase uint8          `json:"max_test_case"`
	Section     string         `json:"section,omitempty"`
	SuccessPC   string         `json:"success_pc"`
	StartPC     string         `json:"start_pc"`
	CycleCap    uint64         `json:"cycle_cap"`
	WallMs      int64          `json:"wall_ms"`
	ROM         map[string]any `json:"rom"`
	Notes       []string       `json:"notes"`
	SoftOnly    bool           `json:"soft_only"`
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, klausBinRel)); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// still try parent if bin missing
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// tb/ is cwd under go test → parent is repo root
	if _, err := os.Stat(filepath.Join(wd, "..", klausBinRel)); err == nil {
		return filepath.Clean(filepath.Join(wd, ".."))
	}
	t.Fatalf("cannot locate %s from %s", klausBinRel, wd)
	return ""
}

func TestKlausDormannFunctional(t *testing.T) {
	root := findRepoRoot(t)
	binPath := filepath.Join(root, klausBinRel)
	raw, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read ROM: %v", err)
	}
	if len(raw) != 65536 {
		t.Fatalf("ROM size %d want 65536", len(raw))
	}
	if raw[klausSuccessPC] != 0x4C ||
		raw[klausSuccessPC+1] != 0x69 ||
		raw[klausSuccessPC+2] != 0x34 {
		t.Fatalf("success trap bytes at $3469 unexpected: %02X %02X %02X",
			raw[klausSuccessPC], raw[klausSuccessPC+1], raw[klausSuccessPC+2])
	}

	var m mem
	copy(m[:], raw)

	c := cpu.New(&m)
	// Do NOT Reset() — ROM $FFFC is res_trap. Suite expects PC=$0400.
	c.PC = klausStartPC
	c.S = 0xFD
	c.P = cpu.FlagU | cpu.FlagI
	c.Cycles = 0

	wall0 := time.Now()
	var lastPC uint16
	stuck := 0
	outcome := "TIMEOUT"
	failAddr := ""
	var maxCase uint8

	for c.Cycles < klausCycleCap && !c.Jammed {
		if tc := m[klausTestCaseZP]; tc > maxCase && tc < 0xF0 {
			maxCase = tc
		}
		pcBefore := c.PC
		c.Step()
		if c.PC == klausSuccessPC && pcBefore == klausSuccessPC {
			// after executing JMP $3469, PC is again $3469
			stuck++
			if stuck >= klausStuckN {
				outcome = "PASS"
				break
			}
			continue
		}
		if c.PC == pcBefore {
			// self-trap (JMP *) that is not success
			stuck++
			if stuck >= klausStuckN {
				outcome = "FAIL"
				failAddr = fmt.Sprintf("%04X", c.PC)
				break
			}
		} else if c.PC == lastPC && lastPC != 0 && c.PC != klausSuccessPC {
			// rare: branch-to-self style traps
			stuck++
			if stuck >= klausStuckN*4 {
				outcome = "FAIL"
				failAddr = fmt.Sprintf("%04X", c.PC)
				break
			}
		} else {
			stuck = 0
		}
		lastPC = c.PC
	}
	wallMs := time.Since(wall0).Milliseconds()

	testCase := m[klausTestCaseZP]
	section := klausSectionNames[testCase]
	if section == "" {
		section = fmt.Sprintf("case_%02X", testCase)
	}

	pass, errN := 0, 0
	if outcome == "PASS" {
		pass = 1
	} else {
		errN = 1
	}

	notes := []string{
		"Soft≠Physical — Digilent mutex Operator; NO flash",
		"Source: Klaus2m5/6502_65C02_functional_tests @ 7954e2db (bin SHA256 fa12bfc7…)",
		"Start PC=$0400; success trap=$3469; test_case@$0200",
		"65C02 extended suite NOT run; undocumented opcodes residual",
		"RDY/SO / mid-instruction IRQ residual",
	}
	if outcome != "PASS" {
		notes = append(notes, fmt.Sprintf("STOPPED outcome=%s pc=%04X test_case=%02X (%s) cycles=%d",
			outcome, c.PC, testCase, section, c.Cycles))
	}

	rec := klausReceipt{
		Suite:       "klaus_dormann_6502_functional",
		Result:      outcome,
		Err:         errN,
		Pass:        pass,
		Cycles:      c.Cycles,
		PCEnd:       fmt.Sprintf("%04X", c.PC),
		FailAddress: failAddr,
		TestCase:    testCase,
		MaxTestCase: maxCase,
		Section:     section,
		SuccessPC:   fmt.Sprintf("%04X", klausSuccessPC),
		StartPC:     fmt.Sprintf("%04X", klausStartPC),
		CycleCap:    klausCycleCap,
		WallMs:      wallMs,
		ROM: map[string]any{
			"path":    klausBinRel,
			"url":     "https://raw.githubusercontent.com/Klaus2m5/6502_65C02_functional_tests/7954e2dbb49c469ea286070bf46cdd71aeb29e4b/bin_files/6502_functional_test.bin",
			"commit":  "7954e2dbb49c469ea286070bf46cdd71aeb29e4b",
			"sha256":  "fa12bfc761e6f9057e4cc01a665a7b800ff01ae91f598af1e39a1201d01953fd",
			"bytes":   65536,
			"license": "GPL-3.0",
		},
		Notes:    notes,
		SoftOnly: true,
	}

	artDir := filepath.Join(root, "artifacts")
	_ = os.MkdirAll(artDir, 0o755)

	txt := fmt.Sprintf("suite=klaus_dormann_6502_functional result=%s err=%d pass=%d cycles=%d pc_end=%s test_case=%02X max_test_case=%02X section=%s soft_only=1\n",
		rec.Result, rec.Err, rec.Pass, rec.Cycles, rec.PCEnd, rec.TestCase, rec.MaxTestCase, rec.Section)
	if failAddr != "" {
		txt += fmt.Sprintf("fail_address=%s\n", failAddr)
	}
	txtPath := filepath.Join(artDir, "witness-klaus.txt")
	jsonPath := filepath.Join(artDir, "witness-klaus.json")
	if err := os.WriteFile(txtPath, []byte(txt), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}
	jb, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	jb = append(jb, '\n')
	if err := os.WriteFile(jsonPath, jb, 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	t.Logf("klaus: %s cycles=%d pc=%s test_case=%02X (%s) wall=%dms",
		outcome, c.Cycles, rec.PCEnd, testCase, section, wallMs)

	if outcome != "PASS" {
		t.Errorf("Klaus functional %s: pc=%s fail=%s test_case=%02X cycles=%d (cap %d)",
			outcome, rec.PCEnd, failAddr, testCase, c.Cycles, klausCycleCap)
	}
}
