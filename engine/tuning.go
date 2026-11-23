// Ported from Player12 src/model/match_tuning.h (GPL-3.0)

package engine

// Tuning convention
//
// C++ `MatchTuning` is a struct of nested structs holding `static constexpr`
// values. In Go every value is a package-level typed constant named
//
//	<Group>_<CPP_NAME>
//
// i.e. `MatchTuning::Shooting::ERROR_BASE_METRES` becomes
// `Shooting_ERROR_BASE_METRES` (the outer `MatchTuning::` is dropped and
// `::` becomes `_`; the C++ spelling of the group and of the constant is kept
// verbatim so ported formulas can be translated mechanically, including the
// common `using S = MatchTuning::Shooting; S::X` pattern -> `Shooting_X`).
//
// Types: float/double -> float64, int/long/std::size_t -> int,
// std::uint32_t -> uint32, std::uint64_t -> uint64. The two
// std::array<float, 3> values are package-level [3]float64 variables (Go has
// no array constants); treat them as read-only.
//
// Other tuning namespaces follow the same rule with their own prefix, e.g.
// `TacticsTuning::TIGHT_MARK_WEIGHT` -> `TacticsTuning_TIGHT_MARK_WEIGHT`
// (tactics.go) and `MatchTracking::GRID_COLUMNS` ->
// `MatchTracking_GRID_COLUMNS` (match_tracking.go).

// MatchTuning::Pitch
const (
	Pitch_LENGTH_METRES                  float64 = 105.0
	Pitch_WIDTH_METRES                   float64 = 68.0
	Pitch_CENTRE                         float64 = 0.5
	Pitch_GOAL_TOP                       float64 = 0.446
	Pitch_GOAL_BOTTOM                    float64 = 0.554
	Pitch_LINEUP_GOALKEEPER_X            float64 = 0.04
	Pitch_GOALKEEPER_MIN_Y               float64 = 0.38
	Pitch_GOALKEEPER_MAX_Y               float64 = 0.62
	Pitch_GOALKEEPER_SWEEP_DEPTH         float64 = 0.18
	Pitch_LEFT_GOAL_KICK_X               float64 = 0.065
	Pitch_RIGHT_GOAL_KICK_X              float64 = 0.935
	Pitch_LEFT_PENALTY_SPOT_X            float64 = 0.105
	Pitch_RIGHT_PENALTY_SPOT_X           float64 = 0.895
	Pitch_LEFT_PENALTY_AREA_EDGE         float64 = 0.157
	Pitch_RIGHT_PENALTY_AREA_EDGE        float64 = 0.843
	Pitch_PENALTY_AREA_HALF_WIDTH_METRES float64 = 20.16
	Pitch_RESTART_INSET                  float64 = 0.01
	Pitch_RESTART_LONGITUDINAL_MARGIN    float64 = 0.04
	Pitch_HOME_KICKOFF_X                 float64 = 0.49
	Pitch_AWAY_KICKOFF_X                 float64 = 0.51
	Pitch_KICKOFF_FORMATION_INSET        float64 = 0.02
	Pitch_KICKOFF_FORMATION_SCALE        float64 = 0.56
	Pitch_PLAYER_MIN_X                   float64 = 0.005
	Pitch_PLAYER_MAX_X                   float64 = 0.995
	Pitch_PLAYER_MIN_Y                   float64 = 0.01
	Pitch_PLAYER_MAX_Y                   float64 = 0.99
)

// MatchTuning::Timing
const (
	// Decision and body-physics step (10 Hz).
	Timing_FIXED_STEP_SECONDS float64 = 0.1
	// Team shape and off-ball targets refresh every this many steps (5 Hz),
	// and at once whenever possession or the ball's flight changes; bodies
	// move every step. Running load is accounted at the same rate.
	Timing_TACTICAL_REFRESH_STEPS uint64 = 2
	// The free ball is integrated in this many sub-steps per fixed step.
	Timing_BALL_SUBSTEPS int = 4
	// MatchFidelity::BACKGROUND merges up to this many fixed steps into one
	// while the ball is dead before a throw-in, goal kick, corner, free kick
	// or penalty (players only walk to their restart spots). Live play keeps
	// the 10 Hz step: a coarser one measurably changes shots and goals.
	Timing_BACKGROUND_STOPPAGE_TICKS       uint32  = 3
	Timing_SECONDS_PER_MINUTE              float64 = 60.0
	Timing_MIN_PLAY_HALF_MINUTES           int     = 3
	Timing_MAX_PLAY_HALF_MINUTES           int     = 15
	Timing_DEFAULT_PLAY_HALF_MINUTES       int     = 5
	Timing_PLAY_STOPPAGE_SECONDS           float64 = 3.0
	Timing_RESTART_SUPPORT_DISTANCE_METRES float64 = 30.0
	Timing_RESTART_SUPPORT_BLEND           float64 = 0.35
	Timing_THROW_OPPONENT_DISTANCE_METRES  float64 = 2.0
	Timing_KICK_OPPONENT_DISTANCE_METRES   float64 = 9.15
	// update() bounds catch-up work so a stalled render frame cannot freeze
	// the live view; remaining whole steps are dropped. Headless code uses
	// advance()/simulateToEnd(), which never drop steps.
	Timing_MAX_FRAME_DELTA_SECONDS    float64 = 4.0
	Timing_MAX_FIXED_STEPS_PER_UPDATE int     = 30
	Timing_HALF_TIME_MINUTE           float64 = 45.0
	Timing_FULL_TIME_MINUTE           float64 = 90.0
	// Knockout ties still level play two halves of extra time.
	Timing_EXTRA_TIME_HALF_MINUTES float64 = 15.0
	// Breaks before extra time and at its half-time (clock stopped).
	Timing_EXTRA_TIME_BREAK_SECONDS     float64 = 10.0
	Timing_EXTRA_TIME_HALF_TIME_SECONDS float64 = 5.0
	// Penalty shootout pacing: before the first kick, between kicks, and
	// the longest a kick can travel before it counts as missed.
	Timing_SHOOTOUT_START_SECONDS         float64 = 20.0
	Timing_SHOOTOUT_KICK_INTERVAL_SECONDS float64 = 12.0
	Timing_SHOOTOUT_MAX_FLIGHT_SECONDS    float64 = 3.0
	// Kicks per side before sudden death.
	Timing_SHOOTOUT_KICKS int = 5
	// Interval in the tunnel; the clock is stopped and nothing is live.
	Timing_HALF_TIME_PAUSE_SECONDS float64 = 10.0
	// Restart delays from the ball going dead to the restart (real averages:
	// throw-in ~18 s, goal kick ~30 s, corner ~37 s), slightly longer here
	// because fewer balls go out of play than in a real match.
	Timing_KICKOFF_DELAY_SECONDS             float64 = 4.0
	Timing_THROW_IN_DELAY_SECONDS            float64 = 21.0
	Timing_GOAL_KICK_DELAY_SECONDS           float64 = 33.0
	Timing_CORNER_DELAY_SECONDS              float64 = 40.0
	Timing_FREE_KICK_DELAY_SECONDS           float64 = 22.0
	Timing_SET_PIECE_FREE_KICK_DELAY_SECONDS float64 = 28.0
	Timing_PENALTY_DELAY_SECONDS             float64 = 55.0
	Timing_CARD_DELAY_SECONDS                float64 = 15.0
	Timing_INJURY_DELAY_SECONDS              float64 = 50.0
	Timing_SUBSTITUTION_DELAY_SECONDS        float64 = 9.0
	// Goal to kick-off: celebration and walk back (clock running).
	Timing_GOAL_CELEBRATION_SECONDS      float64 = 55.0
	Timing_POSSESSION_TRANSITION_SECONDS float64 = 5.0
	Timing_RATING_REFRESH_SECONDS        float64 = 6.0
	Timing_MAX_EVENTS                    int     = 1024
)

// Playback helpers for the live view. The simulation itself never depends
// on them: the viewer chooses how many simulated seconds pass per wall
// second (1 = real time), and highlight playback skips (simulates
// headless) between predicted highlight windows.
// MatchTuning::Playback
const (
	Playback_MIN_SPEED               float64 = 1.0
	Playback_MAX_SPEED               float64 = 30.0
	Playback_DEFAULT_SPEED           float64 = 1.0
	Playback_DEFAULT_HIGHLIGHT_SPEED float64 = 1.0
	// Build-up shown before a highlight trigger (shot, card, penalty...).
	Playback_HIGHLIGHT_LEAD_SECONDS float64 = 15.0
	// A build-up that starts with a restart opens this long before the
	// ball is played (the setup before it is dead time).
	Playback_RESTART_LEAD_SECONDS float64 = 1.2
	// Aftermath shown once the trigger happened.
	Playback_HIGHLIGHT_TAIL_SECONDS float64 = 6.0
	// How far ahead a highlight prediction simulates before giving up.
	Playback_PREDICTION_HORIZON_SECONDS float64 = 900.0
	// Highlight windows closer than this are merged into one.
	Playback_MERGE_GAP_SECONDS float64 = 4.0
)

// Player kinematics: an in-situ acceleration-speed profile
// a(v) = A0 (1 - v / vmax) for speeding up, a stronger braking capacity and
// a speed-dependent lateral (centripetal) limit so sharp turns at speed
// force a player to slow down first.
// MatchTuning::Player
const (
	Player_DEFAULT_ATTRIBUTE float64 = 0.5
	Player_RATING_SCALE      float64 = 100.0
	// Attributes are stretched around a typical professional level so that
	// quality differences between squads matter as much as in real results;
	// the stretch saturates far from it so extreme gaps stay plausible.
	Player_ATTRIBUTE_PIVOT    float64 = 0.66
	Player_ATTRIBUTE_CONTRAST float64 = 3.6
	// The absolute level of a match (mean raw outfield attribute of both
	// elevens) is partly normalised toward the reference level before the
	// stretch, so quality gaps decide as before but a weaker league does not
	// turn into a shooting gallery; the level shows in finishing precision
	// instead (per unit of raw level).
	Player_REFERENCE_LEVEL      float64 = 0.65
	Player_LEVEL_NORMALISATION  float64 = 0.7
	Player_LEVEL_FINISHING_GAIN float64 = 2.5
	// Above the reference finishing improves more slowly.
	Player_ELITE_FINISHING_GAIN float64 = 1.0
	Player_MIN_LEVEL_PRECISION  float64 = 0.6
	Player_ATTRIBUTE_SATURATION float64 = 0.38
	Player_MIN_ATTRIBUTE        float64 = 0.02
	// Real top speeds cluster at 8-9 m/s with a record near 10.4 m/s.
	Player_TOP_SPEED_BASE float64 = 7.4
	Player_TOP_SPEED_PACE float64 = 2.6
	// Theoretical maximum acceleration A0 (~7 m/s^2 for a typical pro).
	Player_ACCELERATION_BASE        float64 = 5.7
	Player_ACCELERATION_PACE        float64 = 1.6
	Player_ACCELERATION_PHYSICALITY float64 = 0.6
	Player_BRAKING_BASE             float64 = 6.0
	Player_BRAKING_PHYSICALITY      float64 = 2.0
	// Lateral acceleration a runner sustains at a standstill / top speed.
	Player_LATERAL_ACCELERATION_SLOW float64 = 9.0
	Player_LATERAL_ACCELERATION_FAST float64 = 5.5
	// Desired-speed approach to a target (comfortable deceleration).
	Player_ARRIVAL_DECELERATION      float64 = 2.0
	Player_ARRIVAL_DEAD_ZONE_METRES  float64 = 0.35
	Player_FACING_TURN_RATE_RADIANS  float64 = 9.0
	Player_MOVEMENT_FACING_THRESHOLD float64 = 0.3
	Player_MINIMUM_STAMINA           float64 = 0.35
	// Fatigue lowers top speed far more than the first-step push.
	Player_FATIGUE_TOP_SPEED_LOSS    float64 = 0.14
	Player_FATIGUE_ACCELERATION_LOSS float64 = 0.06
	// Tired players also choose to run less hard (work-rate loss).
	Player_FATIGUE_WORK_RATE_LOSS float64 = 0.12
	// Top speed kept with an empty repeat-sprint reserve.
	Player_EMPTY_RESERVE_TOP_SPEED float64 = 0.80
	// Intent speed caps as a share of the fresh top speed.
	Player_HOLD_SHAPE_SPEED_SCALE          float64 = 0.36
	Player_CARRY_BALL_SPEED_SCALE          float64 = 0.60
	Player_SUPPORT_SPEED_SCALE             float64 = 0.46
	Player_ATTACKING_RUN_SPEED_SCALE       float64 = 0.95
	Player_PRESS_SPEED_SCALE               float64 = 0.88
	Player_COVER_SPEED_SCALE               float64 = 0.55
	Player_MARKING_SPEED_SCALE             float64 = 0.52
	Player_RECOVERY_SPEED_SCALE            float64 = 0.58
	Player_GOALKEEPER_MOVEMENT_SPEED_SCALE float64 = 0.7
	Player_RESTART_WALK_SPEED_SCALE        float64 = 0.22
	// Distance from the target at which a player runs at the urgency cap.
	Player_URGENCY_DISTANCE_METRES float64 = 25.0
	Player_MAX_URGENCY_SPEED_SCALE float64 = 0.56
	// Perception/intent lag: how quickly a moving target is followed.
	Player_TACTICAL_TARGET_RESPONSE_PER_SECOND float64 = 1.8
	Player_URGENT_TARGET_RESPONSE_PER_SECOND   float64 = 4.5
	// Response lost by a side with no familiarity with its tactics.
	Player_FAMILIARITY_RESPONSE_LOSS float64 = 0.3
	// Two bodies of ~0.4 m radius cannot overlap.
	Player_MINIMUM_BODY_SEPARATION_METRES   float64 = 0.8
	Player_BODY_SEPARATION_SHARE            float64 = 0.5
	Player_BODY_COLLISION_VELOCITY_RETAINED float64 = 0.9
	Player_SUBSTITUTION_SETTLE_SECONDS      float64 = 1.5
	Player_MAX_LOOSE_BALL_LOOKAHEAD_SECONDS float64 = 1.2
	Player_COVER_LOOSE_BALL_OFFSET          float64 = 0.055
	Player_PASS_RECEIVER_LOOKAHEAD_SECONDS  float64 = 0.8
	Player_MINIMUM_PURSUIT_SPEED            float64 = 1.4
	Player_CURRENT_VELOCITY_PURSUIT_WEIGHT  float64 = 0.55
	// Teleports (restart repositioning) are not counted as running.
	Player_MAX_COUNTED_STEP_SPEED float64 = 13.0
)

// Dribbling: a carrier on the move pushes the ball 0.6-3 m ahead and
// catches it up at the next touch, so the ball is exposed between touches
// and a heavy touch can lose it. Defenders jockey goal-side and time their
// challenge for the moment the ball is away from the attacker's foot.
// MatchTuning::Dribble
const (
	// Below this speed the ball is kept at the feet (close control).
	Dribble_CLOSE_CONTROL_SPEED float64 = 2.2
	Dribble_FEET_METRES         float64 = 0.35
	// The carrier can strike the ball only when it is this close.
	Dribble_KICK_REACH_METRES           float64 = 0.9
	Dribble_TOUCH_BASE_METRES           float64 = 0.5
	Dribble_TOUCH_METRES_PER_SPEED      float64 = 0.28
	Dribble_MIN_TOUCH_METRES            float64 = 0.6
	Dribble_MAX_TOUCH_METRES            float64 = 3.0
	Dribble_TOUCH_INTERVAL_BASE         float64 = 1.25
	Dribble_TOUCH_INTERVAL_PER_SPEED    float64 = 0.07
	Dribble_TOUCH_INTERVAL_DRIBBLING    float64 = 0.2
	Dribble_MIN_TOUCH_INTERVAL          float64 = 0.4
	Dribble_MAX_TOUCH_INTERVAL          float64 = 1.2
	Dribble_HEAVY_TOUCH_BASE            float64 = 0.045
	Dribble_HEAVY_TOUCH_PRESSURE        float64 = 1.5
	Dribble_HEAVY_TOUCH_PRESSURE_METRES float64 = 3.0
	Dribble_MIN_HEAVY_PUSH_SPEED        float64 = 2.5
	Dribble_MAX_HEAVY_PUSH_SPEED        float64 = 5.0
	// Take-ons against a jockeying defender in front.
	Dribble_TAKE_ON_RANGE_METRES  float64 = 2.8
	Dribble_TAKE_ON_BASE          float64 = 0.42
	Dribble_TAKE_ON_SKILL         float64 = 0.6
	Dribble_TAKE_ON_PACE          float64 = 0.25
	Dribble_MIN_TAKE_ON           float64 = 0.1
	Dribble_MAX_TAKE_ON           float64 = 0.8
	Dribble_TAKE_ON_ANGLE_RADIANS float64 = 0.5
	Dribble_TAKE_ON_TOUCH_METRES  float64 = 2.4
	// A beaten defender is wrong-footed: momentum lost, no challenge.
	Dribble_BEATEN_SECONDS           float64 = 1.1
	Dribble_BEATEN_VELOCITY_RETAINED float64 = 0.25
)

// Physical load statistics thresholds (m/s).
// MatchTuning::Load
const (
	Load_HIGH_INTENSITY_SPEED float64 = 5.5
	Load_SPRINT_SPEED         float64 = 7.0
	Load_SPRINT_EXIT_SPEED    float64 = 6.5
)

// MatchTuning::Shape
const (
	Shape_MIN_WIDTH_SCALE                float64 = 0.72
	Shape_WIDTH_SLIDER_SCALE             float64 = 0.54
	Shape_BASE_LONGITUDINAL_SHIFT        float64 = 0.12
	Shape_COMPACTNESS_LONGITUDINAL_SHIFT float64 = 0.12
	Shape_BASE_LATERAL_SHIFT             float64 = 0.10
	Shape_COMPACTNESS_LATERAL_SHIFT      float64 = 0.10
	Shape_CARRIER_BASE_ADVANCE           float64 = 0.10
	Shape_CARRIER_DRIBBLING_ADVANCE      float64 = 0.08
	Shape_CARRIER_RISK_ADVANCE           float64 = 0.05
	Shape_CARRIER_EVASION_RANGE_METRES   float64 = 8.0
	Shape_CARRIER_BASE_EVASION           float64 = 0.035
	Shape_CARRIER_DRIBBLING_EVASION      float64 = 0.045
	Shape_CARRIER_CENTRALITY             float64 = 0.16
	// Wide players carrying down the flank keep their lane to cross.
	Shape_WIDE_LANE_DEVIATION float64 = 0.24
	// In the final third a wide forward sometimes cuts inside instead,
	// more often when he shoots better than he crosses.
	Shape_CUT_INSIDE_BASE                     float64 = 0.50
	Shape_CUT_INSIDE_PREFERENCE               float64 = 1.0
	Shape_MIN_CUT_INSIDE                      float64 = 0.10
	Shape_MAX_CUT_INSIDE                      float64 = 0.85
	Shape_CUT_INSIDE_PULL                     float64 = 0.8
	Shape_SUPPORT_BASE_ADVANCE                float64 = 0.035
	Shape_SUPPORT_OFFENSIVE_ADVANCE           float64 = 0.10
	Shape_MAX_ACTIVE_SUPPORTERS               int     = 3
	Shape_NEAR_SUPPORT_SLOT                   int     = 0
	Shape_SQUARE_SUPPORT_SLOT                 int     = 1
	Shape_TRAILING_SUPPORT_SLOT               int     = 2
	Shape_SUPPORT_SELECTION_CONTINUITY_METRES float64 = 7.0
	Shape_NEAR_SUPPORT_DEPTH                  float64 = 0.065
	Shape_NEAR_SUPPORT_WIDTH                  float64 = 0.105
	Shape_SQUARE_SUPPORT_DEPTH                float64 = 0.025
	Shape_SQUARE_SUPPORT_WIDTH                float64 = 0.165
	Shape_TRAILING_SUPPORT_DEPTH              float64 = 0.125
	Shape_TRAILING_SUPPORT_WIDTH              float64 = 0.045
	Shape_TRANSITION_SUPPORT_FORWARD_BONUS    float64 = 0.055
	Shape_RUN_BASE_ADVANCE                    float64 = 0.045
	Shape_RUN_RISK_ADVANCE                    float64 = 0.09
	Shape_ONSIDE_RECOVERY                     float64 = 0.055
	Shape_MAX_SUPPORT_DISTANCE_METRES         float64 = 29.0
	Shape_SUPPORT_LONGITUDINAL_PULL           float64 = 0.20
	Shape_SUPPORT_LATERAL_PULL                float64 = 0.14
	Shape_POSSESSION_PROGRESS_START           float64 = 0.25
	Shape_POSSESSION_BLOCK_PROGRESS           float64 = 0.24
	Shape_MIN_COMMITTED_RUNNERS               int     = 1
	Shape_MAX_COMMITTED_RUNNERS               int     = 3
	Shape_MAX_STRIKER_RUNNERS                 int     = 1
	Shape_SECOND_RUNNER_PROGRESS_THRESHOLD    float64 = 0.38
	Shape_SECOND_RUNNER_ATTACK_THRESHOLD      float64 = 0.80
	Shape_STRIKER_RUN_PRIORITY                float64 = 0.32
	Shape_WINGER_RUN_PRIORITY                 float64 = 0.30
	Shape_ATTACKING_MIDFIELDER_RUN_PRIORITY   float64 = 0.30
	Shape_MIDFIELDER_RUN_PRIORITY             float64 = 0.26
	Shape_HOLDING_MIDFIELDER_RUN_PRIORITY     float64 = 0.08
	Shape_RUN_PACE_PRIORITY                   float64 = 0.22
	Shape_RUN_DEPTH_PRIORITY                  float64 = 0.12
	Shape_RUN_SEPARATION_PRIORITY             float64 = 0.10
	Shape_RUN_CONTINUITY_PRIORITY             float64 = 0.24
	// Spread of the per-attack variation in who makes the runs.
	Shape_RUN_VARIETY_PRIORITY                    float64 = 0.15
	Shape_RUN_ONSIDE_BUFFER                       float64 = 0.012
	Shape_RUN_DEPTH_TARGET_PULL                   float64 = 0.82
	Shape_SECONDARY_RUN_DEPTH_STAGGER             float64 = 0.055
	Shape_RUN_CHANNEL_BLEND                       float64 = 0.55
	Shape_LEFT_WIDE_ATTACK_CHANNEL                float64 = 0.18
	Shape_RIGHT_WIDE_ATTACK_CHANNEL               float64 = 0.82
	Shape_LEFT_INSIDE_FORWARD_CHANNEL             float64 = 0.34
	Shape_RIGHT_INSIDE_FORWARD_CHANNEL            float64 = 0.66
	Shape_LEFT_STRIKER_ATTACK_CHANNEL             float64 = 0.42
	Shape_RIGHT_STRIKER_ATTACK_CHANNEL            float64 = 0.58
	Shape_CENTRAL_ATTACK_CHANNEL                  float64 = 0.50
	Shape_MINIMUM_RUN_CHANNEL_SEPARATION          float64 = 0.16
	Shape_FORWARD_SHORT_OPTION_DEPTH              float64 = 0.09
	Shape_FORWARD_SHORT_OPTION_LATERAL_SEPARATION float64 = 0.13
	// Late runs into the box in the final third (pitch fractions from the
	// goal line and from the centre, away from the ball's side).
	Shape_MIDFIELD_ARRIVAL_DEPTH float64 = 0.12
	Shape_MIDFIELD_ARRIVAL_WIDTH float64 = 0.05
	Shape_FAR_POST_ARRIVAL_DEPTH float64 = 0.07
	Shape_FAR_POST_ARRIVAL_WIDTH float64 = 0.10
	// The far-post run needs the ball out wide.
	Shape_FAR_POST_MIN_BALL_WIDTH          float64 = 0.14
	Shape_FINAL_THIRD_FULLBACK_OVERLAP     float64 = 0.17
	Shape_FINAL_THIRD_SELECTION_CONTINUITY float64 = 0.16
	// A presser jockeys goal-side 1.5-2.5 m off the carrier.
	Shape_PRESSING_STANDOFF_BASE                 float64 = 0.014
	Shape_PRESSING_STANDOFF_CAUTIOUS_BONUS       float64 = 0.01
	Shape_PRESSER_CONTINUITY_SECONDS             float64 = 0.75
	Shape_COVER_PRESS_MINIMUM                    float64 = 0.35
	Shape_COVER_LANE_INTERCEPTION_POINT          float64 = 0.52
	Shape_COVER_LANE_GOAL_SIDE_OFFSET            float64 = 0.018
	Shape_COVER_OUTLET_MAX_DISTANCE_METRES       float64 = 29.0
	Shape_COVER_FORWARD_OPTION_METRES            float64 = 7.0
	Shape_COVER_FALLBACK_DEPTH                   float64 = 0.035
	Shape_COVER_FALLBACK_LATERAL_OFFSET          float64 = 0.055
	Shape_CHANNEL_WEIGHT                         float64 = 1.4
	Shape_DANGER_DEPTH_WEIGHT                    float64 = 0.12
	Shape_BASE_MARK_WEIGHT                       float64 = 0.12
	Shape_COMPACTNESS_MARK_WEIGHT                float64 = 0.20
	Shape_MARK_REFRESH_STEPS                     uint64  = 20
	Shape_TIGHT_MARK_DANGER_DEPTH                float64 = 0.62
	Shape_TIGHT_MARK_WEIGHT                      float64 = 0.7
	Shape_GOAL_SIDE_MARK_METRES                  float64 = 1.5
	Shape_SECOND_LOOSE_BALL_PRESS_THRESHOLD      float64 = 0.9
	Shape_ATTACKING_TRANSITION_SUPPORT_ADVANCE   float64 = 0.065
	Shape_ATTACKING_TRANSITION_RUN_ADVANCE       float64 = 0.13
	Shape_ATTACKING_TRANSITION_BALL_PULL         float64 = 0.12
	Shape_ATTACKING_TRANSITION_CARRIER_ADVANCE   float64 = 0.055
	Shape_IN_FLIGHT_SUPPORT_ADVANCE_PER_SECOND   float64 = 0.03
	Shape_IN_FLIGHT_SUPPORT_BALL_PULL_PER_SECOND float64 = 0.25
	Shape_DEFENSIVE_TRANSITION_RECOVERY          float64 = 0.055
	Shape_DEFENSIVE_TRANSITION_BALL_COMPACTNESS  float64 = 0.16
	// Out-of-possession block (depths measured from the own goal line).
	Shape_BLOCK_LINE_BASE             float64 = -0.03
	Shape_BLOCK_LINE_BALL_FACTOR      float64 = 0.5
	Shape_BLOCK_LINE_PRESSING         float64 = 0.08
	Shape_BLOCK_LINE_MIN              float64 = 0.06
	Shape_BLOCK_LINE_MAX              float64 = 0.42
	Shape_BLOCK_SHORT_BALL_DEPTH      float64 = 0.2
	Shape_BLOCK_LONG_BALL_DEPTH       float64 = 0.7
	Shape_BLOCK_SHORT_LENGTH          float64 = 0.16
	Shape_BLOCK_LONG_LENGTH           float64 = 0.36
	Shape_BLOCK_COMPACTNESS_SQUEEZE   float64 = 0.2
	Shape_BLOCK_FORMATION_BACK        float64 = 0.18
	Shape_BLOCK_FORMATION_FRONT       float64 = 0.80
	Shape_BLOCK_WIDTH_SCALE           float64 = 0.72
	Shape_BLOCK_COMPACTNESS_NARROWING float64 = 0.12
	Shape_BLOCK_BALL_SIDE_SHIFT       float64 = 0.35
)

// MatchTuning::Decision
const (
	Decision_PRESSURE_RADIUS_METRES float64 = 8.5
	Decision_BASE_SHOT_THRESHOLD    float64 = 0.056
	Decision_MIN_SHOT_XG            float64 = 0.005
	// Time on the ball before a carrier re-decides while dribbling.
	Decision_MIN_DRIBBLE_TIME float64 = 0.5
	Decision_MAX_DRIBBLE_TIME float64 = 1.1

	// Scored action selection: every candidate (best pass, shot, carry,
	// shield) is measured in a shared utility currency and the closest
	// choices are resolved by a vision-scaled random perturbation.
	Decision_SHOT_SCORE_SCALE              float64 = 26.0
	Decision_SHOT_BASE_INCLINATION         float64 = 1.5
	Decision_FINAL_THIRD_SHOT_BONUS        float64 = 0.3
	Decision_SHOT_ELIGIBILITY_FLOOR        float64 = 0.012
	Decision_SHOT_ELIGIBILITY_RANGE        float64 = 0.04
	Decision_SHOT_SKILL_BONUS              float64 = 0.28
	Decision_SHOT_PRESSURE_PENALTY         float64 = 0.02
	Decision_CARRY_OPENNESS_WEIGHT         float64 = 1.50
	Decision_CARRY_DRIBBLING_BONUS         float64 = 0.45
	Decision_CARRY_PRESSURE_PENALTY        float64 = 0.30
	Decision_CARRY_RISK_BIAS               float64 = 0.20
	Decision_CARRY_HOLD_PENALTY_PER_SECOND float64 = 0.45
	Decision_SHIELD_PRESSURE_THRESHOLD     float64 = 0.60
	// Carry appetite of a wide forward cutting inside (see Shape).
	Decision_CUT_INSIDE_CARRY_BONUS float64 = 0.8
	Decision_CUT_INSIDE_SHOT_BONUS  float64 = 0.6
	Decision_SHIELD_BONUS           float64 = 0.40
	Decision_SHIELD_DRIBBLING       float64 = 0.12
	Decision_VISION_NOISE_SCALE     float64 = 0.45
	Decision_VISION_DECISION_WEIGHT float64 = 0.6
	// Extra decision noise for a side with no tactical familiarity.
	Decision_FAMILIARITY_NOISE_GAIN         float64 = 0.8
	Decision_LATE_GAME_MINUTE               float64 = 82.0
	Decision_LATE_LEAD_SPECULATIVE_PENALTY  float64 = 0.55
	Decision_COMFORTABLE_LEAD               int     = 2
	Decision_COMFORTABLE_LEAD_SHOT_PENALTY  float64 = 3.0
	Decision_CLEAR_CHANCE_XG                float64 = 0.30
	Decision_COMFORTABLE_LEAD_CARRY_PENALTY float64 = 0.5
	Decision_WIDE_SHOT_WIDTH_DEVIATION      float64 = 0.32
	Decision_WIDE_SHOT_DISCOUNT             float64 = 0.12
	Decision_WIDE_RECYCLE_BONUS             float64 = 0.85
)

// MatchTuning::Passing
const (
	Passing_MIN_DISTANCE_METRES       float64 = 2.5
	Passing_MAX_DISTANCE_METRES       float64 = 50.0
	Passing_IDEAL_DISTANCE_METRES     float64 = 16.0
	Passing_OPENNESS_RADIUS_METRES    float64 = 10.0
	Passing_OPENNESS_WEIGHT           float64 = 1.15
	Passing_LANE_RISK_WEIGHT          float64 = 1.45
	Passing_BASE_PROGRESS_WEIGHT      float64 = 1.4
	Passing_OFFENSIVE_PROGRESS_WEIGHT float64 = 1.15
	// Utility lost per metre away from the ideal pass length.
	Passing_DISTANCE_PENALTY_PER_METRE       float64 = 0.0135
	Passing_SAFE_OUTLET_WEIGHT               float64 = 0.8
	Passing_FORWARD_ROLE_BONUS               float64 = 0.0
	Passing_MIN_ACCEPTABLE_OPTION_SCORE      float64 = -0.15
	Passing_PRESSURE_RELEASE_THRESHOLD       float64 = 0.55
	Passing_PRESSURE_RELEASE_MAX_PROGRESSION float64 = 0.03
	Passing_PROGRESSIVE_PASS_MINIMUM         float64 = 0.035
	// Space behind the opposing line (metres to their goal line) that makes
	// a ball over or through it attractive, and the range to full effect.
	Passing_SPACE_BEHIND_MIN_METRES          float64 = 30.0
	Passing_SPACE_BEHIND_RANGE_METRES        float64 = 25.0
	Passing_SPACE_BEHIND_UTILITY             float64 = 1.2
	Passing_SPACE_BEHIND_COMPLETION_BONUS    float64 = 0.15
	Passing_THROUGH_BALL_MINIMUM_PROGRESSION float64 = 0.09
	Passing_SWITCH_PLAY_MINIMUM_WIDTH        float64 = 0.34
	Passing_WIDE_ATTACK_MINIMUM_Y            float64 = 0.30
	Passing_WIDE_ATTACK_MAXIMUM_Y            float64 = 0.70
	Passing_CENTRAL_TARGET_MINIMUM_Y         float64 = 0.30
	Passing_CENTRAL_TARGET_MAXIMUM_Y         float64 = 0.70
	Passing_CROSS_MINIMUM_PASSER_DEPTH       float64 = 0.62
	Passing_CROSS_MINIMUM_RECEIVER_DEPTH     float64 = 0.70
	Passing_CUTBACK_MINIMUM_PASSER_DEPTH     float64 = 0.86
	Passing_CUTBACK_MAXIMUM_PROGRESSION      float64 = 0.035
	Passing_CUTBACK_MINIMUM_PROGRESSION      float64 = -0.24
	Passing_CROSS_UTILITY_BONUS              float64 = 0.7
	Passing_CUTBACK_UTILITY_BONUS            float64 = 0.62
	Passing_CROSS_COMPLETION_PENALTY         float64 = 0.10
	Passing_CUTBACK_COMPLETION_BONUS         float64 = 0.07
	Passing_CROSS_TARGET_BLEND               float64 = 0.35
	Passing_BASE_COMPLETION_PROBABILITY      float64 = 0.58
	Passing_PASSING_COMPLETION_BONUS         float64 = 0.27
	Passing_OPENNESS_COMPLETION_BONUS        float64 = 0.15
	Passing_LANE_COMPLETION_PENALTY          float64 = 0.50
	Passing_DISTANCE_COMPLETION_PENALTY      float64 = 0.42
	Passing_LONG_PASS_REFERENCE_METRES       float64 = 45.0
	Passing_PRESSURE_COMPLETION_PENALTY      float64 = 0.14
	Passing_MIN_COMPLETION_PROBABILITY       float64 = 0.05
	Passing_MAX_COMPLETION_PROBABILITY       float64 = 0.98
	Passing_COMPLETION_UTILITY_WEIGHT        float64 = 1.5
	// Per unit of risk-taking above neutral: weight gained by progress and
	// lost by the completion chance (below neutral the other way round).
	Passing_RISK_PROGRESS_GAIN          float64 = 0.0
	Passing_RISK_SAFETY_GAIN            float64 = 0.0
	Passing_ACTIVE_RUNNER_UTILITY_BONUS float64 = 0.16
	// Utility per unit of the receiver's shooting chance (final third).
	Passing_SHOT_CREATION_WEIGHT        float64 = 1.0
	Passing_THROUGH_BALL_FORWARD_LEAD   float64 = 0.055
	Passing_THROUGH_BALL_TARGET_BLEND   float64 = 0.60
	Passing_LOFTED_DISTANCE_METRES      float64 = 30.0
	Passing_PASS_PRESSURE_RADIUS_METRES float64 = 7.5
	// Average ball speed used to lead a moving receiver.
	Passing_ESTIMATED_BALL_SPEED float64 = 13.0
	Passing_RECEIVER_LEAD_SCALE  float64 = 0.45
	// Lateral execution error in metres (distance error per metre passed).
	Passing_TECHNICAL_ERROR_METRES float64 = 3.6
	Passing_PRESSURE_ERROR_METRES  float64 = 1.2
	Passing_DISTANCE_ERROR         float64 = 0.012
	Passing_BACK_PASS_ERROR_SCALE  float64 = 0.35
	Passing_MAX_BACK_PASS_METRES   float64 = 28.0
	// Ground passes are struck so they arrive at a controllable pace.
	Passing_ARRIVAL_SPEED_BASE      float64 = 6.5
	Passing_ARRIVAL_SPEED_PASSING   float64 = 3.0
	Passing_ARRIVAL_SPEED_PER_METRE float64 = 0.05
	Passing_MIN_GROUND_SPEED        float64 = 7.0
	Passing_MAX_GROUND_SPEED        float64 = 24.0
	// Lofted balls: horizontal launch speed grows with the distance.
	Passing_LOFTED_BASE_SPEED            float64 = 11.0
	Passing_LOFTED_SPEED_PER_METRE       float64 = 0.22
	Passing_MAX_LOFTED_SPEED             float64 = 24.0
	Passing_LOFTED_ARRIVAL_HEIGHT_METRES float64 = 1.1
	Passing_DELIVERY_HEIGHT_SPREAD       float64 = 0.6
	Passing_OFFSIDE_PERCEPTION_ERROR     float64 = 0.06
	// Runners time their runs imperfectly: they sometimes drift beyond the
	// line, and a passer who misreads it releases an offside pass.
	Passing_RUN_TIMING_GAMBLE        float64 = 0.02
	Passing_OFFSIDE_TIMING_WINDOW    float64 = 0.03
	Passing_OFFSIDE_TIMING_CHANCE    float64 = 0.7
	Passing_RUN_TIMING_EPOCH_SECONDS float64 = 5.0
	// Heading rotation (rad/s) of a curled ground pass.
	Passing_MAX_CURVE           float64 = 0.12
	Passing_CURVE_SKILL_BASE    float64 = 0.4
	Passing_MIN_ACTION_COOLDOWN float64 = 0.8
	Passing_MAX_ACTION_COOLDOWN float64 = 1.4
	// The kicker cannot touch his own kick again for this long.
	Passing_KICKER_LOCKOUT_SECONDS          float64 = 0.35
	Passing_LANE_START_MARGIN               float64 = 0.06
	Passing_LANE_END_MARGIN                 float64 = 0.97
	Passing_BASE_INTERCEPTION_RADIUS_METRES float64 = 2.2
	Passing_DEFENDING_INTERCEPTION_METRES   float64 = 1.7
	Passing_PACE_INTERCEPTION_METRES        float64 = 0.7
	Passing_BASE_INTERCEPTION_RISK          float64 = 0.55
	Passing_LATE_LANE_RISK                  float64 = 0.35
	Passing_OFFSIDE_MARGIN                  float64 = 0.004
)

// MatchTuning::Shooting
const (
	Shooting_PRESSURE_RADIUS_METRES float64 = 4.0
	Shooting_PRESSURE_PENALTY       float64 = 0.45
	// Logistic xG location model (Soccermatics fit on Wyscout PL 2017/18):
	// a central shot from 11 m is worth ~0.18, from 20 m ~0.06.
	Shooting_XG_INTERCEPT           float64 = 0.5103
	Shooting_XG_ANGLE               float64 = 0.6338
	Shooting_XG_DISTANCE            float64 = -0.2798
	Shooting_XG_LINE                float64 = 0.1243
	Shooting_XG_LATERAL             float64 = -0.0300
	Shooting_XG_LINE_SQUARED        float64 = 0.0014
	Shooting_XG_LATERAL_SQUARED     float64 = 0.0041
	Shooting_XG_ANGLE_LINE          float64 = -0.1251
	Shooting_XG_MAX_DISTANCE_METRES float64 = 38.0
	Shooting_BASE_SKILL_FACTOR      float64 = 0.62
	Shooting_SHOOTING_SKILL_FACTOR  float64 = 0.45
	Shooting_MIN_OPEN_PLAY_XG       float64 = 0.005
	Shooting_MAX_OPEN_PLAY_XG       float64 = 0.64
	Shooting_PENALTY_XG             float64 = 0.78
	Shooting_HEADER_XG_FACTOR       float64 = 0.45
	Shooting_GOAL_HALF_WIDTH_METRES float64 = 3.66
	Shooting_KEEPER_CENTRED_METRES  float64 = 0.3
	Shooting_NEAR_SIDE_AIM_CHANCE   float64 = 0.25
	// Share of shots simply struck hard at the frame rather than placed.
	Shooting_POWER_SHOT_BASE_CHANCE       float64 = 0.75
	Shooting_POWER_SHOT_SKILL_REDUCTION   float64 = 0.10
	Shooting_POWER_SHOT_WIDTH_METRES      float64 = 1.3
	Shooting_AIM_POST_INSET_BASE          float64 = 0.45
	Shooting_AIM_POST_INSET_SKILL         float64 = 0.5
	Shooting_AIM_INSET_SPREAD             float64 = 1.2
	Shooting_PENALTY_AIM_INSET_SPREAD     float64 = 0.5
	Shooting_AIM_MIN_HEIGHT_METRES        float64 = 0.2
	Shooting_AIM_MAX_HEIGHT_METRES        float64 = 1.9
	Shooting_HEADER_AIM_MAX_HEIGHT_METRES float64 = 1.4
	// Baseline placement scatter at the reference distance; keeper reach
	// remains geometric. League-level precision scales this scatter rather
	// than changing shot volume or awarding goals from an xG roll.
	Shooting_ERROR_BASE_METRES          float64 = 3.75
	Shooting_ERROR_SKILL_METRES         float64 = 2.4
	Shooting_ERROR_PRESSURE_METRES      float64 = 1.8
	Shooting_ERROR_REFERENCE_METRES     float64 = 16.0
	Shooting_ERROR_MIN_DISTANCE_SCALE   float64 = 0.55
	Shooting_PENALTY_ERROR_SCALE        float64 = 0.16
	Shooting_VERTICAL_ERROR_SHARE       float64 = 0.6
	Shooting_MIN_CROSSING_HEIGHT_METRES float64 = 0.12
	Shooting_MAX_CROSSING_HEIGHT_METRES float64 = 6.0
	// Struck shots leave the foot at 21-33 m/s (lab instep kicks ~28 m/s).
	Shooting_BASE_BALL_SPEED       float64 = 21.0
	Shooting_SHOOTING_SPEED_BONUS  float64 = 9.0
	Shooting_POWER_SPEED_BONUS     float64 = 3.0
	Shooting_MIN_SPEED_VARIATION   float64 = 0.88
	Shooting_RELEASE_HEIGHT_METRES float64 = 0.2
	Shooting_WOODWORK_BAND_METRES  float64 = 0.10
	Shooting_WOODWORK_REBOUND      float64 = 0.35
	Shooting_MAX_SET_PIECE_XG      float64 = 0.82
	Shooting_MIN_GOAL_PROBABILITY  float64 = 0.01
	Shooting_MIN_ACTION_COOLDOWN   float64 = 1.5
	Shooting_MAX_ACTION_COOLDOWN   float64 = 2.5
)

// MatchTuning::Defending
const (
	// Reach of a standing challenge and of a sliding one (to the ball).
	Defending_TACKLE_DISTANCE_METRES        float64 = 1.3
	Defending_TACKLE_DEFENDING_REACH_METRES float64 = 0.3
	Defending_SLIDE_TACKLE_DISTANCE_METRES  float64 = 2.2
	// Engagement waits for the ball to be away from the attacker's foot.
	Defending_CLOSE_CONTROL_ENGAGE_SHARE      float64 = 0.25
	Defending_EXPOSED_ENGAGE_SHARE            float64 = 1.75
	Defending_EXPOSURE_WIN_BONUS              float64 = 0.35
	Defending_SHIELD_PHYSICALITY_PENALTY      float64 = 0.15
	Defending_SLIDE_WIN_PENALTY               float64 = 0.05
	Defending_FROM_BEHIND_WIN_PENALTY         float64 = 0.10
	Defending_FROM_BEHIND_FOUL_FACTOR         float64 = 2.2
	Defending_SLIDE_FOUL_FACTOR               float64 = 1.6
	Defending_EXPOSED_FOUL_RELIEF             float64 = 0.6
	Defending_ENGAGE_RATE_PER_SECOND          float64 = 0.35
	Defending_PRESSING_ENGAGE_BONUS           float64 = 0.3
	Defending_FINAL_THIRD_ENGAGE_FACTOR       float64 = 2.5
	Defending_MIN_TACKLE_COOLDOWN             float64 = 1.4
	Defending_MAX_TACKLE_COOLDOWN             float64 = 3.2
	Defending_PRESSING_COOLDOWN_REDUCTION     float64 = 0.25
	Defending_TACKLE_COOLDOWN_BASE_MULTIPLIER float64 = 1.1
	Defending_BASE_WIN_CHANCE                 float64 = 0.12
	Defending_DEFENDING_WIN_BONUS             float64 = 0.42
	Defending_DRIBBLING_WIN_PENALTY           float64 = 0.27
	// Pressing wins more challenges by making more of them, not better ones:
	// rushed challenges win slightly less often and foul more.
	Defending_PRESSING_WIN_EFFECT       float64 = -0.07
	Defending_PRESSING_FOUL_BONUS       float64 = 0.06
	Defending_MIN_WIN_CHANCE            float64 = 0.08
	Defending_MAX_WIN_CHANCE            float64 = 0.58
	Defending_PHYSICALITY_DUEL_WEIGHT   float64 = 0.12
	Defending_BASE_FOUL_CHANCE          float64 = 0.24
	Defending_RISK_FOUL_BONUS           float64 = 0.10
	Defending_TECHNIQUE_FOUL_BONUS      float64 = 0.10
	Defending_WINNING_TACKLE_FOUL_SHARE float64 = 0.25
	Defending_PENALTY_AREA_FOUL_SCALE   float64 = 0.13
	// A won challenge often only pokes the ball loose.
	Defending_POKE_LOOSE_CHANCE float64 = 0.45
	Defending_MIN_POKE_SPEED    float64 = 3.5
	Defending_MAX_POKE_SPEED    float64 = 8.0
	// Defenders pressed in their own third with no safe pass clear it.
	Defending_CLEARANCE_MAX_DEPTH           float64 = 0.33
	Defending_CLEARANCE_MIN_DISTANCE_METRES float64 = 30.0
	Defending_CLEARANCE_MAX_DISTANCE_METRES float64 = 50.0
	Defending_CLEARANCE_SPEED               float64 = 22.0
	Defending_CLEARANCE_TOUCHLINE_BIAS      float64 = 0.35
	Defending_MIN_RECOVERY_COOLDOWN         float64 = 0.6
	Defending_MAX_RECOVERY_COOLDOWN         float64 = 1.2
	// Reach of an outfield player throwing his body at a shot.
	Defending_BLOCK_DISTANCE_METRES   float64 = 1.6
	Defending_BASE_BLOCK_CHANCE       float64 = 0.55
	Defending_DEFENDING_BLOCK_BONUS   float64 = 0.25
	Defending_DEFLECTION_SPEED_FACTOR float64 = -0.22
	Defending_MAX_DEFLECTION_Y_SPEED  float64 = 5.0
)

// Ball physics in SI units: quadratic air drag with a drag crisis, gravity,
// a restitution bounce, rolling resistance on the grass and curve as a
// heading rotation (a Magnus-effect stand-in).
// MatchTuning::Ball
const (
	Ball_GRAVITY                    float64 = 9.81
	Ball_AIR_DENSITY                float64 = 1.225
	Ball_MASS_KG                    float64 = 0.43
	Ball_CROSS_SECTION_M2           float64 = 0.038
	Ball_DRAG_SUBCRITICAL           float64 = 0.45
	Ball_DRAG_SUPERCRITICAL         float64 = 0.24
	Ball_DRAG_CRISIS_LOW_SPEED      float64 = 8.0
	Ball_DRAG_CRISIS_HIGH_SPEED     float64 = 14.0
	Ball_BOUNCE_RESTITUTION         float64 = 0.62
	Ball_BOUNCE_TANGENTIAL_RETAINED float64 = 0.84
	Ball_MIN_BOUNCE_SPEED           float64 = 1.0
	Ball_ROLLING_DECELERATION       float64 = 0.75
	Ball_ROLLING_VISCOUS_PER_SECOND float64 = 0.05
	Ball_CURVE_DECAY_PER_SECOND     float64 = 0.35
	Ball_STOP_SPEED                 float64 = 0.2
	// A spent shot slower than this is a loose ball again.
	Ball_DEAD_SHOT_SPEED    float64 = 6.0
	Ball_MAX_FLIGHT_SECONDS float64 = 6.0
	// Reach for first touches: feet for a low ball, body up to the chest.
	Ball_GOALKEEPER_CONTROL_RADIUS_METRES float64 = 1.8
	Ball_OUTFIELD_CONTROL_RADIUS_METRES   float64 = 1.0
	Ball_CHEST_CONTROL_RADIUS_METRES      float64 = 0.7
	Ball_LOW_BALL_METRES                  float64 = 0.5
	Ball_BASE_CONTROL_CHANCE              float64 = 0.74
	Ball_TOUCH_SKILL_BONUS                float64 = 0.45
	Ball_SPEED_CONTROL_PENALTY            float64 = 0.0065
	Ball_MIN_CONTROL_CHANCE               float64 = 0.28
	Ball_MAX_CONTROL_CHANCE               float64 = 0.97
	// An opponent in the lane only gets a foot to it some of the time: it
	// depends on how far he has to stretch and how fast the ball is.
	Ball_INTERCEPT_BASE_CHANCE     float64 = 0.62
	Ball_INTERCEPT_DEFENDING_BONUS float64 = 0.35
	Ball_INTERCEPT_SPEED_PENALTY   float64 = 0.018
	Ball_INTERCEPT_MIN_CHANCE      float64 = 0.08
	Ball_FAILED_TRAP_TIME          float64 = 0.55
	Ball_FAILED_TOUCH_LOCKOUT      float64 = 0.3
	Ball_HEAVY_TOUCH_RETAINED      float64 = 0.25
	Ball_MIN_HEAVY_TOUCH_SPEED     float64 = 1.5
	Ball_MAX_HEAVY_TOUCH_SPEED     float64 = 4.5
	Ball_BASE_TRAP_TIME            float64 = 0.25
	Ball_TRAP_SKILL_PENALTY        float64 = 0.35
	Ball_MIN_POST_TOUCH_DELAY      float64 = 0.5
	Ball_MAX_POST_TOUCH_DELAY      float64 = 0.9
	Ball_PASSING_TOUCH_WEIGHT      float64 = 0.85
	Ball_KEEPER_FEET_WEIGHT        float64 = 0.7
	Ball_SAVE_DISTANCE_METRES      float64 = 1.8
	Ball_SAVE_DIVE_TIME            float64 = 1.2
	// A keeper holding the ball waits for his team to push up.
	Ball_MIN_KEEPER_HOLD_SECONDS float64 = 2.5
	Ball_MAX_KEEPER_HOLD_SECONDS float64 = 5.0
	Ball_GOAL_NET_BALL_DEPTH     float64 = 0.02
)

// MatchTuning::Rules
const (
	Rules_MAX_SUBSTITUTIONS_PER_TEAM int = 5
	// One more change (and window) once a tie goes to extra time.
	Rules_EXTRA_TIME_SUBSTITUTIONS int     = 1
	Rules_MINIMUM_PLAYERS          int     = 7
	Rules_PARKED_PLAYER_OFFSET     float64 = 0.06
	Rules_PARKED_PLAYER_SPACING    float64 = 0.025
	Rules_DEFENSIVE_SLOT_DEPTH     float64 = 0.30
	Rules_SHORT_HANDED_DROP        float64 = 0.02
	// Crowd-driven home advantage on execution; referee bias is modelled in
	// Discipline (card biases).
	Rules_HOME_EXECUTION_BONUS float64 = 0.15
	// Home crowd lift on technical and mental attributes (stretched
	// units), worth a few rating points.
	Rules_HOME_ATTRIBUTE_BONUS float64 = 0.19
	// Execution, decision and duel edge per player of numerical advantage
	// (a side reduced to ten is stretched and loses confidence).
	Rules_NUMERICAL_EDGE_PER_PLAYER  float64 = 0.20
	Rules_MAX_NUMERICAL_EDGE_PLAYERS int     = 3
	// Weight of the team edge on the attributes used in duels.
	Rules_DUEL_EDGE_WEIGHT       float64 = 0.8
	Rules_HOME_FINAL_THIRD_START float64 = 0.67
	Rules_AWAY_FINAL_THIRD_START float64 = 0.33
)

// Goalkeeper model in real seconds, metres and m/s.
// MatchTuning::Goalkeeper
const (
	// A set keeper reads the striker's body shape, so his effective
	// reaction to a struck ball is shorter than a raw 0.21 s reaction time.
	Goalkeeper_REACTION_BASE_SECONDS     float64 = 0.12
	Goalkeeper_REACTION_SKILL_SECONDS    float64 = 0.08
	Goalkeeper_SCREENED_REACTION_SECONDS float64 = 0.12
	Goalkeeper_SCREEN_WIDTH_METRES       float64 = 0.9
	Goalkeeper_READ_ERROR_METRES         float64 = 0.8
	Goalkeeper_DIVE_POST_MARGIN          float64 = 0.02
	Goalkeeper_DIVE_ACCELERATION_METRES  float64 = 14.0
	Goalkeeper_DIVE_BASE_SPEED_METRES    float64 = 4.2
	Goalkeeper_DIVE_SKILL_SPEED_METRES   float64 = 1.6
	Goalkeeper_BODY_REACH_METRES         float64 = 2.0
	Goalkeeper_HAND_REACH_HEIGHT_SHARE   float64 = 0.45
	Goalkeeper_DIVE_REACH_HEIGHT_SHARE   float64 = 0.85
	Goalkeeper_SHOULDER_HEIGHT_SHARE     float64 = 0.8
	Goalkeeper_MIN_VERTICAL_REACH_MARGIN float64 = 0.01
	Goalkeeper_HEIGHT_REACH_GAIN         float64 = 0.8
	Goalkeeper_HIGH_BALL_METRES          float64 = 1.9
	Goalkeeper_HIGH_BALL_REACH_SCALE     float64 = 0.85
	Goalkeeper_COMFORT_SPEED_METRES      float64 = 24.0
	Goalkeeper_SAVE_BASE                 float64 = 0.97
	// Handling difficulty conditional on a geometrically reachable shot.
	// Calibrated with season batches; this does not extend physical reach.
	Goalkeeper_SAVE_STRETCH_PENALTY     float64 = 0.15
	Goalkeeper_SAVE_SPEED_PENALTY       float64 = 0.015
	Goalkeeper_SAVE_SKILL_BONUS         float64 = 0.22
	Goalkeeper_MIN_SAVE_CHANCE          float64 = 0.05
	Goalkeeper_MAX_SAVE_CHANCE          float64 = 0.98
	Goalkeeper_HOLD_BASE                float64 = 0.35
	Goalkeeper_HOLD_SKILL               float64 = 0.40
	Goalkeeper_HOLD_STRETCH_PENALTY     float64 = 0.45
	Goalkeeper_HOLD_SPEED_PENALTY       float64 = 0.02
	Goalkeeper_MIN_HOLD_CHANCE          float64 = 0.05
	Goalkeeper_MAX_HOLD_CHANCE          float64 = 0.90
	Goalkeeper_TIP_OVER_METRES          float64 = 2.05
	Goalkeeper_TIP_AROUND_STRETCH       float64 = 0.5
	Goalkeeper_PARRY_SPEED_SHARE        float64 = 0.35
	Goalkeeper_PARRY_MAX_VERTICAL_SPEED float64 = 4.0
	Goalkeeper_RECOVER_TIME_SECONDS     float64 = 1.1
	Goalkeeper_DISTRIBUTE_TIME_SECONDS  float64 = 0.6
	Goalkeeper_PENALTY_READ_BASE        float64 = 0.0
	Goalkeeper_PENALTY_READ_SKILL       float64 = 0.12
	Goalkeeper_PENALTY_STAY_CHANCE      float64 = 0.08
	Goalkeeper_PENALTY_DIVE_METRES      float64 = 1.2
	// Even a keeper who guesses right rarely reaches a placed penalty.
	Goalkeeper_PENALTY_SAVE_FACTOR        float64 = 0.30
	Goalkeeper_CLAIM_LOOKAHEAD_SECONDS    float64 = 0.95
	Goalkeeper_CLAIM_BOX_DEPTH_METRES     float64 = 6.0
	Goalkeeper_SWEEP_ADVANTAGE            float64 = 1.25
	Goalkeeper_RUSH_COVER_DISTANCE_METRES float64 = 5.0
	Goalkeeper_RUSH_CLOSING_SHARE         float64 = 0.55
	Goalkeeper_NEAR_DEPTH_METRES          float64 = 0.8
	Goalkeeper_DEPTH_PER_METRE            float64 = 0.11
	Goalkeeper_SWEEPER_DISTANCE_METRES    float64 = 35.0
	Goalkeeper_SWEEPER_DEPTH_PER_METRE    float64 = 0.18
	Goalkeeper_MAX_DEPTH_METRES           float64 = 16.0
)

// MatchTuning::SetPiece
const (
	SetPiece_CROSSING_FREE_KICK_METRES     float64 = 45.0
	SetPiece_DIRECT_FREE_KICK_METRES       float64 = 30.0
	SetPiece_DIRECT_FREE_KICK_WIDTH_METRES float64 = 16.0
	SetPiece_CROSS_MIN_WIDTH_METRES        float64 = 16.0
	SetPiece_DIRECT_SHOT_BASE              float64 = 0.55
	SetPiece_DIRECT_SHOT_SKILL             float64 = 0.45
	SetPiece_DIRECT_SHOT_DISTANCE_PENALTY  float64 = 0.012
	SetPiece_DIRECT_FREE_KICK_XG           float64 = 0.065
	SetPiece_WALL_BLOCK_BASE               float64 = 0.34
	SetPiece_WALL_BLOCK_SKILL              float64 = 0.20
	SetPiece_WALL_REBOUND_SPEED_SHARE      float64 = 0.35
	// The wall hides the strike: the keeper reacts this much later.
	SetPiece_WALL_SCREEN_SECONDS float64 = 0.55
	// A dead ball is struck with more precision and aimed higher and
	// tighter to the post than a shot on the move.
	SetPiece_FREE_KICK_ERROR_SCALE            float64 = 0.5
	SetPiece_FREE_KICK_AIM_INSET_SPREAD       float64 = 0.6
	SetPiece_FREE_KICK_AIM_MIN_HEIGHT_METRES  float64 = 1.2
	SetPiece_SHORT_CORNER_CHANCE              float64 = 0.12
	SetPiece_SHORT_CORNER_MAX_DISTANCE_METRES float64 = 14.0
	SetPiece_NEAR_POST_CHANCE                 float64 = 0.42
	SetPiece_TARGET_RANDOMNESS                float64 = 0.6
	// Weight of a box attacker's heading reach when choosing the target.
	SetPiece_TARGET_THREAT_WEIGHT     float64 = 0.0
	SetPiece_REST_DEFENDERS           int     = 3
	SetPiece_MARKING_GOAL_SIDE_OFFSET float64 = 0.008
	SetPiece_PENALTY_WAIT_OFFSET      float64 = 0.01
	SetPiece_SET_PIECE_PHASE_SECONDS  float64 = 20.0
	// Defensive clearances and blocks near the own goal line often go out
	// for a corner.
	SetPiece_CLEARANCE_BEHIND_CHANCE float64 = 0.6
	// A defender heading a delivery away near his goal line.
	SetPiece_HEADER_BEHIND_CHANCE float64 = 0.33
	// Chance that a defender's header or touch of a delivery in front of
	// his goal skews toward it (own goals), and its speed and lift.
	// Per eligible defensive contact, rather than per attacking possession.
	// Multiple contacts in the same goalmouth attack compound this hazard.
	SetPiece_OWN_GOAL_TOUCH_CHANCE       float64 = 0.025
	SetPiece_OWN_GOAL_TOUCH_DEPTH_METRES float64 = 9.0
	SetPiece_OWN_GOAL_TOUCH_WIDTH_METRES float64 = 9.0
	SetPiece_OWN_GOAL_MIN_SPEED          float64 = 7.0
	SetPiece_OWN_GOAL_MAX_SPEED          float64 = 13.0
	SetPiece_OWN_GOAL_MAX_LIFT           float64 = 2.0
	SetPiece_BLOCK_BEHIND_CHANCE         float64 = 0.6
	SetPiece_BLOCK_BEHIND_DEPTH          float64 = 0.20
	SetPiece_CROSS_CLEARANCE_DEPTH       float64 = 0.25
)

// Unit conventions. Pitch x is normalised by LENGTH_METRES and y by
// WIDTH_METRES. The public ball height `MatchBall::z` stays in length units
// (z * BALL_Z_METRES = metres) for renderers; velocities are in m/s.
// MatchTuning::Units
const (
	Units_BALL_Z_METRES                float64 = 105.0
	Units_CROSSBAR_HEIGHT_METRES       float64 = 2.44
	Units_BALL_RADIUS_METRES           float64 = 0.11
	Units_DEFAULT_PLAYER_HEIGHT_METRES float64 = 1.80
)

// MatchTuning::Rating
const (
	Rating_BASELINE                    float64 = 6.0
	Rating_MINIMUM                     float64 = 3.0
	Rating_MAXIMUM                     float64 = 10.0
	Rating_GOAL                        float64 = 1.05
	Rating_OWN_GOAL                    float64 = -0.9
	Rating_ASSIST                      float64 = 0.65
	Rating_SHOT_ON_TARGET              float64 = 0.12
	Rating_SHOT_OFF_TARGET             float64 = -0.04
	Rating_KEY_PASS                    float64 = 0.18
	Rating_COMPLETED_PASS              float64 = 0.012
	Rating_FAILED_PASS                 float64 = -0.035
	Rating_TACKLE_WON                  float64 = 0.13
	Rating_INTERCEPTION                float64 = 0.10
	Rating_CLEARANCE                   float64 = 0.05
	Rating_AERIAL_WON                  float64 = 0.04
	Rating_SAVE                        float64 = 0.32
	Rating_GOALKEEPER_GOAL_CONCEDED    float64 = -0.38
	Rating_DEFENDER_GOAL_CONCEDED      float64 = -0.14
	Rating_CLEAN_SHEET_GOALKEEPER      float64 = 0.6
	Rating_CLEAN_SHEET_DEFENDER        float64 = 0.35
	Rating_CLEAN_SHEET_MINIMUM_MINUTES float64 = 60.0
	Rating_FOUL                        float64 = -0.06
	Rating_YELLOW_CARD                 float64 = -0.35
	Rating_RED_CARD                    float64 = -1.6
	Rating_RESULT_BONUS                float64 = 0.25
	Rating_FULL_MATCH_MINUTES          float64 = 90.0
)

// MatchTuning::Discipline
const (
	Discipline_STRICTNESS_SD  float64 = 0.13
	Discipline_MIN_STRICTNESS float64 = 0.65
	Discipline_MAX_STRICTNESS float64 = 1.4
	// Bounds of a referee drawn around a league's own mean strictness.
	Discipline_MIN_CONTEXT_STRICTNESS        float64 = 0.3
	Discipline_MAX_CONTEXT_STRICTNESS        float64 = 2.8
	Discipline_RECKLESS_SHARE                float64 = 0.17
	Discipline_CARELESS_YELLOW_CHANCE        float64 = 0.024
	Discipline_RECKLESS_YELLOW_CHANCE        float64 = 0.235
	Discipline_TACTICAL_YELLOW_CHANCE        float64 = 0.24
	Discipline_SERIOUS_FOUL_PLAY_SHARE       float64 = 0.0015
	Discipline_DOGSO_RED_CHANCE              float64 = 0.38
	Discipline_PENALTY_AREA_DOGSO_RED_CHANCE float64 = 0.18
	Discipline_AWAY_CARD_BIAS                float64 = 1.07
	Discipline_HOME_CARD_BIAS                float64 = 0.94
	Discipline_BOOKED_PLAYER_CAUTION         float64 = 0.25
	Discipline_ADVANTAGE_WINDOW_SECONDS      float64 = 3.0
	Discipline_ADVANTAGE_MIN_OPENNESS        float64 = 0.45
	Discipline_ADVANTAGE_PLAY_CHANCE         float64 = 0.45
	Discipline_DOGSO_MAX_DISTANCE_METRES     float64 = 28.0
	Discipline_DOGSO_MAX_WIDTH_DEVIATION     float64 = 0.22
)

// MatchTuning::Injury
const (
	// Non-contact hazard per match minute for a fresh player at moderate
	// intensity; low condition and sprinting raise it.
	Injury_BASE_HAZARD_PER_MINUTE         float64 = 0.00009
	Injury_FATIGUE_HAZARD_MULTIPLIER      float64 = 3.0
	Injury_INTENSITY_HAZARD_MULTIPLIER    float64 = 1.5
	Injury_CONTACT_INJURY_CHANCE          float64 = 0.006
	Injury_RECKLESS_CONTACT_INJURY_CHANCE float64 = 0.03
	Injury_INJURED_SPEED_SCALE            float64 = 0.55
	Injury_CHECK_INTERVAL_MINUTES         float64 = 1.0
)

// MatchTuning::Stoppage
const (
	Stoppage_FIRST_HALF_BASE_MINUTES  float64 = 1.5
	Stoppage_SECOND_HALF_BASE_MINUTES float64 = 2.5
	Stoppage_EXTRA_TIME_BASE_MINUTES  float64 = 0.5
	Stoppage_MINUTES_PER_GOAL         float64 = 0.7
	Stoppage_MINUTES_PER_SUBSTITUTION float64 = 0.3
	Stoppage_MINUTES_PER_CARD         float64 = 0.3
	Stoppage_MINUTES_PER_INJURY       float64 = 1.2
	Stoppage_MINUTES_PER_PENALTY      float64 = 0.5
	Stoppage_MIN_ADDED_MINUTES        int     = 1
	Stoppage_MAX_ADDED_MINUTES        int     = 12
	// A dangerous attack (corner, penalty, shot in flight) is allowed to
	// finish; this bounds how long the referee waits past the added time.
	Stoppage_MAX_OVERRUN_MINUTES float64 = 1.5
)

// MatchTuning::Substitution
const (
	Substitution_MAX_WINDOWS                 int     = 3
	Substitution_EARLIEST_TACTICAL_MINUTE    float64 = 68.0
	Substitution_FATIGUE_THRESHOLD           float64 = 0.60
	Substitution_FATIGUE_THRESHOLD_LATE_GAIN float64 = 0.12
	Substitution_LATE_GAME_MINUTE            float64 = 80.0
	Substitution_CARD_RISK_MINUTE            float64 = 64.0
	Substitution_CARD_RISK_NEED              float64 = 0.22
	Substitution_TRAILING_CHASE_MINUTE       float64 = 66.0
	Substitution_LEADING_PROTECT_MINUTE      float64 = 75.0
	Substitution_TACTICAL_NEED               float64 = 0.12
	Substitution_MINIMUM_NEED                float64 = 0.22
	// Share of the minimum need that suffices in the last minutes.
	Substitution_LATE_NEED_SHARE float64 = 0.4
	// Share of the minimum need that joins a change already being made.
	Substitution_WINDOW_NEED_SHARE  float64 = 0.95
	Substitution_MINUTE_NEED_GAIN   float64 = 0.012
	Substitution_FATIGUE_NEED_SCALE float64 = 2.2
	// Forwards lose their sharpness first and are replaced most often.
	Substitution_ATTACKER_FATIGUE_NEED_FACTOR float64 = 1.6
	// From this minute a tiring forward is also freshened up tactically
	// (need per minute, scaled by the condition he has lost).
	Substitution_ATTACKER_ROTATION_MINUTE          float64 = 64.0
	Substitution_ATTACKER_ROTATION_NEED_PER_MINUTE float64 = 0.045
	Substitution_ROLE_FIT_BONUS                    float64 = 0.5
)

// Two-pool energy model per real second. The slow pool (`stamina`, the
// match condition) drains with running intensity and caps top speed late
// in the game; the fast pool (repeat-sprint reserve) empties during
// high-intensity bursts and refills within a couple of minutes.
// MatchTuning::Fatigue
const (
	Fatigue_IDLE_DRAIN       float64 = 0.000030
	Fatigue_RUNNING_DRAIN    float64 = 0.00080
	Fatigue_SPRINT_THRESHOLD float64 = 0.78
	Fatigue_SPRINT_DRAIN     float64 = 0.00030
	Fatigue_PRESSING_DRAIN   float64 = 0.00015
	// Extra drain per second for the whole side out of possession, per unit
	// of pressing above the neutral setting.
	Fatigue_TEAM_PRESSING_NEUTRAL         float64 = 0.5
	Fatigue_TEAM_PRESSING_DRAIN           float64 = 0.00040
	Fatigue_ENDURANCE_BASE                float64 = 1.40
	Fatigue_ENDURANCE_RELIEF              float64 = 0.80
	Fatigue_HALF_TIME_RECOVERY            float64 = 0.06
	Fatigue_EXTRA_TIME_BREAK_RECOVERY     float64 = 0.03
	Fatigue_EXTRA_TIME_HALF_TIME_RECOVERY float64 = 0.01
	Fatigue_TECHNIQUE_ERROR_GAIN          float64 = 0.35
	// Reserve drained per second at full sprint (empties in ~25 s).
	Fatigue_RESERVE_DRAIN_PER_SECOND float64 = 0.04
	Fatigue_RESERVE_RECOVERY_SECONDS float64 = 90.0
)

// MatchTuning::Aerial
const (
	Aerial_CONTROL_CEILING_METRES         float64 = 1.55
	Aerial_BASE_JUMP_METRES               float64 = 0.32
	Aerial_PHYSICALITY_JUMP_METRES        float64 = 0.38
	Aerial_HEAD_OFFSET_METRES             float64 = 0.08
	Aerial_GOALKEEPER_ARM_REACH_RATIO     float64 = 1.26
	Aerial_GOALKEEPER_BASE_JUMP_METRES    float64 = 0.30
	Aerial_GOALKEEPER_SKILL_JUMP_METRES   float64 = 0.30
	Aerial_DUEL_RADIUS_METRES             float64 = 1.1
	Aerial_GOALKEEPER_CLAIM_RADIUS_METRES float64 = 2.5
	Aerial_BASE_CLAIM_CHANCE              float64 = 0.50
	Aerial_CROWDING_PENALTY               float64 = 0.08
	Aerial_SKILL_CLAIM_BONUS              float64 = 0.24
	Aerial_DUEL_FOUL_CHANCE               float64 = 0.08
	// Defenders facing a delivery win first contact more often.
	Aerial_DEFENDER_DUEL_ADVANTAGE float64 = 1.3
	Aerial_HEADER_SHOT_MIN_DEPTH   float64 = 0.84
	// Team-mates' lofted balls are cushioned down below this depth.
	Aerial_HEADER_KNOCK_DOWN_MIN_DEPTH    float64 = 0.72
	Aerial_KNOCK_DOWN_MAX_BACKWARD_METRES float64 = 4.0
	Aerial_HEADER_SHOT_WIDTH              float64 = 0.18
	Aerial_HEADER_SHOT_MIN_XG             float64 = 0.06
	Aerial_HEADER_CLEARANCE_SPEED         float64 = 17.0
	// Weakest defensive header as a share of the clearance speed.
	Aerial_MIN_CLEARANCE_SHARE     float64 = 0.45
	Aerial_HEADER_PASS_SPEED       float64 = 10.0
	Aerial_HEADER_LIFT             float64 = 5.0
	Aerial_HEADER_ACCURACY_PENALTY float64 = 0.96
	// Share of physicality (against shooting) in heading precision.
	Aerial_HEADING_PHYSICAL_SHARE float64 = 0.3
	Aerial_HEADER_SPEED_SCALE     float64 = 0.7
	Aerial_ARRIVAL_HEIGHT_METRES  float64 = 2.1
)

// Touchline instructions: a shout nudges the team sliders (and shot
// appetite) and fades linearly; a team talk scales execution, decisions and
// work rate by at most a few per cent for one half.
// MatchTuning::Touchline
const (
	Touchline_SHOUT_DURATION_SECONDS float64 = 600.0
	Touchline_SHOUT_SLIDER_STEP      float64 = 0.2
	Touchline_SHOUT_SHOT_BIAS        float64 = 0.6
	// Each recent shout divides the next one's effect by (1 + recent
	// shouts); one recent shout is forgotten per this many seconds (less
	// than a shout lasts, so a shout made once the last has faded is at
	// full strength).
	Touchline_SHOUT_REPEAT_FADE_SECONDS float64 = 480.0
	// A new shape costs tactical familiarity per metre the slots move on
	// average (at most MAX), recovered linearly over RECOVERY seconds.
	Touchline_RESHAPE_FAMILIARITY_PER_METRE float64 = 0.02
	Touchline_MAX_RESHAPE_FAMILIARITY_COST  float64 = 0.2
	Touchline_RESHAPE_RECOVERY_SECONDS      float64 = 300.0
	Touchline_ENCOURAGE_WORK_RATE           float64 = 0.03
	Touchline_MAX_TEAM_TALK_MODIFIER        float64 = 0.05
	// Score effects on the sliders per goal of lead (at most two), from
	// SCORE_EFFECT_BASE of full strength at kick-off to full at 90 minutes.
	Touchline_SCORE_EFFECT_MAX_GOALS   int     = 3
	Touchline_SCORE_EFFECT_BASE        float64 = 0.4
	Touchline_SCORE_EFFECT_OFFENSIVE   float64 = 0.10
	Touchline_SCORE_EFFECT_RISK        float64 = 0.10
	Touchline_SCORE_EFFECT_PRESSING    float64 = 0.08
	Touchline_SCORE_EFFECT_COMPACTNESS float64 = 0.08
	// A side two or more goals up also stops committing men forward (per
	// goal of lead from Decision::COMFORTABLE_LEAD, at full urgency).
	Touchline_GAME_MANAGEMENT_OFFENSIVE float64 = 0.12
	// A clearly weaker side sits deeper and more compact and commits a few
	// fewer men forward, from UNDERDOG_GAP_START of mean (stretched) outfield
	// quality below the opponent to full effect UNDERDOG_GAP_RANGE later. It
	// still attacks with its forwards: taking them out of the game made big
	// quality gaps far more decisive than real results (NFR-019).
	Touchline_UNDERDOG_GAP_START   float64 = 0.08
	Touchline_UNDERDOG_GAP_RANGE   float64 = 0.30
	Touchline_UNDERDOG_PRESSING    float64 = 0.35
	Touchline_UNDERDOG_COMPACTNESS float64 = 0.40
	Touchline_UNDERDOG_OFFENSIVE   float64 = 0.12
	// AI managers react to the score late in the game.
	Touchline_AI_CHASE_MINUTE          float64 = 65.0
	Touchline_AI_SHOOT_ON_SIGHT_MINUTE float64 = 82.0
	Touchline_AI_PROTECT_MINUTE        float64 = 75.0
)

// External control of one player (play mode). The controlled player's body
// is integrated at a finer fixed rate than the AI step so a stick feels
// responsive; everything else keeps the common step.
// MatchTuning::Control
const (
	// Kinematic sub-steps per fixed step for the controlled player (50 Hz).
	Control_PHYSICS_SUBSTEPS int = 5
	// Run speed without the sprint button, as a share of top speed.
	Control_JOG_SPEED_SHARE float64 = 0.62
	// How long a pressed action waits for a chance to be performed.
	Control_ACTION_BUFFER_SECONDS float64 = 1.0
	// A pass goes to the team-mate best aligned with the aim within this
	// cosine (60 degrees) ...
	Control_PASS_MIN_ALIGNMENT float64 = 0.5
	// ... preferring nearer team-mates by this much per metre.
	Control_PASS_DISTANCE_WEIGHT float64 = 0.006

	// --- Play mode: pass assistance (index = MatchPlayerInput::passAssist) --
	// Aim cone (cosine) of a pass without, with normal and with strong
	// assistance; index 1 is PASS_MIN_ALIGNMENT.
	// ASSIST_MIN_ALIGNMENT: see var Control_ASSIST_MIN_ALIGNMENT below.
	// Weight of the pass's chance of arriving in picking the receiver.
	// ASSIST_COMPLETION_WEIGHT: see var Control_ASSIST_COMPLETION_WEIGHT below.
	// A through ball prefers team-mates going forward (per metre gained)
	// and those already running in behind.
	Control_THROUGH_PROGRESS_WEIGHT float64 = 0.012
	Control_THROUGH_RUNNER_BONUS    float64 = 0.25
	// Space ahead of the receiver a through ball is played into, from a
	// tap to a full bar (metres).
	Control_THROUGH_LEAD_MIN_METRES float64 = 5.0
	Control_THROUGH_LEAD_MAX_METRES float64 = 16.0
	// Length of a pass into space along the stick (ground and lofted).
	Control_SPACE_PASS_MIN_METRES float64 = 10.0
	Control_SPACE_PASS_MAX_METRES float64 = 30.0
	Control_SPACE_LOFT_MIN_METRES float64 = 18.0
	Control_SPACE_LOFT_MAX_METRES float64 = 45.0
	// Power of a tap (no hold), as a share of the bar.
	Control_DEFAULT_POWER float64 = 0.55

	// --- Play mode: shots ----------------------------------------------------
	// Shot pace scale: SHOT_SPEED_MIN_SCALE + SHOT_SPEED_POWER_GAIN * power.
	Control_SHOT_SPEED_MIN_SCALE  float64 = 0.72
	Control_SHOT_SPEED_POWER_GAIN float64 = 0.42
	// Above this share of the bar the shot gets harder to keep down ...
	Control_SHOT_OVERPOWER_FROM float64 = 0.75
	// ... by up to this much extra spread and lift at a full bar.
	Control_SHOT_OVERPOWER_SPREAD      float64 = 0.55
	Control_SHOT_OVERPOWER_LIFT_METRES float64 = 0.9

	// --- Play mode: defending and receiving ---------------------------------
	// Jockeying pace as a share of the fresh top speed.
	Control_JOCKEY_SPEED_SHARE float64 = 0.42
	// Goal-side gap kept to the carrier while jockeying (metres).
	Control_JOCKEY_CONTAIN_METRES float64 = 2.5
	// A patient, jockeying defender is harder to beat one against one.
	Control_JOCKEY_TAKE_ON_PENALTY float64 = 0.08
	// With the stick idle the receiver of a pass moves to meet it, aiming
	// this many seconds of ball travel ahead.
	Control_MEET_PASS_LOOKAHEAD_SECONDS float64 = 0.5
	// On hand-back the AI picks up the run where the momentum carries him
	// (seconds of travel).
	Control_RELEASE_MOMENTUM_SECONDS float64 = 0.6

	// --- Play mode: who becomes the active footballer ------------------------
	// Ball path looked at, and its sampling (seconds).
	Control_SWITCH_HORIZON_SECONDS float64 = 2.5
	Control_SWITCH_SAMPLE_SECONDS  float64 = 0.1
	// Rough deceleration of a ball on the ground for the path (m/s^2).
	Control_SWITCH_BALL_DECELERATION float64 = 2.0
	// Reaction before a team-mate sets off (seconds).
	Control_SWITCH_REACTION_SECONDS float64 = 0.25
	// Head start of the current active footballer (hysteresis) and of a
	// defender already goal-side of the ball.
	Control_SWITCH_CURRENT_PREFERENCE_SECONDS float64 = 0.35
	Control_SWITCH_GOAL_SIDE_BONUS_SECONDS    float64 = 0.2
)

// Control_ASSIST_MIN_ALIGNMENT is MatchTuning::Control::ASSIST_MIN_ALIGNMENT (read-only).
var Control_ASSIST_MIN_ALIGNMENT = [3]float64{0.82, 0.5, 0.25}

// Control_ASSIST_COMPLETION_WEIGHT is MatchTuning::Control::ASSIST_COMPLETION_WEIGHT (read-only).
var Control_ASSIST_COMPLETION_WEIGHT = [3]float64{0.0, 0.25, 0.6}

// Accepted ranges of MatchContext (league character).
// MatchTuning::Context
const (
	Context_MIN_GOAL_RATE_SCALE float64 = 0.5
	Context_MAX_GOAL_RATE_SCALE float64 = 1.5
	// Shot precision grows as goalRateScale^FINISHING_EXPONENT.
	Context_FINISHING_EXPONENT       float64 = 1.7
	Context_MIN_REFEREE_STRICTNESS   float64 = 0.5
	Context_MAX_REFEREE_STRICTNESS   float64 = 2.0
	Context_MAX_REFEREE_SD           float64 = 0.5
	Context_MAX_HOME_ADVANTAGE_SCALE float64 = 2.0
)

// MatchTuning::Statistics
const (
	Statistics_EVEN_POSSESSION_PERCENT float64 = 50.0
	Statistics_PERCENT_SCALE           float64 = 100.0
)
