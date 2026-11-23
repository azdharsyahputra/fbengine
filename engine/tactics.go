// Ported from Player12 src/model/tactics.h and src/model/tactics.cpp (GPL-3.0)

package engine

import "math"

// RoleFamily is the position group of a formation slot; it decides which
// roles the slot can be given (see Tactics_familyForSlot).
type RoleFamily uint8

const (
	RoleFamily_Goalkeeper RoleFamily = iota
	RoleFamily_FullBack
	RoleFamily_CentreBack
	RoleFamily_Holding
	RoleFamily_Central
	RoleFamily_Attacking
	RoleFamily_Wide
	RoleFamily_Striker
	RoleFamily_COUNT
)

// TacticalRole is how a player interprets his slot (values are persisted in
// the tactic). Standard plays the position as drilled, with no extra bias.
type TacticalRole uint8

const (
	TacticalRole_Standard        TacticalRole = 0
	TacticalRole_LineKeeper      TacticalRole = 1  // Stays on his line.
	TacticalRole_SweeperKeeper   TacticalRole = 2  // Starts high and comes out for balls in behind.
	TacticalRole_InsideFullBack  TacticalRole = 3  // Steps into midfield when his side has the ball.
	TacticalRole_Stopper         TacticalRole = 4  // Centre-back who steps out to meet his man.
	TacticalRole_CoverDefender   TacticalRole = 5  // Centre-back who drops off and sweeps behind.
	TacticalRole_Anchor          TacticalRole = 6  // Holds in front of the back line.
	TacticalRole_Playmaker       TacticalRole = 7  // Dictates with riskier, forward passes.
	TacticalRole_BoxToBox        TacticalRole = 8  // Covers both boxes, joins attacks late.
	TacticalRole_SecondStriker   TacticalRole = 9  // Runs beyond the striker from deep.
	TacticalRole_InsideForward   TacticalRole = 10 // Wide forward who cuts in to shoot.
	TacticalRole_TouchlineWinger TacticalRole = 11 // Hugs the line to beat his man and cross.
	TacticalRole_TargetForward   TacticalRole = 12 // Reference point for forward passes.
	TacticalRole_Poacher         TacticalRole = 13 // Lives on the last line, gambles in the box.
	TacticalRole_PressingForward TacticalRole = 14 // First defender, hunts the ball.
	TacticalRole_FalseNine       TacticalRole = 15 // Drops off the line to link play.
	TacticalRole_COUNT           TacticalRole = 16
)

// RoleDuty is the attacking commitment of a role (persisted in the tactic).
type RoleDuty uint8

const (
	RoleDuty_Defend RoleDuty = iota
	RoleDuty_Support
	RoleDuty_Attack
	RoleDuty_COUNT
)

// OppositionInstruction is an individual instruction against one opposing
// player (values are persisted in OppositionInstructions).
type OppositionInstruction uint8

const (
	OppositionInstruction_None         OppositionInstruction = 0
	OppositionInstruction_TightMark    OppositionInstruction = 1 // Stay tight to him; his marker follows him everywhere.
	OppositionInstruction_Press        OppositionInstruction = 2 // Close him down as soon as he receives.
	OppositionInstruction_ShowWeakFoot OppositionInstruction = 3 // Force him onto his weaker foot.
	OppositionInstruction_DoubleUp     OppositionInstruction = 4 // Two players close him down (wide threats).
	OppositionInstruction_COUNT        OppositionInstruction = 5
)

// RoleProfile is the measurable behaviour of a role and duty in the match
// engine. Distances are metres, biases are engine utility units; zero
// everywhere is the Standard role on a Support duty.
type RoleProfile struct {
	// Extra depth toward the opponents' goal while his side has the ball.
	PossessionAdvanceMetres float64
	// Toward his touchline (+) or the middle (-) while his side has it.
	PossessionWidthMetres float64
	// Higher (+) or deeper (-) in the defensive block.
	DefensiveAdvanceMetres float64
	// How keen he is to make runs in behind and into the box.
	RunBias float64
	// Full-backs: how keen to overlap; -1 or less never overlaps.
	OverlapBias float64
	// Appetite for forward passes over safe ones.
	PassDaring float64
	// How much team-mates look for him with forward passes.
	TargetBias float64
	// Eagerness to close down the ball (pressing trigger).
	PressBias float64
	// Willingness to shoot.
	ShotBias float64
	// Wide players: +1 cuts inside, -1 keeps to the line, 0 mixes.
	CutInside float64
	// Goalkeepers: set position further from (+) or nearer (-) his goal.
	KeeperDepthMetres float64
	// Goalkeepers: readiness to sweep balls in behind (+) or not (-).
	KeeperSweep float64
}

// SlotInstruction is the role, duty and in-possession spot of one outfield
// slot. Slots are matched by their out-of-possession position (`Anchor`,
// lineup coordinates: own goal at x = 0).
type SlotInstruction struct {
	Anchor Vector2F
	Role   TacticalRole
	Duty   RoleDuty
	// Shift of the slot while his side has the ball (lineup coordinates).
	PossessionOffset Vector2F
}

// NewSlotInstruction returns a SlotInstruction with the C++ defaults
// (Standard role, Support duty).
func NewSlotInstruction() SlotInstruction {
	return SlotInstruction{Role: TacticalRole_Standard, Duty: RoleDuty_Support}
}

// PlayerInstruction is an instruction against an opposing player for the
// next match.
type PlayerInstruction struct {
	Player      PlayerID
	Instruction OppositionInstruction
}

// PossessionShape lists the ready-made in-possession shapes.
type PossessionShape uint8

const (
	PossessionShape_KeepShape      PossessionShape = iota // Same shape with and without the ball.
	PossessionShape_FullBacksPush                         // Both full-backs join the midfield line.
	PossessionShape_BuildWithThree                        // One full-back tucks in, the other pushes on.
	PossessionShape_NarrowFront                           // Wide players come inside, full-backs give width.
	PossessionShape_COUNT
)

// Engine weights of the role biases and the instructions (namespace
// TacticsTuning).
const (
	// Slots further than this from every stored anchor play Standard.
	TacticsTuning_SLOT_MATCH_DISTANCE         float64 = 0.06
	TacticsTuning_RUN_BIAS_WEIGHT             float64 = 0.22
	TacticsTuning_OVERLAP_BIAS_WEIGHT         float64 = 0.25
	TacticsTuning_PRESS_BIAS_SECONDS          float64 = 0.6
	TacticsTuning_PASS_DARING_PROGRESS_WEIGHT float64 = 3.0
	TacticsTuning_PASS_DARING_SAFETY_WEIGHT   float64 = 1.0
	TacticsTuning_TARGET_BIAS_WEIGHT          float64 = 1.0
	TacticsTuning_SHOT_BIAS_WEIGHT            float64 = 2.0
	TacticsTuning_CUT_INSIDE_SHIFT            float64 = 0.9
	TacticsTuning_KEEPER_SWEEP_DEPTH_GAIN     float64 = 0.5
	TacticsTuning_KEEPER_SWEEP_ADVANTAGE_GAIN float64 = 0.15
	// Engagement rate gained per unit of press bias.
	TacticsTuning_PRESS_BIAS_ENGAGE float64 = 0.35

	// Tight marking: share of the way to the man and goal-side metres.
	TacticsTuning_TIGHT_MARK_WEIGHT           float64 = 0.88
	TacticsTuning_TIGHT_MARK_GOAL_SIDE_METRES float64 = 1.2
	// Completion lost by a pass to a man whose tight marker is this close.
	TacticsTuning_TIGHT_MARK_REACH_METRES       float64 = 3.0
	TacticsTuning_TIGHT_MARK_COMPLETION_PENALTY float64 = 0.10
	// Pressing order: stand-off share kept and engagement gained.
	TacticsTuning_PRESS_ORDER_STANDOFF_SHARE float64 = 0.45
	TacticsTuning_PRESS_ORDER_ENGAGE         float64 = 0.8
	// Weaker foot: presser shade (metres), execution error and shot penalty
	// at full pressure.
	TacticsTuning_WEAK_FOOT_SHADE_METRES float64 = 1.5
	TacticsTuning_WEAK_FOOT_ERROR_GAIN   float64 = 0.45
	TacticsTuning_WEAK_FOOT_SHOT_PENALTY float64 = 0.35
	// Doubling up: second man's distance, engagement and take-on penalty.
	TacticsTuning_DOUBLE_UP_SUPPORT_METRES  float64 = 2.5
	TacticsTuning_DOUBLE_UP_ENGAGE          float64 = 0.4
	TacticsTuning_DOUBLE_UP_TAKE_ON_PENALTY float64 = 0.12
	TacticsTuning_DOUBLE_UP_REACH_METRES    float64 = 5.0

	// Team talks: full effect for the first minutes of a half, then fading
	// out; a talk at REFERENCE (the team-talk cap) gives the full swing.
	TacticsTuning_TALK_FULL_MINUTES     float64 = 10.0
	TacticsTuning_TALK_FADE_END_MINUTES float64 = 20.0
	TacticsTuning_TALK_REFERENCE        float64 = 0.012
	// Decision noise removed and pressing eagerness added at full swing.
	TacticsTuning_TALK_COMPOSURE     float64 = 0.25
	TacticsTuning_TALK_PRESS_SECONDS float64 = 0.3
	TacticsTuning_TALK_ENGAGE        float64 = 0.25
)

// File-private values of tactics.cpp (prefixed to stay unique in the package).
const (
	// Slot classification (lineup coordinates, own goal at x = 0).
	tactics_DEFENCE_LINE_X         float64 = 0.30
	tactics_HOLDING_LINE_X         float64 = 0.42
	tactics_MIDFIELD_LINE_X        float64 = 0.55
	tactics_ATTACKING_LINE_X       float64 = 0.68
	tactics_WIDE_DEVIATION         float64 = 0.28
	tactics_STRIKER_WIDE_DEVIATION float64 = 0.25

	// A special role has to fit this much better than Standard to be picked.
	tactics_ROLE_PREFERENCE_MARGIN float64 = 2.0
	// Full-back duties from pace, stamina and defending (attribute points).
	tactics_ATTACKING_FULL_BACK_MARGIN float64 = 5.0
	tactics_DEFENSIVE_FULL_BACK_MARGIN float64 = 8.0
	// Stat used when a player lacks one (the 0-100 midpoint).
	tactics_DEFAULT_STAT float64 = 50.0
)

var (
	tactics_GOALKEEPER_ROLES  = []TacticalRole{TacticalRole_Standard, TacticalRole_LineKeeper, TacticalRole_SweeperKeeper}
	tactics_FULL_BACK_ROLES   = []TacticalRole{TacticalRole_Standard, TacticalRole_InsideFullBack}
	tactics_CENTRE_BACK_ROLES = []TacticalRole{TacticalRole_Standard, TacticalRole_Stopper, TacticalRole_CoverDefender}
	tactics_HOLDING_ROLES     = []TacticalRole{TacticalRole_Standard, TacticalRole_Anchor, TacticalRole_Playmaker}
	tactics_CENTRAL_ROLES     = []TacticalRole{TacticalRole_Standard, TacticalRole_BoxToBox, TacticalRole_Playmaker, TacticalRole_Anchor}
	tactics_ATTACKING_ROLES   = []TacticalRole{TacticalRole_Standard, TacticalRole_Playmaker, TacticalRole_SecondStriker}
	tactics_WIDE_ROLES        = []TacticalRole{TacticalRole_Standard, TacticalRole_InsideForward, TacticalRole_TouchlineWinger}
	tactics_STRIKER_ROLES     = []TacticalRole{TacticalRole_Standard, TacticalRole_TargetForward, TacticalRole_Poacher, TacticalRole_PressingForward, TacticalRole_FalseNine}

	tactics_ROLE_KEYS = [TacticalRole_COUNT]string{
		"ROLE_STANDARD", "ROLE_LINE_KEEPER", "ROLE_SWEEPER_KEEPER",
		"ROLE_INSIDE_FULL_BACK", "ROLE_STOPPER", "ROLE_COVER_DEFENDER",
		"ROLE_ANCHOR", "ROLE_PLAYMAKER", "ROLE_BOX_TO_BOX",
		"ROLE_SECOND_STRIKER", "ROLE_INSIDE_FORWARD", "ROLE_TOUCHLINE_WINGER",
		"ROLE_TARGET_FORWARD", "ROLE_POACHER", "ROLE_PRESSING_FORWARD",
		"ROLE_FALSE_NINE"}

	tactics_ROLE_DESCRIPTION_KEYS = [TacticalRole_COUNT]string{
		"ROLE_STANDARD_HELP", "ROLE_LINE_KEEPER_HELP",
		"ROLE_SWEEPER_KEEPER_HELP", "ROLE_INSIDE_FULL_BACK_HELP",
		"ROLE_STOPPER_HELP", "ROLE_COVER_DEFENDER_HELP",
		"ROLE_ANCHOR_HELP", "ROLE_PLAYMAKER_HELP",
		"ROLE_BOX_TO_BOX_HELP", "ROLE_SECOND_STRIKER_HELP",
		"ROLE_INSIDE_FORWARD_HELP", "ROLE_TOUCHLINE_WINGER_HELP",
		"ROLE_TARGET_FORWARD_HELP", "ROLE_POACHER_HELP",
		"ROLE_PRESSING_FORWARD_HELP", "ROLE_FALSE_NINE_HELP"}

	tactics_FAMILY_KEYS = [RoleFamily_COUNT]string{
		"ROLE_FAMILY_GOALKEEPER", "ROLE_FAMILY_FULL_BACK",
		"ROLE_FAMILY_CENTRE_BACK", "ROLE_FAMILY_HOLDING",
		"ROLE_FAMILY_CENTRAL", "ROLE_FAMILY_ATTACKING",
		"ROLE_FAMILY_WIDE", "ROLE_FAMILY_STRIKER"}

	tactics_DUTY_KEYS = [RoleDuty_COUNT]string{"ROLE_DUTY_DEFEND", "ROLE_DUTY_SUPPORT", "ROLE_DUTY_ATTACK"}

	tactics_SHAPE_KEYS = [PossessionShape_COUNT]string{
		"TACTIC_SHAPE_KEEP", "TACTIC_SHAPE_FULL_BACKS",
		"TACTIC_SHAPE_BACK_THREE", "TACTIC_SHAPE_NARROW"}

	// Key attributes, most important first.
	tactics_GOALKEEPER_STANDARD  = []string{"Goalkeeping", "Physicality"}
	tactics_FULL_BACK_STANDARD   = []string{"Pace", "Defending", "Stamina"}
	tactics_CENTRE_BACK_STANDARD = []string{"Defending", "Physicality"}
	tactics_HOLDING_STANDARD     = []string{"Defending", "Passing", "Stamina"}
	tactics_CENTRAL_STANDARD     = []string{"Passing", "Stamina", "Vision"}
	tactics_ATTACKING_STANDARD   = []string{"Passing", "Vision", "Dribbling"}
	tactics_WIDE_STANDARD        = []string{"Pace", "Dribbling", "Passing"}
	tactics_STRIKER_STANDARD     = []string{"Shooting", "Pace", "Physicality"}

	tactics_LINE_KEEPER      = []string{"Goalkeeping", "Physicality"}
	tactics_SWEEPER_KEEPER   = []string{"Goalkeeping", "Pace", "Passing"}
	tactics_INSIDE_FULL_BACK = []string{"Passing", "Vision", "Defending"}
	tactics_STOPPER          = []string{"Physicality", "Defending", "Pace"}
	tactics_COVER_DEFENDER   = []string{"Pace", "Defending", "Vision"}
	tactics_ANCHOR           = []string{"Defending", "Physicality", "Passing"}
	tactics_PLAYMAKER        = []string{"Passing", "Vision", "Dribbling"}
	tactics_BOX_TO_BOX       = []string{"Stamina", "Physicality", "Passing"}
	tactics_SECOND_STRIKER   = []string{"Shooting", "Pace", "Dribbling"}
	tactics_INSIDE_FORWARD   = []string{"Shooting", "Dribbling", "Pace"}
	tactics_TOUCHLINE_WINGER = []string{"Pace", "Dribbling", "Passing"}
	tactics_TARGET_FORWARD   = []string{"Physicality", "Shooting", "Passing"}
	tactics_POACHER          = []string{"Shooting", "Pace"}
	tactics_PRESSING_FORWARD = []string{"Stamina", "Physicality", "Pace"}
	tactics_FALSE_NINE       = []string{"Passing", "Vision", "Dribbling"}
)

// tacticsStat is tactics.cpp's anonymous `stat` (renamed: lineup.cpp has a
// different `stat`).
func tacticsStat(player *Player, name string) float64 {
	if value, found := player.GetStats()[name]; found {
		return value
	}
	return tactics_DEFAULT_STAT
}

// tacticsInward is tactics.cpp's anonymous `inward`: toward the middle of
// the pitch from the anchor's side.
func tacticsInward(anchor Vector2F, amount float64) float64 {
	if anchor.Y < 0.5 {
		return amount
	}
	return -amount
}

// Tactics_familyForSlot is the position group of a slot from its lineup
// position.
func Tactics_familyForSlot(anchor Vector2F) RoleFamily {
	width := math.Abs(anchor.Y - 0.5)
	wide := width > tactics_WIDE_DEVIATION
	pick := func(isWide bool, ifWide, otherwise RoleFamily) RoleFamily {
		if isWide {
			return ifWide
		}
		return otherwise
	}
	if anchor.X < tactics_DEFENCE_LINE_X {
		return pick(wide, RoleFamily_FullBack, RoleFamily_CentreBack)
	}
	if anchor.X < tactics_HOLDING_LINE_X {
		return pick(wide, RoleFamily_FullBack, RoleFamily_Holding)
	}
	if anchor.X < tactics_MIDFIELD_LINE_X {
		return pick(wide, RoleFamily_Wide, RoleFamily_Central)
	}
	if anchor.X < tactics_ATTACKING_LINE_X {
		return pick(wide, RoleFamily_Wide, RoleFamily_Attacking)
	}
	return pick(width > tactics_STRIKER_WIDE_DEVIATION, RoleFamily_Wide, RoleFamily_Striker)
}

// Tactics_rolesFor returns the roles a family can use; Standard is always
// first. The returned slice must not be modified.
func Tactics_rolesFor(family RoleFamily) []TacticalRole {
	switch family {
	case RoleFamily_Goalkeeper:
		return tactics_GOALKEEPER_ROLES
	case RoleFamily_FullBack:
		return tactics_FULL_BACK_ROLES
	case RoleFamily_CentreBack:
		return tactics_CENTRE_BACK_ROLES
	case RoleFamily_Holding:
		return tactics_HOLDING_ROLES
	case RoleFamily_Central:
		return tactics_CENTRAL_ROLES
	case RoleFamily_Attacking:
		return tactics_ATTACKING_ROLES
	case RoleFamily_Wide:
		return tactics_WIDE_ROLES
	}
	return tactics_STRIKER_ROLES
}

// Tactics_allows reports whether role is allowed for family.
func Tactics_allows(family RoleFamily, role TacticalRole) bool {
	for _, allowed := range Tactics_rolesFor(family) {
		if allowed == role {
			return true
		}
	}
	return false
}

// Tactics_hasDuty reports whether the family's slots take a duty
// (goalkeepers do not).
func Tactics_hasDuty(family RoleFamily) bool { return family != RoleFamily_Goalkeeper }

// Tactics_profile is the engine profile of a role played with a duty.
func Tactics_profile(role TacticalRole, duty RoleDuty) RoleProfile {
	// Roles are composable numeric biases over the common match AI, rather
	// than separate player controllers. Start neutral, apply the role, then
	// the duty.
	var result RoleProfile
	switch role {
	case TacticalRole_Standard, TacticalRole_COUNT:
	case TacticalRole_LineKeeper:
		result.KeeperDepthMetres = -2.5
		result.KeeperSweep = -0.6
	case TacticalRole_SweeperKeeper:
		result.KeeperDepthMetres = 5.0
		result.KeeperSweep = 1.0
		result.PassDaring = 0.1
	case TacticalRole_InsideFullBack:
		result.PossessionAdvanceMetres = 4.0
		result.PossessionWidthMetres = -14.0
		result.OverlapBias = -1.0
		result.PassDaring = 0.05
	case TacticalRole_Stopper:
		result.DefensiveAdvanceMetres = 4.0
		result.PressBias = 0.4
	case TacticalRole_CoverDefender:
		result.DefensiveAdvanceMetres = -4.0
		result.PressBias = -0.4
	case TacticalRole_Anchor:
		result.PossessionAdvanceMetres = -6.0
		result.DefensiveAdvanceMetres = -3.0
		result.RunBias = -1.0
		result.PassDaring = -0.15
		result.ShotBias = -0.15
		result.PressBias = -0.2
	case TacticalRole_Playmaker:
		result.PossessionAdvanceMetres = -2.0
		result.RunBias = -0.4
		result.PassDaring = 0.25
		result.TargetBias = 0.1
		result.ShotBias = -0.05
	case TacticalRole_BoxToBox:
		result.PossessionAdvanceMetres = 3.0
		result.RunBias = 0.6
		result.PressBias = 0.2
		result.ShotBias = 0.05
	case TacticalRole_SecondStriker:
		result.PossessionAdvanceMetres = 5.0
		result.RunBias = 0.8
		result.ShotBias = 0.12
		result.PressBias = -0.1
	case TacticalRole_InsideForward:
		result.PossessionWidthMetres = -6.0
		result.RunBias = 0.3
		result.ShotBias = 0.12
		result.CutInside = 1.0
	case TacticalRole_TouchlineWinger:
		result.PossessionWidthMetres = 6.0
		result.RunBias = -0.1
		result.ShotBias = -0.1
		result.CutInside = -1.0
	case TacticalRole_TargetForward:
		result.PossessionAdvanceMetres = 2.0
		result.RunBias = -1.0
		result.TargetBias = 0.25
		result.ShotBias = 0.05
		result.PressBias = -0.2
	case TacticalRole_Poacher:
		result.PossessionAdvanceMetres = 3.0
		result.DefensiveAdvanceMetres = 2.0
		result.RunBias = 0.8
		result.ShotBias = 0.15
		result.PressBias = -0.5
	case TacticalRole_PressingForward:
		result.DefensiveAdvanceMetres = 4.0
		result.RunBias = 0.2
		result.PressBias = 1.0
	case TacticalRole_FalseNine:
		result.PossessionAdvanceMetres = -10.0
		result.RunBias = -1.5
		result.PassDaring = 0.2
		result.TargetBias = 0.1
		result.ShotBias = -0.08
	}
	if role == TacticalRole_LineKeeper || role == TacticalRole_SweeperKeeper {
		return result
	}
	switch duty {
	case RoleDuty_Defend:
		result.PossessionAdvanceMetres -= 5.0
		result.RunBias -= 0.5
		result.OverlapBias -= 1.0
		result.PassDaring -= 0.1
		result.ShotBias -= 0.1
	case RoleDuty_Attack:
		result.PossessionAdvanceMetres += 5.0
		result.RunBias += 0.5
		result.OverlapBias += 0.6
		result.PassDaring += 0.05
		result.ShotBias += 0.05
	case RoleDuty_Support, RoleDuty_COUNT:
	}
	return result
}

// Tactics_familyKey is the language key of a family.
func Tactics_familyKey(family RoleFamily) string {
	if family < RoleFamily_COUNT {
		return tactics_FAMILY_KEYS[family]
	}
	return ""
}

// Tactics_roleKey is the language key of a role.
func Tactics_roleKey(role TacticalRole) string {
	if role < TacticalRole_COUNT {
		return tactics_ROLE_KEYS[role]
	}
	return ""
}

// Tactics_roleDescriptionKey is the language key of a role's description.
func Tactics_roleDescriptionKey(role TacticalRole) string {
	if role < TacticalRole_COUNT {
		return tactics_ROLE_DESCRIPTION_KEYS[role]
	}
	return ""
}

// Tactics_dutyKey is the language key of a duty.
func Tactics_dutyKey(duty RoleDuty) string {
	if duty < RoleDuty_COUNT {
		return tactics_DUTY_KEYS[duty]
	}
	return ""
}

// Tactics_possessionShapeKey is the language key of a possession shape.
func Tactics_possessionShapeKey(shape PossessionShape) string {
	if shape < PossessionShape_COUNT {
		return tactics_SHAPE_KEYS[shape]
	}
	return ""
}

// Tactics_keyAttributes returns the attributes that matter most for a role
// in family (names of the player's stats). Must not be modified.
func Tactics_keyAttributes(family RoleFamily, role TacticalRole) []string {
	switch role {
	case TacticalRole_LineKeeper:
		return tactics_LINE_KEEPER
	case TacticalRole_SweeperKeeper:
		return tactics_SWEEPER_KEEPER
	case TacticalRole_InsideFullBack:
		return tactics_INSIDE_FULL_BACK
	case TacticalRole_Stopper:
		return tactics_STOPPER
	case TacticalRole_CoverDefender:
		return tactics_COVER_DEFENDER
	case TacticalRole_Anchor:
		return tactics_ANCHOR
	case TacticalRole_Playmaker:
		return tactics_PLAYMAKER
	case TacticalRole_BoxToBox:
		return tactics_BOX_TO_BOX
	case TacticalRole_SecondStriker:
		return tactics_SECOND_STRIKER
	case TacticalRole_InsideForward:
		return tactics_INSIDE_FORWARD
	case TacticalRole_TouchlineWinger:
		return tactics_TOUCHLINE_WINGER
	case TacticalRole_TargetForward:
		return tactics_TARGET_FORWARD
	case TacticalRole_Poacher:
		return tactics_POACHER
	case TacticalRole_PressingForward:
		return tactics_PRESSING_FORWARD
	case TacticalRole_FalseNine:
		return tactics_FALSE_NINE
	}
	switch family {
	case RoleFamily_Goalkeeper:
		return tactics_GOALKEEPER_STANDARD
	case RoleFamily_FullBack:
		return tactics_FULL_BACK_STANDARD
	case RoleFamily_CentreBack:
		return tactics_CENTRE_BACK_STANDARD
	case RoleFamily_Holding:
		return tactics_HOLDING_STANDARD
	case RoleFamily_Central:
		return tactics_CENTRAL_STANDARD
	case RoleFamily_Attacking:
		return tactics_ATTACKING_STANDARD
	case RoleFamily_Wide:
		return tactics_WIDE_STANDARD
	}
	return tactics_STRIKER_STANDARD
}

// Tactics_roleFit is the suitability of player for a role on the 0-100
// attribute scale (the weighted mean of its key attributes).
func Tactics_roleFit(player *Player, family RoleFamily, role TacticalRole) float64 {
	// Weighted by importance: the first attribute counts most.
	attributes := Tactics_keyAttributes(family, role)
	total := 0.0
	weights := 0.0
	weight := 1.0
	for _, name := range attributes {
		total += tacticsStat(player, name) * weight
		weights += weight
		weight *= 0.7
	}
	if weights > 0.0 {
		return total / weights
	}
	return tactics_DEFAULT_STAT
}

// Tactics_suggestedDuty is the duty that suits the player in a slot of
// family.
func Tactics_suggestedDuty(player *Player, family RoleFamily) RoleDuty {
	if family != RoleFamily_FullBack {
		return RoleDuty_Support
	}
	defending := tacticsStat(player, "Defending")
	engine := (tacticsStat(player, "Pace") + tacticsStat(player, "Stamina")) * 0.5
	flair := (tacticsStat(player, "Pace") + tacticsStat(player, "Dribbling")) * 0.5
	if engine >= defending+tactics_ATTACKING_FULL_BACK_MARGIN {
		return RoleDuty_Attack
	}
	if defending >= flair+tactics_DEFENSIVE_FULL_BACK_MARGIN {
		return RoleDuty_Defend
	}
	return RoleDuty_Support
}

// Tactics_suggestedRole is the role the player suits best in a slot of
// family: Standard unless a special role fits clearly better.
func Tactics_suggestedRole(player *Player, family RoleFamily) TacticalRole {
	best := TacticalRole_Standard
	bestFit := Tactics_roleFit(player, family, TacticalRole_Standard) + tactics_ROLE_PREFERENCE_MARGIN
	for _, role := range Tactics_rolesFor(family) {
		if role == TacticalRole_Standard {
			continue
		}
		fit := Tactics_roleFit(player, family, role)
		if fit > bestFit {
			bestFit = fit
			best = role
		}
	}
	return best
}

// Tactics_possessionOffset is the in-possession shift of a slot for a
// ready-made shape.
func Tactics_possessionOffset(shape PossessionShape, anchor Vector2F) Vector2F {
	family := Tactics_familyForSlot(anchor)
	switch shape {
	case PossessionShape_KeepShape, PossessionShape_COUNT:
	case PossessionShape_FullBacksPush:
		if family == RoleFamily_FullBack {
			return Vector2F{0.22, 0.0}
		}
		if family == RoleFamily_Wide {
			return Vector2F{0.05, tacticsInward(anchor, 0.08)}
		}
	case PossessionShape_BuildWithThree:
		// The left full-back becomes a third centre-back, the right one a
		// wing-back; the holding player screens and the wide men push on.
		if family == RoleFamily_FullBack {
			if anchor.Y < 0.5 {
				return Vector2F{0.02, tacticsInward(anchor, 0.14)}
			}
			return Vector2F{0.25, 0.0}
		}
		if family == RoleFamily_Wide {
			return Vector2F{0.10, 0.0}
		}
		if family == RoleFamily_Holding {
			return Vector2F{-0.04, 0.0}
		}
		if family == RoleFamily_Central {
			return Vector2F{0.03, 0.0}
		}
	case PossessionShape_NarrowFront:
		if family == RoleFamily_FullBack {
			return Vector2F{0.20, 0.0}
		}
		if family == RoleFamily_Wide {
			return Vector2F{0.04, tacticsInward(anchor, 0.18)}
		}
		if family == RoleFamily_Attacking {
			return Vector2F{0.03, 0.0}
		}
	}
	return Vector2F{0.0, 0.0}
}

// Tactics_assignRolesToFit gives every starter of lineup the role and duty
// that fit him (AI clubs); in-possession offsets are cleared.
func Tactics_assignRolesToFit(strategy *Strategy, lineup *Lineup) {
	slots := make([]SlotInstruction, 0, len(lineup.GetOutfieldPlayers()))
	for _, positioned := range lineup.GetOutfieldPlayers() {
		if positioned.Player == nil {
			continue
		}
		family := Tactics_familyForSlot(positioned.Position)
		slots = append(slots, SlotInstruction{
			Anchor:           positioned.Position,
			Role:             Tactics_suggestedRole(positioned.Player, family),
			Duty:             Tactics_suggestedDuty(positioned.Player, family),
			PossessionOffset: Vector2F{0.0, 0.0},
		})
	}
	strategy.SetSlotInstructions(slots)
	if lineup.GetGoalkeeper() != nil {
		strategy.SetKeeperRole(Tactics_suggestedRole(lineup.GetGoalkeeper(), RoleFamily_Goalkeeper))
	} else {
		strategy.SetKeeperRole(TacticalRole_Standard)
	}
}
