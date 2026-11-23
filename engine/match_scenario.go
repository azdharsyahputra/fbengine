// Ported from Player12 src/model/match_scenario.h (GPL-3.0)

package engine

// MatchScenarioPlayer is the placement of a single player within a
// deterministic match scenario.
type MatchScenarioPlayer struct {
	PlayerId  uint32
	Position  Vector2F
	MakingRun bool
}

// NewMatchScenarioPlayer returns a MatchScenarioPlayer with the C++ defaults.
func NewMatchScenarioPlayer() MatchScenarioPlayer {
	return MatchScenarioPlayer{Position: Vector2F{Pitch_CENTRE, Pitch_CENTRE}}
}

// MatchScenario is a fully-specified, deterministic snapshot of players and
// ball that the match engine can load (headless scenario suite).
type MatchScenario struct {
	Players      []MatchScenarioPlayer
	CarrierId    uint32
	BallPosition Vector2F
}

// NewMatchScenario returns a MatchScenario with the C++ defaults.
func NewMatchScenario() MatchScenario {
	return MatchScenario{BallPosition: Vector2F{Pitch_CENTRE, Pitch_CENTRE}}
}
