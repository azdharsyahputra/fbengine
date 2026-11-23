// Ported from Player12 src/model/match_events.h (and matchEventTypeName from src/model/match_rules.cpp) (GPL-3.0)

package engine

// MatchEventType is the machine-readable category of a match event.
type MatchEventType uint8

const (
	MatchEventType_INFO MatchEventType = iota
	MatchEventType_KICK_OFF
	MatchEventType_GOAL
	MatchEventType_OWN_GOAL
	MatchEventType_SHOT
	MatchEventType_SAVE
	MatchEventType_SHOT_BLOCKED
	MatchEventType_SHOT_OFF_TARGET
	MatchEventType_WOODWORK
	MatchEventType_FOUL
	MatchEventType_ADVANTAGE
	MatchEventType_YELLOW_CARD
	MatchEventType_SECOND_YELLOW
	MatchEventType_RED_CARD
	MatchEventType_INJURY
	MatchEventType_SUBSTITUTION
	MatchEventType_OFFSIDE
	MatchEventType_CORNER
	MatchEventType_FREE_KICK
	MatchEventType_PENALTY
	MatchEventType_PENALTY_MISSED
	MatchEventType_THROW_IN
	MatchEventType_GOAL_KICK
	MatchEventType_ADDED_TIME
	MatchEventType_HALF_TIME
	MatchEventType_SECOND_HALF
	MatchEventType_FULL_TIME
	// A kick of the penalty shootout (detail: scored, saved or missed; none
	// when the shootout begins). Shootouts never count as goals or shots.
	MatchEventType_PENALTY_SHOOTOUT
)

// MatchEventTypeName is matchEventTypeName (match_rules.cpp).
func MatchEventTypeName(t MatchEventType) string {
	switch t {
	case MatchEventType_INFO:
		return "info"
	case MatchEventType_KICK_OFF:
		return "kick_off"
	case MatchEventType_GOAL:
		return "goal"
	case MatchEventType_OWN_GOAL:
		return "own_goal"
	case MatchEventType_SHOT:
		return "shot"
	case MatchEventType_SAVE:
		return "save"
	case MatchEventType_SHOT_BLOCKED:
		return "shot_blocked"
	case MatchEventType_SHOT_OFF_TARGET:
		return "shot_off_target"
	case MatchEventType_WOODWORK:
		return "woodwork"
	case MatchEventType_FOUL:
		return "foul"
	case MatchEventType_ADVANTAGE:
		return "advantage"
	case MatchEventType_YELLOW_CARD:
		return "yellow_card"
	case MatchEventType_SECOND_YELLOW:
		return "second_yellow"
	case MatchEventType_RED_CARD:
		return "red_card"
	case MatchEventType_INJURY:
		return "injury"
	case MatchEventType_SUBSTITUTION:
		return "substitution"
	case MatchEventType_OFFSIDE:
		return "offside"
	case MatchEventType_CORNER:
		return "corner"
	case MatchEventType_FREE_KICK:
		return "free_kick"
	case MatchEventType_PENALTY:
		return "penalty"
	case MatchEventType_PENALTY_MISSED:
		return "penalty_missed"
	case MatchEventType_THROW_IN:
		return "throw_in"
	case MatchEventType_GOAL_KICK:
		return "goal_kick"
	case MatchEventType_ADDED_TIME:
		return "added_time"
	case MatchEventType_HALF_TIME:
		return "half_time"
	case MatchEventType_SECOND_HALF:
		return "second_half"
	case MatchEventType_FULL_TIME:
		return "full_time"
	case MatchEventType_PENALTY_SHOOTOUT:
		return "penalty_shootout"
	}
	return "unknown"
}

// MatchEventDetail is the variant of an event, for the commentary
// (MatchEvent.Detail).
type MatchEventDetail uint8

const (
	MatchEventDetail_NONE MatchEventDetail = iota
	// SHOT: a headed attempt.
	MatchEventDetail_HEADER
	// SAVE: parried back into play / tipped behind for a corner.
	MatchEventDetail_PARRIED
	MatchEventDetail_TIPPED_BEHIND
	// SHOT_BLOCKED: a direct free kick stopped by the wall.
	MatchEventDetail_WALL
	// INJURY: hurt in a challenge rather than on his own.
	MatchEventDetail_CONTACT
	// INFO: the match was abandoned (a side fell below seven players).
	MatchEventDetail_ABANDONED
	// PENALTY_SHOOTOUT: the outcome of a kick.
	MatchEventDetail_SCORED
	MatchEventDetail_SAVED
	MatchEventDetail_MISSED
)

// MatchEvent is one entry of the match log. See match_events.h for the
// meaning of each field (timeMinute is the match clock; addedMinute the part
// beyond the regulation end of the period; for goals the secondary player is
// the assist provider, for fouls the fouled player, for substitutions the
// outgoing player and for saves the shooter).
type MatchEvent struct {
	TimeMinute        float64
	Description       string
	Type              MatchEventType
	Period            int
	AddedMinute       float64
	HasTeam           bool
	IsHomeTeam        bool
	PrimaryPlayerId   PlayerID
	SecondaryPlayerId PlayerID
	Position          Vector2F
	Xg                float64
	Detail            MatchEventDetail
	// Score once the event happened (after the goal for a goal).
	HomeScore int
	AwayScore int
	// ADDED_TIME: the announced added minutes.
	Minutes int
	// Penalty shootout score once the event happened (0-0 before one).
	HomeShootout int
	AwayShootout int
}

// NewMatchEvent returns a MatchEvent with the C++ default member values.
func NewMatchEvent() MatchEvent {
	return MatchEvent{
		Type:     MatchEventType_INFO,
		Period:   1,
		Position: Vector2F{Pitch_CENTRE, Pitch_CENTRE},
		Detail:   MatchEventDetail_NONE,
	}
}

// PlayerMatchStats are per-player match statistics. One entry exists for
// every player who took part, including substitutes and players who left the
// pitch.
type PlayerMatchStats struct {
	PlayerId         PlayerID
	IsHomeTeam       bool
	Role             PlayerRole
	Started          bool
	SubstitutedOn    bool
	SubstitutedOff   bool
	SentOff          bool
	Injured          bool
	MinutesPlayed    float64
	Goals            int
	OwnGoals         int
	Assists          int
	Shots            int
	ShotsOnTarget    int
	ExpectedGoals    float64
	PassesAttempted  int
	PassesCompleted  int
	KeyPasses        int
	TacklesAttempted int
	TacklesWon       int
	Interceptions    int
	Clearances       int
	AerialDuelsWon   int
	AerialDuelsLost  int
	Saves            int
	GoalsConceded    int
	FoulsCommitted   int
	FoulsSuffered    int
	YellowCards      int
	RedCards         int
	// Times he took control of the ball (receptions, wins, restarts).
	Touches int
	// Times he started closing down the ball carrier.
	Pressures int
	// Distance covered in metres (the simulation runs in real time).
	DistanceMetres           float64
	SecondHalfDistanceMetres float64
	// Distance above 5.5 m/s (19.8 km/h) and above 7 m/s (25.2 km/h).
	HighIntensityMetres           float64
	SecondHalfHighIntensityMetres float64
	SprintMetres                  float64
	// Efforts above 7 m/s.
	Sprints int
	// Highest speed reached (m/s).
	TopSpeed float64
	// Physical condition in [0, 1] when the player last left or at the end.
	Condition float64
	// Event-driven rating, 6.0 baseline, bounded to [3, 10].
	Rating float64
}

// NewPlayerMatchStats returns PlayerMatchStats with the C++ default member
// values.
func NewPlayerMatchStats() PlayerMatchStats {
	return PlayerMatchStats{Role: PlayerRole_UNKNOWN, Condition: 1.0, Rating: Rating_BASELINE}
}

// SubstitutionReason is why a substitution was made.
type SubstitutionReason uint8

const (
	SubstitutionReason_MANUAL SubstitutionReason = iota
	SubstitutionReason_FATIGUE
	SubstitutionReason_INJURY
	SubstitutionReason_TACTICAL
	SubstitutionReason_CARD_RISK
	SubstitutionReason_GOALKEEPER_REPLACEMENT
)

// MatchSubstitution records one substitution.
type MatchSubstitution struct {
	TimeMinute       float64
	Period           int
	IsHomeTeam       bool
	OutgoingPlayerId PlayerID
	IncomingPlayerId PlayerID
	Reason           SubstitutionReason
}

// NewMatchSubstitution returns a MatchSubstitution with the C++ defaults.
func NewMatchSubstitution() MatchSubstitution {
	return MatchSubstitution{Period: 1, Reason: SubstitutionReason_MANUAL}
}
