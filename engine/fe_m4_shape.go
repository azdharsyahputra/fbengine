// Ported from FootballEngine FootballEngine.Core/Match/Systems/Team/Position.fs,
// Systems/Team/FieldEvaluator.fs and Systems/Team/InterceptTable.fs (GPL-3.0).
//
// M4 (FE_PORT_PLAN §2 M4):
//   - the shape anchor is the ball's inertia point at the earliest control
//     step of anyone on the pitch (Position.fs:166-171, InterceptTable.fs
//     ballInertiaPoint / stepsToReach, plan §0.3 P3-P5), so both blocks shift
//     toward where a pass will be controlled instead of chasing the ball in
//     flight;
//   - FieldEvaluator.refineInto (FieldEvaluator.fs:182-221): each in-possession
//     holder / supporter picks the best of his base target, four points 4 m
//     around it and a spot 1 m behind the offside line, scored on spread from
//     team-mates, distance from opponents, progress and openness; refreshed
//     in two staggered groups (RefineGroups 2, Position.fs:255-262), then the
//     onside clamp and the 2 m pitch inset (Position.fs:297-309).
//     Constants: BalanceConfig.fs:1258-1280 (Positioning), :1282-1289 (Intercept).
//
// Ours (Go side, not FE): the No.10 "between the lines" candidate and lane
// term (plan M4 Go extension), the offside-line candidate restricted to
// forwards, a small hysteresis bonus for the candidate chosen last time
// (anti-jitter), and the ball-side full-back push.
// FootballEngine: https://github.com/DeltaBitsSystem/FootballEngine (GPL-3.0).
package engine

import (
	"math"
	"sort"
)

const (
	// Ball roll (InterceptTable.fs:60-90; plan §0.3 P3).
	feM4Dt             = 0.025
	feM4RollDecel      = 0.1 * 9.81 // RollingResistance·g
	feM4DragK          = 0.05304    // ρπr²/(2m)
	feM4MaxSteps       = 50
	feM4ControlRadius  = 2.5
	feM4BodyAcc        = 3.2 // Acceleration 4.0 · Smoothing 0.8
	feM4BodyTop        = 8.5 // SprintSpeed
	feM4MovingBallMins = 0.5 // m/s: a slower free ball is anchored where it is

	// FieldEvaluator (BalanceConfig.fs:1258-1268).
	feM4TeammateWeight  = 0.3
	feM4OpponentWeight  = 0.5
	feM4GoalWeight      = 0.4
	feM4OppOurWeight    = 0.25
	feM4SearchRadius    = 4.0
	feM4CandidateCount  = 5
	feM4MinTeammateDist = 5.0
	feM4RefineGroups    = 2
	feM4OffsideStep     = 1.0 // candidate 1 m behind the offside line
	feM4TargetInset     = 2.0

	// Ours.
	feM4KeepBonus        = 0.03 // the candidate chosen last time keeps the job
	feM4OffsideReach     = 12.0 // forwards only consider a line this close (m)
	feM4LaneWeight       = 0.3  // No.10: lane from the carrier clear
	feM4LaneRadius       = 2.0  // AttackGeometry.lane radius (CollectiveAttack.fs:58-79)
	feM4BetweenBonus     = 0.12 // No.10: the pocket between the lines
	feM4BetweenMinGap    = 6.0  // lines closer than this leave no pocket (m)
	feM4BetweenHalfWidth = 16.0 // the No.10 pocket stays inside the central lanes
)

// feM4Slot is the per-slot evaluator memory.
type feM4Slot struct {
	occupant *Player
	delta    feM1Vec // chosen point - base (team frame, metres)
	choice   int
	ready    bool
}

type feM4State struct {
	slots   [b3Lanes]feM4Slot
	refresh uint64
	// The ball's predicted points (metres) for this refresh.
	ballPts [feM4MaxSteps + 1]feM1Vec
	anchor  Vector2F
	// The ball-side full-back pushing on this refresh: M3 steers him at his
	// own target instead of through the back line's common shift.
	pushed [b3Lanes]bool
}

// feM4Pushed reports whether the full-back push moved this player's target
// at the last refresh (read by feM3InUnit).
func (e *MatchEngine) feM4Pushed(player *MatchPlayer) bool {
	if !e.cfg.Modules.M4Shape {
		return false
	}
	slot := e.slotOf(player)
	return slot < b3Lanes && e.feM4().pushed[slot]
}

func (e *MatchEngine) feM4() *feM4State {
	if len(e.feM1Data.shape) != len(e.players) {
		e.initFeState()
	}
	return &e.feM4Data
}

// feM4NextGroundSpeed is FE nextGroundSpeed (plan §0.3 P3).
func feM4NextGroundSpeed(v float64) float64 {
	cd := 0.2 + (0.45-0.2)/(1.0+math.Exp((v-12.0)/1.5))
	return math.Max(0.0, v-(feM4RollDecel+feM4DragK*cd*v*v)*feM4Dt)
}

// feM4BodyReach is FE reachableDistance (plan §0.3 P4).
func feM4BodyReach(t float64) float64 {
	ramp := feM4BodyTop / feM4BodyAcc
	if t <= ramp {
		return 0.5 * feM4BodyAcc * t * t
	}
	return feM4BodyTop*t - feM4BodyTop*ramp*0.5
}

// feM4ShapeAnchor is the point the blocks shift around (plan M4(d)): the
// carrier while the ball is held, otherwise ballInertiaPoint(ball, BallStep)
// with BallStep = min stepsToReach over every player on the pitch.
func (e *MatchEngine) feM4ShapeAnchor(carrier *MatchPlayer, pressure Vector2F) Vector2F {
	st := e.feM4()
	st.refresh++
	st.anchor = pressure
	st.pushed = [b3Lanes]bool{}
	if !e.cfg.Modules.M4Shape || carrier != nil {
		return pressure
	}
	speed := length(e.ball.Velocity)
	if speed < feM4MovingBallMins {
		return pressure
	}
	// Decelerate, then move (InterceptTable.ballInertiaPoint).
	p := feM1ToM(e.ball.Position)
	vx, vy := e.ball.Velocity.X, e.ball.Velocity.Y
	st.ballPts[0] = p
	for n := 1; n <= feM4MaxSteps; n++ {
		s := math.Hypot(vx, vy)
		if s > 0 {
			ns := feM4NextGroundSpeed(s)
			vx, vy = vx*ns/s, vy*ns/s
		}
		p.X += vx * feM4Dt
		p.Y += vy * feM4Dt
		st.ballPts[n] = p
	}
	// stepsToReach (plan §0.3 P5) for everyone; BallStep is the earliest.
	ballStep := feM4MaxSteps
	for i := range e.players {
		pl := &e.players[i]
		if !active(pl) {
			continue
		}
		pos := feM1ToM(pl.Position)
		for n := 0; n < ballStep; n++ {
			if feM1Dist(pos, st.ballPts[n]) <= feM4ControlRadius+feM4BodyReach(float64(n)*feM4Dt) {
				ballStep = n
				break
			}
		}
	}
	ballStep = max(ballStep, 1)
	a := feM1ToPitch(st.ballPts[ballStep])
	st.anchor = Vector2F{hClamp(a.X, 0.0, 1.0), hClamp(a.Y, 0.0, 1.0)}
	return st.anchor
}

// feM4Lines returns the opponents' back line and midfield line (team frame
// x of the side attacking, metres): the mean of their four deepest
// outfielders and of the next three. ok is false without enough players.
func (e *MatchEngine) feM4Lines(home bool) (back, mid float64, ok bool) {
	dir := feM1Dir(home)
	var xs [16]float64
	n := 0
	for i := range e.players {
		p := &e.players[i]
		if p.IsHomeTeam == home || !feM1Outfielder(p) || n >= len(xs) {
			continue
		}
		xs[n] = feM1ToM(p.Position).X * dir
		n++
	}
	if n < 7 {
		return 0, 0, false
	}
	v := xs[:n]
	sort.Sort(sort.Reverse(sort.Float64Slice(v)))
	for i := 0; i < 4; i++ {
		back += v[i]
	}
	for i := 4; i < 7; i++ {
		mid += v[i]
	}
	return back / 4.0, mid / 3.0, true
}

// feM4LaneClear is FE AttackGeometry.lane (CollectiveAttack.fs:58-79): no
// opponent outfielder within the radius of the segment (FE metres, world).
func (e *MatchEngine) feM4LaneClear(home bool, from, to feM1Vec, radius float64) bool {
	dx, dy := to.X-from.X, to.Y-from.Y
	lengthSq := dx*dx + dy*dy
	if lengthSq <= 0 {
		return false
	}
	for i := range e.players {
		p := &e.players[i]
		if p.IsHomeTeam == home || !feM1Outfielder(p) {
			continue
		}
		o := feM1ToM(p.Position)
		ox, oy := o.X-from.X, o.Y-from.Y
		f := hClamp((ox*dx+oy*dy)/lengthSq, 0.0, 1.0)
		ex, ey := ox-f*dx, oy-f*dy
		if ex*ex+ey*ey < radius*radius {
			return false
		}
	}
	return true
}

func feM4IsForward(role PlayerRole) bool {
	return role == PlayerRole_ST || role == PlayerRole_CAM || role == PlayerRole_LW ||
		role == PlayerRole_RW || role == PlayerRole_LM || role == PlayerRole_RM
}

// feM4Refine is FieldEvaluator.refineInto plus the Position.fs clamps for an
// in-possession holder or supporter whose base target is base (pitch units).
func (e *MatchEngine) feM4Refine(player *MatchPlayer, base Vector2F, carrier *MatchPlayer, offsideLine float64) Vector2F {
	if !e.cfg.Modules.M4Shape || player.IsGoalkeeper {
		return base
	}
	st := e.feM4()
	slot := e.slotOf(player)
	if slot >= len(st.slots) {
		return base
	}
	s := &st.slots[slot]
	if s.occupant != player.Player {
		*s = feM4Slot{occupant: player.Player}
	}
	home := player.IsHomeTeam
	dir := feM1Dir(home)
	bm := feM1ToM(base)
	bx, by := bm.X*dir, bm.Y // team frame
	ballX := feM1ToM(e.ball.Position).X * dir
	offX := feM1ToM(Vector2F{offsideLine, 0.5}).X * dir
	legalX := math.Max(0.0, math.Max(offX, ballX)) - feM4OffsideStep

	due := !s.ready || int(st.refresh%feM4RefineGroups) == slot%feM4RefineGroups
	if due {
		var cand [24]feM1Vec
		var bonus [24]float64
		n := 0
		cand[n] = feM1Vec{bx, by}
		n++
		for k := 1; k < feM4CandidateCount; k++ {
			ang := 2.0 * math.Pi * float64(k-1) / float64(feM4CandidateCount-1)
			cand[n] = feM1Vec{bx + feM4SearchRadius*math.Cos(ang), by + feM4SearchRadius*math.Sin(ang)}
			n++
		}
		role := player.Player.GetRole()
		// FE adds the spot 1 m behind the offside line for everyone; here
		// only forwards within reach of the line consider it (ours).
		if feM4IsForward(role) && offX-feM4OffsideStep > bx && offX-feM4OffsideStep-bx < feM4OffsideReach {
			cand[n] = feM1Vec{offX - feM4OffsideStep, by}
			n++
		}
		no10 := (role == PlayerRole_CAM || role == PlayerRole_ST) && carrier != nil && carrier != player
		if no10 {
			// Go extension (plan M4): the pocket between the opponent's
			// midfield and back lines, in the receiver's lane.
			if back, mid, ok := e.feM4Lines(home); ok && back-mid >= feM4BetweenMinGap {
				py := by
				if role == PlayerRole_CAM {
					py = hClamp(by, -feM4BetweenHalfWidth, feM4BetweenHalfWidth)
				}
				cand[n] = feM1Vec{(back + mid) * 0.5, py}
				bonus[n] = feM4BetweenBonus
				n++
			}
		}
		var carrierM feM1Vec
		if carrier != nil {
			carrierM = feM1ToM(carrier.Position)
		}
		// Getting open (Phase 2: our M10 rule folded into the refinement, no
		// longer a later overwrite): an attacker ahead of the ball whose lane
		// is shut or who is tight to a defender also weighs the spots 3-6 m
		// across or short, and the clear lane is scored for him.
		openSeek := e.feM10SeeksOpen(player, carrier, carrierM, feM1Vec{bx * dir, by})
		if openSeek {
			for _, st := range feM10OpenSteps {
				c := feM1Vec{bx + st[0], by + st[1]}
				if c.X > legalX || c.X-carrierM.X*dir < 1.0 || math.Abs(c.Y) > feM1HalfWidth-feM4TargetInset {
					continue
				}
				cand[n] = c
				n++
			}
		}
		best, bestScore := 0, math.Inf(-1)
		for i := 0; i < n; i++ {
			c := cand[i]
			score := e.feM4Score(player, home, dir, c) + bonus[i]
			if no10 || openSeek {
				w := feM1Vec{c.X * dir, c.Y}
				if e.feM4LaneClear(home, carrierM, w, feM4LaneRadius) {
					score += feM4LaneWeight
				}
			}
			if s.ready && i == s.choice {
				score += feM4KeepBonus
			}
			if score > bestScore {
				best, bestScore = i, score
			}
		}
		s.choice = best
		s.delta = feM1Vec{cand[best].X - bx, cand[best].Y - by}
		s.ready = true
	}
	x, y := bx+s.delta.X, by+s.delta.Y
	// Onside (plan M4(b), Position.fs:297-309 with CollectiveAttack legalTarget's
	// ball / halfway legality) and the pitch inset.
	x = math.Min(x, legalX)
	x = hClamp(x, -(feM1HalfLength - feM4TargetInset), feM1HalfLength-feM4TargetInset)
	y = hClamp(y, -(feM1HalfWidth - feM4TargetInset), feM1HalfWidth-feM4TargetInset)
	p := feM1ToPitch(feM1Vec{x * dir, y})
	return Vector2F{hClamp(p.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
		hClamp(p.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
}

// feM4Score is FieldEvaluator.scoreAtSlots (FieldEvaluator.fs:79-109) in the
// team frame (c), team-mates and opponents at their current positions.
func (e *MatchEngine) feM4Score(player *MatchPlayer, home bool, dir float64, c feM1Vec) float64 {
	dOur, dOpp := math.MaxFloat64, math.MaxFloat64
	for i := range e.players {
		p := &e.players[i]
		if !active(p) || p == player {
			continue
		}
		m := feM1ToM(p.Position)
		d := math.Hypot(m.X*dir-c.X, m.Y-c.Y)
		if p.IsHomeTeam == home {
			dOur = math.Min(dOur, d)
		} else {
			dOpp = math.Min(dOpp, d)
		}
	}
	spread := hClamp((dOur-feM4MinTeammateDist)/math.Max(1.0, feM4SearchRadius), 0.0, 1.0)
	avoid := 1.0
	if dOpp < math.MaxFloat64 {
		avoid = hClamp(dOpp/(2.0*feM4SearchRadius), 0.0, 1.0)
	}
	progress := hClamp(0.5+c.X/(2.0*feM1HalfLength), 0.0, 1.0)
	openness := hClamp((dOpp-dOur)/(2.0*math.Max(1.0, feM4SearchRadius)), -1.0, 1.0)
	return feM4TeammateWeight*spread + feM4OpponentWeight*avoid +
		feM4GoalWeight*progress + feM4OppOurWeight*openness
}

// Ball-side full-back push (ours, Go side of M4): with his side on the ball
// in the opponent half on his flank, a full-back (not an inverted one)
// holds width and steps up to a few metres behind the ball, so overlaps can
// start from there; the far-side full-back stays as rest defence.
const (
	feM4FullBackStart     = -10.0 // carrier depth (team frame, m) where the push begins
	feM4FullBackRamp      = 10.0  // metres over which it reaches full strength
	feM4FullBackBehind    = 8.0   // metres behind the carrier
	feM4FullBackTouchline = 4.0   // metres inside the touchline
	feM4FullBackSideMin   = 4.0   // carrier this far off-centre is on a flank
	feM4FullBackLevelRamp = 18.0  // metres past the start where he is Ahead of the ball
	feM4FullBackAhead     = 3.0   // metres in front of the ball at full push
	// Possession compaction (ours, Go side of M4): settled wide forwards and
	// the No.10 hold at most this far ahead of the ball, and the midfield
	// pivot no further than this behind it, so the DM-AM gap stays 12-15 m
	// (spec §0.2) and the striker is the deepest-running forward.
	feM4WingerReach = 8.0
	feM4No10Reach   = 7.0
	feM4PivotTrail  = 6.0
	feM4No10CutBack = 3.0
	feM4StrikerLead = 10.0
	feM4FinalReachX = 30.0 // team-frame x (m from halfway) where the reach is gone
)

func (e *MatchEngine) feM4FullBackPush(player *MatchPlayer, target Vector2F, carrier *MatchPlayer) Vector2F {
	if !e.cfg.Modules.M4Shape || carrier == nil || player.Player == nil {
		return target
	}
	role := player.Player.GetRole()
	if (role != PlayerRole_LB && role != PlayerRole_RB) || e.roleProfileOf(player).OverlapBias <= -1.0 {
		return target
	}
	dir := feM1Dir(player.IsHomeTeam)
	c := feM1ToM(carrier.Position)
	base := feM1ToM(player.BasePosition)
	if c.Y*base.Y <= 0 || math.Abs(c.Y) < feM4FullBackSideMin {
		return target
	}
	share := hClamp((c.X*dir-feM4FullBackStart)/feM4FullBackRamp, 0.0, 1.0)
	if share <= 0 {
		return target
	}
	t := feM1ToM(target)
	tx := t.X * dir
	// He closes from Behind metres behind the ball to Ahead metres in front
	// of it as the ball goes from the push start toward the final third.
	behind := feM4FullBackBehind - (feM4FullBackBehind+feM4FullBackAhead)*
		hClamp((c.X*dir-feM4FullBackStart)/feM4FullBackLevelRamp, 0.0, 1.0)
	if want := c.X*dir - behind; want > tx {
		tx += (want - tx) * share
	}
	if slot := e.slotOf(player); slot < b3Lanes {
		e.feM4().pushed[slot] = true
	}
	wide := math.Copysign(feM1HalfWidth-feM4FullBackTouchline, base.Y)
	ty := t.Y + (wide-t.Y)*share
	p := feM1ToPitch(feM1Vec{tx * dir, ty})
	return Vector2F{hClamp(p.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
		hClamp(p.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
}

// feM4Compact is the possession compaction (ours, Go side of M4) for a
// settled holder or supporter: wingers and the No.10 do not hold more than
// a few metres ahead of the ball, the pivot does not trail it by more than
// PivotTrail once the ball is past halfway, and a holding striker stays at
// least StrikerLead ahead of it (the onside clamp follows in feM4Refine), so
// he is the furthest forward.
func (e *MatchEngine) feM4Compact(player *MatchPlayer, target Vector2F, carrier *MatchPlayer) Vector2F {
	if !e.cfg.Modules.M4Shape || carrier == nil || player.Player == nil {
		return target
	}
	dir := feM1Dir(player.IsHomeTeam)
	ballX := feM1ToM(carrier.Position).X * dir
	t := feM1ToM(target)
	tx := t.X * dir
	// In the final third the reach shrinks to nothing at FinalReachX: the
	// wide men and the No.10 arrive level with the ball, not beyond it.
	reach := 1.0 - hClamp(ballX/feM4FinalReachX, 0.0, 1.0)
	switch player.Player.GetRole() {
	case PlayerRole_LW, PlayerRole_RW, PlayerRole_LM, PlayerRole_RM:
		tx = math.Min(tx, ballX+feM4WingerReach*reach)
	case PlayerRole_CAM:
		// Deep in the final third the No.10 waits at the edge of the box,
		// a few metres behind the ball, for the cut-back.
		tx = math.Min(tx, ballX+feM4No10Reach*reach-feM4No10CutBack*(1.0-reach))
	case PlayerRole_CDM, PlayerRole_CM:
		if ballX > -feM4PivotTrail {
			tx = math.Max(tx, ballX-feM4PivotTrail)
		}
	case PlayerRole_ST:
		if player.Intent == PlayerIntent_HOLD_SHAPE {
			tx = math.Max(tx, ballX+feM4StrikerLead)
		}
	default:
		return target
	}
	p := feM1ToPitch(feM1Vec{tx * dir, t.Y})
	return Vector2F{hClamp(p.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X), target.Y}
}

// Byline guard (ours): a wide carrier near the byline turns in toward the
// box instead of dribbling on into the corner flag, where a carrier with no
// option left stands still (the ball-side full-back push sends more
// carriers down the flank).
const (
	feM4BylineRoom     = 8.0  // metres from the goal line where he turns in
	feM4BylineHalfWide = 20.0 // |y| (m) beyond which he is wide of the box
	feM4BylineInward   = 6.0  // metres inward per refresh target
)

func (e *MatchEngine) feM4BylineGuard(player *MatchPlayer, target Vector2F) Vector2F {
	dir := feM1Dir(player.IsHomeTeam)
	p := feM1ToM(player.Position)
	room := feM1HalfLength - p.X*dir
	if room > feM4BylineRoom || math.Abs(p.Y) < feM4BylineHalfWide {
		return target
	}
	t := feM1ToM(target)
	t.X = math.Min(t.X*dir, feM1HalfLength-feM4BylineRoom*0.5) * dir
	t.Y = p.Y - math.Copysign(feM4BylineInward, p.Y)
	q := feM1ToPitch(t)
	return Vector2F{hClamp(q.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
		hClamp(q.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
}
