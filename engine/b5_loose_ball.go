// Ported from Player12 src/model/match_engine.cpp lines 6977–7631 (GPL-3.0)

package engine

import "math"

func (e *MatchEngine) resolveLooseBall() {
	if e.ball.PossessedBy != nil {
		return
	}

	endHeight := e.ball.Z * Units_BALL_Z_METRES
	if !e.ball.IsShot && endHeight > Aerial_CONTROL_CEILING_METRES {
		e.resolveAerialContest()
		return
	}

	// Swept contact over the last sub-step: every player whose reach the ball
	// passed through is a candidate, taken in the order the ball met them.
	start := toMetres(e.substepBallPosition)
	end := toMetres(e.ball.Position)
	segment := Vector2F{end.X - start.X, end.Y - start.Y}
	segmentSquared := segment.X*segment.X + segment.Y*segment.Y
	startHeight := e.substepBallZ * Units_BALL_Z_METRES
	ballSpeed := length(e.ball.Velocity)
	// A ball that has slowed down can be collected by anyone again, including
	// players it flew past earlier.
	if !e.ball.IsShot && ballSpeed <= Ball_MAX_HEAVY_TOUCH_SPEED {
		e.ball.TouchAttempts = 0
	}
	// Handling is allowed in the own area, except for a team-mate's pass.
	handsAllowed := func(player *MatchPlayer) bool {
		return player.IsGoalkeeper &&
			inPenaltyArea(player.Position, player.IsHomeTeam) &&
			!(e.ball.IsPass && e.ball.PassByHome == player.IsHomeTeam)
	}
	type Contact struct {
		slot   int
		along  float64
		gap    float64
		height float64
		reach  float64
	}
	var contacts [32]Contact
	contactCount := 0
	// Nobody further than the largest reach from the segment's bounding box can
	// touch the ball; the margin keeps the exact test below authoritative.
	const REACH_BOUND = Ball_GOALKEEPER_CONTROL_RADIUS_METRES + 0.01
	boundMinX := math.Min(start.X, end.X) - REACH_BOUND
	boundMaxX := math.Max(start.X, end.X) + REACH_BOUND
	boundMinY := math.Min(start.Y, end.Y) - REACH_BOUND
	boundMaxY := math.Max(start.Y, end.Y) + REACH_BOUND
	for slot := 0; slot < len(e.players) && slot < len(contacts); slot++ {
		player := &e.players[slot]
		position := toMetres(player.Position)
		if position.X < boundMinX || position.X > boundMaxX ||
			position.Y < boundMinY || position.Y > boundMaxY {
			continue
		}
		if !active(player) || player.TrapTimer > 0.0 {
			continue
		}
		if e.ball.KickerLockout > 0.0 && e.ball.Kicker == player.Player {
			continue
		}
		if (e.ball.TouchAttempts & (uint32(1) << slot)) != 0 {
			continue
		}
		// Team-mates let a pass run to its receiver, and never block their own
		// side's shot.
		if e.ball.IsPass && player.IsHomeTeam == e.ball.PassByHome &&
			player.Player != e.ball.IntendedReceiver &&
			ballSpeed > Ball_MAX_HEAVY_TOUCH_SPEED {
			continue
		}
		if e.ball.IsShot && player.IsHomeTeam == e.ball.ShotByHome {
			continue
		}

		along := 0.0
		if segmentSquared > EPSILON {
			along = hClamp(((position.X-start.X)*segment.X+(position.Y-start.Y)*segment.Y)/segmentSquared,
				0.0, 1.0)
		}
		gapX := position.X - (start.X + segment.X*along)
		gapY := position.Y - (start.Y + segment.Y*along)
		gapSquared := gapX*gapX + gapY*gapY
		// Cheap reject beyond the largest reach before any square root.
		if gapSquared > Ball_GOALKEEPER_CONTROL_RADIUS_METRES*Ball_GOALKEEPER_CONTROL_RADIUS_METRES {
			continue
		}
		gap := math.Sqrt(gapSquared)
		height := hLerp(startHeight, endHeight, along)
		hands := handsAllowed(player)
		reach := 0.0
		if e.ball.IsShot {
			var reachable bool
			if hands {
				reachable = height <= MatchRules_goalkeeperReachMetres(player.HeightMetres, player.Goalkeeping)
			} else {
				reachable = height <= MatchRules_headerReachMetres(player.HeightMetres, player.Physicality)
			}
			if reachable {
				if hands {
					reach = MatchRules_goalkeeperHandlingRadiusMetres(
						player.HeightMetres, player.Goalkeeping, height, player.IsDiving)
				} else {
					reach = e.cfg.M6.BlockReach
				}
			}
		} else if hands {
			reach = MatchRules_goalkeeperHandlingRadiusMetres(
				player.HeightMetres, player.Goalkeeping, height, player.IsDiving)
		} else {
			if height <= Ball_LOW_BALL_METRES {
				reach = Ball_OUTFIELD_CONTROL_RADIUS_METRES
			} else {
				reach = Ball_CHEST_CONTROL_RADIUS_METRES
			}
		}
		if reach <= 0.0 || gap > reach {
			continue
		}
		contacts[contactCount] = Contact{slot, along, gap, height, reach}
		contactCount++
	}
	insertionSort(contacts[:], contactCount, func(first, second Contact) bool {
		if first.along != second.along {
			return first.along < second.along
		}
		return first.slot < second.slot
	})

	for index := 0; index < contactCount; index++ {
		contact := &contacts[index]
		player := &e.players[contact.slot]
		e.ball.TouchAttempts |= uint32(1) << contact.slot
		stretch := contact.gap / math.Max(contact.reach, EPSILON)

		if e.ball.IsShot {
			// Saves on target are resolved at the keeper's plane; here a keeper
			// gathers a wide shot near him, or an outfielder throws his body in.
			if player.IsGoalkeeper {
				if !e.ball.ShotOnTarget {
					e.setPossession(player)
					player.ActionCooldown = e.randomFloat(Ball_MIN_KEEPER_HOLD_SECONDS,
						Ball_MAX_KEEPER_HOLD_SECONDS)
					return
				}
				continue
			}
			if e.randomFloat(0.0, 1.0) >=
				(Defending_BASE_BLOCK_CHANCE+player.Defending*Defending_DEFENDING_BLOCK_BONUS)*
					(1.0-0.5*stretch) {
				continue
			}
			e.ball.LastPossessor = player.Player
			e.ball.Kicker = player.Player
			e.ball.KickerLockout = Passing_KICKER_LOCKOUT_SECONDS
			e.ball.TouchAttempts = 0
			e.ball.IsShot = false
			e.ball.ShotOnTarget = false
			e.ball.FromThrowIn = false
			e.ball.Velocity.X *= Defending_DEFLECTION_SPEED_FACTOR
			e.ball.Velocity.Y += e.randomFloat(-Defending_MAX_DEFLECTION_Y_SPEED,
				Defending_MAX_DEFLECTION_Y_SPEED)
			ownDepth := 1.0 - player.Position.X
			if player.IsHomeTeam {
				ownDepth = player.Position.X
			}
			if ownDepth < SetPiece_BLOCK_BEHIND_DEPTH &&
				e.randomFloat(0.0, 1.0) < SetPiece_BLOCK_BEHIND_CHANCE {
				e.clearBehind(player)
			}
			e.statsOf(player).Clearances++
			e.logEventActor(MatchEventType_SHOT_BLOCKED, player)
			return
		}

		// Anyone other than the intended receiver has to stretch and react to
		// get a touch: close balls are met, fast wide ones fly past.
		intended := e.ball.IsPass && e.ball.IntendedReceiver == player.Player
		hands := handsAllowed(player)
		if !intended && !hands && ballSpeed > Ball_MAX_HEAVY_TOUCH_SPEED {
			reaction := hClamp(Ball_INTERCEPT_BASE_CHANCE+
				player.Defending*Ball_INTERCEPT_DEFENDING_BONUS-
				ballSpeed*Ball_INTERCEPT_SPEED_PENALTY,
				Ball_INTERCEPT_MIN_CHANCE, 1.0)
			closeness := (1.0 - stretch) * (1.0 - stretch)
			if e.randomFloat(0.0, 1.0) >= reaction+(1.0-reaction)*closeness {
				continue
			}
		}

		// A defender meeting a cross close to his own goal line often plays safe
		// and puts it behind, and now and then turns it into his own net.
		ownDepth := 1.0 - player.Position.X
		if player.IsHomeTeam {
			ownDepth = player.Position.X
		}
		if delivery := e.ball.IsPass && e.ball.PassByHome != player.IsHomeTeam &&
			!player.IsGoalkeeper &&
			(e.lastPassDecision.Intent == PassIntent_CROSS ||
				e.lastPassDecision.Intent == PassIntent_CUTBACK ||
				e.lastPassDecision.Intent == PassIntent_SET_PIECE); delivery && ownDepth < SetPiece_CROSS_CLEARANCE_DEPTH {
			e.statsOf(player).Interceptions++
			if e.tryOwnGoalTouch(player) {
				return
			}
			if e.randomFloat(0.0, 1.0) < SetPiece_CLEARANCE_BEHIND_CHANCE {
				e.clearFlightState()
				e.clearBehind(player)
				return
			}
			e.statsOf(player).Interceptions--
		}

		var touchSkill float64
		if hands {
			touchSkill = player.Goalkeeping
		} else {
			keeperFeet := 0.0
			if player.IsGoalkeeper {
				keeperFeet = player.Goalkeeping * Ball_KEEPER_FEET_WEIGHT
			}
			touchSkill = max(player.Dribbling, player.Passing*Ball_PASSING_TOUCH_WEIGHT, keeperFeet)
		}
		highBallPenalty := 0.0
		if contact.height > Ball_LOW_BALL_METRES {
			highBallPenalty = 0.08
		}
		if controlChance := hClamp(
			Ball_BASE_CONTROL_CHANCE+touchSkill*Ball_TOUCH_SKILL_BONUS-
				ballSpeed*Ball_SPEED_CONTROL_PENALTY-highBallPenalty,
			Ball_MIN_CONTROL_CHANCE, Ball_MAX_CONTROL_CHANCE); e.randomFloat(0.0, 1.0) > controlChance {
			// A heavy first touch: the ball is deadened but pops a metre or two
			// away from the player instead of rolling on at full pace.
			player.IsTrapping = true
			player.TrapTimer = Ball_FAILED_TRAP_TIME
			angle := e.randomFloat(-math.Pi, math.Pi)
			popSpeed := e.randomFloat(Ball_MIN_HEAVY_TOUCH_SPEED, Ball_MAX_HEAVY_TOUCH_SPEED)
			e.ball.Velocity = Vector2F{
				e.ball.Velocity.X*Ball_HEAVY_TOUCH_RETAINED + math.Cos(angle)*popSpeed,
				e.ball.Velocity.Y*Ball_HEAVY_TOUCH_RETAINED + math.Sin(angle)*popSpeed,
			}
			// Near his own goal a player's heavy touch still goes away from it.
			outward := -1.0
			if player.IsHomeTeam {
				outward = 1.0
			}
			if ownDepth < SetPiece_CROSS_CLEARANCE_DEPTH && e.ball.Velocity.X*outward < 0.0 {
				e.ball.Velocity.X = -e.ball.Velocity.X
			}
			e.ball.VelocityZ = 0.0
			e.ball.Z = 0.0
			e.ball.Curve = 0.0
			e.ball.LastPossessor = player.Player
			e.ball.Kicker = player.Player
			e.ball.KickerLockout = Ball_FAILED_TOUCH_LOCKOUT
			e.ball.TouchAttempts = 0
			return
		}

		if e.ball.IsPass && e.ball.PassWasOffside && intended {
			if player.IsHomeTeam {
				e.stats.HomeOffsides++
			} else {
				e.stats.AwayOffsides++
			}
			event := e.logEventActor(MatchEventType_OFFSIDE, player)
			event.Position = player.Position
			e.setupFreeKick(!player.IsHomeTeam, player.Position)
			return
		}

		e.recordPassCompletion(player)
		e.setPossession(player)
		player.IsTrapping = true
		player.TrapTimer = Ball_BASE_TRAP_TIME + (1.0-touchSkill)*Ball_TRAP_SKILL_PENALTY
		if hands {
			player.ActionCooldown = e.randomFloat(Ball_MIN_KEEPER_HOLD_SECONDS, Ball_MAX_KEEPER_HOLD_SECONDS)
		} else {
			player.ActionCooldown = player.TrapTimer +
				e.randomFloat(Ball_MIN_POST_TOUCH_DELAY, Ball_MAX_POST_TOUCH_DELAY)
		}
		return
	}
}

func (e *MatchEngine) resolveAerialContest() bool {
	heightMetres := e.ball.Z * Units_BALL_Z_METRES
	// A goalkeeper in his own box claims first when he can reach the ball.
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || !player.IsGoalkeeper ||
			(e.ball.KickerLockout > 0.0 && e.ball.Kicker == player.Player) ||
			!inPenaltyArea(player.Position, player.IsHomeTeam) ||
			distance(player.Position, e.ball.Position) >
				MatchRules_goalkeeperHandlingRadiusMetres(
					player.HeightMetres, player.Goalkeeping, heightMetres, player.IsDiving) ||
			heightMetres > MatchRules_goalkeeperReachMetres(player.HeightMetres, player.Goalkeeping) {
			continue
		}
		if e.ball.IsPass && e.ball.PassByHome == player.IsHomeTeam {
			continue
		}
		crowding := 0
		for j := range e.players {
			other := &e.players[j]
			if active(other) && other.IsHomeTeam != player.IsHomeTeam &&
				distance(other.Position, e.ball.Position) < Aerial_DUEL_RADIUS_METRES {
				crowding++
			}
		}
		claimChance := hClamp(
			Aerial_BASE_CLAIM_CHANCE+
				player.Goalkeeping*Aerial_SKILL_CLAIM_BONUS-
				float64(crowding)*Aerial_CROWDING_PENALTY,
			0.05, 0.97)
		control := &e.keepers[hBoolIndex(player.IsHomeTeam)]
		control.State = GoalkeeperState_CLAIM
		control.Timer = 0.0
		if e.randomFloat(0.0, 1.0) < claimChance {
			e.recordPassCompletion(player)
			e.setPossession(player)
			player.ActionCooldown = e.randomFloat(Ball_MIN_KEEPER_HOLD_SECONDS, Ball_MAX_KEEPER_HOLD_SECONDS)
			return true
		}
		// Punched clear under pressure.
		e.statsOf(player).Clearances++
		outward := -1.0
		if player.IsHomeTeam {
			outward = 1.0
		}
		e.ball.LastPossessor = player.Player
		e.clearFlightState()
		vx := outward * Aerial_HEADER_CLEARANCE_SPEED * e.randomFloat(0.5, 0.9)
		vy := e.randomFloat(-5.0, 5.0)
		e.ball.Velocity = Vector2F{vx, vy}
		e.ball.VelocityZ = Aerial_HEADER_LIFT
		e.ball.Kicker = player.Player
		e.ball.KickerLockout = Ball_FAILED_TOUCH_LOCKOUT
		e.ball.TouchAttempts = 0
		return true
	}

	// Otherwise everyone who can reach the ball contests it in the air.
	var candidates [6]*MatchPlayer
	var strengths [6]float64
	count := 0
	totalStrength := 0.0
	homeInvolved := false
	awayInvolved := false
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || player.IsGoalkeeper || count == len(candidates) ||
			player.TrapTimer > 0.0 ||
			(e.ball.KickerLockout > 0.0 && e.ball.Kicker == player.Player) {
			continue
		}
		if distance(player.Position, e.ball.Position) > Aerial_DUEL_RADIUS_METRES {
			continue
		}
		// Defenders are goal-side and facing the ball, so they win the first
		// contact more often than the attackers they mark.
		defendingDelivery := e.ball.IsPass && e.ball.PassByHome != player.IsHomeTeam
		advantage := 1.0
		if defendingDelivery {
			advantage = Aerial_DEFENDER_DUEL_ADVANTAGE
		}
		strength := MatchRules_aerialDuelStrength(player.HeightMetres, player.Physicality, heightMetres) *
			advantage
		if strength <= 0.0 {
			continue
		}
		candidates[count] = player
		strengths[count] = strength
		totalStrength += strength
		count++
		if player.IsHomeTeam {
			homeInvolved = true
		} else {
			awayInvolved = true
		}
	}
	if count == 0 {
		return false
	}

	pick := e.randomFloat(0.0, totalStrength)
	winnerIndex := count - 1
	for index := 0; index < count; index++ {
		pick -= strengths[index]
		if pick <= 0.0 {
			winnerIndex = index
			break
		}
	}
	winner := candidates[winnerIndex]
	if contested := homeInvolved && awayInvolved; contested {
		for index := 0; index < count; index++ {
			candidate := candidates[index]
			if candidate == winner {
				e.statsOf(candidate).AerialDuelsWon++
			} else if candidate.IsHomeTeam != winner.IsHomeTeam {
				e.statsOf(candidate).AerialDuelsLost++
			}
		}
		if winner.IsHomeTeam {
			e.stats.HomeAerialDuelsWon++
		} else {
			e.stats.AwayAerialDuelsWon++
		}

		// Pushing or holding in the air: the defender's foul in his own box is a
		// penalty; an attacker's push gives the defence a free kick.
		if e.incidentRoll() < Aerial_DUEL_FOUL_CHANCE {
			var opponent *MatchPlayer
			for index := 0; index < count; index++ {
				if candidates[index].IsHomeTeam != winner.IsHomeTeam {
					opponent = candidates[index]
					break
				}
			}
			if opponent != nil {
				attackerFouls := e.incidentRoll() < 0.6
				winnerAttacking := e.ball.PassByHome == winner.IsHomeTeam
				attacker, defender := opponent, winner
				if winnerAttacking {
					attacker, defender = winner, opponent
				}
				e.clearFlightState()
				e.ball.Velocity = Vector2F{0.0, 0.0}
				e.ball.VelocityZ = 0.0
				e.audit.aerialFoul = e.diag.Audit
				if attackerFouls {
					e.commitFoul(attacker, defender, true, false)
				} else {
					e.commitFoul(defender, attacker, true, false)
				}
				return true
			}
		}
	}
	e.headBall(winner)
	return true
}

func (e *MatchEngine) tryOwnGoalTouch(defender *MatchPlayer) bool {
	// Only a touch in front of his own goal can go in off a defender.
	ownX := 1.0 - defender.Position.X
	if defender.IsHomeTeam {
		ownX = defender.Position.X
	}
	goalLineMetres := ownX * Pitch_LENGTH_METRES
	if lateralMetres := math.Abs(defender.Position.Y-Pitch_CENTRE) * Pitch_WIDTH_METRES; goalLineMetres > SetPiece_OWN_GOAL_TOUCH_DEPTH_METRES ||
		lateralMetres > SetPiece_OWN_GOAL_TOUCH_WIDTH_METRES ||
		e.randomFloat(0.0, 1.0) >= SetPiece_OWN_GOAL_TOUCH_CHANCE {
		return false
	}
	// The ball skews off him toward the goal he defends.
	goalX := 1.01
	if defender.IsHomeTeam {
		goalX = -0.01
	}
	aimMetres := e.randomFloat(-Shooting_GOAL_HALF_WIDTH_METRES, Shooting_GOAL_HALF_WIDTH_METRES)
	target := Vector2F{goalX, Pitch_CENTRE + aimMetres/Pitch_WIDTH_METRES}
	// An accidental defensive touch does not end the attacking restart's
	// origin. Preserve it across the replacement flight for goal accounting.
	fromSetPiece := e.ball.ShotFromSetPiece ||
		(e.setPiecePhaseRemaining > 0.0 && defender.IsHomeTeam != e.setPieceHome)
	height := e.ball.Z
	e.clearFlightState()
	// C++ argument evaluation order is unspecified; Go draws left to right.
	horizontalSpeed := e.randomFloat(SetPiece_OWN_GOAL_MIN_SPEED, SetPiece_OWN_GOAL_MAX_SPEED)
	verticalSpeed := e.randomFloat(0.0, SetPiece_OWN_GOAL_MAX_LIFT)
	e.launchBall(defender, e.ball.Position, target, horizontalSpeed, verticalSpeed, 0.0)
	e.ball.Z = height
	e.ball.ShotFromSetPiece = fromSetPiece
	e.lastShooter = nil
	return true
}

func (e *MatchEngine) clearBehind(defender *MatchPlayer) {
	// Knock the ball over the own goal line, wide of the posts.
	goalX := 1.0
	goalOffset := 0.02
	if defender.IsHomeTeam {
		goalX = 0.0
		goalOffset = -0.02
	}
	side := 1.0
	if defender.Position.Y < Pitch_CENTRE {
		side = -1.0
	}
	target := Vector2F{
		goalX + goalOffset,
		Pitch_CENTRE + side*e.randomFloat(Pitch_GOAL_BOTTOM-Pitch_CENTRE+0.06, 0.45),
	}
	height := e.ball.Z
	e.launchBall(defender, defender.Position, target,
		Aerial_HEADER_CLEARANCE_SPEED, Aerial_HEADER_LIFT*0.5, 0.0)
	e.ball.Z = height
	e.lastShooter = nil
}

func (e *MatchEngine) clearBall(defender *MatchPlayer) {
	e.trackTouch(defender)
	// A long, high clearance upfield and towards the nearer touchline.
	forward := -1.0
	if defender.IsHomeTeam {
		forward = 1.0
	}
	touchline := 1.0
	if defender.Position.Y < Pitch_CENTRE {
		touchline = -1.0
	}
	clearanceMetres := e.randomFloat(Defending_CLEARANCE_MIN_DISTANCE_METRES,
		Defending_CLEARANCE_MAX_DISTANCE_METRES)
	direction := normalized(Vector2F{forward,
		touchline * e.randomFloat(0.0, 1.0) * Defending_CLEARANCE_TOUCHLINE_BIAS * 2.0})
	reach := toPitch(Vector2F{direction.X * clearanceMetres, direction.Y * clearanceMetres})
	target := Vector2F{defender.Position.X + reach.X, defender.Position.Y + reach.Y}
	speed := Defending_CLEARANCE_SPEED * e.randomFloat(0.85, 1.0)
	e.statsOf(defender).Clearances++
	e.clearFlightState()
	e.launchBall(defender, defender.Position, target, speed,
		loftVerticalSpeed(clearanceMetres, speed, 0.0, 0.0), 0.0)
	e.lastShooter = nil
	defender.ActionCooldown = e.randomFloat(Passing_MIN_ACTION_COOLDOWN, Passing_MAX_ACTION_COOLDOWN)
}

func (e *MatchEngine) headBall(header *MatchPlayer) {
	e.trackTouch(header)
	attackDirection := -1.0
	depth := 1.0 - header.Position.X
	if header.IsHomeTeam {
		attackDirection = 1.0
		depth = header.Position.X
	}
	fromTeammate := e.ball.IsPass && e.ball.PassByHome == header.IsHomeTeam
	deliverer := e.findMatchPlayer(e.ball.LastPossessor)
	if fromTeammate && e.ball.PassWasOffside && e.ball.IntendedReceiver == header.Player {
		if header.IsHomeTeam {
			e.stats.HomeOffsides++
		} else {
			e.stats.AwayOffsides++
		}
		event := e.logEventActor(MatchEventType_OFFSIDE, header)
		event.Position = header.Position
		e.setupFreeKick(!header.IsHomeTeam, header.Position)
		return
	}

	// A header at goal needs a real chance; from further out the ball is
	// nodded down for a team-mate instead.
	if fromTeammate && depth >= Aerial_HEADER_SHOT_MIN_DEPTH &&
		math.Abs(header.Position.Y-Pitch_CENTRE) <= Aerial_HEADER_SHOT_WIDTH &&
		e.estimateShotXG(header)*Shooting_HEADER_XG_FACTOR >= Aerial_HEADER_SHOT_MIN_XG {
		if deliverer != nil && deliverer.IsHomeTeam == header.IsHomeTeam && deliverer != header {
			e.statsOf(deliverer).PassesCompleted++
			e.trackPassCompletion(deliverer, header)
			if header.IsHomeTeam {
				e.stats.HomePassesCompleted++
			} else {
				e.stats.AwayPassesCompleted++
			}
			e.shotAssistCandidate = deliverer.Player
		} else {
			e.shotAssistCandidate = nil
		}
		e.ball.Position = header.Position
		// C++ default argument freeKick = false.
		e.takeShot(header, -1.0, true, false)
		return
	}

	if fromTeammate && depth < Aerial_HEADER_KNOCK_DOWN_MIN_DEPTH {
		// A team-mate's lofted ball away from goal is cushioned down and kept.
		e.recordPassCompletion(header)
		e.setPossession(header)
		header.IsTrapping = true
		header.TrapTimer = Ball_BASE_TRAP_TIME + (1.0-header.Dribbling)*Ball_TRAP_SKILL_PENALTY
		header.ActionCooldown = header.TrapTimer +
			e.randomFloat(Ball_MIN_POST_TOUCH_DELAY, Ball_MAX_POST_TOUCH_DELAY)
		return
	}
	if fromTeammate {
		e.recordPassCompletion(header)
	} else if e.ball.IsPass {
		e.statsOf(header).Interceptions++
	}
	ownDepth := 1.0 - depth
	e.clearFlightState()
	e.ball.LastPossessor = header.Player
	e.lastShooter = nil
	e.ball.Position = header.Position
	if !fromTeammate && e.tryOwnGoalTouch(header) {
		return
	}
	if !fromTeammate &&
		ownDepth > 1.0-SetPiece_CROSS_CLEARANCE_DEPTH &&
		e.randomFloat(0.0, 1.0) < SetPiece_HEADER_BEHIND_CHANCE {
		e.statsOf(header).Clearances++
		e.clearBehind(header)
	} else if !fromTeammate && ownDepth > 0.55 {
		// Defensive header: clear the danger, upfield and away from the middle.
		e.statsOf(header).Clearances++
		wide := 1.0
		if header.Position.Y < Pitch_CENTRE {
			wide = -1.0
		}
		dx := attackDirection * e.randomFloat(0.5, 1.0)
		dy := wide * e.randomFloat(0.1, 0.8)
		direction := normalized(Vector2F{dx, dy})
		// Many defensive headers are half-clearances that drop around the edge
		// of the box, where the second ball is fought for.
		speed := Aerial_HEADER_CLEARANCE_SPEED * e.randomFloat(Aerial_MIN_CLEARANCE_SHARE, 1.0)
		height := e.ball.Z
		reach := toPitch(Vector2F{direction.X, direction.Y})
		e.launchBall(header, header.Position,
			Vector2F{header.Position.X + reach.X, header.Position.Y + reach.Y},
			speed, Aerial_HEADER_LIFT, 0.0)
		e.ball.Z = height
	} else {
		// Knock-down or flick-on toward the nearest team-mate who is not
		// behind the header (never back toward the own goal).
		var target *MatchPlayer
		best := math.MaxFloat64
		for i := range e.players {
			other := &e.players[i]
			if !active(other) || other == header ||
				other.IsHomeTeam != header.IsHomeTeam || other.IsGoalkeeper {
				continue
			}
			if (other.Position.X-header.Position.X)*attackDirection*Pitch_LENGTH_METRES <
				-Aerial_KNOCK_DOWN_MAX_BACKWARD_METRES {
				continue
			}
			d := distance(other.Position, header.Position)
			if d < best {
				best = d
				target = other
			}
		}
		aim := Vector2F{header.Position.X + attackDirection*0.1, header.Position.Y}
		if target != nil {
			aim = target.Position
		}
		kx := aim.X + e.randomFloat(-0.03, 0.03)
		ky := aim.Y + e.randomFloat(-0.03, 0.03)
		knockDown := Vector2F{kx, ky}
		height := e.ball.Z
		e.launchBall(header, header.Position, knockDown,
			Aerial_HEADER_PASS_SPEED, Aerial_HEADER_LIFT*0.5, 0.0)
		e.ball.Z = height
	}
}
