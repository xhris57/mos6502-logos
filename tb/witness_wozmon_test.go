// WOZMON Soft Apple-1-ish platform witness — Soft≠Physical.
// Boots classic WOZMON @ $FF00 on Soft 6502 twin + Soft PIA stubs.
// Soft-scripted keyboard injects examine/deposit. No Digilent flash.
// No real PIA timing / NTSC / physical Apple-1 claim.
package tb_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xhris57/mos6502-logos/platforms/apple1"
	"github.com/xhris57/mos6502-logos/twin/cpu"
)

const (
	wozmonSHA256   = "e5af0d1c4057bd8e0ef5cb069c208ff7cc0984a7dff53b12c5cf119de8cb5c25"
	wozmonCycleCap = uint64(5_000_000)
)

type wozCheck struct {
	name   string
	ok     bool
	detail string
}

type wozmonReceipt struct {
	Suite    string           `json:"suite"`
	Result   string           `json:"result"`
	Err      int              `json:"err"`
	Pass     int              `json:"pass"`
	Cycles   uint64           `json:"cycles"`
	PCEnd    string           `json:"pc_end"`
	Console  string           `json:"console"`
	Commands []string         `json:"commands_soft"`
	Checks   []map[string]any `json:"checks"`
	ROM      map[string]any   `json:"rom"`
	Notes    []string         `json:"notes"`
	SoftOnly bool             `json:"soft_only"`
	Residual []string         `json:"residual"`
}

func loadWozmonROM(t *testing.T) ([]byte, string) {
	t.Helper()
	root := findRepoRoot(t)
	candidates := []string{
		filepath.Join(root, "platforms/apple1/wozmon.bin"),
		filepath.Join(root, "programs/wozmon/wozmon.bin"),
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if len(b) != 256 {
			t.Fatalf("wozmon.bin size=%d want 256 path=%s", len(b), p)
		}
		return b, p
	}
	t.Fatal("wozmon.bin not found under platforms/apple1 or programs/wozmon")
	return nil, ""
}

// runUntilIdle runs Soft CPU until NEXTCHAR is spinning with empty Soft KBD
// queue (waiting for key), or cycle cap. Detect idle: PC in NEXTCHAR loop
// ($FF29 area) with no key ready for several consecutive Steps.
func runUntilKBDWait(c *cpu.CPU, b *apple1.Bus, capCycles uint64) {
	idle := 0
	for c.Cycles < capCycles {
		pcBefore := c.PC
		c.Step()
		// Soft: NEXTCHAR polls KBDCR at $FF29; with empty queue bit7=0 → BPL self
		if len(b.KBDQueue()) == 0 && (pcBefore == 0xFF29 || pcBefore == 0xFF2C || c.PC == 0xFF29 || c.PC == 0xFF2C) {
			idle++
			if idle > 8 {
				return
			}
		} else {
			idle = 0
		}
	}
}

func TestWozmonSoftPlatform(t *testing.T) {
	rom, romPath := loadWozmonROM(t)
	sum := sha256.Sum256(rom)
	sumHex := hex.EncodeToString(sum[:])

	var checks []wozCheck
	add := func(name string, ok bool, detail string) {
		checks = append(checks, wozCheck{name, ok, detail})
		if !ok {
			t.Errorf("FAIL %s: %s", name, detail)
		}
	}

	add("rom_sha256", sumHex == wozmonSHA256, fmt.Sprintf("got %s path=%s", sumHex, romPath))
	add("rom_reset_vector", rom[0xFC] == 0x00 && rom[0xFD] == 0xFF,
		fmt.Sprintf("FFFC=%02X%02X want 00FF", rom[0xFC], rom[0xFD]))

	bus := apple1.New(rom)
	// Soft seed: known pattern in low RAM for examine
	for i := 0; i < 16; i++ {
		bus.RAM[i] = byte(0xA0 + i) // A0 A1 … AF
	}

	c := cpu.New(bus)
	c.Reset() // Soft reset → $FF00 via vector in ROM window

	add("reset_pc", c.PC == 0xFF00, fmt.Sprintf("PC=%04X", c.PC))

	commands := []string{
		// After boot Soft prompt `\`, examine $0000..$0007
		"0000.0007\r",
		// Soft deposit at $0010 then examine single
		"0010: DE AD\r",
		"0010.0011\r",
	}

	// Phase 1: boot until Soft prompt (`\` + CR) and idle KBD wait
	runUntilKBDWait(c, bus, wozmonCycleCap)
	bootConsole := bus.ConsoleASCII()
	add("boot_backslash", strings.Contains(bootConsole, `\`),
		fmt.Sprintf("console=%q", bootConsole))

	// Soft-script each command: inject, then run until idle again
	for _, cmd := range commands {
		bus.InjectKeys(cmd)
		runUntilKBDWait(c, bus, wozmonCycleCap)
	}

	console := bus.ConsoleASCII()
	add("examine_addr_print", strings.Contains(console, "0000:"),
		fmt.Sprintf("console=%q", console))
	// Expect Soft dump of A0.. at least first byte
	add("examine_data_a0", strings.Contains(console, "A0"),
		fmt.Sprintf("console=%q", console))
	add("deposit_took", bus.RAM[0x0010] == 0xDE && bus.RAM[0x0011] == 0xAD,
		fmt.Sprintf("mem10=%02X mem11=%02X", bus.RAM[0x0010], bus.RAM[0x0011]))
	add("deposit_examine", strings.Contains(console, "DE") && strings.Contains(console, "AD"),
		fmt.Sprintf("console=%q", console))
	add("under_cycle_cap", c.Cycles < wozmonCycleCap,
		fmt.Sprintf("cycles=%d cap=%d", c.Cycles, wozmonCycleCap))

	errCount, passCount := 0, 0
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

	rec := wozmonReceipt{
		Suite:    "wozmon_soft_apple1",
		Result:   result,
		Err:      errCount,
		Pass:     passCount,
		Cycles:   c.Cycles,
		PCEnd:    fmt.Sprintf("%04X", c.PC),
		Console:  console,
		Commands: commands,
		Checks:   outChecks,
		ROM: map[string]any{
			"path":   romPath,
			"sha256": sumHex,
			"size":   len(rom),
			"org":    "FF00",
		},
		SoftOnly: true,
		Notes: []string{
			"Soft≠Physical — Soft twin + Soft PIA stubs only",
			"No Digilent flash; no silicon claim",
			"No real PIA timing / NTSC / physical Apple-1",
			"Soft-scripted KBD queue; DSP always-ready Soft stub",
		},
		Residual: []string{
			"Soft≠Physical",
			"PIA handshake latency Soft-assumed (always-ready DSP)",
			"Keyboard Soft byte queue ≠ scanned matrix",
			"Instruction-cycle twin only; not Φ1/Φ2 half-cycle",
			"Not silicon / FPGA bitstream / physical replica proven",
		},
	}

	raw, _ := json.MarshalIndent(rec, "", "  ")
	artDir := findArtifacts()
	if artDir != "" {
		_ = os.WriteFile(filepath.Join(artDir, "witness-wozmon.json"), raw, 0o644)
		txt := fmt.Sprintf(
			"suite=wozmon_soft_apple1 result=%s err=%d pass=%d cycles=%d pc_end=%04X soft_only=1\nconsole=%q\ncommands=%q\nrom_sha256=%s\n",
			result, errCount, passCount, c.Cycles, c.PC, console, commands, sumHex,
		)
		_ = os.WriteFile(filepath.Join(artDir, "witness-wozmon.txt"), []byte(txt), 0o644)
	}
	t.Logf("%s", string(raw))
	if errCount > 0 {
		t.Fatalf("wozmon_soft_apple1 FAIL err=%d console=%q", errCount, console)
	}
}
