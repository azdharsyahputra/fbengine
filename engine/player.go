// Ported from Player12 src/model/player.h, src/model/player.cpp and src/model/injury.h (InjuryType only) (GPL-3.0)

package engine

import "math"

// Only the parts of the career Player that the match engine reads are
// ported: identity, role, height, footedness, the attribute map, the overall
// rating, hidden traits and the day-to-day dynamics (condition, injury).
// Market value, contracts, ageing and training are not ported.

// Foot represents a player's preferred foot.
type Foot bool

const (
	Foot_Left  Foot = false
	Foot_Right Foot = true
)

// InjuryType lists the diagnoses used by the injury model (injury.h; values
// are persisted).
type InjuryType uint8

const (
	InjuryType_None InjuryType = iota
	InjuryType_HamstringStrain
	InjuryType_HamstringTightness
	InjuryType_GroinStrain
	InjuryType_AnkleSprain
	InjuryType_QuadricepsStrain
	InjuryType_CalfStrain
	InjuryType_KneeMcl
	InjuryType_ThighContusion
	InjuryType_AchillesTendinopathy
	InjuryType_Concussion
	InjuryType_KneeCartilage
	InjuryType_KneeMeniscus
	InjuryType_KneeAcl
	InjuryType_BackSpasm
	InjuryType_FootContusion
	InjuryType_BrokenMetatarsal
	InjuryType_ShoulderDislocation
	InjuryType_COUNT
)

// PlayerTraits holds hidden personality and physical traits on a 1-100
// scale.
type PlayerTraits struct {
	Professionalism uint8
	Ambition        uint8
	Temperament     uint8
	Loyalty         uint8
	InjuryProneness uint8
}

// NewPlayerTraits returns PlayerTraits with the C++ default member values.
func NewPlayerTraits() PlayerTraits {
	return PlayerTraits{50, 50, 50, 50, 50}
}

// PlayerDynamics_FORM_WINDOW is PlayerDynamics::FORM_WINDOW.
const PlayerDynamics_FORM_WINDOW = 5

// PlayerDynamics is the day-to-day state that changes between matches.
type PlayerDynamics struct {
	Condition             float64 // 0-100 physical freshness.
	Sharpness             float64 // 0-100 match fitness.
	Morale                float64 // 0-100.
	PlayingShare          float64 // Smoothed share of minutes played.
	Injury                InjuryType
	InjuryDays            uint16 // Days until fit again.
	LastInjury            InjuryType
	LastInjuryDay         int32
	LastMatchDay          int32
	SeasonAppearances     uint16
	SeasonMinutes         uint16
	WeekMinutes           uint16
	TransferInterestWeeks uint8 // Unsettled by a bid.
	RatingCount           uint8
	RecentRatings         [PlayerDynamics_FORM_WINDOW]float64 // Newest first.
}

// NewPlayerDynamics returns PlayerDynamics with the C++ default member values.
func NewPlayerDynamics() PlayerDynamics {
	return PlayerDynamics{Condition: 100.0, Sharpness: 60.0, Morale: 60.0}
}

// Player is the persistent career identity, attributes, traits and
// day-to-day state. The engine borrows it as read-only input.
//
// Fields are exported so callers can build squads directly; the engine code
// reads them through the C++-named getters.
type Player struct {
	Id          PlayerID
	TeamId      TeamID
	FirstName   string
	LastName    string
	Role        PlayerRole
	Age         uint8
	Height      uint8 // centimetres (uint8 as in C++; 0 = unknown)
	Foot        Foot
	SquadNumber uint8
	Potential   float64
	Traits      PlayerTraits
	Dynamics    PlayerDynamics
	// Stats is the attribute map ("Pace", "Shooting", "Passing",
	// "Dribbling", "Defending", "Goalkeeping", "Physicality", "Stamina",
	// "Vision"), values on the 1-100 scale.
	Stats map[string]float64
}

// NewPlayer mirrors the C++ constructor for the fields the engine uses.
func NewPlayer(id PlayerID, teamId TeamID, firstName, lastName string,
	role PlayerRole, age uint8, height uint8, foot Foot,
	stats map[string]float64) *Player {
	return &Player{
		Id:        id,
		TeamId:    teamId,
		FirstName: firstName,
		LastName:  lastName,
		Role:      role,
		Age:       age,
		Height:    height,
		Foot:      foot,
		Traits:    NewPlayerTraits(),
		Dynamics:  NewPlayerDynamics(),
		Stats:     stats,
	}
}

// GetId gets the player's ID.
func (p *Player) GetId() PlayerID { return p.Id }

// GetTeamId gets the ID of the player's team.
func (p *Player) GetTeamId() TeamID { return p.TeamId }

// GetName gets the player's full name.
func (p *Player) GetName() string { return p.FirstName + " " + p.LastName }

// GetFirstName gets the player's first name.
func (p *Player) GetFirstName() string { return p.FirstName }

// GetLastName gets the player's last name.
func (p *Player) GetLastName() string { return p.LastName }

// GetAge gets the player's age.
func (p *Player) GetAge() int { return int(p.Age) }

// GetRole gets the player's role.
func (p *Player) GetRole() PlayerRole { return p.Role }

// GetHeight gets the player's height in cm.
func (p *Player) GetHeight() uint8 { return p.Height }

// GetFoot gets the player's preferred foot.
func (p *Player) GetFoot() Foot { return p.Foot }

// GetStats gets the player's stats map.
func (p *Player) GetStats() map[string]float64 { return p.Stats }

// SetStats sets the player's stats map.
func (p *Player) SetStats(stats map[string]float64) { p.Stats = stats }

// GetPotential is the hidden potential on the overall-rating scale.
func (p *Player) GetPotential() float64 { return p.Potential }

// GetTraits returns the hidden personality traits.
func (p *Player) GetTraits() *PlayerTraits { return &p.Traits }

// GetDynamics returns condition, sharpness, morale, injury and form state.
func (p *Player) GetDynamics() *PlayerDynamics { return &p.Dynamics }

// MutableDynamics returns the mutable dynamic state.
func (p *Player) MutableDynamics() *PlayerDynamics { return &p.Dynamics }

// GetSquadNumber returns the squad number (0 when none).
func (p *Player) GetSquadNumber() uint8 { return p.SquadNumber }

// IsAvailable is true when the player is fit enough to be selected.
func (p *Player) IsAvailable() bool { return p.Dynamics.InjuryDays == 0 }

// GetOverall calculates the player's overall rating based on stats and
// configuration.
func (p *Player) GetOverall(statsConfig *StatsConfig) float64 {
	overall := 0.0
	if statsConfig == nil {
		return 0.0
	}
	roleConfig, ok := statsConfig.RoleFocus[RoleUtils_getBroadCategory(p.Role)]
	if !ok {
		return 0.0
	}
	weights := roleConfig.Weights
	statNames := roleConfig.Stats
	for i := 0; i < min(len(statNames), len(weights)); i++ {
		if value, found := p.Stats[statNames[i]]; found {
			overall += value * weights[i]
		}
	}
	return overall
}

// GetForm is the average of the recent match ratings, 0 when none.
func (p *Player) GetForm() float64 {
	if p.Dynamics.RatingCount == 0 {
		return 0.0
	}
	total := 0.0
	for i := 0; i < int(p.Dynamics.RatingCount); i++ {
		total += p.Dynamics.RecentRatings[i]
	}
	return total / float64(p.Dynamics.RatingCount)
}

// PushMatchRating records a match rating (1-10) in the recent form window.
func (p *Player) PushMatchRating(rating float64) {
	ratings := &p.Dynamics.RecentRatings
	copy(ratings[1:], ratings[:len(ratings)-1])
	ratings[0] = math.Max(1.0, math.Min(10.0, rating))
	if int(p.Dynamics.RatingCount) < len(ratings) {
		p.Dynamics.RatingCount++
	}
}
