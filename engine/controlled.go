// Ported from Player12 src/model/match_engine.h and src/model/match_engine.cpp lines 3365-3970 (play-mode seam only) (GPL-3.0)

package engine

// Controlled-player input (play mode) is NOT ported. This file keeps the
// engine's "nobody is controlled" behaviour faithful so that the other
// stages compile and behave exactly as the C++ does in watch mode:
//
//   - isControlled, getControlledPlayer, applyDueInputs and the play-mode
//     inline accessors are faithful ports (with an empty input log they are
//     no-ops / return "not controlled").
//   - setControlledPlayer always refuses (returns false), so nobody is ever
//     controlled and the remaining play-mode methods are unreachable; they
//     panic if reached.
//   - setPlayHalfMinutes, submitInput, loadInputReplay, suggestActivePlayer,
//     nextSwitchCandidate are not provided (public play-mode API only).

func (e *MatchEngine) isControlled(player *MatchPlayer) bool {
	return e.controlledIndex.HasValue && player == &e.players[e.controlledIndex.Value]
}

// getControlledPlayer is the controlled player, or 0 when the AI controls
// everyone.
func (e *MatchEngine) getControlledPlayer() PlayerID {
	// A change scheduled for the next step already counts.
	var scheduled Optional[PlayerID]
	for index := e.inputCursor; index < len(e.inputLog) && e.inputLog[index].Step <= e.stepCounter+1; index++ {
		scheduled = hSome(e.inputLog[index].Player)
	}
	if scheduled.HasValue {
		return scheduled.Value
	}
	if e.controlledIndex.HasValue {
		return e.players[e.controlledIndex.Value].Player.GetId()
	}
	return 0
}

// setControlledPlayer: play mode is not ported; control is always refused.
func (e *MatchEngine) setControlledPlayer(playerId PlayerID) bool {
	_ = playerId
	return false
}

func (e *MatchEngine) getPlayHalfMinutes() int { return e.playHalfMinutes }

func (e *MatchEngine) getInputLog() []MatchInputRecord { return e.inputLog }

func (e *MatchEngine) wasControlled(homeTeam bool) bool {
	return e.everControlled[hBoolIndex(homeTeam)]
}

func (e *MatchEngine) getControlledActions(homeTeam bool) int {
	return e.controlledActions[hBoolIndex(homeTeam)]
}

// applyDueInputs applies input records due at the current step.
func (e *MatchEngine) applyDueInputs() {
	for e.inputCursor < len(e.inputLog) && e.inputLog[e.inputCursor].Step <= e.stepCounter {
		record := e.inputLog[e.inputCursor]
		e.inputCursor++
		if record.PlayHalfMinutes == 0 {
			e.playHalfMinutes = 0
		} else {
			e.playHalfMinutes = hClampInt(record.PlayHalfMinutes, Timing_MIN_PLAY_HALF_MINUTES, Timing_MAX_PLAY_HALF_MINUTES)
		}
		e.pendingPlayHalfMinutes = e.playHalfMinutes
		// An action fires on the press, not while the button stays held.
		pressed := record.Input.Action != MatchInputAction_NONE && record.Input.Action != e.lastInputAction
		e.lastInputAction = record.Input.Action
		// A buffered press survives stick updates but not a change of player.
		samePlayer := e.controlledIndex.HasValue &&
			e.players[e.controlledIndex.Value].Player.GetId() == record.Player
		pending := MatchInputAction_NONE
		pendingRemaining := 0.0
		if samePlayer {
			pending = e.controlInput.Action
			pendingRemaining = e.controlActionRemaining
		}
		e.controlledIndex = Optional[int]{}
		e.controlInput = MatchPlayerInput{}
		e.controlActionRemaining = 0.0
		if record.Player == 0 {
			continue
		}
		found := -1
		for i := range e.players {
			candidate := &e.players[i]
			if active(candidate) && !candidate.IsGoalkeeper && candidate.Player.GetId() == record.Player {
				found = i
				break
			}
		}
		if found < 0 {
			continue
		}
		e.controlledIndex = hSome(found)
		e.controlledPlayerId = record.Player
		e.everControlled[hBoolIndex(e.players[found].IsHomeTeam)] = true
		e.controlInput = record.Input
		if pressed {
			e.controlActionRemaining = Control_ACTION_BUFFER_SECONDS
		} else {
			// A stick update keeps a buffered press alive.
			e.controlInput.Action = pending
			e.controlActionRemaining = pendingRemaining
		}
		// The body keeps its momentum; the AI resumes from where he stands.
		player := &e.players[e.controlledIndex.Value]
		player.MovementTarget = player.Position
	}
}

const hPlayModeNotPorted = "fmengine: play mode (controlled player) is not ported"

// The methods below are unreachable while setControlledPlayer refuses.

func (e *MatchEngine) integrateControlled(player *MatchPlayer, dt float64) {
	panic(hPlayModeNotPorted)
}

func (e *MatchEngine) performControlledAction(carrier *MatchPlayer) bool {
	panic(hPlayModeNotPorted)
}

func (e *MatchEngine) controlledAim(carrier *MatchPlayer) Vector2F {
	panic(hPlayModeNotPorted)
}

func (e *MatchEngine) controlledPassReceiver(carrier *MatchPlayer, aim Vector2F, throughBall bool) *MatchPlayer {
	panic(hPlayModeNotPorted)
}

func (e *MatchEngine) playControlledPass(carrier *MatchPlayer, action MatchInputAction) {
	panic(hPlayModeNotPorted)
}

// shapeControlledShot: C++ float& out-parameters become *float64.
func (e *MatchEngine) shapeControlledShot(shooter *MatchPlayer, halfGoal, inset float64, aimY, aimZ, spread *float64) {
	panic(hPlayModeNotPorted)
}

func (e *MatchEngine) controlledAssistTarget(player *MatchPlayer) Optional[Vector2F] {
	panic(hPlayModeNotPorted)
}

func (e *MatchEngine) switchSeconds(candidate *MatchPlayer) float64 {
	panic(hPlayModeNotPorted)
}
