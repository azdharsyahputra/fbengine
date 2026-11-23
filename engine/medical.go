// Ported from Player12 src/model/medical_centre.h and src/model/medical_centre.cpp (only what MatchEngine uses) (GPL-3.0)

package engine

// MedicalFlag holds instructions of the medical staff for one player (bit
// flags, persisted).
type MedicalFlag = uint8

const (
	MEDICAL_FLAG_NONE MedicalFlag = 0
	// Left out of the assistant's selections while anyone else is fit.
	MEDICAL_FLAG_REST MedicalFlag = 1 << 0
	// Should come off around the hour mark.
	MEDICAL_FLAG_LIMIT_MINUTES MedicalFlag = 1 << 1
)

// MedicalCentre_MINUTE_LIMIT is the minute around which a player limited by
// the medical staff comes off.
const MedicalCentre_MINUTE_LIMIT = 60

// MedicalCentre_substitutionDue reports whether a player with flags should
// come off at minute.
func MedicalCentre_substitutionDue(flags uint8, minute int) bool {
	return (flags&MEDICAL_FLAG_LIMIT_MINUTES) != 0 && minute >= MedicalCentre_MINUTE_LIMIT
}
