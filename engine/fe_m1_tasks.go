// Ported from FootballEngine FootballEngine.Core/Match/Systems/Team/Position.fs (GPL-3.0)
//
// M1 defensive tasks: organised defence (FE `assignOrganizedDefense`,
// Position.fs:602-714), `assignDefensiveTasks` (:340-600: ProtectDepth,
// FirstDefender, Cover, Screens, Contain, RecoverShape) and the role → intent
// mapping of FE Systems/Players/Intent.fs:286-304. Go adaptations, marked in
// place: a Contain fallback in midfield (plan §4 risk 3), P12's zonal/man
// marking (assignMarks, P12 match_engine.cpp:3105-3140) kept as an offset on
// RecoverShape with markers near the own goal held goal-side and in the box,
// and P12's tight-mark order (P12:3084-3090) kept as an override.
// FootballEngine: https://github.com/DeltaBitsSystem/FootballEngine (GPL-3.0).
package engine

import (
	"math"
	"sort"
)

// Go-side box-marking constants (not FE): a marker whose man is inside this
// depth from the own goal line marks him tightly and goal-side.
const (
	feM1BoxMarkDepth     = 18.0 // metres from the own goal line
	feM1BoxMarkHalfWidth = 22.0
	feM1BoxMarkGoalSide  = 1.5 // metres toward the goal centre
	feM1BoxEdgeDepth     = 16.5
	// Box-marker choice: metres of cost credited to P12's assignMarks pairing
	// and to the previous marker of the same man.
	feM1BoxMarkAssignBonus = 4.0
	feM1BoxMarkKeepBonus   = 3.0
	feM1BoxMarkLead        = 0.5 // seconds of the man's run a marker anticipates
	// Back-four line and width while the box is defended: the line sits at
	// a share of the ball's depth, between Min and Depth metres from the own
	// goal line, from a ball depth of BoxDefendDepth (team frame) inward.
	feM1BoxLineDepth       = 16.0
	feM1BoxLineMin         = 10.0
	feM1BoxLineShare       = 0.47
	feM1BoxDefendDepth     = 36.0
	feM1BoxDefendHalfWidth = 26.0
	feM1BoxLineHalfWidth   = 18.0
	// Shape roles hold the six-yard line at the deepest; the retreat gap
	// shrinks to this near goal.
	feM1SixYardDepth  = 5.5
	feM1MinRetreatGap = 2.0
	// Second defender behind the Contain near goal (m toward the goal centre).
	feM1EngageCoverDepth = 4.0
	// The ball counts as pressured with a FirstDefender/Contain goal-side
	// within this distance (m).
	feM1PressureRadius = 3.0
	// Pass interception race: 0.025 s roll steps (3 s) and how close the ball
	// must come to the receiver for him to have it first (FE FootReach 0.7 m
	// plus a stride).
	feM1InterceptSteps         = 120
	feM1InterceptReceiverReach = 1.5
	// A RecoverShape player farther than this from his target runs back
	// (RECOVER_SHAPE) even in organised defence.
	feM1RecoverDistance = 8.0
	// Go addition: how fast the retreat reference steps back up (m/s).
	feM1StepUpSpeed = 3.0
	// Go addition: organised shape roles shift up / sideways at most this (m/s).
	feM1ShiftSpeed = 2.5
	// Go addition: a shape role's target is at most this far ahead of him, so
	// a squeeze is a jog (P12 arrive braking gives about 3.3 m/s at 3 m).
	feM1StepUpLead = 3.0
	// Go addition: a block role more than DropTolerance metres upfield of
	// its target drops at an effort growing from DropMinEffort to a sprint
	// DropFullMetres further out (FE Regroup is all-or-nothing).
	feM1DropTolerance  = 1.5
	feM1DropFullMetres = 16.0
	feM1DropMinEffort  = 0.15
	// Go addition: Regroup effort of a forward (AM / winger / striker) who is
	// still goal-side of the ball.
	feM1ForwardRegroupEffort = 0.35
	// Go addition (running volume): a recovery is a full sprint only when
	// the player is truly out of position, i.e. the ball is goal-side of him
	// or he is more than TrulyOutMetres upfield of his spot; otherwise he
	// gets back at an effort growing from RegroupMinEffort over
	// RegroupFullMetres.
	feM1TrulyOutMetres    = 15.0
	feM1RegroupMinEffort  = 0.35
	feM1RegroupFullMetres = 20.0
	// Regroup effort floor with the ball within BypassedSprintGoalMetres.
	feM1RegroupNearGoalEffort = 0.6
	// A bypassed forward sprints flat out only with the ball this close to
	// his own goal (m); otherwise ForwardBypassedEffort.
	feM1BypassedSprintGoalMetres = 40.0
	feM1ForwardBypassedEffort    = 0.6
)

// feM1Forward: an attacking role (AM, winger, wide midfielder, striker).
func feM1Forward(p *MatchPlayer) bool {
	if p.Player == nil {
		return false
	}
	switch p.Player.GetRole() {
	case PlayerRole_CAM, PlayerRole_LM, PlayerRole_RM, PlayerRole_LW, PlayerRole_RW, PlayerRole_ST:
		return true
	}
	return false
}

// feM1Assign runs after refreshTacticalTargets' per-player loop: it updates
// the episode, assigns the tasks and writes each driven player's target.
func (e *MatchEngine) feM1Assign() {
	s := e.feM1Of()
	for _, home := range [2]bool{true, false} {
		side := &s.sides[hBoolIndex(home)]
		if !side.owns {
			continue
		}
		sliders := e.getEffectiveSliders(home)
		rest1, rest2 := e.feM1RestDefenders(home, side)
		e.feM1UpdateEpisode(home, side, sliders, rest1, rest2)
		if side.episode.open {
			e.feM1AssignTasks(home, side, s, sliders, side.episode.mode,
				side.episode.primary, side.episode.cover, rest1, rest2)
		} else {
			e.feM1Organised(home, side, s, sliders, rest1, rest2)
		}
		e.feM1Apply(home, side, s)
	}
}

// feM1Organised is FE `assignOrganizedDefense` (Position.fs:602-714).
func (e *MatchEngine) feM1Organised(home bool, side *feM1Side, s *feM1State,
	sliders StrategySliders, rest1, rest2 int) {
	previousPrimary, previousCover := -1, -1
	for slot, task := range side.tasks {
		if task.role == feM1RoleFirstDefender {
			previousPrimary = slot
		}
		if task.role == feM1RoleCover {
			previousCover = slot
		}
	}
	dir := feM1Dir(home)
	ball := side.ball
	tx := func(slot int) float64 { return feM1ToM(e.players[slot].Position).X * dir }
	eligible := func(slot int) bool {
		return slot != rest1 && slot != rest2 &&
			e.players[slot].Stamina >= feM1MinimumPressCondition &&
			tx(slot) <= ball.X*dir+feM1BeatenMargin
	}
	depthThreat := rest2 < 0
	if rest2 >= 0 {
		depthLine := tx(rest2)
		for slot := range e.players {
			p := &e.players[slot]
			if p.IsHomeTeam == home || !feM1Outfielder(p) {
				continue
			}
			pm := feM1ToM(p.Position)
			if pm.X*dir < depthLine-feM1DepthSafety && math.Abs(pm.Y) <= feM1CentralHalfWidth {
				depthThreat = true
			}
		}
	}
	// FE TeamDecide.pressGate (Decide.fs:8-9); P12 has no press-height slider,
	// so PressTriggerZone follows the pressing slider.
	pressGate := 17.5 - 70.0*sliders.Pressing
	primary, cover := -1, -1
	if sliders.Pressing >= feM1MinimumPressIntensity && ball.X*dir > pressGate && !depthThreat {
		p, arrival := e.feM1ChooseSlot(home, 1.0, feM1PressPoint(home, ball, side.vel),
			func(slot int) bool { return !eligible(slot) }, previousPrimary, feM1HandoffAdvantage)
		if p >= 0 && arrival <= feM1LocalArrivalWindow {
			c, coverArrival := e.feM1ChooseSlot(home, feM1CoverEffort, feM1CoverPoint(home, ball),
				func(slot int) bool { return !eligible(slot) || slot == p }, previousCover, feM1HandoffAdvantage)
			if c >= 0 && coverArrival <= feM1CoverArrivalWindow {
				primary, cover = p, c
			}
		}
	}
	mode := feM1Retreat
	if primary >= 0 {
		mode = feM1Counterpress
	}
	e.feM1AssignTasks(home, side, s, sliders, mode, primary, cover, rest1, rest2)
}

// feM1AssignTasks is FE `assignDefensiveTasks` (Position.fs:340-600).
func (e *MatchEngine) feM1AssignTasks(home bool, side *feM1Side, s *feM1State,
	sliders StrategySliders, mode feM1Mode, primary, cover, rest1, rest2 int) {
	dir := feM1Dir(home)
	ball := side.ball
	previous := make([]feM1Task, len(side.tasks))
	copy(previous, side.tasks)
	side.clearTasks()
	tasks := side.tasks

	ours := func(slot int) bool {
		p := &e.players[slot]
		return p.IsHomeTeam == home && feM1Outfielder(p)
	}
	shape := func(slot int) feM1Vec {
		if s.shapeSet[slot] {
			return s.shape[slot]
		}
		return feM1ToM(e.players[slot].Position)
	}
	assign := func(slot int, role feM1Role, target feM1Vec, opponent int) {
		inset := feM1TargetInset
		if role == feM1RoleFirstDefender {
			inset = 0.0
		}
		tasks[slot] = feM1Task{role: role, opponent: opponent, target: feM1Vec{
			hClamp(target.X, -feM1HalfLength+inset, feM1HalfLength-inset),
			hClamp(target.Y, -feM1HalfWidth+inset, feM1HalfWidth-inset)}}
	}

	ownGoal := feM1Vec{feM1OwnGoalX(home), 0}
	// Go addition: the shape roles shift at a jog. Their target may step up
	// toward the ball and slide sideways at most feM1ShiftSpeed (dropping back
	// stays immediate), so the block shuffles across and up together instead
	// of sprinting at every touch.
	now := e.getSimulatedSeconds()
	elapsed := now - side.assignTime
	if !side.assignSet {
		elapsed = math.Inf(1)
	}
	side.assignTime, side.assignSet = now, true
	shift := func(slot int, role feM1Role, target feM1Vec) feM1Vec {
		// A player well behind his spot steps up a few metres at a time.
		pm := feM1ToM(e.players[slot].Position)
		if (target.X-pm.X)*dir > feM1StepUpLead {
			target.X = pm.X + dir*feM1StepUpLead
		}
		prev := previous[slot]
		if prev.role != role || math.IsInf(elapsed, 1) {
			return target
		}
		step := feM1ShiftSpeed * elapsed
		if (target.X-prev.target.X)*dir > step {
			target.X = prev.target.X + dir*step
		}
		target.Y = hClamp(target.Y, prev.target.Y-step, prev.target.Y+step)
		return target
	}
	// boxPoint is the goal-side marking spot of an opponent in or near the
	// box: 1.5 m toward the goal centre, and no farther out than the box edge.
	boxPoint := func(o int) (feM1Vec, bool) {
		op := &e.players[o]
		// Track where the man is going (Go: compensates the steering's
		// target filter, like FE's 0.15 s press prediction).
		om := feM1ToM(op.Position)
		om.X += op.Velocity.X * feM1BoxMarkLead
		om.Y += op.Velocity.Y * feM1BoxMarkLead
		depth := (om.X - ownGoal.X) * dir
		if depth >= feM1BoxMarkDepth || math.Abs(om.Y) >= feM1BoxMarkHalfWidth {
			return feM1Vec{}, false
		}
		d := math.Max(feM1Dist(om, ownGoal), EPSILON)
		target := feM1Vec{om.X + (ownGoal.X-om.X)/d*e.feM10BoxGoalSide(),
			om.Y + (ownGoal.Y-om.Y)/d*e.feM10BoxGoalSide()}
		if (target.X-ownGoal.X)*dir > feM1BoxEdgeDepth && depth < feM1BoxEdgeDepth+2.0 {
			target.X = ownGoal.X + dir*feM1BoxEdgeDepth
		}
		return target, true
	}
	engage := feM1Dist(ball, ownGoal) < feM1EngageGoalDistance
	// Go adaptation: FE's 8 m retreat gap (and ProtectDepth's 9 m behind the
	// deepest runner) would put the whole side on the goal line once the
	// ball is in the box. Near goal the gap closes to half the ball's depth
	// beyond the six-yard line, and no shape role drops behind that line.
	refDepth := side.retreatRef - ownGoal.X*dir
	retreatGap := hClamp((refDepth-feM1SixYardDepth)*0.5, feM1MinRetreatGap, feM1RetreatBallGap)
	floor := func(target feM1Vec) feM1Vec {
		minDepth := math.Min(feM1SixYardDepth, math.Max(0, refDepth-1.0))
		if (target.X-ownGoal.X)*dir < minDepth {
			target.X = ownGoal.X + dir*minDepth
		}
		return target
	}
	// Go adaptation: with the ball inside the engage distance, or straight in
	// front of the box within feM1BoxDefendDepth, the back four (the shape
	// roles of CB/LB/RB) defend the box: their line is no farther out than
	// about half the ball's depth (box edge at most), and the full-backs tuck
	// in to the box width.
	boxDefend := engage || (refDepth < feM1BoxDefendDepth &&
		math.Abs(ball.Y) < feM1BoxDefendHalfWidth)
	boxDepth := hClamp(refDepth*feM1BoxLineShare, feM1BoxLineMin, feM1BoxLineDepth)
	boxLine := func(slot int, target feM1Vec) feM1Vec {
		if !boxDefend || e.players[slot].Player == nil {
			return target
		}
		switch e.players[slot].Player.GetRole() {
		case PlayerRole_CB, PlayerRole_LB, PlayerRole_RB:
			target.X = math.Min((target.X-ownGoal.X)*dir, boxDepth)*dir + ownGoal.X
			target.Y = hClamp(target.Y, -feM1BoxLineHalfWidth, feM1BoxLineHalfWidth)
		}
		return target
	}

	// Set once the engagers are known (see below).
	pressured := false
	// Our offside line in the team frame (P12 offsideLine, the second-last
	// defender); opponents deeper than it are offside.
	offsideLine := feM1ToM(Vector2F{e.offsideLine(!home), Pitch_CENTRE}).X * dir
	// 1. ProtectDepth for the two deepest (:398-444).
	protect := func(slot int) {
		if slot < 0 {
			return
		}
		if tasks[slot].role != feM1RoleNone {
			return // marking a man in the box (Go adaptation below)
		}
		if pressured {
			// Go adaptation: with the ball under pressure the rest defenders
			// hold the block line (and the offside line) instead of dropping
			// 9 m behind the deepest runner; FE's drop applies when the
			// carrier is free to play forward.
			assign(slot, feM1RoleProtectDepth, shift(slot, feM1RoleProtectDepth,
				floor(boxLine(slot, shape(slot)))), -1)
			return
		}
		runner := -1
		deepest := side.retreatRef
		for c := range e.players {
			o := &e.players[c]
			if o.IsHomeTeam == home || !feM1Outfielder(o) {
				continue
			}
			om := feM1ToM(o.Position)
			sameCorridor := om.Y >= ball.Y
			if slot == rest1 {
				sameCorridor = om.Y <= ball.Y
			}
			// Go adaptation: a runner standing offside is left to the trap.
			if sameCorridor && om.X*dir < deepest && om.X*dir >= offsideLine {
				runner, deepest = c, om.X*dir
			}
		}
		ownSide := 1.0
		if feM1ToM(e.players[slot].Position).Y < 0 {
			ownSide = -1.0
		}
		y := ball.Y + ownSide*feM1DepthSafety
		if runner >= 0 {
			y = (ball.Y + feM1ToM(e.players[runner].Position).Y) / 2.0
		}
		// Go adaptation: outside the engage distance the line gives the runner
		// DepthSafety (3 m), not FE's 9 m, so the offside line survives.
		gap := feM1DepthSafety + feM1CoverDepth
		if !engage {
			gap = feM1DepthSafety
		}
		x := math.Min(shape(slot).X*dir, deepest-gap)
		assign(slot, feM1RoleProtectDepth, shift(slot, feM1RoleProtectDepth, floor(boxLine(slot,
			feM1Vec{x * dir, hClamp(y, -feM1CentralHalfWidth, feM1CentralHalfWidth)}))), runner)
	}
	available := func(slot int) bool {
		return ours(slot) && slot != rest1 && slot != rest2 && slot != primary && slot != cover
	}
	press := feM1PressPoint(home, ball, side.vel)
	if engage {
		// Go adaptation: near goal "goal-side" is toward the goal centre, not
		// straight down the pitch.
		pred := feM1Vec{ball.X + side.vel.X*feM1CarrierPrediction, ball.Y + side.vel.Y*feM1CarrierPrediction}
		d := math.Max(feM1Dist(pred, ownGoal), EPSILON)
		press = feM1Vec{pred.X + (ownGoal.X-pred.X)/d*feM1ApproachOffset,
			pred.Y + (ownGoal.Y-pred.Y)/d*feM1ApproachOffset}
	}
	// Phase 2 (ours): the Contain holds ContainDistance goal-side of the
	// predicted ball (same construction as the press point); the M6 challenge
	// owns the decision to step in. ContainDistance == ApproachOffset is go4.
	containAt := press
	if cd := e.cfg.M1.ContainDistance; cd != feM1ApproachOffset {
		pred := feM1Vec{ball.X + side.vel.X*feM1CarrierPrediction, ball.Y + side.vel.Y*feM1CarrierPrediction}
		if engage {
			d := math.Max(feM1Dist(pred, ownGoal), EPSILON)
			containAt = feM1Vec{pred.X + (ownGoal.X-pred.X)/d*cd, pred.Y + (ownGoal.Y-pred.Y)/d*cd}
		} else {
			containAt = feM1Vec{hClamp(pred.X-dir*cd, -feM1HalfLength, feM1HalfLength),
				hClamp(pred.Y, -feM1HalfWidth, feM1HalfWidth)}
		}
	}

	// 2. Counterpress: FirstDefender, Cover, up to two Screens (:453-553).
	if mode == feM1Counterpress {
		if primary >= 0 {
			assign(primary, feM1RoleFirstDefender, press, -1)
		}
		if cover >= 0 {
			assign(cover, feM1RoleCover, feM1CoverPoint(home, ball), -1)
		}
		type outlet struct {
			slot        int
			dist, depth float64
		}
		var outlets []outlet
		for c := range e.players {
			o := &e.players[c]
			if o.IsHomeTeam == home || !active(o) {
				continue
			}
			om := feM1ToM(o.Position)
			outlets = append(outlets, outlet{c, feM1Dist(ball, om), om.X * dir})
		}
		sort.SliceStable(outlets, func(i, j int) bool {
			a, b := outlets[i], outlets[j]
			if a.dist != b.dist {
				return a.dist < b.dist
			}
			if a.depth != b.depth {
				return a.depth < b.depth
			}
			return a.slot < b.slot
		})
		screens := 0
		for _, o := range outlets {
			if screens >= feM1MaximumScreens {
				break
			}
			if e.players[o.slot].IsGoalkeeper || o.slot == side.carrierSlot ||
				o.dist > feM1MaxPassExecutionDist {
				continue
			}
			om := feM1ToM(e.players[o.slot].Position)
			target := feM1Vec{ball.X + (om.X-ball.X)*feM1ScreenFraction - dir*feM1ApproachOffset,
				ball.Y + (om.Y-ball.Y)*feM1ScreenFraction}
			prev := -1
			for slot := range previous {
				if available(slot) && previous[slot].role == feM1RoleScreen &&
					previous[slot].opponent == o.slot {
					prev = slot
					break
				}
			}
			chosen, t := e.feM1ChooseSlot(home, feM1ScreenEffort, target,
				func(slot int) bool { return !available(slot) || tasks[slot].role == feM1RoleScreen },
				prev, feM1HandoffAdvantage)
			if chosen >= 0 && t <= feM1LocalArrivalWindow {
				assign(chosen, feM1RoleScreen, target, o.slot)
				screens++
			}
		}
	}

	// 3. Contain (:555-584): not counterpressing and within 35 m of the own
	// goal. Go adaptation (plan §4 risk 3): with no FirstDefender anywhere,
	// a goal-side Contain engages the carrier in midfield too, so carriers do
	// not walk through an unopposed block.
	if mode != feM1Counterpress || primary < 0 {
		// FE gates this on engage (< 35 m); the fallback engages everywhere.
		prev := -1
		for slot := range previous {
			if previous[slot].role == feM1RoleContain {
				prev = slot
				break
			}
		}
		goalSide := func(slot int) bool {
			return feM1ToM(e.players[slot].Position).X*dir <= ball.X*dir+feM1BeatenMargin
		}
		// Go adaptation: inside the engage distance the rest defenders may
		// step out too; a carrier running at the centre-backs is met by one of
		// them, not escorted to goal while they guard depth.
		canEngage := func(slot int) bool {
			if engage {
				return ours(slot) && slot != primary && slot != cover && goalSide(slot)
			}
			return available(slot) && goalSide(slot)
		}
		contain, containArrival := e.feM1ChooseSlot(home, 1.0, press,
			func(slot int) bool { return !canEngage(slot) },
			prev, feM1HandoffAdvantage)
		if !engage && containArrival > e.cfg.M1.ContainWindow {
			// The midfield fallback engages only a carrier someone can reach
			// within the FE local window (2 s); otherwise the block holds.
			contain = -1
		}
		if contain >= 0 {
			assign(contain, feM1RoleContain, containAt, -1)
		}
		// Go adaptation: inside the engage distance the Contain gets a second
		// defender (FE's Cover, otherwise only in counterpress) between the
		// ball and the goal centre, so a carrier who beats one man meets the
		// next instead of an open path to goal.
		if contain >= 0 && engage {
			prevCover := -1
			for slot := range previous {
				if previous[slot].role == feM1RoleCover {
					prevCover = slot
					break
				}
			}
			toGoal := feM1Vec{ownGoal.X - ball.X, ownGoal.Y - ball.Y}
			d := math.Max(math.Hypot(toGoal.X, toGoal.Y), EPSILON)
			gap := math.Min(feM1EngageCoverDepth, d*0.5)
			point := feM1Vec{ball.X + toGoal.X/d*gap, ball.Y + toGoal.Y/d*gap}
			c, _ := e.feM1ChooseSlot(home, feM1CoverEffort, point,
				func(slot int) bool { return !canEngage(slot) || slot == contain },
				prevCover, feM1HandoffAdvantage)
			if c >= 0 {
				assign(c, feM1RoleCover, point, -1)
			}
		}
	}

	// Go adaptation, box defending: every opponent in or near the box gets a
	// goal-side man-marker, the nearest free defender, preferring P12's
	// assignMarks pairing and the previous marker (hysteresis). Rest defenders
	// may mark here instead of standing 9 m behind the deepest runner.
	// The box is defended from the engage distance, or straight in front of
	// it (boxDefend).
	if boxDefend {
		type man struct {
			slot   int
			point  feM1Vec
			danger float64
		}
		var men []man
		for o := range e.players {
			op := &e.players[o]
			if op.IsHomeTeam == home || !feM1Outfielder(op) || o == side.carrierSlot {
				continue
			}
			if point, ok := boxPoint(o); ok {
				men = append(men, man{o, point, feM1Dist(feM1ToM(op.Position), ownGoal)})
			}
		}
		sort.SliceStable(men, func(i, j int) bool { return men[i].danger < men[j].danger })
		for _, m := range men {
			best, bestCost := -1, math.Inf(1)
			for slot := range e.players {
				if !ours(slot) || tasks[slot].role != feM1RoleNone {
					continue
				}
				cost := feM1Dist(feM1ToM(e.players[slot].Position), m.point)
				if slot < len(e.markAssignments) && int(e.markAssignments[slot]) == m.slot {
					cost -= feM1BoxMarkAssignBonus
				}
				if previous[slot].role == feM1RoleMarker && previous[slot].opponent == m.slot {
					cost -= feM1BoxMarkKeepBonus
				}
				if cost < bestCost {
					best, bestCost = slot, cost
				}
			}
			if best >= 0 {
				assign(best, feM1RoleMarker, m.point, m.slot)
			}
		}
	}
	for slot := range e.players {
		if !ours(slot) {
			continue
		}
		if r := tasks[slot].role; r == feM1RoleFirstDefender || r == feM1RoleContain {
			pm := feM1ToM(e.players[slot].Position)
			if feM1Dist(pm, ball) <= feM1PressureRadius && pm.X*dir <= ball.X*dir {
				pressured = true
			}
		}
	}
	protect(rest1)
	protect(rest2)

	// 4. RecoverShape for every unassigned outfielder (:586-600), with P12's
	// marking as an offset before the goal-side clamp.
	width := math.Min(1.0, feM1RetreatWidthScale*(1.0+sliders.WidthUsage))
	for slot := range e.players {
		if !ours(slot) || tasks[slot].role != feM1RoleNone {
			continue
		}
		sh := shape(slot)
		target := feM1Vec{sh.X, sh.Y * width}
		mark := e.feM1MarkOf(slot, side.carrierSlot)
		if _, inBox := boxPoint(max(mark, 0)); mark >= 0 && boxDefend && inBox {
			mark = -1 // already picked up by a box marker
		}
		if mark >= 0 {
			// P12 zonal shading (P12:3105-3140).
			mm := feM1ToM(e.players[mark].Position)
			markDepth := (mm.X - ownGoal.X) * dir
			weight := Shape_BASE_MARK_WEIGHT + sliders.Compactness*Shape_COMPACTNESS_MARK_WEIGHT
			point := mm
			if 1.0-markDepth/Pitch_LENGTH_METRES >= Shape_TIGHT_MARK_DANGER_DEPTH {
				weight = math.Max(weight, e.feM10TightWeight())
				point.X -= dir * e.feM10GoalSideMark()
			}
			target.X += (point.X - target.X) * weight
			target.Y += (point.Y - target.Y) * weight
		}
		role := feM1RoleRecoverShape
		target.X = math.Min(target.X*dir, side.retreatRef-e.feM10RetreatGap(e.players[slot].Player, retreatGap)) * dir
		assign(slot, role, shift(slot, role, floor(boxLine(slot, target))), mark)
	}

	// Go adaptation: P12 keeps a flat back four. The back-line players on
	// shape duty (ProtectDepth / RecoverShape) share one depth, the mean of
	// their targets, so the rest defenders' FE depth moves the whole line.
	backLine := func(slot int) bool {
		p := e.players[slot].Player
		if !ours(slot) || p == nil {
			return false
		}
		r := tasks[slot].role
		role := p.GetRole()
		return (r == feM1RoleProtectDepth || r == feM1RoleRecoverShape) &&
			(role == PlayerRole_CB || role == PlayerRole_LB || role == PlayerRole_RB)
	}
	line, lineN := 0.0, 0
	for slot := range e.players {
		if backLine(slot) {
			line += tasks[slot].target.X * dir
			lineN++
		}
	}
	lineSet := lineN > 0
	if lineSet {
		line /= float64(lineN)
	}
	if lineSet {
		for slot := range e.players {
			if backLine(slot) {
				tasks[slot].target.X = line * dir
			}
		}
	}

	// Go adaptation: with the opponent's pass in flight, the player who can
	// cut it out goes for the ball (P12 chased every pass with its presser).
	if side.inFlight {
		if slot, point := e.feM1Interceptor(home, side); slot >= 0 {
			tasks[slot] = feM1Task{role: feM1RoleIntercept, target: point, opponent: -1}
		}
	}
}

// feM1Interceptor is the FE loose-ball chaser race (plan §0.3 P3/P5,
// InterceptTable.fs:60-90, 220-250) applied to the opponent's ground pass:
// the ball rolls on (FE turf and drag deceleration, 0.025 s steps) until it
// reaches the receiver; the first of our outfielders whose arrival time beats
// the ball to a point of that path cuts it out there. -1 when nobody can.
func (e *MatchEngine) feM1Interceptor(home bool, side *feM1Side) (int, feM1Vec) {
	const dt = 0.025
	pos := side.flightBall
	vx, vy := side.flightVel.X, side.flightVel.Y
	var receiver feM1Vec
	hasReceiver := side.carrierSlot >= 0
	if hasReceiver {
		receiver = feM1ToM(e.players[side.carrierSlot].Position)
	}
	for n := 1; n <= feM1InterceptSteps; n++ {
		speed := math.Hypot(vx, vy)
		if speed < 0.5 {
			break
		}
		cd := 0.2 + (0.45-0.2)/(1.0+math.Exp((speed-12.0)/1.5))
		next := math.Max(0.0, speed-(0.981+0.05304*cd*speed*speed)*dt)
		vx, vy = vx*next/speed, vy*next/speed
		pos = feM1Vec{pos.X + vx*dt, pos.Y + vy*dt}
		if hasReceiver && feM1Dist(pos, receiver) <= feM1InterceptReceiverReach {
			break
		}
		t := float64(n) * dt
		best, bestT := -1, math.Inf(1)
		for slot := range e.players {
			p := &e.players[slot]
			if p.IsHomeTeam != home || !feM1Outfielder(p) {
				continue
			}
			if at := e.feM1ArrivalTime(1.0, p, pos); at <= t && at < bestT {
				best, bestT = slot, at
			}
		}
		if best >= 0 {
			return best, pos
		}
	}
	return -1, feM1Vec{}
}

// feM1MarkOf is the opponent P12's assignMarks gave this slot, if any (and not
// the carrier, whom the press roles handle).
func (e *MatchEngine) feM1MarkOf(slot int, carrierSlot int) int {
	if slot >= len(e.markAssignments) {
		return -1
	}
	assigned := int(e.markAssignments[slot])
	if assigned < 0 || assigned >= len(e.players) || assigned == carrierSlot {
		return -1
	}
	m := &e.players[assigned]
	if !active(m) || m.IsGoalkeeper || m.IsHomeTeam == e.players[slot].IsHomeTeam {
		return -1
	}
	return assigned
}

// feM1Apply maps each task to a P12 intent and target (FE Intent.fs:286-304)
// and records the FE effort of the role (see feM1Effort).
func (e *MatchEngine) feM1Apply(home bool, side *feM1Side, s *feM1State) {
	transition := side.episode.open
	for slot := range e.players {
		side.effort[slot] = -1
		side.individual[slot] = false
		p := &e.players[slot]
		if p.IsHomeTeam != home || !feM1Outfielder(p) {
			continue
		}
		task := side.tasks[slot]
		target := feM1ToPitch(task.target)
		intent := PlayerIntent_HOLD_SHAPE
		pressing, urgent, direct := false, false, false
		far := distance(p.Position, target) > feM1RecoverDistance
		effort := -1.0
		individual := true
		switch task.role {
		case feM1RoleFirstDefender, feM1RoleContain:
			intent, pressing, urgent, direct = PlayerIntent_PRESS_BALL, true, true, true
			effort = 1.0
		case feM1RoleCover:
			intent, pressing = PlayerIntent_COVER_PRESS, true
			effort = feM1CoverEffort
		case feM1RoleScreen:
			intent = PlayerIntent_BLOCK_PASSING_LANE
			effort = feM1ScreenEffort
		case feM1RoleIntercept:
			intent, pressing, urgent, direct = PlayerIntent_CLAIM_LOOSE_BALL, true, true, true
			effort = 1.0
		case feM1RoleMarker:
			// Box marking keeps pace with the man (FE Mark 0.85; FE's jog is
			// 5 m/s, far above a Go jog, so it is not left to the phase).
			intent = PlayerIntent_MARK_OPPONENT
			effort = feM1ScreenEffort
		case feM1RoleProtectDepth, feM1RoleRecoverShape:
			// FE Regroup runs at full effort only while getting back: a player
			// upfield of his target in transition, or far upfield of it at any
			// time; settled players shuffle with the block.
			individual = false
			pm := feM1ToM(p.Position)
			behind := (pm.X - task.target.X) * feM1Dir(home)
			// Stepping up toward the ball is never a sprint.
			if behind > feM1SettledTolerance && (transition || far) {
				intent = PlayerIntent_RECOVER_SHAPE
				effort = 1.0
				// Go addition (running volume): a forward who is still
				// goal-side of the ball trots back; anyone the ball has passed
				// sprints, as does anyone far out of position; a defender or
				// midfielder a little upfield of his spot gets back at a
				// graded effort.
				goalSide := (pm.X-side.ball.X)*feM1Dir(home) < 0
				ballNearGoal := feM1Dist(side.ball, feM1Vec{feM1OwnGoalX(home), 0}) < feM1BypassedSprintGoalMetres
				if goalSide && feM1Forward(p) {
					effort = feM1ForwardRegroupEffort
				} else if !goalSide && feM1Forward(p) && !ballNearGoal {
					// A forward the ball has passed far from his goal runs
					// back hard but not flat out.
					effort = feM1ForwardBypassedEffort
				} else if goalSide && behind < feM1TrulyOutMetres {
					// Near his goal the graded regroup starts higher.
					floor := feM1RegroupMinEffort
					if ballNearGoal {
						floor = feM1RegroupNearGoalEffort
					}
					effort = hClamp(floor+(behind-feM1SettledTolerance)/feM1RegroupFullMetres, floor, 1.0)
				}
			} else if behind > feM1DropTolerance && !feM1Forward(p) {
				// Go addition: a defender or midfielder a few metres upfield
				// of his spot drops with purpose (effort growing with the
				// gap) rather than at the settled walking pace; forwards
				// drift back with the block.
				effort = hClamp((behind-feM1DropTolerance)/feM1DropFullMetres,
					feM1DropMinEffort, 1.0)
			}
			if intent == PlayerIntent_HOLD_SHAPE && task.role == feM1RoleRecoverShape &&
				task.opponent >= 0 {
				// As in P12 (P12:3105-3140), a settled player shading toward
				// his man carries MARK_OPPONENT.
				intent = PlayerIntent_MARK_OPPONENT
			}
		default:
			target = p.TacticalTarget
		}
		if task.role == feM1RoleRecoverShape {
			// P12's tight-mark order (P12:3084-3090) overrides the retreat.
			if tight := e.tightMarkTarget(p); tight != nil {
				intent = PlayerIntent_MARK_OPPONENT
				target = e.tightMarkPoint(p, tight, target)
				individual = true
			}
		}
		target = Vector2F{hClamp(target.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
			hClamp(target.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
		p.Intent = intent
		p.IsPressing = pressing
		p.TacticalTarget = target
		p.TargetOwner = TargetOwner_M1
		p.TargetReason = feM1TargetReason(task.role)
		p.UrgentMovement = urgent
		side.effort[slot] = effort
		side.individual[slot] = individual
		if direct {
			// Plan M1(d): no target lag for Press/Contain.
			p.MovementTarget = target
		}
		if intent == PlayerIntent_PRESS_BALL && s.prevIntent[slot] != PlayerIntent_PRESS_BALL {
			e.statsOf(p).Pressures++
		}
	}
}

// feM1Effort is the FE effort (0 jog … 1 sprint, FE Steering.fs:110-157
// speedMult) that M1 asks of a player it drives this refresh: Press/Contain
// 1.0, Cover/Screen/box marker 0.85, Regroup 1.0 while a ProtectDepth /
// RecoverShape player is getting back goal-side (0.6 for a forward still
// goal-side of the ball), and a graded drop effort for a defender or
// midfielder a few metres upfield of his spot.
// ok is false when M1 does not drive the player or has no opinion (settled
// shape), so the steering keeps its own phase effort. Meant for the M3
// steering, which uses it in place of its phase effort and shape-adjust cap
// (fe_m3_steering.go feM3IntegrateMovements).
func (e *MatchEngine) feM1Effort(player *MatchPlayer) (float64, bool) {
	if !e.cfg.Modules.M1Defence || player.IsGoalkeeper || !active(player) {
		return 0, false
	}
	side := &e.feM1Of().sides[hBoolIndex(player.IsHomeTeam)]
	if !side.owns {
		return 0, false
	}
	slot := e.slotOf(player)
	if slot >= len(side.effort) || side.effort[slot] < 0 {
		return 0, false
	}
	return side.effort[slot], true
}

// feM1Individual reports whether M1 has the player on an individual task
// (press, contain, cover, screen, box or tight marking, interception) this
// refresh, as opposed to a block role (ProtectDepth / RecoverShape) that
// shifts with its unit. For the M3 unit shift (fe_m3_individual.go).
func (e *MatchEngine) feM1Individual(player *MatchPlayer) bool {
	if !e.cfg.Modules.M1Defence || player.IsGoalkeeper || !active(player) {
		return false
	}
	side := &e.feM1Of().sides[hBoolIndex(player.IsHomeTeam)]
	slot := e.slotOf(player)
	return side.owns && slot < len(side.individual) && side.individual[slot]
}
