// Ported from Player12 src/model/match_engine.cpp lines 9514–9701 (GPL-3.0)

package engine

import "math"

// describePendingEvents (9636) and recordHighlight (9648) are not ported
// here: skipped.go provides their no-op versions.

func (e *MatchEngine) applyScenario(scenario *MatchScenario, scenarioState MatchState, scenarioMatchTime float64, scenarioHomeScore int, scenarioAwayScore int) bool {
	if len(scenario.Players) == 0 || e.state == MatchState_FULL_TIME {
		return false
	}

	e.ball = NewMatchBall()
	e.state = scenarioState
	e.transitionSecondsRemaining = 0.0
	e.matchTimeMinutes = scenarioMatchTime
	if scenarioMatchTime > Timing_HALF_TIME_MINUTE {
		e.period = 2
	} else {
		e.period = 1
	}
	e.keepers = [2]GoalkeeperControl{NewGoalkeeperControl(), NewGoalkeeperControl()}
	e.pendingAdvantage = NewPendingAdvantage()
	e.homeScore = scenarioHomeScore
	e.awayScore = scenarioAwayScore
	e.slidersStale = true
	e.accumulator = 0.0
	e.lastPassDecision = NewPassDecision()
	e.lastScenarioDecision.Best = Optional[PassDecision]{}
	e.lastScenarioDecision.RunnerUp = Optional[PassDecision]{}
	e.lastScenarioDecision.Reason = ""
	e.lastScenarioDecision.PassUtility = math.Inf(-1)
	e.lastScenarioDecision.ShotUtility = math.Inf(-1)
	e.lastScenarioDecision.CarryUtility = math.Inf(-1)
	e.lastScenarioDecision.ShieldUtility = math.Inf(-1)

	e.ball.Position = scenario.BallPosition
	carrierFound := false
	for _, placement := range scenario.Players {
		found := -1
		for i := range e.players {
			if e.players[i].Player != nil && e.players[i].Player.GetId() == placement.PlayerId {
				found = i
				break
			}
		}
		if found < 0 {
			continue
		}
		matchPlayer := &e.players[found]
		matchPlayer.Position = placement.Position
		matchPlayer.Velocity = Vector2F{0.0, 0.0}
		matchPlayer.BasePosition = placement.Position
		matchPlayer.MovementTarget = placement.Position
		matchPlayer.IsTrapping = false
		matchPlayer.TrapTimer = 0.0
		matchPlayer.IsPressing = false
		matchPlayer.IsMakingRun = false
		matchPlayer.Stamina = 1.0
		matchPlayer.ActionCooldown = 0.0
		matchPlayer.TackleCooldown = 0.0
		matchPlayer.IsMakingRun = placement.MakingRun
		matchPlayer.IsInjured = false
		matchPlayer.DiveTimer = 0.0
		matchPlayer.IsDiving = false
		carrierFound = carrierFound || placement.PlayerId == scenario.CarrierId
	}
	if !carrierFound {
		return false
	}

	var carrier *MatchPlayer
	for i := range e.players {
		if e.players[i].Player != nil && e.players[i].Player.GetId() == scenario.CarrierId {
			carrier = &e.players[i]
			break
		}
	}
	if carrier == nil {
		return false
	}

	e.ball.PossessedBy = carrier.Player
	e.ball.LastPossessor = carrier.Player
	if carrier.IsHomeTeam {
		e.homePhase = TeamPhase_POSSESSION
		e.awayPhase = TeamPhase_DEFENSIVE_BLOCK
	} else {
		e.awayPhase = TeamPhase_POSSESSION
		e.homePhase = TeamPhase_DEFENSIVE_BLOCK
	}

	// Make the interpolation baseline equal to the scenario so the snapshot is
	// stable, then evaluate the decision through the normal live path. A
	// carrier under external control (play mode) makes no AI decision: he
	// acts on his controller's input like in a live match.
	e.captureInterpolationFrame()
	if !e.isControlled(carrier) {
		e.decideAction(carrier)
	}
	return true
}

// logEvent appends an event and returns a pointer to it. Like the C++
// reference, the pointer is valid until the next logEvent call.
func (e *MatchEngine) logEvent(t MatchEventType) *MatchEvent {
	if len(e.events) >= Timing_MAX_EVENTS {
		// events.erase(events.begin())
		copy(e.events, e.events[1:])
		e.events = e.events[:len(e.events)-1]
	}
	e.events = append(e.events, NewMatchEvent())
	event := &e.events[len(e.events)-1]
	e.undescribedEvents = min(e.undescribedEvents+1, len(e.events))
	event.TimeMinute = e.matchTimeMinutes
	event.Type = t
	event.HomeScore = e.homeScore
	event.AwayScore = e.awayScore
	event.HomeShootout = e.shootout.Goals[0]
	event.AwayShootout = e.shootout.Goals[1]
	event.Period = e.period
	event.AddedMinute = math.Max(0.0, e.matchTimeMinutes-MatchRules_periodEndMinute(e.period))
	event.Position = e.ball.Position
	if isHighlightTrigger(t) {
		e.recordHighlight(t)
	}
	return event
}

func (e *MatchEngine) setTeamNames(homeTeam string, awayTeam string) {
	e.teamNames = [2]string{homeTeam, awayTeam}
}

func (e *MatchEngine) squadPlayerName(playerId PlayerID) string {
	for _, player := range e.squad {
		if player.GetId() == playerId {
			return player.GetName()
		}
	}
	return ""
}

func (e *MatchEngine) logEventActor(t MatchEventType, actor *MatchPlayer) *MatchEvent {
	event := e.logEvent(t)
	event.HasTeam = true
	event.IsHomeTeam = actor.IsHomeTeam
	if actor.Player != nil {
		event.PrimaryPlayerId = actor.Player.GetId()
	} else {
		event.PrimaryPlayerId = 0
	}
	event.Position = actor.Position
	return event
}
