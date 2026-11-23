// Ported from Player12 src/global/types.h, src/global/stats_config.h and parts of src/global/global.h (GPL-3.0)

package engine

// LeagueID is the type alias for League identifiers.
type LeagueID = uint8

// TeamID is the type alias for Team identifiers.
type TeamID = uint16

// PlayerID is the type alias for Player identifiers.
type PlayerID = uint32

// SeasonPhase represents the different phases of a season.
type SeasonPhase uint8

const (
	SeasonPhase_PRE_SEASON     SeasonPhase = iota // Phase before the regular season begins
	SeasonPhase_REGULAR_SEASON                    // The main competitive phase of the season
	SeasonPhase_POST_SEASON                       // Phase after the regular season (e.g., playoffs)
	SeasonPhase_OFF_SEASON                        // Period between seasons with no matches
)

// MatchType represents the type or category of a match.
type MatchType uint8

const (
	MatchType_LEAGUE      MatchType = 0 // A regular league match
	MatchType_FRIENDLY    MatchType = 1 // A non-competitive friendly match
	MatchType_CUP         MatchType = 2 // A cup tournament match
	MatchType_CONTINENTAL MatchType = 3 // A continental club competition match
)

// Matchup represents a single match between two teams.
type Matchup struct {
	HomeTeamId TeamID    // ID of the home team
	AwayTeamId TeamID    // ID of the away team
	Type       MatchType // Type of the match
}

// PlayerRole represents a specific tactical role on the pitch.
type PlayerRole uint8

const (
	PlayerRole_GK  PlayerRole = iota // Goalkeeper
	PlayerRole_CB                    // Center Back
	PlayerRole_LB                    // Left Back
	PlayerRole_RB                    // Right Back
	PlayerRole_CDM                   // Central Defensive Midfielder
	PlayerRole_CM                    // Central Midfielder
	PlayerRole_CAM                   // Central Attacking Midfielder
	PlayerRole_LM                    // Left Midfielder
	PlayerRole_RM                    // Right Midfielder
	PlayerRole_LW                    // Left Winger
	PlayerRole_RW                    // Right Winger
	PlayerRole_ST                    // Striker
	PlayerRole_UNKNOWN
)

// Vector2F is a simple 2D vector for pitch coordinates. The C++ uses float;
// the Go port uses float64 throughout (see PORTING.md).
type Vector2F struct {
	X float64
	Y float64
}

// RoleFocus represents the statistics and their weights for a specific
// player role (stats_config.h).
type RoleFocus struct {
	Stats   []string  // List of statistic names relevant to the role.
	Weights []float64 // Corresponding weights for each statistic.
}

// StatsConfig maps roles to their focus areas and lists all possible stats
// (stats_config.h).
type StatsConfig struct {
	RoleFocus     map[string]RoleFocus // Mapping of role names to their RoleFocus.
	PossibleStats []string             // List of all possible statistics.
}

// From src/global/global.h.
const (
	// Minimum possible value for a player statistic.
	MIN_STAT_VAL uint8 = 1
	// Maximum possible value for a player statistic.
	MAX_STAT_VAL uint8 = 100
	// Base rate at which player statistics increase.
	PLAYER_STAT_INCREASE_BASE float64 = 0.03
	LINEUP_GRID_ROWS          uint8   = 5
	LINEUP_GRID_COLS          uint8   = 5
	LINEUP_GRID_SIZE          uint8   = LINEUP_GRID_ROWS * LINEUP_GRID_COLS
)

// ToRowCol converts a grid index to (row, col) (global.h toRowCol).
func ToRowCol(i int) (int, int) {
	return i / int(LINEUP_GRID_COLS), i % int(LINEUP_GRID_COLS)
}
