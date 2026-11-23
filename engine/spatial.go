// Spatial service (ours, Phase 2 / B): shared, read-only facts about space,
// computed once per tactical refresh and used by the tactical subsystems. It
// is not a decision system: it answers "who gets where first" and "which side
// controls a point", always with the one arrival model (kinematics.go).
//
// Pitch control: the pitch in 3 m cells (35 × 23); per cell the earliest
// arrival of each side (bodyTimeTo at full effort) and the home side's
// control probability, a logistic in the arrival difference
// (Spearman-style, τ = spatialTau). Cheap: 22 players × 805 cells per
// refresh, no RNG, no writes to the simulation.
package engine

import "math"

const (
	spatialCell = 3.0
	spatialNX   = 35 // 105 / 3
	spatialNY   = 23 // ceil(68 / 3)
	spatialTau  = 0.45
)

type spatialState struct {
	epoch   uint64                            // bumped every tactical refresh (spatialInvalidate)
	refresh uint64                            // the epoch the grid was built for (0: never)
	first   [2][spatialNX * spatialNY]float32 // earliest arrival per side (hBoolIndex)
	control [spatialNX * spatialNY]float32    // home control probability
}

// spatialInvalidate marks the grid stale for a new tactical refresh; it is
// rebuilt lazily on the first query (no consumer, no cost).
func (e *MatchEngine) spatialInvalidate() { e.spatial.epoch++ }

// spatialGrid returns the grid, rebuilding it if this refresh has not yet.
func (e *MatchEngine) spatialGrid() *spatialState {
	st := &e.spatial
	if st.refresh != st.epoch || st.epoch == 0 {
		e.spatialBuild()
	}
	return st
}

// spatialBuild rebuilds the grid for the current epoch.
func (e *MatchEngine) spatialBuild() {
	st := &e.spatial
	st.refresh = st.epoch
	for c := 0; c < spatialNX*spatialNY; c++ {
		st.first[0][c], st.first[1][c] = math.MaxFloat32, math.MaxFloat32
	}
	for i := range e.players {
		p := &e.players[i]
		if !active(p) {
			continue
		}
		side := hBoolIndex(p.IsHomeTeam)
		for c := 0; c < spatialNX*spatialNY; c++ {
			if t := float32(e.bodyTimeTo(p, spatialCellCentre(c), 1.0)); t < st.first[side][c] {
				st.first[side][c] = t
			}
		}
	}
	h := hBoolIndex(true)
	for c := 0; c < spatialNX*spatialNY; c++ {
		d := float64(st.first[1-h][c] - st.first[h][c]) // positive: home first
		st.control[c] = float32(1.0 / (1.0 + math.Exp(-d/spatialTau)))
	}
}

// spatialCellCentre is the FE-metre centre of cell c.
func spatialCellCentre(c int) feM1Vec {
	ix, iy := c%spatialNX, c/spatialNX
	return feM1Vec{-feM1HalfLength + (float64(ix)+0.5)*spatialCell, -feM1HalfWidth + (float64(iy)+0.5)*spatialCell}
}

func spatialCellOf(p feM1Vec) int {
	ix := int(hClamp((p.X+feM1HalfLength)/spatialCell, 0, spatialNX-1))
	iy := int(hClamp((p.Y+feM1HalfWidth)/spatialCell, 0, spatialNY-1))
	return iy*spatialNX + ix
}

// SpatialControl is the probability that the given side reaches the point
// (FE metres) first, from the current grid.
func (e *MatchEngine) SpatialControl(home bool, p feM1Vec) float64 {
	c := float64(e.spatialGrid().control[spatialCellOf(p)])
	if home {
		return c
	}
	return 1.0 - c
}

// spatialFirstArrival is the side's earliest arrival at p (seconds, grid).
func (e *MatchEngine) spatialFirstArrival(home bool, p feM1Vec) float64 {
	return float64(e.spatialGrid().first[hBoolIndex(home)][spatialCellOf(p)])
}
