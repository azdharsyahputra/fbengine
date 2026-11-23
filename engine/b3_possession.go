// Ported from Player12 src/model/match_engine.cpp lines 4951–5442 (GPL-3.0)

package engine

import "math"

func (e *MatchEngine) resolvePossessionAndActions(dt float64) {
	// Owning the ball is not an unconditional right to act: first move the
	// dribble touch, then allow challenges, then check touch reach/cooldowns.
	// A tackle may transfer ownership or stop play, so return immediately when
	// that happens instead of letting the former carrier act with a stale
	// ball.
	carrier := e.findMatchPlayer(e.ball.PossessedBy)
	if carrier == nil {
		return
	}
	e.ball.LastPossessor = carrier.Player
	if e.cfg.Modules.M6Challenge {
		// FE M6: the carried ball rolls between stride-timed touches.
		if !e.feCarry(carrier, dt) {
			return
		}
	} else if !e.updateDribble(carrier, dt) {
		return
	}

	// The controlled player tackles only on the button, when in reach.
	if e.controlledIndex.HasValue && e.controlInput.Action != MatchInputAction_NONE {
		controlled := &e.players[e.controlledIndex.Value]
		sliding := e.controlInput.Action == MatchInputAction_SLIDE_TACKLE
		if (sliding || e.controlInput.Action == MatchInputAction_TACKLE) &&
			active(controlled) && controlled.IsHomeTeam != carrier.IsHomeTeam &&
			controlled.TackleCooldown <= 0.0 && !controlled.IsInjured {
			reach := Defending_TACKLE_DISTANCE_METRES +
				controlled.Defending*Defending_TACKLE_DEFENDING_REACH_METRES
			if sliding {
				reach = Defending_SLIDE_TACKLE_DISTANCE_METRES
			}
			if distance(controlled.Position, e.ball.Position) <= reach {
				e.controlInput.Action = MatchInputAction_NONE
				e.controlledActions[hBoolIndex(controlled.IsHomeTeam)]++
				e.attemptTackle(carrier, controlled, sliding)
				if e.ball.PossessedBy != carrier.Player || e.state != MatchState_PLAYING {
					return
				}
			}
		}
	}

	// A keeper holding the ball in his hands cannot be challenged.
	inHands := carrier.IsGoalkeeper && inPenaltyArea(carrier.Position, carrier.IsHomeTeam)
	// The nearest defender to the ball (not to the man) decides whether to
	// challenge. He waits for the ball to be away from the attacker's foot;
	// near his own box and with a pressing instruction he goes in sooner.
	var defender *MatchPlayer
	if !inHands && !e.cfg.Modules.M6Challenge {
		defender = e.findClosestPlayer(e.ball.Position, !carrier.IsHomeTeam, false)
	}
	// FE M6: deterministic challenge trigger instead of the engage hazard.
	if e.cfg.Modules.M6Challenge && !inHands && e.feChallenges(carrier, dt) {
		return
	}
	if defender != nil && defender.TackleCooldown <= 0.0 && !defender.IsInjured &&
		defender.TrapTimer <= 0.0 && !e.isControlled(defender) {
		reach := Defending_TACKLE_DISTANCE_METRES +
			defender.Defending*Defending_TACKLE_DEFENDING_REACH_METRES
		ballDistance := distance(defender.Position, e.ball.Position)
		if ballDistance < Defending_SLIDE_TACKLE_DISTANCE_METRES {
			carrierDepth := carrier.Position.X
			if !carrier.IsHomeTeam {
				carrierDepth = 1.0 - carrier.Position.X
			}
			finalThird := 1.0
			if carrierDepth >= Rules_HOME_FINAL_THIRD_START {
				finalThird = Defending_FINAL_THIRD_ENGAGE_FACTOR
			}
			outOfReach := 1.0
			if ballDistance > reach {
				outOfReach = 0.5
			}
			engageRate := Defending_ENGAGE_RATE_PER_SECOND *
				(1.0 + e.getEffectiveSliders(defender.IsHomeTeam).Pressing*
					Defending_PRESSING_ENGAGE_BONUS) *
				finalThird *
				(Defending_CLOSE_CONTROL_ENGAGE_SHARE +
					Defending_EXPOSED_ENGAGE_SHARE*e.dribbleExposureShare()) *
				outOfReach *
				e.engageBoost(defender, carrier)
			if e.randomFloat(0.0, 1.0) < 1.0-math.Exp(-engageRate*dt) {
				e.attemptTackle(carrier, defender, ballDistance > reach)
				if e.ball.PossessedBy != carrier.Player || e.state != MatchState_PLAYING {
					return
				}
			}
		}
	}

	// The AI waits out its decision time after a touch; a controlled player
	// can play the ball as soon as it is under control.
	controlled := e.isControlled(carrier)
	// FE M6 reflex: closed down within 2.5 m, the carrier decides now.
	reflex := e.cfg.Modules.M6Challenge && !inHands && e.feReflexDue(carrier)
	if (carrier.ActionCooldown <= 0.0 || reflex || (controlled && carrier.TrapTimer <= 0.0)) &&
		e.ball.DribbleExposure <= Dribble_KICK_REACH_METRES {
		if controlled {
			e.performControlledAction(carrier)
		} else {
			e.decideAction(carrier)
		}
	}
}

func (e *MatchEngine) dribbleExposureShare() float64 {
	rangeMetres := e.ball.DribbleTouchLength - Dribble_FEET_METRES
	if rangeMetres <= EPSILON {
		return 0.0
	}
	return hClamp((e.ball.DribbleExposure-Dribble_FEET_METRES)/rangeMetres, 0.0, 1.0)
}

func (e *MatchEngine) updateDribble(carrier *MatchPlayer, dt float64) bool {
	inHands := carrier.IsGoalkeeper && inPenaltyArea(carrier.Position, carrier.IsHomeTeam)
	speed := length(carrier.Velocity)
	if inHands {
		e.ball.DribbleExposure = 0.0
		e.ball.DribbleTouchLength = 0.0
	} else if speed < Dribble_CLOSE_CONTROL_SPEED {
		// Close control: the ball is kept at the feet, shielded by the body.
		e.ball.DribbleExposure = Dribble_FEET_METRES
		e.ball.DribbleTouchLength = 0.0
		e.ball.TouchTimer = 0.0
		if speed > EPSILON {
			e.ball.DribbleDirection = Vector2F{carrier.Velocity.X / speed, carrier.Velocity.Y / speed}
		}
	} else {
		e.ball.TouchTimer -= dt
		if e.ball.TouchTimer <= 0.0 {
			// A touch pushes the ball ahead along the run; faster runs and
			// poorer dribblers push it further, and pressure risks a heavy
			// touch.
			e.ball.DribbleDirection = Vector2F{carrier.Velocity.X / speed, carrier.Velocity.Y / speed}
			pressure := hClamp(1.0-e.nearestOpponentDistance(carrier)/
				Dribble_HEAVY_TOUCH_PRESSURE_METRES, 0.0, 1.0)
			heavyChance := Dribble_HEAVY_TOUCH_BASE * (1.0 - carrier.Dribbling) *
				math.Min(1.0, speed/carrier.MaxSpeed) *
				(1.0 + pressure*Dribble_HEAVY_TOUCH_PRESSURE) *
				e.executionErrorScale(carrier)
			if e.randomFloat(0.0, 1.0) < heavyChance {
				push := speed + e.randomFloat(Dribble_MIN_HEAVY_PUSH_SPEED, Dribble_MAX_HEAVY_PUSH_SPEED)
				origin := e.ball.Position
				e.launchBall(carrier, origin,
					Vector2F{origin.X + e.ball.DribbleDirection.X, origin.Y + e.ball.DribbleDirection.Y},
					push, 0.0, 0.0)
				e.ball.Velocity = Vector2F{e.ball.DribbleDirection.X * push, e.ball.DribbleDirection.Y * push}
				return false
			}
			e.trackTouch(carrier)
			e.ball.DribbleTouchLength = hClamp(
				Dribble_TOUCH_BASE_METRES+speed*Dribble_TOUCH_METRES_PER_SPEED*(1.3-carrier.Dribbling),
				Dribble_MIN_TOUCH_METRES, Dribble_MAX_TOUCH_METRES)
			e.ball.DribbleExposure = e.ball.DribbleTouchLength
			e.ball.TouchTimer = hClamp(
				Dribble_TOUCH_INTERVAL_BASE-speed*Dribble_TOUCH_INTERVAL_PER_SPEED-
					carrier.Dribbling*Dribble_TOUCH_INTERVAL_DRIBBLING,
				Dribble_MIN_TOUCH_INTERVAL, Dribble_MAX_TOUCH_INTERVAL)
		} else {
			// The carrier closes on the ball until the next touch.
			e.ball.DribbleExposure = Dribble_FEET_METRES +
				(e.ball.DribbleExposure-Dribble_FEET_METRES)*
					(e.ball.TouchTimer/(e.ball.TouchTimer+dt))
		}
	}
	ahead := toPitch(Vector2F{e.ball.DribbleDirection.X * e.ball.DribbleExposure,
		e.ball.DribbleDirection.Y * e.ball.DribbleExposure})
	e.ball.Position = Vector2F{hClamp(carrier.Position.X+ahead.X, 0.001, 0.999),
		hClamp(carrier.Position.Y+ahead.Y, 0.001, 0.999)}
	e.ball.Z = 0.0
	e.ball.Velocity = carrier.Velocity
	return true
}

func (e *MatchEngine) attemptTakeOn(carrier *MatchPlayer, defender *MatchPlayer) bool {
	// A feint, a change of direction or a burst of pace against a jockeying
	// defender: skill and pace against his defending decide it.
	jockey := 0.0
	// Play mode: a human defender jockeying is harder to beat.
	if e.controlInput.Jockey && e.isControlled(defender) {
		jockey = Control_JOCKEY_TAKE_ON_PENALTY
	}
	success := hClamp(
		Dribble_TAKE_ON_BASE+
			(carrier.Dribbling-defender.Defending+
				(e.teamEdge(carrier.IsHomeTeam)-e.teamEdge(defender.IsHomeTeam))*
					Rules_DUEL_EDGE_WEIGHT)*Dribble_TAKE_ON_SKILL+
			(carrier.Pace-defender.Pace)*Dribble_TAKE_ON_PACE-
			jockey-
			e.doubleUpPenalty(carrier, defender),
		Dribble_MIN_TAKE_ON, Dribble_MAX_TAKE_ON)
	if e.randomFloat(0.0, 1.0) >= success {
		// Read and stopped: the defender takes the ball (a clean challenge).
		e.attemptTackle(carrier, defender, false)
		return e.ball.PossessedBy == carrier.Player
	}
	// Beaten: the defender is wrong-footed and the ball is pushed past him.
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
	e.ball.DribbleDirection = normalized(Vector2F{
		forward.X*math.Cos(angle) - forward.Y*math.Sin(angle),
		forward.X*math.Sin(angle) + forward.Y*math.Cos(angle)})
	e.ball.DribbleTouchLength = Dribble_TAKE_ON_TOUCH_METRES
	e.ball.DribbleExposure = Dribble_TAKE_ON_TOUCH_METRES
	e.ball.TouchTimer = Dribble_MAX_TOUCH_INTERVAL
	return true
}

func (e *MatchEngine) attemptTackle(carrier *MatchPlayer, defender *MatchPlayer, sliding bool) {
	// Timing matters: a challenge while the ball is away from the attacker's
	// foot wins it cleanly; one into close control or from behind is a foul
	// risk.
	exposure := e.dribbleExposureShare()
	motion := normalized(carrier.Velocity)
	toDefender := metricDirection(carrier.Position, defender.Position)
	fromBehind := motion.X*toDefender.X+motion.Y*toDefender.Y < -0.3
	shielding := length(carrier.Velocity) < Dribble_CLOSE_CONTROL_SPEED
	defenderStrategy := e.getEffectiveSliders(defender.IsHomeTeam)
	// A booked player picks his challenges more carefully.
	caution := 1.0
	if defender.YellowCards > 0 {
		caution = Discipline_BOOKED_PLAYER_CAUTION
	}
	defender.TackleCooldown =
		e.randomFloat(Defending_MIN_TACKLE_COOLDOWN, Defending_MAX_TACKLE_COOLDOWN) *
			(Defending_TACKLE_COOLDOWN_BASE_MULTIPLIER -
				defenderStrategy.Pressing*Defending_PRESSING_COOLDOWN_REDUCTION) /
			caution

	if defender.IsHomeTeam {
		e.stats.HomeTackleAttempts++
	} else {
		e.stats.AwayTackleAttempts++
	}
	e.statsOf(defender).TacklesAttempted++

	context := NewTackleContext()
	context.Defending = defender.Defending +
		e.teamEdge(defender.IsHomeTeam)*Rules_DUEL_EDGE_WEIGHT
	context.Dribbling = carrier.Dribbling +
		e.teamEdge(carrier.IsHomeTeam)*Rules_DUEL_EDGE_WEIGHT
	context.DefenderPhysicality = defender.Physicality
	context.CarrierPhysicality = carrier.Physicality
	context.Pressing = defenderStrategy.Pressing
	context.RiskTaking = defenderStrategy.RiskTaking
	context.Exposure = exposure
	context.Shielding = shielding
	context.Sliding = sliding
	context.FromBehind = fromBehind
	context.InPenaltyArea = inPenaltyArea(carrier.Position, defender.IsHomeTeam)
	context.DefenderBooked = defender.YellowCards > 0
	winChance := MatchRules_tackleWinChance(context)
	foulPropensity := MatchRules_tackleFoulPropensity(context)
	if e.randomFloat(0.0, 1.0) < winChance {
		// Even a challenge that reaches the ball can be late or through the
		// man.
		if e.incidentRoll() < foulPropensity*Defending_WINNING_TACKLE_FOUL_SHARE {
			e.commitFoul(defender, carrier, true, sliding && fromBehind)
			return
		}
		if defender.IsHomeTeam {
			e.stats.HomeTackles++
		} else {
			e.stats.AwayTackles++
		}
		e.statsOf(defender).TacklesWon++
		if e.randomFloat(0.0, 1.0) < Defending_POKE_LOOSE_CHANCE {
			// The challenge knocks the ball away rather than winning it
			// cleanly.
			angle := e.randomFloat(-math.Pi, math.Pi)
			speed := e.randomFloat(Defending_MIN_POKE_SPEED, Defending_MAX_POKE_SPEED)
			e.ball.PossessedBy = nil
			e.ball.LastPossessor = defender.Player
			e.clearFlightState()
			e.ball.Z = 0.0
			e.ball.Velocity = Vector2F{math.Cos(angle) * speed, math.Sin(angle) * speed}
			e.ball.VelocityZ = 0.0
			e.ball.Kicker = defender.Player
			e.ball.KickerLockout = Passing_KICKER_LOCKOUT_SECONDS
			carrier.TrapTimer = Ball_FAILED_TRAP_TIME
			e.lastControlledTeamHome = hSome(defender.IsHomeTeam)
			e.transitionSecondsRemaining = Timing_POSSESSION_TRANSITION_SECONDS
			e.updateTeamPhases()
			return
		}
		e.setPossession(defender)
		defender.ActionCooldown = e.randomFloat(Defending_MIN_RECOVERY_COOLDOWN,
			Defending_MAX_RECOVERY_COOLDOWN)
		return
	}

	if e.incidentRoll() >= foulPropensity {
		return
	}
	e.commitFoul(defender, carrier, false, sliding && fromBehind)
}

func (e *MatchEngine) deniesGoalChance(victim *MatchPlayer, offender *MatchPlayer) bool {
	// Only the player in control of the ball can be denied an obvious chance.
	if e.ball.PossessedBy != victim.Player {
		return false
	}
	goalX := 0.0
	if victim.IsHomeTeam {
		goalX = 1.0
	}
	goal := Vector2F{goalX, Pitch_CENTRE}
	if distance(victim.Position, goal) > Discipline_DOGSO_MAX_DISTANCE_METRES ||
		math.Abs(victim.Position.Y-Pitch_CENTRE) > Discipline_DOGSO_MAX_WIDTH_DEVIATION {
		return false
	}
	// Obvious chance: no covering outfield defender between victim and goal.
	direction := -1.0
	if victim.IsHomeTeam {
		direction = 1.0
	}
	for i := range e.players {
		other := &e.players[i]
		if active(other) && other.IsHomeTeam != victim.IsHomeTeam &&
			!other.IsGoalkeeper && other != offender &&
			(other.Position.X-victim.Position.X)*direction > 0.0 {
			return false
		}
	}
	return true
}

func (e *MatchEngine) commitFoul(offender *MatchPlayer, victim *MatchPlayer, ballWon bool, reckless bool) {
	if offender.IsHomeTeam {
		e.stats.HomeFouls++
	} else {
		e.stats.AwayFouls++
	}
	e.statsOf(offender).FoulsCommitted++
	e.statsOf(victim).FoulsSuffered++

	foulPosition := victim.Position
	inBox := inPenaltyArea(foulPosition, offender.IsHomeTeam)
	carrierDepth := victim.Position.X
	if !victim.IsHomeTeam {
		carrierDepth = 1.0 - victim.Position.X
	}
	victimPhase := e.awayPhase
	if victim.IsHomeTeam {
		victimPhase = e.homePhase
	}
	promisingAttack := carrierDepth > Pitch_CENTRE &&
		(victimPhase == TeamPhase_ATTACKING_TRANSITION ||
			victimPhase == TeamPhase_FINAL_THIRD ||
			e.openSpaceAhead(victim) > Discipline_ADVANTAGE_MIN_OPENNESS)

	context := NewFoulContext()
	if reckless {
		context.SeverityRoll = math.Max(e.incidentRoll(), 0.9)
	} else {
		context.SeverityRoll = e.incidentRoll()
	}
	context.CardRoll = e.incidentRoll()
	context.Strictness = e.refereeStrictness
	context.Tactical = promisingAttack &&
		(victimPhase == TeamPhase_ATTACKING_TRANSITION || !ballWon)
	context.DenyingGoalChance = e.deniesGoalChance(victim, offender)
	context.InPenaltyArea = inBox
	context.OffenderAlreadyBooked = offender.YellowCards > 0
	context.OffenderIsAway = !offender.IsHomeTeam
	context.VenueBiasScale = e.matchContext.HomeAdvantageScale
	sanction := MatchRules_decideFoulSanction(context)
	e.auditFoul(offender, ballWon, reckless, sanction, context.DenyingGoalChance, context.Tactical)

	foulEvent := e.logEventActor(MatchEventType_FOUL, offender)
	foulEvent.SecondaryPlayerId = 0
	if victim.Player != nil {
		foulEvent.SecondaryPlayerId = victim.Player.GetId()
	}
	foulEvent.Position = foulPosition

	// Advantage: the fouled side kept the ball in a promising attack outside
	// the box, so play continues unless the attack breaks down quickly.
	victimKeepsBall := e.ball.PossessedBy == victim.Player
	playAdvantage := !ballWon && victimKeepsBall && !inBox && promisingAttack &&
		sanction != FoulSanction_RED &&
		e.incidentRoll() < Discipline_ADVANTAGE_PLAY_CHANCE
	injuryChance := Injury_CONTACT_INJURY_CHANCE
	if context.SeverityRoll >= 1.0-Discipline_RECKLESS_SHARE {
		injuryChance = Injury_RECKLESS_CONTACT_INJURY_CHANCE
	}
	contactInjury := e.incidentRoll() < injuryChance

	e.applySanction(offender, sanction)
	if e.state == MatchState_FULL_TIME {
		return
	}

	if playAdvantage && !contactInjury {
		if victim.IsHomeTeam {
			e.stats.HomeAdvantagesPlayed++
		} else {
			e.stats.AwayAdvantagesPlayed++
		}
		advantage := e.logEventActor(MatchEventType_ADVANTAGE, victim)
		advantage.Position = foulPosition
		e.pendingAdvantage = PendingAdvantage{
			Active:           true,
			FouledTeamHome:   victim.IsHomeTeam,
			Position:         foulPosition,
			SecondsRemaining: Discipline_ADVANTAGE_WINDOW_SECONDS,
		}
		return
	}

	if contactInjury && victim.OnPitch {
		e.injurePlayer(victim, true)
	}
	if inBox {
		e.setupPenalty(victim.IsHomeTeam)
	} else {
		e.setupFreeKick(victim.IsHomeTeam, foulPosition)
	}
	if sanction != FoulSanction_NONE {
		e.extendRestart(Timing_CARD_DELAY_SECONDS)
	}
}

func (e *MatchEngine) applySanction(offender *MatchPlayer, sanction FoulSanction) {
	if sanction == FoulSanction_NONE || offender.Player == nil {
		return
	}
	entry := e.statsOf(offender)
	e.stoppageLogs[e.period-1].Cards++
	if sanction == FoulSanction_YELLOW {
		offender.YellowCards++
		entry.YellowCards++
		if offender.IsHomeTeam {
			e.stats.HomeYellowCards++
		} else {
			e.stats.AwayYellowCards++
		}
		e.logEventActor(MatchEventType_YELLOW_CARD, offender)
		return
	}

	if sanction == FoulSanction_SECOND_YELLOW {
		offender.YellowCards++
		entry.YellowCards++
		if offender.IsHomeTeam {
			e.stats.HomeYellowCards++
		} else {
			e.stats.AwayYellowCards++
		}
		e.logEventActor(MatchEventType_SECOND_YELLOW, offender)
	} else {
		e.logEventActor(MatchEventType_RED_CARD, offender)
	}
	entry.RedCards++
	entry.SentOff = true
	// A sending-off always stops play; open the stoppage before any forced
	// goalkeeper change so it shares the restart's substitution window.
	e.beginStoppage()
	if offender.IsHomeTeam {
		e.stats.HomeRedCards++
	} else {
		e.stats.AwayRedCards++
	}
	e.removeFromPitch(offender)
}

func (e *MatchEngine) updatePendingAdvantage(dt float64) {
	if !e.pendingAdvantage.Active {
		return
	}
	if carrier := e.findMatchPlayer(e.ball.PossessedBy); carrier != nil &&
		carrier.IsHomeTeam != e.pendingAdvantage.FouledTeamHome {
		// The advantage did not materialise: back for the original free kick.
		e.pendingAdvantage.Active = false
		e.setupFreeKick(e.pendingAdvantage.FouledTeamHome, e.pendingAdvantage.Position)
		return
	}
	e.pendingAdvantage.SecondsRemaining -= dt
	if e.pendingAdvantage.SecondsRemaining <= 0.0 {
		e.pendingAdvantage.Active = false
	}
}
