// Team Shape Controller (ours, docs/PLAN-team-shape.md §4.1): the one owner of the collective
// shape. Every refresh it gives each of the 22 players a slot, the spot his role holds in the
// team shape for the current ball position and phase, for BOTH teams at once:
//   - the side in possession uses the possession formation, the other side the defending one;
//   - after a change of possession both sides take the transition shapes (fitted on the first
//     seconds after a change), easing into the settled shapes over teamShapeTransition seconds
//     (one orchestrated transition for both sides, not two systems).
//
// The formation model (team_shape_table.go) is fitted to full-tracking data (Metrica, two
// matches; audit/fit_formation.js): per slot and phase a quadratic in the ball position, in the
// team frame (u metres from the own goal line along the attack, w metres from the left
// touchline, left = the side of the team's slot-1 full-back).
//
// F1 (shadow): the slots are computed for the replay only (Diagnostics.Slots); nothing reads
// them, so the match is unchanged. Behaviour modules will send requests to the composer
// (compose.go) in F2.
package engine

import "math"

// teamShapeTransition (s): the window after a change of possession the transition shapes
// were fitted on; teamShapeEase (s): its last part, over which they ease into the settled
// shapes.
const (
	teamShapeTransition = 4.0
	teamShapeEase       = 1.5
)

type teamShapeState struct {
	set   bool
	home  bool    // side in possession
	since float64 // when it gained the ball
	// assign[side][k]: the slot (1..10) outfield player k holds; 0 = not yet assigned.
	assign [2][11]int
}

// teamShapeKeep (m²): a player keeps his slot unless a new assignment saves more than this
// in total squared distance per swapped player (hysteresis: slots do not flicker).
const teamShapeKeep = 36.0

// teamShapeEval is the formation model for one slot (0..10) and phase (0 possession,
// 1 defending, 2/3 the same in the first teamShapeTransition seconds after a change of
// possession) at the ball (ub, wb), team frame.
func teamShapeEval(phase, slot int, ub, wb float64) (u, w float64) {
	x, y := (ub-52.5)/52.5, (wb-34.0)/34.0
	f := [8]float64{1, x, y, x * x, y * y, x * y, x * x * x, x * y * y}
	c := &teamShapeModel[phase][slot]
	for i := range f {
		u += c[0][i] * f[i]
		w += c[1][i] * f[i]
	}
	return hClamp(u, 0.5, Pitch_LENGTH_METRES-0.5), hClamp(w, 0.5, Pitch_WIDTH_METRES-0.5)
}

// teamShapeSide is a side's team frame: dir along the length and whether w is mirrored so
// that the slot-1 full-back's side is "left" (low w).
type teamShapeSide struct {
	base   int // index of the side's first slot in e.players
	attack float64
	mirror bool
}

func (e *MatchEngine) teamShapeSide(home bool) teamShapeSide {
	s := teamShapeSide{attack: feM1Dir(home)}
	if !home {
		s.base = len(e.players) / 2
	}
	if s.base+4 < len(e.players) {
		s.mirror = e.players[s.base+1].BasePosition.Y > e.players[s.base+4].BasePosition.Y
	}
	return s
}

// toFrame / fromFrame convert pitch metres (corner origin) to the side's team frame.
func (s teamShapeSide) toFrame(x, y float64) (u, w float64) {
	u, w = x, y
	if s.attack < 0 {
		u = Pitch_LENGTH_METRES - x
	}
	if s.mirror {
		w = Pitch_WIDTH_METRES - y
	}
	return u, w
}

func (s teamShapeSide) fromFrame(u, w float64) (x, y float64) {
	return s.toFrame(u, w) // both maps are involutions
}

// teamShapeSlots returns each player's slot (pitch metres, corner origin) and whether he has
// one (on the pitch, outfield or keeper, slot index known).
func (e *MatchEngine) teamShapeSlots() (slots [][2]float64, ok []bool) {
	n := len(e.players)
	slots, ok = make([][2]float64, n), make([]bool, n)
	// possession: the carrier's side, else the side that last controlled the ball
	st := &e.teamShape
	now := e.getSimulatedSeconds()
	var home, known bool
	if c := e.findMatchPlayer(e.ball.PossessedBy); c != nil {
		home, known = c.IsHomeTeam, true
	} else if e.lastControlledTeamHome.HasValue {
		home, known = e.lastControlledTeamHome.Value, true
	}
	if known && (!st.set || st.home != home) {
		st.set, st.home, st.since = true, home, now
	}
	// the transition shape holds while it was fitted (the first teamShapeTransition
	// seconds), then eases into the settled one over teamShapeEase seconds
	blend := 1.0
	if st.set {
		blend = hClamp((now-st.since-teamShapeTransition)/teamShapeEase+1, 0, 1)
	}
	bx, by := e.ball.Position.X*Pitch_LENGTH_METRES, e.ball.Position.Y*Pitch_WIDTH_METRES
	for _, side := range [2]bool{true, false} {
		fr := e.teamShapeSide(side)
		ub, wb := fr.toFrame(bx, by)
		phase := 1 // defending
		if st.set && st.home == side {
			phase = 0
		}
		// the template: slot positions for this phase (team frame), then the spread
		// correction about the outfield centre, then pitch metres
		var fu, fw [11]float64
		for k := 0; k < 11; k++ {
			u, w := teamShapeEval(phase, k, ub, wb)
			if blend < 1 {
				// the transition shape (fitted on the first seconds after a change),
				// easing into the settled one as the new phase settles
				u0, w0 := teamShapeEval(phase+2, k, ub, wb)
				u, w = u0+(u-u0)*blend, w0+(w-w0)*blend
			}
			fu[k], fw[k] = u, w
		}
		su := teamShapeSpread[phase+2][0] + (teamShapeSpread[phase][0]-teamShapeSpread[phase+2][0])*blend
		sw := teamShapeSpread[phase+2][1] + (teamShapeSpread[phase][1]-teamShapeSpread[phase+2][1])*blend
		cu, cw := 0.0, 0.0
		for k := 1; k < 11; k++ {
			cu += fu[k] / 10
			cw += fw[k] / 10
		}
		var tpl [11][2]float64
		for k := 0; k < 11; k++ {
			u, w := fu[k], fw[k]
			if k > 0 {
				u = hClamp(cu+(u-cu)*su, 0.5, Pitch_LENGTH_METRES-0.5)
				w = hClamp(cw+(w-cw)*sw, 0.5, Pitch_WIDTH_METRES-0.5)
			}
			x, y := fr.fromFrame(u, w)
			tpl[k] = [2]float64{x, y}
		}
		e.teamShapeAssign(side, fr, &tpl)
		si := hBoolIndex(!side)
		for k := 0; k < n/2 && k < 11; k++ {
			i := fr.base + k
			p := &e.players[i]
			if p.Player == nil || !p.OnPitch {
				continue
			}
			slot := k
			if k > 0 && st.assign[si][k] > 0 {
				slot = st.assign[si][k]
			}
			slots[i], ok[i] = tpl[slot], true
		}
	}
	return slots, ok
}

// teamShapeDistance is a helper for probes: metres from a player to his slot.
func teamShapeDistance(p *MatchPlayer, slot [2]float64) float64 {
	return math.Hypot(p.Position.X*Pitch_LENGTH_METRES-slot[0], p.Position.Y*Pitch_WIDTH_METRES-slot[1])
}

// teamShapeAssign gives the side's ten outfielders the ten outfield slots: the assignment
// with the least total squared distance (Hungarian), kept unless the new one is better by
// more than teamShapeKeep per changed player. Real players swap roles as the play moves;
// a fixed player-to-slot map would draw lines no team keeps.
func (e *MatchEngine) teamShapeAssign(home bool, fr teamShapeSide, tpl *[11][2]float64) {
	st := &e.teamShape
	si := hBoolIndex(!home)
	var cost [10][10]float64
	var pos [10][2]float64
	present := [10]bool{}
	for k := 1; k < 11; k++ {
		i := fr.base + k
		if i >= len(e.players) {
			continue
		}
		p := &e.players[i]
		present[k-1] = p.Player != nil && p.OnPitch
		pos[k-1] = [2]float64{p.Position.X * Pitch_LENGTH_METRES, p.Position.Y * Pitch_WIDTH_METRES}
	}
	for a := 0; a < 10; a++ {
		for b := 0; b < 10; b++ {
			if !present[a] {
				continue // an absent player costs nothing anywhere
			}
			dx, dy := pos[a][0]-tpl[b+1][0], pos[a][1]-tpl[b+1][1]
			cost[a][b] = dx*dx + dy*dy
		}
	}
	best := hungarian10(&cost)
	prev := st.assign[si]
	if prev[1] == 0 { // first call: identity, then improve
		for k := 1; k < 11; k++ {
			prev[k] = k
		}
	}
	total := func(as func(a int) int) float64 {
		t := 0.0
		for a := 0; a < 10; a++ {
			t += cost[a][as(a)]
		}
		return t
	}
	cur := total(func(a int) int { return prev[a+1] - 1 })
	nw := total(func(a int) int { return best[a] })
	changed := 0
	for a := 0; a < 10; a++ {
		if best[a] != prev[a+1]-1 {
			changed++
		}
	}
	if changed > 0 && cur-nw > teamShapeKeep*float64(changed) {
		for a := 0; a < 10; a++ {
			prev[a+1] = best[a] + 1
		}
	}
	st.assign[si] = prev
}

// hungarian10 solves the 10x10 assignment (minimum total cost): row -> column.
func hungarian10(c *[10][10]float64) [10]int {
	const n = 10
	var u, v [n + 1]float64
	var p, way [n + 1]int
	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		var minv [n + 1]float64
		var used [n + 1]bool
		for j := range minv {
			minv[j] = math.Inf(1)
		}
		for {
			used[j0] = true
			i0, delta, j1 := p[j0], math.Inf(1), 0
			for j := 1; j <= n; j++ {
				if used[j] {
					continue
				}
				cur := c[i0-1][j-1] - u[i0] - v[j]
				if cur < minv[j] {
					minv[j], way[j] = cur, j0
				}
				if minv[j] < delta {
					delta, j1 = minv[j], j
				}
			}
			for j := 0; j <= n; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}
			j0 = j1
			if p[j0] == 0 {
				break
			}
		}
		for {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
			if j0 == 0 {
				break
			}
		}
	}
	var out [n]int
	for j := 1; j <= n; j++ {
		out[p[j]-1] = j - 1
	}
	return out
}
