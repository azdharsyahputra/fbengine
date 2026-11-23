// Ported from Player12 src/model/match_engine.cpp lines 516–1336 (GPL-3.0)

package engine

import (
	"math"
	"slices"
)

// NewMatchEngine builds a match with a non-deterministic seed (C++ 516).
func NewMatchEngine(home, away *Lineup, homeStrat, awayStrat *Strategy, config *StatsConfig) *MatchEngine {
	return NewMatchEngineWithSeed(home, away, homeStrat, awayStrat, config, makeNonBlockingMatchSeed())
}

// NewMatchEngineWithSeed builds a deterministic match (C++ 524) with the
// default EngineConfig.
func NewMatchEngineWithSeed(home, away *Lineup, homeStrat, awayStrat *Strategy, config *StatsConfig, seed uint32) *MatchEngine {
	return NewMatchEngineWithConfig(home, away, homeStrat, awayStrat, config, seed, DefaultConfig(), Diagnostics{})
}

// NewMatchEngineWithConfig builds a deterministic match owning cfg and diag
// (ours): the configuration is fixed before any construction step reads it.
func NewMatchEngineWithConfig(home, away *Lineup, homeStrat, awayStrat *Strategy, config *StatsConfig, seed uint32, cfg EngineConfig, diag Diagnostics) *MatchEngine {
	e := &MatchEngine{}
	e.hInitDefaults()
	e.cfg = cfg
	e.diag = diag
	e.statsConfig = config
	e.homeStrategy = homeStrat.Clone()
	e.awayStrategy = awayStrat.Clone()
	e.rng = newMt19937(seed)
	e.incidentRng = newMt19937(uint32(splitMix64(uint64(seed))))
	e.matchSeed = seed
	e.homeBench = slices.Clone(home.GetReserves())
	e.awayBench = slices.Clone(away.GetReserves())
	e.designations = [2]SetPieceDesignations{home.GetDesignations(), away.GetDesignations()}

	e.players = make([]MatchPlayer, 0, 22)
	e.playerStats = make([]PlayerMatchStats, 0, 36)
	e.events = make([]MatchEvent, 0, 256)
	e.computeLevel(home, away)
	e.initializePlayers(home, true)
	e.initializePlayers(away, false)
	e.slotAnchors = [2][]Vector2F{slices.Clone(e.formations[0]), slices.Clone(e.formations[1])}
	e.resolveTactics(true)
	e.resolveTactics(false)
	for i := range e.players {
		e.squad = append(e.squad, e.players[i].Player)
	}
	e.squad = append(e.squad, e.homeBench...)
	e.squad = append(e.squad, e.awayBench...)
	e.homeBench = b1eraseNil(e.homeBench)
	e.awayBench = b1eraseNil(e.awayBench)

	e.refreshTargetBlends()
	e.computeUnderdogShares()
	e.refreshEffectiveSliders()
	e.assignMarks()
	e.refereeDraw = gaussian(&e.incidentRng)
	e.refereeStrictness = e.drawRefereeStrictness()

	e.setupKickOff(true)
	e.updateTeamPhases()
	e.logEvent(MatchEventType_KICK_OFF)
	e.describePendingEvents()
	return e
}

// b1eraseNil is std::erase(vector, nullptr).
func b1eraseNil(players []*Player) []*Player {
	return slices.DeleteFunc(players, func(player *Player) bool { return player == nil })
}

func (e *MatchEngine) drawRefereeStrictness() float64 {
	mean := e.matchContext.RefereeStrictnessMean
	return hClamp(mean+e.refereeDraw*e.matchContext.RefereeStrictnessSd,
		Discipline_MIN_STRICTNESS*mean, Discipline_MAX_STRICTNESS*mean)
}

func (e *MatchEngine) setMatchContext(context MatchContext) {
	if e.stepCounter != 0 {
		return
	}
	finite := func(value, fallback float64) float64 {
		if hIsFinite(value) {
			return value
		}
		return fallback
	}
	defaults := NewMatchContext()
	e.matchContext.GoalRateScale = hClamp(finite(context.GoalRateScale, 1.0),
		Context_MIN_GOAL_RATE_SCALE, Context_MAX_GOAL_RATE_SCALE)
	e.matchContext.RefereeStrictnessMean = hClamp(finite(context.RefereeStrictnessMean, 1.0),
		Context_MIN_REFEREE_STRICTNESS, Context_MAX_REFEREE_STRICTNESS)
	e.matchContext.RefereeStrictnessSd = hClamp(
		finite(context.RefereeStrictnessSd, defaults.RefereeStrictnessSd), 0.0, Context_MAX_REFEREE_SD)
	e.matchContext.HomeAdvantageScale = hClamp(finite(context.HomeAdvantageScale, 1.0), 0.0,
		Context_MAX_HOME_ADVANTAGE_SCALE)
	e.finishingPrecision = math.Pow(e.matchContext.GoalRateScale, Context_FINISHING_EXPONENT)
	e.refereeStrictness = e.drawRefereeStrictness()
	// The home crowd's lift is part of the loaded attributes.
	for i := range e.players {
		player := &e.players[i]
		if player.Player != nil && player.IsHomeTeam {
			e.loadAttributes(player, player.Player)
		}
	}
	e.inputRevision++
}

func (e *MatchEngine) initializePlayers(lineup *Lineup, isHomeTeam bool) {
	addPlayer := func(player *Player, position Vector2F) {
		if player == nil {
			return
		}
		if !isHomeTeam {
			position.X = 1.0 - position.X
		}

		matchPlayer := NewMatchPlayer()
		matchPlayer.Player = player
		matchPlayer.IsHomeTeam = isHomeTeam
		matchPlayer.Position = position
		matchPlayer.BasePosition = position
		matchPlayer.MovementTarget = position
		if isHomeTeam {
			matchPlayer.FacingAngle = 0.0
		} else {
			matchPlayer.FacingAngle = math.Pi
		}
		matchPlayer.TargetAngle = matchPlayer.FacingAngle
		e.loadAttributes(&matchPlayer, player)
		matchPlayer.StatsIndex = e.addPlayerStats(&matchPlayer, true)
		e.players = append(e.players, matchPlayer)
	}

	addPlayer(lineup.GetGoalkeeper(), Vector2F{Pitch_LINEUP_GOALKEEPER_X, Pitch_CENTRE})
	if lineup.GetGoalkeeper() != nil && len(e.players) > 0 {
		e.players[len(e.players)-1].IsGoalkeeper = true
	}
	team := hBoolIndex(isHomeTeam)
	for _, positioned := range lineup.GetOutfieldPlayers() {
		before := len(e.players)
		addPlayer(positioned.Player, positioned.Position)
		if len(e.players) > before {
			e.players[len(e.players)-1].FormationSlot = int8(len(e.formations[team]))
		}
		e.formations[team] = append(e.formations[team], positioned.Position)
	}
}

func (e *MatchEngine) loadAttributes(matchPlayer *MatchPlayer, player *Player) {
	// Cache the named career stats once, after match-level normalization.
	matchPlayer.Pace = e.attribute(player, "Pace")
	matchPlayer.Shooting = e.attribute(player, "Shooting")
	matchPlayer.Passing = e.attribute(player, "Passing")
	matchPlayer.Dribbling = e.attribute(player, "Dribbling")
	matchPlayer.Defending = e.attribute(player, "Defending")
	matchPlayer.Goalkeeping = e.attribute(player, "Goalkeeping")
	matchPlayer.Physicality = e.attribute(player, "Physicality")
	matchPlayer.Endurance = e.attribute(player, "Stamina")
	matchPlayer.Vision = e.attribute(player, "Vision")
	if matchPlayer.IsHomeTeam {
		BONUS := Rules_HOME_ATTRIBUTE_BONUS * e.matchContext.HomeAdvantageScale
		for _, value := range []*float64{&matchPlayer.Shooting, &matchPlayer.Passing,
			&matchPlayer.Dribbling, &matchPlayer.Defending,
			&matchPlayer.Goalkeeping, &matchPlayer.Vision} {
			*value = math.Min(1.0, *value+BONUS)
		}
	}
	if player != nil {
		matchPlayer.HeightMetres = MatchRules_playerHeightMetres(player.GetHeight())
	} else {
		matchPlayer.HeightMetres = Units_DEFAULT_PLAYER_HEIGHT_METRES
	}
	matchPlayer.MaxSpeed = Player_TOP_SPEED_BASE + matchPlayer.Pace*Player_TOP_SPEED_PACE
	matchPlayer.Acceleration = Player_ACCELERATION_BASE + matchPlayer.Pace*Player_ACCELERATION_PACE +
		matchPlayer.Physicality*Player_ACCELERATION_PHYSICALITY
	matchPlayer.Braking = Player_BRAKING_BASE + matchPlayer.Physicality*Player_BRAKING_PHYSICALITY
}

func (e *MatchEngine) addPlayerStats(matchPlayer *MatchPlayer, started bool) int {
	entry := NewPlayerMatchStats()
	if matchPlayer.Player != nil {
		entry.PlayerId = matchPlayer.Player.GetId()
		entry.Role = matchPlayer.Player.GetRole()
	} else {
		entry.PlayerId = 0
		entry.Role = PlayerRole_UNKNOWN
	}
	entry.IsHomeTeam = matchPlayer.IsHomeTeam
	entry.Started = started
	entry.SubstitutedOn = !started
	entry.Condition = matchPlayer.Stamina
	e.playerStats = append(e.playerStats, entry)
	return len(e.playerStats) - 1
}

func (e *MatchEngine) statsOf(matchPlayer *MatchPlayer) *PlayerMatchStats {
	return &e.playerStats[matchPlayer.StatsIndex]
}

func (e *MatchEngine) findPlayerStats(playerId PlayerID) *PlayerMatchStats {
	for i := range e.playerStats {
		if e.playerStats[i].PlayerId == playerId {
			return &e.playerStats[i]
		}
	}
	return nil
}

func (e *MatchEngine) getPlayerCondition(playerId PlayerID) Optional[float64] {
	for i := range e.players {
		player := &e.players[i]
		if player.Player != nil && player.Player.GetId() == playerId {
			return hSome(player.Stamina)
		}
	}
	if entry := e.findPlayerStats(playerId); entry != nil {
		return hSome(entry.Condition)
	}
	return Optional[float64]{}
}

func (e *MatchEngine) setPlayerCondition(playerId PlayerID, condition float64) bool {
	if e.state != MatchState_KICK_OFF || e.elapsedMatchMinutes > 0.0 {
		return false
	}
	clamped := hClamp(condition, Player_MINIMUM_STAMINA, 1.0)
	for i := range e.players {
		player := &e.players[i]
		if player.Player != nil && player.Player.GetId() == playerId {
			player.Stamina = clamped
			e.statsOf(player).Condition = player.Stamina
			e.inputRevision++
			return true
		}
	}
	for _, bench := range [2][]*Player{e.homeBench, e.awayBench} {
		if !slices.ContainsFunc(bench, func(candidate *Player) bool { return candidate.GetId() == playerId }) {
			continue
		}
		e.benchConditions = slices.DeleteFunc(e.benchConditions,
			func(entry Pair[PlayerID, float64]) bool { return entry.First == playerId })
		e.benchConditions = append(e.benchConditions, Pair[PlayerID, float64]{playerId, clamped})
		e.inputRevision++
		return true
	}
	return false
}

func (e *MatchEngine) setTacticalFamiliarity(homeTeam bool, familiarity float64) {
	if !hIsFinite(familiarity) {
		return
	}
	if homeTeam {
		e.homeFamiliarity = hClamp(familiarity, 0.0, 1.0)
	} else {
		e.awayFamiliarity = hClamp(familiarity, 0.0, 1.0)
	}
	e.refreshTargetBlends()
	e.inputRevision++
}

func (e *MatchEngine) refreshTargetBlends() {
	for team := 0; team < 2; team++ {
		familiarity := e.effectiveFamiliarity(team)
		response := 1.0 - Player_FAMILIARITY_RESPONSE_LOSS*(1.0-familiarity)
		e.targetBlends[team*2] = 1.0 - math.Exp(-Player_TACTICAL_TARGET_RESPONSE_PER_SECOND*response*e.stepSeconds())
		e.targetBlends[team*2+1] = 1.0 - math.Exp(-Player_URGENT_TARGET_RESPONSE_PER_SECOND*response*e.stepSeconds())
	}
}

func (e *MatchEngine) setStepTicks(ticks uint32) {
	ticks = max(ticks, 1)
	if ticks == e.stepTicks {
		return
	}
	e.stepTicks = ticks
	e.refreshTargetBlends()
}

func (e *MatchEngine) stepSeconds() float64 {
	return Timing_FIXED_STEP_SECONDS * float64(e.stepTicks)
}

func (e *MatchEngine) periodElapsed(ticks uint64) bool {
	stepTicks := uint64(e.stepTicks)
	if e.stepCounter < stepTicks {
		return true
	}
	return e.stepCounter/ticks != (e.stepCounter-stepTicks)/ticks
}

func (e *MatchEngine) familiarityOf(player *MatchPlayer) float64 {
	return e.effectiveFamiliarity(hBoolIndex(player.IsHomeTeam))
}

func (e *MatchEngine) effectiveFamiliarity(team int) float64 {
	drilled := e.awayFamiliarity
	if team == 0 {
		drilled = e.homeFamiliarity
	}
	if e.reshapeSecondsRemaining[team] <= 0.0 {
		return drilled
	}
	return math.Max(0.0, drilled-e.reshapeFamiliarityCost[team]*e.reshapeSecondsRemaining[team]/
		Touchline_RESHAPE_RECOVERY_SECONDS)
}

func (e *MatchEngine) setStrategy(homeTeam bool, strategy *Strategy) {
	command := NewMatchCommandRecord()
	command.Type = MatchCommandType_STRATEGY
	command.HomeTeam = homeTeam
	command.Strategy = strategy.Clone()
	e.recordCommand(&command)
}

func (e *MatchEngine) applyShout(homeTeam bool, shout MatchShout) {
	command := NewMatchCommandRecord()
	command.Type = MatchCommandType_SHOUT
	command.HomeTeam = homeTeam
	command.Shout = shout
	e.recordCommand(&command)
}

func (e *MatchEngine) startShout(homeTeam bool, shout MatchShout) {
	current := &e.shouts[hBoolIndex(homeTeam)]
	current.Shout = shout
	current.RemainingSeconds = Touchline_SHOUT_DURATION_SECONDS
	current.Impact = 1.0 / (1.0 + current.Repeats)
	current.Repeats += 1.0
	e.refreshEffectiveSliders()
	e.inputRevision++
}

func (e *MatchEngine) getShoutStrength(homeTeam bool) float64 {
	shout := &e.shouts[hBoolIndex(homeTeam)]
	return hClamp(shout.RemainingSeconds/Touchline_SHOUT_DURATION_SECONDS, 0.0, 1.0) * shout.Impact
}

func (e *MatchEngine) getActiveShout(homeTeam bool) Optional[MatchShout] {
	shout := &e.shouts[hBoolIndex(homeTeam)]
	if shout.RemainingSeconds <= 0.0 {
		return Optional[MatchShout]{}
	}
	return hSome(shout.Shout)
}

// getFormation returns a copy (the C++ returns the vector by value).
func (e *MatchEngine) getFormation(homeTeam bool) []Vector2F {
	return slices.Clone(e.formations[hBoolIndex(homeTeam)])
}

func (e *MatchEngine) setFormation(homeTeam bool, shape []Vector2F) bool {
	command := NewMatchCommandRecord()
	command.Type = MatchCommandType_FORMATION
	command.HomeTeam = homeTeam
	command.Formation = slices.Clone(shape)
	return e.recordCommand(&command)
}

func (e *MatchEngine) getFormationSlot(playerId PlayerID) Optional[int] {
	for i := range e.players {
		player := &e.players[i]
		if active(player) && !player.IsGoalkeeper && player.FormationSlot >= 0 &&
			player.Player.GetId() == playerId {
			return hSome(int(player.FormationSlot))
		}
	}
	return Optional[int]{}
}

func (e *MatchEngine) movePlayerToSlot(playerId PlayerID, slot int) bool {
	command := NewMatchCommandRecord()
	command.Type = MatchCommandType_MOVE_TO_SLOT
	command.Player = playerId
	command.Slot = hSome(slot)
	for i := range e.players {
		player := &e.players[i]
		if player.Player != nil && player.Player.GetId() == playerId {
			command.HomeTeam = player.IsHomeTeam
		}
	}
	return e.recordCommand(&command)
}

// loadCommandReplay takes the log by value in C++; the Go port sorts a copy.
func (e *MatchEngine) loadCommandReplay(log []MatchCommandRecord) {
	log = slices.Clone(log)
	slices.SortStableFunc(log, func(first, second MatchCommandRecord) int {
		if first.Step < second.Step {
			return -1
		}
		if second.Step < first.Step {
			return 1
		}
		return 0
	})
	e.commandLog = log
	e.commandCursor = 0
	e.inputRevision++
}

// b1cloneCommand deep-copies a command record (C++ copies by value).
func b1cloneCommand(command *MatchCommandRecord) MatchCommandRecord {
	copied := *command
	copied.Strategy = command.Strategy.Clone()
	copied.Formation = slices.Clone(command.Formation)
	return copied
}

func (e *MatchEngine) recordCommand(command *MatchCommandRecord) bool {
	if !e.executeCommand(command) {
		return false
	}
	// A live change ends any replay still pending.
	e.commandLog = e.commandLog[:e.commandCursor]
	e.commandLog = append(e.commandLog, b1cloneCommand(command))
	e.commandLog[len(e.commandLog)-1].Step = e.stepCounter
	e.commandCursor = len(e.commandLog)
	e.inputRevision++
	return true
}

func (e *MatchEngine) applyDueCommands() {
	for e.commandCursor < len(e.commandLog) && e.commandLog[e.commandCursor].Step <= e.stepCounter {
		command := b1cloneCommand(&e.commandLog[e.commandCursor])
		e.commandCursor++
		e.executeCommand(&command)
	}
}

func (e *MatchEngine) executeCommand(command *MatchCommandRecord) bool {
	team := hBoolIndex(command.HomeTeam)
	switch command.Type {
	case MatchCommandType_TEAM_TALK:
		if command.TalkHalf < 1 || command.TalkHalf > 2 || !hIsFinite(command.TalkModifier) {
			return false
		}
		e.teamTalks[team][command.TalkHalf-1] = hClamp(command.TalkModifier,
			-Touchline_MAX_TEAM_TALK_MODIFIER, Touchline_MAX_TEAM_TALK_MODIFIER)
		e.inputRevision++
		return true
	case MatchCommandType_STRATEGY:
		if command.HomeTeam {
			e.homeStrategy = command.Strategy.Clone()
		} else {
			e.awayStrategy = command.Strategy.Clone()
		}
		e.resolveTactics(command.HomeTeam)
		e.refreshEffectiveSliders()
		e.inputRevision++
		return true
	case MatchCommandType_SHOUT:
		e.startShout(command.HomeTeam, command.Shout)
		return true
	case MatchCommandType_FORMATION:
		formation := e.formations[team]
		if len(command.Formation) != len(formation) || e.state == MatchState_FULL_TIME {
			return false
		}
		moved := 0.0
		shape := make([]Vector2F, 0, len(formation))
		for slot := 0; slot < len(formation); slot++ {
			wanted := command.Formation[slot]
			if !hIsFinite(wanted.X) || !hIsFinite(wanted.Y) {
				return false
			}
			shape = append(shape, Vector2F{
				hClamp(wanted.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
				hClamp(wanted.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)})
			moved += distance(shape[len(shape)-1], formation[slot])
		}
		e.formations[team] = shape
		if cost := math.Min(Touchline_MAX_RESHAPE_FAMILIARITY_COST,
			moved/float64(max(len(shape), 1))*Touchline_RESHAPE_FAMILIARITY_PER_METRE); cost > 0.0 {
			// A second change before the first has sunk in adds to what is left.
			e.reshapeFamiliarityCost[team] = math.Min(Touchline_MAX_RESHAPE_FAMILIARITY_COST,
				cost+e.reshapeFamiliarityCost[team]*e.reshapeSecondsRemaining[team]/
					Touchline_RESHAPE_RECOVERY_SECONDS)
			e.reshapeSecondsRemaining[team] = Touchline_RESHAPE_RECOVERY_SECONDS
			e.refreshTargetBlends()
		}
		e.placeFormation(command.HomeTeam)
		e.inputRevision++
		return true
	case MatchCommandType_MOVE_TO_SLOT:
		// std::size_t slot: a negative Go value maps to a huge one (rejected).
		if !command.Slot.HasValue || command.Slot.Value < 0 ||
			command.Slot.Value >= len(e.formations[team]) || e.state == MatchState_FULL_TIME {
			return false
		}
		var mover *MatchPlayer
		for i := range e.players {
			player := &e.players[i]
			if active(player) && !player.IsGoalkeeper && player.FormationSlot >= 0 &&
				player.IsHomeTeam == command.HomeTeam && player.Player.GetId() == command.Player {
				mover = player
				break
			}
		}
		if mover == nil {
			return false
		}
		slot := int8(command.Slot.Value)
		for i := range e.players {
			occupant := &e.players[i]
			if active(occupant) && !occupant.IsGoalkeeper &&
				occupant.IsHomeTeam == command.HomeTeam && occupant.FormationSlot == slot {
				occupant.FormationSlot = mover.FormationSlot
			}
		}
		mover.FormationSlot = slot
		e.placeFormation(command.HomeTeam)
		e.inputRevision++
		return true
	case MatchCommandType_SUBSTITUTION:
		bench := e.awayBench
		if command.HomeTeam {
			bench = e.homeBench
		}
		var inPlayer *Player
		for _, player := range bench {
			if player != nil && player.GetId() == command.Incoming {
				inPlayer = player
				break
			}
		}
		if inPlayer == nil || (command.Slot.HasValue &&
			(command.Slot.Value < 0 || command.Slot.Value >= len(e.formations[team]))) {
			return false
		}
		var outgoing *MatchPlayer
		for i := range e.players {
			player := &e.players[i]
			if active(player) && player.Player.GetId() == command.Player {
				outgoing = player
				break
			}
		}
		if outgoing == nil || outgoing.IsHomeTeam != command.HomeTeam || !e.canSubstitute(command.HomeTeam) {
			return false
		}
		// A change made while the ball is live stops play for it; several
		// changes made together share that stoppage (and window).
		if e.state == MatchState_PLAYING && e.manualSubstitutionStep != e.stepCounter {
			e.beginStoppage()
			e.manualSubstitutionStep = e.stepCounter
		}
		if !e.canSubstitute(outgoing.IsHomeTeam) {
			return false
		}
		e.inputRevision++
		changed := e.performSubstitution(outgoing, inPlayer, SubstitutionReason_MANUAL)
		if changed && command.Slot.HasValue && !outgoing.IsGoalkeeper && outgoing.FormationSlot >= 0 {
			move := NewMatchCommandRecord()
			move.Type = MatchCommandType_MOVE_TO_SLOT
			move.HomeTeam = command.HomeTeam
			move.Player = inPlayer.GetId()
			move.Slot = command.Slot
			e.executeCommand(&move)
		}
		e.describePendingEvents()
		return changed
	}
	return false
}

func (e *MatchEngine) placeFormation(homeTeam bool) {
	formation := e.formations[hBoolIndex(homeTeam)]
	sideSize := 0
	onPitch := 0
	for i := range e.players {
		player := &e.players[i]
		if player.Player == nil || player.IsHomeTeam != homeTeam {
			continue
		}
		sideSize++
		if active(player) {
			onPitch++
		}
	}
	drop := float64(sideSize-onPitch) * Rules_SHORT_HANDED_DROP
	for i := range e.players {
		player := &e.players[i]
		if !active(player) || player.IsHomeTeam != homeTeam || player.IsGoalkeeper ||
			player.FormationSlot < 0 || int(player.FormationSlot) >= len(formation) {
			continue
		}
		spot := formation[int(player.FormationSlot)]
		x := hClamp(spot.X-drop, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X)
		if homeTeam {
			player.BasePosition = Vector2F{x, spot.Y}
		} else {
			player.BasePosition = Vector2F{1.0 - x, spot.Y}
		}
	}
}

func (e *MatchEngine) getEffectiveSliders(homeTeam bool) StrategySliders {
	return e.effectiveSliders[hBoolIndex(homeTeam)]
}

func (e *MatchEngine) refreshEffectiveSliders() {
	e.effectiveSliders = [2]StrategySliders{e.computeEffectiveSliders(true), e.computeEffectiveSliders(false)}
}

func (e *MatchEngine) computeUnderdogShares() {
	// Mean outfield quality of the starting elevens, without the home lift.
	var quality [2]float64
	var counted [2]int
	for i := range e.players {
		player := &e.players[i]
		if player.Player == nil || player.IsGoalkeeper {
			continue
		}
		side := hBoolIndex(player.IsHomeTeam)
		sum := 0.0
		for _, name := range [7]string{"Pace", "Shooting", "Passing", "Dribbling", "Defending",
			"Physicality", "Vision"} {
			sum += e.attribute(player.Player, name)
		}
		quality[side] += sum / 7.0
		counted[side]++
	}
	if counted[0] == 0 || counted[1] == 0 {
		return
	}
	gap := quality[1]/float64(counted[1]) - quality[0]/float64(counted[0])
	share := func(deficit float64) float64 {
		return hClamp((deficit-Touchline_UNDERDOG_GAP_START)/Touchline_UNDERDOG_GAP_RANGE, 0.0, 1.0)
	}
	e.underdogShares = [2]float64{share(gap), share(-gap)}
}

func (e *MatchEngine) computeEffectiveSliders(homeTeam bool) StrategySliders {
	var sliders StrategySliders
	if homeTeam {
		sliders = e.homeStrategy.GetSliders()
	} else {
		sliders = e.awayStrategy.GetSliders()
	}
	// Score effects: a side in front sits deeper and takes fewer risks, a side
	// behind pushes on, more so as the match runs out.
	rawLead := e.awayScore - e.homeScore
	if homeTeam {
		rawLead = e.homeScore - e.awayScore
	}
	lead := hClampInt(rawLead, -Touchline_SCORE_EFFECT_MAX_GOALS, Touchline_SCORE_EFFECT_MAX_GOALS)
	clampSlider := func(value *float64) { *value = hClamp(*value, 0.0, 1.0) }
	if lead != 0 {
		urgency := float64(lead) *
			(Touchline_SCORE_EFFECT_BASE +
				(1.0-Touchline_SCORE_EFFECT_BASE)*
					math.Min(1.0, e.matchTimeMinutes/(2.0*Timing_HALF_TIME_MINUTE)))
		// Only a side chasing the game commits more men forward; one in front
		// keeps its passing but stops pressing and taking risks.
		if lead < 0 {
			sliders.OffensiveBias -= urgency * Touchline_SCORE_EFFECT_OFFENSIVE
		}
		if lead >= Decision_COMFORTABLE_LEAD {
			sliders.OffensiveBias -= urgency / float64(lead) *
				float64(lead-Decision_COMFORTABLE_LEAD+1) *
				Touchline_GAME_MANAGEMENT_OFFENSIVE
		}
		sliders.RiskTaking -= urgency * Touchline_SCORE_EFFECT_RISK
		sliders.Pressing -= urgency * Touchline_SCORE_EFFECT_PRESSING
		sliders.Compactness += urgency * Touchline_SCORE_EFFECT_COMPACTNESS
		clampSlider(&sliders.Pressing)
		clampSlider(&sliders.RiskTaking)
		clampSlider(&sliders.OffensiveBias)
		clampSlider(&sliders.Compactness)
	}
	if underdog := e.underdogShares[hBoolIndex(homeTeam)]; underdog > 0.0 {
		sliders.Pressing -= underdog * Touchline_UNDERDOG_PRESSING
		sliders.Compactness += underdog * Touchline_UNDERDOG_COMPACTNESS
		sliders.OffensiveBias -= underdog * Touchline_UNDERDOG_OFFENSIVE
		clampSlider(&sliders.Pressing)
		clampSlider(&sliders.OffensiveBias)
		clampSlider(&sliders.Compactness)
	}
	strength := e.getShoutStrength(homeTeam)
	if strength <= 0.0 {
		return sliders
	}
	step := Touchline_SHOUT_SLIDER_STEP * strength
	switch e.shouts[hBoolIndex(homeTeam)].Shout {
	case MatchShout_PUSH_HIGHER:
		sliders.OffensiveBias += step
		sliders.Pressing += step * 0.5
	case MatchShout_DROP_DEEPER:
		sliders.OffensiveBias -= step
		sliders.Pressing -= step * 0.75
		sliders.Compactness += step * 0.75
	case MatchShout_PRESS_MORE:
		sliders.Pressing += step * 1.25
	case MatchShout_CALM_DOWN:
		sliders.RiskTaking -= step
		sliders.Pressing -= step * 0.25
	case MatchShout_WORK_BALL_INTO_BOX:
		sliders.RiskTaking -= step * 0.25
	case MatchShout_SHOOT_ON_SIGHT:
		sliders.RiskTaking += step * 0.5
	case MatchShout_ENCOURAGE:
	case MatchShout_STAND_OFF:
		sliders.Pressing -= step * 1.25
		sliders.Compactness += step * 0.5
	case MatchShout_DEMAND_MORE:
		sliders.Pressing += step * 0.5
		sliders.OffensiveBias += step * 0.5
	case MatchShout_HIT_ON_COUNTER:
		sliders.Pressing -= step * 0.5
		sliders.Compactness += step * 0.5
		sliders.RiskTaking += step
	case MatchShout_KEEP_POSSESSION:
		sliders.RiskTaking -= step
		sliders.OffensiveBias -= step * 0.5
		sliders.WidthUsage += step * 0.5
	}
	clampSlider(&sliders.Pressing)
	clampSlider(&sliders.RiskTaking)
	clampSlider(&sliders.OffensiveBias)
	clampSlider(&sliders.WidthUsage)
	clampSlider(&sliders.Compactness)
	return sliders
}

func (e *MatchEngine) shoutShotBias(homeTeam bool) float64 {
	shout := e.shouts[hBoolIndex(homeTeam)].Shout
	bias := 0.0
	if shout == MatchShout_SHOOT_ON_SIGHT {
		bias = 1.0
	} else if shout == MatchShout_WORK_BALL_INTO_BOX {
		bias = -1.0
	}
	return bias * Touchline_SHOUT_SHOT_BIAS * e.getShoutStrength(homeTeam)
}

func (e *MatchEngine) shoutWorkRate(homeTeam bool) float64 {
	shout := e.shouts[hBoolIndex(homeTeam)].Shout
	if shout == MatchShout_ENCOURAGE || shout == MatchShout_DEMAND_MORE {
		return Touchline_ENCOURAGE_WORK_RATE * e.getShoutStrength(homeTeam)
	}
	return 0.0
}

func (e *MatchEngine) setTeamTalkModifier(homeTeam bool, half int, modifier float64) {
	// Logged like every touchline change, so a replay of the command log
	// reproduces a match a team talk played its part in.
	command := NewMatchCommandRecord()
	command.Type = MatchCommandType_TEAM_TALK
	command.HomeTeam = homeTeam
	command.TalkHalf = half
	command.TalkModifier = modifier
	e.recordCommand(&command)
}

func (e *MatchEngine) getTeamTalkModifier(homeTeam bool, half int) float64 {
	if half < 1 || half > 2 {
		return 0.0
	}
	return e.teamTalks[hBoolIndex(homeTeam)][half-1]
}

func (e *MatchEngine) talkOf(homeTeam bool) float64 {
	// A talk lifts (or deflates) the side for the first minutes of its half
	// and has worn off well before the break; extra time has none.
	if e.period < 1 || e.period > 2 {
		return 0.0
	}
	talk := e.teamTalks[hBoolIndex(homeTeam)][e.period-1]
	if talk == 0.0 {
		return 0.0
	}
	minutes := e.matchTimeMinutes
	if e.period == 2 {
		minutes -= Timing_HALF_TIME_MINUTE
	}
	return talk * hClamp((TacticsTuning_TALK_FADE_END_MINUTES-minutes)/
		(TacticsTuning_TALK_FADE_END_MINUTES-TacticsTuning_TALK_FULL_MINUTES), 0.0, 1.0)
}

func (e *MatchEngine) teamEdge(homeTeam bool) float64 {
	balance := 0
	for i := range e.players {
		player := &e.players[i]
		if active(player) {
			if player.IsHomeTeam == homeTeam {
				balance++
			} else {
				balance--
			}
		}
	}
	balance = hClampInt(balance, -Rules_MAX_NUMERICAL_EDGE_PLAYERS, Rules_MAX_NUMERICAL_EDGE_PLAYERS)
	home := 0.0
	if homeTeam {
		home = Rules_HOME_EXECUTION_BONUS * e.matchContext.HomeAdvantageScale
	}
	return home + e.talkOf(homeTeam) + float64(balance)*Rules_NUMERICAL_EDGE_PER_PLAYER
}

func (e *MatchEngine) runAiTouchline(homeTeam bool) {
	if e.period < 2 || e.getShoutStrength(homeTeam) > 0.0 {
		return
	}
	lead := e.awayScore - e.homeScore
	if homeTeam {
		lead = e.homeScore - e.awayScore
	}
	if lead < 0 && e.matchTimeMinutes >= Touchline_AI_SHOOT_ON_SIGHT_MINUTE {
		e.startShout(homeTeam, MatchShout_SHOOT_ON_SIGHT)
	} else if lead < 0 && e.matchTimeMinutes >= Touchline_AI_CHASE_MINUTE {
		e.startShout(homeTeam, MatchShout_PUSH_HIGHER)
	} else if lead > 0 && e.matchTimeMinutes >= Touchline_AI_PROTECT_MINUTE {
		e.startShout(homeTeam, MatchShout_DROP_DEEPER)
	}
}

func (e *MatchEngine) refreshRatings() {
	for i := range e.playerStats {
		entry := &e.playerStats[i]
		goalDifference := e.awayScore - e.homeScore
		if entry.IsHomeTeam {
			goalDifference = e.homeScore - e.awayScore
		}
		goalkeeper := entry.Role == PlayerRole_GK
		defender := entry.Role == PlayerRole_CB || entry.Role == PlayerRole_LB || entry.Role == PlayerRole_RB
		entry.Rating = MatchRules_computeMatchRating(entry, goalkeeper, defender, goalDifference)
	}
}
