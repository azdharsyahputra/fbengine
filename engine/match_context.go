// Ported from Player12 src/model/match_context.h (GPL-3.0)

package engine

// MatchContext holds per-match balance settings, independent of the engine's
// mutable state. Defaults preserve the calibrated engine.
//
// MatchEngine.setMatchContext() accepts settings only before the first step,
// clamps finite values to the Context_* bounds and replaces non-finite values
// with defaults.
type MatchContext struct {
	// Finishing precision: above 1 sharpens shots, below 1 makes them less
	// accurate. Changes execution, not the score or the xG formula directly.
	GoalRateScale float64
	// Mean and spread of the referee strictness drawn at kick-off.
	RefereeStrictnessMean float64
	RefereeStrictnessSd   float64
	// Scales the home edge on attributes, execution and refereeing;
	// 0 represents a neutral venue.
	HomeAdvantageScale float64
}

// NewMatchContext returns a MatchContext with the C++ default member values.
func NewMatchContext() MatchContext {
	return MatchContext{
		GoalRateScale:         1.0,
		RefereeStrictnessMean: 1.0,
		RefereeStrictnessSd:   Discipline_STRICTNESS_SD,
		HomeAdvantageScale:    1.0,
	}
}
