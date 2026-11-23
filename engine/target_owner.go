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
