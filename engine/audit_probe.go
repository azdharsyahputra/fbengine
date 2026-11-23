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
	"strings"
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
	// Carry spells (same carrier continuously): long ones (>=25 m) and how
	// they were defended.
	carry auditCarryState
	// player-steps above 4 m/s by side and owner:reason, and all player-steps
	fast    map[string]int
	fastAll int
	// fouls by kind (ball won / missed, reckless, sanction, offender job)
	fouls map[string]int
	// offside: attacker-steps standing offside by owner:reason; offside passes by kind
	offStand map[string]int
	offSteps int
	offPass map[string]int
	// near-ball marking: attackers within 15 m of the ball; nearest defender <3 m by owner:reason
	nearMark map[string]int
	// shots by source, distance, xG and previous shot gap
	shots    map[string]int
	lastShot float64
	nearAtt  int
	// sprint starts (crossing 4 m/s) by job, and target jumps > 4 m between steps
	sprintStart                    map[string]int
	sprints, targetJumps, jobJumps int
	wasFast                        []bool
	lastTarget                     []Vector2F
	lastReason                     []string
	// Back line while defending: CB pair <2 m and both full-backs on one
	// half of the width, with the owner:reason of the players involved.
	// Attacking full-backs (own side holds, ball in the final third): owner:reason
	// and depth behind the ball.
	fbAtt struct {
		why   map[string]int
		n     int
		depth []float64
		lag   map[string][]float64 // per reason: target behind ball, position behind target
	}
	back struct {
		steps, stack, same int
		stackWhy, sameWhy  map[string]int
	}
	// Pivot (CDM/CM) of the side in possession: reason and depth behind the ball.
	pivot map[string][]float64
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
	// fast movement (>4 m/s) by side and owner:reason: who sets the tempo
	if a.fast == nil {
		a.fast = map[string]int{}
		a.sprintStart = map[string]int{}
		a.wasFast = make([]bool, len(e.players))
		a.lastTarget = make([]Vector2F, len(e.players))
		a.lastReason = make([]string, len(e.players))
	}
	e.auditOffsideStep()
	for i := range e.players {
		p := &e.players[i]
		if !active(p) || p.IsGoalkeeper {
			continue
		}
		k := p.TargetOwner.String() + ":" + p.TargetReason.String()
		jump := math.Hypot((p.TacticalTarget.X-a.lastTarget[i].X)*Pitch_LENGTH_METRES, (p.TacticalTarget.Y-a.lastTarget[i].Y)*Pitch_WIDTH_METRES)
		if jump > 4 && a.lastReason[i] != "" {
			a.targetJumps++
			if k != a.lastReason[i] {
				a.jobJumps++
			}
		}
		a.lastTarget[i], a.lastReason[i] = p.TacticalTarget, k
		fast := math.Hypot(p.Velocity.X, p.Velocity.Y) > 4
		if fast && !a.wasFast[i] {
			a.sprintStart[k]++
			a.sprints++
		}
		a.wasFast[i] = fast
	}
	for i := range e.players {
		p := &e.players[i]
		if !active(p) || p.IsGoalkeeper {
			continue
		}
		a.fastAll++
		if math.Hypot(p.Velocity.X, p.Velocity.Y) <= 4 {
			continue
		}
		side := "def"
		if e.lastControlledTeamHome.HasValue && e.lastControlledTeamHome.Value == p.IsHomeTeam {
			side = "att"
		}
		k := side + " " + p.TargetOwner.String() + ":" + p.TargetReason.String()
		if p.TargetRefiner != TargetOwner_NONE {
			k += "+" + p.TargetRefiner.String()
		}
		a.fast[k]++
	}
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
		e.auditCarry(carrier)
		e.auditPivot(carrier)
		e.auditBuildUp(carrier)
		e.auditChase(carrier, ball)
		e.auditBackLine(carrier)
		e.auditFullBackAttack(carrier)
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
			a.addLayer("LOOSE-RUNNER " + side + " " + p.TargetOwner.String() + ":" + p.TargetReason.String())
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
	e.auditPivotReport(w)
	e.auditCarryReport(w)
	e.auditFoulReport(w)
	e.auditOffsideReport(w)
	e.auditNearMarkReport(w)
	e.auditShotReport(w)
	if len(e.audit.fast) > 0 {
		type kv struct {
			k string
			v int
		}
		var xs []kv
		tot := 0
		for k, v := range e.audit.fast {
			xs = append(xs, kv{k, v})
			tot += v
		}
		sort.Slice(xs, func(i, j int) bool { return xs[i].v > xs[j].v || xs[i].v == xs[j].v && xs[i].k < xs[j].k })
		fmt.Fprintf(w, "AUDIT fast (>4 m/s): %.1f%% of outfield player-steps |", 100*float64(tot)/float64(max(e.audit.fastAll, 1)))
		for i, x := range xs {
			if i < 16 {
				fmt.Fprintf(w, " %s %.1f%% |", x.k, 100*float64(x.v)/float64(e.audit.fastAll))
			}
		}
		fmt.Fprintln(w)
		hours := float64(e.audit.fastAll) * e.stepSeconds() / 3600
		fmt.Fprintf(w, "AUDIT sprint starts (>4 m/s): %.0f per player-hour | target jumps >4 m %.0f per player-hour (job changed in %.0f%%) | starts by job:",
			float64(e.audit.sprints)/hours, float64(e.audit.targetJumps)/hours, 100*float64(e.audit.jobJumps)/math.Max(1, float64(e.audit.targetJumps)))
		type kv2 struct {
			k string
			v int
		}
		var ys []kv2
		for k, v := range e.audit.sprintStart {
			ys = append(ys, kv2{k, v})
		}
		sort.Slice(ys, func(i, j int) bool { return ys[i].v > ys[j].v || ys[i].v == ys[j].v && ys[i].k < ys[j].k })
		for i, y := range ys {
			if i < 14 {
				fmt.Fprintf(w, " %s %.0f%% |", y.k, 100*float64(y.v)/math.Max(1, float64(e.audit.sprints)))
			}
		}
		fmt.Fprintln(w)
	}
	e.auditBackReport(w)
	e.auditFullBackReport(w)
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

func (e *MatchEngine) auditPivot(carrier *MatchPlayer) {
	a := &e.audit
	if a.pivot == nil {
		a.pivot = map[string][]float64{}
	}
	dir := feM1Dir(carrier.IsHomeTeam)
	bx := feM1ToM(carrier.Position).X * dir
	zone := "final"
	switch {
	case bx < -feM1HalfLength/3:
		zone = "own"
	case bx < feM1HalfLength/3:
		zone = "mid"
	}
	for i := range e.players {
		p := &e.players[i]
		if !active(p) || p == carrier || p.IsHomeTeam != carrier.IsHomeTeam || p.Player == nil {
			continue
		}
		if r := p.Player.GetRole(); r != PlayerRole_CDM && r != PlayerRole_CM {
			continue
		}
		k := zone + " " + p.TargetOwner.String() + ":" + p.TargetReason.String()
		if p.TargetRefiner != TargetOwner_NONE {
			k += "+" + p.TargetRefiner.String()
		}
		a.pivot[k] = append(a.pivot[k], feM1ToM(p.Position).X*dir-bx)
	}
}

func (e *MatchEngine) auditPivotReport(w io.Writer) {
	a := &e.audit
	if len(a.pivot) == 0 {
		return
	}
	type kv struct {
		k   string
		n   int
		med float64
	}
	var r []kv
	tot := map[string]int{}
	for k, v := range a.pivot {
		s := append([]float64(nil), v...)
		sort.Float64s(s)
		r = append(r, kv{k, len(v), s[len(s)/2]})
		tot[strings.Fields(k)[0]] += len(v)
	}
	sort.Slice(r, func(i, j int) bool { return r[i].n > r[j].n || r[i].n == r[j].n && r[i].k < r[j].k })
	fmt.Fprintf(w, "AUDIT pivot (CDM/CM, own side in possession): zone owner:reason share / median metres from ball:")
	for _, x := range r {
		z := strings.Fields(x.k)[0]
		if 100*x.n/tot[z] < 3 {
			continue
		}
		fmt.Fprintf(w, " %s %d%% %.0fm |", x.k, 100*x.n/tot[z], x.med)
	}
	fmt.Fprintln(w)
}

// auditCarry tracks one carrier's spell: path length, nearest defender, and
// whether M1 assigned an engager (FirstDefender/Contain) or M6 prepared a
// challenge on him during it.
func (e *MatchEngine) auditCarry(carrier *MatchPlayer) {
	c := &e.audit.carry
	if c.longZone == nil {
		c.longZone, c.longRole = map[string]int{}, map[string]int{}
	}
	pos := feM1ToM(carrier.Position)
	if c.who != carrier {
		e.auditCarryEnd()
		dir := feM1Dir(carrier.IsHomeTeam)
		z := "final"
		switch x := pos.X * dir; {
		case x < -feM1HalfLength/3:
			z = "own"
		case x < feM1HalfLength/3:
			z = "mid"
		}
		c.who, c.path, c.engaged, c.challenged = nil, 0, false, false
		c.steps, c.space, c.decisions, c.carries, c.margin, c.noPass = 0, 0, 0, 0, 0, 0
		c.eng, c.engFar = [3]int{}, c.engFar[:0]
		c.who, c.start, c.last, c.minNear, c.startZone = carrier, pos, pos, math.Inf(1), z
	}
	c.path += feM1Dist(c.last, pos)
	c.last = pos
	def := !carrier.IsHomeTeam
	side := &e.feM1Of().sides[hBoolIndex(def)]
	m6 := e.feM6()
	near, engD := math.Inf(1), math.Inf(1)
	for i := range e.players {
		q := &e.players[i]
		if !active(q) || q.IsHomeTeam != def || q.IsGoalkeeper {
			continue
		}
		near = math.Min(near, feM1Dist(feM1ToM(q.Position), pos))
		if side.owns && i < len(side.tasks) {
			if r := side.tasks[i].role; r == feM1RoleFirstDefender || r == feM1RoleContain {
				c.engaged = true
				engD = math.Min(engD, feM1Dist(feM1ToM(q.Position), pos))
			}
		}
		if i < len(m6.challenges) && m6.challenges[i].active {
			c.challenged = true
		}
	}
	c.minNear = math.Min(c.minNear, near)
	switch {
	case engD <= 5:
		c.eng[0]++
	case !math.IsInf(engD, 1):
		c.eng[1]++
		c.engFar = append(c.engFar, engD)
	default:
		c.eng[2]++
	}
	c.steps++
	if near > 5 {
		c.space++
	}
}

// auditCarryState: carry spells (same carrier continuously), long ones
// (>=25 m displacement) and how they were defended and decided.
type auditCarryState struct {
	who                    *MatchPlayer
	start, last            feM1Vec
	path, minNear          float64
	engaged, challenged    bool
	startZone              string
	steps, space           int     // spell steps / steps with no defender within 5 m
	decisions, carries     int     // M2 decisions in the spell / carry choices
	margin                 float64 // sum of (carry value - best pass value) on carry choices
	noPass                 int     // carry choices with no pass option at all
	eng                    [3]int  // spell steps: engager within 5 m / engager farther / no engager
	engFar                 []float64
	long, total            int
	longEngaged, longChall int
	longNear, longSpace    []float64
	longDecisions          []float64
	longMargin             []float64
	longNoPass, longCarry  int
	longEng                [3]int
	longEngFar             []float64
	longZone, longRole     map[string]int
}

// auditCarryDecision records one M2 choice of the current carrier.
func (e *MatchEngine) auditCarryDecision(carrier *MatchPlayer, carry bool, carryValue, passValue float64) {
	if !e.diag.Audit {
		return
	}
	c := &e.audit.carry
	if c.who != carrier {
		return
	}
	c.decisions++
	if carry {
		c.carries++
		if math.IsInf(passValue, -1) {
			c.noPass++
		} else {
			c.margin += carryValue - passValue
		}
	}
}

func (e *MatchEngine) auditCarryEnd() {
	c := &e.audit.carry
	if c.who == nil {
		return
	}
	c.total++
	if feM1Dist(c.start, c.last) >= 25 {
		c.long++
		if c.engaged {
			c.longEngaged++
		}
		if c.challenged {
			c.longChall++
		}
		c.longNear = append(c.longNear, c.minNear)
		c.longSpace = append(c.longSpace, float64(c.space)/math.Max(1, float64(c.steps)))
		c.longDecisions = append(c.longDecisions, float64(c.decisions))
		if c.carries > c.noPass {
			c.longMargin = append(c.longMargin, c.margin/float64(c.carries-c.noPass))
		}
		c.longNoPass += c.noPass
		for k := range c.eng {
			c.longEng[k] += c.eng[k]
		}
		c.longEngFar = append(c.longEngFar, c.engFar...)
		c.longCarry += c.carries
		c.longZone[c.startZone]++
		if c.who.Player != nil {
			c.longRole[roleShortKeys[c.who.Player.GetRole()]]++
		}
	}
}

func (e *MatchEngine) auditCarryReport(w io.Writer) {
	e.auditCarryEnd()
	c := &e.audit.carry
	if c.total == 0 {
		return
	}
	near := append([]float64(nil), c.longNear...)
	sort.Float64s(near)
	m := math.NaN()
	if len(near) > 0 {
		m = near[len(near)/2]
	}
	med := func(v []float64) float64 {
		if len(v) == 0 {
			return math.NaN()
		}
		v = append([]float64(nil), v...)
		sort.Float64s(v)
		return v[len(v)/2]
	}
	fmt.Fprintf(w, "AUDIT carries >=25 m (displacement): %d of %d spells | M1 engager assigned %d | M6 challenge prepared %d | nearest defender during it median %.1f m | start zone %v | role %v\n",
		c.long, c.total, c.longEngaged, c.longChall, m, c.longZone, c.longRole)
	fmt.Fprintf(w, "AUDIT long-carry decisions: share of spell with no defender within 5 m median %.2f | M2 decisions per spell median %.0f | carry choices %d (no pass option %d) | carry-minus-best-pass margin median %.5f\n",
		med(c.longSpace), med(c.longDecisions), c.longCarry, c.longNoPass, med(c.longMargin))
	n := float64(c.longEng[0] + c.longEng[1] + c.longEng[2])
	fmt.Fprintf(w, "AUDIT long-carry engagement (steps): M1 engager within 5 m %.0f%% | engager farther %.0f%% (median %.1f m) | no engager %.0f%%\n",
		100*float64(c.longEng[0])/n, 100*float64(c.longEng[1])/n, med(c.longEngFar), 100*float64(c.longEng[2])/n)
}

// auditBackLine: the defending side's two centre-backs and two full-backs.
func (e *MatchEngine) auditBackLine(carrier *MatchPlayer) {
	b := &e.audit.back
	if b.stackWhy == nil {
		b.stackWhy, b.sameWhy = map[string]int{}, map[string]int{}
	}
	var cb, fb []*MatchPlayer
	for i := range e.players {
		q := &e.players[i]
		if !active(q) || q.IsHomeTeam == carrier.IsHomeTeam || q.Player == nil {
			continue
		}
		switch roleShortKeys[q.Player.GetRole()] {
		case "ROLE_SHORT_CB":
			cb = append(cb, q)
		case "ROLE_SHORT_LB", "ROLE_SHORT_RB", "ROLE_SHORT_LWB", "ROLE_SHORT_RWB":
			fb = append(fb, q)
		}
	}
	if len(cb) != 2 || len(fb) != 2 {
		return
	}
	why := func(q *MatchPlayer) string { return q.TargetOwner.String() + ":" + q.TargetReason.String() }
	pair := func(a, c string) string {
		if a > c {
			a, c = c, a
		}
		return a + " & " + c
	}
	b.steps++
	if feM1Dist(feM1ToM(cb[0].Position), feM1ToM(cb[1].Position)) < 2 {
		b.stack++
		b.stackWhy[pair(why(cb[0]), why(cb[1]))]++
	}
	y0, y1 := feM1ToM(fb[0].Position).Y, feM1ToM(fb[1].Position).Y
	if y0*y1 > 0 {
		b.same++
		// the crossed one: nearer the centre line; zone of the ball
		q := fb[0]
		if math.Abs(y1) < math.Abs(y0) {
			q = fb[1]
		}
		u := (feM1ToM(e.ball.Position).X*feM1Dir(q.IsHomeTeam) + feM1HalfLength)
		z := "final"
		if u < 35 {
			z = "own"
		} else if u < 70 {
			z = "mid"
		}
		b.sameWhy[z+" crossed "+why(q)]++
	}
}

func (e *MatchEngine) auditBackReport(w io.Writer) {
	b := &e.audit.back
	if b.steps == 0 {
		return
	}
	top := func(m map[string]int, n int) string {
		type kv struct {
			k string
			v int
		}
		var xs []kv
		for k, v := range m {
			xs = append(xs, kv{k, v})
		}
		sort.Slice(xs, func(i, j int) bool { return xs[i].v > xs[j].v || xs[i].v == xs[j].v && xs[i].k < xs[j].k })
		out := ""
		for i, x := range xs {
			if i >= 6 {
				break
			}
			out += fmt.Sprintf(" %s %d%% |", x.k, 100*x.v/n)
		}
		return out
	}
	fmt.Fprintf(w, "AUDIT back line (ball held by the opponent): CBs <2 m %.1f%% |%s\n", 100*float64(b.stack)/float64(b.steps), top(b.stackWhy, max(b.stack, 1)))
	fmt.Fprintf(w, "AUDIT back line: both FBs on one half of the width %.1f%% |%s\n", 100*float64(b.same)/float64(b.steps), top(b.sameWhy, max(b.same, 1)))
}

func (e *MatchEngine) auditFullBackAttack(carrier *MatchPlayer) {
	f := &e.audit.fbAtt
	if f.why == nil {
		f.why = map[string]int{}
	}
	dir := feM1Dir(carrier.IsHomeTeam)
	ball := feM1ToM(e.ball.Position).X * dir
	zone := "F "
	if ball < feM1HalfLength/3 {
		if ball < -feM1HalfLength/3 {
			return
		}
		zone = "M "
	}
	for i := range e.players {
		q := &e.players[i]
		if !active(q) || q.IsHomeTeam != carrier.IsHomeTeam || q.Player == nil || q == carrier {
			continue
		}
		rk := roleShortKeys[q.Player.GetRole()]
		if rk != "ROLE_SHORT_LB" && rk != "ROLE_SHORT_RB" && rk != "ROLE_SHORT_CB" {
			continue
		}
		cbTag := ""
		if rk == "ROLE_SHORT_CB" {
			cbTag = "CB "
		}
		f.n++
		k := q.TargetOwner.String() + ":" + q.TargetReason.String()
		if q.TargetRefiner != TargetOwner_NONE {
			k += "+" + q.TargetRefiner.String()
		}
		bs := " far"
		if feM1ToM(q.BasePosition).Y*feM1ToM(carrier.Position).Y > 0 {
			bs = " ball"
		}
		k = zone + cbTag + k + bs
		f.why[k]++
		f.depth = append(f.depth, ball-feM1ToM(q.Position).X*dir)
		if f.lag == nil {
			f.lag = map[string][]float64{}
		}
		t := feM1ToM(q.TacticalTarget).X * dir
		f.lag[k+" tgtBehindBall"] = append(f.lag[k+" tgtBehindBall"], ball-t)
		f.lag[k+" posBehindTgt"] = append(f.lag[k+" posBehindTgt"], t-feM1ToM(q.Position).X*dir)
		f.lag[k+" fwdSpeed"] = append(f.lag[k+" fwdSpeed"], q.Velocity.X*dir)
		f.lag[k+" moveBehindTgt"] = append(f.lag[k+" moveBehindTgt"], t-feM1ToM(q.MovementTarget).X*dir)
	}
}

func (e *MatchEngine) auditFullBackReport(w io.Writer) {
	f := &e.audit.fbAtt
	if f.n == 0 {
		return
	}
	d := append([]float64(nil), f.depth...)
	sort.Float64s(d)
	type kv struct {
		k string
		v int
	}
	var xs []kv
	for k, v := range f.why {
		xs = append(xs, kv{k, v})
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i].v > xs[j].v || xs[i].v == xs[j].v && xs[i].k < xs[j].k })
	fmt.Fprintf(w, "AUDIT attacking full-backs (F final / M middle third, ball/far flank): behind ball median %.0f m |", d[len(d)/2])
	for i, x := range xs {
		if i >= 14 {
			break
		}
		fmt.Fprintf(w, " %s %d%% |", x.k, 100*x.v/f.n)
	}
	fmt.Fprintln(w)
	var ks []string
	for k, v := range f.lag {
		if len(v) > f.n/20 {
			ks = append(ks, k)
		}
	}
	sort.Strings(ks)
	fmt.Fprint(w, "AUDIT attacking full-backs medians (m):")
	for _, k := range ks {
		v := append([]float64(nil), f.lag[k]...)
		sort.Float64s(v)
		fmt.Fprintf(w, " %s %.0f |", k, v[len(v)/2])
	}
	fmt.Fprintln(w)
}

// auditFoul records one foul (audit only).
func (e *MatchEngine) auditFoul(offender *MatchPlayer, ballWon, reckless bool, sanction FoulSanction, dogso, tactical bool) {
	if !e.diag.Audit {
		return
	}
	a := &e.audit
	if a.fouls == nil {
		a.fouls = map[string]int{}
	}
	kind := "missed"
	if ballWon {
		kind = "ballwon"
	}
	if reckless {
		kind += "+reckless"
	}
	a.fouls["kind "+kind]++
	a.fouls[fmt.Sprintf("sanction %d", sanction)]++
	if dogso {
		a.fouls["dogso"]++
	}
	if tactical {
		a.fouls["tactical"]++
	}
	a.fouls["job "+offender.TargetOwner.String()+":"+offender.TargetReason.String()]++
	a.fouls["speed "+fmt.Sprintf("%.0f", math.Min(math.Hypot(offender.Velocity.X*Pitch_LENGTH_METRES, offender.Velocity.Y*Pitch_WIDTH_METRES), 8))]++
	a.fouls["all"]++
}

func (e *MatchEngine) auditFoulReport(w io.Writer) {
	if len(e.audit.fouls) == 0 {
		return
	}
	var ks []string
	for k := range e.audit.fouls {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	fmt.Fprintf(w, "AUDIT fouls: tackle attempts %d/%d won %d/%d |", e.stats.HomeTackleAttempts, e.stats.AwayTackleAttempts, e.stats.HomeTackles, e.stats.AwayTackles)
	for _, k := range ks {
		fmt.Fprintf(w, " %s %d |", k, e.audit.fouls[k])
	}
	fmt.Fprintln(w)
}

// auditChallenge records one resolved M6 challenge (audit only).
func (e *MatchEngine) auditChallenge(defender *MatchPlayer, sliding bool, outcome string) {
	if !e.diag.Audit {
		return
	}
	a := &e.audit
	if a.fouls == nil {
		a.fouls = map[string]int{}
	}
	k := "standing"
	if sliding {
		k = "sliding"
	}
	a.fouls["chal "+outcome]++
	a.fouls["chal "+k]++
	a.fouls["chal "+k+" "+outcome]++
	a.fouls[fmt.Sprintf("chal intent %d", defender.Intent)]++
	a.fouls["chal all"]++
}

// auditOffsideStep counts attackers standing offside (live play, audit only).
func (e *MatchEngine) auditOffsideStep() {
	a := &e.audit
	if a.offStand == nil {
		a.offStand = map[string]int{}
		a.offPass = map[string]int{}
	}
	if e.ball.PossessedBy == nil {
		return
	}
	holder := e.findMatchPlayer(e.ball.PossessedBy)
	if holder == nil {
		return
	}
	a.offSteps++
	e.auditNearMark(holder)
	line := e.offsideLine(holder.IsHomeTeam)
	for i := range e.players {
		p := &e.players[i]
		if !active(p) || p.IsGoalkeeper || p.IsHomeTeam != holder.IsHomeTeam || p == holder {
			continue
		}
		if e.isOffside(p, p.IsHomeTeam, line, 0.0) {
			a.offStand[p.TargetOwner.String()+":"+p.TargetReason.String()]++
			a.offStand["all"]++
		}
	}
}

// auditOffsidePass classifies a pass flagged offside at the kick (audit only).
func (e *MatchEngine) auditOffsidePass(receiver, passer *MatchPlayer, beyond bool, line float64) {
	if !e.diag.Audit {
		return
	}
	a := &e.audit
	if a.offPass == nil {
		a.offPass = map[string]int{}
	}
	sign := -1.0
	if passer.IsHomeTeam {
		sign = 1.0
	}
	gap := (receiver.Position.X - line) * sign * Pitch_LENGTH_METRES
	kind := "timing"
	if beyond {
		kind = "beyond"
	}
	a.offPass["kind "+kind]++
	a.offPass[fmt.Sprintf("gap %.0f", math.Max(-2, math.Min(gap, 6)))]++
	a.offPass["job "+receiver.TargetOwner.String()+":"+receiver.TargetReason.String()]++
	a.offPass["all"]++
}

func (e *MatchEngine) auditOffsideReport(w io.Writer) {
	a := &e.audit
	if a.offSteps == 0 {
		return
	}
	var ks []string
	for k := range a.offStand {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return a.offStand[ks[i]] > a.offStand[ks[j]] })
	fmt.Fprintf(w, "AUDIT offside standing: per held step |")
	for i, k := range ks {
		if i < 10 {
			fmt.Fprintf(w, " %s %.3f |", k, float64(a.offStand[k])/float64(a.offSteps))
		}
	}
	fmt.Fprintln(w)
	ks = ks[:0]
	for k := range a.offPass {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	fmt.Fprintf(w, "AUDIT offside passes:")
	for _, k := range ks {
		fmt.Fprintf(w, " %s %d |", k, a.offPass[k])
	}
	fmt.Fprintln(w)
}

// auditNearMark: attackers (not the holder) within 15 m of the ball and the
// job of their nearest defender when he is within 3 m (audit only).
func (e *MatchEngine) auditNearMark(holder *MatchPlayer) {
	a := &e.audit
	if a.nearMark == nil {
		a.nearMark = map[string]int{}
	}
	pm := func(p *MatchPlayer) (float64, float64) {
		return p.Position.X * Pitch_LENGTH_METRES, p.Position.Y * Pitch_WIDTH_METRES
	}
	bx, by := e.ball.Position.X*Pitch_LENGTH_METRES, e.ball.Position.Y*Pitch_WIDTH_METRES
	for i := range e.players {
		o := &e.players[i]
		if !active(o) || o.IsGoalkeeper || o.IsHomeTeam != holder.IsHomeTeam || o == holder {
			continue
		}
		ox, oy := pm(o)
		if math.Hypot(ox-bx, oy-by) >= 15 {
			continue
		}
		a.nearAtt++
		var nd *MatchPlayer
		dd := 3.0
		for j := range e.players {
			d := &e.players[j]
			if !active(d) || d.IsGoalkeeper || d.IsHomeTeam == holder.IsHomeTeam {
				continue
			}
			dx, dy := pm(d)
			if h := math.Hypot(dx-ox, dy-oy); h < dd {
				nd, dd = d, h
			}
		}
		if nd != nil {
			a.nearMark[nd.TargetOwner.String()+":"+nd.TargetReason.String()]++
			a.nearMark["all"]++
		}
	}
}

func (e *MatchEngine) auditNearMarkReport(w io.Writer) {
	a := &e.audit
	if a.nearAtt == 0 {
		return
	}
	var ks []string
	for k := range a.nearMark {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return a.nearMark[ks[i]] > a.nearMark[ks[j]] })
	fmt.Fprintf(w, "AUDIT near-ball marking (<3 m, attackers within 15 m): n %d |", a.nearAtt)
	for i, k := range ks {
		if i < 10 {
			fmt.Fprintf(w, " %s %.1f%% |", k, 100*float64(a.nearMark[k])/float64(a.nearAtt))
		}
	}
	fmt.Fprintln(w)
}

// auditShot records one shot (audit only).
func (e *MatchEngine) auditShot(shooter *MatchPlayer, xg float64, header, setPiece, penalty, freeKick bool) {
	if !e.diag.Audit {
		return
	}
	a := &e.audit
	if a.shots == nil {
		a.shots = map[string]int{}
	}
	src := "open"
	switch {
	case penalty:
		src = "penalty"
	case freeKick:
		src = "freekick"
	case header:
		src = "header"
	}
	if setPiece {
		src += "+setpiece"
	}
	gx := Pitch_LENGTH_METRES
	if !shooter.IsHomeTeam {
		gx = 0
	}
	d := math.Hypot(shooter.Position.X*Pitch_LENGTH_METRES-gx, (shooter.Position.Y-0.5)*Pitch_WIDTH_METRES)
	db := "d<11"
	switch {
	case d >= 25:
		db = "d25+"
	case d >= 18:
		db = "d18-25"
	case d >= 11:
		db = "d11-18"
	}
	xb := "xg<.03"
	switch {
	case xg >= 0.3:
		xb = "xg.30+"
	case xg >= 0.1:
		xb = "xg.10-.30"
	case xg >= 0.05:
		xb = "xg.05-.10"
	case xg >= 0.03:
		xb = "xg.03-.05"
	}
	now := e.getSimulatedSeconds()
	if now-a.lastShot < 10 {
		a.shots["within10s"]++
	}
	a.lastShot = now
	a.shots["src "+src]++
	a.shots[db]++
	a.shots[xb]++
	a.shots["all"]++
	a.shots[fmt.Sprintf("xgsum1000")] += int(xg * 1000)
}

func (e *MatchEngine) auditShotReport(w io.Writer) {
	a := &e.audit
	if len(a.shots) == 0 {
		return
	}
	var ks []string
	for k := range a.shots {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	fmt.Fprintf(w, "AUDIT shots:")
	for _, k := range ks {
		fmt.Fprintf(w, " %s %d |", k, a.shots[k])
	}
	fmt.Fprintln(w)
}
