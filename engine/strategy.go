// Ported from Player12 src/model/strategy.h and src/model/strategy.cpp (GPL-3.0)

package engine

import (
	"math"
	"slices"
)

// StrategySliders are team-wide tactical preferences in [0, 1].
//
// MatchEngine combines these base values with score state, relative team
// strength and fading shouts in computeEffectiveSliders().
type StrategySliders struct {
	Pressing      float64 // Team-wide intensity of pressing
	RiskTaking    float64 // Aggressiveness in attack vs. defensive caution
	OffensiveBias float64 // Likelihood to pass forward vs. safer options
	WidthUsage    float64 // Horizontal spread of the team
	Compactness   float64 // Squeeze defensive lines and follow the ball
}

// NewStrategySliders returns StrategySliders with the C++ defaults (0.5).
func NewStrategySliders() StrategySliders {
	return StrategySliders{0.5, 0.5, 0.5, 0.5, 0.5}
}

// RoleWeights are legacy grid-based strategy weights, retained by the
// Strategy API. The live MatchEngine does not use them.
type RoleWeights struct {
	AttackWeight   float64 // Tendency to engage in offensive actions
	DefenseWeight  float64 // Tendency to engage in defensive actions
	MovementRadius int     // Distance from lineup position
}

// NewRoleWeights returns RoleWeights with the C++ defaults.
func NewRoleWeights() RoleWeights { return RoleWeights{0.5, 0.5, 1} }

// Strategy is the tactical setup of a team for a match: sliders, tactical
// slot/keeper roles, possession offsets and opposition orders.
//
// C++ copies Strategy by value; in Go use Clone() when an independent copy
// is needed (the slices would otherwise be shared).
type Strategy struct {
	sliders          StrategySliders
	goalkeeper       RoleWeights     // single goalkeeper
	outfield         [10]RoleWeights // 10 outfield players
	slots            []SlotInstruction
	keeperRole       TacticalRole
	oppositionOrders []PlayerInstruction
}

// NewStrategy is the default constructor: default sliders and weights.
func NewStrategy() Strategy {
	s := Strategy{sliders: NewStrategySliders(), goalkeeper: RoleWeights{0.5, 0.5, 1}}
	for i := range s.outfield {
		s.outfield[i] = RoleWeights{0.5, 0.5, 1}
	}
	return s
}

// Clone returns a deep copy (the C++ copy constructor). Go-only.
func (s *Strategy) Clone() Strategy {
	c := *s
	c.slots = slices.Clone(s.slots)
	c.oppositionOrders = slices.Clone(s.oppositionOrders)
	return c
}

func (s *Strategy) gridWeight(roleIndex int, gridPosition int, attack bool) float64 {
	if roleIndex < 0 || roleIndex > 10 {
		return 0.0 // invalid index
	}
	role := &s.goalkeeper
	if roleIndex != 0 {
		role = &s.outfield[roleIndex-1]
	}
	defaultPosition := 0
	if roleIndex != 0 {
		defaultPosition = roleIndex - 1
	}
	roleRow, roleCol := ToRowCol(defaultPosition)
	cellRow, cellCol := ToRowCol(gridPosition)
	dist := hAbsInt(cellRow-roleRow) + hAbsInt(cellCol-roleCol)
	if dist > role.MovementRadius {
		return 0.0
	}
	if attack {
		return role.AttackWeight
	}
	return role.DefenseWeight
}

// GetAttackWeight gets the attack weight of a player at a grid cell (0 if
// outside the movement area).
func (s *Strategy) GetAttackWeight(roleIndex int, gridPosition int) float64 {
	return s.gridWeight(roleIndex, gridPosition, true)
}

// GetDefenseWeight gets the defense weight of a player at a grid cell (0 if
// outside the movement area).
func (s *Strategy) GetDefenseWeight(roleIndex int, gridPosition int) float64 {
	return s.gridWeight(roleIndex, gridPosition, false)
}

// SetPressing sets the pressing slider value.
func (s *Strategy) SetPressing(value float64) { s.sliders.Pressing = hClamp(value, 0.0, 1.0) }

// SetRiskTaking sets the risk taking slider value.
func (s *Strategy) SetRiskTaking(value float64) { s.sliders.RiskTaking = hClamp(value, 0.0, 1.0) }

// SetOffensiveBias sets the offensive bias slider value.
func (s *Strategy) SetOffensiveBias(value float64) {
	s.sliders.OffensiveBias = hClamp(value, 0.0, 1.0)
}

// SetWidthUsage sets the width usage slider value.
func (s *Strategy) SetWidthUsage(value float64) { s.sliders.WidthUsage = hClamp(value, 0.0, 1.0) }

// SetCompactness sets the compactness slider value.
func (s *Strategy) SetCompactness(value float64) {
	s.sliders.Compactness = hClamp(value, 0.0, 1.0)
}

// SetOutfieldWeights sets the role weights for an outfield player (0-9).
func (s *Strategy) SetOutfieldWeights(playerIndex int, attack, defense float64, radius int) {
	if playerIndex < 0 || playerIndex >= 10 {
		return
	}
	s.outfield[playerIndex].AttackWeight = hClamp(attack, 0.0, 1.0)
	s.outfield[playerIndex].DefenseWeight = hClamp(defense, 0.0, 1.0)
	s.outfield[playerIndex].MovementRadius = max(0, radius)
}

// GetRole gets the role weights for a player (index clamped to 0-10).
func (s *Strategy) GetRole(roleIndex int) *RoleWeights {
	roleIndex = min(max(roleIndex, 0), 10)
	if roleIndex == 0 {
		return &s.goalkeeper
	}
	return &s.outfield[roleIndex-1]
}

// SetAllSliders sets all sliders at once (clamped).
func (s *Strategy) SetAllSliders(newSliders StrategySliders) {
	s.sliders.Pressing = hClamp(newSliders.Pressing, 0.0, 1.0)
	s.sliders.RiskTaking = hClamp(newSliders.RiskTaking, 0.0, 1.0)
	s.sliders.OffensiveBias = hClamp(newSliders.OffensiveBias, 0.0, 1.0)
	s.sliders.WidthUsage = hClamp(newSliders.WidthUsage, 0.0, 1.0)
	s.sliders.Compactness = hClamp(newSliders.Compactness, 0.0, 1.0)
}

// GetSliders retrieves the current slider state.
func (s *Strategy) GetSliders() StrategySliders { return s.sliders }

// SetAllOutfieldWeights applies the same weights to all outfield players.
func (s *Strategy) SetAllOutfieldWeights(attack, defense float64, radius int) {
	for i := 0; i < 10; i++ {
		s.SetOutfieldWeights(i, attack, defense, radius)
	}
}

// GetSlotInstructions returns every stored slot instruction (all formations
// used so far). Must not be modified.
func (s *Strategy) GetSlotInstructions() []SlotInstruction { return s.slots }

// File-private values of strategy.cpp.
const (
	// Anchors closer than this are the same slot.
	strategy_SAME_SLOT_DISTANCE float64 = 0.01
	// Slot instructions remembered across formations.
	strategy_MAX_SLOT_INSTRUCTIONS = 40
)

func strategyAnchorDistance(first, second Vector2F) float64 {
	return math.Hypot(first.X-second.X, first.Y-second.Y)
}

func strategySanitized(instruction SlotInstruction) SlotInstruction {
	finite := func(value float64) bool { return !math.IsInf(value, 0) && !math.IsNaN(value) }
	if !finite(instruction.Anchor.X) || !finite(instruction.Anchor.Y) {
		instruction.Anchor = Vector2F{0.5, 0.5}
	}
	instruction.Anchor.X = hClamp(instruction.Anchor.X, 0.0, 1.0)
	instruction.Anchor.Y = hClamp(instruction.Anchor.Y, 0.0, 1.0)
	if instruction.Role >= TacticalRole_COUNT {
		instruction.Role = TacticalRole_Standard
	}
	if instruction.Duty >= RoleDuty_COUNT {
		instruction.Duty = RoleDuty_Support
	}
	offset := &instruction.PossessionOffset
	if !finite(offset.X) {
		offset.X = 0.0
	}
	if !finite(offset.Y) {
		offset.Y = 0.0
	}
	// The in-possession spot stays on the pitch.
	offset.X = hClamp(instruction.Anchor.X+offset.X, 0.02, 0.98) - instruction.Anchor.X
	offset.Y = hClamp(instruction.Anchor.Y+offset.Y, 0.02, 0.98) - instruction.Anchor.Y
	return instruction
}

// SetSlotInstructions replaces the slot instructions (persistence).
func (s *Strategy) SetSlotInstructions(instructions []SlotInstruction) {
	s.slots = s.slots[:0:0]
	for _, instruction := range instructions {
		s.SetSlot(instruction)
	}
}

// FindSlot returns the instruction of the slot at anchor (lineup
// coordinates): the nearest stored one within
// TacticsTuning_SLOT_MATCH_DISTANCE, else nil.
func (s *Strategy) FindSlot(anchor Vector2F) *SlotInstruction {
	var nearest *SlotInstruction
	best := TacticsTuning_SLOT_MATCH_DISTANCE
	for i := range s.slots {
		gap := strategyAnchorDistance(s.slots[i].Anchor, anchor)
		if gap <= best {
			best = gap
			nearest = &s.slots[i]
		}
	}
	return nearest
}

// SetSlot stores the instruction of the slot at its anchor (replacing it).
func (s *Strategy) SetSlot(instruction SlotInstruction) {
	clean := strategySanitized(instruction)
	s.slots = slices.DeleteFunc(s.slots, func(stored SlotInstruction) bool {
		return strategyAnchorDistance(stored.Anchor, clean.Anchor) < strategy_SAME_SLOT_DISTANCE
	})
	s.slots = append(s.slots, clean)
	// The oldest instructions (formations not used for a while) go first.
	if len(s.slots) > strategy_MAX_SLOT_INSTRUCTIONS {
		s.slots = slices.Delete(s.slots, 0, len(s.slots)-strategy_MAX_SLOT_INSTRUCTIONS)
	}
}

// GetKeeperRole returns the goalkeeper's tactical role.
func (s *Strategy) GetKeeperRole() TacticalRole { return s.keeperRole }

// SetKeeperRole sets the goalkeeper's role (Standard if not allowed).
func (s *Strategy) SetKeeperRole(role TacticalRole) {
	if Tactics_allows(RoleFamily_Goalkeeper, role) {
		s.keeperRole = role
	} else {
		s.keeperRole = TacticalRole_Standard
	}
}

// GetOppositionOrders returns the instructions against players of the next
// opponent. Must not be modified.
func (s *Strategy) GetOppositionOrders() []PlayerInstruction { return s.oppositionOrders }

// SetOppositionOrders stores the opposition orders, dropping empty ones.
func (s *Strategy) SetOppositionOrders(orders []PlayerInstruction) {
	orders = slices.DeleteFunc(slices.Clone(orders), func(order PlayerInstruction) bool {
		return order.Player == 0 || order.Instruction == OppositionInstruction_None ||
			order.Instruction >= OppositionInstruction_COUNT
	})
	s.oppositionOrders = orders
}
