// Ported from FootballEngine FootballEngine.Core/Match/Systems/Ball/Possession.fs (carry, GPL-3.0)
// and FootballEngine.Core/Match/Shared.fs (Gait.cyclesPerSecond); constants from
// Contracts/BalanceConfig.fs (defaultBall, defaultGait). The knock-on touch, the
// corrective touch and the hand-off to the Player12 loose-ball code are ours.
//
// A carried ball is a real rolling ball: on each stride-timed touch it gets the
// velocity that lands it a boot-length ahead of where the carrier will be at the
// next touch, then it rolls under the normal ground friction. Between touches
// it can be reached by a defender's boot (fe_m6_challenge.go), and if the
// carrier ends up more than a leg's reach from it he has lost it.

package engine

import "math"

const (
	feCarryOffset         = 0.32       // Ball.CarryOffset (m)
	feCarryTechniqueError = 0.6        // Ball.CarryTechniqueError
	feCarryTouchInterval  = 7.0 / 40.0 // Ball.CarryTouchInterval 7 subticks (s)
	feCarryMaxSpeed       = 9.0        // Ball.CarryMaxSpeed (m/s)
	feBallFootReach       = 1.05       // Ball.FootReach: beyond this the ball is loose (m)
	feGaitStandingSpeed   = 0.3        // Gait.StandingSpeed (m/s)
	feGaitStepBase        = 2.0        // Gait.StepFrequencyBase
	feGaitStepPerSpeed    = 0.22       // Gait.StepFrequencyPerSpeed
	feGaitStepMax         = 4.6        // Gait.StepFrequencyMax
	feGaitReferenceLeg    = 0.9        // Gait.ReferenceLegLength (m)
	feLegShareOfHeight    = 0.53       // thigh + shin as a share of stature (plan §M6c)
	feRollStep            = 1.0 / 40.0 // FE subtick, used to predict the roll
	feKnockRetouchMetres  = 0.7        // ours: a knocked-on ball is played again once this close
	feCorrectiveGapMetres = 0.9        // ours: touch early when the ball is drifting out of reach
	feKnockSeconds        = 0.5        // ours: the knock-on is aimed to arrive this far ahead in time
	feCloseControlOffset  = 0.22       // ours: touch length with an opponent within reflex range (m)
)

// feGaitCycles is FE Gait.cyclesPerSecond: stride cycles per second (two
// steps each), zero while standing.
func feGaitCycles(speed, heightMetres float64) float64 {
	if speed <= feGaitStandingSpeed {
		return 0.0
	}
	leg := math.Max(0.5, feLegShareOfHeight*heightMetres)
	return 0.5 * math.Min(feGaitStepMax, feGaitStepBase+feGaitStepPerSpeed*speed) *
		math.Sqrt(feGaitReferenceLeg/leg)
}

// feRoll advances a ground ball with launch velocity v (m/s) for the given
// seconds under the engine's rolling model and returns its displacement (m).
func feRoll(v Vector2F, seconds float64) Vector2F {
	speed := length(v)
	if speed <= EPSILON || seconds <= 0.0 {
		return Vector2F{}
	}
	steps := int(math.Ceil(seconds/feRollStep - 1e-9))
	h := seconds / float64(steps)
	flight := FlightState{speed, 0.0, 0.0, false}
	travelled := 0.0
	for i := 0; i < steps && flight.HorizontalSpeed > 0.0; i++ {
		travelled += stepFlight(&flight, h)
	}
	return Vector2F{v.X / speed * travelled, v.Y / speed * travelled}
}

// feSolveRoll is the launch velocity that rolls the ball by offset (m) in the
// given seconds: a straight guess plus FE's two corrections for rolling loss,
// capped at CarryMaxSpeed.
func feSolveRoll(offset Vector2F, seconds float64) Vector2F {
	v := Vector2F{offset.X / seconds, offset.Y / seconds}
	for i := 0; i < 2; i++ {
		arrived := feRoll(v, seconds)
		v.X += (offset.X - arrived.X) / seconds
		v.Y += (offset.Y - arrived.Y) / seconds
	}
	if speed := length(v); speed > feCarryMaxSpeed {
		v.X *= feCarryMaxSpeed / speed
		v.Y *= feCarryMaxSpeed / speed
	}
	return v
}

// feFacing is the carrier's body facing as a metric unit vector.
func feFacing(player *MatchPlayer) Vector2F {
	facing := normalized(Vector2F{math.Cos(player.FacingAngle) * PITCH_LENGTH,
		math.Sin(player.FacingAngle) * PITCH_WIDTH})
	if length(facing) <= EPSILON {
		facing = Vector2F{-1.0, 0.0}
		if player.IsHomeTeam {
			facing.X = 1.0
		}
	}
	return facing
}

// feCarryLead is FE's touch length: a boot-length ahead, longer for a poor or
// tired dribbler (deterministic technique error). Ours: closed down within
// reflex range the carrier keeps it tight (feCloseControlOffset).
func (e *MatchEngine) feCarryLead(carrier *MatchPlayer) float64 {
	technique := hClamp(carrier.Dribbling, 0.0, 1.0)
	err := feCarryTechniqueError * (1.0 - technique*hClamp(carrier.Stamina, 0.0, 1.0))
	offset := feCarryOffset
	if e.nearestOpponentDistance(carrier) <= feReflexRadius {
		offset = feCloseControlOffset
	}
	return offset * (1.0 + err)
}

// feCarry replaces updateDribble while FeM6Challenge is on. It returns false
// when the carrier has lost the ball (the ball is then loose and the Player12
// loose-ball code integrates it this tick).
func (e *MatchEngine) feCarry(carrier *MatchPlayer, dt float64) bool {
	s := e.feM6()
	// A keeper's hand-held ball stays an explicit attachment (FE GKHoldSince).
	if carrier.IsGoalkeeper && inPenaltyArea(carrier.Position, carrier.IsHomeTeam) {
		s.carrier = carrier.Player
		s.knockLength = 0.0
		return e.updateDribble(carrier, dt)
	}
	now := e.getSimulatedSeconds()
	if s.carrier != carrier.Player {
		// The first touch is a reception: it cannot wait for the stride.
		s.carrier = carrier.Player
		s.knockLength = 0.0
		s.lastTouchAt = math.Inf(-1)
		e.ball.TouchTimer = 0.0
	}
	startPosition, startVelocity := e.ball.Position, e.ball.Velocity
	startTimer, startKnock := e.ball.TouchTimer, s.knockLength

	steps := Timing_BALL_SUBSTEPS * max(1, int(e.stepTicks))
	h := dt / float64(steps)
	end := carrier.Position
	velocity := carrier.Velocity
	// The body has already moved this tick; walk it back along its velocity.
	carrierAt := func(elapsed float64) Vector2F {
		back := toPitch(Vector2F{velocity.X * (dt - elapsed), velocity.Y * (dt - elapsed)})
		return Vector2F{end.X - back.X, end.Y - back.Y}
	}
	for i := 0; i < steps; i++ {
		elapsed := float64(i) * h
		body := carrierAt(elapsed)
		e.ball.TouchTimer -= h
		gap := distance(body, e.ball.Position)
		touchAt := now - dt + elapsed
		reach := feBallFootReach
		if s.knockLength > 0.0 {
			reach = feKnockRetouchMetres
		}
		due := e.ball.TouchTimer <= 0.0 && gap <= reach
		// Ours: a touch the stride clock has not reached yet is taken early
		// rather than letting the ball run out of reach.
		corrective := s.knockLength == 0.0 && gap > feCorrectiveGapMetres &&
			gap <= feBallFootReach && touchAt-s.lastTouchAt >= feCarryTouchInterval
		if due || corrective {
			e.feCarryTouch(carrier, body, touchAt)
		}
		e.feRollBall(h)
		if distance(carrierAt(elapsed+h), e.ball.Position) > feBallFootReach+s.knockLength {
			// Lost: restore the ball as it stood at the start of the tick and
			// let the loose-ball code play this tick out.
			e.ball.Position, e.ball.Velocity = startPosition, startVelocity
			e.ball.TouchTimer, s.knockLength = startTimer, startKnock
			e.feLoseBall(carrier)
			return false
		}
	}
	e.ball.Position = Vector2F{hClamp(e.ball.Position.X, 0.001, 0.999),
		hClamp(e.ball.Position.Y, 0.001, 0.999)}
	e.ball.Z = 0.0
	e.ball.VelocityZ = 0.0
	e.ball.Curve = 0.0
	e.ball.DribbleExposure = distance(carrier.Position, e.ball.Position)
	if length(e.ball.Velocity) > EPSILON {
		e.ball.DribbleDirection = normalized(e.ball.Velocity)
	} else {
		e.ball.DribbleDirection = feFacing(carrier)
	}
	return true
}

// feCarryTouch is FE BallPossession.carry's touch: aim the ball at boot reach
// ahead of where the carrier will be at his next touch.
func (e *MatchEngine) feCarryTouch(carrier *MatchPlayer, body Vector2F, touchAt float64) {
	s := e.feM6()
	speed := length(carrier.Velocity)
	interval := feCarryTouchInterval
	if cycles := feGaitCycles(speed, carrier.HeightMetres); cycles > 0.0 {
		// Touch points sit twice per stride cycle (phase 0.08 and 0.58).
		interval = math.Max(feCarryTouchInterval, 0.5/cycles)
	}
	lead := e.feCarryLead(carrier)
	facing := feFacing(carrier)
	fromBall := toMetres(Vector2F{body.X - e.ball.Position.X, body.Y - e.ball.Position.Y})
	offset := Vector2F{
		fromBall.X + carrier.Velocity.X*interval + facing.X*lead,
		fromBall.Y + carrier.Velocity.Y*interval + facing.Y*lead,
	}
	e.ball.Velocity = feSolveRoll(offset, interval)
	e.ball.TouchTimer = interval
	// Exposure (dribbleExposureShare) is read against a leg's full reach.
	e.ball.DribbleTouchLength = feBallFootReach
	s.lastTouchAt = touchAt
	s.knockLength = 0.0
	e.trackTouch(carrier)
}

// feKnockOn pushes the ball `metres` ahead of the carrier along a metric
// direction (a take-on or an evasive touch). Until he plays it again the
// carrier may be up to that far from it without losing it. Ours.
func (e *MatchEngine) feKnockOn(carrier *MatchPlayer, direction Vector2F, metres float64) {
	s := e.feM6()
	fromBall := toMetres(Vector2F{carrier.Position.X - e.ball.Position.X,
		carrier.Position.Y - e.ball.Position.Y})
	offset := Vector2F{fromBall.X + direction.X*metres, fromBall.Y + direction.Y*metres}
	e.ball.Velocity = feSolveRoll(offset, feKnockSeconds)
	e.ball.TouchTimer = feKnockSeconds
	e.ball.DribbleDirection = direction
	e.ball.DribbleTouchLength = metres
	s.lastTouchAt = e.getSimulatedSeconds()
	s.knockLength = metres
	e.trackTouch(carrier)
}

func (e *MatchEngine) feRollBall(h float64) {
	speed := length(e.ball.Velocity)
	if speed <= EPSILON {
		e.ball.Velocity = Vector2F{}
		return
	}
	direction := Vector2F{e.ball.Velocity.X / speed, e.ball.Velocity.Y / speed}
	flight := FlightState{speed, 0.0, 0.0, false}
	travelled := stepFlight(&flight, h)
	d := toPitch(Vector2F{direction.X * travelled, direction.Y * travelled})
	e.ball.Position.X += d.X
	e.ball.Position.Y += d.Y
	e.ball.Velocity = Vector2F{direction.X * flight.HorizontalSpeed, direction.Y * flight.HorizontalSpeed}
}

// feLoseBall frees a carried ball that ran out of the carrier's reach; the
// Player12 loose-ball code decides who gets it (the carrier included).
func (e *MatchEngine) feLoseBall(carrier *MatchPlayer) {
	s := e.feM6()
	s.carrier = nil
	s.knockLength = 0.0
	e.ball.PossessedBy = nil
	e.ball.LastPossessor = carrier.Player
	e.ball.Kicker = carrier.Player
	e.ball.KickerLockout = 0.0
	e.ball.TouchAttempts = 0
	e.ball.Z = 0.0
	e.ball.VelocityZ = 0.0
	e.ball.Curve = 0.0
	e.ball.DribbleExposure = 0.0
}
