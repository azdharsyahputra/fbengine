// Team shape wiring (ours, docs/PLAN-team-shape.md F2): with Modules.TeamShape on, the Team
// Shape Controller's slots and the Position Composer own every live-play target. The legacy
// layers (P12 branches, M4/M5/M10 composition, M1 tasks, loose-ball resolver) still run, but
// their decisions are read as REQUESTS, classified by the reason and intent they recorded:
//   - ball jobs (tier 1): carry, receive, loose-ball claim, press, first defender, contain,
//     intercept, box marker — their target stands;
//   - quota jobs (tier 2): runs in behind, box runs, overlaps, doubling — granted per side up
//     to the quota, the rest hold their slot;
//   - offset jobs (tier 3): support, cover, screen, marks, duties, transition support — the
//     target is kept within a distance of the slot;
//   - everything else (shape, structure, spacing, get-open, M1 shape / protect depth /
//     recover shape, recovery) holds the slot.
//
// The keeper, restarts, set-piece deliveries and stoppages stay with their owners.
package engine

import (
	"math"
	"sort"
)

// teamShapeClass returns the composer tier of a legacy decision and, for tier 2, its quota;
// ok = false keeps the legacy target untouched (keeper, set pieces, restarts). danger: the
// ball is within the defending side's danger depth (M1 recover shape then carries the box
// marking, so it is an offset, not the slot).
func teamShapeClass(p *MatchPlayer, danger bool) (class composeClass, quota composeQuotaKind, slot bool, ok bool) {
	switch p.Intent {
	case PlayerIntent_CARRY_BALL, PlayerIntent_RECEIVE_PASS:
		return composeBall, 0, false, true
	case PlayerIntent_CLAIM_LOOSE_BALL, PlayerIntent_PRESS_BALL:
		// only the resolver's claimer and the designated pressers/interceptors contest the
		// ball; a supporter who also "goes for it" (P12 transition support) keeps his job
		// class below, so a loose ball does not draw a crowd
		switch p.TargetReason {
		case TargetReason_LOOSE_CLAIM, TargetReason_PRESS, TargetReason_M1_FIRST_DEFENDER,
			TargetReason_M1_CONTAIN, TargetReason_M1_SHAPE, TargetReason_RECEIVE, TargetReason_CARRY:
			return composeBall, 0, false, true
		}
	}
	switch p.TargetReason {
	case TargetReason_GK, TargetReason_AERIAL_HOLD, TargetReason_RESTART, TargetReason_HALF_TIME_WALK, TargetReason_SHOOTOUT:
		return 0, 0, false, false
	case TargetReason_CARRY, TargetReason_BYLINE, TargetReason_RECEIVE, TargetReason_LOOSE_CLAIM, TargetReason_PRESS,
		TargetReason_M1_FIRST_DEFENDER, TargetReason_M1_CONTAIN, TargetReason_M1_MARKER:
		return composeBall, 0, false, true
	case TargetReason_RUNNER, TargetReason_DUTY_RUNNER, TargetReason_TRANSITION_RUN:
		return composeQuota, composeRunInBehind, false, true
	case TargetReason_RUNNER_BOX, TargetReason_ARRIVAL, TargetReason_FAR_POST:
		return composeQuota, composeBoxRun, false, true
	case TargetReason_OVERLAP:
		return composeQuota, composeOverlap, false, true
	case TargetReason_DOUBLE:
		return composeQuota, composeExtraPress, false, true
	case TargetReason_SUPPORT_NEAR, TargetReason_SUPPORT_SQUARE, TargetReason_SUPPORT_TRAILING, TargetReason_SHORT_FORWARD,
		TargetReason_M1_COVER, TargetReason_M1_SCREEN, TargetReason_COVER, TargetReason_COVER_LANE,
		TargetReason_TIGHT_MARK, TargetReason_ZONAL_MARK, TargetReason_FLIGHT_MARK,
		TargetReason_LOOSE_COVER, TargetReason_LOOSE_DELAY,
		TargetReason_DUTY_LINK, TargetReason_DUTY_SECURITY, TargetReason_TRANSITION_SUPPORT,
		TargetReason_M1_PROTECT_DEPTH: // the line drops with the deepest runner
		return composeOffset, 0, false, true
	case TargetReason_M1_RECOVER_SHAPE:
		if danger { // near goal it holds the zonal/tight marking of a man
			return composeOffset, 0, false, true
		}
	}
	// shape, structure, spacing, get-open, M1 shape/depth/recover, recover, flight hold, and
	// flight support (with the own pass in flight the others carry on to their slots)
	return 0, 0, true, true
}

// teamShapeDev is the deviation allowed from the slot for a tier-3 request.
func (e *MatchEngine) teamShapeDev(r TargetReason) float64 {
	c := &e.cfg.TeamShape
	switch r {
	case TargetReason_SUPPORT_NEAR, TargetReason_SUPPORT_SQUARE, TargetReason_SUPPORT_TRAILING, TargetReason_SHORT_FORWARD:
		return c.SupportDev
	case TargetReason_DUTY_LINK, TargetReason_DUTY_SECURITY, TargetReason_TRANSITION_SUPPORT,
		TargetReason_LOOSE_COVER, TargetReason_LOOSE_DELAY:
		return c.DutyDev
	case TargetReason_M1_PROTECT_DEPTH:
		return c.DepthDev
	}
	return c.CoverDev
}

// teamShapeCompose runs at the end of the tactical refresh (Modules.TeamShape, live play).
func (e *MatchEngine) teamShapeCompose() {
	n := len(e.players)
	slots, ok := e.teamShapeSlots()
	st := &e.teamShape
	st.last, st.lastOK = slots, ok
	cfg := composeConfig{MinSpacing: e.cfg.TeamShape.MinSpacing, HoldSpeed: composeJog}
	cfg.Quota[composeRunInBehind] = e.cfg.TeamShape.RunQuota
	cfg.Quota[composeOverlap] = e.cfg.TeamShape.OverlapQuota
	cfg.Quota[composeBoxRun] = e.cfg.TeamShape.BoxQuota
	cfg.Quota[composeExtraPress] = e.cfg.TeamShape.PressQuota
	home := make([]bool, n)
	keep := make([]bool, n) // legacy target untouched
	reqs := make([]composeRequest, 0, n)
	for i := range e.players {
		p := &e.players[i]
		home[i] = p.IsHomeTeam
		p.ComposeSpeed = 0
		if !active(p) || p.IsGoalkeeper || !ok[i] {
			keep[i] = true
			continue
		}
		// danger: the ball near the own goal AND the player's slot in the marking zone
		// (box and its edge): only there does M1's recover shape carry a man-marking
		// worth keeping; elsewhere its target is the old block shape, so the slot wins.
		danger := false
		if st.set && st.home != p.IsHomeTeam {
			fr := e.teamShapeSide(p.IsHomeTeam)
			ub, _ := fr.toFrame(e.ball.Position.X*Pitch_LENGTH_METRES, e.ball.Position.Y*Pitch_WIDTH_METRES)
			us, _ := fr.toFrame(slots[i][0], slots[i][1])
			danger = ub < e.cfg.TeamShape.DangerDepth && us < e.cfg.TeamShape.MarkZone
		}
		class, quota, slot, use := teamShapeClass(p, danger)
		if !use {
			keep[i] = true
			continue
		}
		// native defence (F2b): the defending side's jobs come from teamShapeDefence below,
		// not from M1's tasks; of its legacy jobs only the ball contest stays (loose ball,
		// interception of a pass in flight)
		if ts0 := &e.cfg.TeamShape; ts0.NativeDefence && st.set && st.home != p.IsHomeTeam {
			if p.Intent == PlayerIntent_CLAIM_LOOSE_BALL && e.findMatchPlayer(e.ball.PossessedBy) == nil {
				t := [2]float64{p.TacticalTarget.X * Pitch_LENGTH_METRES, p.TacticalTarget.Y * Pitch_WIDTH_METRES}
				reqs = append(reqs, composeRequest{Player: i, Class: composeBall, Target: t, Owner: p.TargetOwner, Reason: p.TargetReason})
			}
			continue // otherwise the slot (or a native job)
		}
		if slot {
			continue // tier 4: holds the slot
		}
		t := [2]float64{p.TacticalTarget.X * Pitch_LENGTH_METRES, p.TacticalTarget.Y * Pitch_WIDTH_METRES}
		reqs = append(reqs, composeRequest{Player: i, Class: class, Quota: quota, Target: t,
			MaxDev: e.teamShapeDev(p.TargetReason), Owner: p.TargetOwner, Reason: p.TargetReason})
	}
	var native []teamShapeJob
	if e.cfg.TeamShape.NativeDefence && st.set {
		native = e.teamShapeDefence(!st.home, slots, ok)
		for _, j := range native {
			reqs = append(reqs, j.req)
		}
	}
	nativeOf := make(map[int]teamShapeJob, len(native))
	for _, j := range native {
		nativeOf[j.req.Player] = j
	}
	// players kept on their legacy target are fixed points for the spacing pass
	for i := range e.players {
		if keep[i] && ok[i] {
			p := &e.players[i]
			reqs = append(reqs, composeRequest{Player: i, Class: composeBall,
				Target: [2]float64{p.TacticalTarget.X * Pitch_LENGTH_METRES, p.TacticalTarget.Y * Pitch_WIDTH_METRES},
				Owner:  p.TargetOwner, Reason: p.TargetReason})
		}
	}
	out := compose(cfg, slots, ok, home, reqs)
	ts := &e.cfg.TeamShape
	now := e.getSimulatedSeconds()
	defer func() {
		if len(st.prev) != n {
			st.prev = make([][2]float64, n)
		}
		for i := range out {
			st.prev[i] = out[i].Target
		}
		st.prevSet, st.prevTime = true, now
	}()
	for i := range e.players {
		p := &e.players[i]
		if keep[i] || !ok[i] {
			continue
		}
		r := out[i]
		p.TacticalTarget = Vector2F{hClamp(r.Target[0]/Pitch_LENGTH_METRES, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
			hClamp(r.Target[1]/Pitch_WIDTH_METRES, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
		native := false
		if j, isNative := nativeOf[i]; isNative && r.Class != -1 && r.Owner == TargetOwner_TSC {
			p.TargetOwner, p.TargetReason, p.TargetRefiner = TargetOwner_TSC, j.req.Reason, TargetOwner_NONE
			p.Intent = j.intent
			p.UrgentMovement = j.urgent
			if j.urgent {
				// an engagement or a man to track near goal: full pace, M3 executes directly
				p.ComposeSpeed, p.ComposeWant = uint8(composeSprint)+1, 99
				continue
			}
			if j.man >= 0 {
				// a marker away from goal tracks his man at the man's pace (plus closing
				// the gap), neither sprinting at every touch nor left behind
				o := &e.players[j.man]
				want := math.Hypot(o.Velocity.X, o.Velocity.Y) + teamShapeDistance(p, r.Target)/e.cfg.TeamShape.CloseTime
				p.ComposeSpeed, p.ComposeWant = uint8(composeRun)+1, math.Min(want, e.cfg.TeamShape.RunSpeed)
				continue
			}
			native = true
		}
		switch {
		case native:
		case r.Class == -1:
			p.TargetOwner, p.TargetReason, p.TargetRefiner = TargetOwner_TSC, TargetReason_SLOT, TargetOwner_NONE
			if p.Intent != PlayerIntent_RECOVER_SHAPE && p.Intent != PlayerIntent_MARK_OPPONENT {
				p.Intent = PlayerIntent_HOLD_SHAPE
			}
		case r.Class == int(composeOffset):
			p.TargetRefiner = TargetOwner_TSC
		default:
			continue // tier 1/2: the legacy effort (M3/M1) keeps the pace
		}
		// slot and offset targets: speed = the target's own speed (feed-forward: a slot
		// moves with the ball) + closing the gap in ts.CloseTime; classes by that speed
		d := teamShapeDistance(p, r.Target)
		vt := 0.0
		if st.prevSet && now > st.prevTime {
			vt = math.Min(7, math.Hypot(r.Target[0]-st.prev[i][0], r.Target[1]-st.prev[i][1])/(now-st.prevTime))
		}
		want := vt + d/ts.CloseTime
		// the defending block shifts at a shuffle; only a player out of position by more
		// than RunDistance runs. The side in possession moves up with the ball freely.
		defending := st.set && st.home != p.IsHomeTeam
		if defending && d <= ts.RunDistance {
			want = math.Min(want, ts.ShiftSpeed)
		}
		// in possession, settled near the spot: walk (real players walk about half the
		// time); a defender keeps his shuffle (urgency near the box matters)
		if !defending && d <= ts.WalkZone {
			want = math.Min(want, ts.WalkSpeed)
		}
		if defending && d > ts.RunDistance {
			want = math.Min(want, ts.DefRunSpeed) // out of position: back at a run, not a sprint
		}
		p.ComposeWant = want
		switch {
		case want > ts.JogSpeed || d > ts.RunDistance:
			p.ComposeSpeed = uint8(composeRun) + 1
		case want > ts.WalkSpeed || d > ts.JogDistance:
			p.ComposeSpeed = uint8(composeJog) + 1
		default:
			p.ComposeSpeed = uint8(composeWalk) + 1
		}
		// a defender the ball has bypassed trots back (unless it nears his goal)
		if st.set && st.home != p.IsHomeTeam && p.ComposeSpeed > uint8(composeWalk)+1 {
			fr := e.teamShapeSide(p.IsHomeTeam)
			ub, _ := fr.toFrame(e.ball.Position.X*Pitch_LENGTH_METRES, e.ball.Position.Y*Pitch_WIDTH_METRES)
			up, _ := fr.toFrame(p.Position.X*Pitch_LENGTH_METRES, p.Position.Y*Pitch_WIDTH_METRES)
			if up > ub+3 && ub > e.cfg.M1.BypassedSprintGoal {
				p.ComposeSpeed = teamShapeTrot
			}
		}
	}
}

// teamShapeTrot is the ComposeSpeed value of a bypassed defender's recovery trot.
const teamShapeTrot = uint8(composeSprint) + 2

// teamShapeSpeed is the speed cap (m/s) of a composer speed class.
func (e *MatchEngine) teamShapeSpeed(class uint8) float64 {
	ts := &e.cfg.TeamShape
	if class == teamShapeTrot {
		return ts.TrotSpeed
	}
	switch composeSpeed(class - 1) {
	case composeWalk:
		return ts.WalkSpeed
	case composeJog:
		return ts.JogSpeed
	case composeRun:
		return ts.RunSpeed
	}
	return 99
}

// teamShapeJob is a native defensive job (F2b): the request plus what it means for the player.
type teamShapeJob struct {
	req    composeRequest
	intent PlayerIntent
	urgent bool
	man    int // marked attacker (markers away from goal), -1 otherwise
}

// teamShapeDefence builds the defending side's jobs on top of its slots (pressure, cover,
// balance, as real defending is taught): one first defender goal-side of the carrier, one
// cover behind him, near the goal a man-marker for each attacker in the danger zone (from the
// slot-holder nearest him), and a back-line player dropping with a real runner in behind.
// Everyone else holds his slot. home = the defending side.
func (e *MatchEngine) teamShapeDefence(home bool, slots [][2]float64, ok []bool) []teamShapeJob {
	ts := &e.cfg.TeamShape
	st := &e.teamShape
	fr := e.teamShapeSide(home)
	var jobs []teamShapeJob
	used := map[int]bool{}
	gx := 0.0 // own goal centre, pitch metres (corner origin)
	if !home {
		gx = Pitch_LENGTH_METRES
	}
	goal := [2]float64{gx, Pitch_WIDTH_METRES / 2}
	bx, by := e.ball.Position.X*Pitch_LENGTH_METRES, e.ball.Position.Y*Pitch_WIDTH_METRES
	ub, _ := fr.toFrame(bx, by)
	pm := func(p *MatchPlayer) [2]float64 {
		return [2]float64{p.Position.X * Pitch_LENGTH_METRES, p.Position.Y * Pitch_WIDTH_METRES}
	}
	toward := func(from, to [2]float64, d float64) [2]float64 {
		dx, dy := to[0]-from[0], to[1]-from[1]
		l := math.Max(math.Hypot(dx, dy), 1e-9)
		return [2]float64{from[0] + dx/l*d, from[1] + dy/l*d}
	}
	defenders := func(f func(i int, p *MatchPlayer) bool) {
		for i := range e.players {
			p := &e.players[i]
			if p.IsHomeTeam != home || !active(p) || p.IsGoalkeeper || !ok[i] || used[i] {
				continue
			}
			if !f(i, p) {
				return
			}
		}
	}
	// 1. first defender + cover against a carrier, or (pressing trigger) against the
	// intended receiver of an opponent pass in flight, so the presser leaves with the pass
	// a keeper on the ball is not chased (real keepers are left about 10 m); the block holds
	c := e.findMatchPlayer(e.ball.PossessedBy)
	if c == nil && e.ball.IsPass && !e.ball.IsShot && e.ball.PassByHome != home && ts.PassTrigger {
		c = e.findMatchPlayer(e.ball.IntendedReceiver)
	}
	if c != nil && c.IsHomeTeam != home && !c.IsGoalkeeper {
		cp := pm(c)
		pred := [2]float64{cp[0] + c.Velocity.X*0.3, cp[1] + c.Velocity.Y*0.3}
		press := toward(pred, goal, e.cfg.M1.ContainDistance)
		best, bt := -1, math.Inf(1)
		defenders(func(i int, p *MatchPlayer) bool {
			t := e.bodyTimeTo(p, feM1ToM(Vector2F{press[0] / Pitch_LENGTH_METRES, press[1] / Pitch_WIDTH_METRES}), 1.0)
			if i == st.fd-1 && st.fdCarrier == c {
				t -= ts.KeepTime // hysteresis: the engaged defender keeps the job (same carrier)
			}
			if t < bt {
				best, bt = i, t
			}
			return true
		})
		// organised press: the ball deep in the opponent's half (Press.Depth past halfway);
		// counter-press: the first seconds after losing the ball, anywhere but near the
		// own goal (real carriers who just won it are pressed ~55% of the time)
		inPress := ub-Pitch_LENGTH_METRES/2 >= e.cfg.Press.Depth ||
			(e.getSimulatedSeconds()-st.since < ts.CounterPress && ub >= ts.EngageDepth)
		window := e.cfg.M1.ContainWindow
		if inPress {
			window = e.cfg.Press.Window
		}
		if best >= 0 && (bt <= window || ub < ts.EngageDepth) {
			st.fd, st.fdCarrier = best+1, c
			used[best] = true
			jobs = append(jobs, teamShapeJob{composeRequest{Player: best, Class: composeBall, Target: press, Priority: 10,
				Owner: TargetOwner_TSC, Reason: TargetReason_M1_FIRST_DEFENDER}, PlayerIntent_PRESS_BALL, true, -1})
			cover := toward(press, goal, ts.CoverGap)
			cb, ct := -1, math.Inf(1)
			defenders(func(i int, p *MatchPlayer) bool {
				t := e.bodyTimeTo(p, feM1ToM(Vector2F{cover[0] / Pitch_LENGTH_METRES, cover[1] / Pitch_WIDTH_METRES}), 1.0)
				if t < ct {
					cb, ct = i, t
				}
				return true
			})
			if cb >= 0 {
				used[cb] = true
				jobs = append(jobs, teamShapeJob{composeRequest{Player: cb, Class: composeBall, Target: cover, Priority: 9,
					Owner: TargetOwner_TSC, Reason: TargetReason_M1_COVER}, PlayerIntent_COVER_PRESS, false, -1})
			}
			// in the press: a second presser takes the carrier's nearest short option
			// (goal-side of him), so the press is not one man chasing passes
			if inPress {
				opt, od := -1, ts.PressOption
				for j := range e.players {
					o := &e.players[j]
					if o.IsHomeTeam == home || !active(o) || o.IsGoalkeeper || o == c {
						continue
					}
					if d := math.Hypot(pm(o)[0]-cp[0], pm(o)[1]-cp[1]); d < od {
						opt, od = j, d
					}
				}
				if opt >= 0 {
					shadow := toward(pm(&e.players[opt]), goal, 2)
					pb, pt := -1, math.Inf(1)
					defenders(func(i int, p *MatchPlayer) bool {
						t := e.bodyTimeTo(p, feM1ToM(Vector2F{shadow[0] / Pitch_LENGTH_METRES, shadow[1] / Pitch_WIDTH_METRES}), 1.0)
						if t < pt {
							pb, pt = i, t
						}
						return true
					})
					if pb >= 0 && pt <= e.cfg.Press.Window {
						used[pb] = true
						jobs = append(jobs, teamShapeJob{composeRequest{Player: pb, Class: composeBall, Target: shadow, Priority: 8,
							Owner: TargetOwner_TSC, Reason: TargetReason_M1_SCREEN}, PlayerIntent_COVER_PRESS, true, -1})
					}
				}
			}
		} else {
			st.fd = 0
		}
	} else {
		st.fd = 0
	}
	// 2. ball-oriented marking: a goal-side marker for each attacker near the ball (and,
	// with the ball near the goal, for each attacker in the danger zone), taken by the
	// slot-holder nearest him; the zone (slot) keeps everyone else. A truly loose ball
	// (no carrier, no pass) is the loose-ball contest's: markers do not follow their men
	// into it, the block holds.
	loose := e.findMatchPlayer(e.ball.PossessedBy) == nil && !e.ball.IsPass && !e.ball.IsShot
	if !loose || ub < ts.DangerDepth {
		type att struct {
			i      int
			d      float64
			danger bool
		}
		var atts []att
		for i := range e.players {
			o := &e.players[i]
			if o.IsHomeTeam == home || !active(o) || o.IsGoalkeeper || o.Player == nil || o.Player == e.ball.PossessedBy {
				continue
			}
			op := pm(o)
			u, _ := fr.toFrame(op[0], op[1])
			danger := ub < ts.DangerDepth && u < ts.MarkZone+5
			near := ts.NearBall
			if ub-Pitch_LENGTH_METRES/2 >= e.cfg.Press.Depth {
				near = ts.PressMarkRadius // a high press picks up the build-up's options man to man
			}
			if danger || math.Hypot(op[0]-bx, op[1]-by) < near {
				atts = append(atts, att{i, math.Hypot(op[0]-goal[0], op[1]-goal[1]), danger})
			}
		}
		sort.Slice(atts, func(a, b int) bool { return atts[a].d < atts[b].d || atts[a].d == atts[b].d && atts[a].i < atts[b].i })
		for _, a := range atts {
			op := pm(&e.players[a.i])
			mark := toward(op, goal, ts.MarkGoalSide)
			au, _ := fr.toFrame(op[0], op[1])
			best, bd := -1, ts.MarkRadius
			defenders(func(i int, p *MatchPlayer) bool {
				// the marker comes from goal-side of the man (a bypassed player does not
				// run back to mark: he would be chasing the play)
				pp := pm(p)
				if pu, _ := fr.toFrame(pp[0], pp[1]); pu > au+ts.MarkBehind {
					return true
				}
				if d := math.Hypot(slots[i][0]-op[0], slots[i][1]-op[1]); d < bd {
					best, bd = i, d
				}
				return true
			})
			if best >= 0 {
				used[best] = true
				jobs = append(jobs, teamShapeJob{composeRequest{Player: best, Class: composeBall, Target: mark, Priority: 5,
					Owner: TargetOwner_TSC, Reason: TargetReason_M1_MARKER}, PlayerIntent_MARK_OPPONENT, a.danger, a.i})
			}
		}
	}
	// 3. a back-line slot-holder drops with a runner going in behind him
	si := hBoolIndex(!home)
	for kk := 1; kk < 11; kk++ {
		i := fr.base + kk
		if i >= len(e.players) || used[i] || !ok[i] {
			continue
		}
		if slot := st.assign[si][kk]; slot < 1 || slot > 4 {
			continue // back-four slots only
		}
		su, sw := fr.toFrame(slots[i][0], slots[i][1])
		drop := su
		for j := range e.players {
			o := &e.players[j]
			if o.IsHomeTeam == home || !active(o) || o.IsGoalkeeper {
				continue
			}
			op := pm(o)
			ou, ow := fr.toFrame(op[0], op[1])
			vx := o.Velocity.X // m/s along the pitch length
			if fr.attack < 0 {
				vx = -vx
			}
			// running at our goal (team frame: decreasing u), level with or beyond the slot,
			// in the player's channel
			if -vx > ts.RunnerSpeed && ou < su+2 && math.Abs(ow-sw) < ts.MarkRadius {
				drop = math.Min(drop, ou-2)
			}
		}
		if drop < su-0.5 {
			x, y := fr.fromFrame(math.Max(drop, 6), sw)
			used[i] = true
			jobs = append(jobs, teamShapeJob{composeRequest{Player: i, Class: composeOffset, Target: [2]float64{x, y}, MaxDev: ts.DepthDev,
				Owner: TargetOwner_TSC, Reason: TargetReason_M1_PROTECT_DEPTH}, PlayerIntent_RECOVER_SHAPE, true, -1})
		}
	}
	return jobs
}
