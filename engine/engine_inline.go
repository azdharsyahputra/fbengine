// Ported from Player12 src/model/match_engine.h (inline member functions defined in the header) (GPL-3.0)

package engine

// The accessors below are defined inline in match_engine.h, so they belong
// to the foundation rather than to a match_engine.cpp stage. Playback,
// highlight and interpolation accessors are not ported (see PORTING.md);
// the play-mode inline accessors live in controlled.go.

// getTacticalFamiliarity is the familiarity in force: the drilled level,
// less a temporary loss after a change of shape (setFormation).
func (e *MatchEngine) getTacticalFamiliarity(homeTeam bool) float64 {
	return e.effectiveFamiliarity(hBoolIndex(homeTeam))
}

// getCommandLog returns every tactical change made from outside,
// step-stamped. Must not be modified.
func (e *MatchEngine) getCommandLog() []MatchCommandRecord { return e.commandLog }

// getSimulatedSteps is the number of fixed steps simulated since kick-off.
func (e *MatchEngine) getSimulatedSteps() uint64 { return e.stepCounter }

// getPlayers returns the on-pitch slots. Must not be modified by callers
// outside the engine.
func (e *MatchEngine) getPlayers() []MatchPlayer { return e.players }

func (e *MatchEngine) getBall() *MatchBall { return &e.ball }

func (e *MatchEngine) getEvents() []MatchEvent { return e.events }

func (e *MatchEngine) getState() MatchState { return e.state }

func (e *MatchEngine) getStats() *MatchStats { return &e.stats }

func (e *MatchEngine) getHomePhase() TeamPhase { return e.homePhase }

func (e *MatchEngine) getAwayPhase() TeamPhase { return e.awayPhase }

func (e *MatchEngine) getTransitionSecondsRemaining() float64 {
	return e.transitionSecondsRemaining
}

func (e *MatchEngine) getLastPassDecision() *PassDecision { return &e.lastPassDecision }

func (e *MatchEngine) getSubstitutions() []MatchSubstitution { return e.substitutions }

// getPlayerStats returns per-player statistics for everyone who took part
// (never truncated).
func (e *MatchEngine) getPlayerStats() []PlayerMatchStats { return e.playerStats }

func (e *MatchEngine) getTracker() *MatchTracker { return &e.tracker }

func (e *MatchEngine) setTracking(on bool) { e.tracker.SetEnabled(on) }

// getPeriod is the current period: 1 or 2, then 3 and 4 in extra time.
func (e *MatchEngine) getPeriod() int { return e.period }

// getAddedMinutes returns the announced added minutes of a period 1-4 (0
// until announced).
func (e *MatchEngine) getAddedMinutes(half int) int {
	if half >= 1 && half <= len(e.addedMinutes) {
		return e.addedMinutes[half-1]
	}
	return 0
}

func (e *MatchEngine) getKnockout() Knockout { return e.knockout }

func (e *MatchEngine) getMatchContext() MatchContext { return e.matchContext }

// wentToExtraTime is true once the match has gone into extra time.
func (e *MatchEngine) wentToExtraTime() bool { return e.extraTimeReached }

// hasShootout is true once a penalty shootout has started.
func (e *MatchEngine) hasShootout() bool { return e.shootout.Started }

// getShootoutScore returns the shootout goals of a side.
func (e *MatchEngine) getShootoutScore(homeTeam bool) int {
	return e.shootout.Goals[hBoolIndex(homeTeam)]
}

// getShootoutKicks returns the shootout kicks taken by a side.
func (e *MatchEngine) getShootoutKicks(homeTeam bool) int {
	return e.shootout.Kicks[hBoolIndex(homeTeam)]
}

// isAbandoned is true when a side fell below seven players and the match was
// stopped.
func (e *MatchEngine) isAbandoned() bool { return e.abandoned }

// getRefereeStrictness is the per-match referee strictness (1 is average).
func (e *MatchEngine) getRefereeStrictness() float64 { return e.refereeStrictness }

func (e *MatchEngine) getHomeScore() int { return e.homeScore }

func (e *MatchEngine) getAwayScore() int { return e.awayScore }

func (e *MatchEngine) getMatchTimeMinutes() float64 { return e.matchTimeMinutes }

// getElapsedMatchMinutes is the total clock minutes played so far,
// including both added times.
func (e *MatchEngine) getElapsedMatchMinutes() float64 { return e.elapsedMatchMinutes }

func (e *MatchEngine) getLastUpdateStepCount() int { return e.lastUpdateStepCount }

func (e *MatchEngine) getDroppedSimulationSteps() uint64 { return e.droppedSimulationSteps }

// getGoalScoredByHome reports whether the last scored goal was conceded by
// the away team's keeper.
func (e *MatchEngine) getGoalScoredByHome() bool { return e.goalScoredByHome }

// getGoalCelebrationRemaining is the seconds left in the goal celebration
// before the kick-off restart.
func (e *MatchEngine) getGoalCelebrationRemaining() float64 { return e.goalCelebrationRemaining }

func (e *MatchEngine) getGoalCelebrationDuration() float64 {
	if e.playHalfMinutes > 0 {
		return Timing_PLAY_STOPPAGE_SECONDS
	}
	return Timing_GOAL_CELEBRATION_SECONDS
}

// getHomeGoalkeeperState is the semantic goalkeeper state of the home side.
func (e *MatchEngine) getHomeGoalkeeperState() GoalkeeperState { return e.keepers[0].State }

// getAwayGoalkeeperState is the semantic goalkeeper state of the away side.
func (e *MatchEngine) getAwayGoalkeeperState() GoalkeeperState { return e.keepers[1].State }

func (e *MatchEngine) getLastScenarioDecision() *ScenarioDecision {
	return &e.lastScenarioDecision
}

// matchClockRate is the match-clock speed-up in play mode (1 in watch mode).
func (e *MatchEngine) matchClockRate() float64 {
	if e.playHalfMinutes > 0 {
		return Timing_HALF_TIME_MINUTE / float64(e.playHalfMinutes)
	}
	return 1.0
}
