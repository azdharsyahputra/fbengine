// Ported from Player12 src/model/lineup.h and src/model/lineup.cpp (GPL-3.0)

package engine

import "slices"

// Not ported (career/UI only): swapPlayers, bringIn, toString,
// generateStartingXI.

// SetPieceDuty lists leadership and dead-ball duties the manager can hand
// out (values are persisted in the lineup).
type SetPieceDuty uint8

const (
	SetPieceDuty_Captain SetPieceDuty = iota
	SetPieceDuty_ViceCaptain
	SetPieceDuty_Penalties
	SetPieceDuty_FreeKicks // Direct free kicks in shooting range.
	SetPieceDuty_CornersLeft
	SetPieceDuty_CornersRight
	SetPieceDuty_LongThrows
	SetPieceDuty_COUNT
)

// SET_PIECE_DUTY_COUNT is the number of set-piece duties.
const SET_PIECE_DUTY_COUNT = int(SetPieceDuty_COUNT)

// SetPieceDesignations is the player designated for each duty; 0 = chosen
// automatically.
type SetPieceDesignations = [SET_PIECE_DUTY_COUNT]PlayerID

// SetPieces_dutyKey is the language key naming duty.
func SetPieces_dutyKey(duty SetPieceDuty) string {
	switch duty {
	case SetPieceDuty_Captain:
		return "SET_PIECE_CAPTAIN"
	case SetPieceDuty_ViceCaptain:
		return "SET_PIECE_VICE_CAPTAIN"
	case SetPieceDuty_Penalties:
		return "SET_PIECE_PENALTIES"
	case SetPieceDuty_FreeKicks:
		return "SET_PIECE_FREE_KICKS"
	case SetPieceDuty_CornersLeft:
		return "SET_PIECE_CORNERS_LEFT"
	case SetPieceDuty_CornersRight:
		return "SET_PIECE_CORNERS_RIGHT"
	case SetPieceDuty_LongThrows:
		return "SET_PIECE_LONG_THROWS"
	}
	return "SET_PIECE_CAPTAIN"
}

// SetPieces_isLeadership is true for the captaincy duties.
func SetPieces_isLeadership(duty SetPieceDuty) bool {
	return duty == SetPieceDuty_Captain || duty == SetPieceDuty_ViceCaptain
}

// lineupStat is lineup.cpp's anonymous `stat` (0 when missing; renamed:
// tactics.cpp has a different `stat`).
func lineupStat(player *Player, name string) float64 {
	if value, found := player.GetStats()[name]; found {
		return value
	}
	return 0.0
}

// SetPieces_score is the suitability of player for a dead-ball duty on the
// 0-100 attribute scale.
func SetPieces_score(duty SetPieceDuty, player *Player) float64 {
	if SetPieces_isLeadership(duty) || player.GetRole() == PlayerRole_GK {
		return 0.0
	}
	// A corner from the side of the preferred foot is the natural delivery.
	const FOOT_BONUS = 3.0
	switch duty {
	case SetPieceDuty_Penalties, SetPieceDuty_FreeKicks:
		return lineupStat(player, "Shooting")*0.75 +
			lineupStat(player, "Passing")*0.15 + lineupStat(player, "Vision")*0.10
	case SetPieceDuty_CornersLeft, SetPieceDuty_CornersRight:
		leftCorner := duty == SetPieceDuty_CornersLeft
		leftFooted := player.GetFoot() == Foot_Left
		bonus := 0.0
		if leftCorner == leftFooted {
			bonus = FOOT_BONUS
		}
		return lineupStat(player, "Passing")*0.6 + lineupStat(player, "Vision")*0.4 + bonus
	case SetPieceDuty_LongThrows:
		// Strength first; every centimetre above 175 adds some distance.
		reach := hClamp((float64(player.GetHeight())-175.0)*0.5, -5.0, 10.0)
		return hClamp(lineupStat(player, "Physicality")*0.85+
			lineupStat(player, "Stamina")*0.15+reach, 0.0, 100.0)
	}
	return 0.0
}

// SetPieces_best is the best candidate for a dead-ball duty among
// candidates (ties: lower id); nil when nobody qualifies.
func SetPieces_best(duty SetPieceDuty, candidates []*Player) *Player {
	var chosen *Player
	chosenScore := 0.0
	for _, player := range candidates {
		if player == nil {
			continue
		}
		value := SetPieces_score(duty, player)
		if value <= 0.0 {
			continue
		}
		if chosen == nil || value > chosenScore ||
			(value == chosenScore && player.GetId() < chosen.GetId()) {
			chosen = player
			chosenScore = value
		}
	}
	return chosen
}

// PositionedPlayer is Lineup::PositionedPlayer.
type PositionedPlayer struct {
	Player   *Player
	Position Vector2F
}

// StandIn is Lineup::StandIn: a starter who was replaced because he could
// not play, and the player standing in for him.
type StandIn struct {
	Regular PlayerID
	StandIn PlayerID
}

// Lineup_MAX_SUBSTITUTES is the number of substitutes named for a match.
const Lineup_MAX_SUBSTITUTES = 9

// Lineup manages the starting XI and reserves of a team.
type Lineup struct {
	goalkeeper      *Player
	outfieldPlayers []PositionedPlayer
	reserves        []*Player
	strategy        Strategy
	designations    SetPieceDesignations
	standIns        []StandIn
}

// NewLineup constructs an empty Lineup.
func NewLineup() *Lineup {
	l := &Lineup{strategy: NewStrategy()}
	l.Clear()
	return l
}

// Clear clears the outfield players and goalkeeper.
func (l *Lineup) Clear() {
	l.goalkeeper = nil
	l.outfieldPlayers = l.outfieldPlayers[:0]
	l.standIns = l.standIns[:0]
}

// SetGoalkeeper sets the goalkeeper (nil allowed).
func (l *Lineup) SetGoalkeeper(gk *Player) {
	l.goalkeeper = gk
	if gk != nil {
		l.RemoveOutfieldPlayer(gk.GetId())
		l.reserves = slices.DeleteFunc(l.reserves, func(player *Player) bool {
			return player != nil && player.GetId() == gk.GetId()
		})
	}
}

// GetGoalkeeper gets the goalkeeper.
func (l *Lineup) GetGoalkeeper() *Player { return l.goalkeeper }

// AddOutfieldPlayer adds an outfield player at the given coordinate.
func (l *Lineup) AddOutfieldPlayer(player *Player, position Vector2F) {
	if player == nil {
		return
	}
	if l.goalkeeper != nil && l.goalkeeper.GetId() == player.GetId() {
		return
	}
	for _, positioned := range l.outfieldPlayers {
		if positioned.Player != nil && positioned.Player.GetId() == player.GetId() {
			return
		}
	}
	position.X = hClamp(position.X, 0.0, 1.0)
	position.Y = hClamp(position.Y, 0.0, 1.0)
	l.outfieldPlayers = append(l.outfieldPlayers, PositionedPlayer{player, position})
}

// MoveOutfieldPlayer updates the position of an existing outfield player.
func (l *Lineup) MoveOutfieldPlayer(playerID PlayerID, newPosition Vector2F) bool {
	for i := range l.outfieldPlayers {
		posPlayer := &l.outfieldPlayers[i]
		if posPlayer.Player != nil && posPlayer.Player.GetId() == playerID {
			posPlayer.Position = Vector2F{hClamp(newPosition.X, 0.0, 1.0), hClamp(newPosition.Y, 0.0, 1.0)}
			return true
		}
	}
	return false
}

// RemoveOutfieldPlayer removes an outfield player from the pitch.
func (l *Lineup) RemoveOutfieldPlayer(playerID PlayerID) {
	l.outfieldPlayers = slices.DeleteFunc(l.outfieldPlayers, func(pp PositionedPlayer) bool {
		return pp.Player != nil && pp.Player.GetId() == playerID
	})
}

// RemovePlayer drops every reference to a player.
func (l *Lineup) RemovePlayer(playerID PlayerID) bool {
	found := false
	if l.goalkeeper != nil && l.goalkeeper.GetId() == playerID {
		l.goalkeeper = nil
		found = true
	}
	matches := func(player *Player) bool { return player != nil && player.GetId() == playerID }
	before := len(l.outfieldPlayers)
	l.outfieldPlayers = slices.DeleteFunc(l.outfieldPlayers, func(p PositionedPlayer) bool { return matches(p.Player) })
	found = found || len(l.outfieldPlayers) != before
	before = len(l.reserves)
	l.reserves = slices.DeleteFunc(l.reserves, matches)
	found = found || len(l.reserves) != before
	l.standIns = slices.DeleteFunc(l.standIns, func(entry StandIn) bool {
		return entry.Regular == playerID || entry.StandIn == playerID
	})
	for i := range l.designations {
		if l.designations[i] != playerID {
			continue
		}
		l.designations[i] = 0
		found = true
	}
	return found
}

// Lineup_roleAt is the position a slot at position asks for.
func Lineup_roleAt(position Vector2F) PlayerRole {
	// Bands match the formation presets: back line about 0.20, holding
	// midfielders 0.37-0.38, midfield 0.43-0.46, number tens and inside
	// wingers about 0.60, forwards from 0.72.
	left := position.Y < 0.2
	right := position.Y > 0.8
	if position.X < 0.32 {
		if left {
			return PlayerRole_LB
		}
		if right {
			return PlayerRole_RB
		}
		return PlayerRole_CB
	}
	if position.X < 0.55 {
		if left {
			return PlayerRole_LM
		}
		if right {
			return PlayerRole_RM
		}
		if position.X < 0.40 {
			return PlayerRole_CDM
		}
		return PlayerRole_CM
	}
	if left {
		return PlayerRole_LW
	}
	if right {
		return PlayerRole_RW
	}
	if position.X < 0.68 {
		return PlayerRole_CAM
	}
	return PlayerRole_ST
}

// GetOutfieldPlayers gets all positioned outfield players. Must not be
// modified.
func (l *Lineup) GetOutfieldPlayers() []PositionedPlayer { return l.outfieldPlayers }

// SetReserves sets the substitutes. Starters and duplicates are skipped;
// more than MAX_SUBSTITUTES are cut down with Lineup_chooseBench().
func (l *Lineup) SetReserves(subs []*Player) {
	l.reserves = l.reserves[:0:0]
	for _, player := range subs {
		if player == nil || (l.goalkeeper != nil && l.goalkeeper.GetId() == player.GetId()) ||
			slices.ContainsFunc(l.outfieldPlayers, func(positioned PositionedPlayer) bool {
				return positioned.Player != nil && positioned.Player.GetId() == player.GetId()
			}) ||
			slices.Contains(l.reserves, player) {
			continue
		}
		l.reserves = append(l.reserves, player)
	}
	if len(l.reserves) > Lineup_MAX_SUBSTITUTES {
		l.reserves = Lineup_chooseBench(l.reserves)
	}
}

// Lineup_chooseBench picks a matchday bench of at most MAX_SUBSTITUTES from
// candidates (best first).
func Lineup_chooseBench(candidates []*Player) []*Player {
	// Available players first, each group keeping the callers' order.
	ordered := make([]*Player, 0, len(candidates))
	for _, available := range []bool{true, false} {
		for _, player := range candidates {
			if player != nil && player.IsAvailable() == available && !slices.Contains(ordered, player) {
				ordered = append(ordered, player)
			}
		}
	}
	const (
		keeper = iota
		defender
		midfielder
		forward
	)
	groupOf := func(player *Player) int {
		switch player.GetRole() {
		case PlayerRole_GK:
			return keeper
		case PlayerRole_CB, PlayerRole_LB, PlayerRole_RB:
			return defender
		case PlayerRole_LW, PlayerRole_RW, PlayerRole_ST:
			return forward
		default:
			return midfielder
		}
	}
	bench := make([]*Player, 0, Lineup_MAX_SUBSTITUTES)
	// Cover first: a keeper, then one player of each outfield line.
	for _, group := range []int{keeper, defender, midfielder, forward} {
		idx := slices.IndexFunc(ordered, func(player *Player) bool {
			return groupOf(player) == group && player.IsAvailable()
		})
		if idx >= 0 && len(bench) < Lineup_MAX_SUBSTITUTES {
			bench = append(bench, ordered[idx])
		}
	}
	// Then the best of the rest, at most one more keeper.
	for _, player := range ordered {
		if len(bench) >= Lineup_MAX_SUBSTITUTES {
			break
		}
		if slices.Contains(bench, player) {
			continue
		}
		if groupOf(player) == keeper {
			keepers := 0
			for _, chosen := range bench {
				if groupOf(chosen) == keeper {
					keepers++
				}
			}
			if keepers >= 2 {
				continue
			}
		}
		bench = append(bench, player)
	}
	return bench
}

// GetReserves gets the reserve players. Must not be modified.
func (l *Lineup) GetReserves() []*Player { return l.reserves }

// SetStrategy sets the strategy for this lineup (copied).
func (l *Lineup) SetStrategy(strat *Strategy) { l.strategy = strat.Clone() }

// GetStrategy gets the strategy for this lineup.
func (l *Lineup) GetStrategy() *Strategy { return &l.strategy }

// GetDesignated returns the designated player of a duty (0 = automatic).
func (l *Lineup) GetDesignated(duty SetPieceDuty) PlayerID {
	if duty < SetPieceDuty_COUNT {
		return l.designations[duty]
	}
	return 0
}

// SetDesignated designates a player (0 = automatic) for a duty.
func (l *Lineup) SetDesignated(duty SetPieceDuty, playerID PlayerID) {
	if duty >= SetPieceDuty_COUNT {
		return
	}
	l.designations[duty] = playerID
	// One player cannot be captain and vice-captain at once.
	if playerID == 0 || !SetPieces_isLeadership(duty) {
		return
	}
	other := SetPieceDuty_Captain
	if duty == SetPieceDuty_Captain {
		other = SetPieceDuty_ViceCaptain
	}
	if l.designations[other] == playerID {
		l.designations[other] = 0
	}
}

// GetDesignations returns every designation, indexed by SetPieceDuty.
func (l *Lineup) GetDesignations() SetPieceDesignations { return l.designations }

// SetDesignations replaces every designation (persistence).
func (l *Lineup) SetDesignations(values SetPieceDesignations) { l.designations = values }

// GetStandIns returns the stand-ins currently filling a regular's place.
func (l *Lineup) GetStandIns() []StandIn { return l.standIns }

// SetStandIns replaces the stand-ins.
func (l *Lineup) SetStandIns(values []StandIn) { l.standIns = values }

// Starters returns the goalkeeper and outfield players of the XI (no nil
// entries).
func (l *Lineup) Starters() []*Player {
	result := make([]*Player, 0, len(l.outfieldPlayers)+1)
	if l.goalkeeper != nil {
		result = append(result, l.goalkeeper)
	}
	for _, positioned := range l.outfieldPlayers {
		if positioned.Player != nil {
			result = append(result, positioned.Player)
		}
	}
	return result
}

// IsStarter is true when the player is in the starting XI.
func (l *Lineup) IsStarter(playerID PlayerID) bool {
	if playerID == 0 {
		return false
	}
	if l.goalkeeper != nil && l.goalkeeper.GetId() == playerID {
		return true
	}
	return slices.ContainsFunc(l.outfieldPlayers, func(positioned PositionedPlayer) bool {
		return positioned.Player != nil && positioned.Player.GetId() == playerID
	})
}

// EffectiveTaker is who performs a dead-ball duty at kick-off.
func (l *Lineup) EffectiveTaker(duty SetPieceDuty) *Player {
	if duty >= SetPieceDuty_COUNT {
		return nil
	}
	xi := l.Starters()
	starter := func(playerID PlayerID) *Player {
		if playerID == 0 {
			return nil
		}
		for _, player := range xi {
			if player.GetId() == playerID {
				return player
			}
		}
		return nil
	}
	if duty == SetPieceDuty_Captain {
		captainID := l.GetDesignated(SetPieceDuty_Captain)
		if captainID == 0 {
			return nil
		}
		if captain := starter(captainID); captain != nil {
			return captain
		}
		return starter(l.GetDesignated(SetPieceDuty_ViceCaptain))
	}
	if duty == SetPieceDuty_ViceCaptain {
		return starter(l.GetDesignated(SetPieceDuty_ViceCaptain))
	}
	if designated := starter(l.GetDesignated(duty)); designated != nil && SetPieces_score(duty, designated) > 0.0 {
		return designated
	}
	return SetPieces_best(duty, xi)
}
