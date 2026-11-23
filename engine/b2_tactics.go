// Ported from Player12 src/model/match_engine.cpp lines 3971–4342 (GPL-3.0)

package engine

import "math"

// NO_ROLE is the anonymous-namespace `const RoleProfile NO_ROLE{}`.
var NO_ROLE = RoleProfile{}

func (e *MatchEngine) resolveTactics(homeTeam bool) {
	// Compile stored slot instructions into match-local numeric profiles. The
	// anchor identifies a formation job, not its current occupant: substitutes
	// inherit that job. Incompatible roles fall back to Standard. Keep this
	// resolution outside the per-player physics loops when adding new roles.
	side := hBoolIndex(homeTeam)
	strategy := &e.awayStrategy
	if homeTeam {
		strategy = &e.homeStrategy
	}
	anchors := e.slotAnchors[side]
	for slot := 0; slot < len(e.slotTactics[side]); slot++ {
		tactic := SlotTactic{Role: TacticalRole_Standard}
		var instruction *SlotInstruction
		if slot < len(anchors) {
			instruction = strategy.FindSlot(anchors[slot])
		}
		if instruction != nil {
			family := Tactics_familyForSlot(anchors[slot])
			if Tactics_allows(family, instruction.Role) {
				tactic.Role = instruction.Role
			} else {
				tactic.Role = TacticalRole_Standard
			}
			tactic.Profile = Tactics_profile(tactic.Role, instruction.Duty)
			tactic.PossessionOffset = instruction.PossessionOffset
		}
		e.slotTactics[side][slot] = tactic
	}
	e.keeperProfiles[side] = Tactics_profile(strategy.GetKeeperRole(), RoleDuty_Support)
	// std::vector copy assignment: the engine keeps its own copy.
	orders := strategy.GetOppositionOrders()
	e.oppositionOrders[side] = append([]PlayerInstruction(nil), orders...)
}

func (e *MatchEngine) getSlotRole(homeTeam bool, slot int) TacticalRole {
	tactics := &e.slotTactics[hBoolIndex(homeTeam)]
	if slot >= 0 && slot < len(tactics) {
		return tactics[slot].Role
	}
	return TacticalRole_Standard
}

func (e *MatchEngine) roleProfileOf(player *MatchPlayer) *RoleProfile {
	side := hBoolIndex(player.IsHomeTeam)
	if player.IsGoalkeeper {
		return &e.keeperProfiles[side]
	}
	slot := int(player.FormationSlot)
	if player.FormationSlot < 0 || slot >= len(e.slotTactics[side]) {
		return &NO_ROLE
	}
	return &e.slotTactics[side][slot].Profile
}

func (e *MatchEngine) possessionAnchor(player *MatchPlayer) Vector2F {
	side := hBoolIndex(player.IsHomeTeam)
	slot := int(player.FormationSlot)
	if player.IsGoalkeeper || player.FormationSlot < 0 ||
		slot >= len(e.slotTactics[side]) {
		return player.BasePosition
	}
	tactic := &e.slotTactics[side][slot]
	if tactic.PossessionOffset.X == 0.0 && tactic.PossessionOffset.Y == 0.0 &&
		tactic.Profile.PossessionAdvanceMetres == 0.0 &&
		tactic.Profile.PossessionWidthMetres == 0.0 {
		return player.BasePosition
	}
	direction := -1.0
	if player.IsHomeTeam {
		direction = 1.0
	}
	lateral := player.BasePosition.Y - Pitch_CENTRE
	// Width pushes toward his own touchline; a central player has none.
	flank := 0.0
	if math.Abs(lateral) >= 0.05 {
		if lateral > 0.0 {
			flank = 1.0
		} else {
			flank = -1.0
		}
	}
	anchor := player.BasePosition
	anchor.X += direction *
		(tactic.PossessionOffset.X +
			tactic.Profile.PossessionAdvanceMetres/Pitch_LENGTH_METRES)
	anchor.Y += tactic.PossessionOffset.Y +
		flank*tactic.Profile.PossessionWidthMetres/Pitch_WIDTH_METRES
	return Vector2F{hClamp(anchor.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
		hClamp(anchor.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
}

func (e *MatchEngine) defensiveRoleShift(player *MatchPlayer) float64 {
	direction := -1.0
	if player.IsHomeTeam {
		direction = 1.0
	}
	return direction * e.roleProfileOf(player).DefensiveAdvanceMetres / Pitch_LENGTH_METRES
}

func (e *MatchEngine) instructionAgainst(target *MatchPlayer) OppositionInstruction {
	orders := e.oppositionOrders[hBoolIndex(!target.IsHomeTeam)]
	if len(orders) == 0 || target.Player == nil {
		return OppositionInstruction_None
	}
	id := target.Player.GetId()
	for _, order := range orders {
		if order.Player == id {
			return order.Instruction
		}
	}
	return OppositionInstruction_None
}

func (e *MatchEngine) tightMarkTarget(marker *MatchPlayer) *MatchPlayer {
	if len(e.oppositionOrders[hBoolIndex(marker.IsHomeTeam)]) == 0 {
		return nil
	}
	assigned := e.markAssignments[e.slotOf(marker)]
	if assigned < 0 || int(assigned) >= len(e.players) {
		return nil
	}
	mark := &e.players[int(assigned)]
	if !active(mark) || mark.IsHomeTeam == marker.IsHomeTeam {
		return nil
	}
	if e.instructionAgainst(mark) == OppositionInstruction_TightMark {
		return mark
	}
	return nil
}

func (e *MatchEngine) tightMarkPoint(marker *MatchPlayer, target *MatchPlayer, current Vector2F) Vector2F {
	direction := -1.0
	if marker.IsHomeTeam {
		direction = 1.0
	}
	point := Vector2F{
		target.Position.X - direction*TacticsTuning_TIGHT_MARK_GOAL_SIDE_METRES/Pitch_LENGTH_METRES,
		target.Position.Y}
	return Vector2F{current.X + (point.X-current.X)*TacticsTuning_TIGHT_MARK_WEIGHT,
		current.Y + (point.Y-current.Y)*TacticsTuning_TIGHT_MARK_WEIGHT}
}

func (e *MatchEngine) tightMarkPenalty(receiver *MatchPlayer) float64 {
	if e.instructionAgainst(receiver) != OppositionInstruction_TightMark {
		return 0.0
	}
	for slot := 0; slot < len(e.players) && slot < 32; slot++ {
		marker := &e.players[slot]
		assigned := int(e.markAssignments[slot])
		// The C++ indexes players[assigned] unchecked; assignMarks only stores
		// valid slots, the bound check just avoids a Go panic.
		if !active(marker) || marker.IsHomeTeam == receiver.IsHomeTeam ||
			assigned < 0 || assigned >= len(e.players) ||
			&e.players[assigned] != receiver {
			continue
		}
		if distance(marker.Position, receiver.Position) <=
			TacticsTuning_TIGHT_MARK_REACH_METRES {
			return TacticsTuning_TIGHT_MARK_COMPLETION_PENALTY *
				Passing_COMPLETION_UTILITY_WEIGHT
		}
	}
	return 0.0
}

func (e *MatchEngine) passingRoleBias(passer *MatchPlayer, receiver *MatchPlayer, progression float64, completion float64) float64 {
	daring := e.roleProfileOf(passer).PassDaring
	target := e.roleProfileOf(receiver).TargetBias
	progressShare := 0.4
	if progression > 0.0 {
		progressShare = 1.0
	}
	return daring*(progression*TacticsTuning_PASS_DARING_PROGRESS_WEIGHT+
		(1.0-completion)*TacticsTuning_PASS_DARING_SAFETY_WEIGHT) +
		target*TacticsTuning_TARGET_BIAS_WEIGHT*progressShare -
		e.tightMarkPenalty(receiver)
}

func (e *MatchEngine) pressEagerness(candidate *MatchPlayer, carrier *MatchPlayer) float64 {
	if carrier == nil || carrier.IsHomeTeam == candidate.IsHomeTeam {
		return 0.0
	}
	// A rousing talk sends the side after the ball sooner (a flat one later).
	return e.roleProfileOf(candidate).PressBias*TacticsTuning_PRESS_BIAS_SECONDS +
		e.talkSwing(candidate.IsHomeTeam)*TacticsTuning_TALK_PRESS_SECONDS
}

func (e *MatchEngine) engageBoost(defender *MatchPlayer, carrier *MatchPlayer) float64 {
	boost := 1.0 +
		e.roleProfileOf(defender).PressBias*TacticsTuning_PRESS_BIAS_ENGAGE +
		e.talkSwing(defender.IsHomeTeam)*TacticsTuning_TALK_ENGAGE
	order := e.instructionAgainst(carrier)
	if order == OppositionInstruction_Press {
		boost += TacticsTuning_PRESS_ORDER_ENGAGE
	}
	if order == OppositionInstruction_DoubleUp {
		boost += TacticsTuning_DOUBLE_UP_ENGAGE
	}
	return math.Max(0.2, boost)
}

func (e *MatchEngine) pressStandOffShare(presser *MatchPlayer, carrier *MatchPlayer) float64 {
	share := 1.0 - 0.2*e.roleProfileOf(presser).PressBias -
		0.2*e.talkSwing(presser.IsHomeTeam)
	if e.instructionAgainst(carrier) == OppositionInstruction_Press {
		share *= TacticsTuning_PRESS_ORDER_STANDOFF_SHARE
	}
	return hClamp(share, 0.3, 1.3)
}

func (e *MatchEngine) weakFootShade(carrier *MatchPlayer) float64 {
	if carrier.Player == nil ||
		e.instructionAgainst(carrier) != OppositionInstruction_ShowWeakFoot {
		return 0.0
	}
	// Facing his attacking direction, a right-footer's strong side is +y for
	// the home side (attacking toward x = 1) and -y for the away side.
	footSide := -1.0
	if carrier.Player.GetFoot() == Foot_Right {
		footSide = 1.0
	}
	teamSide := -1.0
	if carrier.IsHomeTeam {
		teamSide = 1.0
	}
	strongSide := footSide * teamSide
	return strongSide * TacticsTuning_WEAK_FOOT_SHADE_METRES / Pitch_WIDTH_METRES
}

func (e *MatchEngine) weakFootPressure(carrier *MatchPlayer) float64 {
	if e.instructionAgainst(carrier) != OppositionInstruction_ShowWeakFoot {
		return 0.0
	}
	return hClamp(
		(Decision_PRESSURE_RADIUS_METRES-e.nearestOpponentDistance(carrier))/
			Decision_PRESSURE_RADIUS_METRES,
		0.0, 1.0)
}

func (e *MatchEngine) doubleUpPoint(helper *MatchPlayer, carrier *MatchPlayer, firstPresser *MatchPlayer) Optional[Vector2F] {
	if e.instructionAgainst(carrier) != OppositionInstruction_DoubleUp {
		return Optional[Vector2F]{}
	}
	// The second man closes from the side the first presser leaves open,
	// goal-side of the carrier.
	lateral := 1.0
	if helper.Position.Y <= carrier.Position.Y {
		lateral = -1.0
	}
	if firstPresser != nil {
		if firstPresser.Position.Y > carrier.Position.Y {
			lateral = -1.0
		} else {
			lateral = 1.0
		}
	}
	direction := -1.0
	if helper.IsHomeTeam {
		direction = 1.0
	}
	reach := TacticsTuning_DOUBLE_UP_SUPPORT_METRES
	return hSome(Vector2F{
		hClamp(carrier.Position.X-direction*reach*0.5/Pitch_LENGTH_METRES,
			Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
		hClamp(carrier.Position.Y+lateral*reach/Pitch_WIDTH_METRES,
			Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)})
}

func (e *MatchEngine) doubleUpPenalty(carrier *MatchPlayer, defender *MatchPlayer) float64 {
	if e.instructionAgainst(carrier) != OppositionInstruction_DoubleUp {
		return 0.0
	}
	for i := range e.players {
		other := &e.players[i]
		if other == defender || !active(other) ||
			other.IsHomeTeam == carrier.IsHomeTeam || other.IsGoalkeeper {
			continue
		}
		if distance(other.Position, carrier.Position) <=
			TacticsTuning_DOUBLE_UP_REACH_METRES {
			return TacticsTuning_DOUBLE_UP_TAKE_ON_PENALTY
		}
	}
	return 0.0
}

func (e *MatchEngine) talkSwing(homeTeam bool) float64 {
	return hClamp(e.talkOf(homeTeam)/TacticsTuning_TALK_REFERENCE, -1.0, 1.0)
}

func (e *MatchEngine) getTeamTalkEffect(homeTeam bool) float64 {
	return e.talkOf(homeTeam)
}

func (e *MatchEngine) assignMarks() {
	// Greedy one-to-one assignment of outfield markers to opposing outfield
	// players by channel, depth and danger, so nobody is left unmarked while
	// two defenders shadow the same attacker.
	for i := range e.markAssignments {
		e.markAssignments[i] = -1
	}
	score := func(markerHome bool, base Vector2F, opponentX float64, opponentY float64) float64 {
		dangerDepth := opponentX
		if markerHome {
			dangerDepth = 1.0 - opponentX
		}
		return math.Abs(opponentY-base.Y)*Shape_CHANNEL_WEIGHT +
			math.Abs(opponentX-base.X) -
			dangerDepth*Shape_DANGER_DEPTH_WEIGHT
	}
	for _, home := range [2]bool{true, false} {
		var markers [16]uint8
		var opponents [16]uint8
		markerCount := 0
		opponentCount := 0
		for slot := 0; slot < len(e.players) && slot < 32; slot++ {
			candidate := &e.players[slot]
			if !active(candidate) || candidate.IsGoalkeeper {
				continue
			}
			if candidate.IsHomeTeam == home && markerCount < len(markers) {
				markers[markerCount] = uint8(slot)
				markerCount++
			} else if candidate.IsHomeTeam != home && opponentCount < len(opponents) {
				opponents[opponentCount] = uint8(slot)
				opponentCount++
			}
		}
		var scores [16 * 16]float64
		var opponentX [16]float64
		var opponentY [16]float64
		for o := 0; o < opponentCount; o++ {
			opponentX[o] = e.players[opponents[o]].Position.X
			opponentY[o] = e.players[opponents[o]].Position.Y
		}
		for m := 0; m < markerCount; m++ {
			marker := &e.players[markers[m]]
			base := marker.BasePosition
			for o := 0; o < opponentCount; o++ {
				scores[m*16+o] = score(marker.IsHomeTeam, base, opponentX[o], opponentY[o])
			}
		}
		// Each round takes the lowest remaining score (ties: lowest marker, then
		// lowest opponent). Every marker keeps its best free opponent, so a round
		// only rescans the markers whose best opponent was just taken.
		var bestOf [16]uint8
		findBest := func(m int, usedOpponents uint32) {
			best := math.MaxFloat64
			var bestOpponent uint8
			for o := 0; o < opponentCount; o++ {
				if usedOpponents&(1<<uint(o)) != 0 {
					continue
				}
				if scores[m*16+o] < best {
					best = scores[m*16+o]
					bestOpponent = uint8(o)
				}
			}
			bestOf[m] = bestOpponent
		}
		var usedMarkers uint32
		var usedOpponents uint32
		for m := 0; m < markerCount; m++ {
			findBest(m, 0)
		}
		for round := 0; round < min(markerCount, opponentCount); round++ {
			best := math.MaxFloat64
			bestMarker := 0
			for m := 0; m < markerCount; m++ {
				if usedMarkers&(1<<uint(m)) != 0 {
					continue
				}
				if scores[m*16+int(bestOf[m])] < best {
					best = scores[m*16+int(bestOf[m])]
					bestMarker = m
				}
			}
			bestOpponent := bestOf[bestMarker]
			usedMarkers |= 1 << uint(bestMarker)
			usedOpponents |= 1 << uint(bestOpponent)
			e.markAssignments[markers[bestMarker]] = int8(opponents[bestOpponent])
			for m := 0; m < markerCount; m++ {
				if usedMarkers&(1<<uint(m)) == 0 && bestOf[m] == bestOpponent {
					findBest(m, usedOpponents)
				}
			}
		}
	}
}
