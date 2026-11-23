// FE M2 decision trace (ours): Diagnostics.M2Trace=1 prints one line per M2
// decision (2: plus every pass option, 3: plus every candidate rejection) to
// Diagnostics.Out for calibration. The CLI maps FMENGINE_M2TRACE onto it.
package engine

import (
	"fmt"
	"math"
)

// feM2TraceState is the per-match M2 trace (Diagnostics.M2Trace): rejection
// counters, the pending pass awaiting its outcome, and the level-3 candidate
// logger. It never feeds back into the simulation.
type feM2TraceState struct {
	rej, rejF [5]int // candidate rejections: distance, offside, lane, race, kept (rejF: >3 m ahead)
	ahead     bool   // the candidate being evaluated is >3 m ahead
	candLog   func(i int)
	pending   struct { // the last M2 pass, resolved at the next M2 decision
		on       bool
		home     bool
		receiver *MatchPlayer
		t, comp  float64
		dist     float64
		lofted   bool
	}
}

// feM2Count bumps a rejection counter (trace only).
func (e *MatchEngine) feM2Count(i int) {
	t := &e.m2trace
	if t.candLog != nil {
		t.candLog(i)
	}
	if e.diag.M2Trace > 0 {
		t.rej[i]++
		if t.ahead {
			t.rejF[i]++
		}
	}
}

func (e *MatchEngine) feM2TraceResolve(carrier *MatchPlayer) {
	if e.diag.M2Trace == 0 || !e.m2trace.pending.on {
		return
	}
	p := &e.m2trace.pending
	p.on = false
	who := "opp"
	if carrier.IsHomeTeam == p.home {
		who = "mate"
		if carrier == p.receiver {
			who = "recv"
		}
	}
	fmt.Fprintf(e.diag.out(), "PASSOUT comp=%.3f dist=%.1f lofted=%v who=%s dt=%.2f\n", p.comp, p.dist, p.lofted, who, e.getSimulatedSeconds()-p.t)
}

func (e *MatchEngine) feM2Trace(carrier *MatchPlayer, choice *feM2Option, options []feM2Option, keep float64, pressure int) {
	if e.diag.M2Trace == 0 {
		return
	}
	t := &e.m2trace
	defer func() { t.rej = [5]int{}; t.rejF = [5]int{} }()
	here := feM1ToM(carrier.Position)
	tx := here.X * feM1Dir(carrier.IsHomeTeam)
	best := [5]float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()}
	npass := 0
	for i := range options {
		k := options[i].kind
		if k == feM2KindPass {
			npass++
		}
		if math.IsNaN(best[k]) || options[i].value > best[k] {
			best[k] = options[i].value
		}
	}
	prog, comp, dist := 0.0, 0.0, 0.0
	if choice.kind == feM2KindPass {
		t.pending.on, t.pending.home, t.pending.receiver = true, carrier.IsHomeTeam, choice.pass.Receiver
		t.pending.t, t.pending.comp, t.pending.lofted = e.getSimulatedSeconds(), choice.pass.CompletionProbability, choice.pass.Lofted
		to := feM1ToM(choice.pass.TargetPoint)
		prog = (to.X - here.X) * feM1Dir(carrier.IsHomeTeam)
		comp = choice.pass.CompletionProbability
		dist = feM1Dist(here, to)
		t.pending.dist = dist
	}
	role := PlayerRole_CM
	if carrier.Player != nil {
		role = carrier.Player.GetRole()
	}
	if held := e.getSimulatedSeconds() - e.possessionStartSeconds; held > 6.0 {
		fmt.Fprintf(e.diag.out(), "HOLD gk=%v held=%.1f kind=%d x=%.1f y=%.1f state=%d\n", carrier.IsGoalkeeper, held, choice.kind, here.X, here.Y, e.state)
	}
	fmt.Fprintf(e.diag.out(), "M2 t=%.1f home=%v role=%d tx=%.1f kind=%d val=%.4f npass=%d pass=%.4f shot=%.4f carry=%.4f clear=%.4f shield=%.4f keep=%.2f press=%d prog=%.1f comp=%.2f dist=%.1f rej=%v rejf=%v near=%.1f\n",
		e.getSimulatedSeconds(), carrier.IsHomeTeam, role, tx, choice.kind, choice.value, npass,
		best[feM2KindPass], best[feM2KindShot], best[feM2KindCarry], best[feM2KindClear], best[feM2KindShield],
		keep, pressure, prog, comp, dist, t.rej, t.rejF, e.nearestOpponentDistance(carrier))
	if e.diag.M2Trace == 2 {
		for i := range options {
			if options[i].kind != feM2KindPass {
				continue
			}
			o := &options[i].pass
			to := feM1ToM(o.TargetPoint)
			fmt.Fprintf(e.diag.out(), "   opt prog=%.1f d=%.1f comp=%.2f val=%.4f lofted=%v recvNear=%.1f\n",
				(to.X-here.X)*feM1Dir(carrier.IsHomeTeam), feM1Dist(here, to), o.CompletionProbability,
				options[i].value, o.Lofted, e.feM2NearestOpponent(carrier.IsHomeTeam, to))
		}
	}
}
