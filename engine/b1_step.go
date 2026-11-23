// Ported from Player12 src/model/match_engine.cpp lines 1375–2397 and 3940–3950 (GPL-3.0)

package engine

import (
	"math"
	"slices"
)

// Playback/highlight functions (1469-1600) and getInterpolationAlpha (1614)
// are not ported; captureInterpolationFrame is a no-op in skipped.go.

func (e *MatchEngine) update(deltaTime float64) {
	e.lastUpdateStepCount = 0
	if e.state == MatchState_FULL_TIME || !hIsFinite(deltaTime) || deltaTime <= 0.0 {
		return
	}

	const STEP = Timing_FIXED_STEP_SECONDS
	e.accumulator += math.Min(deltaTime, Timing_MAX_FRAME_DELTA_SECONDS)
	for e.accumulator+EPSILON >= STEP && e.state != MatchState_FULL_TIME &&
		e.lastUpdateStepCount < Timing_MAX_FIXED_STEPS_PER_UPDATE {
		e.simulateStep(STEP)
		e.accumulator -= STEP
		e.lastUpdateStepCount++
	}
	if e.lastUpdateStepCount == Timing_MAX_FIXED_STEPS_PER_UPDATE && e.accumulator >= STEP {
		e.droppedSimulationSteps += uint64(math.Floor(e.accumulator / STEP))
		e.accumulator = math.Mod(e.accumulator, STEP)
	}
}

func (e *MatchEngine) advance(seconds float64) float64 {
	e.lastUpdateStepCount = 0
	if e.state == MatchState_FULL_TIME || !hIsFinite(seconds) || seconds <= 0.0 {
		return 0.0
	}
	const STEP = Timing_FIXED_STEP_SECONDS
	steps := int64(math.Ceil(seconds/STEP - EPSILON))
	var done int64
	for ; done < steps && e.state != MatchState_FULL_TIME; done++ {
		e.simulateStep(STEP)
	}
	// C++ int is 32-bit.
	e.lastUpdateStepCount = int(min(done, int64(math.MaxInt32)))
	return float64(done) * STEP
}

func (e *MatchEngine) simulateToEnd() { e.simulateToEndFidelity(MatchFidelity_FULL) }

func (e *MatchEngine) simulateToEndFidelity(fidelity MatchFidelity) {
	background := fidelity == MatchFidelity_BACKGROUND && !e.controlledIndex.HasValue
	// Nobody reads the analytics of an unwatched fixture.
	if background {
		e.tracker.SetEnabled(false)
	}
	// A match lasts at most ~7,200 simulated seconds; the bound only guards
	// against bugs.
	const MAX_STEPS int64 = 120_000
	for step := int64(0); step < MAX_STEPS && e.state != MatchState_FULL_TIME; step++ {
		if background {
			e.setStepTicks(e.backgroundStepTicks())
		}
		e.simulateStep(e.stepSeconds())
	}
	e.setStepTicks(1)
}

func (e *MatchEngine) backgroundStepTicks() uint32 {
	switch e.state {
	case MatchState_THROW_IN, MatchState_GOAL_KICK, MatchState_CORNER_KICK,
		MatchState_FREE_KICK, MatchState_PENALTY:
	default:
		return 1
	}
	// Land on the restart instead of overshooting its delay.
	remaining := math.Ceil(e.setPieceTimer/Timing_FIXED_STEP_SECONDS - 0.001)
	if !(remaining > 1.0) {
		return 1
	}
	return min(uint32(remaining), Timing_BACKGROUND_STOPPAGE_TICKS)
}

func (e *MatchEngine) getSimulatedSeconds() float64 {
	return float64(e.stepCounter) * STEP_SECONDS
}

func (e *MatchEngine) advanceClock(dt float64) {
	clockScale := e.matchClockRate()
	clockDelta := dt * clockScale / Timing_SECONDS_PER_MINUTE
	e.matchTimeMinutes += clockDelta
	e.elapsedMatchMinutes += clockDelta
	e.accumulatePlayerLoad(dt)
}

func (e *MatchEngine) simulateStep(dt float64) {
	e.simulateStepBody(dt)
	e.describePendingEvents()
}

func (e *MatchEngine) simulateStepBody(dt float64) {
	// This is the authoritative tick order. First apply external changes and
	// age timers, then dispatch exactly one match-state branch. In live play,
	// move bodies before resolving touches/actions, and advance a newly kicked
	// ball afterwards. Reordering these stages changes contacts and seeded RNG
	// consumption.
	if e.commandCursor < len(e.commandLog) {
		e.applyDueCommands()
	}
	e.captureInterpolationFrame()
	e.stepCounter += uint64(e.stepTicks)
	if e.inputCursor < len(e.inputLog) {
		e.applyDueInputs()
	}
	// A substituted or dismissed player hands control back (play mode).
	if e.controlledIndex.HasValue {
		controlled := &e.players[e.controlledIndex.Value]
		if !active(controlled) || controlled.IsGoalkeeper ||
			controlled.Player.GetId() != e.controlledPlayerId {
			e.controlledIndex = Optional[int]{}
		}
	}
	e.controlActionRemaining = math.Max(0.0, e.controlActionRemaining-dt)
	if e.controlActionRemaining <= 0.0 {
		e.controlInput.Action = MatchInputAction_NONE
	}
	for i := range e.players {
		player := &e.players[i]
		player.TackleCooldown = math.Max(0.0, player.TackleCooldown-dt)
		player.ActionCooldown = math.Max(0.0, player.ActionCooldown-dt)
		player.TrapTimer = math.Max(0.0, player.TrapTimer-dt)
		player.DiveTimer = math.Max(0.0, player.DiveTimer-dt)
		player.IsTrapping = player.TrapTimer > 0.0
		player.IsDiving = player.DiveTimer > 0.0
	}
	e.ball.KickerLockout = math.Max(0.0, e.ball.KickerLockout-dt)
	// The sliders only move with a fading shout or with the clock while a side
	// leads; otherwise they stay as last computed.
	shouting := e.shouts[0].RemainingSeconds > 0.0 || e.shouts[1].RemainingSeconds > 0.0
	for i := range e.shouts {
		shout := &e.shouts[i]
		shout.RemainingSeconds = math.Max(0.0, shout.RemainingSeconds-dt)
		shout.Repeats = math.Max(0.0, shout.Repeats-dt/Touchline_SHOUT_REPEAT_FADE_SECONDS)
	}
	if e.reshapeSecondsRemaining[0] > 0.0 || e.reshapeSecondsRemaining[1] > 0.0 {
		for i := range e.reshapeSecondsRemaining {
			e.reshapeSecondsRemaining[i] = math.Max(0.0, e.reshapeSecondsRemaining[i]-dt)
		}
		e.refreshTargetBlends()
	}
	if shouting || e.homeScore != e.awayScore || e.slidersStale {
		e.refreshEffectiveSliders()
		e.slidersStale = false
	}
	if e.state == MatchState_PLAYING {
		e.transitionSecondsRemaining = math.Max(0.0, e.transitionSecondsRemaining-dt)
	}
	e.updateTeamPhases()

	// Dead-ball and break states have their own movement/clock rules.
	if e.state == MatchState_GOAL {
		// The scored ball settles in the net while the teams celebrate and walk
		// back; the clock keeps running until the kick-off.
		if e.playHalfMinutes > 0 {
			e.goalCelebrationRemaining = math.Min(e.goalCelebrationRemaining, Timing_PLAY_STOPPAGE_SECONDS)
		}
		e.goalCelebrationRemaining = math.Max(0.0, e.goalCelebrationRemaining-dt)
		e.updateRestartMovement(dt)
		e.advanceClock(dt)
		if e.ball.PossessedBy == nil {
			e.updateBallInNet(dt)
		}
		if e.goalCelebrationRemaining <= 0.0 {
			e.setupKickOff(!e.goalScoredByHome)
		}
		return
	}

	if e.state == MatchState_PENALTY_SHOOTOUT {
		e.updateShootout(dt)
		return
	}

	if e.state == MatchState_HALF_TIME {
		e.runAiSubstitutions()
		if e.state == MatchState_FULL_TIME {
			return
		}
		if e.playHalfMinutes > 0 {
			e.setPieceTimer = math.Min(e.setPieceTimer, Timing_PLAY_STOPPAGE_SECONDS)
		}
		if e.cfg.Modules.M9Restart {
			// Ours: during the break the players walk to their kick-off
			// spots instead of jumping there when the next half starts.
			e.feM9HalfTimeWalk(dt)
		}
		e.setPieceTimer -= dt
		if e.setPieceTimer <= 0.0 {
			e.period++
			e.matchTimeMinutes = MatchRules_periodStartMinute(e.period)
			// The home side kicks off the first half of each game, the away side
			// the second.
			e.setupKickOff(e.period%2 == 1)
			e.logEvent(MatchEventType_SECOND_HALF)
		}
		return
	}

	if e.state != MatchState_PLAYING {
		e.runAiSubstitutions()
		if e.state == MatchState_FULL_TIME {
			return
		}
		if e.state != MatchState_KICK_OFF {
			e.updateRestartMovement(dt)
		}
		if stateClockRuns(e.state) {
			e.advanceClock(dt)
		}
		restartRate := e.matchClockRate()
		if e.playHalfMinutes > 0 {
			e.setPieceTimer = math.Min(e.setPieceTimer, Timing_PLAY_STOPPAGE_SECONDS*restartRate)
		}
		e.setPieceTimer -= dt * restartRate
		if e.userRestartPassRequested() || e.setPieceTimer <= 0.0 {
			e.completeRestart()
		}
	} else {
		e.updateMovement(dt)
		e.advanceClock(dt)
		e.stats.BallInPlayMinutes += dt * e.matchClockRate() / Timing_SECONDS_PER_MINUTE
		e.resolvePossessionAndActions(dt)
		e.trackPressures()
		if e.ball.PossessedBy == nil && e.state == MatchState_PLAYING {
			e.updateBall(dt)
		}
		// Possession belongs to the side in control, including its passes in
		// flight and the loose balls it last played.
		if e.lastControlledTeamHome.HasValue {
			if e.lastControlledTeamHome.Value {
				e.homePossessionMinutes += dt
			} else {
				e.awayPossessionMinutes += dt
			}
			e.stats.HomePossession = e.homePossessionMinutes /
				(e.homePossessionMinutes + e.awayPossessionMinutes) * Statistics_PERCENT_SCALE
			e.stats.AwayPossession = Statistics_PERCENT_SCALE - e.stats.HomePossession
		}
		e.updatePendingAdvantage(dt)
		e.setPiecePhaseRemaining = math.Max(0.0, e.setPiecePhaseRemaining-dt)
		e.injuryCheckTimer += dt / Timing_SECONDS_PER_MINUTE
		if e.injuryCheckTimer >= Injury_CHECK_INTERVAL_MINUTES {
			e.injuryCheckTimer -= Injury_CHECK_INTERVAL_MINUTES
			e.checkInjuries()
		}
	}
	if e.state == MatchState_FULL_TIME {
		return
	}

	e.ratingRefreshTimer += dt
	if e.ratingRefreshTimer >= Timing_RATING_REFRESH_SECONDS {
		e.ratingRefreshTimer -= Timing_RATING_REFRESH_SECONDS
		e.refreshRatings()
	}
	e.updateMatchClock()
}

func (e *MatchEngine) isInAddedTime() bool {
	return e.state != MatchState_HALF_TIME && e.state != MatchState_FULL_TIME &&
		e.state != MatchState_PENALTY_SHOOTOUT &&
		e.matchTimeMinutes >= MatchRules_periodEndMinute(e.period)
}

func (e *MatchEngine) updateMatchClock() {
	if e.state == MatchState_HALF_TIME || e.state == MatchState_FULL_TIME || e.state == MatchState_GOAL ||
		e.state == MatchState_PENALTY_SHOOTOUT {
		return
	}
	half := e.period - 1
	regulationEnd := MatchRules_periodEndMinute(e.period)
	if e.matchTimeMinutes < regulationEnd {
		return
	}
	if e.addedMinutes[half] == 0 {
		e.announceAddedTime()
	}

	// The announced added time is a minimum: stoppages during it extend it.
	required := max(e.addedMinutes[half], MatchRules_computeAddedMinutes(e.stoppageLogs[half], e.period))
	end := regulationEnd + float64(required)
	if e.matchTimeMinutes < end {
		return
	}
	dangerousPhase := e.state == MatchState_PENALTY || e.state == MatchState_CORNER_KICK || e.ball.IsShot
	if dangerousPhase && e.matchTimeMinutes < end+Stoppage_MAX_OVERRUN_MINUTES {
		return
	}
	e.endPeriod()
}

func (e *MatchEngine) announceAddedTime() {
	half := e.period - 1
	e.addedMinutes[half] = MatchRules_computeAddedMinutes(e.stoppageLogs[half], e.period)
	event := e.logEvent(MatchEventType_ADDED_TIME)
	event.Minutes = e.addedMinutes[half]
	event.AddedMinute = 0.0
}

func (e *MatchEngine) endPeriod() {
	e.restartTakerIndex = Optional[int]{}
	e.ball.PossessedBy = nil
	e.ball.Velocity = Vector2F{0.0, 0.0}
	e.ball.VelocityZ = 0.0
	e.clearFlightState()
	e.pendingAdvantage.Active = false
	if e.abandoned {
		e.finishMatch()
		return
	}
	if e.period == 1 {
		e.beginBreak(Timing_HALF_TIME_PAUSE_SECONDS, Fatigue_HALF_TIME_RECOVERY)
		return
	}
	if e.period == 3 {
		e.beginBreak(Timing_EXTRA_TIME_HALF_TIME_SECONDS, Fatigue_EXTRA_TIME_HALF_TIME_RECOVERY)
		return
	}
	if e.knockout.Required && e.tieLevel() && !e.shootout.Started {
		if e.period == 2 && e.knockout.ExtraTime {
			e.extraTimeReached = true
			e.beginBreak(Timing_EXTRA_TIME_BREAK_SECONDS, Fatigue_EXTRA_TIME_BREAK_RECOVERY)
			return
		}
		e.startShootout()
		return
	}
	e.finishMatch()
}

func (e *MatchEngine) beginBreak(seconds float64, recovery float64) {
	e.state = MatchState_HALF_TIME
	e.setPieceTimer = seconds
	e.beginStoppage()
	for i := range e.players {
		player := &e.players[i]
		if !active(player) {
			continue
		}
		player.Stamina = math.Min(1.0, player.Stamina+recovery)
		player.SprintReserve = 1.0
		player.Velocity = Vector2F{0.0, 0.0}
	}
	e.updateTeamPhases()
	e.logEvent(MatchEventType_HALF_TIME)
}

func (e *MatchEngine) tieLevel() bool {
	return e.homeScore+e.knockout.HomeAggregate == e.awayScore+e.knockout.AwayAggregate
}

func (e *MatchEngine) setKnockout(rules Knockout) {
	if e.stepCounter != 0 {
		return
	}
	e.knockout = rules
}

func (e *MatchEngine) getTieWinnerHome() Optional[bool] {
	if !e.knockout.Required || e.state != MatchState_FULL_TIME {
		return Optional[bool]{}
	}
	home := e.homeScore + e.knockout.HomeAggregate
	if away := e.awayScore + e.knockout.AwayAggregate; home != away {
		return hSome(home > away)
	}
	if e.shootout.Goals[0] != e.shootout.Goals[1] {
		return hSome(e.shootout.Goals[0] > e.shootout.Goals[1])
	}
	return Optional[bool]{}
}

func (e *MatchEngine) maxSubstitutions() int {
	extra := 0
	if e.extraTimeReached {
		extra = Rules_EXTRA_TIME_SUBSTITUTIONS
	}
	return Rules_MAX_SUBSTITUTIONS_PER_TEAM + extra
}

func (e *MatchEngine) startShootout() {
	e.shootout.Started = true
	e.shootout.HomeFirst = e.randomFloat(0.0, 1.0) < 0.5
	// Kicking order: the designated taker (or the best one), then the others
	// on the pitch by penalty ability, the goalkeeper last.
	ability := func(player *MatchPlayer) float64 {
		return player.Shooting*0.75 + player.Passing*0.15 + player.Vision*0.10
	}
	for _, home := range [2]bool{true, false} {
		first := e.dutyTaker(home, SetPieceDuty_Penalties)
		var kickers []*MatchPlayer
		for i := range e.players {
			player := &e.players[i]
			if active(player) && player.IsHomeTeam == home {
				kickers = append(kickers, player)
			}
		}
		rank := func(player *MatchPlayer) int {
			if player == first {
				return 0
			}
			if player.IsGoalkeeper {
				return 3
			}
			if player.IsInjured {
				return 2
			}
			return 1
		}
		less := func(left, right *MatchPlayer) bool {
			if rank(left) != rank(right) {
				return rank(left) < rank(right)
			}
			return ability(left) > ability(right)
		}
		slices.SortStableFunc(kickers, func(left, right *MatchPlayer) int {
			if less(left, right) {
				return -1
			}
			if less(right, left) {
				return 1
			}
			return 0
		})
		order := make([]PlayerID, 0, len(kickers))
		for _, kicker := range kickers {
			order = append(order, kicker.Player.GetId())
		}
		e.shootout.Order[hBoolIndex(home)] = order
	}

	// Everyone waits by the halfway line; each side shoots at the goal it
	// attacked, facing the other side's keeper.
	e.state = MatchState_PENALTY_SHOOTOUT
	e.shootout.InFlight = false
	e.shootout.Timer = Timing_SHOOTOUT_START_SECONDS
	e.restartTakerIndex = Optional[int]{}
	e.ball = NewMatchBall()
	var placed [2]int
	for i := range e.players {
		player := &e.players[i]
		if !active(player) {
			continue
		}
		side := hBoolIndex(player.IsHomeTeam)
		player.Velocity = Vector2F{0.0, 0.0}
		if player.IsGoalkeeper {
			e.keepers[side].State = GoalkeeperState_SET_POSITION
			x := Pitch_PLAYER_MAX_X
			if player.IsHomeTeam {
				x = Pitch_PLAYER_MIN_X
			}
			player.Position = Vector2F{x, Pitch_CENTRE}
		} else {
			x := 0.52
			if player.IsHomeTeam {
				x = 0.48
			}
			player.Position = Vector2F{x, 0.32 + 0.04*float64(placed[side])}
			placed[side]++
		}
		player.MovementTarget = player.Position
		player.TacticalTarget = player.Position
		player.TargetOwner = TargetOwner_P12
		player.TargetReason = TargetReason_SHOOTOUT
	}
	e.updateTeamPhases()
	e.logEvent(MatchEventType_PENALTY_SHOOTOUT)
}

func (e *MatchEngine) beginShootoutKick() {
	taken := e.shootout.Kicks[0] + e.shootout.Kicks[1]
	home := (taken%2 == 0) == e.shootout.HomeFirst
	side := hBoolIndex(home)
	var taker *MatchPlayer
	if order := e.shootout.Order[side]; len(order) > 0 {
		id := order[e.shootout.Kicks[side]%len(order)]
		for i := range e.players {
			player := &e.players[i]
			if active(player) && player.Player.GetId() == id {
				taker = player
			}
		}
	}
	e.shootout.KickerHome = home
	if taker != nil {
		e.shootout.Taker = taker.Player.GetId()
	} else {
		e.shootout.Taker = 0
	}
	if taker == nil {
		e.finishShootoutKick(MatchEventDetail_MISSED)
		return
	}
	if keeper := e.findGoalkeeper(!home); keeper != nil {
		x := Pitch_PLAYER_MIN_X
		if home {
			x = Pitch_PLAYER_MAX_X
		}
		keeper.Position = Vector2F{x, Pitch_CENTRE}
		keeper.Velocity = Vector2F{0.0, 0.0}
		keeper.MovementTarget = keeper.Position
	}
	spotX := Pitch_LEFT_PENALTY_SPOT_X
	if home {
		spotX = Pitch_RIGHT_PENALTY_SPOT_X
	}
	spot := Vector2F{spotX, Pitch_CENTRE}
	taker.Position = spot
	taker.Velocity = Vector2F{0.0, 0.0}
	taker.MovementTarget = spot
	e.ball = NewMatchBall()
	e.ball.Position = spot
	e.ball.PossessedBy = taker.Player
	e.ball.LastPossessor = taker.Player
	e.strikeShot(taker, Shooting_PENALTY_XG, false, true, false)
	e.planGoalkeeperDive(!home, true)
	e.shootout.InFlight = true
}

func (e *MatchEngine) updateShootout(dt float64) {
	if !e.shootout.InFlight {
		e.shootout.Timer -= dt
		if e.shootout.Timer <= 0.0 {
			e.beginShootoutKick()
		}
		return
	}
	keeper := e.findGoalkeeper(!e.shootout.KickerHome)
	if keeper != nil {
		e.diveGoalkeeper(keeper, dt)
	}
	lineX := 0.0
	if e.shootout.KickerHome {
		lineX = 1.0
	}
	substeps := Timing_BALL_SUBSTEPS * int(e.stepTicks)
	substep := dt / float64(substeps)
	for index := 0; index < substeps; index++ {
		e.substepBallPosition = e.ball.Position
		e.substepBallZ = e.ball.Z
		e.integrateBall(substep)
		e.ball.ShotElapsedSeconds += substep
		keeperCrossing := keeper != nil && !e.ball.ShotSaveResolved &&
			(((e.substepBallPosition.X-keeper.Position.X > 0.0) !=
				(e.ball.Position.X-keeper.Position.X > 0.0)) ||
				math.Abs(e.ball.Position.X-keeper.Position.X) <= EPSILON)
		if keeperCrossing {
			e.ball.ShotSaveResolved = true
		}
		if keeperCrossing && e.ball.ShotOnTarget && e.attemptSave(keeper).Saved {
			e.finishShootoutKick(MatchEventDetail_SAVED)
			return
		}
		var crossed bool
		if e.shootout.KickerHome {
			crossed = e.ball.Position.X >= lineX
		} else {
			crossed = e.ball.Position.X <= lineX
		}
		if crossed {
			// In when it crosses the line inside the frame (the woodwork misses).
			span := e.ball.Position.X - e.substepBallPosition.X
			t := 1.0
			if math.Abs(span) > EPSILON {
				t = hClamp((lineX-e.substepBallPosition.X)/span, 0.0, 1.0)
			}
			lateralMetres := (e.substepBallPosition.Y + (e.ball.Position.Y-e.substepBallPosition.Y)*t -
				Pitch_CENTRE) * Pitch_WIDTH_METRES
			heightMetres := (e.substepBallZ + (e.ball.Z-e.substepBallZ)*t) * Units_BALL_Z_METRES
			inside := math.Abs(lateralMetres) < Shooting_GOAL_HALF_WIDTH_METRES-Units_BALL_RADIUS_METRES &&
				heightMetres < Units_CROSSBAR_HEIGHT_METRES-Units_BALL_RADIUS_METRES
			if inside {
				e.finishShootoutKick(MatchEventDetail_SCORED)
			} else {
				e.finishShootoutKick(MatchEventDetail_MISSED)
			}
			return
		}
		if e.ball.ShotElapsedSeconds >= Timing_SHOOTOUT_MAX_FLIGHT_SECONDS ||
			(e.ball.ShotElapsedSeconds > 0.3 && length(e.ball.Velocity) < Ball_DEAD_SHOT_SPEED) {
			e.finishShootoutKick(MatchEventDetail_MISSED)
			return
		}
	}
}

func (e *MatchEngine) finishShootoutKick(outcome MatchEventDetail) {
	side := hBoolIndex(e.shootout.KickerHome)
	e.shootout.Kicks[side]++
	if outcome == MatchEventDetail_SCORED {
		e.shootout.Goals[side]++
	}
	e.shootout.InFlight = false
	e.shootout.Timer = Timing_SHOOTOUT_KICK_INTERVAL_SECONDS
	e.clearFlightState()
	e.ball.PossessedBy = nil
	e.ball.Velocity = Vector2F{0.0, 0.0}
	e.ball.VelocityZ = 0.0
	e.keepers[hBoolIndex(!e.shootout.KickerHome)].State = GoalkeeperState_SET_POSITION
	var taker *MatchPlayer
	for i := range e.players {
		player := &e.players[i]
		if active(player) && player.Player.GetId() == e.shootout.Taker {
			taker = player
		}
	}
	var event *MatchEvent
	if taker != nil {
		event = e.logEventActor(MatchEventType_PENALTY_SHOOTOUT, taker)
	} else {
		event = e.logEvent(MatchEventType_PENALTY_SHOOTOUT)
	}
	event.Detail = outcome
	event.HasTeam = true
	event.IsHomeTeam = e.shootout.KickerHome
	if taker != nil {
		// The taker walks back to his team-mates.
		x := 0.53
		if taker.IsHomeTeam {
			x = 0.47
		}
		taker.Position = Vector2F{x, Pitch_CENTRE}
		taker.MovementTarget = taker.Position
	}
	if MatchRules_shootoutDecided(e.shootout.Goals[0], e.shootout.Kicks[0],
		e.shootout.Goals[1], e.shootout.Kicks[1]) {
		e.finishMatch()
	}
}

func (e *MatchEngine) finishMatch() {
	e.state = MatchState_FULL_TIME
	for i := range e.players {
		player := &e.players[i]
		if active(player) {
			e.playerStats[player.StatsIndex].Condition = player.Stamina
		}
	}
	e.refreshRatings()
	e.updateTeamPhases()
	e.logEvent(MatchEventType_FULL_TIME)
}

func (e *MatchEngine) setTransitionPhases(attackingHome bool) {
	if attackingHome {
		e.homePhase = TeamPhase_ATTACKING_TRANSITION
		e.awayPhase = TeamPhase_DEFENSIVE_TRANSITION
	} else {
		e.homePhase = TeamPhase_DEFENSIVE_TRANSITION
		e.awayPhase = TeamPhase_ATTACKING_TRANSITION
	}
}

func (e *MatchEngine) setControlledPhases(attackingHome bool, ballX float64) {
	var finalThird bool
	if attackingHome {
		finalThird = ballX >= Rules_HOME_FINAL_THIRD_START
	} else {
		finalThird = ballX <= Rules_AWAY_FINAL_THIRD_START
	}
	attackingPhase := TeamPhase_POSSESSION
	if finalThird {
		attackingPhase = TeamPhase_FINAL_THIRD
	}
	if attackingHome {
		e.homePhase = attackingPhase
		e.awayPhase = TeamPhase_DEFENSIVE_BLOCK
	} else {
		e.homePhase = TeamPhase_DEFENSIVE_BLOCK
		e.awayPhase = attackingPhase
	}
}

func (e *MatchEngine) updateTeamPhases() {
	if e.state == MatchState_HALF_TIME || e.state == MatchState_FULL_TIME {
		e.homePhase = TeamPhase_STOPPAGE
		e.awayPhase = TeamPhase_STOPPAGE
		return
	}
	if e.state != MatchState_PLAYING {
		e.homePhase = TeamPhase_SET_PIECE
		e.awayPhase = TeamPhase_SET_PIECE
		e.transitionSecondsRemaining = 0.0
		if restartOwner := e.findMatchPlayer(e.ball.PossessedBy); restartOwner != nil {
			e.lastControlledTeamHome = hSome(restartOwner.IsHomeTeam)
		}
		return
	}

	carrier := e.findMatchPlayer(e.ball.PossessedBy)
	if carrier != nil && !e.lastControlledTeamHome.HasValue {
		e.lastControlledTeamHome = hSome(carrier.IsHomeTeam)
	}

	if e.transitionSecondsRemaining > 0.0 && e.lastControlledTeamHome.HasValue {
		e.setTransitionPhases(e.lastControlledTeamHome.Value)
		return
	}

	if carrier != nil {
		e.setControlledPhases(carrier.IsHomeTeam, carrier.Position.X)
		return
	}

	// An intentional pass or shot is still part of the attacking team's
	// controlled phase.
	if (e.ball.IsPass || e.ball.IsShot) && e.lastControlledTeamHome.HasValue {
		e.setControlledPhases(e.lastControlledTeamHome.Value, e.ball.Position.X)
		return
	}

	if !e.lastControlledTeamHome.HasValue {
		e.homePhase = TeamPhase_DEFENSIVE_TRANSITION
		e.awayPhase = TeamPhase_DEFENSIVE_TRANSITION
		return
	}

	e.setTransitionPhases(e.lastControlledTeamHome.Value)
}

func (e *MatchEngine) updateMovement(dt float64) {
	// Off-ball targets refresh at the tactical rate, and immediately when the
	// ball changes hands or starts/ends a flight; bodies move every step.
	var flags uint8
	if e.ball.IsPass {
		flags |= 1
	}
	if e.ball.IsShot {
		flags |= 2
	}
	if e.lastControlledTeamHome.HasValue {
		if e.lastControlledTeamHome.Value {
			flags |= 4
		} else {
			flags |= 8
		}
	}
	if e.periodElapsed(Timing_TACTICAL_REFRESH_STEPS) || e.ball.PossessedBy != e.tacticalOwner ||
		flags != e.tacticalFlags {
		e.tacticalOwner = e.ball.PossessedBy
		e.tacticalFlags = flags
		e.refreshTacticalTargets(dt)
	} else {
		// Between refreshes only the keepers re-read the play.
		holder := e.findMatchPlayer(e.ball.PossessedBy)
		for i := range e.players {
			player := &e.players[i]
			if !active(player) || !player.IsGoalkeeper ||
				e.keepers[hBoolIndex(player.IsHomeTeam)].State == GoalkeeperState_DIVE {
				continue
			}
			if !(e.ball.IsPass && e.ball.IntendedReceiver == player.Player) {
				player.TacticalTarget = e.goalkeeperTarget(player, holder)
				player.TargetOwner = TargetOwner_P12
				player.TargetReason = TargetReason_GK
			}
		}
	}

	// Every target was read from the same snapshot of the play; now the bodies
	// move: diving keepers, the controlled player and a batch of the rest.
	var batch [32]uint8
	batched := 0
	for slot := 0; slot < len(e.players) && slot < len(batch); slot++ {
		player := &e.players[slot]
		if !active(player) {
			continue
		}
		if player.IsGoalkeeper && e.keepers[hBoolIndex(player.IsHomeTeam)].State == GoalkeeperState_DIVE {
			e.diveGoalkeeper(player, dt)
		} else if e.controlledIndex.HasValue && slot == e.controlledIndex.Value {
			e.integrateControlled(player, dt)
		} else {
			batch[batched] = uint8(slot)
			batched++
		}
	}
	e.integrateMovements(batch[:batched], dt, false)
	e.separatePlayers()
}

func (e *MatchEngine) estimatedArrivalTime(candidate *MatchPlayer, pressurePosition Vector2F, carrier *MatchPlayer) float64 {
	toTarget := toMetres(Vector2F{pressurePosition.X - candidate.Position.X,
		pressurePosition.Y - candidate.Position.Y})
	targetDistance := length(toTarget)
	if targetDistance <= EPSILON {
		return 0.0
	}
	pursuitDirection := Vector2F{toTarget.X / targetDistance, toTarget.Y / targetDistance}
	velocityTowardTarget := math.Max(0.0, candidate.Velocity.X*pursuitDirection.X+
		candidate.Velocity.Y*pursuitDirection.Y)
	pursuitSpeed := math.Max(e.currentTopSpeed(candidate)+
		velocityTowardTarget*Player_CURRENT_VELOCITY_PURSUIT_WEIGHT,
		Player_MINIMUM_PURSUIT_SPEED)
	arrivalTime := math.Max(0.0, targetDistance/pursuitSpeed-e.pressEagerness(candidate, carrier))
	if candidate.Intent == PlayerIntent_PRESS_BALL || candidate.Intent == PlayerIntent_CLAIM_LOOSE_BALL {
		arrivalTime = math.Max(0.0, arrivalTime-Shape_PRESSER_CONTINUITY_SECONDS)
	}
	return arrivalTime
}

func (e *MatchEngine) closestOutfieldPair(homeTeam bool, carrier *MatchPlayer, pressurePosition Vector2F) [2]*MatchPlayer {
	var closest [2]*MatchPlayer
	arrivalTimes := [2]float64{math.MaxFloat64, math.MaxFloat64}
	for i := range e.players {
		candidate := &e.players[i]
		if !active(candidate) || candidate.IsHomeTeam != homeTeam || candidate.IsGoalkeeper {
			continue
		}
		arrivalTime := e.estimatedArrivalTime(candidate, pressurePosition, carrier)
		if arrivalTime < arrivalTimes[0] {
			closest[1] = closest[0]
			arrivalTimes[1] = arrivalTimes[0]
			closest[0] = candidate
			arrivalTimes[0] = arrivalTime
		} else if arrivalTime < arrivalTimes[1] {
			closest[1] = candidate
			arrivalTimes[1] = arrivalTime
		}
	}

	// A purposeful pass creates a receiving run. Prioritizing that receiver
	// prevents a merely nearby teammate from making the play look arbitrary.
	if carrier == nil && e.ball.IsPass && e.ball.PassByHome == homeTeam {
		intended := e.findMatchPlayer(e.ball.IntendedReceiver)
		if intended != nil && active(intended) && !intended.IsGoalkeeper && closest[0] != intended {
			closest[1] = closest[0]
			closest[0] = intended
		}
	}
	return closest
}

func (e *MatchEngine) runPriority(candidate *MatchPlayer, carrier *MatchPlayer) float64 {
	rolePriority := Shape_MIDFIELDER_RUN_PRIORITY
	switch candidate.Player.GetRole() {
	case PlayerRole_ST:
		rolePriority = Shape_STRIKER_RUN_PRIORITY
	case PlayerRole_LW, PlayerRole_RW:
		rolePriority = Shape_WINGER_RUN_PRIORITY
	case PlayerRole_CAM:
		rolePriority = Shape_ATTACKING_MIDFIELDER_RUN_PRIORITY
	case PlayerRole_CDM:
		rolePriority = Shape_HOLDING_MIDFIELDER_RUN_PRIORITY
	case PlayerRole_CM, PlayerRole_LM, PlayerRole_RM:
	default:
		return math.Inf(-1)
	}
	depth := 1.0 - candidate.Position.X
	if candidate.IsHomeTeam {
		depth = candidate.Position.X
	}
	separation := 0.0
	if carrier != nil {
		separation = math.Abs(candidate.Position.Y - carrier.Position.Y)
	}
	variety := e.epochNoise(candidate.Player.GetId(), 0x7a11ed, RUN_TIMING_EPOCH_STEPS) *
		Shape_RUN_VARIETY_PRIORITY
	continuity := 0.0
	if candidate.IsMakingRun {
		continuity = Shape_RUN_CONTINUITY_PRIORITY
	}
	return rolePriority + variety +
		candidate.Pace*Shape_RUN_PACE_PRIORITY +
		depth*Shape_RUN_DEPTH_PRIORITY +
		separation*Shape_RUN_SEPARATION_PRIORITY +
		continuity +
		e.roleProfileOf(candidate).RunBias*TacticsTuning_RUN_BIAS_WEIGHT
}

func (e *MatchEngine) stepsWithin(deltaSeconds float64) bool {
	if e.state == MatchState_FULL_TIME || !hIsFinite(deltaSeconds) || deltaSeconds <= 0.0 {
		return false
	}
	return e.accumulator+math.Min(deltaSeconds, Timing_MAX_FRAME_DELTA_SECONDS)+EPSILON >=
		Timing_FIXED_STEP_SECONDS
}
