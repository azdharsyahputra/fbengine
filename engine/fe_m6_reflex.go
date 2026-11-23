// Ported from FootballEngine FootballEngine.Core/Match/Systems/Players/Decision.fs (reflex, GPL-3.0);
// ReflexDecisionRadius from Contracts/BalanceConfig.fs (defaultActions). The
// mapping onto Player12's ActionCooldown (what counts as "no pending action")
// is ours.
//
// A carrier closed down within about two strides decides at once instead of
// waiting out Player12's post-touch delay: he shields, turns away, passes or
// takes the man on, but he does not stand and wait.

package engine

const (
	// feReflexRedecide: ours. After a decision taken under pressure the carrier
	// commits to it this long before the reflex may fire again, unless a new
	// opponent arrives within reach.
	feReflexRedecide = 0.5
)

// feNoteDecision records the carrier's decision (hooked at the top of
// decideAction).
func (e *MatchEngine) feNoteDecision(carrier *MatchPlayer) {
	s := e.feM6()
	s.decisionCarrier = carrier.Player
	s.decisionPossession = e.possessionStartSeconds
	s.decisionAt = e.getSimulatedSeconds()
	s.decisionOpponent = e.nearestOpponentDistance(carrier)
}

// feReflexDue is FE Decision.reflex: the ball is under control, an opponent
// is within ReflexDecisionRadius, and the carrier has no action pending for
// this pressure.
func (e *MatchEngine) feReflexDue(carrier *MatchPlayer) bool {
	if carrier.TrapTimer > 0.0 || e.isControlled(carrier) || e.state != MatchState_PLAYING {
		return false
	}
	if e.nearestOpponentDistance(carrier) > feReflexRadius {
		return false
	}
	s := e.feM6()
	if s.decisionCarrier != carrier.Player || s.decisionPossession != e.possessionStartSeconds {
		return true
	}
	if s.decisionOpponent > feReflexRadius {
		return true
	}
	return e.getSimulatedSeconds()-s.decisionAt >= feReflexRedecide
}
