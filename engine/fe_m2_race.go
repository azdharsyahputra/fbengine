// Ported from FootballEngine FootballEngine.Core/Match/Systems/Team/InterceptTable.fs
// (stepsToReach, reachableDistance) and Systems/Players/PassIntercept.fs (margin,
// riskFor), GPL-3.0. FootballEngine: https://github.com/DeltaBitsSystem/FootballEngine
//
// M2.1 pass completion from a race between the ball and the bodies. Go
// adaptations (plan §4 risk 2):
//   - the ball is rolled with the engine's own physics (stepFlight at the live
//     0.025 s sub-step) at the launch speed passBall will use, not FE's aero;
//   - every body runs at its own current top speed and acceleration (the M1
//     arrival model) starting from its current velocity toward the ball, not
//     FE's uniform 8.5 m/s from a standstill;
//   - lofted passes (passBall lofts beyond 30 m and crosses) are raced along
//     their real flight: the ball can be played only while it is below
//     header height, i.e. as it leaves the foot and where it comes down.
package engine

import "math"

const (
	feM2Dt = 0.025
	// ours: FE MaxSteps 50 (1.25 s) left every pass longer than ~18 m at the
	// cap for both sides (margin 0, completion 0.5), whatever the opposition.
	feM2MaxSteps = 160
	// PassIntercept OffsideCrossRisk.
	feM2OffsideCrossRisk = 2
	// ours: the highest ball a body plays (a header; Player12's header reach
	// is about 2.5 m for an average player).
	feM2ReachHeight = 2.5
)

// Race and option calibration, all ours, fitted against the engine's own
// pass outcomes (in-engine trace, FMENGINE_M2TRACE) and the viewer/cek
// metrics on seeds 19/6/7/3/11/23. Vars so FMENGINE_M2TUNE can sweep them
// (fe_m2_trace.go).
// The race and option calibration (control and intercept radius, reaction
// delay, aerial penalty, receiver loss, curve share, shield, shot value
// scale) is per match in EngineConfig.M2 (config.go); the comments on each
// value are kept there by name.

// feM2Roll is one candidate ball path: s[n] metres along the line and the
// height h[n] after n steps.
type feM2Roll struct {
	origin, target feM1Vec
	ux, uy         float64
	lofted         bool
	s, h           [feM2MaxSteps + 1]float64
}

func (r *feM2Roll) at(n int) feM1Vec {
	return feM1Vec{r.origin.X + r.ux*r.s[n], r.origin.Y + r.uy*r.s[n]}
}

// feM2Launch mirrors passBall's launch speeds (before its execution error).
func feM2Launch(passer *MatchPlayer, travel float64, lofted, delivery bool) (speed, vertical float64) {
	if lofted {
		arrival := Passing_LOFTED_ARRIVAL_HEIGHT_METRES
		if delivery {
			arrival = Aerial_ARRIVAL_HEIGHT_METRES
		}
		speed = math.Min(Passing_LOFTED_BASE_SPEED+travel*Passing_LOFTED_SPEED_PER_METRE,
			Passing_MAX_LOFTED_SPEED)
		return speed, loftVerticalSpeed(travel, speed, 0.0, arrival)
	}
	arrival := Passing_ARRIVAL_SPEED_BASE + passer.Passing*Passing_ARRIVAL_SPEED_PASSING +
		travel*Passing_ARRIVAL_SPEED_PER_METRE
	return hClamp(groundLaunchSpeed(travel, arrival), Passing_MIN_GROUND_SPEED,
		Passing_MAX_GROUND_SPEED), 0.0
}

// feM2BuildRoll fills r for a pass from origin to target, stepping the
// engine's flight and roll physics.
func feM2BuildRoll(r *feM2Roll, origin, target feM1Vec, speed, vertical float64, lofted bool) {
	r.origin, r.target, r.lofted = origin, target, lofted
	d := feM1Dist(origin, target)
	if d > EPSILON {
		r.ux, r.uy = (target.X-origin.X)/d, (target.Y-origin.Y)/d
	} else {
		r.ux, r.uy = 0.0, 0.0
	}
	flight := FlightState{HorizontalSpeed: speed, VerticalSpeed: vertical}
	r.s[0], r.h[0] = 0.0, 0.0
	for n := 1; n <= feM2MaxSteps; n++ {
		r.s[n] = r.s[n-1] + stepFlight(&flight, feM2Dt)
		r.h[n] = flight.Height
	}
}

// feM2Reach is InterceptTable.reachableDistance for one body: its own top
// speed and acceleration, starting at its current speed along dir (unit).
func (e *MatchEngine) feM2Reach(p *MatchPlayer, t, dx, dy float64) float64 {
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

// feM2StepsToReach is InterceptTable.stepsToReach: the first step at which
// the body, starting after delay seconds, is within radius of the ball while
// the ball is low enough to be played (ours: a lofted ball can be met on its
// way up, by the man pressing the passer, and where it comes down, not in
// between). Returns feM2MaxSteps when out of reach; stops early past limit.
func (e *MatchEngine) feM2StepsToReach(p *MatchPlayer, r *feM2Roll, radius, delay float64, first, limit int) int {
	pos := feM1ToM(p.Position)
	if limit > feM2MaxSteps {
		limit = feM2MaxSteps
	}
	for n := first; n <= limit; n++ {
		if r.h[n] > feM2ReachHeight {
			continue
		}
		b := r.at(n)
		dx, dy := b.X-pos.X, b.Y-pos.Y
		d := math.Hypot(dx, dy)
		if d <= radius || e.feM2Reach(p, float64(n)*feM2Dt-delay, dx/d, dy/d)+radius >= d {
			return n
		}
	}
	return feM2MaxSteps
}

// feM2RaceMargin is PassIntercept.margin: interceptor steps + offside risk −
// receiver steps. Outfield opponents only (FE: a keeper never leaves his line
// for a pass), except that a keeper races a ball aimed into his own area
// (ours: the engine's keepers claim balls there).
func (e *MatchEngine) feM2RaceMargin(passer, receiver *MatchPlayer, r *feM2Roll, line float64) int {
	arrival := e.feM2StepsToReach(receiver, r, e.cfg.M2.ControlRadius, 0.0, 0, feM2MaxSteps)
	targetPitch := feM1ToPitch(r.target)
	interceptor := feM2MaxSteps
	for i := range e.players {
		o := &e.players[i]
		if o.IsHomeTeam == passer.IsHomeTeam || !active(o) {
			continue
		}
		if o.IsGoalkeeper && !inPenaltyArea(targetPitch, o.IsHomeTeam) {
			continue
		}
		// From the first step: the ball is struck before anyone can touch it,
		// so a pass played away from a man in contact is not blocked by him.
		if s := e.feM2StepsToReach(o, r, e.cfg.M2.InterceptRadius, e.cfg.M2.ReactionDelay, 1, interceptor); s < interceptor {
			interceptor = s
		}
	}
	risk := 0
	beyond := receiver.Position.X > line
	if !passer.IsHomeTeam {
		beyond = receiver.Position.X < line
	}
	if beyond && math.Abs(r.uy) > 0.5*math.Abs(r.ux) {
		risk = feM2OffsideCrossRisk
	}
	if r.lofted {
		risk -= int(e.cfg.M2.AerialPenalty)
	}
	return interceptor + risk - arrival
}
