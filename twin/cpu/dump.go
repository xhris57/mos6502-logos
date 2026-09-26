package cpu

import (
	"encoding/json"
	"fmt"
	"io"
)

// Snapshot is one machine-checkable dump of register/bus/cycle state.
// Format is intentional for witness receipts (JSON lines or pretty JSON).
type Snapshot struct {
	A      uint8  `json:"A"`
	X      uint8  `json:"X"`
	Y      uint8  `json:"Y"`
	S      uint8  `json:"S"`
	P      uint8  `json:"P"`
	PC     uint16 `json:"PC"`
	Cycles uint64 `json:"cycles"`
	Jammed bool   `json:"jammed,omitempty"`
	PFlags string `json:"P_flags"` // human NVUBDIZC
}

func flagsString(p uint8) string {
	b := []byte{'-', '-', '-', '-', '-', '-', '-', '-'}
	names := []struct {
		bit  uint8
		mark byte
		idx  int
	}{
		{FlagN, 'N', 0},
		{FlagV, 'V', 1},
		{FlagU, 'U', 2},
		{FlagB, 'B', 3},
		{FlagD, 'D', 4},
		{FlagI, 'I', 5},
		{FlagZ, 'Z', 6},
		{FlagC, 'C', 7},
	}
	for _, n := range names {
		if p&n.bit != 0 {
			b[n.idx] = n.mark
		}
	}
	return string(b)
}

func (c *CPU) Snapshot() Snapshot {
	return Snapshot{
		A: c.A, X: c.X, Y: c.Y, S: c.S, P: c.P,
		PC: c.PC, Cycles: c.Cycles, Jammed: c.Jammed,
		PFlags: flagsString(c.P),
	}
}

// DumpJSON writes one Snapshot as JSON (+ newline).
func (c *CPU) DumpJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(c.Snapshot())
}

// DumpLine writes a compact text dump: PC A X Y S P cycles flags
func (c *CPU) DumpLine(w io.Writer) error {
	s := c.Snapshot()
	_, err := fmt.Fprintf(w, "PC=%04X A=%02X X=%02X Y=%02X S=%02X P=%02X cycles=%d flags=%s\n",
		s.PC, s.A, s.X, s.Y, s.S, s.P, s.Cycles, s.PFlags)
	return err
}
