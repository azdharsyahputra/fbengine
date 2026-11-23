// EngineConfig (ours): every gameplay-affecting switch and tuning value of a
// match, owned by that match. Defaults reproduce the go4 engine exactly.
// Environment variables are not read here: the CLI parses them and hands the
// strings to ApplyOverrides (same keys and semantics as the old FMENGINE_*
// init() parsers).
package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ModuleConfig switches the FootballEngine modules and our M10 layer
// (formerly the package variables FeM1Defence … FeM10Structure).
type ModuleConfig struct {
	M1Defence    bool `json:"m1_defence"`
	M2Decision   bool `json:"m2_decision"`
	M3Steering   bool `json:"m3_steering"`
	M4Shape      bool `json:"m4_shape"`
	M5Collective bool `json:"m5_collective"`
	M6Challenge  bool `json:"m6_challenge"`
	M8Planner    bool `json:"m8_planner"`
	M9Restart    bool `json:"m9_restart"`
	M10Structure bool `json:"m10_structure"`
}

// M2Config is the M2 race/value calibration (formerly feM2*V variables).
// Notes carried over from fe_m2_race.go (the former variable block):
//
//	FE ControlRadius 2.5. The live first touch reaches 1.0 m (feet);
//	a receiver stretches a little beyond.
//	  → feM2ControlRadius = 1.5
//	An interceptor's reach without moving; the race starts one step after
//	the strike, so a pass played away from a man in contact is on.
//	  → feM2InterceptRadius = 1.25
//	An opponent reads the pass after it is struck; the receiver expects it.
//	  → feM2ReactionDelay = 0.1
//	A lofted ball must be controlled at chest or head height by a man
//	facing away from the defender (steps of race margin).
//	  → feM2AerialPenalty = 20.0
//	Share of FE held()'s loss charged to the receiver of a pass (FE prices
//	the bare threat at the receiver; a marked receiver then loses some).
//	  → feM2ReceiverLoss = 0.3
//	Share of Player12's random sidespin kept on M2 ground passes. Player12
//	bends a ground pass up to 0.17 rad/s at random, 2-4 m off its line over
//	25 m, which no race (and no passer) can price.
//	  → feM2CurveShare = 0.2
//	Player12's shield as an M2 option. FE has none (its carry is the duel);
//	standing still in contact fed the duel ping-pong.
//	  → feM2Shield = false
//	A shot's value relative to its xG (FE: 1): with the calibrated threat
//	field a shot at xG 1 was taken only inside ~10 m (3-5 shots a team).
//	  → feM2ShotValueScale = 1.7
type M2Config struct {
	ControlRadius     float64 `json:"control_radius"`      // cr
	InterceptRadius   float64 `json:"intercept_radius"`    // ir
	ReactionDelay     float64 `json:"reaction_delay"`      // rd
	AerialPenalty     float64 `json:"aerial_penalty"`      // ap
	ReceiverLoss      float64 `json:"receiver_loss"`       // rl
	CurveShare        float64 `json:"curve_share"`         // cv
	Shield            bool    `json:"shield"`              // sh
	ShotValueScale    float64 `json:"shot_value_scale"`    // ss
	ThreatPeak        float64 `json:"threat_peak"`         // tp
	ThreatDecay       float64 `json:"threat_decay"`        // td
	PassMinimumMargin int     `json:"pass_minimum_margin"` // mm
	LaneClearRadius   float64 `json:"lane_clear_radius"`   // lr
	PassMarginBias    float64 `json:"pass_margin_bias"`    // mb
	LoftedCeiling     float64 `json:"lofted_ceiling"`      // lc
	CarryPressure     float64 `json:"carry_pressure"`      // cp
	DuelLoss          float64 `json:"duel_loss"`           // dw
}

// M1Config is the M1 defensive-task geometry (Phase 2).
type M1Config struct {
	// ContainDistance: metres goal-side of the predicted ball that a Contain
	// holds (jockeying); the M6 challenge decides when he steps in. go4 used
	// the press point (FE ApproachOffset 0.75 m), i.e. on the ball.
	ContainDistance float64 `json:"contain_distance"` // cd
}

// M8Config is the M8 planner calibration.
type M8Config struct {
	Confidence  float64 `json:"confidence"`   // mc
	ExpandCarry bool    `json:"expand_carry"` // mx
}

// M10Config is our attack-structure / calibration layer (fe_m10_structure.go).
type M10Config struct {
	NearDepth     float64 `json:"near_depth"`     // nd
	NearWidth     float64 `json:"near_width"`     // nw
	BoxGoalSide   float64 `json:"box_goal_side"`  // bg
	GoalSide      float64 `json:"goal_side"`      // gs
	TightWeight   float64 `json:"tight_weight"`   // tw
	RetreatFwd    float64 `json:"retreat_fwd"`    // rf
	RetreatMid    float64 `json:"retreat_mid"`    // rm
	BlockLift     float64 `json:"block_lift"`     // bl
	BlockLength   float64 `json:"block_length"`   // bn
	ProgressOwn   float64 `json:"progress_own"`   // pa
	ProgressMid   float64 `json:"progress_mid"`   // pm
	ProgressFinal float64 `json:"progress_final"` // pf
}

// EngineConfig is the complete gameplay configuration of one match.
type EngineConfig struct {
	Modules ModuleConfig `json:"modules"`
	M1      M1Config     `json:"m1"`
	M2      M2Config     `json:"m2"`
	M8      M8Config     `json:"m8"`
	M10     M10Config    `json:"m10"`
}

// DefaultConfig returns the go4 defaults (identical to the former package
// variable initialisers).
func DefaultConfig() EngineConfig {
	return EngineConfig{
		Modules: ModuleConfig{
			M1Defence: true, M2Decision: true, M3Steering: true, M4Shape: true,
			M5Collective: true, M6Challenge: true, M8Planner: false, M9Restart: true,
			M10Structure: true,
		},
		M2: M2Config{
			ControlRadius:     1.5,
			InterceptRadius:   1.25,
			ReactionDelay:     0.1,
			AerialPenalty:     20.0,
			ReceiverLoss:      0.3,
			CurveShare:        0.2,
			Shield:            false,
			ShotValueScale:    1.7,
			ThreatPeak:        feM2ThreatPeak,
			ThreatDecay:       feM2ThreatDecay,
			PassMinimumMargin: -6,
			LaneClearRadius:   feM2LaneClearRadius,
			PassMarginBias:    feM2PassMarginBias,
			LoftedCeiling:     feM2LoftedCeiling,
			CarryPressure:     feM2CarryPressurePenalty,
			DuelLoss:          0.3,
		},
		M1: M1Config{ContainDistance: feM1ApproachOffset},
		M8: M8Config{Confidence: feM8Confidence, ExpandCarry: true},
		M10: M10Config{
			NearDepth: -0.06, NearWidth: 0.14,
			BoxGoalSide: 1.5, GoalSide: 1.5, TightWeight: 0.7,
			RetreatFwd: 0.0, RetreatMid: 6.0,
			BlockLift: 0.0, BlockLength: 0.0,
			ProgressOwn: 0.0, ProgressMid: 0.0005, ProgressFinal: 0.0012,
		},
	}
}

// JSON is the canonical serialisation (struct field order, Go's shortest
// float formatting), used for the replay metadata and the hash.
func (c EngineConfig) JSON() []byte {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err) // plain struct of bools/numbers: cannot fail
	}
	return b
}

// Hash is the sha256 of the canonical JSON, hex encoded.
func (c EngineConfig) Hash() string {
	sum := sha256.Sum256(c.JSON())
	return hex.EncodeToString(sum[:])
}

// ApplyOverrides applies the override strings (the former FMENGINE_ON,
// FMENGINE_OFF, FMENGINE_M2TUNE and FMENGINE_M10 variables, plus FMENGINE_M1)
// with the same semantics as the old init() parsers: ON before OFF, unknown
// keys and unparsable values ignored. It returns the keys it ignored so the
// CLI can warn about typos.
func ApplyOverrides(c *EngineConfig, on, off, m2tune, m10 string, more ...string) (ignored []string) {
	if len(more) > 0 {
		eachKV(more[0], func(k string, f float64) bool {
			switch k {
			case "cd":
				c.M1.ContainDistance = f
			default:
				return false
			}
			return true
		}, &ignored, "M1:")
	}
	for _, m := range strings.Split(on, ",") {
		if strings.TrimSpace(strings.ToUpper(m)) == "M8" {
			c.Modules.M8Planner = true
		} else if strings.TrimSpace(m) != "" {
			ignored = append(ignored, "ON:"+m)
		}
	}
	md := &c.Modules
	for _, m := range strings.Split(off, ",") {
		switch strings.TrimSpace(strings.ToUpper(m)) {
		case "M1":
			md.M1Defence = false
		case "M2":
			md.M2Decision = false
		case "M3":
			md.M3Steering = false
		case "M4":
			md.M4Shape = false
		case "M5":
			md.M5Collective = false
		case "M6":
			md.M6Challenge = false
		case "M8":
			md.M8Planner = false
		case "M9":
			md.M9Restart = false
		case "M10":
			md.M10Structure = false
		case "ALL":
			md.M1Defence, md.M2Decision, md.M3Steering, md.M4Shape, md.M5Collective, md.M6Challenge = false, false, false, false, false, false
			md.M9Restart = false
			md.M8Planner = false
			md.M10Structure = false
		case "":
		default:
			ignored = append(ignored, "OFF:"+m)
		}
	}
	m2 := &c.M2
	eachKV(m2tune, func(k string, f float64) bool {
		switch k {
		case "cr":
			m2.ControlRadius = f
		case "ir":
			m2.InterceptRadius = f
		case "rd":
			m2.ReactionDelay = f
		case "cv":
			m2.CurveShare = f
		case "mc":
			c.M8.Confidence = f
		case "mx":
			c.M8.ExpandCarry = f != 0
		case "sh":
			m2.Shield = f != 0
		case "mm":
			m2.PassMinimumMargin = int(f)
		case "lr":
			m2.LaneClearRadius = f
		case "mb":
			m2.PassMarginBias = f
		case "lc":
			m2.LoftedCeiling = f
		case "ss":
			m2.ShotValueScale = f
		case "dw":
			m2.DuelLoss = f
		case "cp":
			m2.CarryPressure = f
		case "rl":
			m2.ReceiverLoss = f
		case "tp":
			m2.ThreatPeak = f
		case "td":
			m2.ThreatDecay = f
		case "ap":
			m2.AerialPenalty = f
		default:
			return false
		}
		return true
	}, &ignored, "M2TUNE:")
	m := &c.M10
	eachKV(m10, func(k string, f float64) bool {
		switch k {
		case "nd":
			m.NearDepth = f
		case "nw":
			m.NearWidth = f
		case "bg":
			m.BoxGoalSide = f
		case "gs":
			m.GoalSide = f
		case "tw":
			m.TightWeight = f
		case "pa":
			m.ProgressOwn = f
		case "pm":
			m.ProgressMid = f
		case "pf":
			m.ProgressFinal = f
		case "bl":
			m.BlockLift = f
		case "bn":
			m.BlockLength = f
		case "rf":
			m.RetreatFwd = f
		case "rm":
			m.RetreatMid = f
		default:
			return false
		}
		return true
	}, &ignored, "M10:")
	return ignored
}

// eachKV walks "k=v,k=v" pairs; pairs without "=" or with a non-numeric
// value are skipped silently (as before), unknown keys are reported.
func eachKV(s string, set func(k string, f float64) bool, ignored *[]string, prefix string) {
	for _, kv := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			continue
		}
		if !set(k, f) {
			*ignored = append(*ignored, fmt.Sprintf("%s%s", prefix, k))
		}
	}
}
