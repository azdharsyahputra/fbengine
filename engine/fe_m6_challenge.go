// Ported from FootballEngine FootballEngine.Core/Match/Systems/Players/Actions.fs (prepareChallenges),
// Systems/ContactAction.fs (prepare: predicted ball point, commit time) and
// Systems/ContactDynamics.fs (execution error at commit, contact fouls), GPL-3.0;
// constants from Contracts/BalanceConfig.fs (defaultContactAction, Movement).
// The light-contact resolution follows FE_PORT_PLAN.md §M6(b). Ours: the
// standoff breaker, the carrier's evasive touch on a committed challenge, the
// share of duels won once the boot meets the ball, and the dispossessed
// carrier's reset. Player12 (GPL-3.0) supplies the take-on odds (attemptTakeOn),
// the tackle/foul context, the poke-loose share, cooldowns and the foul pipeline.
//
// A defender sent to engage (or one the ball comes nearer to than to its
// carrier) commits a standing tackle as soon as the ball is within boot reach
// plus what he closes during the 0.1 s preparation; a recovering one slides at
// a ball running away from him. He holds when the carrier's body is between
// him and the ball. The boot lands on the ball point predicted at the decision;
// it plays the ball if the ball is still there, and otherwise it may catch the
// carrier's legs.

package engine

import "math"

const (
	feFootReach              = 0.7         // ContactAction.FootReach (m)
	feSlidingReach           = 1.5         // ContactAction.SlidingReach (m)
	feStandingPreparation    = 4.0 / 40.0  // ContactAction.StandingTacklePreparation (s)
	feSlidingPreparation     = 6.0 / 40.0  // ContactAction.SlidingTacklePreparation (s)
	feCommittedDuration      = 4.0 / 40.0  // ContactAction.CommittedDuration (s)
	feRecoveryDuration       = 12.0 / 40.0 // ContactAction.RecoveryDuration (s)
	feTackleBodyClearance    = 0.28        // ContactAction.TackleBodyClearance (m)
	feExecutionErrorScale    = 0.14        // ContactAction.ExecutionErrorScale (m)
	feMaximumExecutionErr    = 0.65        // ContactAction.MaximumExecutionError (m)
	feBootBallContact        = 0.2656      // boot radius 0.1556 + ball radius 0.11 (m)
	feBootLegContact         = 0.2756      // boot on the carrier's leg (m)
	feSprintSpeed            = 8.5         // Movement.SprintSpeed (m/s)
	feRecklessSpeed          = 5.0         // ContactAction.RecklessRelativeSpeed (m/s)
	feReflexRadius           = 2.5         // Actions.ReflexDecisionRadius (m)
	feStandoffSpeed          = 1.5         // ours: a carrier slower than this is held up
	feStandoffEngage         = 1.0         // ours: after this long the nearest opponent goes in
	feStandoffThrough        = 2.0         // ours: ... and no longer respects a shield
	feEvadeMetres            = 1.5         // ours: evasive touch length
	feBehindAwareness        = 0.35        // ours: odds of reading a challenge from behind
	feChallengeStaleness     = 0.3         // ours: drop a challenge whose moment has passed
	feWinFloor               = 0.2         // ours: bounds of the duel won once the boot meets the ball
	feWinCeiling             = 0.95
	feWinBase                = 0.3 // ours: an even duel at the ball goes to the tackler ~55 %
	feWinGain                = 0.9
	feDangerousBallFirst     = 5.1  // ContactAction.DangerousBallFirstSpeed (m/s)
	feSlowBallFirstFoulShare = 0.75 // ours
)

// feChallenges runs each tick while the ball is carried: it lands the boots
// due now, then lets every opponent decide whether to go in. It returns true
// when the carrier no longer has the ball or play stopped.
func (e *MatchEngine) feChallenges(carrier *MatchPlayer, dt float64) bool {
	if e.state != MatchState_PLAYING {
		return false
	}
	s := e.feM6()
	now := e.getSimulatedSeconds()
	for i := range e.players {
		if i >= len(s.challenges) {
			break
		}
		c := &s.challenges[i]
		if !c.active || now+1e-6 < c.commitAt {
			continue
		}
		c.active = false
		defender := &e.players[i]
		if c.target != carrier.Player || !active(defender) || now-c.commitAt > feChallengeStaleness {
			continue
		}
		e.feResolveChallenge(carrier, defender, c)
		if e.ball.PossessedBy != carrier.Player || e.state != MatchState_PLAYING {
			return true
		}
	}

	e.feUpdateStandoff(carrier, dt)

	for i := range e.players {
		defender := &e.players[i]
		if i >= len(s.challenges) || s.challenges[i].active || !active(defender) ||
			defender.IsHomeTeam == carrier.IsHomeTeam || defender.IsGoalkeeper ||
			defender.IsInjured || defender.TackleCooldown > 0.0 || defender.TrapTimer > 0.0 ||
			e.isControlled(defender) {
			continue
		}
		if sliding, ok := e.feChallengeKind(carrier, defender); ok {
			e.fePrepareChallenge(carrier, defender, i, sliding, true)
		}
	}
	return false
}

// feClosing is how fast an engaging defender closes during the preparation
// (FE: SprintSpeed·(0.5 + 0.006·Pace)).
func feClosing(defender *MatchPlayer, engaging bool) float64 {
	if !engaging {
		return 0.0
	}
	return feSprintSpeed * (0.5 + 0.006*defender.Pace*100.0)
}

// feOwnGoalX is the x (pitch units) of the goal the player defends.
func feOwnGoalX(player *MatchPlayer) float64 {
	if player.IsHomeTeam {
		return 0.0
	}
	return 1.0
}

// feGoalSide: the defender is at least as near his own goal as the carrier.
func feGoalSide(defender, carrier *MatchPlayer) bool {
	goal := feOwnGoalX(defender)
	return math.Abs(carrier.Position.X-goal) >= math.Abs(defender.Position.X-goal)
}

// feShielded is FE's shield test: where the carrier and ball will be after
// `seconds`, the carrier's body lies on the defender's line to the ball.
func (e *MatchEngine) feShielded(carrier, defender *MatchPlayer, seconds float64) bool {
	if !feGoalSide(defender, carrier) {
		return false
	}
	b := toMetres(Vector2F{e.ball.Position.X - defender.Position.X, e.ball.Position.Y - defender.Position.Y})
	b.X += e.ball.Velocity.X * seconds
	b.Y += e.ball.Velocity.Y * seconds
	c := toMetres(Vector2F{carrier.Position.X - defender.Position.X, carrier.Position.Y - defender.Position.Y})
	c.X += carrier.Velocity.X * seconds
	c.Y += carrier.Velocity.Y * seconds
	lengthSquared := b.X*b.X + b.Y*b.Y
	along := 0.0
	if lengthSquared > 1e-9 {
		along = (c.X*b.X + c.Y*b.Y) / lengthSquared
	}
	ox, oy := b.X*along-c.X, b.Y*along-c.Y
	return along > 0.0 && along < 1.0 && math.Sqrt(ox*ox+oy*oy) < feTackleBodyClearance
}

// feChallengeKind is FE prepareChallenges' decision for one defender.
func (e *MatchEngine) feChallengeKind(carrier, defender *MatchPlayer) (sliding bool, ok bool) {
	s := e.feM6()
	forced := s.standoffCarrier == carrier.Player && s.standoffSeconds >= feStandoffEngage &&
		defender == e.feStandoffOpponent(carrier)
	engaging := forced || defender.Intent == PlayerIntent_PRESS_BALL ||
		defender.Intent == PlayerIntent_COVER_PRESS
	toBall := toMetres(Vector2F{e.ball.Position.X - defender.Position.X,
		e.ball.Position.Y - defender.Position.Y})
	d := length(toBall)
	closing := feClosing(defender, engaging)
	reach := feFootReach + closing*feStandingPreparation
	slideReach := feSlidingReach + closing*feSlidingPreparation
	if d > math.Max(reach, slideReach) {
		return false, false
	}
	// Only a player sent to engage goes in while the ball is at the carrier's
	// boot; anyone else waits for a touch that leaves it nearer him.
	exposed := engaging || d < distance(carrier.Position, e.ball.Position)
	if !exposed {
		return false, false
	}
	through := forced && s.standoffSeconds >= feStandoffThrough
	if d <= reach && (through || !e.feShielded(carrier, defender, feStandingPreparation)) {
		return false, true
	}
	// A ball moving away can only be reached by going to ground.
	escaping := toBall.X*e.ball.Velocity.X+toBall.Y*e.ball.Velocity.Y > 0.0
	recovering := engaging && !feGoalSide(defender, carrier)
	if ((recovering && escaping) || forced) && d <= slideReach &&
		(through || !e.feShielded(carrier, defender, feSlidingPreparation)) {
		return true, true
	}
	return false, false
}

// fePrepareChallenge is FE ContactActionPlanner.prepare: the boot is sent to
// the ball point predicted at the commit time. The carrier may react to it.
func (e *MatchEngine) fePrepareChallenge(carrier, defender *MatchPlayer, slot int, sliding bool, allowEvade bool) {
	s := e.feM6()
	if slot < 0 || slot >= len(s.challenges) {
		return
	}
	preparation := feStandingPreparation
	if sliding {
		preparation = feSlidingPreparation
	}
	ahead := toPitch(Vector2F{e.ball.Velocity.X * preparation, e.ball.Velocity.Y * preparation})
	s.challenges[slot] = feChallenge{
		active:   true,
		sliding:  sliding,
		target:   carrier.Player,
		commitAt: e.getSimulatedSeconds() + preparation,
		predict:  Vector2F{e.ball.Position.X + ahead.X, e.ball.Position.Y + ahead.Y},
	}
	if allowEvade {
		e.feEvade(carrier, defender)
	}
}

// feTakeOnChance is Player12 attemptTakeOn's success odds: skill and pace
// against the defender's defending.
func (e *MatchEngine) feTakeOnChance(carrier, defender *MatchPlayer) float64 {
	jockey := 0.0
	if e.controlInput.Jockey && e.isControlled(defender) {
		jockey = Control_JOCKEY_TAKE_ON_PENALTY
	}
	return hClamp(
		Dribble_TAKE_ON_BASE+
			(carrier.Dribbling-defender.Defending+
				(e.teamEdge(carrier.IsHomeTeam)-e.teamEdge(defender.IsHomeTeam))*
					Rules_DUEL_EDGE_WEIGHT)*Dribble_TAKE_ON_SKILL+
			(carrier.Pace-defender.Pace)*Dribble_TAKE_ON_PACE-
			jockey-
			e.doubleUpPenalty(carrier, defender),
		Dribble_MIN_TAKE_ON, Dribble_MAX_TAKE_ON)
}

// feEvade: the carrier sees the defender commit and plays the ball away from
// the boot (a drag-back, a feint and push). Ours, on Player12's take-on odds.
func (e *MatchEngine) feEvade(carrier, defender *MatchPlayer) {
	s := e.feM6()
	if e.isControlled(carrier) || carrier.TrapTimer > 0.0 || s.knockLength > 0.0 ||
		e.ball.PossessedBy != carrier.Player {
		return
	}
	chance := e.feTakeOnChance(carrier, defender)
	facing := feFacing(carrier)
	toDefender := metricDirection(carrier.Position, defender.Position)
	if facing.X*toDefender.X+facing.Y*toDefender.Y < -0.3 {
		chance *= feBehindAwareness
	}
	if e.randomFloat(0.0, 1.0) >= chance {
		return
	}
	e.feBeatDefender(carrier, defender, feEvadeMetres)
}

// feBeatDefender is Player12 attemptTakeOn's success branch with the push made
// a real knock-on of the ball.
func (e *MatchEngine) feBeatDefender(carrier, defender *MatchPlayer, metres float64) {
	defender.TackleCooldown = math.Max(defender.TackleCooldown, Dribble_BEATEN_SECONDS)
	defender.Velocity.X *= Dribble_BEATEN_VELOCITY_RETAINED
	defender.Velocity.Y *= Dribble_BEATEN_VELOCITY_RETAINED
	forward := Vector2F{-1.0, 0.0}
	if carrier.IsHomeTeam {
		forward.X = 1.0
	}
	toDefender := metricDirection(carrier.Position, defender.Position)
	side := 1.0
	if toDefender.Y >= 0.0 {
		side = -1.0
	}
	angle := side * Dribble_TAKE_ON_ANGLE_RADIANS
	direction := normalized(Vector2F{
		forward.X*math.Cos(angle) - forward.Y*math.Sin(angle),
		forward.X*math.Sin(angle) + forward.Y*math.Cos(angle)})
	e.feKnockOn(carrier, direction, metres)
}

// feTakeOn replaces attemptTakeOn in decideAction: a beaten defender is left
// behind by a knock-on; a read one goes in on the ball and the boot decides.
func (e *MatchEngine) feTakeOn(carrier, defender *MatchPlayer) {
	if e.randomFloat(0.0, 1.0) < e.feTakeOnChance(carrier, defender) {
		e.feBeatDefender(carrier, defender, Dribble_TAKE_ON_TOUCH_METRES)
		return
	}
	s := e.feM6()
	slot := e.slotOf(defender)
	if slot >= len(s.challenges) || s.challenges[slot].active || e.isControlled(defender) ||
		defender.TrapTimer > 0.0 {
		return
	}
	d := distance(defender.Position, e.ball.Position)
	closing := feClosing(defender, true)
	if d <= feFootReach+closing*feStandingPreparation {
		e.fePrepareChallenge(carrier, defender, slot, false, false)
	} else if d <= feSlidingReach+closing*feSlidingPreparation {
		e.fePrepareChallenge(carrier, defender, slot, true, false)
	}
}

// feResolveChallenge lands the boot (FE commit with execution error) and
// applies the light-contact outcome of FE_PORT_PLAN §M6(b).
func (e *MatchEngine) feResolveChallenge(carrier, defender *MatchPlayer, c *feChallenge) {
	composure := (defender.Vision + defender.Passing) * 50.0
	technique := hClamp((defender.Defending*100.0+composure)/200.0, 0.0, 1.0)
	scale := feExecutionErrorScale * (1.0 - technique*hClamp(defender.Stamina, 0.0, 1.0))
	ex := e.randomFloat(-scale, scale)
	ey := e.randomFloat(-scale, scale)
	ez := e.randomFloat(-scale, scale)
	if magnitude := math.Sqrt(ex*ex + ey*ey + ez*ez); magnitude > feMaximumExecutionErr {
		ex *= feMaximumExecutionErr / magnitude
		ey *= feMaximumExecutionErr / magnitude
	}
	offset := toPitch(Vector2F{ex, ey})
	boot := Vector2F{c.predict.X + offset.X, c.predict.Y + offset.Y}
	ballGap := distance(boot, e.ball.Position)
	legGap := distance(boot, carrier.Position)

	// Committed then recovering; a slider is left on the ground.
	defender.TackleCooldown = math.Max(defender.TackleCooldown, feCommittedDuration+feRecoveryDuration)
	if c.sliding {
		defender.Velocity = Vector2F{}
	}

	// Player12 attemptTackle's context: who is challenging whom, and how.
	motion := normalized(carrier.Velocity)
	toDefender := metricDirection(carrier.Position, defender.Position)
	fromBehind := motion.X*toDefender.X+motion.Y*toDefender.Y < -0.3
	defenderStrategy := e.getEffectiveSliders(defender.IsHomeTeam)
	if defender.IsHomeTeam {
		e.stats.HomeTackleAttempts++
	} else {
		e.stats.AwayTackleAttempts++
	}
	e.statsOf(defender).TacklesAttempted++
	context := NewTackleContext()
	context.Defending = defender.Defending + e.teamEdge(defender.IsHomeTeam)*Rules_DUEL_EDGE_WEIGHT
	context.Dribbling = carrier.Dribbling + e.teamEdge(carrier.IsHomeTeam)*Rules_DUEL_EDGE_WEIGHT
	context.DefenderPhysicality = defender.Physicality
	context.CarrierPhysicality = carrier.Physicality
	context.Pressing = defenderStrategy.Pressing
	context.RiskTaking = defenderStrategy.RiskTaking
	context.Exposure = e.dribbleExposureShare()
	context.Shielding = length(carrier.Velocity) < Dribble_CLOSE_CONTROL_SPEED
	context.Sliding = c.sliding
	context.FromBehind = fromBehind
	context.InPenaltyArea = inPenaltyArea(carrier.Position, defender.IsHomeTeam)
	context.DefenderBooked = defender.YellowCards > 0
	foulPropensity := MatchRules_tackleFoulPropensity(context)

	if ballGap <= feBootBallContact {
		// The boot met the ball. It can still go through the man (Player12's
		// share of the foul propensity for a challenge that wins the ball),
		// a slow standing one less often (FE: ball-first is fair below
		// DangerousBallFirstSpeed).
		share := Defending_WINNING_TACKLE_FOUL_SHARE
		if !c.sliding && length(defender.Velocity) < feDangerousBallFirst {
			share *= feSlowBallFirstFoulShare
		}
		if e.incidentRoll() < foulPropensity*share {
			e.commitFoul(defender, carrier, true, c.sliding && fromBehind)
			return
		}
		// Played the ball, but the carrier may still ride it: Player12's duel
		// odds (skill, strength, exposure) decide who comes away with it.
		won := hClamp(feWinBase+feWinGain*MatchRules_tackleWinChance(context)/Defending_MAX_WIN_CHANCE,
			feWinFloor, feWinCeiling)
		if e.randomFloat(0.0, 1.0) >= won {
			e.feTackleCooldown(defender)
			return
		}
		if defender.IsHomeTeam {
			e.stats.HomeTackles++
		} else {
			e.stats.AwayTackles++
		}
		e.statsOf(defender).TacklesWon++
		// Ours: the man who lost it needs a moment before he can go back in.
		carrier.TackleCooldown = math.Max(carrier.TackleCooldown, Dribble_BEATEN_SECONDS)
		if e.randomFloat(0.0, 1.0) >= Defending_POKE_LOOSE_CHANCE {
			e.setPossession(defender)
			defender.ActionCooldown = e.randomFloat(Defending_MIN_RECOVERY_COOLDOWN,
				Defending_MAX_RECOVERY_COOLDOWN)
			return
		}
		// Poked away along ball - foot (FE deflection impulse); the foot comes
		// in from the tackler's side of the ball.
		direction := metricDirection(defender.Position, e.ball.Position)
		if length(direction) <= EPSILON {
			angle := e.randomFloat(-math.Pi, math.Pi)
			direction = Vector2F{math.Cos(angle), math.Sin(angle)}
		}
		speed := e.randomFloat(Defending_MIN_POKE_SPEED, Defending_MAX_POKE_SPEED)
		e.feM6().carrier = nil
		e.ball.PossessedBy = nil
		e.ball.LastPossessor = defender.Player
		e.clearFlightState()
		e.ball.Z = 0.0
		e.ball.Velocity = Vector2F{direction.X * speed, direction.Y * speed}
		e.ball.VelocityZ = 0.0
		e.ball.Kicker = defender.Player
		e.ball.KickerLockout = Passing_KICKER_LOCKOUT_SECONDS
		carrier.TrapTimer = Ball_FAILED_TRAP_TIME
		e.lastControlledTeamHome = hSome(defender.IsHomeTeam)
		e.transitionSecondsRemaining = Timing_POSSESSION_TRANSITION_SECONDS
		e.updateTeamPhases()
		return
	}

	// Missed the ball; a boot on the carrier's legs is a trip.
	if legGap <= feBootLegContact && e.incidentRoll() < foulPropensity {
		reckless := length(defender.Velocity) >= feRecklessSpeed || (c.sliding && fromBehind)
		e.commitFoul(defender, carrier, false, reckless)
	}
}

// feStandoffOpponent is the nearest outfield opponent to the carrier.
func (e *MatchEngine) feStandoffOpponent(carrier *MatchPlayer) *MatchPlayer {
	return e.findClosestPlayer(carrier.Position, !carrier.IsHomeTeam, false)
}

// feUpdateStandoff counts how long the carrier has been held up: slow, with
// an opponent within reflex range. Ours.
func (e *MatchEngine) feUpdateStandoff(carrier *MatchPlayer, dt float64) {
	s := e.feM6()
	if s.standoffCarrier != carrier.Player {
		s.standoffCarrier = carrier.Player
		s.standoffSeconds = 0.0
	}
	opponent := e.feStandoffOpponent(carrier)
	if opponent != nil && distance(opponent.Position, carrier.Position) <= feReflexRadius &&
		length(carrier.Velocity) < feStandoffSpeed {
		s.standoffSeconds += dt
	} else {
		s.standoffSeconds = 0.0
	}
}

// feStandoffActive reports a standoff long enough that shielding is no longer
// an option for the carrier.
func (e *MatchEngine) feStandoffActive(carrier *MatchPlayer) bool {
	s := e.feM6()
	return s.standoffCarrier == carrier.Player && s.standoffSeconds >= feStandoffEngage
}

// feTackleCooldown is Player12 attemptTackle's cooldown after a challenge the
// carrier rode: the defender has to reset before going in again.
func (e *MatchEngine) feTackleCooldown(defender *MatchPlayer) {
	caution := 1.0
	if defender.YellowCards > 0 {
		caution = Discipline_BOOKED_PLAYER_CAUTION
	}
	defender.TackleCooldown = math.Max(defender.TackleCooldown,
		e.randomFloat(Defending_MIN_TACKLE_COOLDOWN, Defending_MAX_TACKLE_COOLDOWN)*
			(Defending_TACKLE_COOLDOWN_BASE_MULTIPLIER-
				e.getEffectiveSliders(defender.IsHomeTeam).Pressing*Defending_PRESSING_COOLDOWN_REDUCTION)/
			caution)
}
