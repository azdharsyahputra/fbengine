// Position Composer (ours, docs/PLAN-team-shape.md §4.2): turns the Team Shape Controller's
// slots plus the behaviour modules' requests into one target and speed class per player.
// Behaviour modules (M1 engagements, runners, overlaps, presses, M5 duties, support) do not
// write targets; they send requests, and the composer arbitrates by fixed rules:
//  1. ball jobs (carrier, receiver, loose-ball claimer, first defender/contain, active
//     presser) take their target as asked;
//  2. quota jobs (run in behind, overlap, box run, extra presser) are granted per side up to
//     their quota, by priority, the rest hold their slot;
//  3. offset jobs (support, cover, marker, duty) are clamped to MaxDeviation from the slot;
//  4. everyone else holds his slot;
//  5. a final spacing pass keeps non-ball-job teammates MinSpacing apart.
//
// F1: built and unit-tested, not wired (nothing calls it in a match yet).
package engine

import (
	"math"
	"sort"
)

// composeClass is the arbitration tier of a request.
type composeClass uint8

const (
	composeBall   composeClass = iota // tier 1
	composeQuota                      // tier 2
	composeOffset                     // tier 3
)

// composeQuotaKind names the quota a tier-2 request counts against.
type composeQuotaKind uint8

const (
	composeRunInBehind composeQuotaKind = iota
	composeOverlap
	composeBoxRun
	composeExtraPress
	composeQuotaKinds
)

// composeSpeed is the movement class the composer hands to M3 (executor only).
type composeSpeed uint8

const (
	composeWalk composeSpeed = iota
	composeJog
	composeRun
	composeSprint
)

// composeRequest is one behaviour module's wish for one player.
type composeRequest struct {
	Player   int // index in e.players
	Class    composeClass
	Quota    composeQuotaKind // tier 2 only
	Target   [2]float64       // pitch metres, corner origin
	Priority float64          // higher first (tier 2 grants, tie: lower Player)
	MaxDev   float64          // tier 3: metres allowed from the slot
	Speed    composeSpeed
	Owner    TargetOwner
	Reason   TargetReason
}

// composeConfig holds the composer's rules (parameters swept with the scorecard, F3).
type composeConfig struct {
	Quota      [composeQuotaKinds]int // per side
	MinSpacing float64                // metres between non-ball-job teammates
	HoldSpeed  composeSpeed           // speed class of a player holding his slot
}

func defaultComposeConfig() composeConfig {
	return composeConfig{Quota: [composeQuotaKinds]int{2, 1, 2, 1}, MinSpacing: 8, HoldSpeed: composeJog}
}

// composeResult is the composer's decision for one player.
type composeResult struct {
	Target   [2]float64
	Speed    composeSpeed
	Class    int // -1 slot, else the granted request's class
	Owner    TargetOwner
	Reason   TargetReason
	Deviated float64 // metres from the slot
}

// compose arbitrates. slots/ok come from teamShapeSlots; home[i] tells each player's side.
// Deterministic: requests are processed in a fixed order whatever their input order.
func compose(cfg composeConfig, slots [][2]float64, ok []bool, home []bool, reqs []composeRequest) []composeResult {
	n := len(slots)
	out := make([]composeResult, n)
	for i := range out {
		out[i] = composeResult{Target: slots[i], Speed: cfg.HoldSpeed, Class: -1}
	}
	rs := append([]composeRequest(nil), reqs...)
	sort.SliceStable(rs, func(a, b int) bool {
		if rs[a].Class != rs[b].Class {
			return rs[a].Class < rs[b].Class
		}
		if rs[a].Priority != rs[b].Priority {
			return rs[a].Priority > rs[b].Priority
		}
		return rs[a].Player < rs[b].Player
	})
	taken := make([]bool, n)
	var used [2][composeQuotaKinds]int
	for _, r := range rs {
		i := r.Player
		if i < 0 || i >= n || taken[i] {
			continue
		}
		switch r.Class {
		case composeBall:
			out[i] = composeResult{Target: r.Target, Speed: r.Speed, Class: int(r.Class), Owner: r.Owner, Reason: r.Reason}
		case composeQuota:
			s := hBoolIndex(!home[i])
			if used[s][r.Quota] >= cfg.Quota[r.Quota] {
				continue // over quota: holds his slot (a lower tier may still take him)
			}
			used[s][r.Quota]++
			out[i] = composeResult{Target: r.Target, Speed: r.Speed, Class: int(r.Class), Owner: r.Owner, Reason: r.Reason}
		case composeOffset:
			t := r.Target
			if ok[i] {
				dx, dy := t[0]-slots[i][0], t[1]-slots[i][1]
				if d := math.Hypot(dx, dy); d > r.MaxDev && d > 0 {
					t = [2]float64{slots[i][0] + dx/d*r.MaxDev, slots[i][1] + dy/d*r.MaxDev}
				}
			}
			out[i] = composeResult{Target: t, Speed: r.Speed, Class: int(r.Class), Owner: r.Owner, Reason: r.Reason}
		}
		taken[i] = true
	}
	composeSpace(cfg, out, home)
	for i := range out {
		if ok[i] {
			out[i].Deviated = math.Hypot(out[i].Target[0]-slots[i][0], out[i].Target[1]-slots[i][1])
		}
	}
	return out
}

// composeSpace pushes non-ball-job teammates apart to MinSpacing (a few relaxation sweeps in
// index order; ball jobs never move and push the other player the whole gap).
func composeSpace(cfg composeConfig, out []composeResult, home []bool) {
	if cfg.MinSpacing <= 0 {
		return
	}
	fixed := func(i int) bool { return out[i].Class == int(composeBall) }
	for sweep := 0; sweep < 4; sweep++ {
		moved := false
		for a := range out {
			for b := a + 1; b < len(out); b++ {
				if home[a] != home[b] || (fixed(a) && fixed(b)) {
					continue
				}
				dx, dy := out[b].Target[0]-out[a].Target[0], out[b].Target[1]-out[a].Target[1]
				d := math.Hypot(dx, dy)
				if d >= cfg.MinSpacing {
					continue
				}
				ux, uy := 1.0, 0.0
				if d > 1e-9 {
					ux, uy = dx/d, dy/d
				}
				gap := cfg.MinSpacing - d
				switch {
				case fixed(a):
					out[b].Target[0] += ux * gap
					out[b].Target[1] += uy * gap
				case fixed(b):
					out[a].Target[0] -= ux * gap
					out[a].Target[1] -= uy * gap
				default:
					out[a].Target[0] -= ux * gap / 2
					out[a].Target[1] -= uy * gap / 2
					out[b].Target[0] += ux * gap / 2
					out[b].Target[1] += uy * gap / 2
				}
				for _, i := range [2]int{a, b} {
					out[i].Target[0] = hClamp(out[i].Target[0], 0.5, Pitch_LENGTH_METRES-0.5)
					out[i].Target[1] = hClamp(out[i].Target[1], 0.5, Pitch_WIDTH_METRES-0.5)
				}
				moved = true
			}
		}
		if !moved {
			return
		}
	}
}
