// Ported from Player12 src/model/match_engine.cpp lines 3952–3964, 4343–4950 (GPL-3.0)

package engine

import "math"

// currentTopSpeed is the player's top speed now: fresh top speed less
// fatigue, an emptied sprint reserve and injury.
func (e *MatchEngine) currentTopSpeed(player *MatchPlayer) float64 {
	fatigue := hClamp((1.0-player.Stamina)*INVERSE_FATIGUE_RANGE, 0.0, 1.0)
	reserve := Player_EMPTY_RESERVE_TOP_SPEED +
		(1.0-Player_EMPTY_RESERVE_TOP_SPEED)*player.SprintReserve
	injury := 1.0
	if player.IsInjured {
		injury = Injury_INJURED_SPEED_SCALE
	}
	return player.MaxSpeed * (1.0 - Player_FATIGUE_TOP_SPEED_LOSS*fatigue) *
		reserve * injury
}

// b3Lanes is the C++ LANES (fixed mover arrays).
const b3Lanes = 32

func (e *MatchEngine) integrateMovements(slots []uint8, dt float64, walking bool) {
	// FE M3 steering (fe_m3_steering.go) for live play; restarts keep this path.
	if e.cfg.Modules.M3Steering && !walking {
		e.feM3IntegrateMovements(slots, dt)
		return
	}
	const PI = math.Pi
	// The movers are gathered into arrays and advanced with straight-line code
	// (every branch of stepKinematics() evaluated and selected per player). The
	// arithmetic of each player is exactly that of stepKinematics().
	var offsetX, offsetY, velocityX, velocityY, positionX, positionY [b3Lanes]float64
	var facing, nextFacing, nextHeading [b3Lanes]float64
	var turning [b3Lanes]int32
	var maxSpeed, stamina, reserve, injury, acceleration, braking [b3Lanes]float64
	var intentScale, talk, shout, turnRate [b3Lanes]float64
	var movers [b3Lanes]*MatchPlayer
	moving := 0
	for index := 0; index < len(slots); index++ {
		player := &e.players[slots[index]]
		target := player.TacticalTarget
		if walking {
			target = player.MovementTarget
		}
		target.X = hClamp(target.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X)
		target.Y = hClamp(target.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)
		// Players react to a moving target with a perception/intent lag; a side
		// unfamiliar with its tactics reacts later.
		urgent := !walking && player.UrgentMovement
		blendIndex := 2
		if player.IsHomeTeam {
			blendIndex = 0
		}
		if urgent {
			blendIndex++
		}
		targetBlend := e.targetBlends[blendIndex]
		player.MovementTarget.X += (target.X - player.MovementTarget.X) * targetBlend
		player.MovementTarget.Y += (target.Y - player.MovementTarget.Y) * targetBlend
		offset := toMetres(Vector2F{player.MovementTarget.X - player.Position.X,
			player.MovementTarget.Y - player.Position.Y})
		// A player standing on his spot stays put (the common case in stoppages).
		if offset.X*offset.X+offset.Y*offset.Y <=
			Player_ARRIVAL_DEAD_ZONE_METRES*Player_ARRIVAL_DEAD_ZONE_METRES &&
			player.Velocity.X == 0.0 && player.Velocity.Y == 0.0 {
			continue
		}
		lane := moving
		moving++
		movers[lane] = player
		offsetX[lane] = offset.X
		offsetY[lane] = offset.Y
		velocityX[lane] = player.Velocity.X
		velocityY[lane] = player.Velocity.Y
		positionX[lane] = player.Position.X
		positionY[lane] = player.Position.Y
		facing[lane] = player.FacingAngle
		maxSpeed[lane] = player.MaxSpeed
		stamina[lane] = player.Stamina
		reserve[lane] = player.SprintReserve
		injury[lane] = 1.0
		if player.IsInjured {
			injury[lane] = Injury_INJURED_SPEED_SCALE
		}
		acceleration[lane] = player.Acceleration
		braking[lane] = player.Braking
		if walking {
			intentScale[lane] = Player_RESTART_WALK_SPEED_SCALE
			if e.cfg.Modules.M3Steering {
				// FE M3 (ours): personal walking pace at restarts.
				intentScale[lane] *= e.feM3WalkScale(slots[index], player)
			}
		} else {
			intentScale[lane] = movementSpeedScale(player.Intent)
		}
		talk[lane] = e.talkOf(player.IsHomeTeam)
		shout[lane] = e.shoutWorkRate(player.IsHomeTeam)
		turnRate[lane] = player.TurnRate
	}

	for lane := 0; lane < moving; lane++ {
		targetX := offsetX[lane]
		targetY := offsetY[lane]
		targetDistance := math.Sqrt(targetX*targetX + targetY*targetY)
		fatigue := hClamp((1.0-stamina[lane])*INVERSE_FATIGUE_RANGE, 0.0, 1.0)
		topSpeed := maxSpeed[lane] * (1.0 - Player_FATIGUE_TOP_SPEED_LOSS*fatigue) *
			(Player_EMPTY_RESERVE_TOP_SPEED +
				(1.0-Player_EMPTY_RESERVE_TOP_SPEED)*reserve[lane]) *
			injury[lane]
		// Players jog while in position but run hard to recover a lost
		// position. Intent speeds are shares of the fresh top speed; tired
		// players choose to run a little less hard and lose the top end.
		urgency := intentScale[lane]
		if !walking {
			urgency = math.Max(intentScale[lane],
				math.Min(Player_MAX_URGENCY_SPEED_SCALE, targetDistance/Player_URGENCY_DISTANCE_METRES))
		}
		speedCap := math.Min(topSpeed, maxSpeed[lane]*
			(1.0-Player_FATIGUE_WORK_RATE_LOSS*fatigue+talk[lane]+shout[lane])*
			urgency)
		// Arrive without overshooting: the desired speed falls off so a
		// comfortable deceleration stops the player on the target.
		approach := targetDistance - Player_ARRIVAL_DEAD_ZONE_METRES
		arrivalSpeed := math.Sqrt(2.0 * Player_ARRIVAL_DECELERATION * math.Max(approach, 0.0))
		desiredSpeed := 0.0
		if approach > 0.0 {
			desiredSpeed = math.Min(speedCap, arrivalSpeed)
		}
		aimed := targetDistance > EPSILON
		desiredX, desiredY := 0.0, 0.0
		if aimed {
			desiredX = targetX / targetDistance * desiredSpeed
			desiredY = targetY / targetDistance * desiredSpeed
		}

		// Acceleration-speed profile a(v) = A0 (1 - v / vmax), stronger
		// braking, and a lateral limit that forces a player to slow down to
		// turn sharply.
		push := acceleration[lane] * (1.0 - Player_FATIGUE_ACCELERATION_LOSS*fatigue)
		oldX := velocityX[lane]
		oldY := velocityY[lane]
		speed := math.Sqrt(oldX*oldX + oldY*oldY)
		changeX := desiredX - oldX
		changeY := desiredY - oldY
		brake := braking[lane]
		drive := push * math.Max(0.05, 1.0-speed/topSpeed)
		var nextVelocityX, nextVelocityY float64
		if speed < 0.5 {
			// Starting from (almost) a standstill: move straight toward the
			// target.
			limit := brake * dt
			if desiredSpeed >= speed {
				limit = drive * dt
			}
			magnitude := math.Sqrt(changeX*changeX + changeY*changeY)
			scale := 1.0
			if magnitude > limit {
				scale = limit / magnitude
			}
			nextVelocityX = oldX + changeX*scale
			nextVelocityY = oldY + changeY*scale
		} else {
			// On the move: accelerate or brake along the run, turn within the
			// limit.
			tangentX := oldX / speed
			tangentY := oldY / speed
			normalX := -tangentY
			normalY := tangentX
			along := changeX*tangentX + changeY*tangentY
			across := changeX*normalX + changeY*normalY
			if along > 0.0 {
				along = math.Min(along, drive*dt)
			} else {
				along = math.Max(along, -brake*dt)
			}
			speedShare := hClamp(speed/maxSpeed[lane], 0.0, 1.0)
			lateral := hLerp(Player_LATERAL_ACCELERATION_SLOW,
				Player_LATERAL_ACCELERATION_FAST, speedShare) * dt
			across = hClamp(across, -lateral, lateral)
			nextVelocityX = oldX + (tangentX*along + normalX*across)
			nextVelocityY = oldY + (tangentY*along + normalY*across)
		}
		if desiredSpeed == 0.0 && speed <= brake*dt {
			nextVelocityX = 0.0
			nextVelocityY = 0.0
		}
		squaredSpeed := nextVelocityX*nextVelocityX + nextVelocityY*nextVelocityY
		if squaredSpeed > topSpeed*topSpeed {
			capScale := topSpeed / math.Sqrt(squaredSpeed)
			nextVelocityX *= capScale
			nextVelocityY *= capScale
		}

		nextX := positionX[lane] + nextVelocityX*dt*INVERSE_LENGTH
		nextY := positionY[lane] + nextVelocityY*dt*INVERSE_WIDTH
		clampedX := hClamp(nextX, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X)
		clampedY := hClamp(nextY, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)
		if clampedX != nextX {
			nextVelocityX = 0.0
		}
		if clampedY != nextY {
			nextVelocityY = 0.0
		}
		positionX[lane] = clampedX
		positionY[lane] = clampedY
		velocityX[lane] = nextVelocityX
		velocityY[lane] = nextVelocityY

		// Facing (normalised pitch space, for the renderers) turns toward the
		// run at a limited rate.
		angle := fastAtan2(nextVelocityY*INVERSE_WIDTH, nextVelocityX*INVERSE_LENGTH)
		difference := angle - facing[lane]
		if difference > PI {
			difference -= TWO_PI
		} else if difference < -PI {
			difference += TWO_PI
		}
		turn := turnRate[lane] * dt
		turned := facing[lane] + hClamp(difference, -turn, turn)
		if turned > PI {
			turned -= TWO_PI
		} else if turned < -PI {
			turned += TWO_PI
		}
		// squaredSpeed is the speed before the boundary clamp, as in C++.
		running := squaredSpeed > Player_MOVEMENT_FACING_THRESHOLD*Player_MOVEMENT_FACING_THRESHOLD
		nextHeading[lane] = angle
		if running {
			turning[lane] = 1
			nextFacing[lane] = turned
		} else {
			turning[lane] = 0
			nextFacing[lane] = facing[lane]
		}
	}

	for lane := 0; lane < moving; lane++ {
		player := movers[lane]
		player.Velocity = Vector2F{velocityX[lane], velocityY[lane]}
		player.Position = Vector2F{positionX[lane], positionY[lane]}
		player.FacingAngle = nextFacing[lane]
		if turning[lane] != 0 {
			player.TargetAngle = nextHeading[lane]
		}
	}
}

func (e *MatchEngine) stepKinematics(player *MatchPlayer, desired Vector2F, desiredSpeed float64, topSpeed float64, fatigue float64, dt float64) {
	// Acceleration-speed profile a(v) = A0 (1 - v / vmax), stronger braking,
	// and a lateral limit that forces a player to slow down to turn sharply.
	acceleration := player.Acceleration * (1.0 - Player_FATIGUE_ACCELERATION_LOSS*fatigue)
	velocity := player.Velocity
	speed := length(velocity)
	change := Vector2F{desired.X - velocity.X, desired.Y - velocity.Y}
	if desiredSpeed == 0.0 && speed <= player.Braking*dt {
		velocity = Vector2F{0.0, 0.0}
	} else if speed < 0.5 {
		limit := player.Braking * dt
		if desiredSpeed >= speed {
			limit = acceleration * math.Max(0.05, 1.0-speed/topSpeed) * dt
		}
		magnitude := length(change)
		scale := 1.0
		if magnitude > limit {
			scale = limit / magnitude
		}
		velocity.X += change.X * scale
		velocity.Y += change.Y * scale
	} else {
		tangent := Vector2F{velocity.X / speed, velocity.Y / speed}
		normal := Vector2F{-tangent.Y, tangent.X}
		along := change.X*tangent.X + change.Y*tangent.Y
		across := change.X*normal.X + change.Y*normal.Y
		if along > 0.0 {
			along = math.Min(along, acceleration*math.Max(0.05, 1.0-speed/topSpeed)*dt)
		} else {
			along = math.Max(along, -player.Braking*dt)
		}
		speedShare := hClamp(speed/player.MaxSpeed, 0.0, 1.0)
		lateral := hLerp(Player_LATERAL_ACCELERATION_SLOW,
			Player_LATERAL_ACCELERATION_FAST, speedShare) * dt
		across = hClamp(across, -lateral, lateral)
		velocity.X += tangent.X*along + normal.X*across
		velocity.Y += tangent.Y*along + normal.Y*across
	}
	squaredSpeed := velocity.X*velocity.X + velocity.Y*velocity.Y
	if squaredSpeed > topSpeed*topSpeed {
		scale := topSpeed / math.Sqrt(squaredSpeed)
		velocity.X *= scale
		velocity.Y *= scale
	}
	player.Velocity = velocity

	step := toPitch(Vector2F{velocity.X * dt, velocity.Y * dt})
	nextX := player.Position.X + step.X
	nextY := player.Position.Y + step.Y
	player.Position.X = hClamp(nextX, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X)
	player.Position.Y = hClamp(nextY, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)
	if player.Position.X != nextX {
		player.Velocity.X = 0.0
	}
	if player.Position.Y != nextY {
		player.Velocity.Y = 0.0
	}

	if squaredSpeed > Player_MOVEMENT_FACING_THRESHOLD*Player_MOVEMENT_FACING_THRESHOLD {
		// Facing is kept in normalised pitch space for the renderers.
		player.TargetAngle = fastAtan2(player.Velocity.Y*INVERSE_WIDTH,
			player.Velocity.X*INVERSE_LENGTH)
		angleDifference := player.TargetAngle - player.FacingAngle
		if angleDifference > math.Pi {
			angleDifference -= TWO_PI
		} else if angleDifference < -math.Pi {
			angleDifference += TWO_PI
		}
		player.FacingAngle += hClamp(angleDifference, -player.TurnRate*dt, player.TurnRate*dt)
		if player.FacingAngle > math.Pi {
			player.FacingAngle -= TWO_PI
		} else if player.FacingAngle < -math.Pi {
			player.FacingAngle += TWO_PI
		}
	}
}

func (e *MatchEngine) updateRestartSupport(player *MatchPlayer, taker *MatchPlayer) {
	// FE M9: ordinary free kicks get the same support/retreat shape; the side
	// taking a goal kick keeps the build-up shape planned at setup.
	freeKick := e.cfg.Modules.M9Restart && e.state == MatchState_FREE_KICK
	if player.IsGoalkeeper || player == taker || e.restartIsSetPiece ||
		(e.state != MatchState_THROW_IN && e.state != MatchState_GOAL_KICK && !freeKick) {
		return
	}
	if e.cfg.Modules.M9Restart && e.state == MatchState_GOAL_KICK && taker != nil &&
		player.IsHomeTeam == taker.IsHomeTeam {
		return
	}

	player.MovementTarget = player.BasePosition
	if taker != nil && player.IsHomeTeam == taker.IsHomeTeam {
		gap := distance(player.Position, e.ball.Position)
		if gap < Timing_RESTART_SUPPORT_DISTANCE_METRES {
			// Offer a lane inside the pitch, with teammates spread along it.
			player.MovementTarget.X = hClamp(
				e.ball.Position.X+(player.BasePosition.X-e.ball.Position.X)*Timing_RESTART_SUPPORT_BLEND,
				Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X)
			player.MovementTarget.Y = hClamp(
				e.ball.Position.Y+(player.BasePosition.Y-e.ball.Position.Y)*Timing_RESTART_SUPPORT_BLEND,
				Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)
		}
	} else if taker != nil {
		gap := distance(player.MovementTarget, e.ball.Position)
		minimum := Timing_KICK_OPPONENT_DISTANCE_METRES
		if e.state == MatchState_THROW_IN {
			minimum = Timing_THROW_OPPONENT_DISTANCE_METRES
		}
		if gap < minimum {
			direction := metricDirection(e.ball.Position, player.MovementTarget)
			offset := toPitch(Vector2F{direction.X * minimum, direction.Y * minimum})
			player.MovementTarget = Vector2F{e.ball.Position.X + offset.X,
				e.ball.Position.Y + offset.Y}
		}
	}
}

func (e *MatchEngine) updateRestartMovement(dt float64) {
	// During a stoppage players walk to their restart positions (set-piece
	// slots, the restart spot or their last tactical target).
	taker := e.restartTaker()
	// FE M9: targets stay legal every tick and the pace follows the distance.
	m9 := e.cfg.Modules.M9Restart && e.state != MatchState_GOAL
	if e.cfg.Modules.M9Restart && e.state == MatchState_GOAL {
		e.feM9GoalWalk()
	}
	var batch [32]uint8
	batched := 0
	for slot := 0; slot < len(e.players) && slot < len(batch); slot++ {
		player := &e.players[slot]
		if !active(player) {
			continue
		}
		if player.IsGoalkeeper && player != taker {
			e.keepers[hBoolIndex(player.IsHomeTeam)].State = GoalkeeperState_SET_POSITION
			if !e.restartIsSetPiece && e.state != MatchState_GOAL {
				player.MovementTarget = e.goalkeeperTarget(player, nil)
			}
		}
		e.updateRestartSupport(player, taker)
		if m9 {
			e.feM9LegalTarget(player, taker)
		}
		if player.IsGoalkeeper {
			player.Intent = PlayerIntent_GOALKEEP
		} else {
			player.Intent = PlayerIntent_RECOVER_SHAPE
		}
		player.TargetOwner = TargetOwner_P12
		player.TargetRefiner = TargetOwner_NONE
		player.TargetReason = TargetReason_RESTART
		if m9 {
			player.TargetOwner = TargetOwner_M9
			player.TargetRefiner = TargetOwner_NONE
		}
		batch[batched] = uint8(slot)
		batched++
	}
	if m9 {
		e.feM9Integrate(batch[:batched], dt)
	} else {
		e.integrateMovements(batch[:batched], dt, true)
	}
	e.separatePlayers()
}

func (e *MatchEngine) separatePlayers() {
	// Resolve body contact after movement: players cannot overlap. Players who
	// left the pitch are static obstacles. Pairs are swept along the length of
	// the pitch so only neighbours within a body width are examined.
	const MINIMUM = Player_MINIMUM_BODY_SEPARATION_METRES
	const MINIMUM_X = MINIMUM / Pitch_LENGTH_METRES
	// The order persists between steps: players move little per step, so the
	// insertion sort is nearly linear. Ties break on the slot index, so the
	// result does not depend on the previous order.
	order := &e.separationOrder
	count := min(len(e.players), len(order))
	// Nobody moved since a pass that found no contact: nothing to resolve.
	unchanged := e.separationClear && e.separationCount == count
	if unchanged {
		for index := 0; index < count; index++ {
			if e.separationPositions[index].X != e.players[index].Position.X ||
				e.separationPositions[index].Y != e.players[index].Position.Y {
				unchanged = false
				break
			}
		}
	}
	if unchanged {
		return
	}
	if e.separationCount != count {
		e.separationCount = count
		for index := 0; index < count; index++ {
			order[index] = uint8(index)
		}
	}
	var xs [32]float64
	for index := 0; index < count; index++ {
		xs[index] = e.players[index].Position.X
	}
	e.separationClear = true
	insertionSort(order[:], count, func(first, second uint8) bool {
		if xs[first] != xs[second] {
			return xs[first] < xs[second]
		}
		return first < second
	})
	push := func(player *MatchPlayer, offset Vector2F) {
		player.Position.X = hClamp(player.Position.X+offset.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X)
		player.Position.Y = hClamp(player.Position.Y+offset.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)
		player.Velocity.X *= Player_BODY_COLLISION_VELOCITY_RETAINED
		player.Velocity.Y *= Player_BODY_COLLISION_VELOCITY_RETAINED
		xs[e.slotOf(player)] = player.Position.X
	}
	for firstIndex := 0; firstIndex < count; firstIndex++ {
		first := &e.players[order[firstIndex]]
		for secondIndex := firstIndex + 1; secondIndex < count; secondIndex++ {
			if xs[order[secondIndex]]-xs[order[firstIndex]] >= MINIMUM_X {
				break
			}
			second := &e.players[order[secondIndex]]
			firstMoves := first.OnPitch
			secondMoves := second.OnPitch
			if !firstMoves && !secondMoves {
				continue
			}
			separationMetres := toMetres(Vector2F{second.Position.X - first.Position.X,
				second.Position.Y - first.Position.Y})
			squared := separationMetres.X*separationMetres.X + separationMetres.Y*separationMetres.Y
			if squared >= MINIMUM*MINIMUM {
				continue
			}
			separationLengthMetres := math.Sqrt(squared)
			if separationLengthMetres <= EPSILON {
				side := -1.0
				if order[firstIndex]%2 == 0 {
					side = 1.0
				}
				separationMetres = Vector2F{0.0, side}
				separationLengthMetres = 1.0
			}
			share := 1.0
			if firstMoves && secondMoves {
				share = Player_BODY_SEPARATION_SHARE
			}
			correctionMetres := (MINIMUM - separationLengthMetres) * share
			correction := toPitch(Vector2F{
				separationMetres.X / separationLengthMetres * correctionMetres,
				separationMetres.Y / separationLengthMetres * correctionMetres})
			if firstMoves {
				push(first, Vector2F{-correction.X, -correction.Y})
			}
			if secondMoves {
				push(second, correction)
			}
			e.separationClear = false
		}
	}
	for index := 0; index < count; index++ {
		e.separationPositions[index] = e.players[index].Position
	}
}

func (e *MatchEngine) accumulatePlayerLoad(dt float64) {
	// Measure actual travelled distance, then update the slow condition pool,
	// the fast sprint reserve and report totals together. Match-clock minutes
	// can be accelerated in Play, but physical exertion uses simulated
	// seconds. The final pass rejects restart teleports before applying load
	// or fatigue.
	clockScale := e.matchClockRate()
	clockDelta := dt * clockScale / Timing_SECONDS_PER_MINUTE
	for i := range e.players {
		player := &e.players[i]
		if active(player) {
			e.statsOf(player).MinutesPlayed += clockDelta
		}
	}
	// Distance, intensity and energy are accounted over two steps at a time.
	e.loadSeconds += dt
	if len(e.loadAnchors) != len(e.players) {
		e.loadAnchors = make([]Vector2F, len(e.players))
		for index := range e.players {
			if index < len(e.previousPlayerPositions) {
				e.loadAnchors[index] = e.previousPlayerPositions[index]
			} else {
				e.loadAnchors[index] = e.players[index].Position
			}
		}
	}
	if !e.periodElapsed(Timing_TACTICAL_REFRESH_STEPS) {
		return
	}
	dt = e.loadSeconds
	e.loadSeconds = 0.0
	// Gathered into arrays (as the C++ vectorisable layout); the statistics
	// are then updated per player.
	var fromX, fromY, toX, toY, topSpeed, endurance, pressingEffort, sideDrain [b3Lanes]float64
	var stamina, reserve, stepMetres, nextStamina, nextReserve [b3Lanes]float64
	var slots [b3Lanes]uint8
	count := 0
	for index := 0; index < len(e.players) && index < b3Lanes; index++ {
		player := &e.players[index]
		previous := e.loadAnchors[index]
		e.loadAnchors[index] = player.Position
		if !active(player) {
			continue
		}
		// A pressing side pays for it: the players hunting the ball most, and
		// the whole side while out of possession.
		pressing := e.getEffectiveSliders(player.IsHomeTeam).Pressing
		outOfPossession := e.lastControlledTeamHome.HasValue &&
			e.lastControlledTeamHome.Value != player.IsHomeTeam
		lane := count
		count++
		slots[lane] = uint8(index)
		fromX[lane] = previous.X
		fromY[lane] = previous.Y
		toX[lane] = player.Position.X
		toY[lane] = player.Position.Y
		topSpeed[lane] = player.MaxSpeed
		endurance[lane] = player.Endurance
		pressingEffort[lane] = 0.0
		if player.IsPressing {
			pressingEffort[lane] = pressing
		}
		sideDrain[lane] = 0.0
		if outOfPossession {
			sideDrain[lane] = math.Max(0.0, pressing-Fatigue_TEAM_PRESSING_NEUTRAL) *
				Fatigue_TEAM_PRESSING_DRAIN
		}
		stamina[lane] = player.Stamina
		reserve[lane] = player.SprintReserve
	}
	for lane := 0; lane < count; lane++ {
		metresX := (fromX[lane] - toX[lane]) * PITCH_LENGTH
		metresY := (fromY[lane] - toY[lane]) * PITCH_WIDTH
		metres := math.Sqrt(metresX*metresX + metresY*metresY)
		speed := metres / dt
		drain := MatchRules_staminaDrainPerSecond(
			speed/math.Max(topSpeed[lane], EPSILON),
			endurance[lane], pressingEffort[lane]) + sideDrain[lane]
		stepMetres[lane] = metres
		nextStamina[lane] = hClamp(stamina[lane]-drain*dt, Player_MINIMUM_STAMINA, 1.0)
		// Repeat-sprint reserve: emptied by high-intensity running, refilled
		// while jogging or walking.
		intensity := hClamp(
			(speed-Load_HIGH_INTENSITY_SPEED)/
				math.Max(topSpeed[lane]-Load_HIGH_INTENSITY_SPEED, EPSILON),
			0.0, 1.0)
		if speed > Load_HIGH_INTENSITY_SPEED {
			nextReserve[lane] = math.Max(0.0, reserve[lane]-
				Fatigue_RESERVE_DRAIN_PER_SECOND*intensity*intensity*dt)
		} else {
			nextReserve[lane] = reserve[lane] + (1.0-reserve[lane])*dt/
				Fatigue_RESERVE_RECOVERY_SECONDS
		}
	}
	for lane := 0; lane < count; lane++ {
		player := &e.players[slots[lane]]
		metres := stepMetres[lane]
		// Restart repositioning teleports are not running.
		if metres > Player_MAX_COUNTED_STEP_SPEED*dt {
			continue
		}
		speed := metres / dt
		entry := e.statsOf(player)
		entry.DistanceMetres += metres
		if e.period == 2 {
			entry.SecondHalfDistanceMetres += metres
		}
		// Running speed, not the displacement from body contact.
		if speed > entry.TopSpeed {
			entry.TopSpeed = math.Max(entry.TopSpeed, math.Min(speed, length(player.Velocity)))
		}
		if speed >= Load_HIGH_INTENSITY_SPEED {
			entry.HighIntensityMetres += metres
			if e.period == 2 {
				entry.SecondHalfHighIntensityMetres += metres
			}
		}
		if speed >= Load_SPRINT_SPEED {
			entry.SprintMetres += metres
			if !player.IsSprinting {
				entry.Sprints++
			}
			player.IsSprinting = true
		} else if speed < Load_SPRINT_EXIT_SPEED {
			player.IsSprinting = false
		}
		player.Stamina = nextStamina[lane]
		player.SprintReserve = nextReserve[lane]
	}
}
