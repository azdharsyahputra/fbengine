// Command match runs one match from our CSV data and writes a viewer replay.
//
//	go run ./cmd/match [--config=cfg.json|replay.js] "Liverpool" "Man City" 19 [out.js] [formation]
//	go run ./cmd/match --reproduce=replay.js [out.js] [--allow-data-change]
//
// Configuration (the only place the FMENGINE_* environment is read):
//   - default: engine.DefaultConfig() plus the legacy overrides FMENGINE_ON,
//     FMENGINE_OFF, FMENGINE_M2TUNE, FMENGINE_M10, FMENGINE_M1
//     (engine.ApplyOverrides);
//   - --config=FILE: the EngineConfig in FILE (a config JSON, or a schema-2
//     replay's meta.config, hash-checked); the overrides are then ignored.
//
// Diagnostics (never change the match): FMENGINE_M2TRACE=1|2|3, FMENGINE_AUDIT=1,
// FMENGINE_SLOTS=1 (Team Shape Controller slots in the replay, shadow mode).
//
// Reproduction (Phase 2 / F): --reproduce=REPLAY replays the match recorded in
// REPLAY's meta (clubs, formation, seed, EngineConfig) and compares the
// gameplay payload with it. The player data must be unchanged: the CSV's
// sha256 and the line-ups' sha256 (the built squads, every attribute) are
// checked first; --allow-data-change runs anyway and reports the change.
// Exit status: 0 reproduced, 1 mismatch or error, 2 usage.
//
// The replay is schema 2: the schema-1 payload (frames, events, cond) plus
// "live" (one '1'/'0' per sample: live play / dead ball) and "meta" (seed,
// tps, clubs, formation, data file and line-up hashes, full EngineConfig,
// config hash).
//
// Default output: /Users/csadeveloper/fm/fm-game/viewer/replay_go_s<seed>.js
// (open viewer.html?r=replay_go_s<seed>.js). Part of the GPL-3.0 fmengine module.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"fmengine/data"
	"fmengine/engine"
)

// defaultCSV is the player data (kept outside the repository).
const defaultCSV = "/Users/csadeveloper/fm/efem_epl_full.csv"

type vPlayer struct {
	N    int     `json:"n"`
	Name string  `json:"name"`
	Pos  string  `json:"pos"`
	R    float64 `json:"r"`
}
type vTeam struct {
	Name    string    `json:"name"`
	Tactic  string    `json:"tactic"`
	Players []vPlayer `json:"players"`
	Subs    []vPlayer `json:"subs"`
}
type vEvent struct {
	T    int    `json:"t"`
	M    int    `json:"m"`
	K    string `json:"k"`
	Team int    `json:"team"`
	Text string `json:"text"`
}
type vData struct {
	CSV    string `json:"csv"`
	SHA256 string `json:"sha256"`
	// LineupSHA256: hash of both built squads (every Row field and attribute,
	// numbers, labels, bench); catches data or squad-selection changes.
	LineupSHA256 string `json:"lineup_sha256,omitempty"`
}
type vMeta struct {
	Schema     int                 `json:"schema"`
	Engine     string              `json:"engine"`
	Seed       uint32              `json:"seed"`
	TPS        int                 `json:"tps"`
	Home       string              `json:"home"`
	Away       string              `json:"away"`
	Formation  string              `json:"formation"`
	Data       vData               `json:"data"`
	Config     engine.EngineConfig `json:"config"`
	ConfigHash string              `json:"config_hash"`
	LiveRule   string              `json:"live_rule"`
}
type vReplay struct {
	PL        int      `json:"pl"`
	PW        int      `json:"pw"`
	Ticks     int      `json:"ticks"`
	TPS       int      `json:"tps"`
	Home      vTeam    `json:"home"`
	Away      vTeam    `json:"away"`
	Events    []vEvent `json:"events"`
	Frames    []int32  `json:"frames"`
	CondEvery int      `json:"cond_every"`
	Cond      []uint8  `json:"cond"`
	// schema 2 (after the schema-1 payload)
	Schema int    `json:"schema"`
	Live   string `json:"live"`
	// Clock: run-length match clock, pairs (sample, match second).
	Clock []int32 `json:"clock,omitempty"`
	Meta  vMeta   `json:"meta"`
	// Slots: Team Shape Controller slots (shadow mode, FMENGINE_SLOTS=1), same
	// layout as the players in Frames.
	Slots []int32 `json:"slots,omitempty"`
}

func team(sq *data.Squad) vTeam {
	t := vTeam{Name: sq.Club, Tactic: sq.Formation + " | Player12 default"}
	for _, p := range sq.XI {
		t.Players = append(t.Players, vPlayer{p.Number, p.Row.Name, p.Label, p.Row.CA / 10})
	}
	for _, p := range sq.Bench {
		t.Subs = append(t.Subs, vPlayer{p.Number, p.Row.Name, p.Label, p.Row.CA / 10})
	}
	return t
}

// matchSpec is everything that determines a match.
type matchSpec struct {
	Home, Away, Formation string
	Seed                  uint32
	Config                engine.EngineConfig
	Diag                  engine.Diagnostics
	CSV                   string
}

func loadSquad(csv, club string, id engine.TeamID, formation string) (*data.Squad, error) {
	rows, err := data.LoadClub(csv, club)
	if err != nil {
		return nil, err
	}
	return data.BuildSquad(rows, id, formation)
}

// lineupHash hashes both built squads: every Row field (attributes included;
// json sorts map keys, so the encoding is canonical), numbers, labels, bench.
func lineupHash(home, away *data.Squad) string {
	type picked struct {
		Row    data.Row
		Label  string
		Number int
	}
	type squad struct {
		Club, Formation string
		XI, Bench       []picked
	}
	conv := func(sq *data.Squad) squad {
		s := squad{Club: sq.Club, Formation: sq.Formation}
		for _, p := range sq.XI {
			s.XI = append(s.XI, picked{p.Row, p.Label, p.Number})
		}
		for _, p := range sq.Bench {
			s.Bench = append(s.Bench, picked{p.Row, p.Label, p.Number})
		}
		return s
	}
	raw, err := json.Marshal([2]squad{conv(home), conv(away)})
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// play runs the match and builds the replay document.
func play(spec matchSpec) (vReplay, *engine.ReplayData, error) {
	home, err := loadSquad(spec.CSV, spec.Home, 1, spec.Formation)
	if err != nil {
		return vReplay{}, nil, err
	}
	away, err := loadSquad(spec.CSV, spec.Away, 2, spec.Formation)
	if err != nil {
		return vReplay{}, nil, err
	}
	rep, err := engine.RunReplayWith(home.Lineup, away.Lineup, home.Club, away.Club, spec.Seed, 4,
		engine.RunOptions{Config: spec.Config, Diagnostics: spec.Diag})
	if err != nil {
		return vReplay{}, nil, err
	}
	v := vReplay{PL: 105, PW: 68, Ticks: rep.Ticks, TPS: rep.TPS, Home: team(home), Away: team(away),
		Frames: rep.Frames, CondEvery: rep.CondEvery, Cond: rep.Cond, Schema: engine.ReplaySchema}
	live := make([]byte, len(rep.Live))
	for i, l := range rep.Live {
		live[i] = '0'
		if l {
			live[i] = '1'
		}
	}
	v.Live = string(live)
	v.Clock = rep.Clock
	v.Slots = rep.Slots
	v.Meta = vMeta{Schema: engine.ReplaySchema, Engine: "fm-engine-go", Seed: rep.Seed, TPS: rep.TPS,
		Home: home.Club, Away: away.Club, Formation: spec.Formation,
		Data:   vData{CSV: spec.CSV, SHA256: fileSHA256(spec.CSV), LineupSHA256: lineupHash(home, away)},
		Config: rep.Config, ConfigHash: rep.ConfigHash,
		LiveRule: "engine state == PLAYING after the step that produced the sample"}
	for _, e := range rep.Events {
		v.Events = append(v.Events, vEvent{e.T, e.Minute, e.Kind, e.Team, e.Text})
	}
	return v, rep, nil
}

func writeReplay(path string, v vReplay) error {
	js, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(append([]byte("window.REPLAY="), js...), ';', '\n'), 0o644)
}

// readReplay parses a replay file; meta is nil for a schema-1 replay.
func readReplay(path string) (*vReplay, *vMeta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	raw = bytes.TrimSpace(raw)
	r, ok := bytes.CutPrefix(raw, []byte("window.REPLAY="))
	if !ok {
		return nil, nil, fmt.Errorf("%s: not a replay (no window.REPLAY=)", path)
	}
	var doc struct {
		vReplay
		Meta *vMeta `json:"meta"`
	}
	if err := json.Unmarshal(bytes.TrimSuffix(r, []byte(";")), &doc); err != nil {
		return nil, nil, err
	}
	v := doc.vReplay
	if doc.Meta != nil {
		v.Meta = *doc.Meta
	}
	return &v, doc.Meta, nil
}

// metaConfig returns a replay meta's config after checking its hash.
func metaConfig(path string, m *vMeta) (engine.EngineConfig, error) {
	if m == nil {
		return engine.EngineConfig{}, fmt.Errorf("%s: schema-1 replay has no EngineConfig (written before configs were recorded); it cannot be reproduced exactly", path)
	}
	if m.ConfigHash == "" || m.Engine != "fm-engine-go" {
		return engine.EngineConfig{}, fmt.Errorf("%s: not an fm-engine-go replay (no recorded EngineConfig; e.g. real tracking data)", path)
	}
	if h := m.Config.Hash(); h != m.ConfigHash {
		return engine.EngineConfig{}, fmt.Errorf("%s: config hash %s… does not match the recorded %s… (config fields changed since this replay; reproduce it with the engine version that wrote it, see CHANGELOG / git log)", path, short(h), short(m.ConfigHash))
	}
	return m.Config, nil
}

// checkData compares the recorded data hashes with the current data; it
// returns a description of every difference (empty: unchanged).
func checkData(m *vMeta, csv string, lineup string) []string {
	var diffs []string
	if m.Data.SHA256 == "" {
		diffs = append(diffs, "replay records no CSV hash")
	} else if h := fileSHA256(csv); h != m.Data.SHA256 {
		diffs = append(diffs, fmt.Sprintf("CSV %s changed (sha256 %s…, recorded %s…)", csv, short(h), short(m.Data.SHA256)))
	}
	if m.Data.LineupSHA256 == "" {
		diffs = append(diffs, "replay records no line-up hash (written before Phase 2 F)")
	} else if lineup != "" && lineup != m.Data.LineupSHA256 {
		diffs = append(diffs, fmt.Sprintf("line-ups changed (sha256 %s…, recorded %s…)", short(lineup), short(m.Data.LineupSHA256)))
	}
	return diffs
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// loadConfig reads an EngineConfig from a config JSON file or a schema-2
// replay (meta.config, whose hash must match).
func loadConfig(path string) (engine.EngineConfig, *vMeta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return engine.EngineConfig{}, nil, err
	}
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("window.REPLAY=")) {
		_, m, err := readReplay(path)
		if err != nil {
			return engine.EngineConfig{}, nil, err
		}
		cfg, err := metaConfig(path, m)
		return cfg, m, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	cfg := engine.DefaultConfig()
	err = dec.Decode(&cfg)
	return cfg, nil, err
}

func fileSHA256(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// samePayload compares the gameplay payload of two replays (everything the
// match produced; meta and schema excluded).
func samePayload(a, b *vReplay) (bool, string) {
	switch {
	case a.Ticks != b.Ticks:
		return false, fmt.Sprintf("ticks %d vs %d", a.Ticks, b.Ticks)
	case !reflect.DeepEqual(a.Frames, b.Frames):
		for i := range a.Frames {
			if i >= len(b.Frames) || a.Frames[i] != b.Frames[i] {
				return false, fmt.Sprintf("frames differ from sample %d", i/47)
			}
		}
		return false, "frames differ in length"
	case !reflect.DeepEqual(a.Events, b.Events):
		return false, "events differ"
	case !reflect.DeepEqual(a.Cond, b.Cond):
		return false, "condition samples differ"
	case !reflect.DeepEqual(a.Home, b.Home) || !reflect.DeepEqual(a.Away, b.Away):
		return false, "team sheets differ"
	case a.Live != "" && b.Live != "" && a.Live != b.Live:
		return false, "live flags differ"
	}
	return true, ""
}

// reproduce replays the match recorded in path and compares the payload.
func reproduce(path, out, csv string, allowDataChange bool) (bool, string, error) {
	want, m, err := readReplay(path)
	if err != nil {
		return false, "", err
	}
	cfg, err := metaConfig(path, m)
	if err != nil {
		return false, "", err
	}
	spec := matchSpec{Home: m.Home, Away: m.Away, Formation: m.Formation, Seed: m.Seed, Config: cfg, CSV: csv}
	if spec.Formation == "" {
		spec.Formation = "4-2-3-1"
	}
	// Line-up hash before playing: build the squads once.
	home, err := loadSquad(csv, spec.Home, 1, spec.Formation)
	if err != nil {
		return false, "", err
	}
	away, err := loadSquad(csv, spec.Away, 2, spec.Formation)
	if err != nil {
		return false, "", err
	}
	diffs := checkData(m, csv, lineupHash(home, away))
	var notes []string
	for _, d := range diffs {
		hard := strings.Contains(d, "changed")
		if hard && !allowDataChange {
			return false, "", fmt.Errorf("%s: %s (use --allow-data-change to run anyway)", path, d)
		}
		notes = append(notes, d)
	}
	got, _, err := play(spec)
	if err != nil {
		return false, "", err
	}
	if out != "" {
		if err := writeReplay(out, got); err != nil {
			return false, "", err
		}
	}
	ok, why := samePayload(want, &got)
	report := fmt.Sprintf("%s vs %s, seed %d, config %s…", m.Home, m.Away, m.Seed, short(m.ConfigHash))
	if len(notes) > 0 {
		report += " | notes: " + strings.Join(notes, "; ")
	}
	if !ok {
		report += " | " + why
	}
	return ok, report, nil
}

func main() {
	var configPath, reproducePath string
	allowDataChange := false
	args := []string{os.Args[0]}
	for _, a := range os.Args[1:] {
		switch {
		case strings.HasPrefix(a, "--config="):
			configPath = strings.TrimPrefix(a, "--config=")
		case strings.HasPrefix(a, "--reproduce="):
			reproducePath = strings.TrimPrefix(a, "--reproduce=")
		case a == "--allow-data-change":
			allowDataChange = true
		default:
			args = append(args, a)
		}
	}
	if reproducePath != "" {
		out := ""
		if len(args) > 1 {
			out = args[1]
		}
		ok, report, err := reproduce(reproducePath, out, defaultCSV, allowDataChange)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if !ok {
			fmt.Println("MISMATCH:", report)
			os.Exit(1)
		}
		fmt.Println("REPRODUCED:", report)
		return
	}
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: match [--config=cfg.json|replay.js] <home> <away> [seed] [out.js] [formation]\n       match --reproduce=replay.js [out.js] [--allow-data-change]")
		os.Exit(2)
	}
	cfg := engine.DefaultConfig()
	if configPath != "" {
		var m *vMeta
		var err error
		if cfg, m, err = loadConfig(configPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if m != nil {
			for _, d := range checkData(m, defaultCSV, "") {
				if strings.Contains(d, "changed") && !allowDataChange {
					fmt.Fprintf(os.Stderr, "%s: %s (use --allow-data-change to run anyway)\n", configPath, d)
					os.Exit(1)
				}
				fmt.Fprintln(os.Stderr, "note:", d)
			}
		}
		for _, k := range []string{"FMENGINE_ON", "FMENGINE_OFF", "FMENGINE_M2TUNE", "FMENGINE_M10", "FMENGINE_M1"} {
			if os.Getenv(k) != "" {
				fmt.Fprintf(os.Stderr, "warning: %s ignored (--config given)\n", k)
			}
		}
	} else if ignored := engine.ApplyOverrides(&cfg, os.Getenv("FMENGINE_ON"), os.Getenv("FMENGINE_OFF"),
		os.Getenv("FMENGINE_M2TUNE"), os.Getenv("FMENGINE_M10"), os.Getenv("FMENGINE_M1")); len(ignored) > 0 {
		fmt.Fprintf(os.Stderr, "warning: unknown override keys ignored: %v\n", ignored)
	}
	diag := engine.Diagnostics{Audit: os.Getenv("FMENGINE_AUDIT") != "", Slots: os.Getenv("FMENGINE_SLOTS") != "", Out: os.Stderr}
	if t := os.Getenv("FMENGINE_M2TRACE"); t != "" {
		diag.M2Trace, _ = strconv.Atoi(t)
		if diag.M2Trace == 0 {
			diag.M2Trace = 1 // any non-numeric value enabled level 1 before
		}
	}
	seed := uint64(1)
	if len(args) > 3 {
		seed, _ = strconv.ParseUint(args[3], 10, 32)
	}
	out := fmt.Sprintf("/Users/csadeveloper/fm/fm-game/viewer/replay_go_s%d.js", seed)
	if len(args) > 4 && args[4] != "" {
		out = args[4]
	}
	formation := "4-2-3-1"
	if len(args) > 5 {
		formation = args[5]
	}
	v, rep, err := play(matchSpec{Home: args[1], Away: args[2], Formation: formation, Seed: uint32(seed),
		Config: cfg, Diag: diag, CSV: defaultCSV})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := writeReplay(out, v); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	s := rep.Stats
	fmt.Fprint(os.Stderr, rep.Audit)
	fmt.Printf("%s %d-%d %s (seed %d, %d sampel = %.1f menit)\n", v.Home.Name, rep.HomeGoals, rep.AwayGoals, v.Away.Name, seed, rep.Ticks, float64(rep.Ticks)/float64(rep.TPS)/60)
	fmt.Printf("  tembakan %d (%d) vs %d (%d) | foul %d vs %d | kuning %d/%d merah %d/%d | corner %d vs %d | offside %d vs %d\n",
		s[0].Shots, s[0].OnTarget, s[1].Shots, s[1].OnTarget, s[0].Fouls, s[1].Fouls, s[0].Yellow, s[1].Yellow, s[0].Red, s[1].Red,
		s[0].Corners, s[1].Corners, s[0].Offsides, s[1].Offsides)
	fmt.Println("  replay:", out, "| config", rep.ConfigHash[:12])
}
