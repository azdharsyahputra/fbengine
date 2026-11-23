// Ported from FootballEngine FootballEngine.Core/Match/Systems/Team/DefensiveTransition.fs (GPL-3.0)
//
// M1 shared geometry (FE `TransitionGeometry`, lines 1-153): FE metre frame,
// the arrival-time race, earliest-arriver selection with hand-off hysteresis,
// the rest defenders, the press point and the cover point. Constants are FE
// `BalanceConfig.fs:882-912` (DefensiveTransition) unless noted. The arrival
// race uses each player's own P12 top speed and acceleration (plan §0.3 P6).
// FootballEngine: https://github.com/DeltaBitsSystem/FootballEngine (GPL-3.0).
package engine

import "math"

// FE DefensiveTransition config (BalanceConfig.fs:887-912), seconds and metres.
const (
	feM1BaseArrivalWindow     = 1.2
	feM1PressArrivalExtension = 0.8
	feM1LocalArrivalWindow    = 2.0
	feM1CoverArrivalWindow    = 1.8
	feM1HandoffAdvantage      = 0.25
	feM1MinimumPressIntensity = 0.3
	feM1MinimumPressCondition = 0.55
	feM1MinimumLocalPlayers   = 3
	feM1LocalSurplus          = 1
	feM1ApproachOffset        = 0.75
	feM1CarrierPrediction     = 0.15
	feM1CoverDepth            = 6.0
	feM1BeatenMargin          = 2.5
	feM1EscapeProgress        = 10.0
	feM1ThreatGoalDistance    = 28.0
	feM1EngageGoalDistance    = 35.0
	feM1CentralHalfWidth      = 18.0
	feM1DepthSafety           = 3.0
	feM1ScreenFraction        = 0.45
	feM1RetreatBallGap        = 8.0
	feM1RetreatWidthScale     = 0.6
	feM1SettledTolerance      = 3.0
	feM1SettledToleranceGo    = 5.0  // Go adaptation, see feM1AllSettled
	feM1RestHysteresis        = 3.0  // Go adaptation, see feM1RestDefenders
	feM1RetreatTimeout        = 12.0 // Go adaptation, see feM1UpdateEpisode (480 subticks)
	feM1MaximumScreens        = 2
	feM1CoverEffort           = 0.85
	feM1ScreenEffort          = 0.85
	// 24 subticks (Commitment, StabilizationHold) and 240 (MaxPressDuration).
	feM1Commitment        = 0.6
	feM1StabilizationHold = 0.6
	feM1MaxPressDuration  = 6.0
	// Positioning.TargetInset (BalanceConfig.fs:1270), Ball.FootReach for the race,
	// Actions.MaxPassExecutionDistance.
	feM1TargetInset          = 2.0
	feM1FootReach            = 1.05
	feM1MaxPassExecutionDist = 65.0
	// Movement.JogSpeed / SprintSpeed: the effort scale between jog and sprint.
	feM1JogShare = 5.0 / 8.5
	// Mean drive of P12's a(v) = A0(1 - v/vmax) over a ramp; FE uses
	// Acceleration 4.0 · Smoothing 0.8 = 3.2 m/s², about half of P12's A0.
	feM1AccelerationShare = 0.5
	feM1HalfLength        = 52.5
	feM1HalfWidth         = 34.0
)

// feM1Vec is a point in the FE frame: metres from the centre spot.
type feM1Vec struct{ X, Y float64 }

func feM1ToM(p Vector2F) feM1Vec {
	return feM1Vec{(p.X - Pitch_CENTRE) * Pitch_LENGTH_METRES, (p.Y - Pitch_CENTRE) * Pitch_WIDTH_METRES}
}

func feM1ToPitch(p feM1Vec) Vector2F {
	return Vector2F{p.X/Pitch_LENGTH_METRES + Pitch_CENTRE, p.Y/Pitch_WIDTH_METRES + Pitch_CENTRE}
}

func feM1Dir(home bool) float64 {
	if home {
		return 1.0
	}
	return -1.0
}

func feM1OwnGoalX(home bool) float64 { return -feM1HalfLength * feM1Dir(home) }

func feM1Dist(a, b feM1Vec) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// feM1Outfielder is FE `TransitionGeometry.outfielder`: active and not keeping goal.
func feM1Outfielder(p *MatchPlayer) bool { return active(p) && !p.IsGoalkeeper }

// feM1ChooseSlot is FE `chooseSlotFor` (DefensiveTransition.fs:101-129): the
// earliest arriver among the side's outfielders that are not excluded (ties to
// the lower slot); the previous holder keeps the job while he is within margin.
func (e *MatchEngine) feM1ChooseSlot(home bool, effort float64, target feM1Vec,
	excluded func(slot int) bool, previous int, margin float64) (int, float64) {
	best := -1
	bestTime := math.MaxFloat64
	for slot := range e.players {
		p := &e.players[slot]
		if p.IsHomeTeam != home || !feM1Outfielder(p) || excluded(slot) {
			continue
		}
		t := e.bodyTimeTo(p, target, effort)
		if t < bestTime {
			best, bestTime = slot, t
		}
	}
	if previous >= 0 && previous < len(e.players) {
		p := &e.players[previous]
		if p.IsHomeTeam == home && feM1Outfielder(p) && !excluded(previous) {
			held := e.bodyTimeTo(p, target, effort)
			if held <= bestTime+margin {
				return previous, held
			}
		}
	}
	return best, bestTime
}

// feM1RestDefenders is FE `restDefendersV` (:52-72): the two outfielders
// deepest in the team frame (smallest x·dir), ties to the lower slot.
// Go adaptation: Player12's back four is flat, so the deepest two would swap
// every refresh; the previous pair (and centre-backs) keep the job unless
// another player is feM1RestHysteresis metres deeper.
func (e *MatchEngine) feM1RestDefenders(home bool, side *feM1Side) (int, int) {
	dir := feM1Dir(home)
	first, second := -1, -1
	depth := func(slot int) float64 {
		d := feM1ToM(e.players[slot].Position).X * dir
		if side != nil && (slot == side.rest[0] || slot == side.rest[1]) {
			d -= feM1RestHysteresis
		}
		if p := e.players[slot].Player; p != nil && p.GetRole() == PlayerRole_CB {
			// The centre-backs are the natural rest pair of a flat back four.
			d -= feM1RestHysteresis
		}
		return d
	}
	for slot := range e.players {
		p := &e.players[slot]
		if p.IsHomeTeam != home || !feM1Outfielder(p) {
			continue
		}
		if first < 0 || depth(slot) < depth(first) {
			second = first
			first = slot
		} else if second < 0 || depth(slot) < depth(second) {
			second = slot
		}
	}
	if side != nil {
		side.rest = [2]int{first, second}
	}
	if first >= 0 && second >= 0 &&
		feM1ToM(e.players[second].Position).X*dir < feM1ToM(e.players[first].Position).X*dir {
		first, second = second, first
	}
	return first, second
}

// feM1PressPoint is FE `pressPoint` (:78-89): the ball predicted 0.15 s ahead,
// 0.75 m goal-side, clamped to the pitch.
func feM1PressPoint(home bool, ball feM1Vec, vel Vector2F) feM1Vec {
	x := ball.X + vel.X*feM1CarrierPrediction - feM1Dir(home)*feM1ApproachOffset
	y := ball.Y + vel.Y*feM1CarrierPrediction
	return feM1Vec{hClamp(x, -feM1HalfLength, feM1HalfLength), hClamp(y, -feM1HalfWidth, feM1HalfWidth)}
}

// feM1CoverPoint is FE `coverPoint` (:91-97): 6 m goal-side of the ball.
func feM1CoverPoint(home bool, ball feM1Vec) feM1Vec {
	return feM1Vec{ball.X - feM1Dir(home)*feM1CoverDepth, ball.Y}
}
