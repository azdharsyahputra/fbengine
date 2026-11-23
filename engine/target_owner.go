// TargetOwner (ours, observability only): which layer last set the target a
// player steers to. In live play that is MatchPlayer.TacticalTarget; during
// a stoppage the restart walk steers to MovementTarget, owned by the restart
// layer. Recording it never changes a coordinate, intent, effort or the
// module order; it exists to make the layer conflicts listed in
// fm-game/docs/audit-engine-go.md §3 measurable.
package engine

// TargetOwner identifies the layer that last wrote a player's target.
type TargetOwner uint8

const (
	TargetOwner_NONE  TargetOwner = iota
	TargetOwner_P12               // Player12 shape, branches, keeper, shoot-out
	TargetOwner_M1                // FE M1 defensive task (feM1Apply)
	TargetOwner_M4                // FE M4 full-back push / compaction / refine / byline guard
	TargetOwner_M5                // FE M5 collective-attack duty
	TargetOwner_M9                // FE M9 restart walk (legal restart targets, half-time walk)
	TargetOwner_M10               // our M10 structure / getting open
	TargetOwner_LOOSE             // Phase 2 loose-ball resolver (loose_ball.go)
	TargetOwner_COUNT
)

var targetOwnerNames = [TargetOwner_COUNT]string{"none", "P12", "M1", "M4", "M5", "M9", "M10", "LOOSE"}

func (o TargetOwner) String() string {
	if o < TargetOwner_COUNT {
		return targetOwnerNames[o]
	}
	return "?"
}

// TargetReason (ours, observability only): why the owner chose the target —
// the branch or task that produced it. Recorded with TargetOwner.
type TargetReason uint8

const (
	TargetReason_NONE TargetReason = iota
	// In possession (P12 branches, M5 duties, M4/M10 refinements).
	TargetReason_SHAPE
	TargetReason_CARRY
	TargetReason_BYLINE
	TargetReason_RUNNER
	TargetReason_RUNNER_BOX
	TargetReason_ARRIVAL
	TargetReason_FAR_POST
	TargetReason_OVERLAP
	TargetReason_DUTY_LINK
	TargetReason_DUTY_RUNNER
	TargetReason_DUTY_SECURITY
	TargetReason_SUPPORT_NEAR
	TargetReason_SUPPORT_SQUARE
	TargetReason_SUPPORT_TRAILING
	TargetReason_SHORT_FORWARD
	TargetReason_SPACING
	TargetReason_STRUCTURE
	TargetReason_GET_OPEN
	// Out of possession on the Player12 path (M1 not owning).
	TargetReason_PRESS
	TargetReason_DOUBLE
	TargetReason_COVER_LANE
	TargetReason_COVER
	TargetReason_TIGHT_MARK
	TargetReason_DEF_TRANSITION
	TargetReason_ZONAL_MARK
	// Ball in flight or loose.
	TargetReason_RECEIVE
	TargetReason_FLIGHT_SUPPORT
	TargetReason_LOOSE_CLAIM
	TargetReason_LOOSE_COVER
	TargetReason_LOOSE_DELAY
	TargetReason_FLIGHT_MARK
	TargetReason_FLIGHT_RECOVER
	TargetReason_FLIGHT_HOLD
	TargetReason_TRANSITION_RUN
	TargetReason_TRANSITION_SUPPORT
	TargetReason_RECOVER
	TargetReason_AERIAL_HOLD
	// Keepers, M1 tasks, stoppages.
	TargetReason_GK
	TargetReason_M1_SHAPE
	TargetReason_M1_FIRST_DEFENDER
	TargetReason_M1_COVER
	TargetReason_M1_SCREEN
	TargetReason_M1_CONTAIN
	TargetReason_M1_PROTECT_DEPTH
	TargetReason_M1_RECOVER_SHAPE
	TargetReason_M1_MARKER
	TargetReason_RESTART
	TargetReason_HALF_TIME_WALK
	TargetReason_SHOOTOUT
	TargetReason_COUNT
)

var targetReasonNames = [TargetReason_COUNT]string{"none",
	"shape", "carry", "byline", "runner", "runner-box", "arrival", "far-post", "overlap",
	"duty-link", "duty-runner", "duty-security", "support-near", "support-square", "support-trailing",
	"short-forward", "spacing", "structure", "get-open",
	"press", "double", "cover-lane", "cover", "tight-mark", "def-transition", "zonal-mark",
	"receive", "flight-support", "loose-claim", "loose-cover", "loose-delay", "flight-mark",
	"flight-recover", "flight-hold", "transition-run", "transition-support", "recover", "aerial-hold",
	"gk", "m1-shape", "m1-first-defender", "m1-cover", "m1-screen", "m1-contain", "m1-protect-depth",
	"m1-recover-shape", "m1-marker", "restart", "half-time-walk", "shootout"}

func (r TargetReason) String() string {
	if r < TargetReason_COUNT {
		return targetReasonNames[r]
	}
	return "?"
}

// feM1TargetReason maps an M1 task role to its reason.
func feM1TargetReason(role feM1Role) TargetReason {
	switch role {
	case feM1RoleFirstDefender:
		return TargetReason_M1_FIRST_DEFENDER
	case feM1RoleCover:
		return TargetReason_M1_COVER
	case feM1RoleScreen:
		return TargetReason_M1_SCREEN
	case feM1RoleContain:
		return TargetReason_M1_CONTAIN
	case feM1RoleProtectDepth:
		return TargetReason_M1_PROTECT_DEPTH
	case feM1RoleRecoverShape:
		return TargetReason_M1_RECOVER_SHAPE
	case feM1RoleMarker:
		return TargetReason_M1_MARKER
	}
	return TargetReason_M1_SHAPE
}
