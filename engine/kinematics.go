// Body kinematics (ours, Phase 2 / B1): the one arrival model every subsystem
// uses. It is FootballEngine's arrivalTimeFor / reachableDistance
// (DefensiveTransition.fs:10-47, InterceptTable.fs) with the player's own
// current top speed (fatigue, sprint reserve, injury), acceleration
// (Acceleration·feM1AccelerationShare) and current velocity along the way,
// asked in two directions:
//   - bodyTimeTo: seconds to come within foot reach of a point (at an effort
//     between a jog and a sprint) — formerly feM1ArrivalTime (M1, M5, loose);
//   - bodyReach: metres covered along a direction in t seconds at full
//     effort — formerly feM2Reach (the M2 pass race).
//
// The formulas are moved here unchanged.
package engine

import "math"

// bodyTimeTo is the time for p to come within feM1FootReach of target (FE
// metre frame) at the given effort (0 jog … 1 sprint).
func (e *MatchEngine) bodyTimeTo(p *MatchPlayer, target feM1Vec, effort float64) float64 {
	pos := feM1ToM(p.Position)
	dx, dy := target.X-pos.X, target.Y-pos.Y
	norm := math.Hypot(dx, dy)
	d := math.Max(0.0, norm-feM1FootReach)
	if d == 0.0 {
		return 0.0
	}
	speed := e.currentTopSpeed(p) * (feM1JogShare + (1.0-feM1JogShare)*effort)
	if speed <= 0.0 {
		return math.Inf(1)
	}
	acc := math.Max(0.5, p.Acceleration*feM1AccelerationShare)
	initial := hClamp((p.Velocity.X*dx+p.Velocity.Y*dy)/norm, 0.0, speed)
	accelDist := (speed*speed - initial*initial) / (2.0 * acc)
	if d <= accelDist {
		return (math.Sqrt(initial*initial+2.0*acc*d) - initial) / acc
	}
	return (speed-initial)/acc + (d-accelDist)/speed
}

// bodyReach is how far p gets along the unit direction (dx, dy) in t seconds
// at full effort, starting from his current speed along it.
func (e *MatchEngine) bodyReach(p *MatchPlayer, t, dx, dy float64) float64 {
	if t <= 0.0 {
		return 0.0
	}
	top := e.currentTopSpeed(p)
	acc := math.Max(0.5, p.Acceleration*feM1AccelerationShare)
	u := hClamp(p.Velocity.X*dx+p.Velocity.Y*dy, 0.0, top)
	ramp := (top - u) / acc
	if t <= ramp {
		return u*t + 0.5*acc*t*t
	}
	return (top*top-u*u)/(2.0*acc) + top*(t-ramp)
}
