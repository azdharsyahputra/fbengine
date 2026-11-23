// Ported from Player12 src/model/match_engine.cpp lines 2398–2614 (GPL-3.0)

package engine

import "math"

// b2ContainsRunner mirrors std::ranges::find(runners, p) != runners.end().
func b2ContainsRunner(runners [Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer, p *MatchPlayer) bool {
	return b2IndexOf(runners[:], p) >= 0
}

// b2IndexOf returns the index of p in list (std::ranges::find), or -1.
func b2IndexOf(list []*MatchPlayer, p *MatchPlayer) int {
	for i, candidate := range list {
		if candidate == p {
			return i
		}
	}
	return -1
}

func (e *MatchEngine) selectAttackingRunners(homeTeam bool, carrier *MatchPlayer, transitionSource *MatchPlayer) [Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer {
	var runners [Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer
	var scores [Shape_MAX_COMMITTED_RUNNERS]float64
	for i := range scores {
		scores[i] = math.Inf(-1)
	}
	possessionContext := carrier != nil && carrier.IsHomeTeam == homeTeam
	looseTransitionContext := carrier == nil && !e.ball.IsPass &&
		transitionSource != nil &&
		transitionSource.IsHomeTeam == homeTeam
	if !possessionContext && !looseTransitionContext {
		return runners
	}

	sliders := e.getEffectiveSliders(homeTeam)
	attackingPosition := e.ball.Position.X
	if carrier != nil {
		attackingPosition = carrier.Position.X
	}
	attackingProgress := attackingPosition
	if !homeTeam {
		attackingProgress = 1.0 - attackingPosition
	}
	attackingCommitment := sliders.OffensiveBias + sliders.RiskTaking
	phase := e.awayPhase
	if homeTeam {
		phase = e.homePhase
	}
	lead := e.awayScore - e.homeScore
	if homeTeam {
		lead = e.homeScore - e.awayScore
	}
	managingGame := lead >= Decision_COMFORTABLE_LEAD
	runnerCount := Shape_MIN_COMMITTED_RUNNERS
	if !managingGame &&
		(phase == TeamPhase_ATTACKING_TRANSITION ||
			attackingProgress >= Shape_SECOND_RUNNER_PROGRESS_THRESHOLD ||
			attackingCommitment >= Shape_SECOND_RUNNER_ATTACK_THRESHOLD) {
		runnerCount = Shape_MAX_COMMITTED_RUNNERS
	}
	if e.cfg.Modules.M5Collective && possessionContext {
		// Merge with FE M5 (ours): the episode supplies the Link and the
		// Runner, so P12 commits one run in settled possession and two in a
		// counter or the final third; the other forwards hold width and
		// depth (spec §3) instead of all three running every phase.
		runnerCount = min(runnerCount, Shape_MIN_COMMITTED_RUNNERS)
		if phase == TeamPhase_ATTACKING_TRANSITION || phase == TeamPhase_FINAL_THIRD {
			runnerCount = min(Shape_MAX_COMMITTED_RUNNERS, Shape_MIN_COMMITTED_RUNNERS+1)
		}
	}

	// The best runs by priority, with at most one striker among them: the
	// other striker comes short and the late runs come from midfield.
	strikers := 0
	for slot := 0; slot < runnerCount; slot++ {
		for i := range e.players {
			candidate := &e.players[i]
			if !active(candidate) || candidate.IsHomeTeam != homeTeam {
				continue
			}
			if candidate == carrier || candidate.IsInjured ||
				b2ContainsRunner(runners, candidate) || e.feM5HasDuty(candidate) {
				continue
			}
			striker := candidate.Player.GetRole() == PlayerRole_ST
			if striker && strikers >= Shape_MAX_STRIKER_RUNNERS {
				continue
			}
			score := e.runPriority(candidate, carrier)
			if score > scores[slot] {
				scores[slot] = score
				runners[slot] = candidate
			}
		}
		if runners[slot] != nil && runners[slot].Player.GetRole() == PlayerRole_ST {
			strikers++
		}
	}
	return runners
}

func (e *MatchEngine) runChannel(runner *MatchPlayer) float64 {
	role := runner.Player.GetRole()
	if role == PlayerRole_LW {
		return Shape_LEFT_INSIDE_FORWARD_CHANNEL
	}
	if role == PlayerRole_RW {
		return Shape_RIGHT_INSIDE_FORWARD_CHANNEL
	}
	if role == PlayerRole_LM {
		return Shape_LEFT_WIDE_ATTACK_CHANNEL
	}
	if role == PlayerRole_RM {
		return Shape_RIGHT_WIDE_ATTACK_CHANNEL
	}
	if runner.BasePosition.Y < Pitch_CENTRE {
		return Shape_LEFT_STRIKER_ATTACK_CHANNEL
	}
	if runner.BasePosition.Y > Pitch_CENTRE {
		return Shape_RIGHT_STRIKER_ATTACK_CHANNEL
	}
	return Shape_CENTRAL_ATTACK_CHANNEL
}

func (e *MatchEngine) selectFinalThirdPlayer(homeTeam bool, arrival TacticalArrival, carrier *MatchPlayer, homeRunners [Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer, awayRunners [Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer) *MatchPlayer {
	if carrier == nil || carrier.IsHomeTeam != homeTeam {
		return nil
	}
	phase := e.awayPhase
	if homeTeam {
		phase = e.homePhase
	}
	if phase != TeamPhase_FINAL_THIRD {
		return nil
	}
	ballSide := carrier.Position.Y - Pitch_CENTRE
	if arrival == TacticalArrival_FAR_POST &&
		math.Abs(ballSide) < Shape_FAR_POST_MIN_BALL_WIDTH {
		return nil
	}

	var selected *MatchPlayer
	bestScore := math.MaxFloat64
	for i := range e.players {
		candidate := &e.players[i]
		if !active(candidate) || candidate.IsHomeTeam != homeTeam {
			continue
		}
		if candidate == carrier || candidate.IsInjured {
			continue
		}
		runners := awayRunners
		if homeTeam {
			runners = homeRunners
		}
		if b2ContainsRunner(runners, candidate) {
			continue
		}
		role := candidate.Player.GetRole()
		wide := role == PlayerRole_LM || role == PlayerRole_RM || role == PlayerRole_LW || role == PlayerRole_RW
		farSide := (candidate.BasePosition.Y-Pitch_CENTRE)*ballSide < 0.0
		eligible := false
		if arrival == TacticalArrival_FULLBACK {
			eligible = role == PlayerRole_LB || role == PlayerRole_RB
		} else if arrival == TacticalArrival_FAR_POST {
			eligible = wide && farSide
		} else {
			eligible = role == PlayerRole_CM || role == PlayerRole_CAM
		}
		if !eligible {
			continue
		}

		overlapBias := 0.0
		if arrival == TacticalArrival_FULLBACK {
			overlapBias = e.roleProfileOf(candidate).OverlapBias
		}
		if overlapBias <= -1.0 {
			continue
		}
		score := math.Abs(candidate.BasePosition.Y-carrier.Position.Y) -
			overlapBias*TacticsTuning_OVERLAP_BIAS_WEIGHT
		if arrival == TacticalArrival_FAR_POST {
			score = -score
		}
		continuityIntent := PlayerIntent_ATTACK_BOX
		if arrival == TacticalArrival_FULLBACK {
			continuityIntent = PlayerIntent_OVERLAP
		}
		if candidate.Intent == continuityIntent {
			score -= Shape_FINAL_THIRD_SELECTION_CONTINUITY
		}
		if score < bestScore {
			bestScore = score
			selected = candidate
		}
	}
	return selected
}

func (e *MatchEngine) selectActiveSupporters(homeTeam bool, carrier *MatchPlayer, runners [Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer, midfieldArrival *MatchPlayer, farPostRunner *MatchPlayer, overlappingFullback *MatchPlayer) [Shape_MAX_ACTIVE_SUPPORTERS]*MatchPlayer {
	var supporters [Shape_MAX_ACTIVE_SUPPORTERS]*MatchPlayer
	var scores [Shape_MAX_ACTIVE_SUPPORTERS]float64
	for i := range scores {
		scores[i] = math.MaxFloat64
	}
	if carrier == nil || carrier.IsHomeTeam != homeTeam {
		return supporters
	}
	for i := range e.players {
		candidate := &e.players[i]
		if !active(candidate) || candidate.IsHomeTeam != homeTeam {
			continue
		}
		if candidate == carrier || candidate.IsGoalkeeper ||
			candidate == midfieldArrival || candidate == farPostRunner ||
			candidate == overlappingFullback ||
			b2ContainsRunner(runners, candidate) || e.feM5HasDuty(candidate) {
			continue
		}
		role := candidate.Player.GetRole()
		eligibleSupportRole := role == PlayerRole_CDM || role == PlayerRole_CM || role == PlayerRole_CAM ||
			role == PlayerRole_LM || role == PlayerRole_RM || role == PlayerRole_LW ||
			role == PlayerRole_RW || role == PlayerRole_ST
		if e.cfg.Modules.M5Collective && role == PlayerRole_ST {
			// Ours: the striker keeps the last line (run, hold or come short
			// as the one short forward) instead of a support slot behind.
			eligibleSupportRole = false
		}
		if !eligibleSupportRole {
			continue
		}
		continuity := 0.0
		if candidate.Intent == PlayerIntent_OFFER_SUPPORT {
			continuity = Shape_SUPPORT_SELECTION_CONTINUITY_METRES
		}
		score := distance(candidate.Position, carrier.Position) - continuity
		for slot := 0; slot < len(supporters); slot++ {
			if score >= scores[slot] {
				continue
			}
			for shifted := len(supporters) - 1; shifted > slot; shifted-- {
				supporters[shifted] = supporters[shifted-1]
				scores[shifted] = scores[shifted-1]
			}
			supporters[slot] = candidate
			scores[slot] = score
			break
		}
	}
	if e.feM5Active(homeTeam) {
		// Merge with FE M5: the Link is the near option and the Security the
		// trailing one, so P12 keeps only the square supporter (ours).
		return [Shape_MAX_ACTIVE_SUPPORTERS]*MatchPlayer{Shape_SQUARE_SUPPORT_SLOT: supporters[0]}
	}
	return supporters
}

// selectShortForward is the one forward who comes short to the carrier
// (P12's complementary forward, P12:3005-3016); the others hold width and
// depth. Without an M5 episode only; with one the Link is the short option.
// Ours: Player12 brings every non-running forward short.
func (e *MatchEngine) selectShortForward(homeTeam bool, carrier *MatchPlayer, runners [Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer, supporters [Shape_MAX_ACTIVE_SUPPORTERS]*MatchPlayer, excluded ...*MatchPlayer) *MatchPlayer {
	if carrier == nil || carrier.IsHomeTeam != homeTeam || e.feM5Active(homeTeam) {
		return nil
	}
	var best *MatchPlayer
	bestScore := math.MaxFloat64
	for i := range e.players {
		candidate := &e.players[i]
		if !active(candidate) || candidate.IsHomeTeam != homeTeam || candidate == carrier ||
			candidate.IsInjured || b2ContainsRunner(runners, candidate) ||
			b2IndexOf(supporters[:], candidate) >= 0 || b2IndexOf(excluded, candidate) >= 0 {
			continue
		}
		role := candidate.Player.GetRole()
		if role != PlayerRole_ST && role != PlayerRole_LW && role != PlayerRole_RW && role != PlayerRole_CAM {
			continue
		}
		score := distance(candidate.Position, carrier.Position)
		if candidate.Intent == PlayerIntent_OFFER_SUPPORT {
			score -= Shape_SUPPORT_SELECTION_CONTINUITY_METRES
		}
		if role == PlayerRole_ST {
			// The striker comes short only when no other forward is near.
			score += Shape_SHORT_FORWARD_STRIKER_PENALTY_METRES
		}
		if score < bestScore {
			best, bestScore = candidate, score
		}
	}
	return best
}

func (e *MatchEngine) findCoverOutlet(defendingHome bool, carrier *MatchPlayer) *MatchPlayer {
	if carrier == nil || carrier.IsHomeTeam == defendingHome {
		return nil
	}
	possessionDirection := -1.0
	if carrier.IsHomeTeam {
		possessionDirection = 1.0
	}
	var outlet *MatchPlayer
	bestScore := math.MaxFloat64
	for i := range e.players {
		candidate := &e.players[i]
		if !active(candidate) || candidate.IsHomeTeam != carrier.IsHomeTeam {
			continue
		}
		if candidate == carrier || candidate.IsGoalkeeper {
			continue
		}
		outletDistance := distance(candidate.Position, carrier.Position)
		if outletDistance > Shape_COVER_OUTLET_MAX_DISTANCE_METRES {
			continue
		}
		forwardOption := (candidate.Position.X-carrier.Position.X)*possessionDirection > 0.0
		forwardBonus := 0.0
		if forwardOption {
			forwardBonus = Shape_COVER_FORWARD_OPTION_METRES
		}
		score := outletDistance - forwardBonus
		if score < bestScore {
			bestScore = score
			outlet = candidate
		}
	}
	return outlet
}
