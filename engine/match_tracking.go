// Ported from Player12 src/model/match_tracking.h (types only) (GPL-3.0)

package engine

// Only the types of the per-match tracker are ported. The recording logic
// (match_tracking.cpp) is not: MatchTracker records nothing, and the
// MatchEngine track* methods (match_engine_tracking.cpp) are no-ops in
// skipped.go. The tracker is output-only, so this does not change a match.

// Grid and thresholds of the per-match tracking (namespace MatchTracking).
const (
	// Columns along the pitch (thirds are 4 columns each).
	MatchTracking_GRID_COLUMNS = 12
	// Rows across the pitch (flanks are 3 rows each).
	MatchTracking_GRID_ROWS = 9
	MatchTracking_CELLS     = MatchTracking_GRID_COLUMNS * MatchTracking_GRID_ROWS
	// Momentum buckets: five clock minutes each, up to 120 minutes.
	MatchTracking_BUCKET_MINUTES = 5
	MatchTracking_BUCKETS        = 24
	// Stats entries tracked (starters, substitutes and replacements).
	MatchTracking_MAX_PLAYERS = 40
	// A completed open-play pass that gains this much ground towards the
	// opponent goal, ending outside the passer's own defensive 40%, is
	// progressive.
	MatchTracking_PROGRESSIVE_METRES    float64 = 10.0
	MatchTracking_PROGRESSIVE_MIN_END_X float64 = 0.4
	// The closest pressing player within this distance of the carrier
	// applies a pressure (counted once per player and possession spell).
	MatchTracking_PRESSURE_RADIUS_METRES float64 = 4.0
)

// TrackedPlayer holds the tracked numbers of one stats entry.
type TrackedPlayer struct {
	Touches    [MatchTracking_CELLS]uint16
	TouchCount uint16
	// Sums of the touch positions (attacking frame).
	SumX              float64
	SumY              float64
	ProgressivePasses uint16
	Pressures         uint16
	// xG of the shots his passes created.
	ExpectedAssists float64
	// Possession spell of his last pressure (counts once per spell).
	PressureSpell float64
}

// NewTrackedPlayer returns a TrackedPlayer with the C++ defaults.
func NewTrackedPlayer() TrackedPlayer { return TrackedPlayer{PressureSpell: -1.0} }

// TrackedShot is how a tracked shot came about.
type TrackedShot struct {
	SetPiece bool
	Header   bool
	Penalty  bool
}

// MatchTracker is the per-match recorder (types only in this port).
type MatchTracker struct {
	enabled      bool
	players      []TrackedPlayer
	passMatrix   []uint16
	shotKinds    []TrackedShot
	finalThird   [2][MatchTracking_BUCKETS]uint16
	passOrigin   Vector2F
	passOpenPlay bool
}

// NewMatchTracker returns a MatchTracker with the C++ defaults.
func NewMatchTracker() MatchTracker {
	return MatchTracker{enabled: true, passOrigin: Vector2F{0.5, 0.5}}
}

// SetEnabled switches recording on or off.
func (t *MatchTracker) SetEnabled(on bool) { t.enabled = on }

// IsEnabled reports whether recording is on.
func (t *MatchTracker) IsEnabled() bool { return t.enabled }

// PlayerCount is the number of tracked stats entries.
func (t *MatchTracker) PlayerCount() int { return len(t.players) }

// Player returns nil for players without any tracked action.
func (t *MatchTracker) Player(index int) *TrackedPlayer {
	if index < 0 || index >= len(t.players) {
		return nil
	}
	return &t.players[index]
}

// Shots returns the tracked shots.
func (t *MatchTracker) Shots() []TrackedShot { return t.shotKinds }

// FinalThirdTouches returns final-third touches per bucket, index 0 home.
func (t *MatchTracker) FinalThirdTouches() *[2][MatchTracking_BUCKETS]uint16 {
	return &t.finalThird
}
