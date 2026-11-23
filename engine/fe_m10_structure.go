// M10 attack structure (ours, not from Player12 or FootballEngine).
//
// Measured against the SkillCorner 2020 EPL/LaLiga tracking (docs/spec-engine-go.md
// §0.15): with the ball, real strikers sit on the last defender (AM→ST 6-10 m,
// ours was 2 m), real wingers hug the touchline 35-61 % of the time (ours 3-13 %)
// and the attackers therefore offer a forward pass. Without it 49-70 % of the
// carrier's decisions in the attacking half had no forward option at all, so
// the ball went backwards (38 % of passes vs 24 % real).
//
// Applied last to a settled in-possession holder (HOLD_SHAPE, no runner / duty /
// support job), after the M4 refinement:
//   - the striker pins the last line (1.5 m onside), central channel;
//   - a winger keeps the width: touchline lane, or the half-space when his own
//     full-back is overlapping on that side; the weak-side winger may climb to
//     the last line (back post / switch target);
//   - a full-back joins in the opponent half (weak side stays narrower, as rest
//     defence).
package engine

import "math"

// The layer is switched by EngineConfig.Modules.M10Structure (FMENGINE_OFF=M10).

const (
	feM10StrikerOnside   = 1.5  // m behind the offside line
	feM10StrikerChannel  = 12.0 // |y| limit for the holding striker (m)
	feM10WingerTouchline = 3.5  // m in from the touchline
	feM10WingerHalfSpace = 19.0 // |y| when the full-back overlaps outside him
	feM10WingerLead      = 6.0  // ball-side winger: at most this ahead of the ball
	feM10WingerOnside    = 2.0  // weak-side winger: behind the line
	feM10FullBackStart   = -5.0 // team-frame ball x (m) where the full-backs join
	feM10FullBackTrail   = 12.0 // ball-side full-back at most this behind the ball
	feM10FullBackWeak    = 22.0 // weak-side full-back at most this behind the ball
	feM10FullBackTuck    = 22.0 // weak-side full-back |y|
)

func (e *MatchEngine) feM10Structure(player *MatchPlayer, target Vector2F, carrier *MatchPlayer, offsideLine float64) Vector2F {
	if !e.cfg.Modules.M10Structure || carrier == nil || player.Player == nil || player.IsGoalkeeper ||
		player.Intent != PlayerIntent_HOLD_SHAPE {
		return target
	}
	home := player.IsHomeTeam
	dir := feM1Dir(home)
	t := feM1ToM(target)
	tx, ty := t.X*dir, t.Y
	ball := feM1ToM(carrier.Position)
	bx := ball.X * dir
	offX := feM1ToM(Vector2F{offsideLine, 0.5}).X * dir
	legal := math.Max(0.0, math.Max(offX, bx))
	base := feM1ToM(player.BasePosition)
	side := math.Copysign(1.0, base.Y)
	ballSide := ball.Y*side > 0
	switch player.Player.GetRole() {
	case PlayerRole_ST:
		tx = math.Max(tx, legal-feM10StrikerOnside)
		ty = hClamp(ty, -feM10StrikerChannel, feM10StrikerChannel)
	case PlayerRole_LW, PlayerRole_RW, PlayerRole_LM, PlayerRole_RM:
		lane := feM1HalfWidth - feM10WingerTouchline
		if ballSide && e.feM10FullBackOverlaps(home, side) {
			lane = feM10WingerHalfSpace
		}
		ty = side * lane
		if ballSide {
			tx = math.Min(math.Max(tx, bx), bx+feM10WingerLead)
		} else {
			tx = math.Max(tx, math.Min(legal-feM10WingerOnside, bx+feM10WingerLead*2))
		}
		tx = math.Min(tx, legal-feM10WingerOnside*0.5)
	case PlayerRole_LB, PlayerRole_RB:
		if bx < feM10FullBackStart {
			return target
		}
		if ballSide {
			tx = math.Max(tx, bx-feM10FullBackTrail)
		} else {
			tx = math.Max(tx, bx-feM10FullBackWeak)
			ty = side * math.Min(math.Abs(ty), feM10FullBackTuck)
		}
	default:
		return target
	}
	tx = math.Min(tx, feM1HalfLength-feM4TargetInset)
	p := feM1ToPitch(feM1Vec{tx * dir, ty})
	return Vector2F{hClamp(p.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
		hClamp(p.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
}

// feM10FullBackOverlaps: the side's full-back on that flank is pushed on or
// overlapping this refresh.
func (e *MatchEngine) feM10FullBackOverlaps(home bool, side float64) bool {
	for i := range e.players {
		p := &e.players[i]
		if p.IsHomeTeam != home || !active(p) || p.Player == nil {
			continue
		}
		r := p.Player.GetRole()
		if r != PlayerRole_LB && r != PlayerRole_RB {
			continue
		}
		if feM1ToM(p.BasePosition).Y*side <= 0 {
			continue
		}
		return p.Intent == PlayerIntent_OVERLAP || e.feM4Pushed(p)
	}
	return false
}

// Near support slot (pitch units, depth > 0 = behind the ball): FMENGINE_M10
// nd/nw override for sweeps.

func (e *MatchEngine) feM10NearSupport() (depth, width float64) {
	return e.cfg.M10.NearDepth, e.cfg.M10.NearWidth
}

// Getting open (ours, folded into the M4 refinement in Phase 2): an attacker
// ahead of the ball whose lane from the
// carrier is shut, or who stands within feM10OpenSpace of a defender, moves to
// the spot within a few metres that is open (clear lane) and furthest from the
// nearest defender — the separation a real receiver makes before he asks for
// the ball.
const (
	feM10OpenAhead  = 3.0 // only targets this far ahead of the ball (m)
	feM10OpenRadius = 2.0 // lane radius (FE AttackGeometry.lane)
	feM10OpenSpace  = 4.5 // wanted metres from the nearest defender
	feM10OpenStay   = 0.6 // m of extra space a move must win (anti-jitter)
)

var feM10OpenSteps = [...][2]float64{{0, 3}, {0, -3}, {-3, 0}, {3, 0}, {0, 6}, {0, -6}, {-3, 4}, {-3, -4}, {3, 4}, {3, -4}, {-6, 0}}

// feM10SeeksOpen: the M4 refinement should look for an open spot for this
// player (ahead of the ball, lane shut or a defender within OpenSpace).
func (e *MatchEngine) feM10SeeksOpen(player, carrier *MatchPlayer, carrierM, t feM1Vec) bool {
	if !e.cfg.Modules.M10Structure || carrier == nil || carrier == player || player.Player == nil ||
		player.IsGoalkeeper || player.IsMakingRun || player.Player.GetRole() == PlayerRole_CB {
		return false
	}
	home := player.IsHomeTeam
	if (t.X-carrierM.X)*feM1Dir(home) < feM10OpenAhead {
		return false
	}
	return !e.spatialLaneOpen(home, carrierM, t, feM10OpenRadius) || e.feM2NearestOpponent(home, t) < feM10OpenSpace
}

// Marking distance (ours, M10). Measured on the SkillCorner tracking, the
// attackers ahead of the ball stand a median 3.5 m from the nearest defender;
// with Player12's 1.5 m goal-side tight marking (weight 0.7) and M1's 1.5 m
// box markers ours stood 1.6-1.9 m, so every forward option was covered and
// the ball went back to the free men behind.

func (e *MatchEngine) feM10BoxGoalSide() float64 {
	if !e.cfg.Modules.M10Structure {
		return feM1BoxMarkGoalSide
	}
	return e.cfg.M10.BoxGoalSide
}

func (e *MatchEngine) feM10GoalSideMark() float64 {
	if !e.cfg.Modules.M10Structure {
		return Shape_GOAL_SIDE_MARK_METRES
	}
	return e.cfg.M10.GoalSide
}

func (e *MatchEngine) feM10TightWeight() float64 {
	if !e.cfg.Modules.M10Structure {
		return Shape_TIGHT_MARK_WEIGHT
	}
	return e.cfg.M10.TightWeight
}

// Retreat gap per role (ours, M10). M1's RecoverShape keeps every unassigned
// outfielder at least the retreat gap (up to 8 m) goal-side of the ball. Real
// forwards stay as the first line (cover shadow on the centre-backs and the
// pivot); with them 8 m behind the ball the opponent's back line and pivot
// stood 10-13 m free (real 6-7 m) and recycling was always the safest pass.

func (e *MatchEngine) feM10RetreatGap(p *Player, gap float64) float64 {
	if !e.cfg.Modules.M10Structure || p == nil {
		return gap
	}
	switch p.GetRole() {
	case PlayerRole_ST, PlayerRole_LW, PlayerRole_RW, PlayerRole_CAM:
		return math.Min(gap, e.cfg.M10.RetreatFwd)
	case PlayerRole_LM, PlayerRole_RM, PlayerRole_CM, PlayerRole_CDM:
		return math.Min(gap, e.cfg.M10.RetreatMid)
	}
	return gap
}

// Defensive block height/length (ours, M10; pitch units): measured real
// out-of-possession lines sit ~10 m higher than Player12's block.

func (e *MatchEngine) feM10BlockLift() float64 {
	if !e.cfg.Modules.M10Structure {
		return 0
	}
	return e.cfg.M10.BlockLift
}

func (e *MatchEngine) feM10BlockLength() float64 {
	if !e.cfg.Modules.M10Structure {
		return 0
	}
	return e.cfg.M10.BlockLength
}

// The progress appetite moved to M2 (fe_m2_terms.go, m2.progress_*).
