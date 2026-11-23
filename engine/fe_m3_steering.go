// Ported from FootballEngine FootballEngine.Core/Match/Systems/Players/Steering.fs (GPL-3.0)
// M3 steering (FE_PORT_PLAN §2 M3): effort comes from the intent only
// (Steering.fs:110-140, DefensiveTransition efforts BalanceConfig.fs:911-913),
// no Player12 distance urgency (P12:4438), Player12's arrive braking
// sqrt(2·a·(d-0.35)) and kinematics (P12:4452-4455, stepKinematics), no target
// lag for press / loose ball / receive / carry. The press velocity feed-forward
// is the plan's Go-side improvement (ours). Per-player individuality is in
// fe_m3_individual.go (ours).
package engine

import "math"

// FE desired = Jog + (Sprint - Jog)·mult. FE's absolute 5.0 / 8.5 m/s are too
// fast for running volume (plan §2 M3, §5), so the same law is expressed as
// shares of the player's own top speed.
const (
	feM3JogShare    = 0.24
	feM3SprintShare = 0.96
	// FE DefensiveTransition (BalanceConfig.fs:911-913).
	feM3CoverEffort  = 0.85
	feM3ScreenEffort = 0.85
	// FE RecoveryEffort is 1.0; every recovering Go player carries
	// RECOVER_SHAPE in transition, so this is ours (volume).
	feM3RecoveryEffort = 0.3
	// Press / ExecuteRun / Contain (Steering.fs:128-135).
	feM3RunEffort = 1.0
	// Press feed-forward is used inside this range of the carrier (m).
	feM3FeedForwardMetres = 15.0
	// It fades out from Near+Fade to Near metres so the presser settles
	// rather than running through a carrier who checks back.
	feM3FeedForwardNear = 1.5
	feM3FeedForwardFade = 3.0
	// No effort share reaches the full top speed (personal multipliers).
	feM3MaxShare = 0.97
	// Back four out of possession, shape intents (ours; Player12's
	// RECOVERY_SPEED_SCALE): the line must hold its depth while it moves.
	feM3BackLineShare = 0.48 // Player12 RECOVERY_SPEED_SCALE 0.58; lowered for running volume
	// Live-play speed ceiling (m/s, ours): Player12's top speed reaches
	// 10 m/s at Pace 1; broadcast tracking rarely shows outfielders above
	// about 9.5 m/s. (The 4 Hz viewer replay samples 0.1 s steps at 0.2/0.3 s
	// intervals, so its frame speeds read up to 1.2x the true speed.)
	feM3SpeedCap = 9.3
	// Support / shape effort in settled possession (ours; FE phase boost
	// 0.25): a jog of about 0.3 of top speed.
	feM3SettledShare = 0.19
	// Settled-share multiplier from a short run (ShortRun m) to a long one
	// (ShortRun + LongRun m).
	feM3SettledMin      = 0.7
	feM3SettledMax      = 1.7
	feM3SettledShortRun = 3.0
	feM3SettledLongRun  = 20.0
	feM3TowardBallCos   = 0.75
	// Ball-side full-back push (fe_m4_shape.go): a purposeful run forward.
	feM3FullBackPushShare = 0.5
	// Support effort of a defender or midfielder in an attacking transition
	// (ours; FE TransitionAttack boost 0.35 is kept for forwards).
	feM3TransitionSupportBoost = 0.05
	// ...unless the ball is this far ahead of him (m).
	feM3TransitionCatchUp = 12.0
)

// feM3PhaseBoost is Steering.fs:116-121.
func feM3PhaseBoost(phase TeamPhase) float64 {
	switch phase {
	case TeamPhase_POSSESSION, TeamPhase_FINAL_THIRD:
		return 0.25
	case TeamPhase_ATTACKING_TRANSITION:
		return 0.35
	case TeamPhase_DEFENSIVE_TRANSITION:
		return 0.20
	}
	return 0.10
}

// feM3Effort is the share of the player's top speed his intent asks for.
func feM3Effort(intent PlayerIntent, phase TeamPhase) float64 {
	transition := phase == TeamPhase_DEFENSIVE_TRANSITION || phase == TeamPhase_ATTACKING_TRANSITION
	mult := feM3PhaseBoost(phase)
	switch intent {
	case PlayerIntent_CARRY_BALL:
		// FE CarryEffort 0.15 is tied to FE carry physics; keep Player12's.
		return Player_CARRY_BALL_SPEED_SCALE
	case PlayerIntent_PRESS_BALL, PlayerIntent_CLAIM_LOOSE_BALL,
		PlayerIntent_RUN_IN_BEHIND, PlayerIntent_ATTACK_BOX, PlayerIntent_OVERLAP,
		PlayerIntent_RECEIVE_PASS:
		mult = feM3RunEffort
	case PlayerIntent_COVER_PRESS, PlayerIntent_BLOCK_PASSING_LANE:
		if transition {
			mult = feM3CoverEffort
		}
	case PlayerIntent_MARK_OPPONENT:
		if transition {
			mult = feM3ScreenEffort
		} else {
			return feM3SettledShare
		}
	case PlayerIntent_OFFER_SUPPORT, PlayerIntent_HOLD_SHAPE:
		// Running volume (ours): settled support, shape and zonal marking
		// move at a walk-jog rather than FE's phase boost; transitions keep it.
		if !transition {
			return feM3SettledShare
		}
	case PlayerIntent_RECOVER_SHAPE:
		// FE Regroup: the recovery effort only in transition.
		if phase == TeamPhase_DEFENSIVE_TRANSITION {
			mult = feM3RecoveryEffort
		}
	case PlayerIntent_GOALKEEP:
		return Player_GOALKEEPER_MOVEMENT_SPEED_SCALE
	}
	return feM3JogShare + (feM3SprintShare-feM3JogShare)*mult
}

// feM3IntegrateMovements replaces the live-play branch of integrateMovements.
func (e *MatchEngine) feM3IntegrateMovements(slots []uint8, dt float64) {
	st := e.feM3()
	st.beginStep(e)
	st.gatherUnits(e, slots)
	carrier := e.findMatchPlayer(e.ball.PossessedBy)
	for _, index := range slots {
		player := &e.players[index]
		s := st.slot(e, int(index), player)
		raw := feM3RawTarget(player)
		if player.Intent == PlayerIntent_CARRY_BALL && player == carrier && feM3UsesCarryTarget(player) {
			raw = e.feM3CarryTarget(player, s)
		} else {
			s.carrying = false
		}
		phase := e.awayPhase
		if player.IsHomeTeam {
			phase = e.homePhase
		}
		var defending bool
		if carrier != nil {
			defending = carrier.IsHomeTeam != player.IsHomeTeam
		} else {
			defending = e.lastControlledTeamHome.HasValue &&
				e.lastControlledTeamHome.Value != player.IsHomeTeam
		}
		role := player.Player.GetRole()
		backLine := defending && !player.IsGoalkeeper &&
			feM3ClassOf(player.Intent) == feM3Shape &&
			(role == PlayerRole_CB || role == PlayerRole_LB || role == PlayerRole_RB)

		if player.ComposeSpeed > 0 && e.state == MatchState_PLAYING {
			// F2: a composer target is already smooth (slots follow the ball as a
			// function, offsets are capped): M3 executes it directly, without the
			// shape filter, deadband and reaction delays that made players trail
			// a moving slot by ~10 m.
			player.MovementTarget = raw
			s.filtered, s.filterReady, s.pending, s.moving, s.idle = raw, true, false, true, Vector2F{}
		} else {
			e.feM3Follow(player, s, raw, backLine, dt)
		}
		s.trackTarget(player.MovementTarget, dt)
		idle := e.feM3IdleOffset(player, s, backLine)
		steer := player.MovementTarget
		if idle.X != 0.0 || idle.Y != 0.0 {
			p := toPitch(idle)
			steer.X = hClamp(steer.X+p.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X)
			steer.Y = hClamp(steer.Y+p.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)
		}
		offset := toMetres(Vector2F{steer.X - player.Position.X, steer.Y - player.Position.Y})
		targetDistance := math.Hypot(offset.X, offset.Y)
		if targetDistance <= Player_ARRIVAL_DEAD_ZONE_METRES &&
			player.Velocity.X == 0.0 && player.Velocity.Y == 0.0 {
			continue
		}

		fatigue := hClamp((1.0-player.Stamina)*INVERSE_FATIGUE_RANGE, 0.0, 1.0)
		topSpeed := math.Min(e.currentTopSpeed(player), feM3SpeedCap)
		share := feM3Effort(player.Intent, phase)
		// A catch-up or push run is a run, not a settled adjustment
		// (m3.catch_up_run): the adjustment pace cap does not apply, and the
		// catch-up reads the tactical spot (not the lagging steering point).
		running := false
		attackDir := -1.0
		if player.IsHomeTeam {
			attackDir = 1.0
		}
		catchUp := func() bool {
			if !e.cfg.M3.CatchUpRun {
				return offset.X*attackDir > e.cfg.M3.CatchUpDistance
			}
			return (raw.X-player.Position.X)*Pitch_LENGTH_METRES*attackDir > e.cfg.M3.CatchUpDistance
		}
		if share == feM3SettledShare {
			// A settled adjustment walks when it is short and jogs when it is
			// long (ours): spreads the speed spectrum instead of one pace.
			// A run straight at the ball is never hurried: support spreads to
			// give angles rather than converging on the carrier.
			most := feM3SettledMax
			ball := toMetres(Vector2F{e.ball.Position.X - player.Position.X, e.ball.Position.Y - player.Position.Y})
			if bd := math.Hypot(ball.X, ball.Y); bd > EPSILON && targetDistance > EPSILON &&
				(offset.X*ball.X+offset.Y*ball.Y)/(bd*targetDistance) > feM3TowardBallCos {
				most = 1.0
			}
			share *= feM3SettledMin + (most-feM3SettledMin)*
				hClamp((s.runLength-feM3SettledShortRun)/feM3SettledLongRun, 0.0, 1.0)
			// Phase 2 (ours): catching up with the play. A settled supporter
			// or holder whose spot is more than CatchUpDistance ahead in the
			// attacking direction moves up at CatchUpShare (a jog-run), so the
			// pivot does not trail an attack by 30 m; other settled moves,
			// including runs at the ball, keep the walk-jog.
			if cu := e.cfg.M3.CatchUpShare; cu > 0 && catchUp() {
				share = math.Max(share, cu)
				running = true
			}
		}
		if e.feM4Pushed(player) {
			// The ball-side full-back getting forward runs (ours).
			share = math.Max(share, feM3FullBackPushShare)
			running = true
		}
		if player.Intent == PlayerIntent_OFFER_SUPPORT && phase == TeamPhase_ATTACKING_TRANSITION &&
			!feM1Forward(player) {
			// A defender or midfielder supports a counter at a jog unless the
			// ball is already well past him (then FE's transition boost).
			ahead := (e.ball.Position.X - player.Position.X) * Pitch_LENGTH_METRES
			if !player.IsHomeTeam {
				ahead = -ahead
			}
			if ahead < feM3TransitionCatchUp {
				share = feM3JogShare + (feM3SprintShare-feM3JogShare)*feM3TransitionSupportBoost
			}
			// ...and catches up with the play like a settled supporter.
			if cu := e.cfg.M3.CatchUpShare; e.cfg.M3.CatchUpRun && cu > 0 && catchUp() {
				share = math.Max(share, cu)
				running = true
			}
		}
		// A player on an M1 task (press, cover, screen, box marker, recovery
		// run) runs at the FE effort of that task, on the same jog→sprint law.
		m1Effort, m1Task := e.feM1Effort(player)
		if m1Task {
			share = feM3JogShare + (feM3SprintShare-feM3JogShare)*m1Effort
		}
		if player.IsGoalkeeper {
			// Keepers keep Player12's urgency to reach the ball-goal line.
			share = math.Max(share, math.Min(Player_MAX_URGENCY_SPEED_SCALE,
				targetDistance/Player_URGENCY_DISTANCE_METRES))
		} else if m1Task {
			// FE effort with the personal multiplier only: a recovery run is
			// a real sprint, faded by the arrive braking near the spot.
			share *= s.personal
		} else if backLine {
			// The back four keep one pace (ours): no personal variation, and
			// at least Player12's recovery share so the line drops together.
			share = math.Max(share, feM3BackLineShare)
		} else if feM3ClassOf(player.Intent) != feM3Direct {
			share *= s.personal * s.runScale
		} else {
			share *= s.personal
		}
		share = math.Min(share, feM3MaxShare)
		speedCap := math.Min(topSpeed, player.MaxSpeed*
			(1.0-Player_FATIGUE_WORK_RATE_LOSS*fatigue+e.talkOf(player.IsHomeTeam)+
				e.shoutWorkRate(player.IsHomeTeam))*share)
		if !player.IsGoalkeeper && !backLine && !m1Task && !(running && e.cfg.M3.CatchUpRun) {
			speedCap = math.Min(speedCap, s.feM3AdjustCap(player))
		}
		// F2: a slot or offset target moves at the composer's speed class (M3 only
		// executes; the intent-based shares above do not apply).
		if player.ComposeSpeed > 0 && !player.IsGoalkeeper && e.state == MatchState_PLAYING {
			speedCap = math.Min(topSpeed, e.teamShapeSpeed(player.ComposeSpeed))
			if player.ComposeSpeed != teamShapeTrot {
				speedCap = math.Min(speedCap, math.Max(player.ComposeWant, e.cfg.TeamShape.WalkSpeed))
			}
		}
		if s.idling {
			speedCap = math.Min(speedCap, s.idleSpeed)
		}
		// Arrive (Player12): a comfortable deceleration stops him on the spot.
		approach := targetDistance - Player_ARRIVAL_DEAD_ZONE_METRES
		desiredSpeed := 0.0
		if approach > 0.0 {
			desiredSpeed = math.Min(speedCap,
				math.Sqrt(2.0*Player_ARRIVAL_DECELERATION*approach))
		}
		desired := Vector2F{}
		if targetDistance > EPSILON {
			dirX, dirY := offset.X/targetDistance, offset.Y/targetDistance
			if !backLine {
				dirX, dirY = e.feM3Bend(player, s, dirX, dirY, targetDistance, defending, dt)
			}
			desired = Vector2F{dirX * desiredSpeed, dirY * desiredSpeed}
		}
		// Press feed-forward (ours): match the carrier's velocity and close
		// the gap on top, instead of braking to a stop at a moving point.
		if player.Intent == PlayerIntent_PRESS_BALL && carrier != nil &&
			carrier.IsHomeTeam != player.IsHomeTeam &&
			distance(player.Position, carrier.Position) < feM3FeedForwardMetres {
			gap := distance(player.Position, carrier.Position)
			weight := hClamp((gap-feM3FeedForwardNear)/feM3FeedForwardFade, 0.0, 1.0)
			desired.X += carrier.Velocity.X * weight
			desired.Y += carrier.Velocity.Y * weight
			magnitude := math.Hypot(desired.X, desired.Y)
			if magnitude > speedCap {
				desired.X *= speedCap / magnitude
				desired.Y *= speedCap / magnitude
				magnitude = speedCap
			}
			desiredSpeed = magnitude
		}
		// Target feed-forward (ours): a player tracking a moving spot (the
		// line dropping, the block sliding) moves with it and closes the gap
		// on top, instead of trailing it by the arrive-braking distance.
		if backLine {
			desired.X += s.targetVel.X
			desired.Y += s.targetVel.Y
			magnitude := math.Hypot(desired.X, desired.Y)
			if magnitude > speedCap {
				desired.X *= speedCap / magnitude
				desired.Y *= speedCap / magnitude
				magnitude = speedCap
			}
			desiredSpeed = magnitude
		}
		e.stepKinematics(player, desired, desiredSpeed, topSpeed, fatigue, dt)
	}
}
