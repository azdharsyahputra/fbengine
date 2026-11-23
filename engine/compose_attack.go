// Attack target composition (ours, Phase 2 / A1): the single place where an
// in-possession off-ball player's target is finished. Before it, the target
// went through a chain where each layer overwrote the last (P12 branch → M5
// duty → M4 full-back push → M4 compaction → M4 refine → M10 structure → M10
// getting open), so a later layer silently undid an earlier one's decision
// (fm-game/docs/audit-engine-go.md §3.1).
//
// Ownership now:
//   - the job (runner, arrival, overlap, duty, supporter, short forward,
//     hold) is decided by the branch above (P12) or M5; an M5 duty target is
//     final;
//   - a holder's spot comes from exactly one structure rule for his role:
//     full-back the ball-side push is moving → M4 full-back push;
//     other full-backs, striker, wingers   → M10 structure;
//     No.10 and central midfielders        → M4 compaction;
//   - M4 refine then adjusts the spacing (getting open — lane and separation
//     — is part of its candidate set and scoring) and is recorded as the
//     refiner, never as the owner.
package engine

// composeAttack finishes the target of an in-possession off-ball player whose
// job the branch above has set. It returns the final target, the spot after
// the structure rule (for the audit), the owner, reason and refiner.
func (e *MatchEngine) composeAttack(player *MatchPlayer, target Vector2F, carrier *MatchPlayer, line float64,
	duty bool, owner TargetOwner, reason TargetReason) (final, structured Vector2F, o TargetOwner, r TargetReason, refiner TargetOwner) {
	o, r, refiner = owner, reason, TargetOwner_NONE
	if duty || player.Player == nil ||
		(player.Intent != PlayerIntent_HOLD_SHAPE && player.Intent != PlayerIntent_OFFER_SUPPORT) {
		return target, target, o, r, refiner
	}
	if player.Intent == PlayerIntent_HOLD_SHAPE {
		if t, so := e.structureRule(player, target, carrier, line); so != TargetOwner_NONE && t != target {
			target, o, r = t, so, TargetReason_STRUCTURE
		}
	}
	structured = target
	if e.cfg.Modules.M4Shape {
		if t := e.feM4Refine(player, target, carrier, line); t != target {
			target, refiner = t, TargetOwner_M4
		}
	}
	return target, structured, o, r, refiner
}

// structureRule applies the one structure rule that owns this holder's role.
func (e *MatchEngine) structureRule(player *MatchPlayer, target Vector2F, carrier *MatchPlayer, line float64) (Vector2F, TargetOwner) {
	m4, m10 := e.cfg.Modules.M4Shape, e.cfg.Modules.M10Structure
	switch player.Player.GetRole() {
	case PlayerRole_LB, PlayerRole_RB:
		if m4 {
			if t := e.feM4FullBackPush(player, target, carrier); e.feM4Pushed(player) {
				return t, TargetOwner_M4
			}
		}
		if m10 {
			return e.feM10Structure(player, target, carrier, line), TargetOwner_M10
		}
	case PlayerRole_ST, PlayerRole_LW, PlayerRole_RW, PlayerRole_LM, PlayerRole_RM:
		if m10 {
			return e.feM10Structure(player, target, carrier, line), TargetOwner_M10
		}
		if m4 {
			return e.feM4Compact(player, target, carrier), TargetOwner_M4
		}
	case PlayerRole_CAM, PlayerRole_CDM, PlayerRole_CM:
		if m4 {
			return e.feM4Compact(player, target, carrier), TargetOwner_M4
		}
	}
	return target, TargetOwner_NONE
}
