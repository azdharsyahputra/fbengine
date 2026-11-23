// Ported from Player12 src/model/match_engine.cpp lines 5443–5944 (GPL-3.0)

package engine

import (
	"fmt"
	"math"
)

func (e *MatchEngine) decideAction(carrier *MatchPlayer) {
	// Decision layer: score pass/shot/carry/shield on comparable utility scales,
	// perturb close choices by decision quality, record the explanation, and
	// execute exactly one winner. Utilities are preferences, not probabilities.
	// The execution functions and subsequent contacts decide the outcome.
	if e.cfg.Modules.M6Challenge {
		e.feNoteDecision(carrier)
	}
	// FE M2: expected-value options and the pass race (fe_m2_decide.go).
	if e.cfg.Modules.M2Decision {
		e.feM2Decide(carrier)
		return
	}
	strategy := e.getEffectiveSliders(carrier.IsHomeTeam)
	pressure := hClamp((Decision_PRESSURE_RADIUS_METRES-
		e.nearestOpponentDistance(carrier))/
		Decision_PRESSURE_RADIUS_METRES,
		0.0, 1.0)
	shotXG := e.estimateShotXG(carrier)
	opennessAhead := e.openSpaceAhead(carrier)

	// Evaluate the passing candidates first: the task is pure (no random draws)
	// and the deterministic best plus runner-up options are visible in the
	// decision snapshot even when a shot or dribble is eventually chosen.
	option := e.choosePassTarget(carrier)

	// Every candidate is scored in a shared utility currency.
	// Vision (scanning and reading the play) dominates decision quality, with
	// passing technique contributing to how well options are judged.
	decisionQuality := carrier.Vision*Decision_VISION_DECISION_WEIGHT +
		carrier.Passing*(1.0-Decision_VISION_DECISION_WEIGHT)
	visionNoiseScale := (1.0 - decisionQuality) * Decision_VISION_NOISE_SCALE *
		(1.0 + (1.0-e.familiarityOf(carrier))*
			Decision_FAMILIARITY_NOISE_GAIN) *
		math.Max(0.5, 1.0-e.teamEdge(carrier.IsHomeTeam)) *
		(1.0 - e.talkSwing(carrier.IsHomeTeam)*TacticsTuning_TALK_COMPOSURE)

	passScore := math.Inf(-1)
	if option.HasValue {
		passScore = option.Value.Utility
		carrierDepth := carrier.Position.X
		if !carrier.IsHomeTeam {
			carrierDepth = 1.0 - carrier.Position.X
		}
		pinnedToByline := carrierDepth >= Rules_HOME_FINAL_THIRD_START &&
			math.Abs(Pitch_CENTRE-carrier.Position.Y) >=
				Decision_WIDE_SHOT_WIDTH_DEVIATION
		if pinnedToByline && option.Value.TargetPoint.X <= carrier.Position.X {
			passScore += Decision_WIDE_RECYCLE_BONUS
		}
	}

	shotScore := math.Inf(-1)
	if shotXG >= Decision_MIN_SHOT_XG {
		// The "have-a-go" inclination only matters from a credible shooting
		// range: it fades out completely for absurd-distance attempts, so the
		// scored decision can never invent a nonsensical long shot.
		rangeEligibility := hClamp((shotXG-Decision_SHOT_ELIGIBILITY_FLOOR)/
			Decision_SHOT_ELIGIBILITY_RANGE,
			0.0, 1.0)
		carrierDepth := carrier.Position.X
		if !carrier.IsHomeTeam {
			carrierDepth = 1.0 - carrier.Position.X
		}
		finalThirdBonus := 0.0
		if carrierDepth >= Rules_HOME_FINAL_THIRD_START {
			finalThirdBonus = Decision_FINAL_THIRD_SHOT_BONUS
		}
		shotScore = (shotXG-Decision_BASE_SHOT_THRESHOLD)*
			Decision_SHOT_SCORE_SCALE +
			Decision_SHOT_BASE_INCLINATION*
				(0.6+opennessAhead)*rangeEligibility +
			finalThirdBonus +
			carrier.Shooting*Decision_SHOT_SKILL_BONUS -
			pressure*Decision_SHOT_PRESSURE_PENALTY +
			e.shoutShotBias(carrier.IsHomeTeam)*rangeEligibility
		if carrierDepth >= Rules_HOME_FINAL_THIRD_START &&
			math.Abs(Pitch_CENTRE-carrier.Position.Y) >=
				Decision_WIDE_SHOT_WIDTH_DEVIATION {
			shotScore *= Decision_WIDE_SHOT_DISCOUNT
		}
		// A wide forward who has cut inside is looking for his shot.
		if e.cutsInside(carrier) {
			shotScore +=
				Decision_CUT_INSIDE_SHOT_BONUS * rangeEligibility
		}
		// Roles shoot more or less readily; a man forced onto his weaker foot
		// under pressure is less inclined to try.
		shotScore +=
			e.roleProfileOf(carrier).ShotBias*TacticsTuning_SHOT_BIAS_WEIGHT*
				rangeEligibility -
				e.weakFootPressure(carrier)*TacticsTuning_WEAK_FOOT_SHOT_PENALTY
	}

	// The longer a player has had the ball, the keener he is to release it:
	// individual possessions last about a second, rarely more than three.
	heldSeconds := e.getSimulatedSeconds() - e.possessionStartSeconds
	carryScore := opennessAhead*Decision_CARRY_OPENNESS_WEIGHT +
		carrier.Dribbling*Decision_CARRY_DRIBBLING_BONUS -
		pressure*Decision_CARRY_PRESSURE_PENALTY +
		strategy.RiskTaking*Decision_CARRY_RISK_BIAS -
		heldSeconds*Decision_CARRY_HOLD_PENALTY_PER_SECOND

	// A wide forward cutting inside wants to run at the box himself.
	if e.cutsInside(carrier) {
		carryScore += Decision_CUT_INSIDE_CARRY_BONUS
	}

	shieldScore := math.Inf(-1)
	if passUnavailableOrWeak := !option.HasValue ||
		passScore <= Passing_MIN_ACCEPTABLE_OPTION_SCORE; pressure >= Decision_SHIELD_PRESSURE_THRESHOLD &&
		passUnavailableOrWeak {
		shieldScore = pressure*Decision_SHIELD_BONUS +
			carrier.Dribbling*Decision_SHIELD_DRIBBLING
	}
	// FE M6 standoff breaker: a carrier held up too long must play.
	if e.cfg.Modules.M6Challenge && e.feStandoffActive(carrier) {
		shieldScore = math.Inf(-1)
	}

	// A side protecting a late lead no longer forces speculative long-range
	// attempts; it prefers to retain the ball. Credible chances are unaffected.
	carrierScore := e.awayScore
	opponentScore := e.homeScore
	if carrier.IsHomeTeam {
		carrierScore = e.homeScore
		opponentScore = e.awayScore
	}
	if e.matchTimeMinutes >= Decision_LATE_GAME_MINUTE &&
		carrierScore > opponentScore &&
		shotScore > math.Inf(-1) &&
		shotXG < Decision_BASE_SHOT_THRESHOLD {
		shotScore -= Decision_LATE_LEAD_SPECULATIVE_PENALTY
	}
	// Game state: a side two or more goals up manages the game, keeping the
	// ball rather than forcing half-chances.
	if lead := carrierScore - opponentScore; lead >= Decision_COMFORTABLE_LEAD {
		margin := float64(lead - Decision_COMFORTABLE_LEAD + 1)
		if shotScore > math.Inf(-1) &&
			shotXG < Decision_CLEAR_CHANCE_XG {
			shotScore -=
				Decision_COMFORTABLE_LEAD_SHOT_PENALTY * margin
		}
		carryScore -=
			Decision_COMFORTABLE_LEAD_CARRY_PENALTY * margin
	}

	// Vision scales how much randomness perturbs close choices. The noise is
	// bounded so a truly nonsensical option can never win.
	shotScore += visionNoiseScale * e.randomFloat(-1.0, 1.0)
	carryScore += visionNoiseScale * e.randomFloat(-1.0, 1.0)
	if shieldScore > math.Inf(-1) {
		shieldScore += visionNoiseScale * e.randomFloat(-1.0, 1.0)
	}
	if option.HasValue {
		passScore += visionNoiseScale * e.randomFloat(-1.0, 1.0)
	}

	e.lastScenarioDecision.PassUtility = passScore
	e.lastScenarioDecision.ShotUtility = shotScore
	e.lastScenarioDecision.CarryUtility = carryScore
	e.lastScenarioDecision.ShieldUtility = shieldScore

	e.lastScenarioDecision.Action = ScenarioAction_NONE
	if shotScore >= passScore && shotScore >= carryScore &&
		shotScore >= shieldScore {
		e.lastScenarioDecision.Action = ScenarioAction_SHOT
		e.lastScenarioDecision.Reason =
			fmt.Sprintf("shot with estimated xG %s", b4ToString(shotXG))
		e.takeShot(carrier, -1.0, false, false)
		return
	}
	if carryScore >= passScore && carryScore >= shieldScore {
		e.lastScenarioDecision.Action = ScenarioAction_CARRY
		if option.HasValue {
			e.lastScenarioDecision.Reason = fmt.Sprintf(
				`carry:"space ahead (value %s) beats the best pass with intent %s`,
				b4ToString(opennessAhead), passIntentName(option.Value.Intent))
		} else {
			e.lastScenarioDecision.Reason =
				"carry:space ahead and no acceptable pass candidate in range"
		}
		e.b4Carry(carrier)
		return
	}
	ownDepth := carrier.Position.X
	if !carrier.IsHomeTeam {
		ownDepth = 1.0 - carrier.Position.X
	}
	if shieldScore >= passScore &&
		ownDepth < Defending_CLEARANCE_MAX_DEPTH {
		e.lastScenarioDecision.Action = ScenarioAction_CLEAR
		e.lastScenarioDecision.Reason =
			"clear:pressed in the own third with no safe outlet"
		e.clearBall(carrier)
		return
	}
	if shieldScore >= passScore {
		e.lastScenarioDecision.Action = ScenarioAction_SHIELD
		e.lastScenarioDecision.Reason =
			"shield:retain and protect the ball under pressure with no safe " +
				"outlet"
		carrier.ActionCooldown =
			e.randomFloat(Decision_MIN_DRIBBLE_TIME,
				Decision_MAX_DRIBBLE_TIME)
		return
	}

	e.lastScenarioDecision.Action = ScenarioAction_PASS
	var receiverId PlayerID
	if option.Value.Receiver != nil && option.Value.Receiver.Player != nil {
		receiverId = option.Value.Receiver.Player.GetId()
	}
	e.lastScenarioDecision.Reason = fmt.Sprintf(
		"pass:%s to player %d (utility %s)", passIntentName(option.Value.Intent),
		receiverId,
		b4ToString(option.Value.Utility))
	e.passBall(carrier, &option.Value, false)
}

// b4Carry executes a carry decision (shared by the Player12 and FE M2 paths).
func (e *MatchEngine) b4Carry(carrier *MatchPlayer) {
	carrier.ActionCooldown =
		e.randomFloat(Decision_MIN_DRIBBLE_TIME,
			Decision_MAX_DRIBBLE_TIME)
	// A defender standing in the path must be beaten, not run through.
	if e.state == MatchState_PLAYING {
		blocker :=
			e.findClosestPlayer(carrier.Position, !carrier.IsHomeTeam, false)
		forward := -1.0
		if carrier.IsHomeTeam {
			forward = 1.0
		}
		if blocker != nil && blocker.TackleCooldown <= 0.0 &&
			distance(blocker.Position, carrier.Position) <
				Dribble_TAKE_ON_RANGE_METRES &&
			(blocker.Position.X-carrier.Position.X)*forward > 0.0 {
			if e.cfg.Modules.M6Challenge {
				e.feTakeOn(carrier, blocker)
			} else {
				e.attemptTakeOn(carrier, blocker)
			}
		}
	}
}

// b4ToString mirrors std::to_string(float): fixed notation, six decimals.
func b4ToString(value float64) string {
	return fmt.Sprintf("%f", value)
}

func (e *MatchEngine) evaluatePassOption(passer *MatchPlayer, receiver *MatchPlayer) PassOption {
	// Estimate one receiver's safety, progress and space without consuming RNG
	// or changing the match. The completion estimate ranks options; passBall
	// does not roll against it to guarantee a completed pass. Error at release,
	// ball flight and contested receptions determine whether it arrives.
	strategy := e.getEffectiveSliders(passer.IsHomeTeam)
	direction := -1.0
	if passer.IsHomeTeam {
		direction = 1.0
	}
	passDistance := distance(passer.Position, receiver.Position)
	progression := (receiver.Position.X - passer.Position.X) * direction
	openness := hClamp(e.nearestOpponentDistance(receiver)/
		Passing_OPENNESS_RADIUS_METRES,
		0.0, 1.0)
	laneRisk := e.passingLaneRisk(passer, receiver)
	pressure := hClamp((Decision_PRESSURE_RADIUS_METRES-
		e.nearestOpponentDistance(passer))/
		Decision_PRESSURE_RADIUS_METRES,
		0.0, 1.0)
	role := PlayerRole_CM
	if receiver.Player != nil {
		role = receiver.Player.GetRole()
	}
	forwardRole := role == PlayerRole_ST || role == PlayerRole_LW ||
		role == PlayerRole_RW || role == PlayerRole_CAM
	passerDepth := passer.Position.X
	if !passer.IsHomeTeam {
		passerDepth = 1.0 - passer.Position.X
	}
	receiverDepth := receiver.Position.X
	if !receiver.IsHomeTeam {
		receiverDepth = 1.0 - receiver.Position.X
	}
	widePasser := passer.Position.Y <= Passing_WIDE_ATTACK_MINIMUM_Y ||
		passer.Position.Y >= Passing_WIDE_ATTACK_MAXIMUM_Y
	centralReceiver := receiver.Position.Y >= Passing_CENTRAL_TARGET_MINIMUM_Y &&
		receiver.Position.Y <= Passing_CENTRAL_TARGET_MAXIMUM_Y
	cutbackOption := widePasser && centralReceiver &&
		passerDepth >= Passing_CUTBACK_MINIMUM_PASSER_DEPTH &&
		receiverDepth >= Passing_CROSS_MINIMUM_RECEIVER_DEPTH &&
		progression >= Passing_CUTBACK_MINIMUM_PROGRESSION &&
		progression <= Passing_CUTBACK_MAXIMUM_PROGRESSION
	crossOption := !cutbackOption && widePasser && centralReceiver &&
		passerDepth >= Passing_CROSS_MINIMUM_PASSER_DEPTH &&
		receiverDepth >= Passing_CROSS_MINIMUM_RECEIVER_DEPTH
	safeOutlet := pressure * math.Max(0.0, -progression) *
		Passing_SAFE_OUTLET_WEIGHT
	// A high defensive line leaves space in behind: a forward already running
	// into it is worth finding, and the ball is easier to play there.
	spaceBehindShare := 0.0
	// Any player running in behind is worth finding, a midfielder arriving
	// late as much as a striker.
	if receiver.IsMakingRun && progression > 0.0 {
		line := e.offsideLine(passer.IsHomeTeam)
		behind := line
		if passer.IsHomeTeam {
			behind = 1.0 - line
		}
		spaceBehind := behind * Pitch_LENGTH_METRES
		spaceBehindShare = hClamp(
			(spaceBehind-Passing_SPACE_BEHIND_MIN_METRES)/
				Passing_SPACE_BEHIND_RANGE_METRES,
			0.0, 1.0)
	}
	crossPenalty := 0.0
	if crossOption {
		crossPenalty = Passing_CROSS_COMPLETION_PENALTY
	}
	cutbackBonus := 0.0
	if cutbackOption {
		cutbackBonus = Passing_CUTBACK_COMPLETION_BONUS
	}
	completionProbability := hClamp(
		Passing_BASE_COMPLETION_PROBABILITY+
			passer.Passing*Passing_PASSING_COMPLETION_BONUS+
			openness*Passing_OPENNESS_COMPLETION_BONUS-
			laneRisk*Passing_LANE_COMPLETION_PENALTY-
			// Completion falls steeply for long passes (~86% at 10-20 m,
			// ~41% beyond 40 m).
			passDistance*passDistance/
				(Passing_LONG_PASS_REFERENCE_METRES*
					Passing_LONG_PASS_REFERENCE_METRES)*
				Passing_DISTANCE_COMPLETION_PENALTY-
			pressure*Passing_PRESSURE_COMPLETION_PENALTY-
			crossPenalty+
			cutbackBonus+
			spaceBehindShare*
				Passing_SPACE_BEHIND_COMPLETION_BONUS,
		Passing_MIN_COMPLETION_PROBABILITY,
		Passing_MAX_COMPLETION_PROBABILITY)
	// Risk-taking trades safety for progress: a patient side values keeping
	// the ball, a daring one the metres gained.
	daring := strategy.RiskTaking - 0.5
	forwardBonus := 0.0
	if forwardRole && progression > 0.0 {
		forwardBonus = Passing_FORWARD_ROLE_BONUS
	}
	utility := openness*Passing_OPENNESS_WEIGHT -
		laneRisk*Passing_LANE_RISK_WEIGHT +
		progression*
			(Passing_BASE_PROGRESS_WEIGHT+
				strategy.OffensiveBias*
					Passing_OFFENSIVE_PROGRESS_WEIGHT)*
			(1.0+daring*Passing_RISK_PROGRESS_GAIN) -
		math.Abs(passDistance-Passing_IDEAL_DISTANCE_METRES)*
			Passing_DISTANCE_PENALTY_PER_METRE +
		safeOutlet +
		forwardBonus +
		completionProbability*Passing_COMPLETION_UTILITY_WEIGHT*
			(1.0-daring*Passing_RISK_SAFETY_GAIN)
	if receiver.IsMakingRun && progression > 0.0 {
		utility += Passing_ACTIVE_RUNNER_UTILITY_BONUS
	}
	utility +=
		e.passingRoleBias(passer, receiver, progression, completionProbability)
	// In the final third a team-mate in a shooting position is worth finding.
	if receiverDepth >= Rules_HOME_FINAL_THIRD_START {
		utility +=
			e.estimateShotXG(receiver) * Passing_SHOT_CREATION_WEIGHT
	}
	utility += spaceBehindShare * Passing_SPACE_BEHIND_UTILITY
	if crossOption {
		utility += Passing_CROSS_UTILITY_BONUS
	}
	if cutbackOption {
		utility += Passing_CUTBACK_UTILITY_BONUS
	}

	intent := PassIntent_RECYCLE
	if cutbackOption {
		intent = PassIntent_CUTBACK
	} else if crossOption {
		intent = PassIntent_CROSS
	} else if pressure >= Passing_PRESSURE_RELEASE_THRESHOLD &&
		progression <=
			Passing_PRESSURE_RELEASE_MAX_PROGRESSION {
		intent = PassIntent_PRESSURE_RELEASE
	} else if math.Abs(receiver.Position.Y-passer.Position.Y) >=
		Passing_SWITCH_PLAY_MINIMUM_WIDTH {
		intent = PassIntent_SWITCH_PLAY
	} else if receiver.IsMakingRun &&
		progression >=
			Passing_THROUGH_BALL_MINIMUM_PROGRESSION {
		intent = PassIntent_THROUGH_BALL
	} else if progression >= Passing_PROGRESSIVE_PASS_MINIMUM {
		intent = PassIntent_PROGRESSIVE
	}

	flightEstimate := passDistance / Passing_ESTIMATED_BALL_SPEED
	lead := toPitch(Vector2F{receiver.Velocity.X * flightEstimate *
		Passing_RECEIVER_LEAD_SCALE,
		receiver.Velocity.Y * flightEstimate *
			Passing_RECEIVER_LEAD_SCALE})
	target := Vector2F{hClamp(receiver.Position.X+lead.X, 0.0, 1.0),
		hClamp(receiver.Position.Y+lead.Y, 0.0, 1.0)}
	if intent == PassIntent_THROUGH_BALL {
		target.X += (receiver.MovementTarget.X - target.X) *
			Passing_THROUGH_BALL_TARGET_BLEND
		target.Y += (receiver.MovementTarget.Y - target.Y) *
			Passing_THROUGH_BALL_TARGET_BLEND
		target.X += direction * Passing_THROUGH_BALL_FORWARD_LEAD
		target.X = hClamp(target.X, Pitch_PLAYER_MIN_X,
			Pitch_PLAYER_MAX_X)
		target.Y = hClamp(target.Y, Pitch_PLAYER_MIN_Y,
			Pitch_PLAYER_MAX_Y)
	} else if intent == PassIntent_CROSS {
		target.X += (receiver.MovementTarget.X - target.X) *
			Passing_CROSS_TARGET_BLEND
		target.Y += (receiver.MovementTarget.Y - target.Y) *
			Passing_CROSS_TARGET_BLEND
	}
	travelDistance := distance(passer.Position, target)

	return PassOption{
		Receiver:              receiver,
		TargetPoint:           target,
		Intent:                intent,
		Utility:               utility,
		Progression:           progression,
		LaneRisk:              laneRisk,
		CompletionProbability: completionProbability,
		PassDistance:          travelDistance,
		Lofted: !receiver.IsGoalkeeper &&
			(intent == PassIntent_CROSS ||
				travelDistance > Passing_LOFTED_DISTANCE_METRES),
	}
}

// b4ToDecision is choosePassTarget's toDecision lambda.
func b4ToDecision(option *PassOption) PassDecision {
	var receiverId PlayerID
	if option.Receiver != nil && option.Receiver.Player != nil {
		receiverId = option.Receiver.Player.GetId()
	}
	return PassDecision{
		PasserId:              0,
		ReceiverId:            receiverId,
		TargetPoint:           option.TargetPoint,
		Intent:                option.Intent,
		Utility:               option.Utility,
		Progression:           option.Progression,
		LaneRisk:              option.LaneRisk,
		CompletionProbability: option.CompletionProbability,
	}
}

func (e *MatchEngine) choosePassTarget(passer *MatchPlayer) Optional[PassOption] {
	var best Optional[PassOption]
	bestScore := math.Inf(-1)
	var runnerUp Optional[PassOption]
	runnerUpScore := math.Inf(-1)

	defenderLine := e.offsideLine(passer.IsHomeTeam)
	for i := range e.players {
		candidate := &e.players[i]
		if candidate == passer || candidate.IsHomeTeam != passer.IsHomeTeam ||
			!active(candidate) {
			continue
		}

		passDistance := distance(passer.Position, candidate.Position)
		if passDistance < Passing_MIN_DISTANCE_METRES ||
			passDistance > Passing_MAX_DISTANCE_METRES {
			continue
		}
		// Nobody plays a long ball back towards his own goal.
		if candidate.IsGoalkeeper &&
			passDistance > Passing_MAX_BACK_PASS_METRES {
			continue
		}
		// The passer judges the offside line from what he sees: poorer vision
		// misreads tight lines, which is where real offsides come from.
		if perceivedLineError := e.hashNoise(passer.Player.GetId(), candidate.Player.GetId()) *
			(1.0 - passer.Vision) *
			Passing_OFFSIDE_PERCEPTION_ERROR; e.isOffside(candidate, passer.IsHomeTeam, defenderLine,
			perceivedLineError) {
			continue
		}

		option := e.evaluatePassOption(passer, candidate)
		if option.Utility > bestScore {
			runnerUp = best
			runnerUpScore = bestScore
			bestScore = option.Utility
			best = hSome(option)
		} else if option.Utility > runnerUpScore {
			runnerUpScore = option.Utility
			runnerUp = hSome(option)
		}
	}

	e.lastScenarioDecision.Best = Optional[PassDecision]{}
	e.lastScenarioDecision.RunnerUp = Optional[PassDecision]{}
	if best.HasValue && best.Value.Receiver != nil && best.Value.Receiver.Player != nil {
		e.lastScenarioDecision.Best = hSome(b4ToDecision(&best.Value))
	}
	if runnerUp.HasValue && runnerUp.Value.Receiver != nil && runnerUp.Value.Receiver.Player != nil {
		e.lastScenarioDecision.RunnerUp = hSome(b4ToDecision(&runnerUp.Value))
	}

	if bestScore < Passing_MIN_ACCEPTABLE_OPTION_SCORE {
		return Optional[PassOption]{}
	}
	return best
}
