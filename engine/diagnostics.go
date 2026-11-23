// Diagnostics (ours): observability options of one match. They never change
// the simulation (verified by replay comparison) and are therefore not part
// of EngineConfig or its hash.
package engine

import (
	"io"
	"os"
)

// Diagnostics selects per-match trace and audit output.
type Diagnostics struct {
	// M2Trace: 0 off, 1 one line per M2 decision, 2 plus every pass option,
	// 3 plus every candidate rejection.
	M2Trace int
	// Audit enables the read-only probe (audit_probe.go).
	Audit bool
	// Slots records the Team Shape Controller's slots in the replay (shadow mode,
	// team_shape.go); read-only.
	Slots bool
	// Out receives trace lines; nil means os.Stderr.
	Out io.Writer
}

func (d *Diagnostics) out() io.Writer {
	if d.Out == nil {
		return os.Stderr
	}
	return d.Out
}
