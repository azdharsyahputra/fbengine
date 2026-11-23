// Command match runs one match from our CSV data and writes a viewer replay.
//
//	go run ./cmd/match [--config=cfg.json|replay.js] "Liverpool" "Man City" 19 [out.js] [formation]
//
// Configuration (the only place the FMENGINE_* environment is read):
//   - default: engine.DefaultConfig() plus the legacy overrides FMENGINE_ON,
//     FMENGINE_OFF, FMENGINE_M2TUNE, FMENGINE_M10 (engine.ApplyOverrides);
//   - --config=FILE: the EngineConfig in FILE (a config JSON, or a schema-2
//     replay's meta.config, hash-checked); the overrides are then ignored.
//
// Diagnostics (never change the match): FMENGINE_M2TRACE=1|2|3, FMENGINE_AUDIT=1.
//
// The replay is schema 2: the schema-1 payload (frames, events, cond) plus
// "live" (one '1'/'0' per sample: live play / dead ball) and "meta"
// (seed, tps, formation, data file hash, full EngineConfig, config hash).
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
	"strconv"
	"strings"

	"fmengine/data"
	"fmengine/engine"
)

const csvPath = "/Users/csadeveloper/fm/efem_epl_full.csv"

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
	Meta   vMeta  `json:"meta"`
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

func load(club string, id engine.TeamID, formation string) *data.Squad {
	rows, err := data.LoadClub(csvPath, club)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sq, err := data.BuildSquad(rows, id, formation)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return sq
}

// loadConfig reads an EngineConfig from a config JSON file or a schema-2
// replay (meta.config, whose hash must match).
func loadConfig(path string) (engine.EngineConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return engine.EngineConfig{}, err
	}
	raw = bytes.TrimSpace(raw)
	if r, ok := bytes.CutPrefix(raw, []byte("window.REPLAY=")); ok {
		var rep struct {
			Schema int    `json:"schema"`
			Meta   *vMeta `json:"meta"`
		}
		if err := json.Unmarshal(bytes.TrimSuffix(r, []byte(";")), &rep); err != nil {
			return engine.EngineConfig{}, err
		}
		if rep.Meta == nil {
			return engine.EngineConfig{}, fmt.Errorf("%s: schema-1 replay has no EngineConfig (written before configs were recorded); it cannot be reproduced exactly", path)
		}
		if h := rep.Meta.Config.Hash(); h != rep.Meta.ConfigHash {
			return engine.EngineConfig{}, fmt.Errorf("%s: config hash %s does not match the recorded %s (config fields changed since this replay)", path, h, rep.Meta.ConfigHash)
		}
		return rep.Meta.Config, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	cfg := engine.DefaultConfig()
	err = dec.Decode(&cfg)
	return cfg, err
}

func fileSHA256(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func main() {
	var configPath string
	args := []string{os.Args[0]}
	for _, a := range os.Args[1:] {
		if v, ok := strings.CutPrefix(a, "--config="); ok {
			configPath = v
		} else {
			args = append(args, a)
		}
	}
	os.Args = args
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: match [--config=cfg.json|replay.js] <home> <away> [seed] [out.js] [formation]")
		os.Exit(2)
	}
	cfg := engine.DefaultConfig()
	if configPath != "" {
		var err error
		if cfg, err = loadConfig(configPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
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
	diag := engine.Diagnostics{Audit: os.Getenv("FMENGINE_AUDIT") != "", Out: os.Stderr}
	if t := os.Getenv("FMENGINE_M2TRACE"); t != "" {
		diag.M2Trace, _ = strconv.Atoi(t)
		if diag.M2Trace == 0 {
			diag.M2Trace = 1 // any non-numeric value enabled level 1 before
		}
	}
	seed := uint64(1)
	if len(os.Args) > 3 {
		seed, _ = strconv.ParseUint(os.Args[3], 10, 32)
	}
	out := fmt.Sprintf("/Users/csadeveloper/fm/fm-game/viewer/replay_go_s%d.js", seed)
	if len(os.Args) > 4 && os.Args[4] != "" {
		out = os.Args[4]
	}
	formation := "4-2-3-1"
	if len(os.Args) > 5 {
		formation = os.Args[5]
	}
	home := load(os.Args[1], 1, formation)
	away := load(os.Args[2], 2, formation)
	rep, err := engine.RunReplayWith(home.Lineup, away.Lineup, home.Club, away.Club, uint32(seed), 4,
		engine.RunOptions{Config: cfg, Diagnostics: diag})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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
	v.Meta = vMeta{Schema: engine.ReplaySchema, Engine: "fm-engine-go", Seed: rep.Seed, TPS: rep.TPS,
		Home: home.Club, Away: away.Club, Formation: formation,
		Data: vData{CSV: csvPath, SHA256: fileSHA256(csvPath)}, Config: rep.Config, ConfigHash: rep.ConfigHash,
		LiveRule: "engine state == PLAYING after the step that produced the sample"}
	for _, e := range rep.Events {
		v.Events = append(v.Events, vEvent{e.T, e.Minute, e.Kind, e.Team, e.Text})
	}
	js, _ := json.Marshal(v)
	if err := os.WriteFile(out, append(append([]byte("window.REPLAY="), js...), ';', '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	s := rep.Stats
	fmt.Fprint(os.Stderr, rep.Audit)
	fmt.Printf("%s %d-%d %s (seed %d, %d sampel = %.1f menit)\n", home.Club, rep.HomeGoals, rep.AwayGoals, away.Club, seed, rep.Ticks, float64(rep.Ticks)/float64(rep.TPS)/60)
	fmt.Printf("  tembakan %d (%d) vs %d (%d) | foul %d vs %d | kuning %d/%d merah %d/%d | corner %d vs %d | offside %d vs %d\n",
		s[0].Shots, s[0].OnTarget, s[1].Shots, s[1].OnTarget, s[0].Fouls, s[1].Fouls, s[0].Yellow, s[1].Yellow, s[0].Red, s[1].Red,
		s[0].Corners, s[1].Corners, s[0].Offsides, s[1].Offsides)
	fmt.Println("  replay:", out, "| config", rep.ConfigHash[:12])
}
