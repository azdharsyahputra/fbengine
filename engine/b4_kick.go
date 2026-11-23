// Ported from Player12 src/model/match_engine.cpp lines 5946–6468 (GPL-3.0)

package engine

import (
	"math"
	"sort"
	"sync"
)

func (e *MatchEngine) passBall(passer *MatchPlayer, option *PassOption, forceLofted bool) {
	if option.Receiver == nil || option.Receiver.Player == nil {
		return
	}
	receiver := option.Receiver
	target := option.TargetPoint
	passDistance := option.PassDistance
	if passDistance <= EPSILON {
		return
	}
	lofted := forceLofted || option.Lofted

	e.clearFlightState()
	e.ball.IntendedReceiver = receiver.Player
	e.ball.IsPass = true
	e.ball.PassByHome = passer.IsHomeTeam
	defenderLine := e.offsideLine(passer.IsHomeTeam)
	e.ball.PassWasOffside =
		e.state != MatchState_KICK_OFF && e.state != MatchState_THROW_IN &&
			e.state != MatchState_GOAL_KICK && e.state != MatchState_CORNER_KICK &&
			e.isOffside(receiver, passer.IsHomeTeam, defenderLine, 0.0)
	if !e.ball.PassWasOffside && e.state == MatchState_PLAYING &&
		option.Progression > 0.0 &&
		(receiver.IsMakingRun || isAttackingRole(receiver.Player.GetRole())) {
		// A runner level with the line at the moment of the pass has often
		// timed his run a fraction early: the tighter, the likelier.
		sign := -1.0
		if passer.IsHomeTeam {
			sign = 1.0
		}
		gap := (defenderLine - receiver.Position.X) * sign
		var opponentHalf bool
		if passer.IsHomeTeam {
			opponentHalf = receiver.Position.X > Pitch_CENTRE
		} else {
			opponentHalf = receiver.Position.X < Pitch_CENTRE
		}
		if opponentHalf && gap >= 0.0 &&
			gap < Passing_OFFSIDE_TIMING_WINDOW {
			chance :=
				e.cfg.M2.OffsideTiming *
					(1.0 - gap/Passing_OFFSIDE_TIMING_WINDOW)
			e.ball.PassWasOffside =
				(e.hashNoise(receiver.Player.GetId(), 0x5eed)+1.0)*0.5 < chance
		}
	}
	if e.ball.PassWasOffside {
		e.auditOffsidePass(receiver, passer, e.isOffside(receiver, passer.IsHomeTeam, defenderLine, 0.0), defenderLine)
	}
	pressure := hClamp((Passing_PASS_PRESSURE_RADIUS_METRES-
		e.nearestOpponentDistance(passer))/
		Passing_PASS_PRESSURE_RADIUS_METRES,
		0.0, 1.0)
	var passerId, receiverId PlayerID
	if passer.Player != nil {
		passerId = passer.Player.GetId()
	}
	if receiver.Player != nil {
		receiverId = receiver.Player.GetId()
	}
	decisionIntent := PassIntent_SET_PIECE
	if e.state == MatchState_PLAYING {
		decisionIntent = option.Intent
	}
	e.lastPassDecision = PassDecision{
		PasserId:              passerId,
		ReceiverId:            receiverId,
		TargetPoint:           target,
		Intent:                decisionIntent,
		Utility:               option.Utility,
		Progression:           option.Progression,
		LaneRisk:              option.LaneRisk,
		CompletionProbability: option.CompletionProbability,
	}
	// Lateral execution error (metres) grows with poor technique, pressure,
	// distance, fatigue and away nerves. Passes back to the own keeper are
	// played safely.
	direct := metricDirection(passer.Position, target)
	care := 1.0
	if receiver.IsGoalkeeper {
		care = Passing_BACK_PASS_ERROR_SCALE
	}
	error :=
		((1.0-passer.Passing)*Passing_TECHNICAL_ERROR_METRES +
			pressure*Passing_PRESSURE_ERROR_METRES +
			passDistance*Passing_DISTANCE_ERROR) *
			e.executionErrorScale(passer) * care * e.randomFloat(-1.0, 1.0)
	miss := toPitch(Vector2F{-direct.Y * error, direct.X * error})
	target.X = hClamp(target.X+miss.X, 0.0, 1.0)
	target.Y = hClamp(target.Y+miss.Y, 0.0, 1.0)
	travel := distance(passer.Position, target)
	delivery := option.Intent == PassIntent_CROSS ||
		e.state == MatchState_CORNER_KICK ||
		(e.state == MatchState_FREE_KICK && lofted)
	if lofted {
		// Lofted balls are launched so they drop at the target: long passes at
		// chest height to be controlled, crosses at head height to be attacked.
		var arrivalMetres float64
		if delivery {
			arrivalMetres = Aerial_ARRIVAL_HEIGHT_METRES +
				e.randomFloat(-Passing_DELIVERY_HEIGHT_SPREAD,
					Passing_DELIVERY_HEIGHT_SPREAD)
		} else {
			arrivalMetres = Passing_LOFTED_ARRIVAL_HEIGHT_METRES
		}
		speed :=
			math.Min(Passing_LOFTED_BASE_SPEED+
				travel*Passing_LOFTED_SPEED_PER_METRE,
				Passing_MAX_LOFTED_SPEED)
		e.launchBall(passer, e.ball.Position, target, speed,
			loftVerticalSpeed(travel, speed, 0.0, arrivalMetres), 0.0)
	} else {
		// Ground passes are weighted to arrive at a controllable pace; longer
		// passes and better passers zip the ball in harder.
		arrivalSpeed :=
			Passing_ARRIVAL_SPEED_BASE +
				passer.Passing*Passing_ARRIVAL_SPEED_PASSING +
				travel*Passing_ARRIVAL_SPEED_PER_METRE
		speed := hClamp(groundLaunchSpeed(travel, arrivalSpeed),
			Passing_MIN_GROUND_SPEED,
			Passing_MAX_GROUND_SPEED)
		curve := e.randomFloat(-Passing_MAX_CURVE,
			Passing_MAX_CURVE) *
			(Passing_CURVE_SKILL_BASE + passer.Passing)
		if e.cfg.Modules.M2Decision {
			// FE M2 races a straight roll; see feM2CurveShare.
			curve *= e.cfg.M2.CurveShare
		}
		e.launchBall(passer, e.ball.Position, target, speed, 0.0, curve)
	}
	e.ball.IsAerialDelivery = lofted && delivery
	e.ball.FromThrowIn = e.state == MatchState_THROW_IN
	if passer.IsGoalkeeper {
		control := &e.keepers[hBoolIndex(passer.IsHomeTeam)]
		control.State = GoalkeeperState_DISTRIBUTE
		control.Timer = 0.0
	}

	// Keep externally visible AI state consistent with the newly released
	// pass. Without this, the final fixed step of a rendered frame could expose
	// the pre-pass run assignments until the next simulation update.
	for i := range e.players {
		teammate := &e.players[i]
		if teammate.IsHomeTeam != passer.IsHomeTeam || !active(teammate) {
			continue
		}
		teammate.IsMakingRun = false
		if teammate == receiver {
			teammate.Intent = PlayerIntent_RECEIVE_PASS
		} else if !teammate.IsGoalkeeper {
			teammate.Intent = PlayerIntent_OFFER_SUPPORT
		}
	}

	if passer.IsHomeTeam {
		e.stats.HomePassesAttempted++
	} else {
		e.stats.AwayPassesAttempted++
	}
	e.statsOf(passer).PassesAttempted++
	e.trackPassRelease(passer)
	if e.state == MatchState_PLAYING {
		progressivePasses := &e.stats.AwayProgressivePasses
		throughBalls := &e.stats.AwayThroughBalls
		crosses := &e.stats.AwayCrosses
		cutbacks := &e.stats.AwayCutbacks
		switchesOfPlay := &e.stats.AwaySwitchesOfPlay
		if passer.IsHomeTeam {
			progressivePasses = &e.stats.HomeProgressivePasses
			throughBalls = &e.stats.HomeThroughBalls
			crosses = &e.stats.HomeCrosses
			cutbacks = &e.stats.HomeCutbacks
			switchesOfPlay = &e.stats.HomeSwitchesOfPlay
		}
		if option.Intent == PassIntent_PROGRESSIVE {
			*progressivePasses++
		} else if option.Intent == PassIntent_THROUGH_BALL {
			*throughBalls++
		} else if option.Intent == PassIntent_CROSS {
			*crosses++
		} else if option.Intent == PassIntent_CUTBACK {
			*cutbacks++
		} else if option.Intent == PassIntent_SWITCH_PLAY {
			*switchesOfPlay++
		}
	}
	passer.ActionCooldown =
		e.randomFloat(Passing_MIN_ACTION_COOLDOWN,
			Passing_MAX_ACTION_COOLDOWN)
}

func (e *MatchEngine) strikeShot(shooter *MatchPlayer, forcedXG float64, header bool, penalty bool, freeKick bool) float64 {
	goalX := 0.0
	if shooter.IsHomeTeam {
		goalX = 1.0
	}
	metres :=
		distance(shooter.Position, Vector2F{goalX, Pitch_CENTRE})
	xg := forcedXG
	if forcedXG < 0.0 {
		xg = e.estimateShotXG(shooter)
	}
	if header && forcedXG < 0.0 {
		xg *= Shooting_HEADER_XG_FACTOR
	}
	maxXG := Shooting_MAX_OPEN_PLAY_XG
	if forcedXG >= 0.0 {
		maxXG = Shooting_MAX_SET_PIECE_XG
	}
	xg = hClamp(xg, Shooting_MIN_GOAL_PROBABILITY, maxXG)

	keeper := e.findGoalkeeper(!shooter.IsHomeTeam)
	pressure := hClamp((Shooting_PRESSURE_RADIUS_METRES-
		e.nearestOpponentDistance(shooter))/
		Shooting_PRESSURE_RADIUS_METRES,
		0.0, 1.0)

	// Aim for the side away from the keeper, a little inside the post; better
	// finishers pick tighter spots. The execution error then grows with
	// distance, pressure, weak technique, fatigue and headers.
	const HALF_GOAL = Shooting_GOAL_HALF_WIDTH_METRES
	keeperOffset := 0.0
	if keeper != nil {
		keeperOffset = (keeper.Position.Y - Pitch_CENTRE) *
			Pitch_WIDTH_METRES
	}
	side := 1.0
	if keeperOffset > 0.0 {
		side = -1.0
	}
	if math.Abs(keeperOffset) < Shooting_KEEPER_CENTRED_METRES ||
		e.randomFloat(0.0, 1.0) < Shooting_NEAR_SIDE_AIM_CHANCE {
		if e.randomFloat(0.0, 1.0) < 0.5 {
			side = -1.0
		} else {
			side = 1.0
		}
	}
	// From the spot a taker can pick a corner; in open play the spot is
	// chosen on the move and lands further inside the post.
	// A free kick is curled over the wall into a corner.
	maximumInsetSpread := Shooting_AIM_INSET_SPREAD
	if penalty {
		maximumInsetSpread = Shooting_PENALTY_AIM_INSET_SPREAD
	} else if freeKick {
		maximumInsetSpread = SetPiece_FREE_KICK_AIM_INSET_SPREAD
	}
	inset :=
		Shooting_AIM_POST_INSET_BASE +
			(1.0-shooter.Shooting)*Shooting_AIM_POST_INSET_SKILL +
			e.randomFloat(0.0, maximumInsetSpread)
	aimY := side * math.Max(0.0, HALF_GOAL-inset)
	if !penalty && !freeKick && !header &&
		e.randomFloat(0.0, 1.0) <
			Shooting_POWER_SHOT_BASE_CHANCE-
				shooter.Shooting*
					Shooting_POWER_SHOT_SKILL_REDUCTION {
		// Struck hard at the frame rather than placed: often near the keeper.
		aimY = keeperOffset +
			e.randomFloat(-Shooting_POWER_SHOT_WIDTH_METRES,
				Shooting_POWER_SHOT_WIDTH_METRES)
	}
	var aimZ float64
	if freeKick {
		aimZ = e.randomFloat(SetPiece_FREE_KICK_AIM_MIN_HEIGHT_METRES,
			Shooting_AIM_MAX_HEIGHT_METRES)
	} else {
		maxHeight := Shooting_AIM_MAX_HEIGHT_METRES
		if header {
			maxHeight = Shooting_HEADER_AIM_MAX_HEIGHT_METRES
		}
		aimZ =
			e.randomFloat(Shooting_AIM_MIN_HEIGHT_METRES, maxHeight)
	}
	// Heading well is as much about strength and timing as about finishing.
	finishing := shooter.Shooting
	if header {
		finishing = shooter.Shooting*
			(1.0-Aerial_HEADING_PHYSICAL_SHARE) +
			shooter.Physicality*
				Aerial_HEADING_PHYSICAL_SHARE
	}
	spread :=
		(Shooting_ERROR_BASE_METRES +
			(1.0-finishing)*Shooting_ERROR_SKILL_METRES +
			pressure*Shooting_ERROR_PRESSURE_METRES) *
			math.Max(metres/Shooting_ERROR_REFERENCE_METRES,
				Shooting_ERROR_MIN_DISTANCE_SCALE) *
			e.executionErrorScale(shooter)
	if header {
		spread *= Aerial_HEADER_ACCURACY_PENALTY
	}
	if penalty {
		spread *= Shooting_PENALTY_ERROR_SCALE
	} else {
		spread /= e.finishingPrecision * e.levelPrecision
	}
	if freeKick {
		spread *= SetPiece_FREE_KICK_ERROR_SCALE
	}
	// Play mode: the human's aim and power (no effect on AI shots).
	if e.controlledShot.Active {
		e.shapeControlledShot(shooter, HALF_GOAL, inset, &aimY, &aimZ, &spread)
	}
	crossingY := aimY + gaussian(&e.rng)*spread
	crossingZ :=
		hClamp(aimZ+gaussian(&e.rng)*spread*
			Shooting_VERTICAL_ERROR_SHARE,
			Shooting_MIN_CROSSING_HEIGHT_METRES,
			Shooting_MAX_CROSSING_HEIGHT_METRES)
	onTarget :=
		math.Abs(crossingY) <
			HALF_GOAL-Units_BALL_RADIUS_METRES &&
			crossingZ < Units_CROSSBAR_HEIGHT_METRES-
				Units_BALL_RADIUS_METRES
	targetY :=
		Pitch_CENTRE + crossingY/Pitch_WIDTH_METRES

	headerScale := 1.0
	if header {
		headerScale = Aerial_HEADER_SPEED_SCALE
	}
	controlScale := 1.0
	if e.controlledShot.Active {
		controlScale = Control_SHOT_SPEED_MIN_SCALE +
			Control_SHOT_SPEED_POWER_GAIN*
				e.controlledShot.Power
	}
	speed :=
		(Shooting_BASE_BALL_SPEED +
			shooter.Shooting*Shooting_SHOOTING_SPEED_BONUS +
			shooter.Physicality*Shooting_POWER_SPEED_BONUS) *
			e.randomFloat(Shooting_MIN_SPEED_VARIATION, 1.0) *
			headerScale *
			controlScale
	goalPoint := Vector2F{goalX, targetY}
	startHeight := Shooting_RELEASE_HEIGHT_METRES
	if header {
		startHeight = e.ball.Z * Units_BALL_Z_METRES
	}
	goalDistance := distance(shooter.Position, goalPoint)
	e.clearFlightState()
	e.launchBall(shooter, e.ball.Position, goalPoint, speed,
		loftVerticalSpeed(goalDistance, speed, startHeight, crossingZ),
		0.0)
	e.ball.Z = startHeight / Units_BALL_Z_METRES
	e.ball.IsShot = true
	e.ball.ShotByHome = shooter.IsHomeTeam
	e.ball.ShotOnTarget = onTarget
	e.ball.ShotXG = xg
	e.ball.ShotTargetY = targetY
	e.ball.ShotTargetHeightMetres = crossingZ
	e.ball.ShotElapsedSeconds = 0.0
	e.ball.ShotIsHeader = header
	e.ball.ShotIsPenalty = penalty
	e.ball.ShotSaveResolved = false
	e.lastShooter = shooter.Player
	return xg
}

func (e *MatchEngine) takeShot(shooter *MatchPlayer, forcedXG float64, header bool, freeKick bool) {
	penalty := e.state == MatchState_PENALTY
	xg := e.strikeShot(shooter, forcedXG, header, penalty, freeKick)

	shooterStats := e.statsOf(shooter)
	shooterStats.Shots++
	shooterStats.ExpectedGoals += xg
	insideBox := inPenaltyArea(shooter.Position, !shooter.IsHomeTeam)
	setPiece :=
		penalty || e.restartIsSetPiece ||
			(e.setPiecePhaseRemaining > 0.0 && shooter.IsHomeTeam == e.setPieceHome)
	e.ball.ShotFromSetPiece = setPiece
	if shooter.IsHomeTeam {
		e.stats.HomeShots++
		e.stats.HomeShotXG += xg
		if insideBox {
			e.stats.HomeShotsInsideBox++
		}
		if header {
			e.stats.HomeHeadedShots++
		}
		if setPiece {
			e.stats.HomeSetPieceShots++
		}
	} else {
		e.stats.AwayShots++
		e.stats.AwayShotXG += xg
		if insideBox {
			e.stats.AwayShotsInsideBox++
		}
		if header {
			e.stats.AwayHeadedShots++
		}
		if setPiece {
			e.stats.AwaySetPieceShots++
		}
	}

	// Credit the pass that created the chance.
	if !header {
		if e.lastCompletedReceiver == shooter.Player {
			e.shotAssistCandidate = e.lastCompletedPasser
		} else {
			e.shotAssistCandidate = nil
		}
	}
	if creator := e.findMatchPlayer(e.shotAssistCandidate); creator != nil && creator.IsHomeTeam == shooter.IsHomeTeam {
		e.statsOf(creator).KeyPasses++
	}
	e.trackShot(shooter, xg, setPiece, header, penalty)
	e.auditShot(shooter, xg, header, setPiece, penalty, freeKick)

	event := e.logEventActor(MatchEventType_SHOT, shooter)
	if header {
		event.Detail = MatchEventDetail_HEADER
	}
	event.Xg = xg
	event.Position = shooter.Position
	shooter.ActionCooldown =
		e.randomFloat(Shooting_MIN_ACTION_COOLDOWN,
			Shooting_MAX_ACTION_COOLDOWN)
	e.planGoalkeeperDive(!shooter.IsHomeTeam, penalty)
}

func (e *MatchEngine) launchBall(kicker *MatchPlayer, origin Vector2F, target Vector2F, horizontalSpeed float64, verticalSpeed float64, curve float64) {
	direction := metricDirection(origin, target)
	e.ball.Position = origin
	e.ball.Z = 0.0
	e.ball.Velocity = Vector2F{direction.X * horizontalSpeed,
		direction.Y * horizontalSpeed}
	e.ball.VelocityZ = verticalSpeed
	e.ball.Curve = curve
	e.ball.PossessedBy = nil
	e.ball.LastPossessor = kicker.Player
	e.ball.Kicker = kicker.Player
	e.ball.KickerLockout = Passing_KICKER_LOCKOUT_SECONDS
	e.ball.TouchAttempts = 0
}

// groundLaunchSpeed's function-local static table (C++ lambda-initialised
// once on first use).
const (
	b4SPEED_STEP = 0.25
	b4ENTRIES    = 161
)

var (
	b4StoppingDistanceOnce sync.Once
	b4StoppingDistance     [b4ENTRIES]float64
)

func b4StoppingDistanceTable() *[b4ENTRIES]float64 {
	b4StoppingDistanceOnce.Do(func() {
		const DT = Timing_FIXED_STEP_SECONDS /
			float64(Timing_BALL_SUBSTEPS)
		for index := 1; index < b4ENTRIES; index++ {
			flight := FlightState{HorizontalSpeed: float64(index) * b4SPEED_STEP,
				VerticalSpeed: 0.0, Height: 0.0, Bounced: false}
			travelled := 0.0
			for flight.HorizontalSpeed > 0.0 {
				travelled += stepFlight(&flight, DT)
			}
			b4StoppingDistance[index] = travelled
		}
	})
	return &b4StoppingDistance
}

func groundLaunchSpeed(distanceMetres float64, arrivalSpeed float64) float64 {
	// Rolling is one-dimensional and deterministic, so the distance a ball
	// rolls before stopping, D(v), is tabulated once with the live physics.
	// A ball launched at v0 arrives at va after D(v0) - D(va) metres.
	const SPEED_STEP = b4SPEED_STEP
	const ENTRIES = b4ENTRIES
	stoppingDistance := b4StoppingDistanceTable()
	distanceFor := func(speed float64) float64 {
		position :=
			hClamp(speed/SPEED_STEP, 0.0, float64(ENTRIES-1))
		low := int(position)
		high := min(low+1, ENTRIES-1)
		share := position - float64(low)
		return stoppingDistance[low] +
			(stoppingDistance[high]-stoppingDistance[low])*share
	}
	needed := distanceFor(arrivalSpeed) + distanceMetres
	// std::ranges::lower_bound: first entry not less than needed.
	index := sort.SearchFloat64s(stoppingDistance[:], needed)
	if index == ENTRIES {
		return float64(ENTRIES-1) * SPEED_STEP
	}
	if index == 0 {
		return 0.0
	}
	below := stoppingDistance[index-1]
	share :=
		(needed - below) / math.Max(stoppingDistance[index]-below, EPSILON)
	return (float64(index-1) + share) * SPEED_STEP
}

func flightHeightAtDistance(horizontalSpeed, verticalSpeed, startHeight, distanceMetres float64) float64 {
	const DT = Timing_FIXED_STEP_SECONDS /
		float64(Timing_BALL_SUBSTEPS)
	flight := FlightState{HorizontalSpeed: horizontalSpeed, VerticalSpeed: verticalSpeed,
		Height: startHeight, Bounced: false}
	travelled := 0.0
	for sample := 0; float64(sample)*DT <
		Ball_MAX_FLIGHT_SECONDS &&
		flight.HorizontalSpeed > 0.0; sample++ {
		previousHeight := flight.Height
		step := stepFlight(&flight, DT)
		if flight.Bounced {
			return -0.25 * math.Max(0.0, distanceMetres-travelled-step)
		}
		if travelled+step >= distanceMetres {
			share :=
				(distanceMetres - travelled) / math.Max(step, EPSILON)
			return previousHeight + (flight.Height-previousHeight)*share
		}
		travelled += step
	}
	return -1.0e6
}

func loftVerticalSpeed(distanceMetres float64, horizontalSpeed float64, startHeight float64, arrivalHeight float64) float64 {
	// Bisection on the vertical launch speed with the live flight physics: the
	// height reached over the target distance rises with the launch angle.
	// Probe from the drag-free estimate and its correction, step on (in
	// growing steps) until two probes bracket the arrival height, then close
	// in with the Illinois false-position method: a handful of flights, and
	// the lowest trajectory that gets there when two exist.
	const MAX_VERTICAL = 28.0
	const TOLERANCE_METRES = 0.03
	const MAX_STEP = 4.0
	const MAX_FLIGHTS = 14
	// Flight time under quadratic air drag at the launch speed's drag
	// coefficient: x(t) = ln(1 + k v t) / k.
	launchSpeed := math.Max(horizontalSpeed, 1.0)
	drag := FLIGHT_DRAG_SCALE * dragCoefficient(launchSpeed)
	flightTime :=
		math.Expm1(drag*distanceMetres) / (drag * launchSpeed)
	first := hClamp((arrivalHeight-startHeight)/flightTime+
		0.5*Ball_GRAVITY*flightTime,
		0.0, MAX_VERTICAL)
	firstMiss := flightHeightAtDistance(horizontalSpeed, first, startHeight,
		distanceMetres) -
		arrivalHeight
	if math.Abs(firstMiss) < TOLERANCE_METRES {
		return first
	}
	second := hClamp(first-firstMiss/flightTime, 0.0, MAX_VERTICAL)
	secondMiss := flightHeightAtDistance(horizontalSpeed, second,
		startHeight, distanceMetres) -
		arrivalHeight
	flights := 2
	for (firstMiss < 0.0) == (secondMiss < 0.0) {
		if math.Abs(secondMiss) < TOLERANCE_METRES {
			return second
		}
		step := hClamp((second-first)*1.5, -MAX_STEP, MAX_STEP)
		next := hClamp(second+step, 0.0, MAX_VERTICAL)
		// Pinned at a bound (or not moving): nothing closer is reachable.
		if next == second || flights >= MAX_FLIGHTS {
			return second
		}
		first = second
		firstMiss = secondMiss
		second = next
		secondMiss = flightHeightAtDistance(horizontalSpeed, second, startHeight,
			distanceMetres) -
			arrivalHeight
		flights++
	}
	below := second
	belowMiss := secondMiss
	above := first
	aboveMiss := firstMiss
	if firstMiss < 0.0 {
		below = first
		belowMiss = firstMiss
		above = second
		aboveMiss = secondMiss
	}
	if math.Abs(aboveMiss) < TOLERANCE_METRES {
		return above
	}
	lastSide := 0
	estimate := above
	for ; flights < MAX_FLIGHTS; flights++ {
		estimate = below + (above-below)*belowMiss/(belowMiss-aboveMiss)
		miss := flightHeightAtDistance(horizontalSpeed, estimate,
			startHeight, distanceMetres) -
			arrivalHeight
		if math.Abs(miss) < TOLERANCE_METRES {
			return estimate
		}
		if miss < 0.0 {
			below = estimate
			belowMiss = miss
			if lastSide < 0 {
				aboveMiss *= 0.5
			}
			lastSide = -1
		} else {
			above = estimate
			aboveMiss = miss
			if lastSide > 0 {
				belowMiss *= 0.5
			}
			lastSide = 1
		}
	}
	return estimate
}
