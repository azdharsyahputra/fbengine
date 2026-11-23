// Ported from Player12 src/model/match_engine.cpp lines 8999–9512 (GPL-3.0)

package engine

import "math"

func (e *MatchEngine) getSubstitutionsUsed(homeTeam bool) int {
	if homeTeam {
		return e.stats.HomeSubstitutions
	}
	return e.stats.AwaySubstitutions
}

func (e *MatchEngine) getSubstitutionWindowsUsed(homeTeam bool) int {
	if homeTeam {
		return e.homeSubstitutionWindows
	}
	return e.awaySubstitutionWindows
}

func (e *MatchEngine) canSubstitute(homeTeam bool) bool {
	if e.state == MatchState_FULL_TIME || e.state == MatchState_PENALTY_SHOOTOUT {
		return false
	}
	if e.getSubstitutionsUsed(homeTeam) >= e.maxSubstitutions() {
		return false
	}
	// Half-time changes and further changes in an already used stoppage do
	// not consume a new window.
	lastWindow := e.awayLastWindowStoppage
	if homeTeam {
		lastWindow = e.homeLastWindowStoppage
	}
	sameStoppage := lastWindow == e.stoppageSequence &&
		(e.state != MatchState_PLAYING || e.manualSubstitutionStep == e.stepCounter)
	windows := Substitution_MAX_WINDOWS
	if e.extraTimeReached {
		windows += Rules_EXTRA_TIME_SUBSTITUTIONS
	}
	return e.state == MatchState_HALF_TIME || e.getSubstitutionWindowsUsed(homeTeam) < windows ||
		sameStoppage
}

func (e *MatchEngine) setAutoSubstitutions(home bool, away bool) {
	if home != e.homeAutoSubstitutions || away != e.awayAutoSubstitutions {
		e.inputRevision++
	}
	e.homeAutoSubstitutions = home
	e.awayAutoSubstitutions = away
}

func (e *MatchEngine) substitutePlayer(outPlayerId uint32, inPlayer *Player, slot Optional[int]) bool {
	if inPlayer == nil || e.state == MatchState_FULL_TIME {
		return false
	}
	command := NewMatchCommandRecord()
	command.Type = MatchCommandType_SUBSTITUTION
	command.HomeTeam = b6ContainsPlayer(e.homeBench, inPlayer)
	command.Player = outPlayerId
	command.Incoming = inPlayer.GetId()
	command.Slot = slot
	return e.recordCommand(&command)
}

func (e *MatchEngine) performSubstitution(outgoing *MatchPlayer, inPlayer *Player, reason SubstitutionReason) bool {
	if inPlayer == nil || !active(outgoing) || !e.canSubstitute(outgoing.IsHomeTeam) {
		return false
	}
	homeTeam := outgoing.IsHomeTeam
	if e.state != MatchState_HALF_TIME {
		lastWindow := &e.awayLastWindowStoppage
		if homeTeam {
			lastWindow = &e.homeLastWindowStoppage
		}
		if *lastWindow != e.stoppageSequence {
			*lastWindow = e.stoppageSequence
			if homeTeam {
				e.homeSubstitutionWindows++
			} else {
				e.awaySubstitutionWindows++
			}
		}
		e.stoppageLogs[e.period-1].Substitutions++
		e.extendRestart(Timing_SUBSTITUTION_DELAY_SECONDS)
	}
	if homeTeam {
		e.stats.HomeSubstitutions++
	} else {
		e.stats.AwaySubstitutions++
	}
	bench := &e.awayBench
	if homeTeam {
		bench = &e.homeBench
	}
	// std::erase(bench, inPlayer)
	kept := (*bench)[:0]
	for _, candidate := range *bench {
		if candidate != inPlayer {
			kept = append(kept, candidate)
		}
	}
	*bench = kept

	leaving := e.statsOf(outgoing)
	leaving.SubstitutedOff = true
	leaving.Condition = outgoing.Stamina

	hadPossession := e.ball.PossessedBy == outgoing.Player
	outgoingId := outgoing.Player.GetId()
	if !b6ContainsPlayer(e.squad, inPlayer) {
		e.squad = append(e.squad, inPlayer)
	}
	if incomingKeeper := inPlayer.GetRole() == PlayerRole_GK; incomingKeeper && !outgoing.IsGoalkeeper {
		// A replacement keeper takes over from any emergency keeper.
		for i := range e.players {
			teammate := &e.players[i]
			if teammate.IsHomeTeam != homeTeam || !teammate.IsGoalkeeper {
				continue
			}
			teammate.IsGoalkeeper = false
			// An emergency keeper goes back out to the slot the new one frees.
			if active(teammate) && outgoing.FormationSlot >= 0 {
				teammate.FormationSlot = outgoing.FormationSlot
				e.placeFormation(homeTeam)
			}
		}
		keeperX := 1.0 - Pitch_LINEUP_GOALKEEPER_X
		if homeTeam {
			keeperX = Pitch_LINEUP_GOALKEEPER_X
		}
		outgoing.BasePosition = Vector2F{keeperX, Pitch_CENTRE}
		outgoing.IsGoalkeeper = true
	}
	outgoing.Player = inPlayer
	e.loadAttributes(outgoing, inPlayer)
	outgoing.Stamina = 1.0
	for _, entry := range e.benchConditions {
		if entry.First == inPlayer.GetId() {
			outgoing.Stamina = entry.Second
			break
		}
	}
	outgoing.SprintReserve = 1.0
	outgoing.IsInjured = false
	outgoing.YellowCards = 0
	outgoing.ActionCooldown = Player_SUBSTITUTION_SETTLE_SECONDS
	outgoing.StatsIndex = e.addPlayerStats(outgoing, false)
	if hadPossession {
		e.ball.PossessedBy = inPlayer
	}
	if e.ball.LastPossessor != nil && e.ball.LastPossessor.GetId() == outgoingId {
		e.ball.LastPossessor = inPlayer
	}

	e.substitutions = append(e.substitutions, MatchSubstitution{
		TimeMinute:       e.matchTimeMinutes,
		Period:           e.period,
		IsHomeTeam:       homeTeam,
		OutgoingPlayerId: outgoingId,
		IncomingPlayerId: inPlayer.GetId(),
		Reason:           reason,
	})
	event := e.logEventActor(MatchEventType_SUBSTITUTION, outgoing)
	event.SecondaryPlayerId = outgoingId
	return true
}

func (e *MatchEngine) chooseReplacement(homeTeam bool, role PlayerRole, wantGoalkeeper bool) *Player {
	bench := e.awayBench
	if homeTeam {
		bench = e.homeBench
	}
	var best *Player
	bestScore := math.Inf(-1)
	for _, candidate := range bench {
		if candidate == nil {
			continue
		}
		if keeper := candidate.GetRole() == PlayerRole_GK; keeper != wantGoalkeeper {
			continue
		}
		fit := 0.0
		if roleGroup(candidate.GetRole()) == roleGroup(role) {
			fit = Substitution_ROLE_FIT_BONUS
		}
		score := fit + candidate.GetOverall(e.statsConfig)/Player_RATING_SCALE
		if score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	return best
}

func (e *MatchEngine) runAiSubstitutions() {
	if e.homeLastAiReviewStoppage != e.stoppageSequence {
		e.homeLastAiReviewStoppage = e.stoppageSequence
		e.runAiSubstitutionsFor(true)
		if e.homeAutoSubstitutions {
			e.runAiTouchline(true)
		}
	}
	if e.awayLastAiReviewStoppage != e.stoppageSequence {
		e.awayLastAiReviewStoppage = e.stoppageSequence
		e.runAiSubstitutionsFor(false)
		if e.awayAutoSubstitutions {
			e.runAiTouchline(false)
		}
	}
}

func (e *MatchEngine) setMedicalFlags(playerId PlayerID, flags uint8) {
	for i := range e.medicalFlags {
		if e.medicalFlags[i].First == playerId {
			e.medicalFlags[i].Second = flags
			return
		}
	}
	if flags != 0 {
		e.medicalFlags = append(e.medicalFlags, Pair[PlayerID, uint8]{playerId, flags})
	}
}

func (e *MatchEngine) runMedicalSubstitutions(homeTeam bool) {
	if !e.b6AutoManaged(homeTeam) || e.state == MatchState_PENALTY {
		return
	}
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || player.IsHomeTeam != homeTeam || player.IsGoalkeeper {
			continue
		}
		id := player.Player.GetId()
		// The limit counts his own minutes on the pitch: a flagged substitute
		// starts from nothing when he comes on.
		entry := -1
		for index := range e.medicalFlags {
			if e.medicalFlags[index].First == id {
				entry = index
				break
			}
		}
		if entry < 0 ||
			!MedicalCentre_substitutionDue(e.medicalFlags[entry].Second, int(e.statsOf(player).MinutesPlayed)) {
			continue
		}
		if !e.canSubstitute(homeTeam) {
			return
		}
		// Like for like, as for a tired player.
		if replacement := e.chooseReplacement(homeTeam, player.Player.GetRole(), false); replacement != nil {
			e.performSubstitution(player, replacement, SubstitutionReason_FATIGUE)
		}
	}
}

// b6Need is the local struct Need of runAiSubstitutionsFor.
type b6Need struct {
	player      *MatchPlayer
	score       float64
	reason      SubstitutionReason
	wantedGroup int
}

func b6NewNeed() b6Need {
	return b6Need{reason: SubstitutionReason_FATIGUE, wantedGroup: 2}
}

func (e *MatchEngine) runAiSubstitutionsFor(homeTeam bool) {
	// Injured players leave at the first stoppage: replaced when possible, or
	// the team continues a player short.
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || player.IsHomeTeam != homeTeam || !player.IsInjured {
			continue
		}
		if autoManaged := e.b6AutoManaged(homeTeam); !autoManaged {
			continue
		}
		// Treatment and the walk off hold up the restart.
		e.extendRestart(Timing_INJURY_DELAY_SECONDS)
		var replacement *Player
		if e.canSubstitute(homeTeam) {
			replacement = e.chooseReplacement(homeTeam, player.Player.GetRole(), player.IsGoalkeeper)
		}
		if replacement == nil && player.IsGoalkeeper && e.canSubstitute(homeTeam) {
			replacement = e.chooseReplacement(homeTeam, player.Player.GetRole(), false)
		}
		if replacement == nil ||
			!e.performSubstitution(player, replacement, SubstitutionReason_INJURY) {
			e.removeFromPitch(player)
		}
	}

	if len(e.medicalFlags) != 0 {
		e.runMedicalSubstitutions(homeTeam)
	}
	if !e.b6AutoManaged(homeTeam) ||
		e.state == MatchState_PENALTY || e.period < 2 ||
		e.matchTimeMinutes < Substitution_EARLIEST_TACTICAL_MINUTE {
		return
	}

	goalDifference := e.awayScore - e.homeScore
	if homeTeam {
		goalDifference = e.homeScore - e.awayScore
	}
	minute := e.matchTimeMinutes
	fatigueThreshold := Substitution_FATIGUE_THRESHOLD
	if minute >= Substitution_LATE_GAME_MINUTE {
		fatigueThreshold += Substitution_FATIGUE_THRESHOLD_LATE_GAIN
	}
	picks := [3]b6Need{b6NewNeed(), b6NewNeed(), b6NewNeed()}
	lateShare := 1.0
	if minute >= Substitution_LATE_GAME_MINUTE {
		lateShare = Substitution_LATE_NEED_SHARE
	}
	minimumNeed := Substitution_MINIMUM_NEED * lateShare
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || player.IsHomeTeam != homeTeam || player.IsGoalkeeper {
			continue
		}
		role := player.Player.GetRole()
		need := b6Need{player, 0.0, SubstitutionReason_FATIGUE, roleGroup(role)}
		attackerFactor := 1.0
		if isAttackingRole(role) {
			attackerFactor = Substitution_ATTACKER_FATIGUE_NEED_FACTOR
		}
		need.score = math.Max(0.0, fatigueThreshold-player.Stamina)*
			Substitution_FATIGUE_NEED_SCALE*attackerFactor +
			(minute-Substitution_EARLIEST_TACTICAL_MINUTE)*
				Substitution_MINUTE_NEED_GAIN*
				(1.0-player.Stamina)
		if isAttackingRole(role) && minute >= Substitution_ATTACKER_ROTATION_MINUTE {
			need.score += (minute - Substitution_ATTACKER_ROTATION_MINUTE) *
				Substitution_ATTACKER_ROTATION_NEED_PER_MINUTE *
				(1.0 - player.Stamina)
		}
		if player.YellowCards > 0 && !isAttackingRole(role) && minute >= Substitution_CARD_RISK_MINUTE {
			need.score += Substitution_CARD_RISK_NEED
			need.reason = SubstitutionReason_CARD_RISK
		}
		if goalDifference < 0 && isDefensiveRole(role) && minute >= Substitution_TRAILING_CHASE_MINUTE {
			need.score += Substitution_TACTICAL_NEED
			need.reason = SubstitutionReason_TACTICAL
			need.wantedGroup = 3
		} else if goalDifference > 0 && isAttackingRole(role) && minute >= Substitution_LEADING_PROTECT_MINUTE {
			need.score += Substitution_TACTICAL_NEED
			need.reason = SubstitutionReason_TACTICAL
			need.wantedGroup = 1
		}
		if need.score < minimumNeed*Substitution_WINDOW_NEED_SHARE {
			continue
		}
		slot := -1
		for index := range picks {
			if need.score > picks[index].score {
				slot = index
				break
			}
		}
		if slot < 0 {
			continue
		}
		// std::move_backward(slot, prev(end), end)
		for index := len(picks) - 1; index > slot; index-- {
			picks[index] = picks[index-1]
		}
		picks[slot] = need
	}

	// Keep one change in reserve for late injuries until the final minutes.
	reserve := 0
	if minute < Substitution_LATE_GAME_MINUTE {
		reserve = 1
	}
	// The clearest need opens a window; a second or third change joins it at
	// a lower need, as managers make double changes to save windows.
	if picks[0].score < minimumNeed {
		return
	}
	for _, need := range picks {
		if need.player == nil || !e.canSubstitute(homeTeam) {
			break
		}
		if e.getSubstitutionsUsed(homeTeam) >= e.maxSubstitutions()-reserve {
			break
		}
		wantedRole := need.player.Player.GetRole()
		if need.wantedGroup == 3 {
			wantedRole = PlayerRole_ST
		} else if need.wantedGroup == 1 {
			wantedRole = PlayerRole_CB
		}
		replacement := e.chooseReplacement(homeTeam, wantedRole, false)
		if replacement == nil {
			break
		}
		e.performSubstitution(need.player, replacement, need.reason)
	}
}

func (e *MatchEngine) checkInjuries() {
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || player.IsInjured {
			continue
		}
		speedRatio := length(player.Velocity) / math.Max(player.MaxSpeed, EPSILON)
		tiredness := 1.0 - player.Stamina
		hazard := Injury_BASE_HAZARD_PER_MINUTE *
			(1.0 + tiredness*tiredness*Injury_FATIGUE_HAZARD_MULTIPLIER) *
			(1.0 + speedRatio*Injury_INTENSITY_HAZARD_MULTIPLIER)
		probability := 1.0 - math.Exp(-hazard*Injury_CHECK_INTERVAL_MINUTES)
		if e.incidentRoll() < probability {
			e.injurePlayer(player, false)
		}
	}
}

func (e *MatchEngine) injurePlayer(player *MatchPlayer, fromContact bool) {
	if !active(player) || player.IsInjured {
		return
	}
	player.IsInjured = true
	e.statsOf(player).Injured = true
	if player.IsHomeTeam {
		e.stats.HomeInjuries++
	} else {
		e.stats.AwayInjuries++
	}
	e.stoppageLogs[e.period-1].Injuries++
	event := e.logEventActor(MatchEventType_INJURY, player)
	if fromContact {
		event.Detail = MatchEventDetail_CONTACT
	}
	event.Position = player.Position
	// A player who cannot run no longer carries the ball forward.
	if e.ball.PossessedBy == player.Player {
		player.ActionCooldown = 0.0
	}
}

func (e *MatchEngine) removeFromPitch(player *MatchPlayer) {
	if !active(player) {
		return
	}
	homeTeam := player.IsHomeTeam
	vacated := player.BasePosition
	vacatedSlot := player.FormationSlot
	wasKeeper := player.IsGoalkeeper
	e.statsOf(player).Condition = player.Stamina
	if e.ball.PossessedBy == player.Player {
		e.ball.PossessedBy = nil
		e.ball.Velocity = Vector2F{0.0, 0.0}
	}
	// Park beside the dugouts, clear of play and of each other.
	parked := 0
	for i := range e.players {
		other := &e.players[i]
		if other.Player != nil && !other.OnPitch {
			parked++
		}
	}
	player.OnPitch = false
	player.IsGoalkeeper = false
	player.IsPressing = false
	player.IsMakingRun = false
	player.Intent = PlayerIntent_HOLD_SHAPE
	player.Velocity = Vector2F{0.0, 0.0}
	side := 1.0
	if homeTeam {
		side = -1.0
	}
	player.Position = Vector2F{
		Pitch_CENTRE + side*(Rules_PARKED_PLAYER_OFFSET+float64(parked)*Rules_PARKED_PLAYER_SPACING),
		Pitch_PLAYER_MIN_Y,
	}
	player.MovementTarget = player.Position
	if e.restartTaker() == player {
		replacement := e.findClosestPlayer(e.ball.Position, homeTeam, false)
		e.setRestartTaker(replacement)
		if replacement != nil && e.state != MatchState_PLAYING {
			e.ball.PossessedBy = replacement.Player
		}
	}
	if wasKeeper {
		e.ensureGoalkeeper(homeTeam)
	}
	if wasKeeper {
		e.rebalanceShape(homeTeam, vacated, -1)
	} else {
		e.rebalanceShape(homeTeam, vacated, vacatedSlot)
	}

	// Law 3: a match cannot continue with fewer than seven players a side.
	remaining := 0
	for i := range e.players {
		other := &e.players[i]
		if active(other) && other.IsHomeTeam == homeTeam {
			remaining++
		}
	}
	if remaining < Rules_MINIMUM_PLAYERS && !e.abandoned {
		e.abandoned = true
		e.logEvent(MatchEventType_INFO).Detail = MatchEventDetail_ABANDONED
		e.period = max(e.period, 2)
		e.endPeriod()
	}
}

func (e *MatchEngine) rebalanceShape(homeTeam bool, vacatedBase Vector2F, vacatedSlot int8) {
	// A side reduced to ten fills a defensive hole with the nearest midfielder
	// and drops a little deeper overall.
	ownDepth := 1.0 - vacatedBase.X
	if homeTeam {
		ownDepth = vacatedBase.X
	}
	if ownDepth < Rules_DEFENSIVE_SLOT_DEPTH {
		var cover *MatchPlayer
		best := math.MaxFloat64
		for i := range e.players {
			player := &e.players[i]
			if !active(player) || player.IsHomeTeam != homeTeam || player.IsGoalkeeper {
				continue
			}
			depth := 1.0 - player.BasePosition.X
			if homeTeam {
				depth = player.BasePosition.X
			}
			if depth <= ownDepth+EPSILON {
				continue
			}
			d := distance(player.BasePosition, vacatedBase)
			if d < best {
				best = d
				cover = player
			}
		}
		if cover != nil {
			cover.BasePosition = vacatedBase
			cover.FormationSlot = vacatedSlot
		}
	}
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || player.IsHomeTeam != homeTeam || player.IsGoalkeeper {
			continue
		}
		side := 1.0
		if homeTeam {
			side = -1.0
		}
		player.BasePosition.X = hClamp(
			player.BasePosition.X+side*Rules_SHORT_HANDED_DROP,
			Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X)
	}
}

func (e *MatchEngine) ensureGoalkeeper(homeTeam bool) {
	if e.findGoalkeeper(homeTeam) != nil {
		return
	}
	if autoManaged := e.b6AutoManaged(homeTeam); autoManaged && e.canSubstitute(homeTeam) {
		if keeper := e.chooseReplacement(homeTeam, PlayerRole_GK, true); keeper != nil {
			// Sacrifice the most advanced outfield player for the new keeper.
			var sacrificed *MatchPlayer
			mostAdvanced := math.Inf(-1)
			for i := range e.players {
				player := &e.players[i]
				if !active(player) || player.IsHomeTeam != homeTeam {
					continue
				}
				depth := 1.0 - player.BasePosition.X
				if homeTeam {
					depth = player.BasePosition.X
				}
				if depth > mostAdvanced {
					mostAdvanced = depth
					sacrificed = player
				}
			}
			if sacrificed != nil &&
				e.performSubstitution(sacrificed, keeper, SubstitutionReason_GOALKEEPER_REPLACEMENT) {
				return
			}
		}
	}
	// Otherwise the outfield player best with his hands goes in goal.
	var emergency *MatchPlayer
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || player.IsHomeTeam != homeTeam {
			continue
		}
		if emergency == nil || player.Goalkeeping > emergency.Goalkeeping {
			emergency = player
		}
	}
	if emergency == nil {
		return
	}
	emergency.IsGoalkeeper = true
	keeperX := 1.0 - Pitch_LINEUP_GOALKEEPER_X
	if homeTeam {
		keeperX = Pitch_LINEUP_GOALKEEPER_X
	}
	emergency.BasePosition = Vector2F{keeperX, Pitch_CENTRE}
}

// b6AutoManaged is the C++ `homeTeam ? homeAutoSubstitutions :
// awayAutoSubstitutions`.
func (e *MatchEngine) b6AutoManaged(homeTeam bool) bool {
	if homeTeam {
		return e.homeAutoSubstitutions
	}
	return e.awayAutoSubstitutions
}

// b6ContainsPlayer is std::ranges::find(players, player) != end.
func b6ContainsPlayer(players []*Player, player *Player) bool {
	for _, candidate := range players {
		if candidate == player {
			return true
		}
	}
	return false
}
