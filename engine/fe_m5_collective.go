// Ported from FootballEngine FootballEngine.Core/Match/Systems/Team/CollectiveAttack.fs (GPL-3.0)
//
// M5 (FE_PORT_PLAN §2 M5): collective attack duties without the learning
// part. When an outfielder has the ball his side may open an attack
// episode: one of four patterns (ThirdMan, AttractSwitch, ExploitDepth,
// SecureProgression) gives three team-mates duties with targets relative to
// the carrier: Link (the next receiver, 6 m ahead, 8 m wide), Runner (the run
// beyond, 13 m ahead) and Security (9 m behind, rest defence). Duty holders
// are the earliest arrivers (DefensiveTransition arrivalTime, plan §0.3 P6)
// with a clear first lane; candidates are scored on geometry only
// (CollectiveAttack.fs:158-297, `learned` and exploration dropped). The
// episode lives until the ball is lost, another team-mate gets it, the
// 12 s deadline, or the lane stays shut for 1.5 s; completed passes to the
// expected receiver advance Preparing → Connected → Exploiting
// (:405-436, :464-632). Constants: BalanceConfig.fs:841-879.
//
// Go adaptations (ours): pass completion is read from the change of carrier
// at the tactical refresh (Player12 has no FE event stream); a pause in the
// refresh (stoppage) ends the episode as FE's Stopped; P12's tactical
// sliders stand in for FE Tactics (Directness ← OffensiveBias, Width ←
// WidthUsage, Mentality ← 2·RiskTaking−1).
// FootballEngine: https://github.com/DeltaBitsSystem/FootballEngine (GPL-3.0).
package engine

import "math"

const (
	feM5Duration          = 12.0 // 480 subticks
	feM5Recovery          = 2.0  // 80 subticks
	feM5CounterHold       = 1.5  // 60 subticks
	feM5MinDutySeparation = 3.0
	feM5SupportDepth      = 6.0
	feM5SupportWidth      = 8.0
	feM5RunDepth          = 13.0
	feM5SwitchWidth       = 24.0
	feM5SecurityDepth     = 9.0
	feM5OffsideMargin     = 1.5
	feM5MaxArrival        = 4.0
	feM5LaneRadius        = 2.0
	feM5MinCondition      = 0.3
	feM5MinScore          = 0.1
	// Decision bias (Decision.fs:331-376): 0.02·(0.22·confidence·…), with
	// confidence 0.5 without learning.
	feM5BiasScale      = 0.02
	feM5CooperationW   = 0.22
	feM5BiasConfidence = 0.5
	// Ours: seconds added to the arrival race of a role outside the duty's
	// natural group (forward as Security, back or pivot as Runner).
	feM5RolePenalty = 1.0
	// Ours: the Security trails the ball by at most this (m).
	feM5SecurityTrail = 7.0
)

type feM5Pattern int

const (
	feM5ThirdMan feM5Pattern = iota
	feM5AttractSwitch
	feM5ExploitDepth
	feM5SecureProgression
	feM5PatternCount
)

type feM5Stage int

const (
	feM5Preparing feM5Stage = iota
	feM5Connected
	feM5Exploiting
)

// Duty kinds.
const (
	feM5Link = iota
	feM5Runner
	feM5Security
)

type feM5Episode struct {
	active       bool
	pattern      feM5Pattern
	stage        feM5Stage
	owner        int
	duty         [3]int
	occupant     [3]*Player
	target       [3]feM1Vec // FE world metres
	deadline     float64
	blocked      bool
	blockedSince float64
}

type feM5Side struct {
	ep          feM5Episode
	availableAt float64
}

type feM5State struct {
	sides    [2]feM5Side
	lastStep uint64
	started  bool
}

func (e *MatchEngine) feM5() *feM5State {
	if len(e.feM1Data.shape) != len(e.players) {
		e.initFeState()
	}
	return &e.feM5Data
}

func (e *MatchEngine) feM5Now() float64 {
	return float64(e.stepCounter) * Timing_FIXED_STEP_SECONDS
}

// feM5LegalTarget is AttackGeometry.legalTarget (:81-90): no deeper than
// 1.5 m short of the offside line, the ball or the halfway line, whichever
// is farthest, and inside the pitch.
func (e *MatchEngine) feM5LegalTarget(home bool, x, y float64) feM1Vec {
	dir := feM1Dir(home)
	line := feM1ToM(Vector2F{e.offsideLine(home), 0.5}).X
	ball := feM1ToM(e.ball.Position).X
	legalX := math.Max(0.0, math.Max(line*dir, ball*dir)) - feM5OffsideMargin
	x = math.Min(x*dir, legalX) * dir
	return feM1Vec{
		hClamp(x, -(feM1HalfLength - feM1TargetInset), feM1HalfLength-feM1TargetInset),
		hClamp(y, -(feM1HalfWidth - feM1TargetInset), feM1HalfWidth-feM1TargetInset),
	}
}

// feM5Space is AttackGeometry.space (:46-56).
// Phase 2 (B3): the shared pitch control at the point (was the nearest
// opponent's distance over SpaceScale).
func (e *MatchEngine) feM5Space(home bool, point feM1Vec) float64 {
	return e.SpatialControl(home, point)
}

func feM5Separated(a, b, c feM1Vec) bool {
	return feM1Dist(a, b) >= feM5MinDutySeparation && feM1Dist(a, c) >= feM5MinDutySeparation &&
		feM1Dist(b, c) >= feM5MinDutySeparation
}

func feM5Eligible(p *MatchPlayer) bool {
	return feM1Outfielder(p) && p.Stamina >= feM5MinCondition && !p.IsInjured
}

// feM5Readiness is clamp01((Anticipation + Teamwork)/200)·cognitive·condition.
func feM5Readiness(p *MatchPlayer) float64 {
	anticipation := feM3Attr(p.Player, "anticipation", p.Vision)
	teamwork := feM3Attr(p.Player, "teamwork", 0.6)
	cognitive := 0.5 + 0.5/(1.0+math.Exp(-3.0*(p.Stamina-0.5)))
	return hClamp((anticipation+teamwork)/2.0, 0.0, 1.0) * cognitive * p.Stamina
}

type feM5Candidate struct {
	pattern  feM5Pattern
	duty     [3]int
	target   [3]feM1Vec
	geometry float64
}

// feM5Candidates is AttackCandidates.candidates (:158-297) without learning.
func (e *MatchEngine) feM5Candidates(home bool, carrier *MatchPlayer, out *[feM5PatternCount]feM5Candidate) int {
	dir := feM1Dir(home)
	origin := feM1ToM(carrier.Position)
	sign := 1.0
	if origin.Y < 0 {
		sign = -1.0
	}
	point := func(depth, y float64) feM1Vec {
		return e.feM5LegalTarget(home, origin.X+dir*depth, y)
	}
	carrierSlot := e.slotOf(carrier)
	// kind: feM5Link / Runner / Security.
	choose := func(target feM1Vec, ex1, ex2 int, kind int) int {
		best, bestT, bestRaw := -1, 0.0, 0.0
		for slot := range e.players {
			p := &e.players[slot]
			if p.IsHomeTeam != home || slot == carrierSlot || slot == ex1 || slot == ex2 || !feM5Eligible(p) {
				continue
			}
			penalty := 0.0
			if p.Player != nil {
				role := p.Player.GetRole()
				// Ours: a centre-back is rest defence (Security), never the
				// Link or the Runner; the striker is never the Security. The
				// rotation keeps the structure: a forward is slow to take the
				// Security, a back or pivot slow to take the Runner.
				if (kind != feM5Security && role == PlayerRole_CB) || (kind == feM5Security && role == PlayerRole_ST) {
					continue
				}
				forward := feM4IsForward(role)
				if (kind == feM5Security && forward) || (kind == feM5Runner && !forward) {
					penalty = feM5RolePenalty
				}
			}
			raw := e.bodyTimeTo(p, target, 1.0)
			t := raw + penalty
			if best < 0 || t < bestT {
				best, bestT, bestRaw = slot, t, raw
			}
		}
		if best >= 0 && bestRaw <= feM5MaxArrival {
			return best
		}
		return -1
	}
	sliders := e.getEffectiveSliders(home)
	directness := sliders.OffensiveBias
	width := sliders.WidthUsage
	mentality := 2.0*sliders.RiskTaking - 1.0
	n := 0
	for pattern := feM5Pattern(0); pattern < feM5PatternCount; pattern++ {
		var link, runner feM1Vec
		switch pattern {
		case feM5ExploitDepth:
			link = point(feM5SupportDepth, origin.Y+sign*feM5SupportWidth)
		default:
			link = point(feM5SupportDepth, origin.Y-sign*feM5SupportWidth)
		}
		switch pattern {
		case feM5ThirdMan:
			runner = point(feM5RunDepth, origin.Y+sign*feM5SupportWidth)
		case feM5AttractSwitch:
			runner = point(feM5SupportDepth, -sign*feM5SwitchWidth)
		case feM5ExploitDepth:
			runner = point(feM5RunDepth, origin.Y-sign*feM5SupportWidth)
		default:
			runner = point(feM5SupportDepth, origin.Y+sign*feM5SupportWidth)
		}
		security := point(-feM5SecurityDepth, origin.Y+sign*feM5SupportWidth)
		linkSlot := choose(link, -1, -1, feM5Link)
		if linkSlot < 0 {
			continue
		}
		runnerSlot := choose(runner, linkSlot, -1, feM5Runner)
		if runnerSlot < 0 {
			continue
		}
		securitySlot := choose(security, linkSlot, runnerSlot, feM5Security)
		if securitySlot < 0 {
			continue
		}
		first := feM1ToM(e.players[linkSlot].Position)
		if pattern == feM5ExploitDepth {
			first = feM1ToM(e.players[runnerSlot].Position)
		}
		if !feM5Separated(link, runner, security) || !e.spatialLaneOpen(home, origin, first, feM5LaneRadius) {
			continue
		}
		space := e.feM5Space(home, runner)
		openness := 0.0
		if e.spatialLaneOpen(home, link, runner, feM5LaneRadius) {
			openness = 1.0
		}
		var tactical float64
		switch pattern {
		case feM5ThirdMan:
			tactical = 1.0 - directness
		case feM5AttractSwitch:
			tactical = width
		case feM5ExploitDepth:
			tactical = directness
		default:
			tactical = 1.0 - math.Max(0.0, mentality)
		}
		geometry := (space + openness + feM5Readiness(&e.players[linkSlot]) +
			feM5Readiness(&e.players[runnerSlot]) + tactical) / 5.0
		out[n] = feM5Candidate{pattern, [3]int{linkSlot, runnerSlot, securitySlot},
			[3]feM1Vec{link, runner, security}, geometry}
		n++
	}
	return n
}

func (side *feM5Side) finish(now float64) {
	side.ep.active = false
	side.availableAt = now + feM5Recovery
}

// feM5Update is TeamAttack.update (:464-632) for both sides; it runs at the
// start of refreshTacticalTargets.
func (e *MatchEngine) feM5Update(carrier *MatchPlayer) {
	if !e.cfg.Modules.M5Collective {
		return
	}
	st := e.feM5()
	now := e.feM5Now()
	stopped := st.started && e.stepCounter-st.lastStep > 3*uint64(max(e.stepTicks, 1))*Timing_TACTICAL_REFRESH_STEPS
	st.started = true
	st.lastStep = e.stepCounter
	for s := 0; s < 2; s++ {
		home := s == 0
		side := &st.sides[s]
		if stopped {
			if side.ep.active {
				side.finish(now)
			}
			continue
		}
		if side.ep.active {
			e.feM5Continue(home, side, carrier, now)
			continue
		}
		if now < side.availableAt || carrier == nil || carrier.IsHomeTeam != home || carrier.IsGoalkeeper {
			continue
		}
		var cands [feM5PatternCount]feM5Candidate
		n := e.feM5Candidates(home, carrier, &cands)
		best := -1
		for i := 0; i < n; i++ {
			if best < 0 || cands[i].geometry > cands[best].geometry {
				best = i
			}
		}
		if best < 0 || cands[best].geometry < feM5MinScore {
			side.availableAt = now + feM5Recovery
			continue
		}
		c := cands[best]
		side.ep = feM5Episode{active: true, pattern: c.pattern, stage: feM5Preparing,
			owner: e.slotOf(carrier), duty: c.duty, target: c.target, deadline: now + feM5Duration}
		for k, slot := range c.duty {
			side.ep.occupant[k] = e.players[slot].Player
		}
	}
}

func (e *MatchEngine) feM5Continue(home bool, side *feM5Side, carrier *MatchPlayer, now float64) {
	ep := &side.ep
	for k, slot := range ep.duty {
		p := &e.players[slot]
		if !feM1Outfielder(p) || p.Player != ep.occupant[k] {
			side.finish(now) // ParticipantLeft
			return
		}
	}
	if ep.owner < 0 || ep.owner >= len(e.players) || !feM1Outfielder(&e.players[ep.owner]) {
		side.finish(now)
		return
	}
	var lost bool
	if carrier != nil {
		lost = carrier.IsHomeTeam != home
	} else {
		lost = !e.lastControlledTeamHome.HasValue || e.lastControlledTeamHome.Value != home
	}
	if lost {
		side.finish(now) // LostBall
		return
	}
	if carrier != nil && e.slotOf(carrier) != ep.owner {
		// A completed pass: the expected receiver advances the stage, any
		// other team-mate means the play deviated (:405-436).
		expected := ep.duty[feM5Link]
		if (ep.stage == feM5Preparing && ep.pattern == feM5ExploitDepth) || ep.stage == feM5Connected {
			expected = ep.duty[feM5Runner]
		}
		if ep.stage == feM5Exploiting || e.slotOf(carrier) != expected {
			side.finish(now) // Deviated
			return
		}
		if ep.stage == feM5Preparing && ep.pattern != feM5ExploitDepth {
			ep.stage = feM5Connected
		} else {
			ep.stage = feM5Exploiting
		}
		ep.owner = expected
		ep.blocked = false
	}
	if now >= ep.deadline {
		side.finish(now) // TimedOut
		return
	}
	targetSlot := ep.duty[feM5Runner]
	if ep.stage == feM5Preparing && ep.pattern != feM5ExploitDepth {
		targetSlot = ep.duty[feM5Link]
	}
	ownFlight := carrier == nil && (e.ball.IsPass && e.ball.PassByHome == home || e.ball.IsShot && e.ball.ShotByHome == home)
	clear := ep.stage == feM5Exploiting || ownFlight ||
		e.spatialLaneOpen(home, feM1ToM(e.ball.Position), feM1ToM(e.players[targetSlot].Position), feM5LaneRadius)
	if clear {
		ep.blocked = false
	} else if !ep.blocked {
		ep.blocked, ep.blockedSince = true, now
	} else if now-ep.blockedSince >= feM5CounterHold {
		side.finish(now) // Countered
		return
	}
	var legal [3]feM1Vec
	for k := range legal {
		legal[k] = e.feM5LegalTarget(home, ep.target[k].X, ep.target[k].Y)
	}
	// Go adaptation (ours): the Security follows the ball up the pitch, so
	// he is never more than SecurityTrail behind it (FE keeps him at the
	// episode origin, which leaves a hole once the ball has moved on).
	dir := feM1Dir(home)
	if minX := feM1ToM(e.ball.Position).X*dir - feM5SecurityTrail; legal[feM5Security].X*dir < minX {
		legal[feM5Security] = e.feM5LegalTarget(home, minX*dir, legal[feM5Security].Y)
	}
	if !feM5Separated(legal[0], legal[1], legal[2]) {
		side.finish(now) // Countered
		return
	}
	ep.target = legal
}

// feM5DutyOf returns the duty (feM5Link / Runner / Security) and its target
// (pitch units) of a player whose side has an open episode and who is not
// the ball carrier; ok is false otherwise.
func (e *MatchEngine) feM5DutyOf(player *MatchPlayer) (kind int, target Vector2F, ok bool) {
	if !e.cfg.Modules.M5Collective || player.IsGoalkeeper {
		return 0, Vector2F{}, false
	}
	ep := &e.feM5().sides[hBoolIndex(player.IsHomeTeam)].ep
	if !ep.active {
		return 0, Vector2F{}, false
	}
	slot := e.slotOf(player)
	if slot == ep.owner {
		return 0, Vector2F{}, false
	}
	for k, d := range ep.duty {
		if d == slot && ep.occupant[k] == player.Player {
			return k, feM1ToPitch(ep.target[k]), true
		}
	}
	return 0, Vector2F{}, false
}

// feM5HasDuty reports whether the player holds a duty in an open episode.
func (e *MatchEngine) feM5HasDuty(player *MatchPlayer) bool {
	_, _, ok := e.feM5DutyOf(player)
	return ok
}

// feM5Active reports whether the side has an open episode.
func (e *MatchEngine) feM5Active(home bool) bool {
	return e.cfg.Modules.M5Collective && e.feM5().sides[hBoolIndex(home)].ep.active
}

// feM5RunnerRuns: the Runner of a depth pattern makes a run beyond
// (RUN_IN_BEHIND, plan M5(d)); a switch or securing Runner offers support.
func (e *MatchEngine) feM5RunnerRuns(home bool) bool {
	ep := &e.feM5().sides[hBoolIndex(home)].ep
	return ep.active && (ep.pattern == feM5ThirdMan || ep.pattern == feM5ExploitDepth)
}

// feM5LinkSide is the lateral side (+1 / -1 in pitch y) of the Link's target
// relative to the carrier, for placing P12's square supporter opposite it.
func (e *MatchEngine) feM5LinkSide(home bool, carrier *MatchPlayer) (float64, bool) {
	ep := &e.feM5().sides[hBoolIndex(home)].ep
	if !e.cfg.Modules.M5Collective || !ep.active || carrier == nil {
		return 0, false
	}
	link := feM1ToPitch(ep.target[feM5Link])
	if link.Y >= carrier.Position.Y {
		return 1.0, true
	}
	return -1.0, true
}

// FeM5PassBias is the M5 decision bias (Decision.fs:331-376) for a pass
// from passer to receiver, in goal-probability units: the episode Owner
// gets 0.02·(0.22·confidence·cognitive·condition·Teamwork/100) toward the
// stage's expected receiver, zero otherwise. For the M2 decision layer.
func (e *MatchEngine) FeM5PassBias(passer, receiver *MatchPlayer) float64 {
	if !e.cfg.Modules.M5Collective || passer == nil || receiver == nil {
		return 0
	}
	ep := &e.feM5().sides[hBoolIndex(passer.IsHomeTeam)].ep
	if !ep.active || e.slotOf(passer) != ep.owner {
		return 0
	}
	expected := ep.duty[feM5Link]
	switch {
	case ep.stage == feM5Exploiting:
		return 0
	case ep.stage == feM5Connected, ep.pattern == feM5ExploitDepth:
		expected = ep.duty[feM5Runner]
	}
	if e.slotOf(receiver) != expected {
		return 0
	}
	cognitive := 0.5 + 0.5/(1.0+math.Exp(-3.0*(passer.Stamina-0.5)))
	teamwork := feM3Attr(passer.Player, "teamwork", 0.6)
	return feM5BiasScale * feM5CooperationW * feM5BiasConfidence * cognitive * passer.Stamina * teamwork
}

// Shape_KEEPER_SUPPORT_DEPTH (pitch units, ours): with the keeper on the
// ball, P12's support slots (6-13 m behind the carrier) are mirrored to this
// far in front of him plus the slot's own depth.
const Shape_KEEPER_SUPPORT_DEPTH = 0.15

// Shape_SHORT_FORWARD_STRIKER_PENALTY_METRES (ours): selectShortForward adds
// this to the striker's distance, so a wide forward or the No.10 comes short
// first and the striker keeps the last line.
const Shape_SHORT_FORWARD_STRIKER_PENALTY_METRES = 15.0
