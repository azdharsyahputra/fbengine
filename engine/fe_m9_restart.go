// FE M9 RestartPlanner: legal restart positions without teleports.
// Port of FootballEngine Restart/RestartPlanner.fs (GPL-3.0,
// https://github.com/DeltaBitsSystem/FootballEngine) layered on Player12's
// restart setup (taker choice, routines). See FE_PORT_PLAN.md §2 M9.
//
// What changes behind FeM9Restart:
//   - nobody is teleported at a restart: the taker and every other player walk
//     (or jog, for long trips) to a planned target;
//   - targets pass through awayFrom (FE) so opponents respect 9.15 m at free
//     kicks, corners and kick-offs, 2 m at throw-ins, the penalty area at goal
//     kicks, and the area plus arc at penalties;
//   - set pieces get a real arrangement: a wall sized by distance and angle,
//     runners spread over near post / penalty spot / far post, the edge of the
//     box, a short option, and a rest defence (full-backs and a holding
//     midfielder) against the opponents' outlet forward;
//   - goal kicks get a build-up shape (CBs split, DM drops, FBs high and wide);
//   - the delay is drawn per restart (longer for dangerous set pieces) and the
//     kick waits until the restart is legal (FE RestartRules.validate), with a
//     hard cap so nothing can freeze.
package engine

import "math"

const (
	feM9WallDistance     = 9.15 // Law 13: opponents at free kicks and corners.
	feM9TargetTolerance  = 0.35 // Plan beyond the boundary (FE TargetTolerance).
	feM9BallClearance    = 2.0  // Team-mates keep this off the ball (FE BallClearance).
	feM9ThrowDistance    = 2.0  // Law 15.
	feM9FootReach        = 1.05 // Executor must reach the ball (FE FootReach).
	feM9SettledMetres    = 2.5  // Set-piece players count as in place within this.
	feM9MaxOvertime      = 15.0 // Seconds the kick may wait beyond its delay.
	feM9ThrowInsetMetres = 3.0  // Throw-in receivers stay on the field (FE 34-31).
	feM9WallSpacing      = 0.85 // Shoulder to shoulder, above body separation.
	feM9WalkPaceMin      = 1.4  // m/s for short repositioning.
	feM9WalkPaceMax      = 5.0  // m/s jog for long trips (never a sprint).
	feM9WalkPacePerMetre = 0.11
)

// ---------------------------------------------------------------- geometry

func feM9Clamp(p Vector2F) Vector2F {
	return Vector2F{hClamp(p.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
		hClamp(p.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
}

func feM9Offset(p Vector2F, dxMetres, dyMetres float64) Vector2F {
	return feM9Clamp(Vector2F{p.X + dxMetres/Pitch_LENGTH_METRES, p.Y + dyMetres/Pitch_WIDTH_METRES})
}

// feM9AwayFrom is FE RestartPlanner.awayFrom: keep point when it is at least
// required metres from spot, otherwise project it radially onto the circle;
// when the pitch clamp pulls it back inside, take the axis point that reaches
// the full distance, nearest to point.
func feM9AwayFrom(spot, point Vector2F, required float64) Vector2F {
	point = feM9Clamp(point)
	d := distance(spot, point)
	projected := point
	if d < required {
		if d < 0.001 {
			projected = feM9Offset(spot, required, 0.0)
		} else {
			dir := metricDirection(spot, point)
			projected = feM9Offset(spot, dir.X*required, dir.Y*required)
		}
	}
	if distance(spot, projected) >= required-1e-6 {
		return projected
	}
	candidates := [4]Vector2F{
		feM9Offset(spot, required, 0.0), feM9Offset(spot, -required, 0.0),
		feM9Offset(spot, 0.0, required), feM9Offset(spot, 0.0, -required)}
	best := candidates[0]
	bestReach, bestGap := -1.0, math.Inf(1)
	for _, c := range candidates {
		reach := math.Min(required, distance(spot, c))
		gap := distance(point, c)
		if reach > bestReach+1e-9 || (math.Abs(reach-bestReach) <= 1e-9 && gap < bestGap) {
			best, bestReach, bestGap = c, reach, gap
		}
	}
	return best
}

// feM9Frame measures positions from the goal a side attacks: depth from the
// goal line, lateral from the centre, positive towards the ball's side.
type feM9Frame struct {
	goalX, inward, side float64
}

func feM9FrameFor(attackingHome bool, ball Vector2F) feM9Frame {
	f := feM9Frame{goalX: 0.0, inward: 1.0, side: 1.0}
	if attackingHome {
		f.goalX, f.inward = 1.0, -1.0
	}
	if ball.Y < Pitch_CENTRE {
		f.side = -1.0
	}
	return f
}

func (f feM9Frame) at(depth, lateral float64) Vector2F {
	return feM9Clamp(Vector2F{f.goalX + f.inward*depth/Pitch_LENGTH_METRES,
		Pitch_CENTRE + f.side*lateral/Pitch_WIDTH_METRES})
}

func (f feM9Frame) depth(p Vector2F) float64 { return math.Abs(p.X-f.goalX) * Pitch_LENGTH_METRES }

func (f feM9Frame) lateral(p Vector2F) float64 {
	return (p.Y - Pitch_CENTRE) * Pitch_WIDTH_METRES * f.side
}

// ------------------------------------------------------------------- roles

// feM9RestScore ranks who stays back when the own side attacks a set piece:
// full-backs and holding midfielders first, centre-backs and strikers last.
func feM9RestScore(p *MatchPlayer) float64 {
	switch p.Player.GetRole() {
	case PlayerRole_LB, PlayerRole_RB:
		return 3.0
	case PlayerRole_CDM:
		return 2.6
	case PlayerRole_CM:
		return 2.0
	case PlayerRole_LM, PlayerRole_RM:
		return 1.0
	case PlayerRole_CAM:
		return 0.3
	case PlayerRole_LW, PlayerRole_RW:
		return 0.2
	case PlayerRole_CB:
		return -1.0
	case PlayerRole_ST:
		return -2.0
	}
	return 0.0
}

func feM9IsHolding(p *MatchPlayer) bool {
	r := p.Player.GetRole()
	return r == PlayerRole_CDM || r == PlayerRole_CM
}

// feM9OutletScore ranks who the defending side leaves upfield for the counter.
func feM9OutletScore(p *MatchPlayer) float64 {
	switch p.Player.GetRole() {
	case PlayerRole_ST:
		return 3.0
	case PlayerRole_LW, PlayerRole_RW:
		return 2.0
	case PlayerRole_CAM:
		return 1.8
	case PlayerRole_LM, PlayerRole_RM:
		return 1.5
	case PlayerRole_CM:
		return 0.5
	case PlayerRole_LB, PlayerRole_RB:
		return -1.0
	case PlayerRole_CB:
		return -2.0
	}
	return 0.0
}

func (e *MatchEngine) feM9Squad(home bool, skip *MatchPlayer) []*MatchPlayer {
	out := make([]*MatchPlayer, 0, 11)
	for i := range e.players {
		p := &e.players[i]
		if active(p) && p.IsHomeTeam == home && !p.IsGoalkeeper && p != skip {
			out = append(out, p)
		}
	}
	return out
}

func feM9SortBy(list []*MatchPlayer, score func(*MatchPlayer) float64) {
	insertionSort(list, len(list), func(a, b *MatchPlayer) bool {
		sa, sb := score(a), score(b)
		if sa != sb {
			return sa > sb
		}
		return a.Player.GetId() < b.Player.GetId()
	})
}

// --------------------------------------------------------------- set piece

// feM9WallSize: 5 close and central, fewer with distance and angle; wide
// crossing positions get a two-man block at most.
func feM9WallSize(goalMetres, lateralMetres float64) int {
	n := 0
	switch {
	case goalMetres <= 20.0:
		n = 5
	case goalMetres <= 24.0:
		n = 4
	case goalMetres <= 29.0:
		n = 3
	case goalMetres <= 36.0:
		n = 2
	case goalMetres <= SetPiece_CROSSING_FREE_KICK_METRES:
		n = 1
	}
	if lateralMetres > Pitch_PENALTY_AREA_HALF_WIDTH_METRES {
		n = min(n, 2)
	} else if lateralMetres > 12.0 && n > 2 {
		n--
	}
	return n
}

type feM9Spot struct{ depth, lateral float64 }

// feM9ArrangeSetPiece places both sides for a corner or an attacking free
// kick. Targets go to MovementTarget; everyone walks there.
func (e *MatchEngine) feM9ArrangeSetPiece(attackingHome bool, ball Vector2F, corner bool) {
	f := feM9FrameFor(attackingHome, ball)
	taker := e.restartTaker()
	jit := func(m float64) float64 { return e.randomFloat(-m, m) }
	bd := f.depth(ball)
	bl := math.Abs(f.lateral(ball))
	goal := Vector2F{f.goalX, Pitch_CENTRE}
	goalMetres := distance(ball, goal)
	direct := !corner && goalMetres <= SetPiece_DIRECT_FREE_KICK_METRES &&
		bl <= SetPiece_DIRECT_FREE_KICK_WIDTH_METRES
	// Free kicks: the defending line, and the attackers just onside of it.
	line := hClamp(bd*0.5, 10.5, 15.5)

	// ---- attacking side
	attackers := e.feM9Squad(attackingHome, taker)
	feM9SortBy(attackers, feM9RestScore)
	n := len(attackers)
	restCount := min(SetPiece_REST_DEFENDERS, max(1, n-5))
	if n <= 2 {
		restCount = 0
	}
	rest := attackers[:restCount]
	forward := append([]*MatchPlayer(nil), attackers[restCount:]...)
	feM9SortBy(forward, setPieceBoxThreat)

	screenDepth := math.Max(26.0, bd+4.0) + jit(2.0)
	deepDepth := math.Min(math.Max(46.0, bd+12.0), 62.0) + jit(2.0)
	screenUsed := false
	wide := 1.0
	if e.randomFloat(0.0, 1.0) < 0.5 {
		wide = -1.0
	}
	for _, p := range rest {
		if !screenUsed && feM9IsHolding(p) {
			p.MovementTarget = f.at(screenDepth, -3.0+jit(3.0))
			screenUsed = true
			continue
		}
		p.MovementTarget = f.at(deepDepth+jit(1.5), wide*(8.0+jit(2.5)))
		wide = -wide
	}

	var boxSlots []feM9Spot
	var edgeSlots []feM9Spot
	switch {
	case corner:
		boxSlots = []feM9Spot{{6.5, 0.5}, {11.0, 0.0}, {6.0, -4.5}, {5.0, 3.5}, {9.0, -8.5}, {12.5, -5.0}}
		edgeSlots = []feM9Spot{{18.5, 2.0}, {21.5, -7.0}}
	case direct:
		d := line + 0.6
		boxSlots = []feM9Spot{{d, -4.5}, {d, 5.5}, {d, -8.5}, {d, 9.5}, {d, -12.5}, {d, 13.0}}
		edgeSlots = []feM9Spot{{math.Max(line+7.0, 19.0), -6.0}}
	default:
		d := line + 0.6
		boxSlots = []feM9Spot{{d, 5.0}, {d, 1.0}, {d, -3.0}, {d, -7.0}, {d, -11.0}, {d, 9.0}}
		edgeSlots = []feM9Spot{{math.Max(line+7.0, 19.0), -3.0}, {math.Max(line+10.0, 22.0), 6.0}}
	}
	edgeCount := 0
	if len(forward) >= 5 {
		edgeCount = 1
	}
	boxCount := min(len(forward)-edgeCount, len(boxSlots))
	extra := len(forward) - edgeCount - boxCount
	edgeCount += extra
	shortOption := Optional[Vector2F]{}
	var runners []*MatchPlayer
	for i, p := range forward {
		switch {
		case i < boxCount:
			s := boxSlots[i]
			p.MovementTarget = f.at(s.depth+jit(0.9), s.lateral+jit(1.0))
			runners = append(runners, p)
		default:
			k := i - boxCount
			if corner && k == 0 && edgeCount > 1 && e.randomFloat(0.0, 1.0) < 0.35 {
				// Short option about ten metres from the flag.
				p.MovementTarget = f.at(8.0+jit(1.5), 26.0+jit(1.5))
				shortOption = hSome(p.MovementTarget)
				continue
			}
			if direct && k == 0 {
				// A second man over the ball (dummy / layoff).
				p.MovementTarget = feM9AwayFrom(ball,
					feM9Offset(ball, 0.0, -f.side*1.4), 1.2)
				continue
			}
			s := edgeSlots[min(k, len(edgeSlots)-1)]
			p.MovementTarget = f.at(s.depth+jit(1.5), s.lateral+float64(k/len(edgeSlots))*4.0+jit(2.0))
		}
	}

	// ---- defending side
	required := feM9WallDistance + feM9TargetTolerance
	defenders := e.feM9Squad(!attackingHome, nil)
	if keeper := e.findGoalkeeper(!attackingHome); keeper != nil {
		var t Vector2F
		switch {
		case corner:
			t = f.at(0.7, -0.6+jit(0.4))
		case direct:
			t = f.at(0.6, -1.2)
		default:
			t = f.at(hClamp(line*0.25, 1.0, 4.0), -1.0)
		}
		keeper.MovementTarget = feM9AwayFrom(ball, t, required)
	}
	if own := e.findGoalkeeper(attackingHome); own != nil && own != taker {
		own.MovementTarget = f.at(Pitch_LENGTH_METRES-18.0, 0.0)
	}
	feM9SortBy(defenders, feM9OutletScore)
	outlets := 0
	if len(defenders) >= 6 {
		outlets = 1
		if corner && len(defenders) >= 10 && e.randomFloat(0.0, 1.0) < 0.3 {
			outlets = 2
		}
	}
	for i := 0; i < outlets; i++ {
		lat := 3.0 + jit(3.0)
		if i == 1 {
			lat = -lat - 6.0
		}
		defenders[i].MovementTarget = f.at(deepDepth-6.0+jit(2.0), lat)
	}
	pool := append([]*MatchPlayer(nil), defenders[outlets:]...)
	feM9SortBy(pool, func(p *MatchPlayer) float64 { return p.Defending + p.Physicality })

	wallN := 0
	if !corner {
		wallN = min(feM9WallSize(goalMetres, bl), len(pool))
	}
	if wallN > 0 {
		// The wall stands on the line from the ball to the near-post half of
		// the goal (wide: to the near-post zone), its end man on that post.
		aim := f.at(0.0, 1.8)
		if bl > Pitch_PENALTY_AREA_HALF_WIDTH_METRES || !direct {
			aim = f.at(6.0, 2.0)
		}
		u := metricDirection(ball, aim)
		v := Vector2F{-u.Y, u.X}
		centre := feM9Offset(ball, u.X*required, u.Y*required)
		wall := pool[len(pool)-wallN:]
		for k, p := range wall {
			off := (float64(k) - float64(wallN-1)/2.0) * feM9WallSpacing
			p.MovementTarget = feM9AwayFrom(ball, feM9Offset(centre, v.X*off, v.Y*off), required)
		}
		pool = pool[:len(pool)-wallN]
	}
	next := 0
	for _, r := range runners {
		if next >= len(pool) {
			break
		}
		lat := f.lateral(r.MovementTarget)
		pool[next].MovementTarget = f.at(math.Max(f.depth(r.MovementTarget)-0.9, 0.8), lat*0.93)
		next++
	}
	var zones []feM9Spot
	if corner {
		zones = []feM9Spot{{1.2, 4.3}, {5.3, 1.8}, {17.0, 1.0}, {5.3, -2.6}, {3.0, -5.5}, {11.0, -6.0}}
	} else {
		zones = []feM9Spot{{line - 1.5, -1.0}, {line + 6.0, 0.0}, {line - 3.0, 4.0}, {line - 3.0, -6.0}}
	}
	if shortOption.HasValue && next < len(pool) {
		// Someone closes the short option, from the legal distance.
		pool[next].MovementTarget = feM9AwayFrom(ball, feM9Offset(shortOption.Value, f.inward*1.0, 0.0), required)
		next++
	}
	for z := 0; next < len(pool); z, next = z+1, next+1 {
		s := zones[z%len(zones)]
		pool[next].MovementTarget = f.at(s.depth+jit(0.8), s.lateral+float64(z/len(zones))*3.0+jit(0.8))
	}
	for _, p := range defenders {
		p.MovementTarget = feM9AwayFrom(ball, p.MovementTarget, required)
	}
	for _, p := range attackers {
		p.MovementTarget = feM9AwayFrom(ball, p.MovementTarget, 1.2)
	}
}

// ----------------------------------------------------------------- penalty

func (e *MatchEngine) feM9ArrangePenalty(attackingHome bool, spot Vector2F, taker *MatchPlayer) {
	f := feM9FrameFor(attackingHome, Vector2F{0.5, Pitch_CENTRE + 0.01})
	jit := func(m float64) float64 { return e.randomFloat(-m, m) }
	arc := feM9WallDistance + 1.0
	if keeper := e.findGoalkeeper(!attackingHome); keeper != nil {
		keeper.MovementTarget = f.at(0.0, 0.0)
	}
	if own := e.findGoalkeeper(attackingHome); own != nil && own != taker {
		own.MovementTarget = f.at(Pitch_LENGTH_METRES-16.0, 0.0)
	}
	attackers := e.feM9Squad(attackingHome, taker)
	feM9SortBy(attackers, setPieceBoxThreat)
	defenders := e.feM9Squad(!attackingHome, nil)
	feM9SortBy(defenders, feM9OutletScore)
	outlet := 0
	if len(defenders) >= 6 {
		defenders[0].MovementTarget = f.at(36.0+jit(3.0), 3.0+jit(3.0))
		outlet = 1
	}
	pool := append([]*MatchPlayer(nil), defenders[outlet:]...)
	feM9SortBy(pool, func(p *MatchPlayer) float64 { return p.Defending + p.Physicality })
	edgeD := min(5, len(pool))
	edgeA := min(4, max(0, len(attackers)-2))
	// Pairs along the edge of the area on both sides of the D: the defender
	// on the inside, the attacker beside him.
	slot := func(k int) (Vector2F, Vector2F) {
		side := 1.0
		if k%2 == 1 {
			side = -1.0
		}
		mag := 8.8 + 2.6*float64(k/2) + jit(0.4)
		if mag > 19.0 {
			// Outside the arc, further out.
			return f.at(21.5+jit(0.5), side*(mag-14.0)), f.at(23.0+jit(0.5), side*(mag-12.0))
		}
		return f.at(17.1, side*mag), f.at(17.6+jit(0.3), side*(mag+1.1))
	}
	for k := 0; k < max(edgeD, edgeA); k++ {
		d, a := slot(k)
		if k < edgeD {
			pool[k].MovementTarget = feM9AwayFrom(spot, d, arc)
		}
		if k < edgeA {
			attackers[k].MovementTarget = feM9AwayFrom(spot, a, arc)
		}
	}
	// The rest wait behind the D for the rebound; the attackers' last two
	// (the most defensive) stay back against the counter.
	for k := edgeD; k < len(pool); k++ {
		pool[k].MovementTarget = f.at(23.5+jit(1.5), float64(k%2*2-1)*(4.0+2.5*float64(k-edgeD)+jit(1.0)))
	}
	wide := 1.0
	for k := edgeA; k < len(attackers); k++ {
		if k >= len(attackers)-2 {
			attackers[k].MovementTarget = f.at(40.0+jit(3.0), wide*(9.0+jit(3.0)))
		} else {
			attackers[k].MovementTarget = f.at(24.5+jit(1.5), wide*(2.0+2.5*float64(k-edgeA)+jit(1.0)))
		}
		wide = -wide
	}
	if taker != nil {
		taker.MovementTarget = feM9AwayFrom(spot, f.at(13.0, 0.0), 1.0)
	}
}

// --------------------------------------------------------------- goal kick

// feM9ArrangeGoalKick: build-up shape for the side taking the kick. Short:
// centre-backs split to the corners of the area, a holding midfielder drops
// between them, full-backs high and wide. Long: the block pushes up.
func (e *MatchEngine) feM9ArrangeGoalKick(home bool) {
	f := feM9FrameFor(!home, Vector2F{0.5, Pitch_CENTRE + 0.01}) // depth from own goal
	jit := func(m float64) float64 { return e.randomFloat(-m, m) }
	long := e.randomFloat(0.0, 1.0) < 0.35
	push := 0.0
	if long {
		push = 13.0
	}
	squad := e.feM9Squad(home, nil)
	sideOf := func(p *MatchPlayer, fallback float64) float64 {
		dy := (p.BasePosition.Y - Pitch_CENTRE) * f.side
		if math.Abs(dy) < 0.02 {
			return fallback
		}
		return math.Copysign(1.0, dy)
	}
	cb, dm := 0, 0
	for _, p := range squad {
		var depth, lat float64
		switch p.Player.GetRole() {
		case PlayerRole_CB:
			s := sideOf(p, float64(cb%2*2-1))
			cb++
			if long {
				depth, lat = 26.0+jit(3.0), s*(11.0+jit(2.0))
			} else {
				depth, lat = 14.0+jit(2.0), s*(17.0+jit(2.0))
			}
		case PlayerRole_LB, PlayerRole_RB:
			depth, lat = 30.0+push+jit(5.0), sideOf(p, 1.0)*(29.0+jit(2.5))
		case PlayerRole_CDM, PlayerRole_CM:
			if dm == 0 && !long {
				depth, lat = 21.0+jit(3.0), jit(4.0)
			} else {
				depth, lat = 33.0+push+jit(4.0), sideOf(p, float64(dm%2*2-1))*(9.0+jit(3.0))
			}
			dm++
		case PlayerRole_ST:
			depth, lat = 60.0+push*0.6+jit(5.0), jit(5.0)
		default:
			depth, lat = 47.0+push+jit(6.0), sideOf(p, 1.0)*(20.0+jit(6.0))
		}
		p.MovementTarget = f.at(depth, lat)
	}
}

// ---------------------------------------------------------------- kick-off

func (e *MatchEngine) feM9KickOffTaker(home bool) *MatchPlayer {
	var best *MatchPlayer
	bestGap := math.Inf(1)
	for i := range e.players {
		p := &e.players[i]
		if !active(p) || p.IsHomeTeam != home || p.IsGoalkeeper {
			continue
		}
		x := Pitch_KICKOFF_FORMATION_INSET + p.BasePosition.X*Pitch_KICKOFF_FORMATION_SCALE
		if !home {
			x = (1.0 - Pitch_KICKOFF_FORMATION_INSET) - (1.0-p.BasePosition.X)*Pitch_KICKOFF_FORMATION_SCALE
		}
		if gap := distance(Vector2F{x, p.BasePosition.Y}, Vector2F{Pitch_CENTRE, Pitch_CENTRE}); gap < bestGap {
			best, bestGap = p, gap
		}
	}
	return best
}

func feM9KickOffSpot(home bool) Vector2F {
	if home {
		return Vector2F{Pitch_HOME_KICKOFF_X, Pitch_CENTRE}
	}
	return Vector2F{Pitch_AWAY_KICKOFF_X, Pitch_CENTRE}
}

// feM9GoalWalk: during the celebration the next kick-off taker walks to the
// centre spot, so the kick-off itself moves nobody far.
func (e *MatchEngine) feM9GoalWalk() {
	home := !e.goalScoredByHome
	if taker := e.feM9KickOffTaker(home); taker != nil {
		taker.MovementTarget = feM9KickOffSpot(home)
	}
}

// ------------------------------------------------------------ per-tick law

// feM9LegalTarget keeps every target legal while players walk (FE targets are
// planned once; re-applying awayFrom is idempotent).
func (e *MatchEngine) feM9LegalTarget(player, taker *MatchPlayer) {
	if taker == nil {
		return
	}
	ball := e.ball.Position
	if player == taker {
		player.MovementTarget = ball
		return
	}
	opponent := player.IsHomeTeam != taker.IsHomeTeam
	switch e.state {
	case MatchState_FREE_KICK, MatchState_CORNER_KICK:
		if opponent {
			player.MovementTarget = feM9AwayFrom(ball, player.MovementTarget, feM9WallDistance+feM9TargetTolerance)
		} else if !e.restartIsSetPiece {
			player.MovementTarget = feM9AwayFrom(ball, player.MovementTarget, feM9BallClearance)
		}
	case MatchState_THROW_IN:
		t := player.MovementTarget
		inset := feM9ThrowInsetMetres / Pitch_WIDTH_METRES
		t.Y = hClamp(t.Y, inset, 1.0-inset)
		if opponent {
			player.MovementTarget = feM9AwayFrom(ball, t, feM9ThrowDistance+feM9TargetTolerance)
		} else {
			player.MovementTarget = feM9AwayFrom(ball, t, feM9BallClearance)
		}
	case MatchState_GOAL_KICK:
		if opponent {
			t := feM9AwayFrom(ball, player.MovementTarget, feM9WallDistance+feM9TargetTolerance)
			// Law 16: opponents stay outside the area until the ball is in play.
			if inPenaltyArea(t, taker.IsHomeTeam) {
				edge := Pitch_LEFT_PENALTY_AREA_EDGE + 1.0/Pitch_LENGTH_METRES
				if !taker.IsHomeTeam {
					edge = Pitch_RIGHT_PENALTY_AREA_EDGE - 1.0/Pitch_LENGTH_METRES
				}
				t.X = edge
			}
			player.MovementTarget = t
		} else {
			player.MovementTarget = feM9AwayFrom(ball, player.MovementTarget, feM9BallClearance)
		}
	}
}

// feM9Integrate walks the batch with a pace that grows with the distance to
// the target (walk when close, jog for long trips), never above a jog.
func (e *MatchEngine) feM9Integrate(batch []uint8, dt float64) {
	var saved [32]float64
	for i, slot := range batch {
		p := &e.players[slot]
		saved[i] = p.MaxSpeed
		walk := Player_RESTART_WALK_SPEED_SCALE * p.MaxSpeed
		if walk <= EPSILON {
			continue
		}
		want := hClamp(feM9WalkPaceMin+feM9WalkPacePerMetre*distance(p.Position, p.MovementTarget),
			feM9WalkPaceMin, feM9WalkPaceMax)
		p.MaxSpeed *= hClamp(want/walk, 1.0, 2.6)
	}
	e.integrateMovements(batch, dt, true)
	for i, slot := range batch {
		e.players[slot].MaxSpeed = saved[i]
	}
}

// ---------------------------------------------------------------- validate

// feM9Ready is FE RestartRules.validate on current positions: the executor
// reaches the ball, opponents respect the restart distance, nobody else is on
// the ball, and set-piece players have reached their places.
func (e *MatchEngine) feM9Ready() bool {
	taker := e.restartTaker()
	if taker == nil || !active(taker) || e.state == MatchState_KICK_OFF {
		return true
	}
	ball := e.ball.Position
	if distance(taker.Position, ball) > feM9FootReach {
		return false
	}
	attHome := taker.IsHomeTeam
	for i := range e.players {
		p := &e.players[i]
		if !active(p) || p == taker {
			continue
		}
		d := distance(p.Position, ball)
		opponent := p.IsHomeTeam != attHome
		switch e.state {
		case MatchState_FREE_KICK, MatchState_CORNER_KICK:
			if opponent && d < feM9WallDistance-1.0 {
				return false
			}
		case MatchState_THROW_IN:
			if opponent && d < feM9ThrowDistance-0.5 {
				return false
			}
		case MatchState_GOAL_KICK:
			if opponent && inPenaltyArea(p.Position, attHome) {
				return false
			}
		case MatchState_PENALTY:
			if p.IsGoalkeeper && opponent {
				if distance(p.Position, p.MovementTarget) > 1.0 {
					return false
				}
			} else if !p.IsGoalkeeper && (inPenaltyArea(p.Position, !attHome) ||
				distance(p.Position, ball) < feM9WallDistance-0.3) {
				return false
			}
		}
		if e.state != MatchState_THROW_IN && d < 1.0 {
			return false
		}
		if (e.restartIsSetPiece || e.state == MatchState_PENALTY) &&
			distance(p.Position, p.MovementTarget) > feM9SettledMetres {
			return false
		}
	}
	return true
}

// feM9Delay draws the minimum wait before a restart (seconds); the kick then
// also waits for feM9Ready. Dangerous set pieces take longest.
func (e *MatchEngine) feM9Delay(state MatchState, goalMetres float64) float64 {
	switch state {
	case MatchState_THROW_IN:
		return e.randomFloat(9.0, 16.0)
	case MatchState_GOAL_KICK:
		return e.randomFloat(19.0, 32.0)
	case MatchState_CORNER_KICK:
		return e.randomFloat(22.0, 34.0)
	case MatchState_PENALTY:
		return e.randomFloat(45.0, 70.0)
	case MatchState_FREE_KICK:
		if goalMetres <= SetPiece_CROSSING_FREE_KICK_METRES {
			danger := hClamp((SetPiece_CROSSING_FREE_KICK_METRES-goalMetres)/25.0, 0.0, 1.0)
			return 14.0 + 18.0*danger + e.randomFloat(0.0, 10.0)
		}
		return e.randomFloat(7.0, 15.0)
	}
	return Timing_KICKOFF_DELAY_SECONDS
}

// feM9HalfTimeWalk (ours): toward the kick-off formation during the
// half-time break, paced to arrive as the break ends (the break may be
// shortened to a few seconds), so setupKickOff's placement is not a jump
// across the pitch in the replay.
func (e *MatchEngine) feM9HalfTimeWalk(dt float64) {
	share := 1.0
	if e.setPieceTimer > dt {
		share = dt / e.setPieceTimer
	}
	// The next half is kicked off by the home side in odd periods.
	home := (e.period+1)%2 == 1
	taker := e.feM9KickOffTaker(home)
	for i := range e.players {
		p := &e.players[i]
		if !active(p) {
			continue
		}
		to := e.kickOffPosition(p)
		if p == taker {
			to = feM9KickOffSpot(home)
		}
		p.Position.X += (to.X - p.Position.X) * share
		p.Position.Y += (to.Y - p.Position.Y) * share
		p.Velocity = Vector2F{}
		p.MovementTarget = to
		p.TargetOwner = TargetOwner_M9
		p.TargetReason = TargetReason_HALF_TIME_WALK
	}
}
