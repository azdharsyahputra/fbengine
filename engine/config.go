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
	// TeamShape: the Team Shape Controller + Position Composer own the targets
	// in live play (team_shape.go, team_shape_compose.go); off = go16.
	TeamShape bool `json:"team_shape"`
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
	PassMarginBias    float64 `json:"pass_margin_bias"`    // mb
	LoftedCeiling     float64 `json:"lofted_ceiling"`      // lc
	CarryPressure     float64 `json:"carry_pressure"`      // cp
	DuelLoss          float64 `json:"duel_loss"`           // dw
	// Progress appetite (goal-probability units per metre a pass gains),
	// by the carrier's third: own / middle / final (M10 keys pa/pm/pf).
	ProgressOwn   float64 `json:"progress_own"`
	ProgressMid   float64 `json:"progress_mid"`
	ProgressFinal float64 `json:"progress_final"`
	// ToFeet: retry a pass cut off in the lead as a pass to feet.
	ToFeet bool `json:"to_feet"`
}

// M1Config is the M1 defensive-task geometry (Phase 2).
type M1Config struct {
	// ContainDistance: metres goal-side of the predicted ball that a Contain
	// holds (jockeying); the M6 challenge decides when he steps in. go4 used
	// the press point (FE ApproachOffset 0.75 m), i.e. on the ball.
	ContainDistance float64 `json:"contain_distance"` // cd
	// ContainWindow: outside the engage distance a Contain is sent only at a
	// carrier someone reaches within this many seconds (FE local window 2 s,
	// go4-go12; go13 4 s: carriers in midfield space get a defender out).
	ContainWindow float64 `json:"contain_window"` // cw
	// RecoverClearance: a bypassed player's recovery run keeps this many
	// metres off the ball's line (0: straight at his spot, go9).
	RecoverClearance float64 `json:"recover_clearance"` // rc
	// ForwardBypassedEffort: recovery effort (0 jog … 1 sprint) of a forward
	// the ball has passed, unless the ball is within BypassedSprintGoal metres
	// of his own goal (then he sprints).
	ForwardBypassedEffort float64 `json:"forward_bypassed_effort"` // fb
	BypassedSprintGoal    float64 `json:"bypassed_sprint_goal"`    // fg
	// RestGap: the least lateral gap (m) between the two ProtectDepth
	// defenders; they spread about their midpoint, each keeping his side
	// (0: none, go4-go13 — with no runner both aimed at the same spot; go14 9).
	RestGap float64 `json:"rest_gap"` // rg
	// FullBackCross: how far (m) past the pitch's centre line a full-back on
	// shape duty (ProtectDepth / RecoverShape) may go toward the far side;
	// he tucks in, he does not swap flanks (99: unlimited, go4-go13; go14 -2,
	// i.e. he stays 2 m on his own side).
	FullBackCross float64 `json:"full_back_cross"` // fc
}

// LooseConfig is the single loose-ball resolver (Phase 2, loose_ball.go):
// who contests a ball nobody controls, and how much the team that last had it
// is drawn toward it.
type LooseConfig struct {
	// ContestGap: a side whose fastest player reaches the ball's control point
	// this many seconds after the other side's does not sprint at it; he
	// delays goal-side instead. < 0 disables (go5: both sides always chase).
	ContestGap float64 `json:"contest_gap"` // lg
	// DelayDistance: metres goal-side of the control point the delayer holds.
	DelayDistance float64 `json:"delay_distance"` // ld
	// SupportPull: share of the way the last side's supporters are pulled
	// toward the ball's y while it is loose (go5: Player12's 0.12).
	SupportPull float64 `json:"support_pull"` // lp
}

// PressConfig is M1's organised press (Phase 2 / C3): when the opponent has
// the ball deep in its own half, the defending side engages the carrier and
// its forwards and midfielders step up instead of holding the block shape.
type PressConfig struct {
	// Depth: team-frame x (FE metres, + toward the opponent's goal) of the
	// ball beyond which the organised press applies; >= 60 disables it.
	Depth float64 `json:"depth"` // pz
	// Window: seconds within which someone must reach the carrier to be sent
	// (the Contain fallback's window outside the press zone is m1.contain_window).
	Window float64 `json:"window"` // pw
	// ForwardGap / MidGap: forwards / midfielders on shape duty stand no more
	// than this many metres behind the ball.
	ForwardGap float64 `json:"forward_gap"` // pg
	MidGap     float64 `json:"mid_gap"`     // pn
	// Effort: the presser's approach effort (0 jog … 1 sprint).
	Effort float64 `json:"effort"` // pe
}

// M3Config is movement execution (Phase 2).
type M3Config struct {
	// CatchUpShare: share of top speed (0 off) at which a settled supporter /
	// holder moves to a spot more than CatchUpDistance metres ahead of him in
	// the attacking direction (otherwise settled moves walk-jog at ~0.19).
	CatchUpShare    float64 `json:"catch_up_share"`    // cs
	CatchUpDistance float64 `json:"catch_up_distance"` // cx
	// CatchUpRun: the catch-up and full-back push runs are runs, not settled
	// adjustments, so the adjustment pace cap (0.9 + 0.2 m/s per metre of
	// run) does not apply, the catch-up reads the tactical spot and also
	// applies to attacking-transition support (false: go4-go14, they moved
	// at ~2-3 m/s; go15 true).
	CatchUpRun bool `json:"catch_up_run"` // cr (0/1)
}

// M8Config is the M8 planner calibration.
type M8Config struct {
	Confidence  float64 `json:"confidence"`   // mc
	ExpandCarry bool    `json:"expand_carry"` // mx
}

// M10Config is our attack-structure / calibration layer (fe_m10_structure.go).
type M10Config struct {
	NearDepth   float64 `json:"near_depth"`    // nd
	NearWidth   float64 `json:"near_width"`    // nw
	BoxGoalSide float64 `json:"box_goal_side"` // bg
	GoalSide    float64 `json:"goal_side"`     // gs
	TightWeight float64 `json:"tight_weight"`  // tw
	RetreatFwd  float64 `json:"retreat_fwd"`   // rf
	RetreatMid  float64 `json:"retreat_mid"`   // rm
	BlockLift   float64 `json:"block_lift"`    // bl
	BlockLength float64 `json:"block_length"`  // bn
	// OverlapLead: the overlapping full-back's spot is at least this far (m)
	// ahead of the ball, in the touchline lane, onside, when the ball is on
	// his flank (0: his shape spot + 18 m, go4-go14; go15 6).
	OverlapLead float64 `json:"overlap_lead"` // ol
	// PossessionStep: with the ball, the holders' block moves up by this
	// share of the ball's progress past a quarter of the pitch (Player12
	// Shape_POSSESSION_BLOCK_PROGRESS 0.24, go4-go15; go16 1.6).
	PossessionStep float64 `json:"possession_step"` // ps
	// PossessionStepMax: at most this many metres of that step (0: no cap,
	// go4-go15; go16 30), so the back line joins the circulation in midfield without
	// standing at the halfway line when the ball is in the final third.
	PossessionStepMax float64 `json:"possession_step_max"` // pq
	// FullBackTrail: in the opponent half the ball-side full-back on shape
	// duty is at most this far (m) behind the ball (12: go4-go14).
	FullBackTrail float64 `json:"full_back_trail"` // ft (go15 2)
	// FullBackWeak: the weak-side full-back at most this far (m) behind the
	// ball (22: go4-go14; go15 12).
	FullBackWeak float64 `json:"full_back_weak"` // fw
	// FullBackStart: team-frame ball x (m) from which the full-backs join
	// (-5: go4-go14; go15 -20).
	FullBackStart float64 `json:"full_back_start"` // fz
}

// EngineConfig is the complete gameplay configuration of one match.
type EngineConfig struct {
	Modules ModuleConfig `json:"modules"`
	M1      M1Config     `json:"m1"`
	Loose   LooseConfig  `json:"loose"`
	Press   PressConfig  `json:"press"`
	M2      M2Config     `json:"m2"`
	M3      M3Config     `json:"m3"`
	M8      M8Config     `json:"m8"`
	M10     M10Config    `json:"m10"`
	// TeamShape: the composer's rules (team_shape_compose.go).
	TeamShape TeamShapeConfig `json:"team_shape"`
}

// TeamShapeConfig: the Position Composer's rules (docs/PLAN-team-shape.md §4.2),
// swept together with the scorecard, never per metric.
type TeamShapeConfig struct {
	RunQuota     int     `json:"run_quota"`     // rq: runs in behind per side
	OverlapQuota int     `json:"overlap_quota"` // oq
	BoxQuota     int     `json:"box_quota"`     // bq: box runs / arrivals
	PressQuota   int     `json:"press_quota"`   // xq: extra pressers (doubling)
	MinSpacing   float64 `json:"min_spacing"`   // sm: metres between non-ball-job teammates
	SupportDev   float64 `json:"support_dev"`   // sd: support / short options from the slot
	CoverDev     float64 `json:"cover_dev"`     // vd: cover, screen, lane cover, marks
	DutyDev      float64 `json:"duty_dev"`      // dd: M5 duties, transition support
	WalkSpeed    float64 `json:"walk_speed"`    // ws (m/s): slot holder, target < JogDistance
	JogSpeed     float64 `json:"jog_speed"`     // js
	RunSpeed     float64 `json:"run_speed"`     // ru: target > RunDistance
	JogDistance  float64 `json:"jog_distance"`  // jd (m)
	RunDistance  float64 `json:"run_distance"`  // rd (m)
	// TrotSpeed (m/s): a defender the ball has bypassed (it is between him and
	// his goal) recovers at a trot unless the ball is within M1.BypassedSprintGoal
	// of his goal (go10's rule, now in the composer).
	TrotSpeed float64 `json:"trot_speed"` // tt
	// CloseTime (s): a slot/offset target's gap is closed in this time on top of
	// the target's own speed (feed-forward).
	CloseTime float64 `json:"close_time"` // ct
	// AnchorTime (s): the shape follows the ball through this time constant.
	AnchorTime float64 `json:"anchor_time"` // at
	// DepthDev (m): M1 protect depth may drop this far from the slot (runners).
	DepthDev float64 `json:"depth_dev"` // pd
	// DangerDepth (m): ball this close to the own goal line = M1 recover shape keeps its
	// marking (offset) instead of the slot.
	DangerDepth float64 `json:"danger_depth"` // dg
	// MarkZone (m): ... and only for a player whose slot is this close to his goal line.
	MarkZone float64 `json:"mark_zone"` // mz
	// ShiftSpeed (m/s): a slot/offset holder within RunDistance of his target moves at
	// most this fast (the block shuffles; it does not sprint with every pass).
	ShiftSpeed float64 `json:"shift_speed"` // sh
	// WalkZone (m): within this of the target a slot/offset holder walks.
	WalkZone float64 `json:"walk_zone"` // wz
	// NativeDefence: the defending side's jobs come from the Team Shape Controller
	// (first defender, cover, markers, back line), not from M1's tasks.
	NativeDefence bool `json:"native_defence"` // nv (0/1)
	// KeepTime (s): the engaged first defender keeps the job unless another is this much faster.
	KeepTime float64 `json:"keep_time"` // kt
	// EngageDepth (m): within this of the own goal line a first defender always engages.
	EngageDepth float64 `json:"engage_depth"` // ed
	// CoverGap (m): the cover stands this far behind the first defender, toward the goal.
	CoverGap float64 `json:"cover_gap"` // cg
	// MarkRadius (m): near goal, a slot-holder this close to an attacker marks him.
	MarkRadius float64 `json:"mark_radius"` // mr
	// RunnerSpeed (m/s): an attacker running at the goal this fast past a back-line slot
	// makes that slot-holder drop with him.
	RunnerSpeed float64 `json:"runner_speed"` // rv
	// PressOption (m): in the press, the carrier's nearest option within this is screened.
	PressOption float64 `json:"press_option"` // po
	// DefRunSpeed (m/s): a defender more than RunDistance from his slot returns at most this fast.
	DefRunSpeed float64 `json:"def_run_speed"` // dv
	// NearBall (m): attackers this close to the ball are marked by the nearest slot-holder.
	NearBall float64 `json:"near_ball"` // nb
	// MarkGoalSide (m): a marker stands this far goal-side of his man.
	MarkGoalSide float64 `json:"mark_goal_side"` // mg
	// CounterPress (s): after losing the ball the side presses as in the press zone for
	// this long (away from its own goal).
	CounterPress float64 `json:"counter_press"` // cp
	// PressMarkRadius (m): in the press zone, attackers this close to the ball are marked.
	PressMarkRadius float64 `json:"press_mark_radius"` // pr
	// PassTrigger: with an opponent pass in flight the first defender goes for its
	// intended receiver (he leaves with the pass, not after the control).
	PassTrigger bool `json:"pass_trigger"` // pt (0/1)
	// MarkBehind (m): a marker may stand at most this far beyond his man (attack
	// direction); 99 = any (a bypassed player may run back to mark).
	MarkBehind float64 `json:"mark_behind"` // mb
	// SpeedTable: non-action moves are capped at the real players' speed percentiles per
	// role group and phase (team_shape_speed.go, from Metrica).
	SpeedTable bool `json:"speed_table"` // sq (0/1)
}

// DefaultConfig returns the go4 defaults (identical to the former package
// variable initialisers).
func DefaultConfig() EngineConfig {
	return EngineConfig{
		Modules: ModuleConfig{
			M1Defence: true, M2Decision: true, M3Steering: true, M4Shape: true,
			M5Collective: true, M6Challenge: true, M8Planner: false, M9Restart: true,
			M10Structure: true, TeamShape: false,
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
			PassMarginBias:    feM2PassMarginBias,
			LoftedCeiling:     feM2LoftedCeiling,
			CarryPressure:     feM2CarryPressurePenalty,
			DuelLoss:          0.5, // go11 (go4-go10: 0.3): carriers avoid contact, fouls 18.8 → 15.9
			ProgressOwn:       0.0,
			ProgressMid:       0.0005,
			ProgressFinal:     0.003, // go11 (go4-go10: 0.0012): backward 27 → 24 % (real 24)
			ToFeet:            true,
		},
		// Phase 2: a jockeying Contain 2 m goal-side (go4: feM1ApproachOffset,
		// 0.75 m = on the ball). 5 seeds: cek ping-pong 34→20, chase≥3
		// 10.6→8.4, loose ≥5 11.6→9.0, fouls 24→21 per side.
		M1: M1Config{ContainDistance: 2.0, ContainWindow: 4.0,
			// Phase 2 D (go10): a bypassed forward trots back (effort 0, ~2 m/s)
			// and sprints only with the ball within 25 m of his goal (go9: 0.6
			// and 40 m). 20 seeds: chase≥3 10.1 → 5.2 % (real 5.8).
			ForwardBypassedEffort: 0, BypassedSprintGoal: 25,
			// go14: back-line spacing and flank discipline.
			RestGap: 9, FullBackCross: -2},
		// Phase 2 C3 (go9): press when the ball is in the opponent's third,
		// engaging a carrier someone reaches within 6 s. The block step-up
		// (ForwardGap/MidGap) stays off: it raised chase≥3 to 11.6-15.
		// go12: settled supporters catch up with the attack at 0.6 of top speed
		// (go11: 0 → the pivot trailed a final-third attack by ~29 m).
		M3: M3Config{CatchUpShare: 0.6, CatchUpDistance: 8, CatchUpRun: true},
		TeamShape: TeamShapeConfig{RunQuota: 2, OverlapQuota: 1, BoxQuota: 2, PressQuota: 1,
			MinSpacing: 8, SupportDev: 10, CoverDev: 8, DutyDev: 10,
			WalkSpeed: 1.6, JogSpeed: 3.2, RunSpeed: 5.5, JogDistance: 4, RunDistance: 12, CloseTime: 2.5,
			DangerDepth: 40, MarkZone: 30, ShiftSpeed: 2.4, WalkZone: 3,
			NativeDefence: true, KeepTime: 0.75, EngageDepth: 35, CoverGap: 6, MarkRadius: 12, RunnerSpeed: 3.5, PressOption: 20, DefRunSpeed: 4.5, NearBall: 25, MarkGoalSide: 2, CounterPress: 4, PressMarkRadius: 40, PassTrigger: true, MarkBehind: 99,
			TrotSpeed: 2.6, AnchorTime: 3, DepthDev: 5},
		Press: PressConfig{Depth: 17.5, Window: 6.0, ForwardGap: 99, MidGap: 99, Effort: 1.0},
		Loose: LooseConfig{ContestGap: 0.25, DelayDistance: 4.0, SupportPull: Shape_ATTACKING_TRANSITION_BALL_PULL},
		M8:    M8Config{Confidence: feM8Confidence, ExpandCarry: true},
		M10: M10Config{
			NearDepth: -0.06, NearWidth: 0.14,
			BoxGoalSide: 1.5, GoalSide: 1.5, TightWeight: 0.7,
			RetreatFwd: 0.0, RetreatMid: 6.0,
			BlockLift: 0.0, BlockLength: 0.0,
			// go15: full-backs join the attack from 20 m in their own half,
			// ball side level with the ball, weak side 12 m behind; an overlap
			// goes 6 m past the ball.
			FullBackTrail: 2.0, FullBackWeak: 12.0, FullBackStart: -20.0, OverlapLead: 6.0,
			// go16: the holders' block steps up with the ball (1.6 of its
			// progress past a quarter, at most 30 m; P12 0.24 uncapped).
			PossessionStep: 1.6, PossessionStepMax: 30,
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
			case "cw":
				c.M1.ContainWindow = f
			case "cs":
				c.M3.CatchUpShare = f
			case "cx":
				c.M3.CatchUpDistance = f
			case "rq":
				c.TeamShape.RunQuota = int(f)
			case "oq":
				c.TeamShape.OverlapQuota = int(f)
			case "bq":
				c.TeamShape.BoxQuota = int(f)
			case "xq":
				c.TeamShape.PressQuota = int(f)
			case "sm":
				c.TeamShape.MinSpacing = f
			case "sd":
				c.TeamShape.SupportDev = f
			case "vd":
				c.TeamShape.CoverDev = f
			case "dd":
				c.TeamShape.DutyDev = f
			case "ws":
				c.TeamShape.WalkSpeed = f
			case "js":
				c.TeamShape.JogSpeed = f
			case "ru":
				c.TeamShape.RunSpeed = f
			case "jd":
				c.TeamShape.JogDistance = f
			case "rd":
				c.TeamShape.RunDistance = f
			case "tt":
				c.TeamShape.TrotSpeed = f
			case "ct":
				c.TeamShape.CloseTime = f
			case "at":
				c.TeamShape.AnchorTime = f
			case "pd":
				c.TeamShape.DepthDev = f
			case "dg":
				c.TeamShape.DangerDepth = f
			case "mz":
				c.TeamShape.MarkZone = f
			case "sh":
				c.TeamShape.ShiftSpeed = f
			case "wz":
				c.TeamShape.WalkZone = f
			case "nv":
				c.TeamShape.NativeDefence = f != 0
			case "kt":
				c.TeamShape.KeepTime = f
			case "ed":
				c.TeamShape.EngageDepth = f
			case "cg":
				c.TeamShape.CoverGap = f
			case "mr":
				c.TeamShape.MarkRadius = f
			case "rv":
				c.TeamShape.RunnerSpeed = f
			case "po":
				c.TeamShape.PressOption = f
			case "dv":
				c.TeamShape.DefRunSpeed = f
			case "nb":
				c.TeamShape.NearBall = f
			case "mg":
				c.TeamShape.MarkGoalSide = f
			case "cp":
				c.TeamShape.CounterPress = f
			case "pr":
				c.TeamShape.PressMarkRadius = f
			case "pt":
				c.TeamShape.PassTrigger = f != 0
			case "mb":
				c.TeamShape.MarkBehind = f
			case "sq":
				c.TeamShape.SpeedTable = f != 0
			case "cr":
				c.M3.CatchUpRun = f != 0
			case "rc":
				c.M1.RecoverClearance = f
			case "fb":
				c.M1.ForwardBypassedEffort = f
			case "fc":
				c.M1.FullBackCross = f
			case "rg":
				c.M1.RestGap = f
			case "fg":
				c.M1.BypassedSprintGoal = f
			case "lg":
				c.Loose.ContestGap = f
			case "ld":
				c.Loose.DelayDistance = f
			case "lp":
				c.Loose.SupportPull = f
			case "pz":
				c.Press.Depth = f
			case "pw":
				c.Press.Window = f
			case "pg":
				c.Press.ForwardGap = f
			case "pn":
				c.Press.MidGap = f
			case "pe":
				c.Press.Effort = f
			default:
				return false
			}
			return true
		}, &ignored, "M1:")
	}
	for _, m := range strings.Split(on, ",") {
		if strings.TrimSpace(strings.ToUpper(m)) == "M8" {
			c.Modules.M8Planner = true
		} else if strings.TrimSpace(strings.ToUpper(m)) == "TS" {
			c.Modules.TeamShape = true
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
		case "TS":
			md.TeamShape = false
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
			c.M2.ProgressOwn = f
		case "pm":
			c.M2.ProgressMid = f
		case "pf":
			c.M2.ProgressFinal = f
		case "bl":
			m.BlockLift = f
		case "bn":
			m.BlockLength = f
		case "rf":
			m.RetreatFwd = f
		case "rm":
			m.RetreatMid = f
		case "ol":
			m.OverlapLead = f
		case "ps":
			m.PossessionStep = f
		case "pq":
			m.PossessionStepMax = f
		case "ft":
			m.FullBackTrail = f
		case "fw":
			m.FullBackWeak = f
		case "fz":
			m.FullBackStart = f
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
