// Ported from FootballEngine FootballEngine.Core/Match/Systems/Players/DecisionValue.fs
// and the xG/threat primitives of Shared.fs (GPL-3.0); constants from
// Contracts/BalanceConfig.fs `defaultExpectedValue` unless marked "ours".
// FootballEngine: https://github.com/DeltaBitsSystem/FootballEngine (GPL-3.0).
//
// M2 option values, all in goal probability: what an action adds to the
// carrier's side scoring minus what it gives the opponent. Every formula is in
// the FE metre frame (feM1ToM): origin at the centre spot, the side attacks
// +x·dir.
package engine

import "math"

const (
	// Threat field (plan §0.3 P1). FE: ThreatFloor 0.012, ThreatPeak 1.0,
	// ThreatDecay 20 m.
	feM2ThreatFloor = 0.012
	// ours: FE's peak 1.0 values a ball held 11 m out at 0.59 goals, 2.5x the
	// engine's own xG there, so carriers never shot and dribbled into the box
	// until tackled.
	feM2ThreatPeak  = 0.3
	feM2ThreatDecay = 20.0

	// Race → completion (DecisionValue.passCompletion). FE: scale 10 steps,
	// no bias. Ours, fitted to the engine's own outcomes (pass → next decision
	// by the receiver, seeds 7/19, ~2000 ground passes): the live pass error,
	// first touch and trap cost about 0.25 s of race margin.
	feM2PassMarginScale = 12.0
	feM2PassMarginBias  = 10.0
	feM2PassSkillSteps  = 6.0
	// ours: even an unopposed lofted ball is sometimes misjudged (same fit).
	feM2LoftedCeiling = 0.95

	// Carry (DecisionValue.carryKeep / carry / blockers).
	feM2CarryAdvance         = 5.0
	feM2CarryKeepBase        = 0.95
	feM2CarryPressurePenalty = 0.12
	feM2PressureRadius       = 4.0
	feM2CarryBlockPenalty    = 0.08
	feM2CarryBlockRange      = 15.0

	// Shot (Decision.xGValue / fillPlan).
	feM2ShotBlockShare = 0.4
	feM2ShotTurnLimit  = 1.75
	feM2ShootingRange  = 32.0

	// Cross, clear, hold.
	feM2CrossBaseSuccess = 0.25
	feM2HeaderConversion = 0.12
	feM2ClearConcession  = 0.3
	feM2LossNear         = 1.0
	feM2LossRange        = 4.0
	feM2SupportRelief    = 0.3
	feM2SupportDistance  = 10.0
	// Decision.fs: a clearance is an option only in the own third.
	feM2OwnThirdX = -20.0

	// Choice (Decision.finish).
	feM2Temperature = 0.0005
	feM2StyleValue  = 0.02
	// MentalityRiskThresholdShift (TacticalTradeoffs).
	feM2MentalityRiskShift = 0.08

	// Perception.LaneClear: two opponents within this of the lane make a wall.
	feM2LaneClearRadius = 2.5
	// Perception cognitive fatigue (CognitiveFatigueFloor / Steepness).
	feM2CognitiveFloor     = 0.5
	feM2CognitiveSteepness = 3.0
)

func feM2GoalX(home bool) float64 { return feM1HalfLength * feM1Dir(home) }

// feM2DistanceToGoal is MatchMath.distanceToGoal: to the centre of the goal
// the side attacks.
func feM2DistanceToGoal(home bool, p feM1Vec) float64 {
	return math.Hypot(p.X-feM2GoalX(home), p.Y)
}

// feM2Threat is DecisionValue.threat: chance a possession at p ends in a goal
// for the side.
func (e *MatchEngine) feM2Threat(home bool, p feM1Vec) float64 {
	return feM2ThreatFloor + e.cfg.M2.ThreatPeak*math.Exp(-feM2DistanceToGoal(home, p)/e.cfg.M2.ThreatDecay)
}

// feM2Concession is DecisionValue.concession: the opponent's threat at p.
func (e *MatchEngine) feM2Concession(home bool, p feM1Vec) float64 { return e.feM2Threat(!home, p) }

// feM2PassCompletion is DecisionValue.passCompletion (passing 0..1 here).
func (e *MatchEngine) feM2PassCompletion(passing float64, margin int) float64 {
	effective := float64(margin) - e.cfg.M2.PassMarginBias + feM2PassSkillSteps*(passing*100.0-60.0)/40.0
	return 1.0 / (1.0 + math.Exp(-effective/feM2PassMarginScale))
}

// feM2PassValue is DecisionValue.pass: what the receiver holds if it
// arrives, the opponent's threat where the lane is cut if it does not.
// arrived is the value of the ball at the receiver: FE uses threat(to); the
// M2 path passes held(to) (see feM2PassOptions).
func (e *MatchEngine) feM2PassValue(home bool, completion, arrived float64, from, to feM1Vec) float64 {
	mid := feM1Vec{(from.X + to.X) / 2.0, (from.Y + to.Y) / 2.0}
	return completion*arrived - (1.0-completion)*e.feM2Concession(home, mid)
}

// feM2NearestOpponent is the distance (m) from p to the side's nearest
// active opponent.
func (e *MatchEngine) feM2NearestOpponent(home bool, p feM1Vec) float64 {
	best := math.MaxFloat64
	for i := range e.players {
		o := &e.players[i]
		if o.IsHomeTeam != home && active(o) {
			best = math.Min(best, feM1Dist(feM1ToM(o.Position), p))
		}
	}
	return best
}

// feM2SupportedAt: a team-mate other than skip within SupportDistance of p.
func (e *MatchEngine) feM2SupportedAt(home bool, p feM1Vec, skip1, skip2 *MatchPlayer) bool {
	for i := range e.players {
		q := &e.players[i]
		if q == skip1 || q == skip2 || q.IsHomeTeam != home || !active(q) {
			continue
		}
		if feM1Dist(feM1ToM(q.Position), p) < feM2SupportDistance {
			return true
		}
	}
	return false
}

// feM2CarryValue is DecisionValue.carry. Ours: the carry advances toward
// the goal centre and never past it; FE's straight +x·dir advance valued a
// winger on the byline as if he could carry 5 m beyond it, next to the post.
func (e *MatchEngine) feM2CarryValue(home bool, keep float64, p feM1Vec) float64 {
	ux, uy, d := feM2ToGoal(home, p)
	step := math.Min(feM2CarryAdvance, d)
	ahead := feM1Vec{p.X + ux*step, p.Y + uy*step}
	return keep*e.feM2Threat(home, ahead) - (1.0-keep)*e.feM2Concession(home, p)
}

// feM2ToGoal is the unit vector and distance from p to the attacked goal.
func feM2ToGoal(home bool, p feM1Vec) (ux, uy, d float64) {
	dx, dy := feM2GoalX(home)-p.X, -p.Y
	d = math.Hypot(dx, dy)
	if d < EPSILON {
		return feM1Dir(home), 0.0, 0.0
	}
	return dx / d, dy / d, d
}

// feM2CarryKeep is DecisionValue.carryKeep with CarrySkillWeight 0: the
// physical duel (M6) already reads Dribbling against Tackling.
//
// Ours: nearest is the distance to the nearest opponent. Inside the M6
// challenge range a carry is a duel the tackler wins about half the time
// (fe_m6_challenge.go), which FE's head count does not see; the duel share
// fades out by feM2DuelRange.
func (e *MatchEngine) feM2CarryKeep(opponents, blockers int, nearest float64) float64 {
	duel := e.cfg.M2.DuelLoss * hClamp((feM2DuelRange-nearest)/(feM2DuelRange-feM2DuelReach), 0.0, 1.0)
	return hClamp(feM2CarryKeepBase-e.cfg.M2.CarryPressure*float64(opponents)-
		feM2CarryBlockPenalty*float64(blockers)-duel, 0.0, 1.0)
}

const (
	// ours: full duel risk within the M6 sliding reach, none beyond 4 m.
	feM2DuelReach = 1.5
	feM2DuelRange = 4.0
)

// feM2DuelLossV: ours, the duel share charged to a carry in contact. The
// engine keeps ~88 % of carries in contact to the next decision, but each one
// is a duel the viewer reads as a possession flip; 0.3 trades those
// ping-pongs against more lay-offs.

// feM2CrossValue is DecisionValue.cross: a met cross is a header at goal, one
// that is not is the opponent's ball in their own box. crossing is 0..1.
func (e *MatchEngine) feM2CrossValue(home bool, crossing float64) float64 {
	met := hClamp(feM2CrossBaseSuccess*crossing*100.0/60.0, 0.0, 1.0)
	box := feM1Vec{feM2GoalX(home) - 8.0*feM1Dir(home), 0.0}
	return met*feM2HeaderConversion - (1.0-met)*e.feM2Concession(home, box)
}

// feM2ClearValue is DecisionValue.clear.
func (e *MatchEngine) feM2ClearValue(home bool, p feM1Vec) float64 {
	return -feM2ClearConcession * e.feM2Concession(home, p)
}

// feM2HeldScaled is held() with the loss scaled by share (ours, for the
// receiver of a pass).
func (e *MatchEngine) feM2HeldScaled(home bool, p feM1Vec, nearestOpponent float64, supported bool, share float64) float64 {
	loss := share * feM2LossNear * math.Exp(-nearestOpponent/feM2LossRange)
	if supported {
		loss *= 1.0 - feM2SupportRelief
	}
	return (1.0-loss)*e.feM2Threat(home, p) - loss*e.feM2Concession(home, p)
}

// feM2HeldValue is DecisionValue.held: a ball kept at p, which the nearest
// opponent may take, less so with a team-mate in support.
func (e *MatchEngine) feM2HeldValue(home bool, p feM1Vec, nearestOpponent float64, supported bool) float64 {
	loss := feM2LossNear * math.Exp(-nearestOpponent/feM2LossRange)
	if supported {
		loss *= 1.0 - feM2SupportRelief
	}
	return (1.0-loss)*e.feM2Threat(home, p) - loss*e.feM2Concession(home, p)
}

// feM2OpponentsNear counts the side's active opponents within radius of p.
func (e *MatchEngine) feM2OpponentsNear(home bool, p feM1Vec, radius float64) int {
	n := 0
	for i := range e.players {
		o := &e.players[i]
		if o.IsHomeTeam == home || !active(o) {
			continue
		}
		if feM1Dist(feM1ToM(o.Position), p) < radius {
			n++
		}
	}
	return n
}

// feM2Blockers is DecisionValue.blockers: opponents inside a 45° half-angle
// cone within CarryBlockRange. Ours: the cone points at the goal, the way
// feM2CarryValue advances (FE: along +x·dir).
func (e *MatchEngine) feM2Blockers(home bool, p feM1Vec) int {
	ux, uy, _ := feM2ToGoal(home, p)
	n := 0
	for i := range e.players {
		o := &e.players[i]
		if o.IsHomeTeam == home || !active(o) {
			continue
		}
		q := feM1ToM(o.Position)
		dx, dy := q.X-p.X, q.Y-p.Y
		ahead := dx*ux + dy*uy
		lateral := math.Abs(dx*uy - dy*ux)
		if ahead > 0.0 && ahead < feM2CarryBlockRange && lateral < ahead {
			n++
		}
	}
	return n
}

// feM2ShotBlockers is DecisionValue.shotBlockers: outfield opponents inside
// the ball→posts triangle widened by half a metre.
func (e *MatchEngine) feM2ShotBlockers(home bool, p feM1Vec) int {
	dir := feM1Dir(home)
	depth := (feM2GoalX(home) - p.X) * dir
	if depth <= 0.0 {
		return 0
	}
	const halfGoal = Shooting_GOAL_HALF_WIDTH_METRES
	n := 0
	for i := range e.players {
		o := &e.players[i]
		if o.IsHomeTeam == home || !active(o) || o.IsGoalkeeper {
			continue
		}
		q := feM1ToM(o.Position)
		t := (q.X - p.X) * dir / depth
		if t <= 0.0 || t >= 1.0 {
			continue
		}
		low := p.Y + (-halfGoal-p.Y)*t - 0.5
		high := p.Y + (halfGoal-p.Y)*t + 0.5
		if q.Y > low && q.Y < high {
			n++
		}
	}
	return n
}

// feM2LaneClear is Perception.LaneClear: fewer than two opponents within
// LaneClearBlockRadius of the carrier→receiver segment.
func (e *MatchEngine) feM2LaneClear(home bool, from, to feM1Vec) bool {
	dx, dy := to.X-from.X, to.Y-from.Y
	len2 := dx*dx + dy*dy
	blockers := 0
	for i := range e.players {
		o := &e.players[i]
		if o.IsHomeTeam == home || !active(o) {
			continue
		}
		q := feM1ToM(o.Position)
		var off2 float64
		if len2 < 1.0e-9 {
			off2 = (q.X-from.X)*(q.X-from.X) + (q.Y-from.Y)*(q.Y-from.Y)
		} else {
			t := ((q.X-from.X)*dx + (q.Y-from.Y)*dy) / len2
			if t < 0.0 || t > 1.0 {
				continue
			}
			px, py := from.X+dx*t-q.X, from.Y+dy*t-q.Y
			off2 = px*px + py*py
		}
		if off2 < e.cfg.M2.LaneClearRadius*e.cfg.M2.LaneClearRadius {
			blockers++
			if blockers >= 2 {
				return false
			}
		}
	}
	return true
}

// feM2Cognitive is FE Perception's cognitive fatigue factor from condition.
func feM2Cognitive(p *MatchPlayer) float64 {
	return feM2CognitiveFloor + (1.0-feM2CognitiveFloor)/
		(1.0+math.Exp(-feM2CognitiveSteepness*(p.Stamina-0.5)))
}
