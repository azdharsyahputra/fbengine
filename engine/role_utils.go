// Ported from Player12 src/model/role_utils.h and src/model/role_utils.cpp (GPL-3.0)

package engine

// The localised shortName/longName (they need the language manager) are not
// ported; RoleUtils_shortNameArg returns the language-key argument.

var roleShortKeys = [PlayerRole_UNKNOWN]string{
	"ROLE_SHORT_GK", "ROLE_SHORT_CB", "ROLE_SHORT_LB", "ROLE_SHORT_RB",
	"ROLE_SHORT_CDM", "ROLE_SHORT_CM", "ROLE_SHORT_CAM", "ROLE_SHORT_LM",
	"ROLE_SHORT_RM", "ROLE_SHORT_LW", "ROLE_SHORT_RW", "ROLE_SHORT_ST"}

const roleNoRole = "-"

// RoleUtils_shortNameArg is the inbox argument for the short name
// ("@ROLE_SHORT_GK").
func RoleUtils_shortNameArg(role PlayerRole) string {
	if role < PlayerRole_UNKNOWN {
		return "@" + roleShortKeys[role]
	}
	return roleNoRole
}

// RoleUtils_toString converts a PlayerRole enum to its string representation.
func RoleUtils_toString(role PlayerRole) string {
	switch role {
	case PlayerRole_GK:
		return "GK"
	case PlayerRole_CB:
		return "CB"
	case PlayerRole_LB:
		return "LB"
	case PlayerRole_RB:
		return "RB"
	case PlayerRole_CDM:
		return "CDM"
	case PlayerRole_CM:
		return "CM"
	case PlayerRole_CAM:
		return "CAM"
	case PlayerRole_LM:
		return "LM"
	case PlayerRole_RM:
		return "RM"
	case PlayerRole_LW:
		return "LW"
	case PlayerRole_RW:
		return "RW"
	case PlayerRole_ST:
		return "ST"
	default:
		return "UNKNOWN"
	}
}

// RoleUtils_fromString converts a string to a PlayerRole enum.
func RoleUtils_fromString(roleStr string) PlayerRole {
	switch roleStr {
	case "GK":
		return PlayerRole_GK
	case "CB":
		return PlayerRole_CB
	case "LB":
		return PlayerRole_LB
	case "RB":
		return PlayerRole_RB
	case "CDM":
		return PlayerRole_CDM
	case "CM":
		return PlayerRole_CM
	case "CAM":
		return PlayerRole_CAM
	case "LM":
		return PlayerRole_LM
	case "RM":
		return PlayerRole_RM
	case "LW":
		return PlayerRole_LW
	case "RW":
		return PlayerRole_RW
	case "ST":
		return PlayerRole_ST
	// Handle legacy strings for backwards compatibility if needed
	case "Goalkeeper":
		return PlayerRole_GK
	case "Defender":
		return PlayerRole_CB
	case "Midfielder":
		return PlayerRole_CM
	case "Striker":
		return PlayerRole_ST
	}
	return PlayerRole_UNKNOWN
}

// RoleUtils_getBaseCoordinate gets the base pitch coordinate for a given
// role. Coordinates are normalized: X [0.0 (home goal) to 1.0 (away goal)],
// Y [0.0 (top) to 1.0 (bottom)].
func RoleUtils_getBaseCoordinate(role PlayerRole) Vector2F {
	// Assume Home team plays from Left (0.0) to Right (1.0).
	// Y goes from Top (0.0) to Bottom (1.0).
	switch role {
	case PlayerRole_GK:
		return Vector2F{0.05, 0.50}
	case PlayerRole_CB:
		return Vector2F{0.15, 0.50}
	case PlayerRole_LB:
		return Vector2F{0.15, 0.15}
	case PlayerRole_RB:
		return Vector2F{0.15, 0.85}
	case PlayerRole_CDM:
		return Vector2F{0.30, 0.50}
	case PlayerRole_CM:
		return Vector2F{0.45, 0.50}
	case PlayerRole_CAM:
		return Vector2F{0.60, 0.50}
	case PlayerRole_LM:
		return Vector2F{0.45, 0.15}
	case PlayerRole_RM:
		return Vector2F{0.45, 0.85}
	case PlayerRole_LW:
		return Vector2F{0.75, 0.15}
	case PlayerRole_RW:
		return Vector2F{0.75, 0.85}
	case PlayerRole_ST:
		return Vector2F{0.85, 0.50}
	default:
		return Vector2F{0.50, 0.50}
	}
}

// RoleUtils_getBroadCategory gets the broad category of the role (e.g.
// "Striker", "Defender").
func RoleUtils_getBroadCategory(role PlayerRole) string {
	switch role {
	case PlayerRole_GK:
		return "Goalkeeper"
	case PlayerRole_CB, PlayerRole_LB, PlayerRole_RB:
		return "Defender"
	case PlayerRole_CDM, PlayerRole_CM, PlayerRole_CAM, PlayerRole_LM, PlayerRole_RM:
		return "Midfielder"
	case PlayerRole_LW, PlayerRole_RW, PlayerRole_ST:
		return "Striker"
	default:
		return "Unknown"
	}
}
