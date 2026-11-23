// Spatial service (ours, Phase 2 / B): shared, read-only facts about space,
// computed once per tactical refresh and used by the tactical subsystems. It
// is not a decision system: it answers "who gets where first" and "which side
// controls a point", always with the one arrival model (kinematics.go).
//
// Pitch control: SpatialControl(side, point) — a logistic in the two sides'
// earliest arrivals at the point (bodyTimeTo at full effort, τ = spatialTau),
// computed on demand. Lanes: spatialLaneBlockers / spatialLaneOpen. No RNG,
// no writes to the simulation. (A 3 m grid rebuilt per refresh was tried in
// B0: 7× slower with refreshes nearly every step, so queries are exact.)
package engine

import "math"

// spatialTau (s): arrival-time difference over which control goes from
// even to ~90 % (Spearman-style logistic).
const spatialTau = 0.45

// SpatialControl is the probability that the given side reaches the point
// (FE metres) first: a logistic in the two sides' earliest arrivals (shared
// body model). Computed exactly on demand (22 arrival times), so consumers
// that ask a few hundred points per refresh cost less than the grid.
func (e *MatchEngine) SpatialControl(home bool, p feM1Vec) float64 {
	own, opp := math.Inf(1), math.Inf(1)
	for i := range e.players {
		q := &e.players[i]
		if !active(q) {
			continue
		}
		t := e.bodyTimeTo(q, p, 1.0)
		if q.IsHomeTeam == home {
			own = math.Min(own, t)
		} else {
			opp = math.Min(opp, t)
		}
	}
	if math.IsInf(own, 1) {
		return 0
	}
	if math.IsInf(opp, 1) {
		return 1
	}
	return 1.0 / (1.0 + math.Exp(-(opp-own)/spatialTau))
}

// spatialLaneBlockers is the one geometric lane definition (Phase 2 / B2):
// the number of active opponents (keepers included) whose projection falls
// on the segment from→to and who stand within radius of it (FE metres,
// world). Consumers set their own tolerance: positioning (M4, M5, M10) wants
// an empty lane (spatialLaneOpen); M2 rejects a pass only behind two bodies,
// since its ball-vs-bodies race prices a single one.
func (e *MatchEngine) spatialLaneBlockers(home bool, from, to feM1Vec, radius float64) int {
	dx, dy := to.X-from.X, to.Y-from.Y
	lengthSq := dx*dx + dy*dy
	n := 0
	for i := range e.players {
		o := &e.players[i]
		if o.IsHomeTeam == home || !active(o) {
			continue
		}
		q := feM1ToM(o.Position)
		ox, oy := q.X-from.X, q.Y-from.Y
		var off2 float64
		if lengthSq < 1.0e-9 {
			off2 = ox*ox + oy*oy
		} else {
			t := (ox*dx + oy*dy) / lengthSq
			if t < 0.0 || t > 1.0 {
				continue
			}
			ex, ey := ox-t*dx, oy-t*dy
			off2 = ex*ex + ey*ey
		}
		if off2 < radius*radius {
			n++
		}
	}
	return n
}

// spatialLaneOpen: no opponent in the lane (positioning).
func (e *MatchEngine) spatialLaneOpen(home bool, from, to feM1Vec, radius float64) bool {
	return e.spatialLaneBlockers(home, from, to, radius) == 0
}
