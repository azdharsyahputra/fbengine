// Ported from FootballEngine FootballEngine.Core/Match/Systems/Players/Intent.fs (GPL-3.0)
// carryTarget (Intent.fs:52-111) with Positioning config TargetInset 2.0,
// CarrySearchRadius 7.0, SupportForwardDistance 4.0, CarryCutInsideDepth 3.0
// (Contracts/BalanceConfig.fs:1270-1276).
package engine

import "math"

const (
	feM3TargetInset         = 2.0
	feM3CarrySearchRadius   = 7.0
	feM3SupportForward      = 4.0
	feM3CarryCutInsideDepth = 3.0
)

// feM3UsesCarryTarget: per FE_PORT_PLAN M3, FE's two-candidate space test is
// the centre-back carry target only; wide forwards keep Player12's.
func feM3UsesCarryTarget(player *MatchPlayer) bool {
	return player.Player != nil && player.Player.GetRole() == PlayerRole_CB
}

// feM3CarryTarget: step forward 4 m (or back 2 m with no room ahead) and 7 m
// to the side, preferring the centre, and pick the candidate whose nearest
// opponent is farthest.
func (e *MatchEngine) feM3CarryTarget(player *MatchPlayer, s *feM3Slot) Vector2F {
	dir := -1.0
	if player.IsHomeTeam {
		dir = 1.0
	}
	pos := toMetres(Vector2F{player.Position.X - 0.5, player.Position.Y - 0.5})
	px, py := pos.X, pos.Y
	halfLength := Pitch_LENGTH_METRES * 0.5
	halfWidth := Pitch_WIDTH_METRES * 0.5
	roomAhead := (halfLength*dir - px) * dir
	forwardStep := feM3SupportForward
	if roomAhead < feM3SupportForward {
		forwardStep = -feM3SupportForward * 0.5
	}
	preferred := 1.0
	if py >= 0.0 {
		preferred = -1.0
	}
	safeHalfWidth := halfWidth - feM3TargetInset
	wasMovingInward := s.carrying && math.Abs(s.carryY) < math.Abs(py)
	forceInward := roomAhead < feM3CarryCutInsideDepth ||
		math.Abs(py) >= safeHalfWidth-feM3CarrySearchRadius ||
		(wasMovingInward && math.Abs(py) > safeHalfWidth-feM3CarrySearchRadius*2.0)

	bestX := px*dir + forwardStep
	bestY := py + preferred*feM3CarrySearchRadius
	bestSpace := -math.MaxFloat64
	for candidate := 0; candidate < 2; candidate++ {
		sign := preferred
		if !forceInward && candidate == 1 {
			sign = -preferred
		}
		tx := hClamp(px*dir+forwardStep, -(halfLength - feM3TargetInset), halfLength-feM3TargetInset)
		ty := hClamp(py+sign*feM3CarrySearchRadius, -safeHalfWidth, safeHalfWidth)
		nearest := math.MaxFloat64
		for i := range e.players {
			opponent := &e.players[i]
			if !active(opponent) || opponent.IsHomeTeam == player.IsHomeTeam {
				continue
			}
			o := toMetres(Vector2F{opponent.Position.X - 0.5, opponent.Position.Y - 0.5})
			nearest = math.Min(nearest, math.Hypot(tx-o.X*dir, ty-o.Y))
		}
		if nearest > bestSpace || (nearest == bestSpace && candidate == 0) {
			bestSpace = nearest
			bestX, bestY = tx, ty
		}
	}
	s.carrying = true
	s.carryY = bestY
	return Vector2F{
		hClamp(bestX*dir/Pitch_LENGTH_METRES+0.5, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
		hClamp(bestY/Pitch_WIDTH_METRES+0.5, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y),
	}
}
