// Ported from Player12 src/model/match_engine.cpp lines 6470–6976 (GPL-3.0)

package engine

import "math"

func (e *MatchEngine) planGoalkeeperDive(defendingHome bool, penalty bool) {
	keeper := e.findGoalkeeper(defendingHome)
	if keeper == nil {
		return
	}
	control := &e.keepers[hBoolIndex(defendingHome)]
	control.State = GoalkeeperState_DIVE
	control.Timer = 0.0
	control.LateralVelocity = 0.0
	if penalty {
		// From 11 m the keeper must commit at the kick: he picks a side, and a
		// good keeper reads the taker a little more often.
		shotSide := 1.0
		if e.ball.ShotTargetY < Pitch_CENTRE {
			shotSide = -1.0
		}
		guess := 1.0
		if e.randomFloat(0.0, 1.0) < 0.5 {
			guess = -1.0
		}
		if e.randomFloat(0.0, 1.0) <
			Goalkeeper_PENALTY_READ_BASE+keeper.Goalkeeping*Goalkeeper_PENALTY_READ_SKILL {
			guess = shotSide
		}
		if e.randomFloat(0.0, 1.0) < Goalkeeper_PENALTY_STAY_CHANCE {
			guess = 0.0
		}
		control.DiveTargetY = Pitch_CENTRE + guess*Goalkeeper_PENALTY_DIVE_METRES/Pitch_WIDTH_METRES
		control.ReactionRemaining = 0.0
		return
	}

	// Predict where the shot crosses the keeper's plane and react after a
	// skill-dependent delay; defenders between ball and goal screen the shot.
	goalX := 1.0
	if defendingHome {
		goalX = 0.0
	}
	span := goalX - e.ball.Position.X
	t := 1.0
	if math.Abs(span) > EPSILON {
		t = hClamp((keeper.Position.X-e.ball.Position.X)/span, 0.0, 1.0)
	}
	predictedY := e.ball.Position.Y + (e.ball.ShotTargetY-e.ball.Position.Y)*t
	readError := gaussian(&e.rng) * Goalkeeper_READ_ERROR_METRES *
		(1.0 - keeper.Goalkeeping*0.6) / Pitch_WIDTH_METRES
	control.DiveTargetY = hClamp(predictedY+readError,
		Pitch_GOAL_TOP-Goalkeeper_DIVE_POST_MARGIN,
		Pitch_GOAL_BOTTOM+Goalkeeper_DIVE_POST_MARGIN)

	screened := false
	for i := range e.players {
		other := &e.players[i]
		if !active(other) || other.IsGoalkeeper {
			continue
		}
		along := (other.Position.X - e.ball.Position.X) / span
		if along <= 0.05 || along >= 0.95 {
			continue
		}
		laneY := e.ball.Position.Y + (e.ball.ShotTargetY-e.ball.Position.Y)*along
		if math.Abs(other.Position.Y-laneY)*Pitch_WIDTH_METRES < Goalkeeper_SCREEN_WIDTH_METRES {
			screened = true
			break
		}
	}
	screenedSeconds := 0.0
	if screened {
		screenedSeconds = Goalkeeper_SCREENED_REACTION_SECONDS
	}
	reactionSeconds := Goalkeeper_REACTION_BASE_SECONDS +
		(1.0-keeper.Goalkeeping)*Goalkeeper_REACTION_SKILL_SECONDS + screenedSeconds
	// Flight substeps account for time between launch and the keeper's first
	// movement update. A restart launch can have no flight time in this tick.
	control.ReactionRemaining = reactionSeconds
}

func (e *MatchEngine) diveGoalkeeper(keeper *MatchPlayer, dt float64) {
	control := &e.keepers[hBoolIndex(keeper.IsHomeTeam)]
	control.Timer += dt
	if !e.ball.IsShot || e.ball.ShotSaveResolved {
		control.State = GoalkeeperState_RECOVER
		control.Timer = 0.0
		keeper.DiveTimer = math.Max(keeper.DiveTimer, Goalkeeper_RECOVER_TIME_SECONDS)
		keeper.Velocity = Vector2F{0.0, 0.0}
		return
	}
	if control.ReactionRemaining > 0.0 {
		// A reaction can finish partway through a step. Use the remaining time
		// for movement instead of delaying the keeper for another whole step.
		waiting := math.Min(control.ReactionRemaining, dt)
		control.ReactionRemaining -= waiting
		dt -= waiting
		if dt <= 0.0 {
			keeper.Velocity = Vector2F{X: 0.0, Y: 0.0}
			return
		}
	}
	// Lateral dive dynamics in m/s (the keeper moves across the width).
	// Move the visible body into reach rather than granting a distant save.
	const DIVE_CLOSING_GAIN = 1.25
	acceleration := Goalkeeper_DIVE_ACCELERATION_METRES * DIVE_CLOSING_GAIN / Pitch_WIDTH_METRES
	maximumSpeed := math.Min(e.currentTopSpeed(keeper),
		(Goalkeeper_DIVE_BASE_SPEED_METRES+keeper.Goalkeeping*Goalkeeper_DIVE_SKILL_SPEED_METRES)*
			DIVE_CLOSING_GAIN) / Pitch_WIDTH_METRES
	offset := control.DiveTargetY - keeper.Position.Y
	direction := -1.0
	if offset >= 0.0 {
		direction = 1.0
	}
	control.LateralVelocity = hClamp(control.LateralVelocity+direction*acceleration*dt,
		-maximumSpeed, maximumSpeed)
	step := control.LateralVelocity * dt
	if math.Abs(step) >= math.Abs(offset) {
		step = offset
		control.LateralVelocity = 0.0
	}
	keeper.Position.Y = hClamp(keeper.Position.Y+step, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)
	keeper.Velocity = Vector2F{0.0, step * Pitch_WIDTH_METRES / math.Max(dt, EPSILON)}
	keeper.MovementTarget = keeper.Position
	keeper.IsDiving = true
}

func (e *MatchEngine) goalkeeperTarget(keeper *MatchPlayer, carrier *MatchPlayer) Vector2F {
	control := &e.keepers[hBoolIndex(keeper.IsHomeTeam)]
	setState := func(next GoalkeeperState) {
		if control.State != next {
			control.State = next
			control.Timer = 0.0
		}
	}
	control.Timer += e.stepSeconds()
	goalX := 1.0
	outward := -1.0
	ballDepth := 1.0 - e.ball.Position.X
	if keeper.IsHomeTeam {
		goalX = 0.0
		outward = 1.0
		ballDepth = e.ball.Position.X
	}
	goalCentre := Vector2F{goalX, Pitch_CENTRE}

	if e.ball.PossessedBy == keeper.Player {
		setState(GoalkeeperState_HOLD)
		return keeper.Position
	}
	if keeper.DiveTimer > 0.0 {
		setState(GoalkeeperState_RECOVER)
		return keeper.Position
	}
	if control.State == GoalkeeperState_DISTRIBUTE &&
		control.Timer < Goalkeeper_DISTRIBUTE_TIME_SECONDS {
		return keeper.Position
	}

	// Claim a high delivery dropping into the own box.
	if e.ball.PossessedBy == nil && e.ball.IsAerialDelivery &&
		e.ball.PassByHome != keeper.IsHomeTeam {
		// Anticipate high flight, then close on the ball itself once it drops
		// within handling height. Continuing to lead a low cross keeps the
		// keeper several metres ahead of it and prevents a physical catch.
		height := e.ball.Z * Units_BALL_Z_METRES
		lookahead := 0.0
		if height > MatchRules_goalkeeperReachMetres(keeper.HeightMetres, keeper.Goalkeeping) {
			lookahead = Goalkeeper_CLAIM_LOOKAHEAD_SECONDS
		}
		drift := toPitch(Vector2F{e.ball.Velocity.X * lookahead, e.ball.Velocity.Y * lookahead})
		landing := Vector2F{e.ball.Position.X + drift.X, e.ball.Position.Y + drift.Y}
		if inPenaltyArea(landing, keeper.IsHomeTeam) &&
			distance(landing, keeper.Position) < Goalkeeper_CLAIM_BOX_DEPTH_METRES {
			setState(GoalkeeperState_CLAIM)
			return landing
		}
	}

	// Sweep a loose ball or through ball he can reach before any attacker; a
	// sweeper keeper comes further and sooner, a line keeper hardly at all.
	if keeperSweep := e.roleProfileOf(keeper).KeeperSweep; carrier == nil && !e.ball.IsShot &&
		ballDepth < Pitch_GOALKEEPER_SWEEP_DEPTH*(1.0+keeperSweep*TacticsTuning_KEEPER_SWEEP_DEPTH_GAIN) &&
		!(e.ball.IsPass && e.ball.PassByHome == keeper.IsHomeTeam) {
		keeperDistance := distance(keeper.Position, e.ball.Position)
		keeperFirst := true
		for i := range e.players {
			other := &e.players[i]
			if !active(other) || other.IsHomeTeam == keeper.IsHomeTeam {
				continue
			}
			if distance(other.Position, e.ball.Position) <
				keeperDistance*Goalkeeper_SWEEP_ADVANTAGE*
					(1.0-keeperSweep*TacticsTuning_KEEPER_SWEEP_ADVANTAGE_GAIN) {
				keeperFirst = false
				break
			}
		}
		if keeperFirst {
			setState(GoalkeeperState_SWEEP)
			lookahead := math.Min(keeperDistance/math.Max(e.currentTopSpeed(keeper), EPSILON),
				Player_MAX_LOOSE_BALL_LOOKAHEAD_SECONDS)
			lead := toPitch(Vector2F{e.ball.Velocity.X * lookahead, e.ball.Velocity.Y * lookahead})
			return Vector2F{e.ball.Position.X + lead.X, e.ball.Position.Y + lead.Y}
		}
	}

	// Rush a carrier who is through on goal inside the box.
	if carrier != nil && carrier.IsHomeTeam != keeper.IsHomeTeam &&
		inPenaltyArea(carrier.Position, keeper.IsHomeTeam) {
		covered := false
		for i := range e.players {
			other := &e.players[i]
			if !active(other) || other.IsHomeTeam != keeper.IsHomeTeam || other.IsGoalkeeper {
				continue
			}
			if (carrier.Position.X-other.Position.X)*outward > 0.0 &&
				distance(other.Position, carrier.Position) < Goalkeeper_RUSH_COVER_DISTANCE_METRES {
				covered = true
				break
			}
		}
		if !covered {
			setState(GoalkeeperState_RUSH)
			return Vector2F{
				goalCentre.X + (carrier.Position.X-goalCentre.X)*Goalkeeper_RUSH_CLOSING_SHARE,
				goalCentre.Y + (carrier.Position.Y-goalCentre.Y)*Goalkeeper_RUSH_CLOSING_SHARE,
			}
		}
	}

	// Set position on the ball-goal line, advancing to narrow the angle as the
	// threat approaches and acting as a sweeper when play is far away.
	setState(GoalkeeperState_SET_POSITION)
	threat := e.ball.Position
	if carrier != nil {
		threat = carrier.Position
	}
	threatMetres := distance(threat, goalCentre)
	depthMetres := Goalkeeper_NEAR_DEPTH_METRES + threatMetres*Goalkeeper_DEPTH_PER_METRE
	if threatMetres > Goalkeeper_SWEEPER_DISTANCE_METRES {
		depthMetres += (threatMetres - Goalkeeper_SWEEPER_DISTANCE_METRES) * Goalkeeper_SWEEPER_DEPTH_PER_METRE
	}
	// A sweeper keeper stands higher (a line keeper deeper) the further away
	// the play is.
	keeperDepth := e.roleProfileOf(keeper).KeeperDepthMetres
	depthMetres = hClamp(depthMetres+keeperDepth*math.Min(1.0, threatMetres/40.0), 0.5,
		Goalkeeper_MAX_DEPTH_METRES+math.Max(0.0, keeperDepth))
	dxMetres := (threat.X - goalCentre.X) * Pitch_LENGTH_METRES
	dyMetres := (threat.Y - goalCentre.Y) * Pitch_WIDTH_METRES
	norm := math.Max(math.Sqrt(dxMetres*dxMetres+dyMetres*dyMetres), EPSILON)
	return Vector2F{
		goalCentre.X + dxMetres/norm*depthMetres/Pitch_LENGTH_METRES,
		hClamp(goalCentre.Y+dyMetres/norm*depthMetres/Pitch_WIDTH_METRES,
			Pitch_GOALKEEPER_MIN_Y, Pitch_GOALKEEPER_MAX_Y),
	}
}

func (e *MatchEngine) updateBall(dt float64) {
	// Integrate short flight segments so a fast ball cannot jump through a
	// keeper or a line between player ticks. Keep this resolution order: shot
	// save at the keeper's plane, goal/out-of-bounds, then other loose contacts.
	// Every resolver may change state or possession, ending the remaining flight.
	// The sub-step length is the same at every fidelity.
	substeps := Timing_BALL_SUBSTEPS * int(e.stepTicks)
	substep := dt / float64(substeps)
	for index := 0; index < substeps; index++ {
		if e.ball.PossessedBy != nil || e.state != MatchState_PLAYING {
			return
		}
		e.substepBallPosition = e.ball.Position
		e.substepBallZ = e.ball.Z
		e.integrateBall(substep)

		// A shot that has died (smothered, spent or trickling) is a loose ball.
		if e.ball.IsShot && e.ball.ShotElapsedSeconds > 0.3 &&
			length(e.ball.Velocity) < Ball_DEAD_SHOT_SPEED {
			e.ball.IsShot = false
			e.ball.ShotOnTarget = false
			e.ball.TouchAttempts = 0
		}
		if e.ball.IsShot {
			e.ball.ShotElapsedSeconds += substep
			// Headers can launch mid-flight; restarts may not fly this tick.
			// Later ticks age the reaction in diveGoalkeeper instead.
			// C++: keepers.at(static_cast<std::size_t>(ball.shotByHome)).
			ageInitialGoalkeeperReaction(&e.keepers[hBoolIndex(!e.ball.ShotByHome)], substep)
			keeper := e.findGoalkeeper(!e.ball.ShotByHome)
			if !e.ball.ShotSaveResolved && keeper != nil {
				// Resolve the save when the ball reaches the keeper's plane.
				before := e.substepBallPosition.X - keeper.Position.X
				if after := e.ball.Position.X - keeper.Position.X; (before > 0.0) != (after > 0.0) ||
					math.Abs(after) <= EPSILON {
					e.resolveShotAtGoalkeeper(keeper)
				}
				if e.ball.PossessedBy != nil || e.state != MatchState_PLAYING {
					return
				}
			}
		}
		e.checkOutOfBounds()
		if e.state != MatchState_PLAYING {
			return
		}
		e.resolveLooseBall()
	}
}

func (e *MatchEngine) integrateBall(dt float64) {
	speed := length(e.ball.Velocity)
	direction := Vector2F{0.0, 0.0}
	if speed > EPSILON {
		direction = Vector2F{e.ball.Velocity.X / speed, e.ball.Velocity.Y / speed}
	}
	if math.Abs(e.ball.Curve) > EPSILON {
		// Spin bends the flight (a Magnus-effect stand-in) and slowly decays.
		rotation := e.ball.Curve * dt
		cosine := math.Cos(rotation)
		sine := math.Sin(rotation)
		direction = Vector2F{direction.X*cosine - direction.Y*sine,
			direction.X*sine + direction.Y*cosine}
		e.ball.Curve *= 1.0 - Ball_CURVE_DECAY_PER_SECOND*dt
	}
	flight := FlightState{speed, e.ball.VelocityZ, e.ball.Z * Units_BALL_Z_METRES, false}
	travelled := stepFlight(&flight, dt)
	// A delivery that has bounced is no longer a clean aerial ball.
	if flight.Bounced {
		e.ball.IsAerialDelivery = false
	}
	step := toPitch(Vector2F{direction.X * travelled, direction.Y * travelled})
	e.ball.Position.X += step.X
	e.ball.Position.Y += step.Y
	e.ball.Z = flight.Height / Units_BALL_Z_METRES
	e.ball.VelocityZ = flight.VerticalSpeed
	e.ball.Velocity = Vector2F{direction.X * flight.HorizontalSpeed, direction.Y * flight.HorizontalSpeed}
	if flight.HorizontalSpeed <= 0.0 {
		e.ball.Curve = 0.0
	}
}

func (e *MatchEngine) attemptSave(keeper *MatchPlayer) SaveAttempt {
	span := e.ball.Position.X - e.substepBallPosition.X
	t := 1.0
	if math.Abs(span) > EPSILON {
		t = hClamp((keeper.Position.X-e.substepBallPosition.X)/span, 0.0, 1.0)
	}
	crossingY := e.substepBallPosition.Y + (e.ball.Position.Y-e.substepBallPosition.Y)*t
	crossingHeight := (e.substepBallZ + (e.ball.Z-e.substepBallZ)*t) * Units_BALL_Z_METRES
	gapMetres := math.Abs(crossingY-keeper.Position.Y) * Pitch_WIDTH_METRES

	lateralReach := MatchRules_goalkeeperHandlingRadiusMetres(
		keeper.HeightMetres, keeper.Goalkeeping, crossingHeight, keeper.IsDiving)
	if lateralReach <= 0.0 || gapMetres > lateralReach {
		return SaveAttempt{}
	}

	stretch := gapMetres / math.Max(lateralReach, EPSILON)
	speedExcess := math.Max(0.0, length(e.ball.Velocity)-Goalkeeper_COMFORT_SPEED_METRES)
	penaltyFactor := 1.0
	if e.ball.ShotIsPenalty {
		penaltyFactor = Goalkeeper_PENALTY_SAVE_FACTOR
	}
	if saveChance := hClamp(
		Goalkeeper_SAVE_BASE-
			stretch*stretch*Goalkeeper_SAVE_STRETCH_PENALTY-
			speedExcess*Goalkeeper_SAVE_SPEED_PENALTY+
			(keeper.Goalkeeping-Player_DEFAULT_ATTRIBUTE)*Goalkeeper_SAVE_SKILL_BONUS,
		Goalkeeper_MIN_SAVE_CHANCE, Goalkeeper_MAX_SAVE_CHANCE) * penaltyFactor; e.randomFloat(0.0, 1.0) >= saveChance {
		return SaveAttempt{}
	}
	return SaveAttempt{true, stretch, speedExcess, crossingHeight}
}

func (e *MatchEngine) resolveShotAtGoalkeeper(goalkeeper *MatchPlayer) {
	keeper := goalkeeper
	e.ball.ShotSaveResolved = true
	if !e.ball.ShotOnTarget {
		return
	}
	attempt := e.attemptSave(keeper)
	if !attempt.Saved {
		return
	}
	stretch := attempt.Stretch
	speedExcess := attempt.SpeedExcess
	crossingHeight := attempt.CrossingHeight

	if holdChance := hClamp(
		Goalkeeper_HOLD_BASE+
			keeper.Goalkeeping*Goalkeeper_HOLD_SKILL-
			stretch*Goalkeeper_HOLD_STRETCH_PENALTY-
			speedExcess*Goalkeeper_HOLD_SPEED_PENALTY,
		Goalkeeper_MIN_HOLD_CHANCE, Goalkeeper_MAX_HOLD_CHANCE); e.randomFloat(0.0, 1.0) < holdChance {
		e.makeSave(keeper)
		return
	}
	e.parryShot(keeper, crossingHeight > Goalkeeper_TIP_OVER_METRES ||
		stretch > Goalkeeper_TIP_AROUND_STRETCH)
}

func (e *MatchEngine) parryShot(goalkeeper *MatchPlayer, overTheBar bool) {
	keeper := goalkeeper
	shooter := e.findMatchPlayer(e.lastShooter)
	if keeper.IsHomeTeam {
		e.stats.HomeSaves++
		e.stats.AwayOnTarget++
	} else {
		e.stats.AwaySaves++
		e.stats.HomeOnTarget++
	}
	e.statsOf(keeper).Saves++
	if shooter != nil {
		e.statsOf(shooter).ShotsOnTarget++
	}
	keeper.DiveTimer = Goalkeeper_RECOVER_TIME_SECONDS
	keeper.IsDiving = true
	event := e.logEventActor(MatchEventType_SAVE, keeper)
	if overTheBar {
		event.Detail = MatchEventDetail_TIPPED_BEHIND
	} else {
		event.Detail = MatchEventDetail_PARRIED
	}
	event.SecondaryPlayerId = 0
	if e.lastShooter != nil {
		event.SecondaryPlayerId = e.lastShooter.GetId()
	}
	e.ball.IsShot = false
	e.ball.ShotOnTarget = false
	e.ball.LastPossessor = keeper.Player
	if overTheBar {
		e.setupCorner(!keeper.IsHomeTeam, e.ball.Position.Y < Pitch_CENTRE)
		return
	}
	// The ball spills back into the box where a rebound can be contested.
	outward := -1.0
	if keeper.IsHomeTeam {
		outward = 1.0
	}
	speed := length(e.ball.Velocity) * Goalkeeper_PARRY_SPEED_SHARE
	vx := outward * speed * e.randomFloat(0.35, 0.9)
	vy := speed * e.randomFloat(-0.9, 0.9)
	e.ball.Velocity = Vector2F{vx, vy}
	e.ball.VelocityZ = e.randomFloat(0.0, Goalkeeper_PARRY_MAX_VERTICAL_SPEED)
	e.ball.Position.X = keeper.Position.X + outward*0.004
	e.ball.Kicker = keeper.Player
	e.ball.KickerLockout = Passing_KICKER_LOCKOUT_SECONDS
	e.ball.TouchAttempts = 0
}

func (e *MatchEngine) recordPassCompletion(receiver *MatchPlayer) {
	if !e.ball.IsPass {
		return
	}
	passer := e.findMatchPlayer(e.ball.LastPossessor)
	if receiver.IsHomeTeam == e.ball.PassByHome {
		if receiver.IsHomeTeam {
			e.stats.HomePassesCompleted++
		} else {
			e.stats.AwayPassesCompleted++
		}
		if passer != nil && passer.IsHomeTeam == receiver.IsHomeTeam && passer != receiver {
			e.statsOf(passer).PassesCompleted++
			e.trackPassCompletion(passer, receiver)
			e.lastCompletedPasser = passer.Player
			e.lastCompletedReceiver = receiver.Player
		}
		return
	}
	e.statsOf(receiver).Interceptions++
}
