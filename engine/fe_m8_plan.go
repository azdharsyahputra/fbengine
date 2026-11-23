// Ported from FootballEngine FootballEngine.Core/Match/Systems/Players/Planner.fs
// (state value, pass/carry expansion, root scoring; constants from
// Contracts/BalanceConfig.fs `defaultPlanner`), GPL-3.0.
// FootballEngine: https://github.com/DeltaBitsSystem/FootballEngine
//
// M8 lite: a one-step lookahead on top of the M2 options. Each pass and the
// carry is expanded once: the receiver (or the carrier after his carry) is
// put where the ball arrives and his own M2 options (passes, shot, carry) are
// valued there. A root option scores
//
//	max(utility, utility + confidence·(bestChildValue − rootValue))
//
// (Planner.fs root scoring with a beam of depth 2), so a pass to a man who
// then has a good pass or shot beats an equal pass to a dead end. Go
// simplifications (ours): depth 2 instead of FE's 4 with a beam of 3, and the
// other players are not projected along their velocities.
package engine

import "math"

const (
	// AnticipationConfidence.
	feM8Confidence = 0.65
	// DribbleDuration (s) and the carry speed scale (Planner CarrySpeed).
	feM8DribbleDuration = 0.5
	feM8SprintSpeed     = 8.5
)

// Tunables (FMENGINE_M2TUNE mc, mx).
// Calibration (confidence, carry expansion) is per match in EngineConfig.M8.

// feM8Refine raises each option's value to its planner score.
func (e *MatchEngine) feM8Refine(carrier *MatchPlayer, options []feM2Option, nearest float64) {
	home := carrier.IsHomeTeam
	here := feM1ToM(carrier.Position)
	root := e.feM2HeldValue(home, here, nearest, e.feM2Supported(carrier))
	// confidence = 0.65·Anticipation·cognitive·condition (Anticipation ← vision).
	confidence := e.cfg.M8.Confidence * carrier.Vision * feM2Cognitive(carrier) * hClamp(carrier.Stamina, 0.0, 1.0)
	if confidence <= 0.0 {
		return
	}
	for i := range options {
		o := &options[i]
		var child float64
		switch o.kind {
		case feM2KindPass:
			if o.pass.Intent == PassIntent_CROSS || o.pass.Receiver == nil {
				continue
			}
			child = e.feM8ChildValue(o.pass.Receiver, o.pass.TargetPoint, carrier)
		case feM2KindCarry:
			if !e.cfg.M8.ExpandCarry {
				continue
			}
			// The carry moves the ball and the carrier CarrySpeed·0.5 s toward
			// goal (Planner carry expansion; CarrySpeed = 8.5·Dribbling·condition).
			ux, uy, d := feM2ToGoal(home, here)
			step := math.Min(feM8SprintSpeed*carrier.Dribbling*hClamp(carrier.Stamina, 0.0, 1.0)*feM8DribbleDuration, d)
			ahead := feM1ToPitch(feM1Vec{here.X + ux*step, here.Y + uy*step})
			child = e.feM8ChildValue(carrier, ahead, nil)
		default:
			continue
		}
		if score := o.value + confidence*(child-root); score > o.value {
			o.value = score
		}
	}
}

// feM8ChildValue is the best M2 option value of actor with the ball at the
// pitch point at: his passes (skip excluded: FE expands unvisited team-mates
// only), a shot within range and a carry. The actor and the ball are moved
// there for the evaluation and put back.
func (e *MatchEngine) feM8ChildValue(actor *MatchPlayer, at Vector2F, skip *MatchPlayer) float64 {
	savedPos, savedBall := actor.Position, e.ball.Position
	actor.Position, e.ball.Position = at, at
	defer func() { actor.Position, e.ball.Position = savedPos, savedBall }()

	home := actor.IsHomeTeam
	p := feM1ToM(at)
	var buf [16]feM2Option
	best := math.Inf(-1)
	for _, o := range e.feM2PassOptions(actor, skip, buf[:0]) {
		best = math.Max(best, o.value)
	}
	if feM2DistanceToGoal(home, p) < feM2ShootingRange {
		xg := e.estimateShotXG(actor) *
			math.Pow(1.0-feM2ShotBlockShare, float64(e.feM2ShotBlockers(home, p)))
		best = math.Max(best, xg*e.cfg.M2.ShotValueScale)
	}
	if !actor.IsGoalkeeper {
		nearest := e.feM2NearestOpponent(home, p)
		keep := e.feM2CarryKeep(e.feM2OpponentsNear(home, p, feM2PressureRadius), e.feM2Blockers(home, p), nearest)
		best = math.Max(best, e.feM2CarryValue(home, keep, p))
	}
	return best
}
