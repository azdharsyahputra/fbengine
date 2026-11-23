// Adapter (not from Player12): runs a full match and samples it for the fm-game
// replay viewer (viewer/replay.js format). Part of the GPL-3.0 fmengine module.
package engine

import (
	"fmt"
	"math"
	"strings"
)

// ReplayEvent is one viewer event; T is the sample index (TPS samples per second).
type ReplayEvent struct {
	T      int
	Minute int
	Kind   string // goal restart foul offside miss save yellow red ht sub
	Team   int    // 0 home, 1 away, -1 none
	Text   string
}

// TeamStats is the per-team match summary printed by the CLI.
type TeamStats struct {
	Goals, Shots, OnTarget, Fouls, Yellow, Red, Corners, Offsides int
	Passes, PassesCompleted                                       int
	PossessionShare                                               float64
}

// ReplaySchema is the replay format version written by RunReplayWith (ours):
//
//	1 (implicit, no "schema" key): frames/events/cond only;
//	2: adds the per-sample live-play flag and the effective EngineConfig +
//	   hash. The gameplay payload is unchanged from schema 1.
const ReplaySchema = 2

// ReplayData is a sampled match. Frames hold, per sample, ball x,y,z then 22×(x,y),
// in decimetres (metres × 10, -1 = off the pitch), slot order = home 0..10, away 0..10.
type ReplayData struct {
	TPS       int
	Ticks     int
	Frames    []int32
	Events    []ReplayEvent
	Cond      []uint8 // 22 values every CondEvery samples
	CondEvery int
	Stats     [2]TeamStats
	HomeGoals int
	AwayGoals int
	// Live[i] is true when the engine was in live play (MatchState_PLAYING)
	// at sample i; dead-ball periods (restarts, goal celebration, kick-off,
	// half time) are false.
	Live []bool
	// Seed, Config and ConfigHash reproduce the match: the same seed, line-ups
	// and Config give an identical replay.
	Seed       uint32
	Config     EngineConfig
	ConfigHash string
	// Audit is the probe summary when Diagnostics.Audit is on.
	Audit string
	// Slots: the Team Shape Controller's slot per player and sample, same layout
	// as the players in Frames (x, y decimetres; -1 without a slot). Only with
	// Diagnostics.Slots (shadow mode).
	Slots []int32
	// Clock is the match clock, run-length: pairs (sample, match second)
	// at every change of the whole match second (first half from 0, second
	// from 2700); the clock stops during some stoppages, so it is not the
	// sample count.
	Clock []int32
}

// RunOptions configures one RunReplayWith call.
type RunOptions struct {
	Config      EngineConfig
	Diagnostics Diagnostics
}

// RunReplay simulates home vs away to full time with the default
// EngineConfig and samples it at tps per second.
func RunReplay(home, away *Lineup, homeName, awayName string, seed uint32, tps int) (*ReplayData, error) {
	return RunReplayWith(home, away, homeName, awayName, seed, tps, RunOptions{Config: DefaultConfig()})
}

// RunReplayWith is RunReplay with an explicit per-match configuration.
// Matches are independent: several may run concurrently.
func RunReplayWith(home, away *Lineup, homeName, awayName string, seed uint32, tps int, opt RunOptions) (*ReplayData, error) {
	cfg := &StatsConfig{}
	e := NewMatchEngineWithConfig(home, away, home.GetStrategy(), away.GetStrategy(), cfg, seed, opt.Config, opt.Diagnostics)
	e.setTeamNames(homeName, awayName)
	if len(e.players) != 22 {
		return nil, fmt.Errorf("expected 22 players on the pitch, got %d", len(e.players))
	}
	out := &ReplayData{TPS: tps, CondEvery: 40, Seed: seed, Config: opt.Config, ConfigHash: opt.Config.Hash()}
	sample := 1.0 / float64(tps)
	next := 0.0
	seen := 0
	byID := map[PlayerID]string{}
	name := func(id PlayerID) string {
		if n, ok := byID[id]; ok {
			return n
		}
		for i := range e.players {
			if p := e.players[i].Player; p != nil && p.GetId() == id {
				return strings.TrimSpace(p.GetName())
			}
		}
		for _, l := range []*Lineup{home, away} {
			for _, p := range l.GetReserves() {
				if p.GetId() == id {
					return strings.TrimSpace(p.GetName())
				}
			}
		}
		return "?"
	}
	for i := range e.players {
		if p := e.players[i].Player; p != nil {
			byID[p.GetId()] = strings.TrimSpace(p.GetName())
		}
	}
	const maxSteps = 200000
	prev := cSnap(e)
	for step := 0; step < maxSteps && e.state != MatchState_FULL_TIME; step++ {
		t0 := e.getSimulatedSeconds()
		e.simulateStep(e.stepSeconds())
		t1 := e.getSimulatedSeconds()
		// events raised during this step
		for ; seen < len(e.events); seen++ {
			if ev, ok := toReplayEvent(&e.events[seen], out.Ticks, name, homeName, awayName); ok {
				out.Events = append(out.Events, ev)
			}
		}
		// sample at exact 1/tps instants, interpolating inside the engine step
		// (engine steps are 0.1 s; sampling whole steps gave 0.2/0.3 s gaps)
		e.auditStep()
		cur := cSnap(e)
		live := e.state == MatchState_PLAYING
		clockSec := int32(math.Floor(e.matchTimeMinutes*60.0 + 1e-6))
		for t1+1e-9 >= next {
			a := 1.0
			if t1 > t0 {
				a = hClamp((next-t0)/(t1-t0), 0, 1)
			}
			if n := len(out.Clock); n == 0 || out.Clock[n-1] != clockSec {
				out.Clock = append(out.Clock, int32(out.Ticks), clockSec)
			}
			out.capture(e, prev, cur, a)
			out.Live = append(out.Live, live)
			if e.diag.Slots {
				var slots [][2]float64
				var ok []bool
				if e.cfg.Modules.TeamShape {
					slots, ok = e.teamShape.last, e.teamShape.lastOK
				} else {
					slots, ok = e.teamShapeSlots()
				}
				if len(slots) != len(e.players) {
					slots, ok = make([][2]float64, len(e.players)), make([]bool, len(e.players))
				}
				for i := range slots {
					if !ok[i] {
						out.Slots = append(out.Slots, -1, -1)
						continue
					}
					out.Slots = append(out.Slots, dm(slots[i][0]), dm(slots[i][1]))
				}
			}
			next += sample
		}
		prev = cur
	}
	out.HomeGoals, out.AwayGoals = e.homeScore, e.awayScore
	out.Stats = e.cSummary()
	if e.diag.Audit {
		var b strings.Builder
		e.auditReport(&b)
		out.Audit = b.String()
	}
	return out, nil
}

func dm(v float64) int32 { return int32(math.Round(v * 10)) }

// cSnap holds ball (x,y,z metres) then per player (x,y metres, on pitch).
type cSnapshot struct {
	ball [3]float64
	pl   [][2]float64
	on   []bool
}

func cSnap(e *MatchEngine) cSnapshot {
	s := cSnapshot{ball: [3]float64{e.ball.Position.X * Pitch_LENGTH_METRES, e.ball.Position.Y * Pitch_WIDTH_METRES, math.Max(0, e.ball.Z*Units_BALL_Z_METRES)}}
	for i := range e.players {
		p := &e.players[i]
		s.pl = append(s.pl, [2]float64{p.Position.X * Pitch_LENGTH_METRES, p.Position.Y * Pitch_WIDTH_METRES})
		s.on = append(s.on, p.Player != nil && p.OnPitch)
	}
	return s
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func (r *ReplayData) capture(e *MatchEngine, prev, cur cSnapshot, a float64) {
	r.Frames = append(r.Frames, dm(lerp(prev.ball[0], cur.ball[0], a)), dm(lerp(prev.ball[1], cur.ball[1], a)), dm(lerp(prev.ball[2], cur.ball[2], a)))
	for i := range cur.pl {
		if !cur.on[i] {
			r.Frames = append(r.Frames, -1, -1)
			continue
		}
		p0 := prev.pl[i]
		if !prev.on[i] {
			p0 = cur.pl[i]
		}
		r.Frames = append(r.Frames, dm(lerp(p0[0], cur.pl[i][0], a)), dm(lerp(p0[1], cur.pl[i][1], a)))
	}
	if r.Ticks%r.CondEvery == 0 {
		for i := range e.players {
			r.Cond = append(r.Cond, uint8(math.Round(hClamp(e.players[i].Stamina, 0, 1)*100)))
		}
	}
	r.Ticks++
}

var restartText = map[MatchEventType]string{
	MatchEventType_CORNER:    "Tendangan sudut",
	MatchEventType_FREE_KICK: "Tendangan bebas",
	MatchEventType_THROW_IN:  "Lemparan ke dalam",
	MatchEventType_GOAL_KICK: "Tendangan gawang",
	MatchEventType_PENALTY:   "Penalti",
}

func toReplayEvent(ev *MatchEvent, tick int, name func(PlayerID) string, homeName, awayName string) (ReplayEvent, bool) {
	team := -1
	club := ""
	if ev.HasTeam {
		team = 1
		club = awayName
		if ev.IsHomeTeam {
			team = 0
			club = homeName
		}
	}
	out := ReplayEvent{T: tick, Minute: int(ev.TimeMinute), Team: team}
	p1 := name(ev.PrimaryPlayerId)
	switch ev.Type {
	case MatchEventType_GOAL:
		out.Kind = "goal"
		out.Text = fmt.Sprintf("GOL! %s (%s) — %s %d-%d %s", p1, club, homeName, ev.HomeScore, ev.AwayScore, awayName)
	case MatchEventType_OWN_GOAL:
		// The goal belongs to the scorer's opponents (the viewer's score and
		// highlight build-up read Team).
		out.Kind = "goal"
		if team >= 0 {
			out.Team = 1 - team
		}
		out.Text = fmt.Sprintf("GOL bunuh diri %s (%s) — %s %d-%d %s", p1, club, homeName, ev.HomeScore, ev.AwayScore, awayName)
	case MatchEventType_SAVE:
		// Team stays the keeper's side (viewer convention: save → 1 - attacker).
		out.Kind, out.Text = "save", "Penyelamatan "+p1
		if ev.SecondaryPlayerId != 0 {
			out.Text += " — tembakan " + name(ev.SecondaryPlayerId)
		}
	case MatchEventType_SHOT_BLOCKED:
		// Logged with the blocking defender; the viewer reads a miss's Team as
		// the shooting side.
		out.Kind, out.Text = "miss", "Tembakan diblok "+p1
		if team >= 0 {
			out.Team = 1 - team
		}
	case MatchEventType_SHOT_OFF_TARGET:
		out.Kind, out.Text = "miss", "Tembakan "+p1+" melenceng"
	case MatchEventType_WOODWORK:
		out.Kind, out.Text = "miss", "Tembakan "+p1+" kena tiang"
	case MatchEventType_PENALTY_MISSED:
		out.Kind, out.Text = "miss", "Penalti "+p1+" gagal"
	case MatchEventType_FOUL:
		out.Kind, out.Text = "foul", "Pelanggaran oleh "+p1
	case MatchEventType_YELLOW_CARD:
		out.Kind, out.Text = "yellow", "Kartu kuning untuk "+p1+"."
	case MatchEventType_SECOND_YELLOW:
		out.Kind, out.Text = "red", "Kartu kuning kedua — "+p1+" diusir!"
	case MatchEventType_RED_CARD:
		out.Kind, out.Text = "red", "KARTU MERAH langsung untuk "+p1+"!"
	case MatchEventType_OFFSIDE:
		out.Kind, out.Text = "offside", "Offside: "+p1
	case MatchEventType_HALF_TIME:
		out.Kind, out.Text = "ht", "Turun minum"
	case MatchEventType_SUBSTITUTION:
		out.Kind = "sub"
		out.Text = fmt.Sprintf("Pergantian %s: %s masuk, %s keluar", club, p1, name(ev.SecondaryPlayerId))
	default:
		if t, ok := restartText[ev.Type]; ok {
			out.Kind, out.Text = "restart", t
		} else {
			return out, false
		}
	}
	return out, true
}

// cSummary collects the team statistics the CLI prints.
func (e *MatchEngine) cSummary() [2]TeamStats {
	var s [2]TeamStats
	s[0].Goals, s[1].Goals = e.homeScore, e.awayScore
	for i := range e.events {
		ev := &e.events[i]
		if !ev.HasTeam {
			continue
		}
		t := 1
		if ev.IsHomeTeam {
			t = 0
		}
		switch ev.Type {
		case MatchEventType_GOAL, MatchEventType_SHOT_OFF_TARGET, MatchEventType_WOODWORK, MatchEventType_PENALTY_MISSED:
			s[t].Shots++
			if ev.Type == MatchEventType_GOAL {
				s[t].OnTarget++
			}
		case MatchEventType_SAVE:
			// logged with the keeper: the shot is the other side's
			s[1-t].Shots++
			s[1-t].OnTarget++
		case MatchEventType_SHOT_BLOCKED:
			// logged with the blocking defender
			s[1-t].Shots++
		case MatchEventType_FOUL:
			s[t].Fouls++
		case MatchEventType_YELLOW_CARD:
			s[t].Yellow++
		case MatchEventType_SECOND_YELLOW, MatchEventType_RED_CARD:
			s[t].Red++
		case MatchEventType_CORNER:
			s[t].Corners++
		case MatchEventType_OFFSIDE:
			s[t].Offsides++
		}
	}
	return s
}
