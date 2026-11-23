// Ported from Player12 src/model/match_engine.cpp lines 44-515, 1337-1375 and 8694-8997 (GPL-3.0)

package engine

import (
	"math"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// Go-only helpers (prefixed h; not in the C++)
// ---------------------------------------------------------------------------

// hClamp is std::clamp for float64 (lo <= hi assumed, as in C++).
func hClamp(value, lo, hi float64) float64 {
	if value < lo {
		return lo
	}
	if hi < value {
		return hi
	}
	return value
}

// hClampInt is std::clamp for int.
func hClampInt(value, lo, hi int) int {
	if value < lo {
		return lo
	}
	if hi < value {
		return hi
	}
	return value
}

// hLerp is std::lerp(a, b, t) = a + t (b - a).
func hLerp(a, b, t float64) float64 { return a + t*(b-a) }

// hAbsInt is std::abs for int.
func hAbsInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// hIsFinite is std::isfinite.
func hIsFinite(value float64) bool { return !math.IsInf(value, 0) && !math.IsNaN(value) }

// hBoolIndex maps a home flag to the C++ `homeTeam ? 0 : 1` side index.
func hBoolIndex(homeTeam bool) int {
	if homeTeam {
		return 0
	}
	return 1
}

// hInitDefaults applies the C++ default member initializers of MatchEngine
// (and of the member structs whose defaults are not Go zero values). The
// constructor (stage B1) calls it before anything else.
func (e *MatchEngine) hInitDefaults() {
	e.ball = NewMatchBall()
	e.stepTicks = 1
	e.refereeStrictness = 1.0
	e.matchContext = NewMatchContext()
	e.finishingPrecision = 1.0
	e.levelPrecision = 1.0
	e.homeAutoSubstitutions = true
	e.awayAutoSubstitutions = true
	e.manualSubstitutionStep = math.MaxUint64
	e.period = 1
	e.knockout = NewKnockout()
	e.shootout = NewShootout()
	e.pendingAdvantage = NewPendingAdvantage()
	e.setPieceHome = true
	e.previousBallPosition = Vector2F{Pitch_CENTRE, Pitch_CENTRE}
	e.stats = NewMatchStats()
	e.tracker = NewMatchTracker()
	e.lastPassDecision = NewPassDecision()
	e.lastScenarioDecision = NewScenarioDecision()
	e.keepers = [2]GoalkeeperControl{NewGoalkeeperControl(), NewGoalkeeperControl()}
	e.state = MatchState_KICK_OFF
	e.homePhase = TeamPhase_SET_PIECE
	e.awayPhase = TeamPhase_SET_PIECE
	e.substepBallPosition = Vector2F{Pitch_CENTRE, Pitch_CENTRE}
	e.homeFamiliarity = 1.0
	e.awayFamiliarity = 1.0
	e.shouts = [2]ShoutState{NewShoutState(), NewShoutState()}
	e.effectiveSliders = [2]StrategySliders{NewStrategySliders(), NewStrategySliders()}
	e.controlInput = NewMatchPlayerInput()
	e.lastInputAction = MatchInputAction_NONE
	e.lastTriggerType = MatchEventType_INFO
	e.playbackMode = MatchPlaybackMode_FULL_MATCH
	e.playbackSpeed = Playback_DEFAULT_SPEED
	e.highlightSpeed = Playback_DEFAULT_HIGHLIGHT_SPEED
}

// ---------------------------------------------------------------------------
// Anonymous-namespace helpers of match_engine.cpp (lines 44-515), same names
// ---------------------------------------------------------------------------

const EPSILON float64 = 0.00001

// ageInitialGoalkeeperReaction credits only flight since launch before the
// keeper's first body update.
func ageInitialGoalkeeperReaction(control *GoalkeeperControl, flightSeconds float64) {
	if control.State != GoalkeeperState_DIVE || control.Timer != 0.0 {
		return
	}
	control.ReactionRemaining = math.Max(0.0, control.ReactionRemaining-flightSeconds)
}

var makeNonBlockingMatchSeedSequence atomic.Uint32

// makeNonBlockingMatchSeed mixes a clock value and a process-wide sequence
// (non-deterministic seed for the seedless constructor).
func makeNonBlockingMatchSeed() uint32 {
	clockValue := uint64(time.Now().UnixNano())
	mixed := clockValue ^ (uint64(makeNonBlockingMatchSeedSequence.Add(1)) << 32)
	return uint32(mixed ^ (mixed >> 32))
}

const (
	PITCH_LENGTH = Pitch_LENGTH_METRES
	PITCH_WIDTH  = Pitch_WIDTH_METRES
	TWO_PI       = 2.0 * math.Pi
	// Share of the drag-free horizontal travel a lofted ball really covers.
	AIR_TRAVEL_SHARE    float64 = 0.88
	HIGHLIGHT_LEAD              = Playback_HIGHLIGHT_LEAD_SECONDS
	HIGHLIGHT_TAIL              = Playback_HIGHLIGHT_TAIL_SECONDS
	HIGHLIGHT_MERGE_GAP         = Playback_MERGE_GAP_SECONDS
	PREDICTION_HORIZON          = Playback_PREDICTION_HORIZON_SECONDS
	STEP_SECONDS                = Timing_FIXED_STEP_SECONDS
)

// RUN_TIMING_EPOCH_STEPS = RUN_TIMING_EPOCH_SECONDS / FIXED_STEP_SECONDS
// (truncated to uint32, as the C++ static_cast).
var RUN_TIMING_EPOCH_STEPS = uint32(Passing_RUN_TIMING_EPOCH_SECONDS / Timing_FIXED_STEP_SECONDS)

func length(vector Vector2F) float64 {
	return math.Sqrt(vector.X*vector.X + vector.Y*vector.Y)
}

// fastAtan2 is an atan2 approximation (max error ~0.0015 rad) for per-step
// facing updates; the facing angle is presentation state only.
func fastAtan2(y, x float64) float64 {
	absX := math.Abs(x)
	absY := math.Abs(y)
	if absX < 1e-12 && absY < 1e-12 {
		return 0.0
	}
	ratio := math.Min(absX, absY) / math.Max(absX, absY)
	squared := ratio * ratio
	angle := ((-0.0464964749*squared+0.15931422)*squared-0.327622764)*squared*ratio + ratio
	if absY > absX {
		angle = 1.57079637 - angle
	}
	if x < 0.0 {
		angle = 3.14159274 - angle
	}
	if y < 0.0 {
		return -angle
	}
	return angle
}

// toMetres is the metric offset (metres) of a normalised pitch
// displacement.
func toMetres(normalisedDelta Vector2F) Vector2F {
	return Vector2F{normalisedDelta.X * PITCH_LENGTH, normalisedDelta.Y * PITCH_WIDTH}
}

const (
	INVERSE_LENGTH        = 1.0 / PITCH_LENGTH
	INVERSE_WIDTH         = 1.0 / PITCH_WIDTH
	INVERSE_FATIGUE_RANGE = 1.0 / (1.0 - Player_MINIMUM_STAMINA)
)

// toPitch is the normalised pitch displacement of a metric offset.
func toPitch(metres Vector2F) Vector2F {
	return Vector2F{metres.X * INVERSE_LENGTH, metres.Y * INVERSE_WIDTH}
}

// distance is the isotropic distance in metres between two normalised pitch
// points.
func distance(first, second Vector2F) float64 {
	return length(toMetres(Vector2F{first.X - second.X, first.Y - second.Y}))
}

func normalized(vector Vector2F) Vector2F {
	magnitude := length(vector)
	if magnitude <= EPSILON {
		return Vector2F{0.0, 0.0}
	}
	return Vector2F{vector.X / magnitude, vector.Y / magnitude}
}

// metricDirection is the unit direction in metric space from one pitch
// point to another.
func metricDirection(from, to Vector2F) Vector2F {
	return normalized(toMetres(Vector2F{to.X - from.X, to.Y - from.Y}))
}

// uniform01 is the portable mapping of the 32-bit output to [0, 1) (24 bits
// of mantissa).
func uniform01(engine *mt19937) float64 {
	return float64(engine.next()>>8) * 0x1p-24
}

// gaussian is a standard normal sample (Box-Muller) from portable uniforms.
func gaussian(engine *mt19937) float64 {
	first := math.Max(uniform01(engine), 1e-7)
	second := uniform01(engine)
	return math.Sqrt(-2.0*math.Log(first)) * math.Cos(TWO_PI*second)
}

func stateClockRuns(state MatchState) bool {
	return state != MatchState_KICK_OFF && state != MatchState_HALF_TIME &&
		state != MatchState_FULL_TIME && state != MatchState_PENALTY_SHOOTOUT
}

func stretchAttribute(raw float64) float64 {
	return hClamp(
		Player_ATTRIBUTE_PIVOT+
			Player_ATTRIBUTE_SATURATION*
				math.Tanh((raw-Player_ATTRIBUTE_PIVOT)*Player_ATTRIBUTE_CONTRAST/Player_ATTRIBUTE_SATURATION),
		Player_MIN_ATTRIBUTE, 1.0)
}

func active(player *MatchPlayer) bool {
	return player.Player != nil && player.OnPitch
}

func isHighlightTrigger(t MatchEventType) bool {
	return t == MatchEventType_SHOT || t == MatchEventType_GOAL || t == MatchEventType_OWN_GOAL ||
		t == MatchEventType_PENALTY || t == MatchEventType_YELLOW_CARD ||
		t == MatchEventType_SECOND_YELLOW || t == MatchEventType_RED_CARD ||
		t == MatchEventType_PENALTY_SHOOTOUT
}

// dragCoefficient is the air drag coefficient with the drag crisis between
// two speeds.
func dragCoefficient(speed float64) float64 {
	t := hClamp((speed-Ball_DRAG_CRISIS_LOW_SPEED)/(Ball_DRAG_CRISIS_HIGH_SPEED-Ball_DRAG_CRISIS_LOW_SPEED), 0.0, 1.0)
	smooth := t * t * (3.0 - 2.0*t)
	return hLerp(Ball_DRAG_SUBCRITICAL, Ball_DRAG_SUPERCRITICAL, smooth)
}

// FlightState is the ball state along its horizontal direction of travel.
// One physics function serves the live integrator and the launch solvers so
// predicted and real flights match exactly.
type FlightState struct {
	HorizontalSpeed float64
	VerticalSpeed   float64
	Height          float64
	Bounced         bool
}

// FLIGHT_DRAG_SCALE is the quadratic drag per unit of drag coefficient
// (1/m): 0.5 rho A / m.
const FLIGHT_DRAG_SCALE = 0.5 * Ball_AIR_DENSITY * Ball_CROSS_SECTION_M2 / Ball_MASS_KG

// stepFlight advances a flight by dt; returns the horizontal distance
// travelled.
func stepFlight(flight *FlightState, dt float64) float64 {
	const DRAG_SCALE = FLIGHT_DRAG_SCALE
	flight.Bounced = false
	if airborne := flight.Height > 0.0 || flight.VerticalSpeed > 0.0; airborne {
		speed := math.Sqrt(flight.HorizontalSpeed*flight.HorizontalSpeed + flight.VerticalSpeed*flight.VerticalSpeed)
		// Implicit quadratic drag stays stable at any sub-step length.
		damping := 1.0 / (1.0 + DRAG_SCALE*dragCoefficient(speed)*speed*dt)
		flight.HorizontalSpeed *= damping
		flight.VerticalSpeed = flight.VerticalSpeed*damping - Ball_GRAVITY*dt
		flight.Height += flight.VerticalSpeed * dt
		if flight.Height <= 0.0 {
			flight.Height = 0.0
			flight.Bounced = true
			flight.VerticalSpeed = -flight.VerticalSpeed * Ball_BOUNCE_RESTITUTION
			if flight.VerticalSpeed < Ball_MIN_BOUNCE_SPEED {
				flight.VerticalSpeed = 0.0
			}
			flight.HorizontalSpeed *= Ball_BOUNCE_TANGENTIAL_RETAINED
		}
		return flight.HorizontalSpeed * dt
	}
	// Rolling: constant rolling resistance, a small viscous term and air drag.
	speed := flight.HorizontalSpeed
	deceleration := Ball_ROLLING_DECELERATION + Ball_ROLLING_VISCOUS_PER_SECOND*speed +
		DRAG_SCALE*dragCoefficient(speed)*speed*speed
	next := math.Max(0.0, speed-deceleration*dt)
	if next < Ball_STOP_SPEED {
		flight.HorizontalSpeed = 0.0
	} else {
		flight.HorizontalSpeed = next
	}
	return 0.5 * (speed + next) * dt
}

// inPenaltyArea reports whether a point lies in the penalty area defended
// by the given side.
func inPenaltyArea(position Vector2F, defendingHome bool) bool {
	const HALF_WIDTH = Pitch_PENALTY_AREA_HALF_WIDTH_METRES / Pitch_WIDTH_METRES
	if math.Abs(position.Y-Pitch_CENTRE) > HALF_WIDTH {
		return false
	}
	if defendingHome {
		return position.X <= Pitch_LEFT_PENALTY_AREA_EDGE
	}
	return position.X >= Pitch_RIGHT_PENALTY_AREA_EDGE
}

func isDefensiveRole(role PlayerRole) bool {
	return role == PlayerRole_CB || role == PlayerRole_LB || role == PlayerRole_RB || role == PlayerRole_CDM
}

func isAttackingRole(role PlayerRole) bool {
	return role == PlayerRole_ST || role == PlayerRole_LW || role == PlayerRole_RW || role == PlayerRole_CAM
}

// roleGroup is the coarse role group used to pick like-for-like substitutes.
func roleGroup(role PlayerRole) int {
	if role == PlayerRole_GK {
		return 0
	}
	if role == PlayerRole_CB || role == PlayerRole_LB || role == PlayerRole_RB {
		return 1
	}
	if isAttackingRole(role) {
		return 3
	}
	return 2
}

// insertionSort is a stable insertion sort of the first count entries
// (tiny fixed arrays). Pass a slice of the array: insertionSort(arr[:], n,
// less).
func insertionSort[T any](values []T, count int, less func(a, b T) bool) {
	for index := 1; index < count; index++ {
		value := values[index]
		slot := index
		for slot > 0 && less(value, values[slot-1]) {
			values[slot] = values[slot-1]
			slot--
		}
		values[slot] = value
	}
}

func splitMix64(value uint64) uint64 {
	value += 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}

func stateName(state MatchState) string {
	switch state {
	case MatchState_KICK_OFF:
		return "kick_off"
	case MatchState_PLAYING:
		return "playing"
	case MatchState_THROW_IN:
		return "throw_in"
	case MatchState_GOAL_KICK:
		return "goal_kick"
	case MatchState_CORNER_KICK:
		return "corner_kick"
	case MatchState_FREE_KICK:
		return "free_kick"
	case MatchState_PENALTY:
		return "penalty"
	case MatchState_GOAL:
		return "goal"
	case MatchState_PENALTY_SHOOTOUT:
		return "penalty_shootout"
	case MatchState_HALF_TIME:
		return "half_time"
	case MatchState_FULL_TIME:
		return "full_time"
	}
	return "unknown"
}

func teamPhaseName(phase TeamPhase) string {
	switch phase {
	case TeamPhase_STOPPAGE:
		return "stoppage"
	case TeamPhase_SET_PIECE:
		return "set_piece"
	case TeamPhase_DEFENSIVE_BLOCK:
		return "defensive_block"
	case TeamPhase_DEFENSIVE_TRANSITION:
		return "defensive_transition"
	case TeamPhase_ATTACKING_TRANSITION:
		return "attacking_transition"
	case TeamPhase_POSSESSION:
		return "possession"
	case TeamPhase_FINAL_THIRD:
		return "final_third"
	}
	return "unknown"
}

func intentName(intent PlayerIntent) string {
	switch intent {
	case PlayerIntent_HOLD_SHAPE:
		return "hold_shape"
	case PlayerIntent_CARRY_BALL:
		return "carry_ball"
	case PlayerIntent_OFFER_SUPPORT:
		return "offer_support"
	case PlayerIntent_RECEIVE_PASS:
		return "receive_pass"
	case PlayerIntent_RUN_IN_BEHIND:
		return "run_in_behind"
	case PlayerIntent_ATTACK_BOX:
		return "attack_box"
	case PlayerIntent_OVERLAP:
		return "overlap"
	case PlayerIntent_PRESS_BALL:
		return "press_ball"
	case PlayerIntent_COVER_PRESS:
		return "cover_press"
	case PlayerIntent_BLOCK_PASSING_LANE:
		return "block_passing_lane"
	case PlayerIntent_MARK_OPPONENT:
		return "mark_opponent"
	case PlayerIntent_CLAIM_LOOSE_BALL:
		return "claim_loose_ball"
	case PlayerIntent_RECOVER_SHAPE:
		return "recover_shape"
	case PlayerIntent_GOALKEEP:
		return "goalkeep"
	}
	return "unknown"
}

func movementSpeedScale(intent PlayerIntent) float64 {
	switch intent {
	case PlayerIntent_CARRY_BALL:
		return Player_CARRY_BALL_SPEED_SCALE
	case PlayerIntent_OFFER_SUPPORT:
		return Player_SUPPORT_SPEED_SCALE
	case PlayerIntent_RECEIVE_PASS:
		return Player_ATTACKING_RUN_SPEED_SCALE
	case PlayerIntent_RUN_IN_BEHIND, PlayerIntent_ATTACK_BOX, PlayerIntent_OVERLAP:
		return Player_ATTACKING_RUN_SPEED_SCALE
	case PlayerIntent_PRESS_BALL, PlayerIntent_CLAIM_LOOSE_BALL:
		return Player_PRESS_SPEED_SCALE
	case PlayerIntent_COVER_PRESS, PlayerIntent_BLOCK_PASSING_LANE:
		return Player_COVER_SPEED_SCALE
	case PlayerIntent_MARK_OPPONENT:
		return Player_MARKING_SPEED_SCALE
	case PlayerIntent_RECOVER_SHAPE:
		return Player_RECOVERY_SPEED_SCALE
	case PlayerIntent_GOALKEEP:
		return Player_GOALKEEPER_MOVEMENT_SPEED_SCALE
	case PlayerIntent_HOLD_SHAPE:
		return Player_HOLD_SHAPE_SPEED_SCALE
	}
	return Player_HOLD_SHAPE_SPEED_SCALE
}

func passIntentName(intent PassIntent) string {
	switch intent {
	case PassIntent_RECYCLE:
		return "recycle"
	case PassIntent_PROGRESSIVE:
		return "progressive"
	case PassIntent_THROUGH_BALL:
		return "through_ball"
	case PassIntent_CROSS:
		return "cross"
	case PassIntent_CUTBACK:
		return "cutback"
	case PassIntent_SWITCH_PLAY:
		return "switch_play"
	case PassIntent_PRESSURE_RELEASE:
		return "pressure_release"
	case PassIntent_SET_PIECE:
		return "set_piece"
	}
	return "unknown"
}

func scenarioActionName(action ScenarioAction) string {
	switch action {
	case ScenarioAction_SHOT:
		return "shot"
	case ScenarioAction_PASS:
		return "pass"
	case ScenarioAction_CARRY:
		return "carry"
	case ScenarioAction_SHIELD:
		return "shield"
	case ScenarioAction_CLEAR:
		return "clear"
	case ScenarioAction_NONE:
		return "none"
	}
	return "unknown"
}

// GoalkeeperStateName is goalkeeperStateName (declared in match_engine.h).
func GoalkeeperStateName(state GoalkeeperState) string {
	switch state {
	case GoalkeeperState_SET_POSITION:
		return "set_position"
	case GoalkeeperState_SWEEP:
		return "sweep"
	case GoalkeeperState_RUSH:
		return "rush"
	case GoalkeeperState_CLAIM:
		return "claim"
	case GoalkeeperState_DIVE:
		return "dive"
	case GoalkeeperState_HOLD:
		return "hold"
	case GoalkeeperState_DISTRIBUTE:
		return "distribute"
	case GoalkeeperState_RECOVER:
		return "recover"
	}
	return "unknown"
}

// ---------------------------------------------------------------------------
// Noise and execution helpers (match_engine.cpp lines 1337-1375)
// ---------------------------------------------------------------------------

// counterNoise is counter-based noise keyed by (seed, tick, salt, key),
// uniform in [-1, 1).
func counterNoise(seed uint32, tick uint64, salt uint32, key uint32) float64 {
	mixed := splitMix64((uint64(seed) << 32) ^ tick ^ (uint64(salt) << 48) ^ (uint64(key) << 16))
	return float64(mixed>>40)/float64(uint32(1)<<23) - 1.0
}

func (e *MatchEngine) hashNoise(salt uint32, key uint32) float64 {
	return counterNoise(e.matchSeed, e.stepCounter, salt, key)
}

// epochNoise is like hashNoise, but constant over windows of epochSteps
// steps.
func (e *MatchEngine) epochNoise(salt uint32, key uint32, epochSteps uint32) float64 {
	return counterNoise(e.matchSeed, e.stepCounter/uint64(max(epochSteps, 1)), salt, key)
}

func (e *MatchEngine) executionErrorScale(player *MatchPlayer) float64 {
	// Tired players lose precision, and a home crowd lifts the home side a
	// little: both scale the technical error of passes and shots.
	fatigue := 1.0 + (1.0-player.Stamina)*Fatigue_TECHNIQUE_ERROR_GAIN
	return fatigue * math.Max(0.5, 1.0-e.teamEdge(player.IsHomeTeam)) *
		(1.0 + e.weakFootPressure(player)*TacticsTuning_WEAK_FOOT_ERROR_GAIN)
}

// ---------------------------------------------------------------------------
// MatchEngine helper methods (match_engine.cpp lines 8694-8997)
// ---------------------------------------------------------------------------

func (e *MatchEngine) resetPositions() {
	for i := range e.players {
		player := &e.players[i]
		if !active(player) {
			continue
		}
		player.Position = player.BasePosition
		player.MovementTarget = player.BasePosition
		player.Velocity = Vector2F{0.0, 0.0}
		player.Intent = PlayerIntent_HOLD_SHAPE
		player.IsPressing = false
		player.IsMakingRun = false
		player.ActionCooldown = 0.0
	}
}

// findClosestPlayer (C++ default includeGoalkeeper = true).
func (e *MatchEngine) findClosestPlayer(position Vector2F, homeTeam bool, includeGoalkeeper bool) *MatchPlayer {
	var closest *MatchPlayer
	bestDistance := math.MaxFloat64
	for i := range e.players {
		player := &e.players[i]
		if player.IsHomeTeam != homeTeam || !active(player) || (!includeGoalkeeper && player.IsGoalkeeper) {
			continue
		}
		candidateDistance := distance(position, player.Position)
		if candidateDistance < bestDistance {
			bestDistance = candidateDistance
			closest = player
		}
	}
	return closest
}

func (e *MatchEngine) findGoalkeeper(homeTeam bool) *MatchPlayer {
	for i := range e.players {
		player := &e.players[i]
		if player.IsHomeTeam == homeTeam && active(player) && player.IsGoalkeeper {
			return player
		}
	}
	return nil
}

func (e *MatchEngine) findMatchPlayer(player *Player) *MatchPlayer {
	if player == nil {
		return nil
	}
	for i := range e.players {
		if e.players[i].Player == player {
			return &e.players[i]
		}
	}
	return nil
}

func (e *MatchEngine) nearestOpponentDistance(player *MatchPlayer) float64 {
	bestDistance := math.MaxFloat64
	for i := range e.players {
		opponent := &e.players[i]
		if opponent.IsHomeTeam != player.IsHomeTeam && active(opponent) {
			bestDistance = math.Min(bestDistance, distance(player.Position, opponent.Position))
		}
	}
	return bestDistance
}

func (e *MatchEngine) openSpaceAhead(carrier *MatchPlayer) float64 {
	const OPEN_SPACE_RADIUS_METRES = 13.5
	direction := -1.0
	if carrier.IsHomeTeam {
		direction = 1.0
	}
	probeDepth := hClamp(carrier.Position.X+direction*0.16, 0.02, 0.98)
	CHANNELS := [3]float64{0.25, 0.50, 0.75}
	openness := 0.0
	for _, channelY := range CHANNELS {
		probe := Vector2F{probeDepth, channelY}
		nearest := math.MaxFloat64
		for i := range e.players {
			opponent := &e.players[i]
			if opponent.IsHomeTeam == carrier.IsHomeTeam || !active(opponent) {
				continue
			}
			nearest = math.Min(nearest, distance(probe, opponent.Position))
		}
		openness += hClamp(nearest/OPEN_SPACE_RADIUS_METRES, 0.0, 1.0)
	}
	return openness / 3.0
}

func (e *MatchEngine) passingLaneRisk(passer *MatchPlayer, receiver *MatchPlayer) float64 {
	segment := toMetres(Vector2F{receiver.Position.X - passer.Position.X, receiver.Position.Y - passer.Position.Y})
	segmentLengthSquared := segment.X*segment.X + segment.Y*segment.Y
	if segmentLengthSquared <= EPSILON {
		return 1.0
	}

	combinedSafety := 1.0
	for i := range e.players {
		opponent := &e.players[i]
		if !active(opponent) || opponent.IsHomeTeam == passer.IsHomeTeam {
			continue
		}
		fromPasser := toMetres(Vector2F{opponent.Position.X - passer.Position.X, opponent.Position.Y - passer.Position.Y})
		projection := hClamp((fromPasser.X*segment.X+fromPasser.Y*segment.Y)/segmentLengthSquared, 0.0, 1.0)
		if projection < Passing_LANE_START_MARGIN || projection > Passing_LANE_END_MARGIN {
			continue
		}

		laneDistance := length(Vector2F{fromPasser.X - segment.X*projection, fromPasser.Y - segment.Y*projection})
		interceptionRadius := Passing_BASE_INTERCEPTION_RADIUS_METRES +
			opponent.Defending*Passing_DEFENDING_INTERCEPTION_METRES +
			opponent.Pace*Passing_PACE_INTERCEPTION_METRES
		individualRisk := hClamp(1.0-laneDistance/math.Max(interceptionRadius, EPSILON), 0.0, 1.0)
		combinedSafety *= 1.0 - individualRisk*(Passing_BASE_INTERCEPTION_RISK+projection*Passing_LATE_LANE_RISK)
	}
	return hClamp(1.0-combinedSafety, 0.0, 1.0)
}

func (e *MatchEngine) estimateShotXG(shooter *MatchPlayer) float64 {
	// Logistic expected-goals model on distance, visible goal angle, distance
	// to the goal line and lateral offset, scaled by technique and pressure.
	goalX := 0.0
	if shooter.IsHomeTeam {
		goalX = 1.0
	}
	lineMetres := math.Abs(goalX-shooter.Position.X) * Pitch_LENGTH_METRES
	lateralMetres := math.Abs(Pitch_CENTRE-shooter.Position.Y) * Pitch_WIDTH_METRES
	metres := math.Sqrt(lineMetres*lineMetres + lateralMetres*lateralMetres)
	// The fitted surface is only valid within shooting range.
	if metres > Shooting_XG_MAX_DISTANCE_METRES {
		return Shooting_MIN_OPEN_PLAY_XG
	}

	toTop := toMetres(Vector2F{goalX - shooter.Position.X, Pitch_GOAL_TOP - shooter.Position.Y})
	toBottom := toMetres(Vector2F{goalX - shooter.Position.X, Pitch_GOAL_BOTTOM - shooter.Position.Y})
	denominator := length(toTop) * length(toBottom)
	cosine := 1.0
	if denominator > EPSILON {
		cosine = hClamp((toTop.X*toBottom.X+toTop.Y*toBottom.Y)/denominator, -1.0, 1.0)
	}
	angle := math.Acos(cosine)
	logit := Shooting_XG_INTERCEPT + Shooting_XG_ANGLE*angle +
		Shooting_XG_DISTANCE*metres + Shooting_XG_LINE*lineMetres +
		Shooting_XG_LATERAL*lateralMetres +
		Shooting_XG_LINE_SQUARED*lineMetres*lineMetres +
		Shooting_XG_LATERAL_SQUARED*lateralMetres*lateralMetres +
		Shooting_XG_ANGLE_LINE*angle*lineMetres
	location := 1.0 / (1.0 + math.Exp(-logit))
	pressure := hClamp((Shooting_PRESSURE_RADIUS_METRES-e.nearestOpponentDistance(shooter))/Shooting_PRESSURE_RADIUS_METRES, 0.0, 1.0)
	return hClamp(
		location*(Shooting_BASE_SKILL_FACTOR+shooter.Shooting*Shooting_SHOOTING_SKILL_FACTOR)*
			(1.0-pressure*Shooting_PRESSURE_PENALTY),
		Shooting_MIN_OPEN_PLAY_XG, Shooting_MAX_OPEN_PLAY_XG)
}

func (e *MatchEngine) offsideLine(attackingHome bool) float64 {
	nearestGoalDefender := math.Inf(1)
	if attackingHome {
		nearestGoalDefender = math.Inf(-1)
	}
	secondNearestGoalDefender := nearestGoalDefender
	defenderCount := 0
	for i := range e.players {
		player := &e.players[i]
		if player.IsHomeTeam == attackingHome || !active(player) {
			continue
		}
		defenderCount++
		position := player.Position.X
		if attackingHome {
			if position >= nearestGoalDefender {
				secondNearestGoalDefender = nearestGoalDefender
				nearestGoalDefender = position
			} else if position > secondNearestGoalDefender {
				secondNearestGoalDefender = position
			}
		} else {
			if position <= nearestGoalDefender {
				secondNearestGoalDefender = nearestGoalDefender
				nearestGoalDefender = position
			} else if position < secondNearestGoalDefender {
				secondNearestGoalDefender = position
			}
		}
	}
	if defenderCount < 2 {
		if attackingHome {
			return 1.0
		}
		return 0.0
	}
	return secondNearestGoalDefender
}

// isOffside reports whether receiver stands offside against the given
// offside line (C++ default lineTolerance = 0).
func (e *MatchEngine) isOffside(receiver *MatchPlayer, attackingHome bool, defenderLine float64, lineTolerance float64) bool {
	margin := Passing_OFFSIDE_MARGIN + lineTolerance

	if attackingHome {
		return receiver.Position.X > Pitch_CENTRE &&
			receiver.Position.X > e.ball.Position.X+Passing_OFFSIDE_MARGIN &&
			receiver.Position.X > defenderLine+margin
	}

	return receiver.Position.X < Pitch_CENTRE &&
		receiver.Position.X < e.ball.Position.X-Passing_OFFSIDE_MARGIN &&
		receiver.Position.X < defenderLine-margin
}

// rawAttribute is the rating of an attribute in [0, 1] before the stretch.
func (e *MatchEngine) rawAttribute(player *Player, name string) float64 {
	if stat, found := player.GetStats()[name]; found {
		return stat / Player_RATING_SCALE
	}
	return player.GetOverall(e.statsConfig) / Player_RATING_SCALE
}

func (e *MatchEngine) attribute(player *Player, name string) float64 {
	if player == nil {
		return Player_DEFAULT_ATTRIBUTE
	}
	return stretchAttribute(e.rawAttribute(player, name) + e.levelShift)
}

// computeLevel sets the match level from both elevens (see the Player_*
// level constants).
func (e *MatchEngine) computeLevel(home *Lineup, away *Lineup) {
	// Mean raw outfield quality of both elevens.
	sum := 0.0
	counted := 0
	for _, lineup := range [2]*Lineup{home, away} {
		for _, positioned := range lineup.GetOutfieldPlayers() {
			if positioned.Player == nil {
				continue
			}
			for _, name := range [7]string{"Pace", "Shooting", "Passing", "Dribbling", "Defending", "Physicality", "Vision"} {
				sum += e.rawAttribute(positioned.Player, name)
			}
			counted += 7
		}
	}
	if counted == 0 {
		return
	}
	level := sum / float64(counted)
	e.levelShift = Player_LEVEL_NORMALISATION * (Player_REFERENCE_LEVEL - level)
	fromReference := level - Player_REFERENCE_LEVEL
	gain := Player_ELITE_FINISHING_GAIN
	if fromReference < 0.0 {
		gain = Player_LEVEL_FINISHING_GAIN
	}
	e.levelPrecision = math.Max(Player_MIN_LEVEL_PRECISION, 1.0+fromReference*gain)
}

// randomFloat is uniform in [minimum, maximum) from the play stream.
func (e *MatchEngine) randomFloat(minimum, maximum float64) float64 {
	return minimum + (maximum-minimum)*uniform01(&e.rng)
}

// incidentRoll is uniform in [0, 1) from the referee/injury stream.
func (e *MatchEngine) incidentRoll() float64 { return uniform01(&e.incidentRng) }

func (e *MatchEngine) isHomePlayer(player *Player) bool {
	if player == nil {
		return false
	}
	for i := range e.players {
		if e.players[i].Player == player {
			return e.players[i].IsHomeTeam
		}
	}
	return false
}
