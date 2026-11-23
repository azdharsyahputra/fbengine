// Package data loads our FM-style player CSVs (efem_*_full.csv) into the
// Player12-derived engine: full FM attributes are folded into the engine's nine
// aggregate attributes, and a matchday XI is picked for a formation preset.
// The engine is GPL-3.0 (see ../NOTICE.md); this adapter is part of that module.
package data

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"fmengine/engine"
)

// Row is one CSV player with the raw FM attributes (0-100 scale, FM 1-20 × 5).
type Row struct {
	ID        engine.PlayerID
	Name      string
	Club      string
	Age       int
	Positions []string // CSV tokens: GK CB LB RB LWB RWB DM CM AM LM RM LW RW ST
	LeftFoot  float64  // 1-20
	RightFoot float64  // 1-20
	CA        float64
	Attr      map[string]float64
}

// LoadClub reads every player of club from a full-attribute CSV.
func LoadClub(path, club string) ([]Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	head, err := r.Read()
	if err != nil {
		return nil, err
	}
	col := map[string]int{}
	for i, h := range head {
		col[h] = i
	}
	var out []Row
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(rec[col["club"]], club) {
			continue
		}
		num := func(k string) float64 {
			i, ok := col[k]
			if !ok || i >= len(rec) {
				return 0
			}
			v, _ := strconv.ParseFloat(rec[i], 64)
			return v
		}
		row := Row{
			ID:        engine.PlayerID(num("player_id")),
			Name:      rec[col["name"]],
			Club:      rec[col["club"]],
			Age:       int(num("age")),
			LeftFoot:  num("leftFoot"),
			RightFoot: num("rightFoot"),
			CA:        num("currentAbility"),
			Attr:      map[string]float64{},
		}
		for _, p := range strings.Split(rec[col["positions"]], ",") {
			if p = strings.TrimSpace(p); p != "" {
				row.Positions = append(row.Positions, p)
			}
		}
		for _, k := range fmAttrs {
			row.Attr[k] = num(k)
		}
		out = append(out, row)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("club %q not found in %s", club, path)
	}
	return out, nil
}

var fmAttrs = []string{"corners", "crossing", "dribbling", "finishing", "firstTouch", "freekicks", "heading",
	"longshots", "longThrow", "marking", "passing", "penalties", "tackling", "technique", "ballControl",
	"aggression", "anticipation", "bravery", "composure", "concentration", "decisions", "determination",
	"flair", "leadership", "offTheBall", "positioning", "teamwork", "vision", "workrate", "acceleration",
	"agility", "balance", "jumpingReach", "naturalFitness", "pace", "stamina", "strength", "aerialReach",
	"commandOfArea", "communication", "eccentricity", "handling", "kicking", "oneVsOne", "punching",
	"reflexes", "rushingOut", "throwing", "pressure", "importantMatches", "consistency"}

func w(a map[string]float64, parts ...any) float64 {
	s, tw := 0.0, 0.0
	for i := 0; i+1 < len(parts); i += 2 {
		k := parts[i].(string)
		wt := parts[i+1].(float64)
		s += a[k] * wt
		tw += wt
	}
	return s / tw
}

// Aggregates folds FM attributes into Player12's nine engine attributes (1-100).
// The full FM set is also kept under "fm.<name>" for later engine layers.
func Aggregates(a map[string]float64) map[string]float64 {
	st := map[string]float64{
		"Pace":        w(a, "pace", 0.6, "acceleration", 0.4),
		"Shooting":    w(a, "finishing", 0.5, "longshots", 0.2, "composure", 0.15, "technique", 0.15),
		"Passing":     w(a, "passing", 0.55, "technique", 0.25, "decisions", 0.2),
		"Dribbling":   w(a, "dribbling", 0.45, "ballControl", 0.2, "agility", 0.15, "balance", 0.1, "flair", 0.1),
		"Defending":   w(a, "tackling", 0.3, "marking", 0.3, "positioning", 0.25, "anticipation", 0.15),
		"Goalkeeping": w(a, "handling", 0.25, "reflexes", 0.3, "oneVsOne", 0.15, "aerialReach", 0.1, "commandOfArea", 0.1, "kicking", 0.1),
		"Physicality": w(a, "strength", 0.4, "jumpingReach", 0.2, "balance", 0.2, "bravery", 0.2),
		"Stamina":     w(a, "stamina", 0.7, "naturalFitness", 0.3),
		"Vision":      w(a, "vision", 0.5, "decisions", 0.25, "anticipation", 0.25),
	}
	for k, v := range a {
		st["fm."+k] = v
	}
	return st
}

var tokenRole = map[string]engine.PlayerRole{
	"GK": engine.PlayerRole_GK, "CB": engine.PlayerRole_CB, "LB": engine.PlayerRole_LB, "LWB": engine.PlayerRole_LB,
	"RB": engine.PlayerRole_RB, "RWB": engine.PlayerRole_RB, "DM": engine.PlayerRole_CDM, "CM": engine.PlayerRole_CM,
	"AM": engine.PlayerRole_CAM, "LM": engine.PlayerRole_LM, "RM": engine.PlayerRole_RM, "LW": engine.PlayerRole_LW,
	"RW": engine.PlayerRole_RW, "ST": engine.PlayerRole_ST,
}

// Slot is one formation position (own goal at x = 0, y in [0,1]).
type Slot struct {
	Role  engine.PlayerRole
	Label string // viewer label: GK CB FB DM CM AM ST
	Pos   engine.Vector2F
}

// Formations are the Player12 presets (src/gui/view_models/formation.cpp).
var Formations = map[string][]Slot{
	"4-2-3-1": {
		{engine.PlayerRole_LB, "FB", engine.Vector2F{X: 0.20, Y: 0.12}}, {engine.PlayerRole_CB, "CB", engine.Vector2F{X: 0.20, Y: 0.38}},
		{engine.PlayerRole_CB, "CB", engine.Vector2F{X: 0.20, Y: 0.62}}, {engine.PlayerRole_RB, "FB", engine.Vector2F{X: 0.20, Y: 0.88}},
		{engine.PlayerRole_CDM, "DM", engine.Vector2F{X: 0.37, Y: 0.38}}, {engine.PlayerRole_CDM, "DM", engine.Vector2F{X: 0.37, Y: 0.62}},
		{engine.PlayerRole_LW, "AM", engine.Vector2F{X: 0.60, Y: 0.15}}, {engine.PlayerRole_CAM, "AM", engine.Vector2F{X: 0.58, Y: 0.50}},
		{engine.PlayerRole_RW, "AM", engine.Vector2F{X: 0.60, Y: 0.85}}, {engine.PlayerRole_ST, "ST", engine.Vector2F{X: 0.80, Y: 0.50}},
	},
	"4-3-3": {
		{engine.PlayerRole_LB, "FB", engine.Vector2F{X: 0.20, Y: 0.12}}, {engine.PlayerRole_CB, "CB", engine.Vector2F{X: 0.20, Y: 0.38}},
		{engine.PlayerRole_CB, "CB", engine.Vector2F{X: 0.20, Y: 0.62}}, {engine.PlayerRole_RB, "FB", engine.Vector2F{X: 0.20, Y: 0.88}},
		{engine.PlayerRole_CM, "DM", engine.Vector2F{X: 0.44, Y: 0.28}}, {engine.PlayerRole_CDM, "DM", engine.Vector2F{X: 0.38, Y: 0.50}},
		{engine.PlayerRole_CM, "DM", engine.Vector2F{X: 0.44, Y: 0.72}}, {engine.PlayerRole_LW, "AM", engine.Vector2F{X: 0.72, Y: 0.14}},
		{engine.PlayerRole_ST, "ST", engine.Vector2F{X: 0.80, Y: 0.50}}, {engine.PlayerRole_RW, "AM", engine.Vector2F{X: 0.72, Y: 0.86}},
	},
}

// fit scores how well a CSV player suits a role (1 = natural, 0 = never).
func fit(r Row, role engine.PlayerRole) float64 {
	best := 0.0
	for i, t := range r.Positions {
		pr, ok := tokenRole[t]
		if !ok {
			continue
		}
		s := 0.0
		switch {
		case pr == role:
			s = 1.0
		case near(pr, role):
			s = 0.8
		case wider(pr, role):
			s = 0.55
		}
		s *= 1.0 - 0.05*float64(i) // listed later = slightly less natural
		if s > best {
			best = s
		}
	}
	return best
}

func near(a, b engine.PlayerRole) bool {
	pairs := [][2]engine.PlayerRole{
		{engine.PlayerRole_CDM, engine.PlayerRole_CM}, {engine.PlayerRole_CAM, engine.PlayerRole_CM},
		{engine.PlayerRole_LW, engine.PlayerRole_LM}, {engine.PlayerRole_RW, engine.PlayerRole_RM},
		{engine.PlayerRole_LW, engine.PlayerRole_RW}, {engine.PlayerRole_CAM, engine.PlayerRole_LW},
		{engine.PlayerRole_CAM, engine.PlayerRole_RW}, {engine.PlayerRole_ST, engine.PlayerRole_CAM},
	}
	for _, p := range pairs {
		if (p[0] == a && p[1] == b) || (p[0] == b && p[1] == a) {
			return true
		}
	}
	return false
}

func wider(a, b engine.PlayerRole) bool {
	group := func(r engine.PlayerRole) int {
		switch r {
		case engine.PlayerRole_CB, engine.PlayerRole_LB, engine.PlayerRole_RB:
			return 1
		case engine.PlayerRole_CDM, engine.PlayerRole_CM, engine.PlayerRole_CAM, engine.PlayerRole_LM, engine.PlayerRole_RM:
			return 2
		case engine.PlayerRole_LW, engine.PlayerRole_RW, engine.PlayerRole_ST:
			return 3
		}
		return 0
	}
	return group(a) != 0 && group(a) == group(b)
}

// Picked is a chosen player with his viewer metadata.
type Picked struct {
	Player *engine.Player
	Row    Row
	Label  string
	Number int
}

// Squad is a matchday squad ready for the engine.
type Squad struct {
	Club      string
	Lineup    *engine.Lineup
	XI        []Picked // [0] = GK, then formation slot order
	Bench     []Picked
	Formation string
}

func toPlayer(r Row, teamID engine.TeamID, role engine.PlayerRole, number int) *engine.Player {
	parts := strings.SplitN(r.Name, " ", 2)
	first, last := parts[0], ""
	if len(parts) == 2 {
		last = parts[1]
	}
	foot := engine.Foot_Right
	if r.LeftFoot > r.RightFoot {
		foot = engine.Foot_Left
	}
	p := engine.NewPlayer(r.ID, teamID, first, last, role, uint8(r.Age), 0, foot, Aggregates(r.Attr))
	p.SquadNumber = uint8(number)
	p.Potential = r.CA
	return p
}

// BuildSquad picks a GK and a formation XI (greedy by fit × ability, scarcest
// roles first) plus a seven-man bench.
func BuildSquad(rows []Row, teamID engine.TeamID, formation string) (*Squad, error) {
	slots, ok := Formations[formation]
	if !ok {
		return nil, fmt.Errorf("unknown formation %q", formation)
	}
	used := map[int]bool{}
	pick := func(role engine.PlayerRole) int {
		best, bi := -1.0, -1
		for i, r := range rows {
			if used[i] {
				continue
			}
			s := fit(r, role) * (0.5 + r.CA/100)
			if s > best {
				best, bi = s, i
			}
		}
		if bi >= 0 {
			used[bi] = true
		}
		return bi
	}
	sq := &Squad{Club: rows[0].Club, Lineup: engine.NewLineup(), Formation: formation}
	gk := pick(engine.PlayerRole_GK)
	if gk < 0 {
		return nil, fmt.Errorf("%s: no goalkeeper", rows[0].Club)
	}
	gp := toPlayer(rows[gk], teamID, engine.PlayerRole_GK, 1)
	sq.Lineup.SetGoalkeeper(gp)
	sq.XI = append(sq.XI, Picked{gp, rows[gk], "GK", 1})
	// scarcest roles first so a lone natural full-back isn't spent elsewhere
	order := make([]int, len(slots))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		ca, cb := 0, 0
		for _, r := range rows {
			if fit(r, slots[order[a]].Role) >= 1 {
				ca++
			}
			if fit(r, slots[order[b]].Role) >= 1 {
				cb++
			}
		}
		return ca < cb
	})
	chosen := make([]Picked, len(slots))
	for _, si := range order {
		s := slots[si]
		i := pick(s.Role)
		if i < 0 {
			return nil, fmt.Errorf("%s: not enough players", rows[0].Club)
		}
		p := toPlayer(rows[i], teamID, s.Role, si+2)
		chosen[si] = Picked{p, rows[i], s.Label, si + 2}
	}
	for si, c := range chosen {
		sq.Lineup.AddOutfieldPlayer(c.Player, slots[si].Pos)
		sq.XI = append(sq.XI, c)
	}
	// bench: best remaining by ability, at least one goalkeeper
	var rest []int
	for i := range rows {
		if !used[i] {
			rest = append(rest, i)
		}
	}
	sort.Slice(rest, func(a, b int) bool { return rows[rest[a]].CA > rows[rest[b]].CA })
	var subs []*engine.Player
	haveGK := false
	n := 12
	for _, i := range rest {
		isGK := len(rows[i].Positions) > 0 && rows[i].Positions[0] == "GK"
		if isGK && haveGK {
			continue
		}
		if len(subs) == 6 && !haveGK && !isGK {
			continue
		}
		role := tokenRole[rows[i].Positions[0]]
		p := toPlayer(rows[i], teamID, role, n)
		label := map[engine.PlayerRole]string{engine.PlayerRole_GK: "GK", engine.PlayerRole_CB: "CB", engine.PlayerRole_LB: "FB",
			engine.PlayerRole_RB: "FB", engine.PlayerRole_CDM: "DM", engine.PlayerRole_CM: "DM", engine.PlayerRole_ST: "ST"}[role]
		if label == "" {
			label = "AM"
		}
		sq.Bench = append(sq.Bench, Picked{p, rows[i], label, n})
		subs = append(subs, p)
		haveGK = haveGK || isGK
		n++
		if len(subs) == 7 {
			break
		}
	}
	sq.Lineup.SetReserves(subs)
	return sq, nil
}
