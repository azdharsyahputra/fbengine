// Ported from Player12 src/model/match_engine.cpp lines 7632–7944 (GPL-3.0)

package engine

import "math"

func (e *MatchEngine) setPossession(player *MatchPlayer) {
	// Central ownership transition: clear the old flight bookkeeping and update
	// the team transition context as well as the ball pointer. New reception or
	// interception code should use this path so phases and statistics agree.
	e.trackTouch(player)
	// A set piece's phase ends once the defending side has the ball.
	if player.IsHomeTeam != e.setPieceHome {
		e.setPiecePhaseRemaining = 0.0
	}
	if e.state == MatchState_PLAYING && e.lastControlledTeamHome.HasValue &&
		e.lastControlledTeamHome.Value != player.IsHomeTeam {
		e.transitionSecondsRemaining = Timing_POSSESSION_TRANSITION_SECONDS
		e.lastCompletedPasser = nil
		e.lastCompletedReceiver = nil
	}
	e.lastControlledTeamHome = hSome(player.IsHomeTeam)
	e.lastShooter = nil
	if e.ball.PossessedBy != player.Player {
		e.possessionStartSeconds = e.getSimulatedSeconds()
		e.statsOf(player).Touches++
	}
	e.ball.PossessedBy = player.Player
	e.ball.LastPossessor = player.Player
	e.ball.Position = player.Position
	e.ball.Z = 0.0
	e.ball.Velocity = Vector2F{0.0, 0.0}
	e.ball.VelocityZ = 0.0
	e.clearFlightState()
	// The first touch brings the ball under close control in front of him.
	e.ball.DribbleExposure = 0.0
	e.ball.DribbleTouchLength = 0.0
	e.ball.TouchTimer = 0.0
	facing := Vector2F{math.Cos(player.FacingAngle) * PITCH_LENGTH,
		math.Sin(player.FacingAngle) * PITCH_WIDTH}
	e.ball.DribbleDirection = normalized(facing)
	if length(e.ball.DribbleDirection) <= EPSILON {
		x := -1.0
		if player.IsHomeTeam {
			x = 1.0
		}
		e.ball.DribbleDirection = Vector2F{x, 0.0}
	}
	e.updateTeamPhases()
}

func (e *MatchEngine) clearFlightState() {
	e.ball.IntendedReceiver = nil
	e.ball.IsPass = false
	e.ball.PassWasOffside = false
	e.ball.IsShot = false
	e.ball.ShotOnTarget = false
	e.ball.ShotXG = 0.0
	e.ball.ShotIsHeader = false
	e.ball.ShotIsPenalty = false
	e.ball.ShotFromSetPiece = false
	e.ball.ShotSaveResolved = false
	e.ball.ShotElapsedSeconds = 0.0
	e.ball.IsAerialDelivery = false
	e.ball.FromThrowIn = false
	e.ball.Curve = 0.0
}

func (e *MatchEngine) checkOutOfBounds() {
	if e.ball.Position.X <= 0.0 || e.ball.Position.X >= 1.0 {
		rightGoalLine := e.ball.Position.X >= 1.0
		lineX := 0.0
		if rightGoalLine {
			lineX = 1.0
		}
		span := e.ball.Position.X - e.substepBallPosition.X
		t := 1.0
		if math.Abs(span) > EPSILON {
			t = hClamp((lineX-e.substepBallPosition.X)/span, 0.0, 1.0)
		}
		crossingY := e.substepBallPosition.Y + (e.ball.Position.Y-e.substepBallPosition.Y)*t
		crossingHeight := (e.substepBallZ + (e.ball.Z-e.substepBallZ)*t) * Units_BALL_Z_METRES
		lateralMetres := math.Abs(crossingY-Pitch_CENTRE) * Pitch_WIDTH_METRES
		const RADIUS = Units_BALL_RADIUS_METRES
		const HALF_GOAL = Shooting_GOAL_HALF_WIDTH_METRES
		const BAR = Units_CROSSBAR_HEIGHT_METRES
		if lateralMetres < HALF_GOAL-RADIUS && crossingHeight < BAR-RADIUS && !e.ball.FromThrowIn {
			// Any ball that crosses the line inside the frame is a goal.
			e.ball.Position.Y = crossingY
			e.scoreGoal(rightGoalLine)
			return
		}
		hitsPost := math.Abs(lateralMetres-HALF_GOAL) < RADIUS+Shooting_WOODWORK_BAND_METRES &&
			crossingHeight < BAR+RADIUS
		if hitsBar := lateralMetres < HALF_GOAL+RADIUS &&
			math.Abs(crossingHeight-BAR) < RADIUS+Shooting_WOODWORK_BAND_METRES; e.ball.IsShot && (hitsPost || hitsBar) {
			shooter := e.findMatchPlayer(e.lastShooter)
			event := e.logEvent(MatchEventType_WOODWORK)
			if shooter != nil {
				event.HasTeam = true
				event.IsHomeTeam = shooter.IsHomeTeam
				event.PrimaryPlayerId = shooter.Player.GetId()
			}
			e.ball.IsShot = false
			e.ball.ShotOnTarget = false
			if rightGoalLine {
				e.ball.Position.X = 1.0 - 0.004
			} else {
				e.ball.Position.X = 0.004
			}
			e.ball.Position.Y = crossingY
			vx := -e.ball.Velocity.X * Shooting_WOODWORK_REBOUND
			vy := e.ball.Velocity.Y + e.randomFloat(-3.0, 3.0)
			e.ball.Velocity = Vector2F{vx, vy}
			e.ball.TouchAttempts = 0
			e.ball.VelocityZ = math.Abs(e.ball.VelocityZ) * 0.3
			return
		}

		if e.ball.IsShot {
			// ours: a shot wide or over was never logged, so match stats and
			// replays undercounted shots (~28 a match).
			if shooter := e.findMatchPlayer(e.lastShooter); shooter != nil && shooter.Player != nil {
				event := e.logEvent(MatchEventType_SHOT_OFF_TARGET)
				event.HasTeam = true
				event.IsHomeTeam = shooter.IsHomeTeam
				event.PrimaryPlayerId = shooter.Player.GetId()
			}
		}
		lastTouchHome := e.isHomePlayer(e.ball.LastPossessor)
		if !rightGoalLine {
			// Home defends the left goal.
			if lastTouchHome {
				e.setupCorner(false, e.ball.Position.Y < Pitch_CENTRE)
			} else {
				e.setupGoalKick(true)
			}
		} else {
			// Away defends the right goal.
			if !lastTouchHome && e.ball.LastPossessor != nil {
				e.setupCorner(true, e.ball.Position.Y < Pitch_CENTRE)
			} else {
				e.setupGoalKick(false)
			}
		}
		return
	}

	if e.ball.Position.Y <= 0.0 || e.ball.Position.Y >= 1.0 {
		receivingHome := !e.isHomePlayer(e.ball.LastPossessor)
		e.setupThrowIn(receivingHome)
	}
}

func (e *MatchEngine) scoreGoal(homeTeam bool) {
	last := e.findMatchPlayer(e.ball.LastPossessor)
	shooter := e.findMatchPlayer(e.lastShooter)
	scorer := last
	// A shot parried or deflected in stays the shooter's goal; only a
	// defender's own deliberate touch makes it an own goal.
	if shooter != nil && shooter.IsHomeTeam == homeTeam &&
		(last == nil || last.IsHomeTeam != homeTeam) {
		scorer = shooter
	}
	ownGoal := scorer != nil && scorer.IsHomeTeam != homeTeam

	if homeTeam {
		e.homeScore++
	} else {
		e.awayScore++
	}
	e.slidersStale = true
	e.stoppageLogs[e.period-1].Goals++

	// Set-piece totals include own goals caused by a restart. Header and
	// penalty conversion totals still describe the attacking player's shot.
	if e.ball.ShotFromSetPiece {
		if homeTeam {
			e.stats.HomeSetPieceGoals++
		} else {
			e.stats.AwaySetPieceGoals++
		}
	}
	if e.ball.IsShot && scorer != nil && !ownGoal {
		if e.ball.ShotIsHeader {
			if homeTeam {
				e.stats.HomeHeadedGoals++
			} else {
				e.stats.AwayHeadedGoals++
			}
		}
		if e.ball.ShotIsPenalty {
			if homeTeam {
				e.stats.HomePenaltyGoals++
			} else {
				e.stats.AwayPenaltyGoals++
			}
		}
	}
	if scorer != nil && !ownGoal {
		// Every goal is an attempt on target; a knock-down or a ball bundled in
		// without a recorded shot is logged as one here.
		entry := e.statsOf(scorer)
		if scorer != shooter {
			entry.Shots++
			if homeTeam {
				e.stats.HomeShots++
			} else {
				e.stats.AwayShots++
			}
		}
		entry.ShotsOnTarget++
		if homeTeam {
			e.stats.HomeOnTarget++
		} else {
			e.stats.AwayOnTarget++
		}
	}
	var assisterId PlayerID = 0
	if scorer != nil && !ownGoal {
		e.statsOf(scorer).Goals++
		if assister := e.findMatchPlayer(e.shotAssistCandidate); assister != nil && assister != scorer &&
			assister.IsHomeTeam == homeTeam {
			e.statsOf(assister).Assists++
			assisterId = assister.Player.GetId()
		}
	} else if scorer != nil {
		e.statsOf(scorer).OwnGoals++
	}
	for i := range e.players {
		player := &e.players[i]
		if active(player) && player.IsHomeTeam != homeTeam {
			e.playerStats[player.StatsIndex].GoalsConceded++
		}
	}

	eventType := MatchEventType_GOAL
	if ownGoal {
		eventType = MatchEventType_OWN_GOAL
	}
	event := e.logEvent(eventType)
	// The team fields describe the primary player's side, so an own goal is
	// reported for the defender's team (it counts for the other side).
	event.HasTeam = true
	event.IsHomeTeam = homeTeam
	event.PrimaryPlayerId = 0
	event.Position = e.ball.Position
	if scorer != nil {
		event.IsHomeTeam = scorer.IsHomeTeam
		event.PrimaryPlayerId = scorer.Player.GetId()
		event.Position = scorer.Position
	}
	event.SecondaryPlayerId = assisterId
	event.Xg = 0.0
	if e.ball.IsShot {
		event.Xg = e.ball.ShotXG
	}

	e.goalScoredByHome = homeTeam
	e.goalCelebrationRemaining = e.getGoalCelebrationDuration()
	e.beginStoppage()
	e.state = MatchState_GOAL
	e.ball.PossessedBy = nil
	e.ball.ShotByHome = homeTeam
	e.pendingAdvantage.Active = false
	e.restartTakerIndex = Optional[int]{}
	e.restartIsSetPiece = false
	// Everyone walks back to the kick-off formation during the celebration.
	for i := range e.players {
		player := &e.players[i]
		if active(player) {
			player.MovementTarget = e.kickOffPosition(player)
		}
	}
	e.updateTeamPhases()
}

func (e *MatchEngine) updateBallInNet(dt float64) {
	e.integrateBall(dt)
	netDepth := Ball_GOAL_NET_BALL_DEPTH
	if e.ball.ShotByHome {
		e.ball.Position.X = hClamp(e.ball.Position.X, 1.0, 1.0+netDepth)
	} else {
		e.ball.Position.X = hClamp(e.ball.Position.X, -netDepth, 0.0)
	}
	e.ball.Position.Y = hClamp(e.ball.Position.Y, Pitch_GOAL_TOP, Pitch_GOAL_BOTTOM)
	// The ball settles into the net instead of bouncing out of the goal mouth.
	e.ball.VelocityZ = 0.0
	e.ball.Z = 0.0
	e.ball.Curve = 0.0
}

func (e *MatchEngine) makeSave(goalkeeper *MatchPlayer) {
	shooter := e.findMatchPlayer(e.lastShooter)
	if goalkeeper.IsHomeTeam {
		e.stats.HomeSaves++
		e.stats.AwayOnTarget++
	} else {
		e.stats.AwaySaves++
		e.stats.HomeOnTarget++
	}
	e.statsOf(goalkeeper).Saves++
	if shooter != nil {
		e.statsOf(shooter).ShotsOnTarget++
	}
	var shooterId PlayerID = 0
	if e.lastShooter != nil {
		shooterId = e.lastShooter.GetId()
	}
	goalkeeper.IsDiving = true
	goalkeeper.DiveTimer = Ball_SAVE_DIVE_TIME
	e.setPossession(goalkeeper)
	goalkeeper.ActionCooldown = e.randomFloat(Ball_MIN_KEEPER_HOLD_SECONDS, Ball_MAX_KEEPER_HOLD_SECONDS)
	e.state = MatchState_PLAYING
	e.keepers[hBoolIndex(goalkeeper.IsHomeTeam)].State = GoalkeeperState_HOLD
	e.updateTeamPhases()
	event := e.logEventActor(MatchEventType_SAVE, goalkeeper)
	event.SecondaryPlayerId = shooterId
}

func (e *MatchEngine) extendRestart(seconds float64) {
	if e.state != MatchState_PLAYING && e.state != MatchState_GOAL &&
		e.state != MatchState_HALF_TIME && e.state != MatchState_FULL_TIME {
		e.setPieceTimer += seconds
	}
}

func (e *MatchEngine) beginStoppage() {
	// Play has stopped: a pending advantage lapses. Several stoppage causes in
	// the same step (a red card and its free kick) form one stoppage, so they
	// share one substitution window.
	e.pendingAdvantage.Active = false
	e.setPiecePhaseRemaining = 0.0
	if e.stoppageSequence != 0 && e.lastStoppageStep == e.stepCounter {
		return
	}
	e.lastStoppageStep = e.stepCounter
	e.stoppageSequence++
}
