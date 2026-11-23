// Ours: per-match side state for the FE M6 port (challenges, carry touches,
// reflex and standoff). Part of the GPL-3.0 fmengine module.
//
// M6 needs a little memory that the Player12 MatchEngine struct has no field
// for (pending challenges, the carry touch clock, the last decision). It is
// kept beside the engine, keyed by a weak pointer, so match_engine.go is left
// untouched and the entry disappears with the engine.

package engine

// feChallenge is one defender's standing or sliding tackle between the moment
// he decides to go in and the moment his boot lands (FE ContactAction
// Preparing -> Committed).
type feChallenge struct {
	active   bool
	sliding  bool
	target   *Player
	commitAt float64  // simulated seconds at which the boot lands
	predict  Vector2F // ball point (pitch coordinates) the boot is sent to
}

type feM6State struct {
	challenges [32]feChallenge // indexed like e.players

	// Carry touch clock.
	carrier     *Player
	lastTouchAt float64
	knockLength float64 // metres the last touch pushed the ball beyond normal reach

	// Reflex: the carrier's last decision.
	decisionCarrier    *Player
	decisionPossession float64
	decisionAt         float64
	decisionOpponent   float64

	// Standoff: seconds the carrier has been held up with an opponent close.
	standoffCarrier *Player
	standoffSeconds float64
}

// feM6 returns this match's M6 state (formerly a global map keyed by a weak
// engine pointer; MatchEngine is never copied, so a field is equivalent).
func (e *MatchEngine) feM6() *feM6State {
	return &e.feM6Data
}
