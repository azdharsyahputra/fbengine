// Ported from FootballEngine FootballEngine.Core/Match/Systems/Team/DefensiveTransition.fs (GPL-3.0)
//
// M1 defensive episode (FE `TeamTransition`, lines 156-481) and its creation
// on a change of possession (FE Stepper/EventApply.fs:285-307): a side that
// loses the ball assesses whether a counter-press is viable, commits to
// Counterpress or Retreat for at least 0.6 s, and ends the episode once its
// players have been goal-side and on their tasks for 0.6 s. Go adaptation: the
// episode is opened from refreshTacticalTargets (it watches
// lastControlledTeamHome) instead of inside setPossession, and the engine's
// per-match M1 state is MatchEngine.feM1Data (see feM1Of).
// FootballEngine: https://github.com/DeltaBitsSystem/FootballEngine (GPL-3.0).
package engine

import "math"

type feM1Mode uint8

const (
	feM1Assessing feM1Mode = iota
	feM1Counterpress
	feM1Retreat
)

type feM1Reason uint8

const (
	feM1AwaitingAssessment feM1Reason = iota
	feM1TacticalRestraint
	feM1DepthThreat
	feM1ArrivalTooLate
	feM1Outnumbered
	feM1MissingCover
	feM1LocalAdvantage
	feM1PressureBroken
	feM1PressWindowElapsed
)

type feM1Role uint8

const (
	feM1RoleNone feM1Role = iota
	feM1RoleFirstDefender
	feM1RoleCover
	feM1RoleScreen
	feM1RoleContain
	feM1RoleProtectDepth
	feM1RoleRecoverShape
	feM1RoleMarker
	feM1RoleIntercept // Go: cuts out the opponent's pass in flight
)

type feM1Task struct {
	role     feM1Role
	target   feM1Vec
	opponent int // e.players slot, -1 none
}

// feM1Episode is FE `DefensiveEpisode` (Contracts/DefensiveTransition.fs).
type feM1Episode struct {
	open             bool
	lostAt           float64
	origin           feM1Vec
	mode             feM1Mode
	reason           feM1Reason
	modeSince        float64
	commitUntil      float64
	primary, cover   int
	tactics          StrategySliders
	participants     int
	stabilizingSince float64 // < 0: not stabilising
}

type feM1Assessment struct {
	primary, cover        int
	arrival, coverArrival float64
	window                float64
	local, opponents      int
	depth                 int
	reason                feM1Reason
	viable                bool
}

type feM1Side struct {
	episode feM1Episode
	tasks   []feM1Task
	effort  []float64 // FE effort per slot this refresh, -1 none
	// Per slot this refresh: an individual task (press, contain, cover,
	// screen, box or tight marking, interception), not a block role.
	individual []bool
	// This refresh: M1 drives the side's outfielders.
	owns bool
	// The ball reference (FE ball position/velocity): the carrier's ball, or
	// the intended receiver while the opponent's pass is in flight.
	ball        feM1Vec
	vel         Vector2F
	carrierSlot int
	// The opponent's ground pass is in flight (ball position/velocity).
	inFlight   bool
	pressZone  bool // Phase 2 C3: this refresh is an organised press (fe_m1_tasks.go)
	flightBall feM1Vec
	flightVel  Vector2F
	rest       [2]int
	// Go addition: the team-frame ball depth the retreat clamp is measured
	// from. It follows the ball toward the own goal at once and steps back up
	// at most feM1StepUpSpeed, so a recycled ball does not pull the block
	// upfield (and back again) on every pass.
	retreatRef     float64
	retreatRefTime float64
	retreatRefSet  bool
	// Time of the previous task assignment (shape-role shift rate).
	assignTime float64
	assignSet  bool
}

type feM1State struct {
	sides       [2]feM1Side
	shape       []feM1Vec
	shapeSet    []bool
	prevIntent  []PlayerIntent
	lastControl Optional[bool]
	live        bool
}

// feM1Of returns the engine's M1 state (MatchEngine.feM1Data), sized to the
// players on the pitch by initFeState.
func (e *MatchEngine) feM1Of() *feM1State {
	if len(e.feM1Data.shape) != len(e.players) {
		e.initFeState()
	}
	return &e.feM1Data
}

func (side *feM1Side) clearTasks() {
	for i := range side.tasks {
		side.tasks[i] = feM1Task{opponent: -1}
		side.effort[i] = -1
		side.individual[i] = false
	}
}

func (e *MatchEngine) feM1Participants(home bool) int {
	n := 0
	for i := range e.players {
		if e.players[i].IsHomeTeam == home && active(&e.players[i]) {
			n++
		}
	}
	return n
}

// feM1Begin runs at the top of refreshTacticalTargets: it opens and closes
// episodes on changes of possession and decides which side M1 drives.
func (e *MatchEngine) feM1Begin(carrier *MatchPlayer) {
	s := e.feM1Of()
	now := e.getSimulatedSeconds()
	live := e.state == MatchState_PLAYING
	ball := feM1ToM(e.ball.Position)
	if !live {
		// FE update: play not live → the episode ends Interrupted.
		for i := range s.sides {
			s.sides[i].episode.open = false
			s.sides[i].clearTasks()
		}
	} else if e.lastControlledTeamHome.HasValue {
		holder := e.lastControlledTeamHome.Value
		if s.live && s.lastControl.HasValue && s.lastControl.Value != holder {
			// EventApply.fs:285-307: the losing side opens an episode.
			loser := &s.sides[hBoolIndex(!holder)]
			loser.clearTasks()
			loser.retreatRefSet = false
			loser.episode = feM1Episode{open: true, lostAt: now, origin: ball,
				mode: feM1Assessing, reason: feM1AwaitingAssessment, modeSince: now,
				commitUntil: now, primary: -1, cover: -1,
				tactics:      e.getEffectiveSliders(!holder),
				participants: e.feM1Participants(!holder), stabilizingSince: -1}
		}
		// The gaining side's episode ends (Recovered).
		s.sides[hBoolIndex(holder)].episode.open = false
	}
	s.lastControl = e.lastControlledTeamHome
	s.live = live

	for i := range s.prevIntent {
		s.prevIntent[i] = e.players[i].Intent
		s.shapeSet[i] = false
	}
	for _, home := range [2]bool{true, false} {
		side := &s.sides[hBoolIndex(home)]
		side.owns = false
		side.inFlight = false
		side.carrierSlot = -1
		if !live {
			continue
		}
		if carrier != nil {
			if carrier.IsHomeTeam != home {
				side.owns = true
				side.ball = ball
				side.vel = carrier.Velocity
				side.carrierSlot = e.slotOf(carrier)
			}
		} else if e.ball.IsPass && !e.ball.IsShot &&
			(!e.ball.IsAerialDelivery || e.lastPassDecision.Intent != PassIntent_SET_PIECE) &&
			e.ball.PassByHome != home && e.lastControlledTeamHome.HasValue &&
			e.lastControlledTeamHome.Value != home {
			// Go adaptation: during the opponent's pass the tasks are kept,
			// anchored on the intended receiver (FE keeps them through
			// Receiving control). Phase 2 (A3): open-play crosses too, so M1
			// owns every defending phase except set-piece deliveries (M9's
			// arrangement) and the loose-ball contest (loose_ball.go).
			side.owns = true
			side.ball = ball
			side.vel = e.ball.Velocity
			side.inFlight = true
			side.flightBall = ball
			side.flightVel = e.ball.Velocity
			if receiver := e.findMatchPlayer(e.ball.IntendedReceiver); receiver != nil && active(receiver) {
				side.ball = feM1ToM(receiver.Position)
				side.vel = receiver.Velocity
				side.carrierSlot = e.slotOf(receiver)
			}
		}
		if !side.owns {
			side.clearTasks()
			side.retreatRefSet = false
			side.assignSet = false
			continue
		}
		bx := side.ball.X * feM1Dir(home)
		if !side.retreatRefSet || bx <= side.retreatRef {
			side.retreatRef = bx
		} else {
			side.retreatRef = math.Min(bx, side.retreatRef+feM1StepUpSpeed*math.Max(0, now-side.retreatRefTime))
		}
		side.retreatRefTime = now
		side.retreatRefSet = true
	}
}

// feM1Owns reports whether M1 sets this player's target this refresh.
func (e *MatchEngine) feM1Owns(player *MatchPlayer) bool {
	if player.IsGoalkeeper || !active(player) {
		return false
	}
	return e.feM1Of().sides[hBoolIndex(player.IsHomeTeam)].owns
}

// feM1CaptureShape records the P12 block target (the FE shape target) of a
// player M1 drives; refreshTacticalTargets computes it before the
// out-of-possession branch.
func (e *MatchEngine) feM1CaptureShape(player *MatchPlayer, shape Vector2F) {
	s := e.feM1Of()
	slot := e.slotOf(player)
	if slot < len(s.shape) {
		s.shape[slot] = feM1ToM(shape)
		s.shapeSet[slot] = true
	}
}

// feM1Assess is FE `TeamTransition.assess` (DefensiveTransition.fs:156-257).
func (e *MatchEngine) feM1Assess(home bool, side *feM1Side, sliders StrategySliders, rest1, rest2 int) feM1Assessment {
	ep := &side.episode
	dir := feM1Dir(home)
	ball := side.ball
	tx := func(slot int) float64 { return feM1ToM(e.players[slot].Position).X * dir }

	primary, arrival := e.feM1ChooseSlot(home, 1.0, feM1PressPoint(home, ball, side.vel),
		func(slot int) bool {
			return slot == rest1 || slot == rest2 ||
				e.players[slot].Stamina < feM1MinimumPressCondition ||
				tx(slot) > ball.X*dir+feM1BeatenMargin
		}, ep.primary, feM1HandoffAdvantage)
	cover, coverArrival := e.feM1ChooseSlot(home, feM1CoverEffort, feM1CoverPoint(home, ball),
		func(slot int) bool {
			return slot == primary || slot == rest1 || slot == rest2 ||
				tx(slot) > ball.X*dir ||
				e.players[slot].Stamina < feM1MinimumPressCondition
		}, ep.cover, feM1HandoffAdvantage)

	local, opponents, depth := 0, 0, 0
	for slot := range e.players {
		p := &e.players[slot]
		if p.IsHomeTeam == home && feM1Outfielder(p) &&
			p.Stamina >= feM1MinimumPressCondition &&
			e.bodyTimeTo(p, ball, 1.0) <= feM1LocalArrivalWindow {
			local++
		}
	}
	depthLine := feM1OwnGoalX(home) * dir
	if rest2 >= 0 {
		depthLine = tx(rest2)
	}
	for slot := range e.players {
		p := &e.players[slot]
		if p.IsHomeTeam == home || !feM1Outfielder(p) {
			continue
		}
		if e.bodyTimeTo(p, ball, 1.0) <= feM1LocalArrivalWindow {
			opponents++
		}
		pm := feM1ToM(p.Position)
		if pm.X*dir < depthLine-feM1DepthSafety && math.Abs(pm.Y) <= feM1CentralHalfWidth {
			depth++
		}
	}
	window := feM1BaseArrivalWindow + sliders.Pressing*feM1PressArrivalExtension
	nearGoal := feM1Dist(ball, feM1Vec{feM1OwnGoalX(home), 0}) < feM1ThreatGoalDistance

	var reason feM1Reason
	switch {
	case sliders.Pressing < feM1MinimumPressIntensity:
		reason = feM1TacticalRestraint
	case depth > 0 || rest2 < 0 || (nearGoal && coverArrival > arrival):
		reason = feM1DepthThreat
	case primary < 0 || arrival > window:
		reason = feM1ArrivalTooLate
	case local < feM1MinimumLocalPlayers || local < opponents+feM1LocalSurplus:
		reason = feM1Outnumbered
	case cover < 0 || coverArrival > feM1CoverArrivalWindow:
		reason = feM1MissingCover
	default:
		reason = feM1LocalAdvantage
	}
	return feM1Assessment{primary: primary, cover: cover, arrival: arrival,
		coverArrival: coverArrival, window: window, local: local, opponents: opponents,
		depth: depth, reason: reason, viable: reason == feM1LocalAdvantage}
}

// feM1AllSettled is FE `allSettled` (:269-294): every outfielder goal-side of
// the ball and within 3 m of his task target.
func (e *MatchEngine) feM1AllSettled(home bool, side *feM1Side) bool {
	dir := feM1Dir(home)
	for slot := range e.players {
		p := &e.players[slot]
		if p.IsHomeTeam != home || !feM1Outfielder(p) {
			continue
		}
		task := side.tasks[slot]
		if task.role == feM1RoleNone {
			return false
		}
		pm := feM1ToM(p.Position)
		if pm.X*dir > side.ball.X*dir {
			return false
		}
		// Go adaptation: P12 bodies trail a moving target (target blend and
		// arrive braking), so the ball-tracking roles are not held to FE's
		// 3 m, and the engagers (who chase a moving carrier) are not checked.
		tolerance := feM1SettledTolerance
		switch task.role {
		case feM1RoleFirstDefender, feM1RoleContain:
			tolerance = math.Inf(1)
		case feM1RoleRecoverShape, feM1RoleProtectDepth, feM1RoleMarker:
			tolerance = feM1SettledToleranceGo
		}
		off := feM1Dist(pm, task.target)
		if task.role == feM1RoleProtectDepth {
			// The rest defenders' job is depth: only their depth error counts.
			off = math.Abs(pm.X - task.target.X)
		}
		if off > tolerance {
			return false
		}
	}
	return true
}

// feM1UpdateEpisode is FE `TeamTransition.update` (:296-481) for live play.
func (e *MatchEngine) feM1UpdateEpisode(home bool, side *feM1Side, sliders StrategySliders, rest1, rest2 int) {
	ep := &side.episode
	if !ep.open {
		return
	}
	now := e.getSimulatedSeconds()
	dir := feM1Dir(home)
	a := e.feM1Assess(home, side, sliders, rest1, rest2)

	progression := (ep.origin.X - side.ball.X) * dir
	firstBeaten := ep.primary >= 0 && ep.primary < len(e.players) &&
		feM1ToM(e.players[ep.primary].Position).X*dir > side.ball.X*dir+feM1BeatenMargin
	broken := progression > feM1EscapeProgress || (firstBeaten && !a.viable)
	elapsed := now-ep.lostAt >= feM1MaxPressDuration
	participants := e.feM1Participants(home)
	forced := broken || a.primary < 0 || a.cover < 0 ||
		a.reason == feM1DepthThreat || a.reason == feM1MissingCover ||
		ep.tactics != sliders || ep.participants != participants

	mode, reason := ep.mode, ep.reason
	switch {
	case ep.mode == feM1Retreat:
		// One-way.
	case broken:
		mode, reason = feM1Retreat, feM1PressureBroken
	case elapsed:
		mode, reason = feM1Retreat, feM1PressWindowElapsed
	case ep.mode == feM1Assessing || forced || now >= ep.commitUntil:
		mode, reason = feM1Retreat, a.reason
		if a.viable {
			mode = feM1Counterpress
		}
	}
	primary, cover := -1, -1
	if mode == feM1Counterpress {
		primary, cover = a.primary, a.cover
	}

	settled := ep.mode == feM1Retreat && mode == feM1Retreat && a.depth == 0 &&
		e.feM1AllSettled(home, side)
	if settled {
		if ep.stabilizingSince < 0 {
			ep.stabilizingSince = now
		}
	} else {
		ep.stabilizingSince = -1
	}
	if mode != ep.mode {
		ep.modeSince = now
		ep.commitUntil = now + feM1Commitment
	}
	ep.mode, ep.reason = mode, reason
	ep.primary, ep.cover = primary, cover
	ep.tactics = sliders
	ep.participants = participants
	// Go adaptation: P12/M3 bodies trail their moving targets, so FE's settle
	// test can stay unmet for a whole spell of possession; a retreat older than
	// feM1RetreatTimeout hands over to organised defence anyway.
	timedOut := mode == feM1Retreat && now-ep.lostAt >= feM1RetreatTimeout
	if (settled && now-ep.stabilizingSince >= feM1StabilizationHold) || timedOut {
		// Stabilized / Retreated / PressureBeaten: organised defence takes over.
		ep.open = false
		side.clearTasks()
	}
}
