// Ported from Player12 src/model/match_engine.cpp lines 7945–8693 (GPL-3.0)

package engine

import "math"

func (e *MatchEngine) setupKickOff(homeKickingOff bool) {
	// The goal already opened this break in play. Changes during its
	// celebration and the following kick-off share one substitution window.
	if e.state != MatchState_GOAL {
		e.beginStoppage()
	}
	e.restartIsSetPiece = false
	e.pendingAdvantage.Active = false
	e.resetPositions()
	// At kick-off every player must be in their own half. The normal tactical
	// positions deliberately span more of the pitch and are restored through
	// regular movement once play starts.
	for i := range e.players {
		player := &e.players[i]
		if !active(player) {
			continue
		}
		player.Position = e.kickOffPosition(player)
		player.MovementTarget = player.Position
	}
	e.ball = NewMatchBall()
	e.lastShooter = nil
	centre := Vector2F{Pitch_CENTRE, Pitch_CENTRE}
	taker := e.findClosestPlayer(centre, homeKickingOff, false)
	if e.cfg.Modules.M9Restart {
		// FE M9: the player who walked to the spot during the celebration.
		if planned := e.feM9KickOffTaker(homeKickingOff); planned != nil {
			taker = planned
		}
	}
	if taker == nil {
		taker = e.findClosestPlayer(centre, homeKickingOff, true)
	}
	e.setRestartTaker(taker)
	if taker != nil {
		x := Pitch_AWAY_KICKOFF_X
		if homeKickingOff {
			x = Pitch_HOME_KICKOFF_X
		}
		taker.Position = Vector2F{x, Pitch_CENTRE}
		taker.MovementTarget = taker.Position
		e.ball.PossessedBy = taker.Player
		e.ball.LastPossessor = taker.Player
	}
	e.state = MatchState_KICK_OFF
	e.setPieceTimer = Timing_KICKOFF_DELAY_SECONDS
	e.updateTeamPhases()
}

func (e *MatchEngine) kickOffPosition(player *MatchPlayer) Vector2F {
	var x float64
	if player.IsHomeTeam {
		x = Pitch_KICKOFF_FORMATION_INSET + player.BasePosition.X*Pitch_KICKOFF_FORMATION_SCALE
	} else {
		x = (1.0 - Pitch_KICKOFF_FORMATION_INSET) - (1.0-player.BasePosition.X)*Pitch_KICKOFF_FORMATION_SCALE
	}
	if e.cfg.Modules.M9Restart {
		// FE M9: outside the centre circle (the taker is placed on the spot).
		position := feM9AwayFrom(Vector2F{Pitch_CENTRE, Pitch_CENTRE}, Vector2F{x, player.BasePosition.Y},
			feM9WallDistance+1.0)
		if player.IsHomeTeam {
			position.X = math.Min(position.X, Pitch_CENTRE-0.005)
		} else {
			position.X = math.Max(position.X, Pitch_CENTRE+0.005)
		}
		return position
	}
	return Vector2F{x, player.BasePosition.Y}
}

func (e *MatchEngine) setupThrowIn(homeTeam bool) {
	e.beginStoppage()
	e.restartIsSetPiece = false
	e.ball.Position.X = hClamp(e.ball.Position.X, Pitch_RESTART_LONGITUDINAL_MARGIN,
		1.0-Pitch_RESTART_LONGITUDINAL_MARGIN)
	if e.ball.Position.Y < Pitch_CENTRE {
		e.ball.Position.Y = Pitch_RESTART_INSET
	} else {
		e.ball.Position.Y = 1.0 - Pitch_RESTART_INSET
	}
	e.ball.Z = 0.0
	e.ball.VelocityZ = 0.0
	// A designated long-throw specialist takes the throws in the attacking
	// third; elsewhere (and without one) the nearest player does.
	attackingDepth := 1.0 - e.ball.Position.X
	if homeTeam {
		attackingDepth = e.ball.Position.X
	}
	var taker *MatchPlayer
	if attackingDepth >= Rules_HOME_FINAL_THIRD_START {
		taker = e.dutyTaker(homeTeam, SetPieceDuty_LongThrows)
	}
	if taker == nil {
		taker = e.findClosestPlayer(e.ball.Position, homeTeam, false)
	}
	e.setRestartTaker(taker)
	// The taker walks to the touchline spot during the stoppage.
	if taker != nil {
		taker.MovementTarget = e.ball.Position
	}
	e.ball.Velocity = Vector2F{0.0, 0.0}
	e.ball.PossessedBy = b6PlayerOf(taker)
	e.ball.LastPossessor = e.ball.PossessedBy
	e.clearFlightState()
	e.state = MatchState_THROW_IN
	e.setPieceTimer = Timing_THROW_IN_DELAY_SECONDS
	if e.cfg.Modules.M9Restart {
		e.setPieceTimer = e.feM9Delay(MatchState_THROW_IN, 0.0)
	}
	e.updateTeamPhases()
	event := e.logEvent(MatchEventType_THROW_IN)
	event.HasTeam = true
	event.IsHomeTeam = homeTeam
	event.Position = e.ball.Position
	if taker != nil && taker.Player != nil {
		event.PrimaryPlayerId = taker.Player.GetId()
	}
}

func (e *MatchEngine) setupGoalKick(homeTeam bool) {
	e.beginStoppage()
	e.restartIsSetPiece = false
	x := Pitch_RIGHT_GOAL_KICK_X
	if homeTeam {
		x = Pitch_LEFT_GOAL_KICK_X
	}
	e.ball.Position = Vector2F{x, Pitch_CENTRE}
	e.ball.Z = 0.0
	e.ball.VelocityZ = 0.0
	taker := e.findGoalkeeper(homeTeam)
	if taker == nil {
		taker = e.findClosestPlayer(e.ball.Position, homeTeam, true)
	}
	e.setRestartTaker(taker)
	if taker != nil {
		taker.MovementTarget = e.ball.Position
	}
	e.ball.Velocity = Vector2F{0.0, 0.0}
	e.ball.PossessedBy = b6PlayerOf(taker)
	e.ball.LastPossessor = e.ball.PossessedBy
	e.clearFlightState()
	e.state = MatchState_GOAL_KICK
	e.setPieceTimer = Timing_GOAL_KICK_DELAY_SECONDS
	if e.cfg.Modules.M9Restart {
		e.feM9ArrangeGoalKick(homeTeam)
		if taker != nil {
			taker.MovementTarget = e.ball.Position
		}
		e.setPieceTimer = e.feM9Delay(MatchState_GOAL_KICK, 0.0)
	}
	e.updateTeamPhases()
	event := e.logEvent(MatchEventType_GOAL_KICK)
	event.HasTeam = true
	event.IsHomeTeam = homeTeam
	event.Position = e.ball.Position
}

func (e *MatchEngine) setupCorner(homeTeam bool, topCorner bool) {
	e.beginStoppage()
	e.restartIsSetPiece = true
	x := Pitch_RESTART_INSET
	if homeTeam {
		x = 1.0 - Pitch_RESTART_INSET
	}
	y := 1.0 - Pitch_RESTART_INSET
	if topCorner {
		y = Pitch_RESTART_INSET
	}
	e.ball.Position = Vector2F{x, y}
	e.ball.Z = 0.0
	e.ball.VelocityZ = 0.0
	// Left and right follow the formation: the corner on the side of the
	// team's left-sided positions (low y) is its left corner.
	duty := SetPieceDuty_CornersRight
	if topCorner {
		duty = SetPieceDuty_CornersLeft
	}
	taker := e.dutyTaker(homeTeam, duty)
	if taker == nil {
		taker = e.findClosestPlayer(e.ball.Position, homeTeam, false)
	}
	e.setRestartTaker(taker)
	if taker != nil && e.cfg.Modules.M9Restart {
		// FE M9: the taker walks to the flag instead of appearing there.
		taker.MovementTarget = e.ball.Position
	} else if taker != nil {
		e.placeTaker(taker, e.ball.Position)
	}
	e.ball.Velocity = Vector2F{0.0, 0.0}
	e.ball.PossessedBy = b6PlayerOf(taker)
	e.ball.LastPossessor = e.ball.PossessedBy
	e.clearFlightState()
	if e.cfg.Modules.M9Restart {
		e.feM9ArrangeSetPiece(homeTeam, e.ball.Position, true)
	} else {
		e.arrangeSetPiece(homeTeam, e.ball.Position)
	}
	e.state = MatchState_CORNER_KICK
	e.setPieceTimer = Timing_CORNER_DELAY_SECONDS
	if e.cfg.Modules.M9Restart {
		e.setPieceTimer = e.feM9Delay(MatchState_CORNER_KICK, 0.0)
	}
	e.updateTeamPhases()
	if homeTeam {
		e.stats.HomeCorners++
	} else {
		e.stats.AwayCorners++
	}
	event := e.logEvent(MatchEventType_CORNER)
	event.HasTeam = true
	event.IsHomeTeam = homeTeam
	event.Position = e.ball.Position
	if taker != nil && taker.Player != nil {
		event.PrimaryPlayerId = taker.Player.GetId()
	}
}

func (e *MatchEngine) setupFreeKick(homeTeam bool, foulPos Vector2F) {
	e.beginStoppage()
	e.pendingAdvantage.Active = false
	e.ball.Position = Vector2F{
		hClamp(foulPos.X, Pitch_RESTART_LONGITUDINAL_MARGIN, 1.0-Pitch_RESTART_LONGITUDINAL_MARGIN),
		hClamp(foulPos.Y, Pitch_RESTART_LONGITUDINAL_MARGIN, 1.0-Pitch_RESTART_LONGITUDINAL_MARGIN),
	}
	e.ball.Z = 0.0
	e.ball.VelocityZ = 0.0
	goalX := 0.0
	if homeTeam {
		goalX = 1.0
	}
	goal := Vector2F{goalX, Pitch_CENTRE}
	goalMetres := distance(e.ball.Position, goal)
	// Free kicks in the attacking third are set pieces taken by the specialist.
	e.restartIsSetPiece = goalMetres <= SetPiece_CROSSING_FREE_KICK_METRES
	var taker *MatchPlayer
	if e.restartIsSetPiece {
		taker = e.dutyTaker(homeTeam, SetPieceDuty_FreeKicks)
	}
	if taker == nil {
		taker = e.findClosestPlayer(e.ball.Position, homeTeam, false)
	}
	e.setRestartTaker(taker)
	if taker != nil && e.restartIsSetPiece && !e.cfg.Modules.M9Restart {
		e.placeTaker(taker, e.ball.Position)
	} else if taker != nil {
		taker.MovementTarget = e.ball.Position
	}
	e.ball.Velocity = Vector2F{0.0, 0.0}
	e.ball.PossessedBy = b6PlayerOf(taker)
	e.ball.LastPossessor = e.ball.PossessedBy
	e.clearFlightState()
	if e.restartIsSetPiece && e.cfg.Modules.M9Restart {
		e.feM9ArrangeSetPiece(homeTeam, e.ball.Position, false)
	} else if e.restartIsSetPiece {
		e.arrangeSetPiece(homeTeam, e.ball.Position)
	}
	e.state = MatchState_FREE_KICK
	if e.restartIsSetPiece {
		e.setPieceTimer = Timing_SET_PIECE_FREE_KICK_DELAY_SECONDS
	} else {
		e.setPieceTimer = Timing_FREE_KICK_DELAY_SECONDS
	}
	if e.cfg.Modules.M9Restart {
		e.setPieceTimer = e.feM9Delay(MatchState_FREE_KICK, goalMetres)
	}
	e.updateTeamPhases()
	event := e.logEvent(MatchEventType_FREE_KICK)
	event.HasTeam = true
	event.IsHomeTeam = homeTeam
	event.Position = e.ball.Position
	if taker != nil && taker.Player != nil {
		event.PrimaryPlayerId = taker.Player.GetId()
	}
}

func (e *MatchEngine) setupPenalty(homeTeam bool) {
	e.beginStoppage()
	e.restartIsSetPiece = true
	e.pendingAdvantage.Active = false
	spotX := Pitch_LEFT_PENALTY_SPOT_X
	if homeTeam {
		spotX = Pitch_RIGHT_PENALTY_SPOT_X
	}
	spot := Vector2F{spotX, Pitch_CENTRE}
	taker := e.dutyTaker(homeTeam, SetPieceDuty_Penalties)
	e.setRestartTaker(taker)
	e.ball.Position = spot
	e.ball.Z = 0.0
	e.ball.VelocityZ = 0.0
	e.ball.Velocity = Vector2F{0.0, 0.0}
	e.ball.PossessedBy = b6PlayerOf(taker)
	e.ball.LastPossessor = e.ball.PossessedBy
	if e.cfg.Modules.M9Restart {
		// FE M9: everyone walks to a legal place outside the area and the arc.
		e.feM9ArrangePenalty(homeTeam, spot, taker)
	} else if taker != nil {
		e.placeTaker(taker, spot)
	}
	// Everyone else waits outside the area; the keeper stands on his line.
	edge := Pitch_LEFT_PENALTY_AREA_EDGE + SetPiece_PENALTY_WAIT_OFFSET
	if homeTeam {
		edge = Pitch_RIGHT_PENALTY_AREA_EDGE - SetPiece_PENALTY_WAIT_OFFSET
	}
	for i := range e.players {
		player := &e.players[i]
		if e.cfg.Modules.M9Restart || !active(player) || player == taker {
			continue
		}
		if player.IsGoalkeeper && player.IsHomeTeam != homeTeam {
			// Law 14: the defending keeper stays on his goal line.
			lineX := Pitch_PLAYER_MIN_X
			if homeTeam {
				lineX = Pitch_PLAYER_MAX_X
			}
			player.MovementTarget = Vector2F{lineX, Pitch_CENTRE}
			player.Position = player.MovementTarget
			continue
		}
		if player.IsGoalkeeper {
			continue
		}
		var inside bool
		if homeTeam {
			inside = player.Position.X > edge
		} else {
			inside = player.Position.X < edge
		}
		if inside {
			player.MovementTarget.X = edge
		}
	}
	e.clearFlightState()
	if homeTeam {
		e.stats.HomePenalties++
	} else {
		e.stats.AwayPenalties++
	}
	e.stoppageLogs[e.period-1].Penalties++
	e.state = MatchState_PENALTY
	e.setPieceTimer = Timing_PENALTY_DELAY_SECONDS
	if e.cfg.Modules.M9Restart {
		e.setPieceTimer = e.feM9Delay(MatchState_PENALTY, 0.0)
	}
	e.updateTeamPhases()
	event := e.logEvent(MatchEventType_PENALTY)
	event.HasTeam = true
	event.IsHomeTeam = homeTeam
	event.Position = spot
	if taker != nil && taker.Player != nil {
		event.PrimaryPlayerId = taker.Player.GetId()
	}
}

func (e *MatchEngine) placeTaker(taker *MatchPlayer, spot Vector2F) {
	taker.Position = Vector2F{
		hClamp(spot.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
		hClamp(spot.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y),
	}
	taker.MovementTarget = taker.Position
	taker.Velocity = Vector2F{0.0, 0.0}
	// Anyone standing on the spot steps aside for the taker.
	for i := range e.players {
		other := &e.players[i]
		if !active(other) || other == taker {
			continue
		}
		dx := (other.Position.X - taker.Position.X) * Pitch_LENGTH_METRES
		dy := (other.Position.Y - taker.Position.Y) * Pitch_WIDTH_METRES
		gap := math.Sqrt(dx*dx + dy*dy)
		if gap >= Player_MINIMUM_BODY_SEPARATION_METRES {
			continue
		}
		towardCentre := -1.0
		if taker.Position.Y < Pitch_CENTRE {
			towardCentre = 1.0
		}
		away := Vector2F{0.0, towardCentre}
		if gap > EPSILON {
			away = Vector2F{dx / gap, dy / gap}
		}
		push := Player_MINIMUM_BODY_SEPARATION_METRES - gap
		other.Position.X = hClamp(other.Position.X+away.X*push/Pitch_LENGTH_METRES,
			Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X)
		other.Position.Y = hClamp(other.Position.Y+away.Y*push/Pitch_WIDTH_METRES,
			Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)
	}
}

func (e *MatchEngine) restartTaker() *MatchPlayer {
	if e.restartTakerIndex.HasValue && e.restartTakerIndex.Value < len(e.players) {
		return &e.players[e.restartTakerIndex.Value]
	}
	return nil
}

func (e *MatchEngine) setRestartTaker(taker *MatchPlayer) {
	if taker != nil {
		e.restartTakerIndex = hSome(e.slotOf(taker))
	} else {
		e.restartTakerIndex = Optional[int]{}
	}
}

// slotOf is the index of player in e.players (C++ pointer arithmetic). A
// pointer outside the slice yields len(e.players), which restartTaker treats
// as no taker.
func (e *MatchEngine) slotOf(player *MatchPlayer) int {
	for i := range e.players {
		if &e.players[i] == player {
			return i
		}
	}
	return len(e.players)
}

func (e *MatchEngine) bestSetPieceTaker(homeTeam bool, shooting bool) *MatchPlayer {
	var best *MatchPlayer
	bestScore := math.Inf(-1)
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || player.IsHomeTeam != homeTeam || player.IsGoalkeeper || player.IsInjured {
			continue
		}
		var score float64
		if shooting {
			score = player.Shooting*0.75 + player.Passing*0.15 + player.Vision*0.10
		} else {
			score = player.Passing*0.6 + player.Vision*0.4
		}
		if score > bestScore {
			bestScore = score
			best = player
		}
	}
	return best
}

func (e *MatchEngine) dutyTaker(homeTeam bool, duty SetPieceDuty) *MatchPlayer {
	if designated := e.designations[hBoolIndex(homeTeam)][duty]; designated != PlayerID(0) {
		for i := range e.players {
			player := &e.players[i]
			if active(player) && player.IsHomeTeam == homeTeam && !player.IsGoalkeeper &&
				!player.IsInjured && player.Player.GetId() == designated {
				return player
			}
		}
	}
	switch duty {
	case SetPieceDuty_Penalties, SetPieceDuty_FreeKicks:
		return e.bestSetPieceTaker(homeTeam, true)
	case SetPieceDuty_CornersLeft, SetPieceDuty_CornersRight:
		return e.bestSetPieceTaker(homeTeam, false)
	default:
		return nil
	}
}

// setPieceBoxThreat is the anonymous-namespace helper of match_engine.cpp.
func setPieceBoxThreat(candidate *MatchPlayer) float64 {
	roleBias := 0.0
	switch candidate.Player.GetRole() {
	case PlayerRole_ST:
		roleBias = 0.12
	case PlayerRole_CB:
		roleBias = 0.0
	case PlayerRole_LB, PlayerRole_RB:
		roleBias = -0.3
	case PlayerRole_CDM:
		roleBias = -0.15
	default:
	}
	return MatchRules_headerReachMetres(candidate.HeightMetres, candidate.Physicality) + roleBias
}

func (e *MatchEngine) arrangeSetPiece(attackingHome bool, ballPosition Vector2F) {
	// Box targets for the attacking side, in metres from the goal line and
	// from the centre of the goal (positive towards the ball's side).
	goalX := 0.0
	inward := 1.0
	if attackingHome {
		goalX = 1.0
		inward = -1.0
	}
	ballSide := 1.0
	if ballPosition.Y < Pitch_CENTRE {
		ballSide = -1.0
	}
	spot := func(depthMetres, lateralMetres float64) Vector2F {
		return Vector2F{
			goalX + inward*depthMetres/Pitch_LENGTH_METRES,
			Pitch_CENTRE + ballSide*lateralMetres/Pitch_WIDTH_METRES,
		}
	}
	attackSlots := [5]Vector2F{
		spot(5.5, 3.0), spot(6.5, -3.5), spot(10.5, 0.0), spot(8.0, 1.0),
		spot(17.5, -2.0)}

	// Rank attackers by aerial threat; the best go into the box.
	var attackers [11]*MatchPlayer
	attackerCount := 0
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || player.IsHomeTeam != attackingHome ||
			player.IsGoalkeeper || player == e.restartTaker() ||
			attackerCount == len(attackers) {
			continue
		}
		attackers[attackerCount] = player
		attackerCount++
	}
	// Centre-backs and strikers attack the delivery; full-backs and a
	// holding midfielder form the rest defence.
	insertionSort(attackers[:], attackerCount, func(first, second *MatchPlayer) bool {
		a := setPieceBoxThreat(first)
		if b := setPieceBoxThreat(second); a != b {
			return a > b
		}
		return first.Player.GetId() < second.Player.GetId()
	})
	restCount := 0
	if attackerCount > SetPiece_REST_DEFENDERS {
		restCount = attackerCount - SetPiece_REST_DEFENDERS
	}
	boxAttackers := min(len(attackSlots), restCount)
	for index := 0; index < boxAttackers; index++ {
		attackers[index].MovementTarget = attackSlots[index]
	}

	// Defenders: the keeper on his line, markers goal-side of each attacker in
	// the box, and the rest zonal at the near post and the edge of the area.
	var defenders [11]*MatchPlayer
	defenderCount := 0
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || player.IsHomeTeam == attackingHome ||
			defenderCount == len(defenders) {
			continue
		}
		if player.IsGoalkeeper {
			player.MovementTarget = spot(0.8, 0.0)
			continue
		}
		defenders[defenderCount] = player
		defenderCount++
	}
	insertionSort(defenders[:], defenderCount, func(first, second *MatchPlayer) bool {
		a := first.Defending + first.Physicality
		if b := second.Defending + second.Physicality; a != b {
			return a > b
		}
		return first.Player.GetId() < second.Player.GetId()
	})
	next := 0
	for index := 0; index < boxAttackers && next < defenderCount; index, next = index+1, next+1 {
		mark := attackers[index].MovementTarget
		defenders[next].MovementTarget = Vector2F{
			mark.X - inward*SetPiece_MARKING_GOAL_SIDE_OFFSET,
			mark.Y}
	}
	zones := [3]Vector2F{spot(3.0, 2.5), spot(6.0, 0.0), spot(16.0, 0.0)}
	for zone := 0; zone < len(zones) && next < defenderCount; zone, next = zone+1, next+1 {
		defenders[next].MovementTarget = zones[zone]
	}
}

// userRestartPassRequested belongs to play mode, which is not ported: no
// player is ever controlled, so a user restart pass is never requested.
func (e *MatchEngine) userRestartPassRequested() bool {
	return false
}

func (e *MatchEngine) completeRestart() {
	// Restart setup functions own placement and waiting; this function owns the
	// actual release. Preserve the restart state until the kick is executed:
	// passBall uses it for offside exemptions and set-piece classification.
	restartState := e.state
	// FE M9: the kick waits until the restart is legal (players walked to
	// their places), up to a hard cap. The timer keeps running below zero, so
	// its magnitude is the overtime waited.
	if e.cfg.Modules.M9Restart && !e.userRestartPassRequested() && e.setPieceTimer > -feM9MaxOvertime &&
		!e.feM9Ready() {
		return
	}
	taker := e.restartTaker()
	userPass := e.userRestartPassRequested()
	e.restartTakerIndex = Optional[int]{}
	if taker == nil || !active(taker) {
		e.ball.PossessedBy = nil
		e.ball.Velocity = Vector2F{0.0, 0.0}
		e.ball.VelocityZ = 0.0
		e.clearFlightState()
		e.restartIsSetPiece = false
		e.state = MatchState_PLAYING
		e.updateTeamPhases()
		return
	}

	// The taker has walked to the spot; line him up exactly on the ball.
	if restartState != MatchState_KICK_OFF {
		taker.Position = e.ball.Position
		taker.Velocity = Vector2F{0.0, 0.0}
	}
	if restartState == MatchState_PENALTY {
		e.ball.Position = taker.Position
		e.takeShot(taker, Shooting_PENALTY_XG, false, false)
		e.state = MatchState_PLAYING
		e.restartIsSetPiece = false
		e.updateTeamPhases()
		return
	}

	// Corners, free kicks near goal and throw-ins in the attacking third open
	// a set-piece phase for the side taking them.
	restartDepth := 1.0 - e.ball.Position.X
	if taker.IsHomeTeam {
		restartDepth = e.ball.Position.X
	}
	if e.restartIsSetPiece ||
		(restartState == MatchState_THROW_IN && restartDepth >= Rules_HOME_FINAL_THIRD_START) {
		e.setPiecePhaseRemaining = SetPiece_SET_PIECE_PHASE_SECONDS
		e.setPieceHome = taker.IsHomeTeam
	}
	if userPass {
		action := e.controlInput.Action
		e.playControlledPass(taker, action)
		e.controlInput.Action = MatchInputAction_NONE
		e.controlActionRemaining = 0.0
		e.controlledActions[hBoolIndex(taker.IsHomeTeam)]++
	} else {
		e.takeSetPiece(taker, restartState)
	}
	if e.state == restartState {
		e.state = MatchState_PLAYING
	}
	e.restartIsSetPiece = false
	e.updateTeamPhases()
}

func (e *MatchEngine) takeSetPiece(taker *MatchPlayer, restartState MatchState) {
	if restartState == MatchState_CORNER_KICK {
		e.takeCorner(taker)
		return
	}
	if restartState == MatchState_FREE_KICK && e.restartIsSetPiece {
		goalX := 0.0
		if taker.IsHomeTeam {
			goalX = 1.0
		}
		goal := Vector2F{goalX, Pitch_CENTRE}
		goalMetres := distance(e.ball.Position, goal)
		lateralMetres := math.Abs(e.ball.Position.Y-Pitch_CENTRE) * Pitch_WIDTH_METRES
		shootingRange := goalMetres <= SetPiece_DIRECT_FREE_KICK_METRES &&
			lateralMetres <= SetPiece_DIRECT_FREE_KICK_WIDTH_METRES
		if shootChance := hClamp(
			SetPiece_DIRECT_SHOT_BASE+
				taker.Shooting*SetPiece_DIRECT_SHOT_SKILL-
				goalMetres*SetPiece_DIRECT_SHOT_DISTANCE_PENALTY,
			0.0, 0.95); shootingRange && e.randomFloat(0.0, 1.0) < shootChance {
			e.takeDirectFreeKick(taker)
			return
		}
		wide := lateralMetres > SetPiece_CROSS_MIN_WIDTH_METRES ||
			goalMetres > SetPiece_DIRECT_FREE_KICK_METRES
		if wide {
			e.takeCorner(taker)
			return
		}
	}

	passOption := e.choosePassTarget(taker)
	if !passOption.HasValue && e.playHalfMinutes > 0 {
		// A restart must release the ball even when no lane meets the AI's
		// open-play safety threshold. Find the nearest teammate as an outlet.
		var outlet *MatchPlayer
		nearest := math.Inf(1)
		for i := range e.players {
			candidate := &e.players[i]
			if !active(candidate) || candidate == taker ||
				candidate.IsHomeTeam != taker.IsHomeTeam || candidate.IsGoalkeeper {
				continue
			}
			gap := distance(taker.Position, candidate.Position)
			if gap > EPSILON && gap < nearest {
				outlet = candidate
				nearest = gap
			}
		}
		if outlet != nil {
			passOption = hSome(e.evaluatePassOption(taker, outlet))
		}
	}
	if passOption.HasValue {
		e.passBall(taker, &passOption.Value, restartState == MatchState_GOAL_KICK)
	} else {
		e.setPossession(taker)
	}
}

func (e *MatchEngine) takeCorner(taker *MatchPlayer) {
	// Routine: short, near post, far post or penalty spot, delivered at head
	// height to the attacker assigned to that zone.
	var target *MatchPlayer
	roll := e.randomFloat(0.0, 1.0)
	if roll < SetPiece_SHORT_CORNER_CHANCE {
		shortOption := e.choosePassTarget(taker)
		if shortOption.HasValue &&
			shortOption.Value.PassDistance < SetPiece_SHORT_CORNER_MAX_DISTANCE_METRES {
			groundPass := shortOption.Value
			groundPass.Lofted = false
			e.passBall(taker, &groundPass, false)
			return
		}
	}
	goalX := 0.0
	if taker.IsHomeTeam {
		goalX = 1.0
	}
	goal := Vector2F{goalX, Pitch_CENTRE}
	bestScore := math.Inf(-1)
	nearPostShare := SetPiece_NEAR_POST_CHANCE
	nearPost := roll < SetPiece_SHORT_CORNER_CHANCE+nearPostShare
	for i := range e.players {
		candidate := &e.players[i]
		if !active(candidate) || candidate == taker ||
			candidate.IsHomeTeam != taker.IsHomeTeam || candidate.IsGoalkeeper {
			continue
		}
		if !inPenaltyArea(candidate.MovementTarget, !taker.IsHomeTeam) {
			continue
		}
		targetMetres := distance(candidate.MovementTarget, goal)
		sideSign := 1.0
		if e.ball.Position.Y < Pitch_CENTRE {
			sideSign = -1.0
		}
		sideMetres := (candidate.MovementTarget.Y - Pitch_CENTRE) * sideSign * Pitch_WIDTH_METRES
		threat := MatchRules_headerReachMetres(candidate.HeightMetres, candidate.Physicality)
		zone := -sideMetres
		if nearPost {
			zone = sideMetres
		}
		score := threat*SetPiece_TARGET_THREAT_WEIGHT + zone*0.15 -
			targetMetres*0.05 +
			e.randomFloat(0.0, SetPiece_TARGET_RANDOMNESS)
		if score > bestScore {
			bestScore = score
			target = candidate
		}
	}
	if target == nil {
		if fallback := e.choosePassTarget(taker); fallback.HasValue {
			e.passBall(taker, &fallback.Value, true)
		} else {
			e.setPossession(taker)
		}
		return
	}
	delivery := e.evaluatePassOption(taker, target)
	delivery.TargetPoint = target.MovementTarget
	delivery.PassDistance = distance(taker.Position, delivery.TargetPoint)
	delivery.Lofted = true
	delivery.Intent = PassIntent_CROSS
	e.passBall(taker, &delivery, true)
}

func (e *MatchEngine) takeDirectFreeKick(taker *MatchPlayer) {
	// The wall blocks a share of attempts; the rest are struck over it with
	// a little less precision, and the keeper's view is partly screened.
	if wallChance := hClamp(
		SetPiece_WALL_BLOCK_BASE-taker.Shooting*SetPiece_WALL_BLOCK_SKILL,
		0.05, 0.6); e.randomFloat(0.0, 1.0) < wallChance {
		e.takeShot(taker, -1.0, false, false)
		outward := 1.0
		if taker.IsHomeTeam {
			outward = -1.0
		}
		speed := length(e.ball.Velocity) * SetPiece_WALL_REBOUND_SPEED_SHARE
		e.ball.TouchAttempts = 0
		var wallPlayer *MatchPlayer
		best := math.MaxFloat64
		for i := range e.players {
			player := &e.players[i]
			if !active(player) || player.IsHomeTeam == taker.IsHomeTeam || player.IsGoalkeeper {
				continue
			}
			d := distance(player.Position, e.ball.Position)
			if d < best {
				best = d
				wallPlayer = player
			}
		}
		e.ball.IsShot = false
		e.ball.ShotOnTarget = false
		e.ball.ShotSaveResolved = true
		// Braced initialisation: the two draws happen left to right.
		vx := outward * speed * e.randomFloat(0.3, 1.0)
		vy := speed * e.randomFloat(-0.8, 0.8)
		e.ball.Velocity = Vector2F{vx, vy}
		e.ball.VelocityZ = e.randomFloat(0.0, Aerial_HEADER_LIFT)
		if wallPlayer != nil {
			e.ball.Position = wallPlayer.Position
			e.ball.LastPossessor = wallPlayer.Player
			e.ball.Kicker = wallPlayer.Player
			e.ball.KickerLockout = Passing_KICKER_LOCKOUT_SECONDS
			e.statsOf(wallPlayer).Clearances++
			e.logEventActor(MatchEventType_SHOT_BLOCKED, wallPlayer).Detail = MatchEventDetail_WALL
		}
		return
	}
	e.takeShot(taker, SetPiece_DIRECT_FREE_KICK_XG, false, true)
	if keeper := e.findGoalkeeper(!taker.IsHomeTeam); keeper != nil {
		e.keepers[hBoolIndex(keeper.IsHomeTeam)].ReactionRemaining += SetPiece_WALL_SCREEN_SECONDS
	}
}

// b6PlayerOf is the C++ `taker ? taker->player : nullptr`.
func b6PlayerOf(player *MatchPlayer) *Player {
	if player != nil {
		return player.Player
	}
	return nil
}
