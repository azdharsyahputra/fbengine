// Ported from FootballEngine FootballEngine.Core/Match/Systems/Players/Decision.fs
// (passOptionsInto, fillPlan, finish), GPL-3.0.
// FootballEngine: https://github.com/DeltaBitsSystem/FootballEngine
//
// M2 decision: every carrier option valued in goal probability (fe_m2_value.go),
// pass completion from the ball-vs-bodies race (fe_m2_race.go), one softmax
// choice at FE's temperature. It replaces Player12's decideAction scoring,
// evaluatePassOption utility and passingLaneRisk; Player12's execution
// (takeShot, passBall, clearBall, the carry/take-on path), its pass intent
// classification and target lead are kept. Player12 utility-unit bonuses
// (WIDE_RECYCLE_BONUS, role biases, SHOT_CREATION_WEIGHT, safeOutlet, vision
// noise) are not used on this path (plan §4 risk 1).
package engine

import (
	"fmt"
	"math"
)

const (
	// ours: forced-release times (s) on the ball; see feM2Decide.
	feM2HoldLimit     = 8.0
	feM2KeeperRelease = 5.0
	// ours (M10): a lead this far (m) from the receiver gets a to-feet retry.
	feM2FeetRetry = 1.5
)

const (
	feM2KindPass = iota
	feM2KindShot
	feM2KindCarry
	feM2KindClear
	feM2KindShield
)

type feM2Option struct {
	kind  int
	value float64
	pass  PassOption
	terms feM2PassTerms // pass options: the value's named terms (trace)
}

// feM2PassOptions is Decision.passOptionsInto: every receiver the lane, the
// offside line and the race allow, valued by DecisionValue.pass. FE's
// FaceCone filter is deliberately not ported (plan M2.2: it biases passes
// backward).
// skip (may be nil) is a team-mate left out (the M8 planner's visited passer).
func (e *MatchEngine) feM2PassOptions(carrier, skip *MatchPlayer, out []feM2Option) []feM2Option {
	home := carrier.IsHomeTeam
	from := feM1ToM(carrier.Position)
	ballM := feM1ToM(e.ball.Position)
	line := e.offsideLine(home)
	var roll feM2Roll
	for i := range e.players {
		cand := &e.players[i]
		// FE: never the carrier or the keeper.
		if cand == carrier || cand == skip || cand.IsHomeTeam != home || !active(cand) || cand.IsGoalkeeper {
			continue
		}
		e.m2trace.ahead = (cand.Position.X-carrier.Position.X)*Pitch_LENGTH_METRES*feM1Dir(home) > 3.0
		if e.diag.M2Trace == 3 {
			c := cand
			e.m2trace.candLog = func(i int) {
				cm := feM1ToM(c.Position)
				fmt.Fprintf(e.diag.out(), "CAND r=%d tx=%.1f ahead=%.1f d=%.1f role=%d intent=%d near=%.1f\n", i,
					from.X*feM1Dir(home), (cm.X-from.X)*feM1Dir(home), feM1Dist(from, cm), c.Player.GetRole(), c.Intent,
					e.feM2NearestOpponent(home, cm))
			}
		}
		d := distance(carrier.Position, cand.Position)
		if d < Passing_MIN_DISTANCE_METRES || d > Passing_MAX_DISTANCE_METRES {
			e.feM2Count(0)
			continue
		}
		// Offside as the passer reads it (Player12's vision-dependent misread;
		// FE reads the line 1 m generously, OffsideReadMargin).
		misread := e.hashNoise(carrier.Player.GetId(), cand.Player.GetId()) *
			(1.0 - carrier.Vision) * Passing_OFFSIDE_PERCEPTION_ERROR
		if e.isOffside(cand, home, line, misread+feM2OffsideReadMargin()) {
			e.feM2Count(1)
			continue
		}
		// Player12's intent classification and lead target (its utility is
		// replaced below).
		opt := e.evaluatePassOption(carrier, cand)
		// Ground lane only for a ground pass (ours): a cross or lofted ball
		// goes over the bodies in between; the race still contests the drop.
		// Phase 2 (B2): the shared lane definition; M2 rejects only a lane
		// with two bodies in it (FE Perception.LaneClear), the race prices one.
		if !opt.Lofted && e.spatialLaneBlockers(home, from, feM1ToM(cand.Position), feM2LaneRadius) >= 2 {
			e.feM2Count(2)
			continue
		}
		to := feM1ToM(opt.TargetPoint)
		travel := feM1Dist(ballM, to)
		delivery := opt.Intent == PassIntent_CROSS
		speed, vertical := feM2Launch(carrier, travel, opt.Lofted, delivery)
		feM2BuildRoll(&roll, ballM, to, speed, vertical, opt.Lofted)
		margin := e.feM2RaceMargin(carrier, cand, &roll, line)
		if margin < e.cfg.M2.PassMinimumMargin && !opt.Lofted && !delivery && e.cfg.M2.ToFeet &&
			distance(opt.TargetPoint, cand.Position) > feM2FeetRetry/Pitch_LENGTH_METRES {
			// To feet (ours): the lead into space is cut off, but the
			// receiver can stop and take it where he stands (m2.to_feet).
			feet := opt
			feet.TargetPoint = cand.Position
			fto := feM1ToM(feet.TargetPoint)
			fs, fv := feM2Launch(carrier, feM1Dist(ballM, fto), false, false)
			feM2BuildRoll(&roll, ballM, fto, fs, fv, false)
			if m := e.feM2RaceMargin(carrier, cand, &roll, line); m >= e.cfg.M2.PassMinimumMargin {
				opt, to, margin = feet, fto, m
			}
		}
		if margin < e.cfg.M2.PassMinimumMargin {
			e.feM2Count(3)
			continue
		}
		e.feM2Count(4)
		completion := e.feM2PassCompletion(carrier.Passing, margin)
		if opt.Lofted {
			completion *= e.cfg.M2.LoftedCeiling
		}
		// The pass value is M2's alone, as named terms (fe_m2_terms.go):
		// FE's expected value (core), the M5 cooperation bias and the
		// progress appetite.
		var terms feM2PassTerms
		if delivery {
			terms.core = e.feM2CrossValue(home, (carrier.Passing+carrier.Vision)*0.5)
		} else {
			// ours: the ball arriving at a marked receiver is worth FE's held()
			// there (the M8 planner's state value), not the bare threat: the
			// man on his back can still take it while he controls and turns.
			arrived := e.feM2Threat(home, to)
			if e.cfg.M2.ReceiverLoss > 0.0 {
				arrived = e.feM2HeldScaled(home, to, e.feM2NearestOpponent(home, to),
					e.feM2SupportedAt(home, to, carrier, cand), e.cfg.M2.ReceiverLoss)
			}
			terms.core = e.feM2PassValue(home, completion, arrived, from, to)
		}
		// FE M5 collective duty bias (Decision.fs:331-376), already in
		// goal-probability units (fe_m5_collective.go).
		terms.cooperation = e.FeM5PassBias(carrier, cand)
		terms.progress = e.feM2ProgressTerm(from.X*feM1Dir(home)) * (to.X - from.X) * feM1Dir(home)
		value := terms.sum()
		opt.Utility = value
		opt.CompletionProbability = completion
		opt.LaneRisk = 1.0 - completion
		out = append(out, feM2Option{kind: feM2KindPass, value: value, pass: opt, terms: terms})
	}
	return out
}

// feM2OffsideReadMargin is FE OffsideReadMargin (1 m) in pitch units.
func feM2OffsideReadMargin() float64 { return 1.0 / Pitch_LENGTH_METRES }

// feM2Decide is Decision.fillPlan + finish on the live carrier: build the
// option set, choose by softmax, execute through Player12's actions.
func (e *MatchEngine) feM2Decide(carrier *MatchPlayer) {
	e.feM2TraceResolve(carrier)
	home := carrier.IsHomeTeam
	dir := feM1Dir(home)
	here := feM1ToM(carrier.Position)
	strategy := e.getEffectiveSliders(home)
	nearest := e.nearestOpponentDistance(carrier)
	pressure := e.feM2OpponentsNear(home, here, feM2PressureRadius)
	cognitive := feM2Cognitive(carrier)
	riskShift := (strategy.RiskTaking - 0.5) * 2.0 * feM2MentalityRiskShift

	var buf [16]feM2Option
	options := e.feM2PassOptions(carrier, nil, buf[:0])

	// Best and runner-up passes for the decision snapshot.
	bestPass, runnerUp := -1, -1
	for i := range options {
		if bestPass < 0 || options[i].value > options[bestPass].value {
			runnerUp = bestPass
			bestPass = i
		} else if runnerUp < 0 || options[i].value > options[runnerUp].value {
			runnerUp = i
		}
	}
	e.lastScenarioDecision.Best = Optional[PassDecision]{}
	e.lastScenarioDecision.RunnerUp = Optional[PassDecision]{}
	if bestPass >= 0 {
		e.lastScenarioDecision.Best = hSome(b4ToDecision(&options[bestPass].pass))
	}
	if runnerUp >= 0 {
		e.lastScenarioDecision.RunnerUp = hSome(b4ToDecision(&options[runnerUp].pass))
	}

	// Shot: in range and facing within ShotTurnLimit of goal. The belief is
	// the engine's own xG (what strikeShot will convert at), discounted per
	// body in the shooting triangle (ShotBlockShare).
	shotValue := math.Inf(-1)
	shotXG := 0.0
	if feM2DistanceToGoal(home, here) < feM2ShootingRange {
		// FacingAngle lives in normalised pitch space (b3_movement).
		toGoal := math.Atan2(-here.Y/Pitch_WIDTH_METRES, (feM2GoalX(home)-here.X)/Pitch_LENGTH_METRES)
		turn := math.Abs(math.Remainder(toGoal-carrier.FacingAngle, 2.0*math.Pi))
		if turn <= feM2ShotTurnLimit {
			shotXG = e.estimateShotXG(carrier) *
				math.Pow(1.0-feM2ShotBlockShare, float64(e.feM2ShotBlockers(home, here)))
			shotValue = shotXG*e.cfg.M2.ShotValueScale + feM2StyleValue*riskShift
			options = append(options, feM2Option{kind: feM2KindShot, value: shotValue})
		}
	}

	// Carry: attribute-free keep, from the pressure and the block ahead. A
	// keeper with the ball at his feet does not dribble out (ours: FE's keeper
	// distributes through its own system); he passes or kicks long.
	keep := e.feM2CarryKeep(pressure, e.feM2Blockers(home, here), nearest)
	carryValue := math.Inf(-1)
	// Forced release (ours): a keeper distributes within the law's six
	// seconds and an outfielder on the ball for feM2HoldLimit (stuck at a
	// flag, held up) must pass, cross, shoot or kick it away.
	held := e.getSimulatedSeconds() - e.possessionStartSeconds
	release := held >= feM2HoldLimit || (carrier.IsGoalkeeper && held >= feM2KeeperRelease)
	if !carrier.IsGoalkeeper && !release {
		carryValue = e.feM2CarryValue(home, keep, here) + feM2StyleValue*riskShift
		options = append(options, feM2Option{kind: feM2KindCarry, value: carryValue})
	}

	// Clear: own third, an opponent within PressureRadius (a keeper always
	// may).
	if (here.X*dir < feM2OwnThirdX && (pressure > 0 || carrier.IsGoalkeeper)) || release {
		options = append(options, feM2Option{kind: feM2KindClear, value: e.feM2ClearValue(home, here)})
	}

	// Shield (Player12 action, valued by DecisionValue.held): only under
	// pressure, and never once the M6 standoff breaker has fired.
	shieldValue := math.Inf(-1)
	if e.cfg.M2.Shield && !release && pressure > 0 && !carrier.IsGoalkeeper && !(e.cfg.Modules.M6Challenge && e.feStandoffActive(carrier)) {
		shieldValue = e.feM2HeldValue(home, here, nearest, e.feM2Supported(carrier))
		options = append(options, feM2Option{kind: feM2KindShield, value: shieldValue})
	}

	// FE M8 (lite): one-step lookahead on the passes and the carry.
	if e.cfg.Modules.M8Planner {
		e.feM8Refine(carrier, options, nearest)
	}

	if len(options) == 0 {
		options = append(options, feM2Option{kind: feM2KindClear, value: e.feM2ClearValue(home, here)})
	}

	// Softmax at FE's temperature, colder when tired (effectiveTemperature).
	temperature := feM2Temperature * (0.4 + 0.6*cognitive)
	maximum := math.Inf(-1)
	for i := range options {
		maximum = math.Max(maximum, options[i].value)
	}
	sum := 0.0
	for i := range options {
		sum += math.Exp((options[i].value - maximum) / temperature)
	}
	draw := e.randomFloat(0.0, 1.0)
	chosen := len(options) - 1
	acc := 0.0
	for i := range options {
		acc += math.Exp((options[i].value-maximum)/temperature) / sum
		if draw <= acc {
			chosen = i
			break
		}
	}
	choice := &options[chosen]

	passValue := math.Inf(-1)
	if bestPass >= 0 {
		passValue = options[bestPass].value
	}
	e.lastScenarioDecision.PassUtility = passValue
	e.lastScenarioDecision.ShotUtility = shotValue
	e.lastScenarioDecision.CarryUtility = carryValue
	e.lastScenarioDecision.ShieldUtility = shieldValue
	e.lastScenarioDecision.Action = ScenarioAction_NONE

	e.auditCarryDecision(carrier, choice.kind == feM2KindCarry, carryValue, passValue)
	switch choice.kind {
	case feM2KindShot:
		e.lastScenarioDecision.Action = ScenarioAction_SHOT
		e.lastScenarioDecision.Reason = fmt.Sprintf("shot with believed xG %s", b4ToString(shotXG))
		e.takeShot(carrier, -1.0, false, false)
	case feM2KindCarry:
		e.lastScenarioDecision.Action = ScenarioAction_CARRY
		e.lastScenarioDecision.Reason = fmt.Sprintf("carry:keep %s value %s beats the best pass %s",
			b4ToString(keep), b4ToString(carryValue), b4ToString(passValue))
		e.b4Carry(carrier)
	case feM2KindClear:
		e.lastScenarioDecision.Action = ScenarioAction_CLEAR
		e.lastScenarioDecision.Reason = "clear:pressed in the own third, every other option worth less"
		e.clearBall(carrier)
	case feM2KindShield:
		e.lastScenarioDecision.Action = ScenarioAction_SHIELD
		e.lastScenarioDecision.Reason = "shield:holding the ball is worth more than any release"
		carrier.ActionCooldown = e.randomFloat(Decision_MIN_DRIBBLE_TIME, Decision_MAX_DRIBBLE_TIME)
	default:
		opt := &choice.pass
		e.lastScenarioDecision.Action = ScenarioAction_PASS
		var receiverId PlayerID
		if opt.Receiver != nil && opt.Receiver.Player != nil {
			receiverId = opt.Receiver.Player.GetId()
		}
		e.lastScenarioDecision.Reason = fmt.Sprintf("pass:%s to player %d (value %s, completion %s)",
			passIntentName(opt.Intent), receiverId, b4ToString(opt.Utility),
			b4ToString(opt.CompletionProbability))
		e.passBall(carrier, opt, false)
	}
	e.feM2Trace(carrier, choice, options, keep, pressure)
}

// feM2Supported: a team-mate within SupportDistance of the carrier.
func (e *MatchEngine) feM2Supported(carrier *MatchPlayer) bool {
	for i := range e.players {
		p := &e.players[i]
		if p != carrier && p.IsHomeTeam == carrier.IsHomeTeam && active(p) &&
			distance(p.Position, carrier.Position) < feM2SupportDistance {
			return true
		}
	}
	return false
}
