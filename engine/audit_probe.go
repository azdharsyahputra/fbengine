// Audit probe (ours, read-only): ground-truth measurements taken from the
// engine state each step, for checking the viewer/cek replay metrics and for
// layer-ownership statistics. Diagnostics.Audit enables it (the CLI maps
// FMENGINE_AUDIT=1); it reads state only (no RNG draws, no writes to the
// simulation), so replays are identical with it on or off. State is per match.
package engine

import (
	"fmt"
	"io"
	"math"
	"sort"
)

type auditState struct {
	steps                                int
	held, pass, aerial, shot, loose      int
	carrierGap                           []float64 // m, ball to carrier while held
	looseClaim, looseRunners, passRunner []int     // per step
	pressNear5, pressIntent              []int     // defenders within 5 m / PRESS intents, ball held
	boxSteps, boxDef, boxAtt, boxFree    [3]int    // per zone: 0 front of box (= cek crit), 1 wide final third, 2 inside box
	ownerM1, ownerP12Def                 int
	layer                                map[string]int
	owners                               [TargetOwner_COUNT]int // player-steps per target owner (live play)
	ownerIntent                          map[string]int         // "owner intent" pairs (live play)
	deadOwners                           [TargetOwner_COUNT]int // player-steps per owner during stoppages
	// Possession changes (live play, engine truth): cause and ping-pong.
	poss struct {
		lastTeam   int // 0 home, 1 away, -1 none (new chain)
		lastHolder *MatchPlayer
		sinceHeld  int // live steps since anyone held the ball
		sawPass    bool
		sawShot    bool
		passTeam   int
		heldSince  float64
		changes    map[string]int
		history    []auditChange
		pingpong   map[string]int
	}
	// Chase anatomy (cek chase.js definition: defenders >3 m/s, >5 m away,
	// heading at the ball within cos 0.8, while the ball is held).
	chase struct {
		held, three int
		who         map[string]int
	}
	// Pressure on a carrier in his own third (build-up), defending side's view.
	buildUp struct {
		steps, engaged   int
		nearest, engager []float64
		forwardNearest   []float64
		modeCounterpress int
	}
	// Visual steals: an opponent nearer the ball than the carrier and within
	// 0.8 m (what the replay holder test reads as a change of possession).
	visual struct {
		steps, episodes, heldSteps int
		on                         bool
		byIntent, byM1Role         map[string]int
		gapDef, gapCar             []float64
	}
}

type auditChange struct {
	t     float64
	team  int
	cause string
}

func (a *auditState) addLayer(k string) {
	if a.layer == nil {
		a.layer = map[string]int{}
	}
	a.layer[k]++
}

func (e *MatchEngine) auditLayers(p *MatchPlayer, duty bool, base, structured, final Vector2F, owner TargetOwner) {
	moved := func(a, b Vector2F) bool {
		return math.Hypot((a.X-b.X)*Pitch_LENGTH_METRES, (a.Y-b.Y)*Pitch_WIDTH_METRES) > 0.5
	}
	k := fmt.Sprintf("intent=%s", auditIntent(p.Intent))
	if duty {
		k += " M5duty"
	}
	if moved(base, structured) {
		k += " +structure(" + owner.String() + ")"
	}
	if moved(structured, final) {
		k += " +refine(M4)"
	}
	e.audit.addLayer(k)
}

func auditIntent(i PlayerIntent) string {
	return [...]string{"HOLD", "CARRY", "SUPPORT", "RECEIVE", "RUN", "BOX", "OVERLAP", "PRESS", "COVER", "BLOCK", "MARK", "LOOSE", "RECOVER", "GK"}[i]
}

// auditStep samples one step (called by RunReplay): target owners every
// step, the rest in live play only.
func (e *MatchEngine) auditStep() {
	if !e.diag.Audit {
		return
	}
	a := &e.audit
	for i := range e.players {
		p := &e.players[i]
		if !active(p) || p.TargetOwner >= TargetOwner_COUNT {
			continue
		}
		if e.state != MatchState_PLAYING {
			a.deadOwners[p.TargetOwner]++
			continue
		}
		a.owners[p.TargetOwner]++
		if a.ownerIntent == nil {
			a.ownerIntent = map[string]int{}
		}
		k := p.TargetOwner.String() + ":" + p.TargetReason.String()
		if p.TargetRefiner != TargetOwner_NONE {
			k += "+" + p.TargetRefiner.String()
		}
		a.ownerIntent[k]++
	}
	if e.state != MatchState_PLAYING {
		a.poss.sinceHeld, a.poss.sawPass, a.poss.sawShot = 0, false, false
		a.poss.lastTeam = -1 // a restart starts a new possession chain
		return
	}
	e.auditPossession()
	a.steps++
	ball := toMetres(e.ball.Position)
	carrier := e.findMatchPlayer(e.ball.PossessedBy)
	towards := func(p *MatchPlayer) bool { // >3 m/s, >5 m away, heading at the ball (cos>0.8)
		pm := toMetres(p.Position)
		v := p.Velocity // already m/s
		sp := math.Hypot(v.X, v.Y)
		dx, dy := ball.X-pm.X, ball.Y-pm.Y
		d := math.Hypot(dx, dy)
		return sp > 3 && d > 5 && (v.X*dx+v.Y*dy)/(sp*d) > 0.8
	}
	count := func(f func(p *MatchPlayer) bool) int {
		n := 0
		for i := range e.players {
			p := &e.players[i]
			if active(p) && !p.IsGoalkeeper && f(p) {
				n++
			}
		}
		return n
	}
	switch {
	case carrier != nil:
		a.held++
		e.auditBuildUp(carrier)
		e.auditChase(carrier, ball)
		e.auditVisualSteal(carrier, ball)
		cm := toMetres(carrier.Position)
		a.carrierGap = append(a.carrierGap, math.Hypot(ball.X-cm.X, ball.Y-cm.Y))
		def := !carrier.IsHomeTeam
		a.pressNear5 = append(a.pressNear5, count(func(p *MatchPlayer) bool {
			pm := toMetres(p.Position)
			return p.IsHomeTeam == def && math.Hypot(pm.X-cm.X, pm.Y-cm.Y) < 5
		}))
		a.pressIntent = append(a.pressIntent, count(func(p *MatchPlayer) bool {
			return p.IsHomeTeam == def && (p.Intent == PlayerIntent_PRESS_BALL || p.Intent == PlayerIntent_COVER_PRESS)
		}))
		if e.feM1Owns(carrier) || e.feM1Of().sides[hBoolIndex(def)].owns {
			a.ownerM1++
		} else {
			a.ownerP12Def++
		}
		// Box coverage, same zones and definitions as viewer/cek/crit.js:
		// ball depth u from the defended goal line; zone 0 = in front of the
		// box (16 <= u <= 35, 14 <= y <= 54, cek's "bertahan depan kotak"),
		// 1 = wide in the final third (u <= 35, outside that band), 2 = in
		// the box. Defenders/attackers = outfielders; an attacker in the box
		// is free with no defending outfielder within 3 m.
		gx := 0.0
		if carrier.IsHomeTeam {
			gx = Pitch_LENGTH_METRES
		}
		u := math.Abs(ball.X - gx)
		zone := -1
		switch {
		case u >= 16 && u <= 35 && ball.Y >= 14 && ball.Y <= 54:
			zone = 0
		case u < 16 && ball.Y > 13.84 && ball.Y < 54.16:
			zone = 2
		case u <= 35:
			zone = 1
		}
		if zone >= 0 {
			a.boxSteps[zone]++
			inBox := func(p *MatchPlayer) bool {
				pm := toMetres(p.Position)
				return math.Abs(pm.X-gx) < 16.5 && pm.Y > 13.84 && pm.Y < 54.16
			}
			for i := range e.players {
				p := &e.players[i]
				if !active(p) || p.IsGoalkeeper || !inBox(p) {
					continue
				}
				if p.IsHomeTeam == def {
					a.boxDef[zone]++
					continue
				}
				a.boxAtt[zone]++
				pm := toMetres(p.Position)
				free := true
				for j := range e.players {
					q := &e.players[j]
					if active(q) && !q.IsGoalkeeper && q.IsHomeTeam == def {
						qm := toMetres(q.Position)
						if math.Hypot(qm.X-pm.X, qm.Y-pm.Y) <= 3 {
							free = false
						}
					}
				}
				if free {
					a.boxFree[zone]++
				}
			}
		}
	case e.ball.IsShot:
		a.shot++
	case e.ball.IsPass && e.ball.IsAerialDelivery:
		a.aerial++
		a.passRunner = append(a.passRunner, count(towards))
	case e.ball.IsPass:
		a.pass++
		a.passRunner = append(a.passRunner, count(towards))
	default:
		a.loose++
		a.looseRunners = append(a.looseRunners, count(func(p *MatchPlayer) bool {
			if !towards(p) {
				return false
			}
			side := "def"
			if e.lastControlledTeamHome.HasValue && e.lastControlledTeamHome.Value == p.IsHomeTeam {
				side = "last-control"
			}
			a.addLayer("LOOSE-RUNNER " + side + " intent=" + auditIntent(p.Intent))
			return true
		}))
		a.looseClaim = append(a.looseClaim, count(func(p *MatchPlayer) bool {
			return p.Intent == PlayerIntent_CLAIM_LOOSE_BALL || p.Intent == PlayerIntent_PRESS_BALL || p.Intent == PlayerIntent_COVER_PRESS
		}))
	}
}

func auditQ(v []int, q float64) int {
	if len(v) == 0 {
		return -1
	}
	s := append([]int(nil), v...)
	sort.Ints(s)
	return s[int(float64(len(s)-1)*q)]
}

func auditShare(v []int, min int) float64 {
	if len(v) == 0 {
		return 0
	}
	n := 0
	for _, x := range v {
		if x >= min {
			n++
		}
	}
	return 100 * float64(n) / float64(len(v))
}

// auditReport writes the probe summary (RunReplay stores it in ReplayData.Audit).
func (e *MatchEngine) auditReport(w io.Writer) {
	if !e.diag.Audit {
		return
	}
	a := &e.audit
	t := float64(a.steps)
	fmt.Fprintf(w, "AUDIT live steps %d | ball held %.0f%% ground pass %.0f%% aerial %.0f%% shot %.1f%% truly loose %.1f%%\n",
		a.steps, 100*float64(a.held)/t, 100*float64(a.pass)/t, 100*float64(a.aerial)/t, 100*float64(a.shot)/t, 100*float64(a.loose)/t)
	g := append([]float64(nil), a.carrierGap...)
	sort.Float64s(g)
	far := 0
	for _, x := range g {
		if x > 0.8 {
			far++
		}
	}
	if len(g) > 0 {
		fmt.Fprintf(w, "AUDIT held: ball-carrier gap median %.2f m p90 %.2f m, >0.8 m (replay holder test fails) %.0f%%\n",
			g[len(g)/2], g[len(g)*9/10], 100*float64(far)/float64(len(g)))
	}
	fmt.Fprintf(w, "AUDIT truly loose: runners-at-ball median %d p90 %d ≥5 %.1f%% | claim/press intents median %d ≥3 %.1f%%\n",
		auditQ(a.looseRunners, .5), auditQ(a.looseRunners, .9), auditShare(a.looseRunners, 5), auditQ(a.looseClaim, .5), auditShare(a.looseClaim, 3))
	fmt.Fprintf(w, "AUDIT pass in flight: runners-at-ball median %d p90 %d ≥5 %.1f%%\n",
		auditQ(a.passRunner, .5), auditQ(a.passRunner, .9), auditShare(a.passRunner, 5))
	fmt.Fprintf(w, "AUDIT pressing (held): defenders <5 m median %d ≥2 %.0f%% ≥3 %.1f%% | press/cover intents median %d | M1 owns defence %.0f%% of held steps\n",
		auditQ(a.pressNear5, .5), auditShare(a.pressNear5, 2), auditShare(a.pressNear5, 3), auditQ(a.pressIntent, .5),
		100*float64(a.ownerM1)/math.Max(1, float64(a.ownerM1+a.ownerP12Def)))
	for z, name := range []string{"front of box (cek crit zone)", "wide final third", "in the box"} {
		if a.boxSteps[z] > 0 {
			fmt.Fprintf(w, "AUDIT box [%s]: defenders in box %.2f attackers in box %.2f free attackers (no defender <=3 m) %.0f%%\n",
				name, float64(a.boxDef[z])/float64(a.boxSteps[z]), float64(a.boxAtt[z])/float64(a.boxSteps[z]),
				100*float64(a.boxFree[z])/math.Max(1, float64(a.boxAtt[z])))
		}
	}
	share := func(c [TargetOwner_COUNT]int) string {
		t := 0
		for _, n := range c {
			t += n
		}
		out := ""
		for o, n := range c {
			if n > 0 {
				out += fmt.Sprintf(" %s %.1f%%", TargetOwner(o), 100*float64(n)/float64(t))
			}
		}
		return out
	}
	e.auditPossessionReport(w)
	e.auditBuildUpReport(w)
	e.auditChaseReport(w)
	e.auditVisualReport(w)
	fmt.Fprintf(w, "AUDIT target owner (player-steps) live:%s | stoppages:%s\n", share(a.owners), share(a.deadOwners))
	type kv struct {
		k string
		n int
	}
	var oi []kv
	ot := 0
	for k, n := range a.ownerIntent {
		oi = append(oi, kv{k, n})
		ot += n
	}
	sort.Slice(oi, func(i, j int) bool { return oi[i].n > oi[j].n || oi[i].n == oi[j].n && oi[i].k < oi[j].k })
	fmt.Fprintf(w, "AUDIT target owner:reason (live, all):")
	for i := 0; i < len(oi); i++ {
		fmt.Fprintf(w, " %s %.1f%% |", oi[i].k, 100*float64(oi[i].n)/float64(ot))
	}
	fmt.Fprintln(w)
	var r []kv
	tot := 0
	for k, n := range a.layer {
		r = append(r, kv{k, n})
		tot += n
	}
	sort.Slice(r, func(i, j int) bool { return r[i].n > r[j].n || r[i].n == r[j].n && r[i].k < r[j].k })
	fmt.Fprintf(w, "AUDIT in-possession off-ball target layers (refreshes, %d):\n", tot)
	for i := 0; i < len(r) && i < 16; i++ {
		fmt.Fprintf(w, "   %5.1f%% %s\n", 100*float64(r[i].n)/float64(tot), r[i].k)
	}
}

// auditPossession classifies every change of the team in possession during
// live play (engine truth, from PossessedBy) and counts ping-pongs
// (A→B→A within 6 s with B's spell under 3 s), keyed by the causes.
func (e *MatchEngine) auditPossession() {
	p := &e.audit.poss
	if p.changes == nil {
		p.changes, p.pingpong = map[string]int{}, map[string]int{}
		p.lastTeam = -1
	}
	carrier := e.findMatchPlayer(e.ball.PossessedBy)
	if carrier == nil {
		p.sinceHeld++
		if e.ball.IsPass {
			p.sawPass = true
			p.passTeam = 1
			if e.ball.PassByHome {
				p.passTeam = 0
			}
		}
		if e.ball.IsShot {
			p.sawShot = true
		}
		return
	}
	team := 1
	if carrier.IsHomeTeam {
		team = 0
	}
	now := e.getSimulatedSeconds()
	if p.lastTeam >= 0 && team != p.lastTeam {
		cause := "loose ball"
		switch {
		case p.sinceHeld == 0:
			cause = "direct steal (tackle/challenge)"
		case p.sawShot:
			cause = "after a shot"
		case p.sawPass && p.passTeam != team:
			cause = "pass intercepted/lost"
		}
		if p.sinceHeld > 0 && p.sinceHeld <= 5 && !p.sawPass && !p.sawShot {
			cause = "loose <0.5 s (knock/duel)"
		}
		// contested: an opponent within 2 m of the new carrier at the gain
		cm := toMetres(carrier.Position)
		for i := range e.players {
			q := &e.players[i]
			if active(q) && q.IsHomeTeam != carrier.IsHomeTeam {
				qm := toMetres(q.Position)
				if math.Hypot(qm.X-cm.X, qm.Y-cm.Y) < 2 {
					cause += " [contested]"
					break
				}
			}
		}
		p.changes[cause]++
		p.history = append(p.history, auditChange{now, team, cause})
		if n := len(p.history); n >= 2 {
			b, c := p.history[n-2], p.history[n-1] // B gained, then A regained
			if c.t-b.t < 3 && n >= 3 && p.history[n-3].team == c.team && c.t-p.history[n-3].t < 6 {
				p.pingpong["B gained by: "+b.cause]++
				p.pingpong["A regained by: "+c.cause]++
				p.pingpong["total"]++
			}
		}
	}
	if team != p.lastTeam {
		p.heldSince = now
	}
	p.lastTeam, p.lastHolder = team, carrier
	p.sinceHeld, p.sawPass, p.sawShot = 0, false, false
}

func (e *MatchEngine) auditPossessionReport(w io.Writer) {
	p := &e.audit.poss
	type kv struct {
		k string
		n int
	}
	dump := func(title string, m map[string]int) {
		var r []kv
		tot := 0
		for k, n := range m {
			if k == "total" {
				continue
			}
			r = append(r, kv{k, n})
			tot += n
		}
		sort.Slice(r, func(i, j int) bool { return r[i].n > r[j].n || r[i].n == r[j].n && r[i].k < r[j].k })
		fmt.Fprintf(w, "AUDIT %s (n=%d):", title, tot)
		for _, x := range r {
			fmt.Fprintf(w, " %s %d |", x.k, x.n)
		}
		fmt.Fprintln(w)
	}
	dump("possession changes by cause", p.changes)
	fmt.Fprintf(w, "AUDIT engine ping-pong (A→B→A <6 s, B <3 s): %d\n", p.pingpong["total"])
	dump("ping-pong causes", p.pingpong)
}

func (e *MatchEngine) auditVisualSteal(carrier *MatchPlayer, ball Vector2F) {
	v := &e.audit.visual
	if v.byIntent == nil {
		v.byIntent, v.byM1Role = map[string]int{}, map[string]int{}
	}
	v.heldSteps++
	cm := toMetres(carrier.Position)
	dc := math.Hypot(cm.X-ball.X, cm.Y-ball.Y)
	var nearest *MatchPlayer
	best := math.Inf(1)
	slot := -1
	for i := range e.players {
		q := &e.players[i]
		if !active(q) || q.IsHomeTeam == carrier.IsHomeTeam {
			continue
		}
		qm := toMetres(q.Position)
		if d := math.Hypot(qm.X-ball.X, qm.Y-ball.Y); d < best {
			best, nearest, slot = d, q, i
		}
	}
	on := nearest != nil && best <= 0.8 && best < dc
	if on {
		v.steps++
		if !v.on {
			v.episodes++
			v.byIntent[auditIntent(nearest.Intent)]++
			role := "P12 (M1 not owning)"
			if side := &e.feM1Of().sides[hBoolIndex(nearest.IsHomeTeam)]; side.owns && slot < len(side.tasks) {
				role = fmt.Sprintf("M1 role %d", side.tasks[slot].role)
			}
			v.byM1Role[role]++
			v.gapDef = append(v.gapDef, best)
			v.gapCar = append(v.gapCar, dc)
		}
	}
	v.on = on
}

func (e *MatchEngine) auditVisualReport(w io.Writer) {
	v := &e.audit.visual
	if v.heldSteps == 0 {
		return
	}
	med := func(x []float64) float64 {
		if len(x) == 0 {
			return math.NaN()
		}
		y := append([]float64(nil), x...)
		sort.Float64s(y)
		return y[len(y)/2]
	}
	fmt.Fprintf(w, "AUDIT visual steals (opponent nearer the ball than the carrier, <=0.8 m): %d episodes, %.1f%% of held steps | at start: defender %.2f m vs carrier %.2f m from ball (median)\n",
		v.episodes, 100*float64(v.steps)/float64(v.heldSteps), med(v.gapDef), med(v.gapCar))
	keys := func(m map[string]int) string {
		var r []string
		for k, n := range m {
			r = append(r, fmt.Sprintf("%s %d", k, n))
		}
		sort.Strings(r)
		return fmt.Sprint(r)
	}
	fmt.Fprintf(w, "AUDIT visual steals by defender intent %s | by M1 role (1 FirstDefender 2 Cover 3 Screen 4 Contain 5 ProtectDepth 6 RecoverShape 7 Marker) %s\n", keys(v.byIntent), keys(v.byM1Role))
}

// auditBuildUp: carrier (not keeper) in his own third; how close the
// defending side is, whether M1 has an engager (FirstDefender/Contain) on him,
// and how close the defending forwards (ST/W/CAM) are.
func (e *MatchEngine) auditBuildUp(carrier *MatchPlayer) {
	if carrier.IsGoalkeeper {
		return
	}
	cm := feM1ToM(carrier.Position)
	if cm.X*feM1Dir(carrier.IsHomeTeam) > -feM1HalfLength/3 {
		return
	}
	b := &e.audit.buildUp
	b.steps++
	def := !carrier.IsHomeTeam
	side := &e.feM1Of().sides[hBoolIndex(def)]
	if side.episode.mode == feM1Counterpress {
		b.modeCounterpress++
	}
	near, fwd, eng := math.Inf(1), math.Inf(1), math.Inf(1)
	for i := range e.players {
		p := &e.players[i]
		if !active(p) || p.IsHomeTeam != def || p.IsGoalkeeper {
			continue
		}
		d := feM1Dist(feM1ToM(p.Position), cm)
		near = math.Min(near, d)
		if feM1Forward(p) {
			fwd = math.Min(fwd, d)
		}
		if side.owns && i < len(side.tasks) {
			if r := side.tasks[i].role; r == feM1RoleFirstDefender || r == feM1RoleContain {
				eng = math.Min(eng, d)
			}
		}
	}
	b.nearest = append(b.nearest, near)
	b.forwardNearest = append(b.forwardNearest, fwd)
	if !math.IsInf(eng, 1) {
		b.engaged++
		b.engager = append(b.engager, eng)
	}
}

func (e *MatchEngine) auditBuildUpReport(w io.Writer) {
	b := &e.audit.buildUp
	if b.steps == 0 {
		return
	}
	med := func(x []float64) float64 {
		if len(x) == 0 {
			return math.NaN()
		}
		y := append([]float64(nil), x...)
		sort.Float64s(y)
		return y[len(y)/2]
	}
	fmt.Fprintf(w, "AUDIT build-up (carrier in own third): steps %d | nearest defender median %.1f m | nearest forward %.1f m | M1 engager assigned %.0f%% (at %.1f m) | counterpress mode %.0f%%\n",
		b.steps, med(b.nearest), med(b.forwardNearest), 100*float64(b.engaged)/float64(b.steps), med(b.engager),
		100*float64(b.modeCounterpress)/float64(b.steps))
}

func (e *MatchEngine) auditChase(carrier *MatchPlayer, ball Vector2F) {
	c := &e.audit.chase
	if c.who == nil {
		c.who = map[string]int{}
	}
	c.held++
	def := !carrier.IsHomeTeam
	dir := feM1Dir(def)
	var runners []*MatchPlayer
	for i := range e.players {
		p := &e.players[i]
		if !active(p) || p.IsGoalkeeper || p.IsHomeTeam != def {
			continue
		}
		pm := toMetres(p.Position)
		v := p.Velocity
		sp := math.Hypot(v.X, v.Y)
		dx, dy := ball.X-pm.X, ball.Y-pm.Y
		d := math.Hypot(dx, dy)
		if sp > 3 && d > 5 && (v.X*dx+v.Y*dy)/(sp*d) > 0.8 {
			runners = append(runners, p)
		}
	}
	if len(runners) < 3 {
		return
	}
	c.three++
	for _, p := range runners {
		k := p.TargetOwner.String() + ":" + p.TargetReason.String()
		// upfield of the ball (bypassed: running back toward his own goal)
		if (feM1ToM(p.Position).X-feM1ToM(carrier.Position).X)*dir > 0 {
			k += " [bypassed " + roleShortKeys[p.Player.GetRole()]
			sp := math.Hypot(p.Velocity.X, p.Velocity.Y)
			switch {
			case sp < 5:
				k += " <5m/s]"
			case sp < 7:
				k += " 5-7m/s]"
			default:
				k += " >7m/s]"
			}
		}
		c.who[k]++
	}
}

func (e *MatchEngine) auditChaseReport(w io.Writer) {
	c := &e.audit.chase
	if c.held == 0 {
		return
	}
	type kv struct {
		k string
		n int
	}
	var r []kv
	tot := 0
	for k, n := range c.who {
		r = append(r, kv{k, n})
		tot += n
	}
	sort.Slice(r, func(i, j int) bool { return r[i].n > r[j].n || r[i].n == r[j].n && r[i].k < r[j].k })
	fmt.Fprintf(w, "AUDIT chase>=3 (engine velocity): %.1f%% of held steps | runners by owner:reason:", 100*float64(c.three)/float64(c.held))
	for i := 0; i < len(r) && i < 14; i++ {
		fmt.Fprintf(w, " %s %.0f%% |", r[i].k, 100*float64(r[i].n)/float64(tot))
	}
	fmt.Fprintln(w)
}
