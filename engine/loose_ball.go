// Loose-ball resolver (ours, Phase 2): the single owner of who contests a
// ball that nobody controls and no pass is carrying. Before it, Player12's
// branch sent each side's fastest player (and a second one under heavy
// pressing) straight at the ball, however hopeless the race, and pulled the
// last side's supporters toward it: the audit probe found ≥5 players running
// at a truly loose ball in ~25% of such moments (real football: far fewer).
//
// The race uses one model: M1's arrival time (feM1ArrivalTime, full effort)
// to the ball's control point (M4's inertia anchor, where the ball will be
// playable). A side that loses the race by more than Loose.ContestGap does
// not sprint at the ball: its first man delays ContestGap… DelayDistance
// goal-side of the control point, ready for the winner's first touch.
package engine

import "math"

type looseRace struct {
	active bool
	late   [2]bool // per side (hBoolIndex): lost the race by more than ContestGap
	point  Vector2F
}

// looseRaceFor resolves the race for this refresh. pressers are each side's
// arrival-ordered pair from closestOutfieldPair.
func (e *MatchEngine) looseRaceFor(carrier *MatchPlayer, anchor Vector2F, home, away [2]*MatchPlayer) looseRace {
	r := looseRace{point: anchor}
	if carrier != nil || e.ball.IsPass || e.ball.IsShot || e.cfg.Loose.ContestGap < 0 ||
		home[0] == nil || away[0] == nil {
		return r
	}
	r.active = true
	at := feM1ToM(anchor)
	th := e.feM1ArrivalTime(1.0, home[0], at)
	ta := e.feM1ArrivalTime(1.0, away[0], at)
	r.late[hBoolIndex(true)] = th-ta > e.cfg.Loose.ContestGap
	r.late[hBoolIndex(false)] = ta-th > e.cfg.Loose.ContestGap
	return r
}

// looseDelayPoint is where a side that lost the race waits: DelayDistance
// toward its own goal from the control point.
func (e *MatchEngine) looseDelayPoint(r looseRace, homeTeam bool) Vector2F {
	p := feM1ToM(r.point)
	goal := feM1Vec{feM1OwnGoalX(homeTeam), 0}
	d := math.Max(feM1Dist(p, goal), EPSILON)
	step := math.Min(e.cfg.Loose.DelayDistance, d)
	q := feM1ToPitch(feM1Vec{p.X + (goal.X-p.X)/d*step, p.Y + (goal.Y-p.Y)/d*step})
	return Vector2F{hClamp(q.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X), hClamp(q.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
}
