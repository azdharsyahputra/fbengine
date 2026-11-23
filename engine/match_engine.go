// Ported from Player12 src/model/match_engine.h (GPL-3.0)

package engine

import "math"

// All types, enums and structs of match_engine.h, plus the MatchEngine
// struct with every data member. MatchEngine methods are NOT implemented
// here; see METHODS.md for the planned Go signatures and PORTING.md for the
// stage that owns each one.

// MatchState is the state of play.
type MatchState int

const (
	MatchState_KICK_OFF MatchState = iota
	MatchState_PLAYING
	MatchState_THROW_IN
	MatchState_GOAL_KICK
	MatchState_CORNER_KICK
	MatchState_FREE_KICK
	MatchState_PENALTY
	MatchState_GOAL
	// A level knockout match decided from the spot (clock stopped).
	MatchState_PENALTY_SHOOTOUT
	// Half-time, and the breaks before and during extra time.
	MatchState_HALF_TIME
	MatchState_FULL_TIME
)

// TeamPhase is team-level context shared by individual player decisions.
type TeamPhase int

const (
	TeamPhase_STOPPAGE TeamPhase = iota
	TeamPhase_SET_PIECE
	TeamPhase_DEFENSIVE_BLOCK
	TeamPhase_DEFENSIVE_TRANSITION
	TeamPhase_ATTACKING_TRANSITION
	TeamPhase_POSSESSION
	TeamPhase_FINAL_THIRD
)

// PlayerIntent is the high-level decision currently driving a player's
// renderer-independent AI.
type PlayerIntent int

const (
	PlayerIntent_HOLD_SHAPE PlayerIntent = iota
	PlayerIntent_CARRY_BALL
	PlayerIntent_OFFER_SUPPORT
	PlayerIntent_RECEIVE_PASS
	PlayerIntent_RUN_IN_BEHIND
	PlayerIntent_ATTACK_BOX
	PlayerIntent_OVERLAP
	PlayerIntent_PRESS_BALL
	PlayerIntent_COVER_PRESS
	PlayerIntent_BLOCK_PASSING_LANE
	PlayerIntent_MARK_OPPONENT
	PlayerIntent_CLAIM_LOOSE_BALL
	PlayerIntent_RECOVER_SHAPE
	PlayerIntent_GOALKEEP
)

// PassIntent is the tactical purpose of the most recently executed pass.
type PassIntent int

const (
	PassIntent_RECYCLE PassIntent = iota
	PassIntent_PROGRESSIVE
	PassIntent_THROUGH_BALL
	PassIntent_CROSS
	PassIntent_CUTBACK
	PassIntent_SWITCH_PLAY
	PassIntent_PRESSURE_RELEASE
	PassIntent_SET_PIECE
)

// PassDecision describes an executed or evaluated pass.
type PassDecision struct {
	PasserId              uint32
	ReceiverId            uint32
	TargetPoint           Vector2F
	Intent                PassIntent
	Utility               float64
	Progression           float64
	LaneRisk              float64
	CompletionProbability float64
}

// NewPassDecision returns a PassDecision with the C++ default member values.
func NewPassDecision() PassDecision {
	return PassDecision{TargetPoint: Vector2F{Pitch_CENTRE, Pitch_CENTRE}, Intent: PassIntent_RECYCLE}
}

// ScenarioAction is the action chosen in a scenario evaluation.
type ScenarioAction int

const (
	ScenarioAction_NONE ScenarioAction = iota
	ScenarioAction_SHOT
	ScenarioAction_PASS
	ScenarioAction_CARRY
	ScenarioAction_SHIELD
	ScenarioAction_CLEAR
)

// GoalkeeperState is the semantic goalkeeper state. Animation stays out of
// the engine.
type GoalkeeperState int

const (
	GoalkeeperState_SET_POSITION GoalkeeperState = iota
	GoalkeeperState_SWEEP
	GoalkeeperState_RUSH
	GoalkeeperState_CLAIM
	GoalkeeperState_DIVE
	GoalkeeperState_HOLD
	GoalkeeperState_DISTRIBUTE
	GoalkeeperState_RECOVER
)

// ScenarioDecision holds the best and runner-up pass candidates plus the
// final chosen reason.
type ScenarioDecision struct {
	Action        ScenarioAction
	Best          Optional[PassDecision]
	RunnerUp      Optional[PassDecision]
	Reason        string
	PassUtility   float64
	ShotUtility   float64
	CarryUtility  float64
	ShieldUtility float64
}

// NewScenarioDecision returns a ScenarioDecision with the C++ default member
// values (utilities at -infinity).
func NewScenarioDecision() ScenarioDecision {
	return ScenarioDecision{
		Action:        ScenarioAction_NONE,
		PassUtility:   math.Inf(-1),
		ShotUtility:   math.Inf(-1),
		CarryUtility:  math.Inf(-1),
		ShieldUtility: math.Inf(-1),
	}
}

// MatchPlayer is one on-pitch slot. Position, BasePosition and
// MovementTarget are normalised pitch coordinates; Velocity is in metres per
// second (x along the length, y across the width). FacingAngle is the
// heading in normalised pitch space, as renderers expect.
type MatchPlayer struct {
	// TargetOwner is the layer that last set the target this player steers
	// to (target_owner.go); observability only.
	TargetOwner TargetOwner
	// TargetReason is why the owner chose it (target_owner.go).
	TargetReason TargetReason
	// TargetRefiner is the layer that fine-tuned the owner's spot (M4
	// spacing), NONE if untouched.
	TargetRefiner  TargetOwner
	Player         *Player
	IsHomeTeam     bool
	Position       Vector2F
	Velocity       Vector2F
	BasePosition   Vector2F
	MovementTarget Vector2F
	Intent         PlayerIntent
	// Latest tactical target (refreshed at the tactical rate) and whether it
	// is chased urgently.
	TacticalTarget Vector2F
	UrgentMovement bool

	FacingAngle float64
	TargetAngle float64
	TurnRate    float64

	IsTrapping  bool
	TrapTimer   float64
	IsDiving    bool
	DiveTimer   float64
	IsPressing  bool
	IsMakingRun bool

	// Slow energy pool (match condition) in [MINIMUM_STAMINA, 1].
	Stamina float64
	// Fast repeat-sprint reserve in [0, 1].
	SprintReserve float64
	// Fresh top speed (m/s), maximum acceleration A0 and braking (m/s^2).
	MaxSpeed       float64
	Acceleration   float64
	Braking        float64
	IsSprinting    bool
	TackleCooldown float64
	ActionCooldown float64

	Pace        float64
	Shooting    float64
	Passing     float64
	Dribbling   float64
	Defending   float64
	Goalkeeping float64
	Physicality float64
	// Stamina attribute (endurance); Stamina above is the live condition.
	Endurance    float64
	Vision       float64
	HeightMetres float64

	// False once sent off or forced off injured with no substitute left.
	OnPitch bool
	// Goalkeeper duties (the natural keeper, or an emergency replacement).
	IsGoalkeeper bool
	IsInjured    bool
	// Outfield slot of the side's formation (see MatchEngine.getFormation);
	// -1 for the starting goalkeeper. Ignored while keeping goal.
	FormationSlot int8
	YellowCards   int
	// Index into MatchEngine.getPlayerStats() for the current occupant.
	StatsIndex int
}

// NewMatchPlayer returns a MatchPlayer with the C++ default member values.
func NewMatchPlayer() MatchPlayer {
	return MatchPlayer{
		Position:       Vector2F{Pitch_CENTRE, Pitch_CENTRE},
		BasePosition:   Vector2F{Pitch_CENTRE, Pitch_CENTRE},
		MovementTarget: Vector2F{Pitch_CENTRE, Pitch_CENTRE},
		Intent:         PlayerIntent_HOLD_SHAPE,
		TacticalTarget: Vector2F{Pitch_CENTRE, Pitch_CENTRE},
		TurnRate:       Player_FACING_TURN_RATE_RADIANS,
		Stamina:        1.0,
		SprintReserve:  1.0,
		MaxSpeed:       Player_TOP_SPEED_BASE,
		Acceleration:   Player_ACCELERATION_BASE,
		Braking:        Player_BRAKING_BASE,
		Pace:           Player_DEFAULT_ATTRIBUTE,
		Shooting:       Player_DEFAULT_ATTRIBUTE,
		Passing:        Player_DEFAULT_ATTRIBUTE,
		Dribbling:      Player_DEFAULT_ATTRIBUTE,
		Defending:      Player_DEFAULT_ATTRIBUTE,
		Goalkeeping:    Player_DEFAULT_ATTRIBUTE,
		Physicality:    Player_DEFAULT_ATTRIBUTE,
		Endurance:      Player_DEFAULT_ATTRIBUTE,
		Vision:         Player_DEFAULT_ATTRIBUTE,
		HeightMetres:   Units_DEFAULT_PLAYER_HEIGHT_METRES,
		OnPitch:        true,
		FormationSlot:  -1,
	}
}

// MatchBall is the ball. Position is normalised; Z is the height in length
// units (Z * Units_BALL_Z_METRES = metres) for renderers. Velocities are in
// m/s and Curve is the heading rotation rate in rad/s.
type MatchBall struct {
	Position  Vector2F
	Z         float64
	Velocity  Vector2F
	VelocityZ float64
	Curve     float64

	PossessedBy      *Player
	LastPossessor    *Player
	IntendedReceiver *Player
	// The player who last kicked or deflected the ball.
	Kicker *Player
	// Seconds during which the kicker cannot touch the ball again.
	KickerLockout float64
	// Players (by slot bit) who already tried to reach this ball in flight.
	TouchAttempts uint32
	// While dribbled: metres ahead of the carrier's feet and the metric
	// direction of the last touch.
	DribbleExposure    float64
	DribbleTouchLength float64
	DribbleDirection   Vector2F
	TouchTimer         float64

	IsPass         bool
	PassByHome     bool
	PassWasOffside bool
	IsShot         bool
	ShotByHome     bool
	// Whether the shot's flight crosses the goal line inside the frame.
	ShotOnTarget bool
	ShotXG       float64
	// Predicted goal-line crossing of the current shot (y and metres high).
	ShotTargetY            float64
	ShotTargetHeightMetres float64
	ShotElapsedSeconds     float64
	ShotIsHeader           bool
	ShotIsPenalty          bool
	ShotFromSetPiece       bool
	ShotSaveResolved       bool
	// Lofted delivery (cross, corner, long ball) that can be contested.
	IsAerialDelivery bool
	// Thrown in and untouched since: it cannot score directly.
	FromThrowIn bool
}

// NewMatchBall returns a MatchBall with the C++ default member values.
func NewMatchBall() MatchBall {
	return MatchBall{
		Position:         Vector2F{Pitch_CENTRE, Pitch_CENTRE},
		DribbleDirection: Vector2F{1.0, 0.0},
		ShotTargetY:      Pitch_CENTRE,
	}
}

// MatchStats are the team statistics of a match.
type MatchStats struct {
	HomeShots             int
	AwayShots             int
	HomeOnTarget          int
	AwayOnTarget          int
	HomeCorners           int
	AwayCorners           int
	HomeFouls             int
	AwayFouls             int
	HomeSaves             int
	AwaySaves             int
	HomePassesAttempted   int
	AwayPassesAttempted   int
	HomePassesCompleted   int
	AwayPassesCompleted   int
	HomeProgressivePasses int
	AwayProgressivePasses int
	HomeThroughBalls      int
	AwayThroughBalls      int
	HomeCrosses           int
	AwayCrosses           int
	HomeCutbacks          int
	AwayCutbacks          int
	HomeSwitchesOfPlay    int
	AwaySwitchesOfPlay    int
	HomeTackles           int
	AwayTackles           int
	HomeOffsides          int
	AwayOffsides          int
	HomeYellowCards       int
	AwayYellowCards       int
	HomeRedCards          int
	AwayRedCards          int
	HomeTackleAttempts    int
	AwayTackleAttempts    int
	HomeInjuries          int
	AwayInjuries          int
	HomePenalties         int
	AwayPenalties         int
	HomeSubstitutions     int
	AwaySubstitutions     int
	HomeAerialDuelsWon    int
	AwayAerialDuelsWon    int
	HomeHeadedShots       int
	AwayHeadedShots       int
	HomeSetPieceShots     int
	AwaySetPieceShots     int
	HomeAdvantagesPlayed  int
	AwayAdvantagesPlayed  int
	HomeShotsInsideBox    int
	AwayShotsInsideBox    int
	HomeHeadedGoals       int
	AwayHeadedGoals       int
	HomeSetPieceGoals     int
	AwaySetPieceGoals     int
	HomePenaltyGoals      int
	AwayPenaltyGoals      int
	HomePossession        float64
	AwayPossession        float64
	HomeShotXG            float64
	AwayShotXG            float64
	// Clock minutes with the ball in play (restarts excluded).
	BallInPlayMinutes float64
}

// NewMatchStats returns MatchStats with the C++ default member values.
func NewMatchStats() MatchStats {
	return MatchStats{
		HomePossession: Statistics_EVEN_POSSESSION_PERCENT,
		AwayPossession: Statistics_EVEN_POSSESSION_PERCENT,
	}
}

// MatchShout lists touchline instructions a manager can shout during play.
type MatchShout uint8

const (
	MatchShout_PUSH_HIGHER MatchShout = iota
	MatchShout_DROP_DEEPER
	MatchShout_PRESS_MORE
	MatchShout_CALM_DOWN
	MatchShout_ENCOURAGE
	MatchShout_WORK_BALL_INTO_BOX
	MatchShout_SHOOT_ON_SIGHT
	MatchShout_STAND_OFF
	MatchShout_DEMAND_MORE
	MatchShout_HIT_ON_COUNTER
	MatchShout_KEEP_POSSESSION
)

// MatchCommandType is the kind of an external tactical change.
type MatchCommandType uint8

const (
	MatchCommandType_STRATEGY MatchCommandType = iota
	MatchCommandType_SHOUT
	MatchCommandType_FORMATION
	MatchCommandType_MOVE_TO_SLOT
	MatchCommandType_SUBSTITUTION
	// A team talk's modifier for one half (see setTeamTalkModifier).
	MatchCommandType_TEAM_TALK
)

// MatchCommandRecord is a tactical change made from outside the engine (the
// touchline), stamped with the fixed step it was made after. Only the fields
// of its type are used.
type MatchCommandRecord struct {
	// Value of MatchEngine.getSimulatedSteps() when the change was made.
	Step     uint64
	Type     MatchCommandType
	HomeTeam bool
	Strategy Strategy
	Shout    MatchShout
	// Outfield slot positions in lineup coordinates (FORMATION).
	Formation []Vector2F
	// Player moved (MOVE_TO_SLOT) or replaced (SUBSTITUTION).
	Player PlayerID
	// Bench player coming on (SUBSTITUTION).
	Incoming PlayerID
	// Target slot (MOVE_TO_SLOT, optional for SUBSTITUTION).
	Slot Optional[int]
	// Half (1 or 2) and modifier of a TEAM_TALK.
	TalkHalf     int
	TalkModifier float64
}

// NewMatchCommandRecord returns a MatchCommandRecord with the C++ default
// member values.
func NewMatchCommandRecord() MatchCommandRecord {
	return MatchCommandRecord{
		Type:     MatchCommandType_STRATEGY,
		HomeTeam: true,
		Strategy: NewStrategy(),
		Shout:    MatchShout_ENCOURAGE,
		TalkHalf: 1,
	}
}

// MatchInputAction is a one-shot action requested by an external
// controller (play mode).
type MatchInputAction uint8

const (
	MatchInputAction_NONE MatchInputAction = iota
	MatchInputAction_PASS
	MatchInputAction_LOFTED_PASS
	MatchInputAction_SHOOT
	MatchInputAction_CLEAR
	MatchInputAction_TACKLE
	MatchInputAction_SLIDE_TACKLE
	// A ground pass into the space ahead of a team-mate's run.
	MatchInputAction_THROUGH_BALL
)

// MatchPlayerInput is the controller state for the externally controlled
// player (play mode; not ported beyond the type, see controlled.go).
type MatchPlayerInput struct {
	// Stick: run direction, magnitude 0..1 of the run speed (0 = stand).
	MoveX float64
	MoveY float64
	// Sprint button: run at top speed instead of the jog share.
	Sprint bool
	// Action button, performed at the first step it is possible.
	Action MatchInputAction
	// Pass aim direction; zero aims along the stick (or the facing).
	AimX float64
	AimY float64
	// Strength of the action in [0, 1] from how long its button was held.
	Power float64
	// Jockey (held).
	Jockey bool
	// Pass assistance: 0 along the stick, 1 (default) leans toward the best
	// placed team-mate near the aim, 2 picks him in a wide cone.
	PassAssist uint8
}

// NewMatchPlayerInput returns a MatchPlayerInput with the C++ defaults.
func NewMatchPlayerInput() MatchPlayerInput { return MatchPlayerInput{PassAssist: 1} }

// MatchInputRecord is a controller change, stamped with the fixed step it
// takes effect on.
type MatchInputRecord struct {
	// Fixed step (see MatchEngine.getSimulatedSteps()) that applies it.
	Step uint64
	// Controlled player from this step; 0 hands the player back to the AI.
	Player PlayerID
	Input  MatchPlayerInput
	// Real minutes per half in Play; 0 preserves the watch clock.
	PlayHalfMinutes int
}

// NewMatchInputRecord returns a MatchInputRecord with the C++ defaults.
func NewMatchInputRecord() MatchInputRecord {
	return MatchInputRecord{Input: NewMatchPlayerInput()}
}

// MatchFidelity is the simulation detail of a headless match.
type MatchFidelity uint8

const (
	// The live engine's 10 Hz fixed step: watched matches and quick results.
	MatchFidelity_FULL MatchFidelity = iota
	// Unwatched background fixtures: live play is identical to FULL, but
	// the walk to each restart spot is simulated in coarser steps
	// (Timing_BACKGROUND_STOPPAGE_TICKS).
	MatchFidelity_BACKGROUND
)

// MatchPlaybackMode is how advancePlayback() presents the match (playback
// is not ported; the type is kept for the engine fields).
type MatchPlaybackMode int

const (
	// Every simulated second is shown at the playback speed.
	MatchPlaybackMode_FULL_MATCH MatchPlaybackMode = iota
	// Only highlight windows are shown; the rest is simulated headless.
	MatchPlaybackMode_HIGHLIGHTS
)

// MatchHighlight is a stretch of play worth showing, in simulated seconds
// since kick-off.
type MatchHighlight struct {
	Type           MatchEventType
	StartSeconds   float64
	TriggerSeconds float64
	EndSeconds     float64
	// Match clock minute of the (most important) trigger.
	TriggerMinute float64
}

// TacticalArrival is MatchEngine::TacticalArrival (private in C++).
type TacticalArrival int

const (
	TacticalArrival_MIDFIELDER TacticalArrival = iota
	TacticalArrival_FAR_POST
	TacticalArrival_FULLBACK
)

// Shootout is MatchEngine::Shootout: penalty shootout progress (index 0
// home).
type Shootout struct {
	Started    bool
	HomeFirst  bool
	InFlight   bool
	KickerHome bool
	Taker      PlayerID
	Timer      float64
	Kicks      [2]int
	Goals      [2]int
	// Kicking order of each side (player ids), cycled when exhausted.
	Order [2][]PlayerID
}

// NewShootout returns a Shootout with the C++ default member values.
func NewShootout() Shootout { return Shootout{HomeFirst: true, KickerHome: true} }

// PendingAdvantage is MatchEngine::PendingAdvantage.
type PendingAdvantage struct {
	Active           bool
	FouledTeamHome   bool
	Position         Vector2F
	SecondsRemaining float64
}

// NewPendingAdvantage returns a PendingAdvantage with the C++ defaults.
func NewPendingAdvantage() PendingAdvantage {
	return PendingAdvantage{Position: Vector2F{Pitch_CENTRE, Pitch_CENTRE}}
}

// GoalkeeperControl is MatchEngine::GoalkeeperControl.
type GoalkeeperControl struct {
	State             GoalkeeperState
	Timer             float64
	ReactionRemaining float64
	DiveTargetY       float64
	LateralVelocity   float64
}

// NewGoalkeeperControl returns a GoalkeeperControl with the C++ defaults.
func NewGoalkeeperControl() GoalkeeperControl {
	return GoalkeeperControl{State: GoalkeeperState_SET_POSITION, DiveTargetY: Pitch_CENTRE}
}

// ShoutState is MatchEngine::ShoutState.
type ShoutState struct {
	Shout            MatchShout
	RemainingSeconds float64
	// Share of the full effect (lower for repeated shouts).
	Impact float64
	// Recent shouts, fading by one per SHOUT_REPEAT_FADE_SECONDS.
	Repeats float64
}

// NewShoutState returns a ShoutState with the C++ default member values.
func NewShoutState() ShoutState { return ShoutState{Shout: MatchShout_ENCOURAGE, Impact: 1.0} }

// SlotTactic is MatchEngine::SlotTactic: role of an outfield slot and its
// in-possession shift.
type SlotTactic struct {
	Role             TacticalRole
	Profile          RoleProfile
	PossessionOffset Vector2F
}

// ControlledShot is MatchEngine::ControlledShot: aim and power of the
// controlled player's shot being struck.
type ControlledShot struct {
	Active bool
	// Aimed crossing point (metres from the goal's centre), if aimed.
	AimMetres Optional[float64]
	Power     float64
}

// PassOption is MatchEngine::PassOption.
type PassOption struct {
	Receiver              *MatchPlayer
	TargetPoint           Vector2F
	Intent                PassIntent
	Utility               float64
	Progression           float64
	LaneRisk              float64
	CompletionProbability float64
	PassDistance          float64
	Lofted                bool
}

// NewPassOption returns a PassOption with the C++ default member values.
func NewPassOption() PassOption {
	return PassOption{TargetPoint: Vector2F{Pitch_CENTRE, Pitch_CENTRE}, Intent: PassIntent_RECYCLE}
}

// SaveAttempt is MatchEngine::SaveAttempt.
type SaveAttempt struct {
	Saved          bool
	Stretch        float64
	SpeedExcess    float64
	CrossingHeight float64
}

// MatchEngine is the stateful, deterministic-when-seeded live match
// simulation in real match time. Home attacks toward x=1 and away attacks
// toward x=0.
//
// Field names are the C++ member names verbatim (lowerCamel, unexported).
// Default member initializers of the C++ class are applied by hInitDefaults
// (helpers.go), which the constructor (stage B1) must call first.
type MatchEngine struct {
	players      []MatchPlayer
	ball         MatchBall
	statsConfig  *StatsConfig
	homeStrategy Strategy
	awayStrategy Strategy
	rng          mt19937
	// Referee decisions and injuries draw from their own stream so tuning
	// them does not reshuffle player decisions and ball physics.
	incidentRng mt19937
	matchSeed   uint32
	// Simulated time in fixed-step ticks (FIXED_STEP_SECONDS each).
	stepCounter uint64
	// Ticks advanced per simulated step (1, or more in background fidelity).
	stepTicks         uint32
	refereeStrictness float64
	// Standard normal draw behind the referee's strictness.
	refereeDraw  float64
	matchContext MatchContext
	// Shot precision factor from MatchContext.GoalRateScale.
	finishingPrecision float64
	// Raw attribute shift toward the reference level, and the finishing
	// precision of the match's level.
	levelShift     float64
	levelPrecision float64

	playerStats   []PlayerMatchStats
	substitutions []MatchSubstitution
	homeBench     []*Player
	awayBench     []*Player
	// Everyone in both matchday squads (names for the commentary).
	squad []*Player
	// Set-piece designations of each side (index 0 home), from the lineups.
	designations             [2]SetPieceDesignations
	homeAutoSubstitutions    bool
	awayAutoSubstitutions    bool
	homeSubstitutionWindows  int
	awaySubstitutionWindows  int
	stoppageSequence         uint32
	lastStoppageStep         uint64
	manualSubstitutionStep   uint64
	homeLastWindowStoppage   uint32
	awayLastWindowStoppage   uint32
	homeLastAiReviewStoppage uint32
	awayLastAiReviewStoppage uint32

	period           int
	abandoned        bool
	addedMinutes     [4]int
	stoppageLogs     [4]StoppageLog
	knockout         Knockout
	extraTimeReached bool
	// Penalty shootout progress (index 0 home).
	shootout            Shootout
	elapsedMatchMinutes float64
	injuryCheckTimer    float64

	pendingAdvantage PendingAdvantage

	// Chain used for assists and key passes.
	lastCompletedPasser   *Player
	lastCompletedReceiver *Player
	shotAssistCandidate   *Player
	lastShooter           *Player
	restartIsSetPiece     bool
	// Shots by the attacking side shortly after a corner or attacking free
	// kick count as set-piece shots, until the defence wins the ball or play
	// stops.
	setPiecePhaseRemaining float64
	setPieceHome           bool

	previousPlayerPositions    []Vector2F
	previousPlayerFacingAngles []float64
	previousBallPosition       Vector2F
	previousBallZ              float64

	events []MatchEvent
	// Trailing events still without their commentary line.
	undescribedEvents    int
	teamNames            [2]string
	stats                MatchStats
	tracker              MatchTracker
	lastPassDecision     PassDecision
	lastScenarioDecision ScenarioDecision
	// Index 0 is the home keeper, 1 the away keeper.
	keepers                    [2]GoalkeeperControl
	state                      MatchState
	homePhase                  TeamPhase
	awayPhase                  TeamPhase
	lastControlledTeamHome     Optional[bool]
	transitionSecondsRemaining float64
	// Slot index of the restart taker (an index keeps the engine copyable).
	restartTakerIndex        Optional[int]
	setPieceTimer            float64
	goalScoredByHome         bool
	goalCelebrationRemaining float64
	accumulator              float64
	ratingRefreshTimer       float64
	// When the current carrier gained the ball (simulated seconds).
	possessionStartSeconds float64
	// Ball state at the start of the current ball sub-step.
	substepBallPosition Vector2F
	substepBallZ        float64
	homeFamiliarity     float64
	awayFamiliarity     float64
	// Index 0 is the home side, 1 the away side.
	shouts [2]ShoutState
	// Sliders in force (tactics plus shout), refreshed when they change.
	effectiveSliders [2]StrategySliders
	// The score changed (or was set) since the sliders were computed.
	slidersStale bool
	// Underdog caution by side (0 = not the weaker side), from the XIs.
	underdogShares [2]float64
	// Team-talk modifiers by side and half.
	teamTalks [2][2]float64
	// Slot roles by side (index 0 home), indexed by formation slot.
	slotTactics    [2][16]SlotTactic
	keeperProfiles [2]RoleProfile
	// Kick-off slot positions by side (lineup coordinates): the keys that
	// match slots to the tactics' role instructions.
	slotAnchors [2][]Vector2F
	// Instructions each side (index 0 home) gives against opposing players.
	oppositionOrders [2][]PlayerInstruction
	// Per-step target blends: home tactical/urgent, away tactical/urgent.
	targetBlends    [4]float64
	separationOrder [32]uint8
	separationCount int
	// Positions after the last separation pass and whether it found no
	// contact (an unchanged, contact-free field needs no new pass).
	separationPositions [32]Vector2F
	separationClear     bool
	// Ball situation at the last tactical refresh (event-driven refresh).
	tacticalOwner *Player
	tacticalFlags uint8
	// Seconds and anchor positions since the last load accounting.
	loadSeconds float64
	loadAnchors []Vector2F
	// Opponent slot each outfield player marks (-1 when unassigned).
	markAssignments [32]int8
	// FootballEngine module state (FE_PORT_PLAN): M1 defensive tasks and
	// episodes (fe_m1_*.go) and M3 per-player steering memory (fe_m3_*.go).
	// Sized by initFeState; unused while FMENGINE_OFF switches the module off.
	feM1Data feM1State
	feM3Data feM3State
	// M4 evaluator memory (fe_m4_shape.go) and M5 attack episodes
	// (fe_m5_collective.go).
	feM4Data feM4State
	// Per-match configuration and diagnostics (config.go, diagnostics.go):
	// set once at construction, never shared between matches.
	cfg  EngineConfig
	diag Diagnostics
	// Per-match FE M6 state (formerly a global map keyed by engine).
	feM6Data feM6State
	// Per-match diagnostics state (never read by the simulation).
	m2trace feM2TraceState
	// Spatial service grid (spatial.go), rebuilt every tactical refresh.
	spatial  spatialState
	audit    auditState
	feM5Data feM5State
	// Medical staff instructions by squad player (see setMedicalFlags).
	medicalFlags []Pair[PlayerID, uint8]
	// Pre-match condition of bench players, applied when they come on.
	benchConditions []Pair[PlayerID, float64]

	// External control (play mode); an index keeps the engine copyable.
	controlledIndex Optional[int]
	// Who was handed the controls (the slot may change hands).
	controlledPlayerId PlayerID
	// By side (index 0 home): ever controlled, and the actions played.
	everControlled         [2]bool
	controlledActions      [2]int
	playHalfMinutes        int
	pendingPlayHalfMinutes int
	controlInput           MatchPlayerInput
	controlActionRemaining float64
	lastInputAction        MatchInputAction
	controlledShot         ControlledShot
	inputLog               []MatchInputRecord
	inputCursor            int
	commandLog             []MatchCommandRecord
	commandCursor          int
	// Outfield slot positions by side (index 0 home), lineup coordinates.
	formations [2][]Vector2F
	// Familiarity lost to a change of shape and the seconds it still lasts.
	reshapeFamiliarityCost  [2]float64
	reshapeSecondsRemaining [2]float64

	highlights            []MatchHighlight
	highlightTriggerCount uint64
	lastTriggerType       MatchEventType
	lastTriggerSeconds    float64
	lastTriggerMinute     float64
	playbackMode          MatchPlaybackMode
	playbackSpeed         float64
	highlightSpeed        float64
	scheduledHighlight    Optional[MatchHighlight]
	// Bumped by manual interventions; stale predictions are discarded.
	inputRevision          uint32
	scheduledRevision      uint32
	homePossessionMinutes  float64
	awayPossessionMinutes  float64
	matchTimeMinutes       float64
	homeScore              int
	awayScore              int
	lastUpdateStepCount    int
	droppedSimulationSteps uint64
}

// initFeState sizes the FE module state (feM1Data, feM3Data) to the players
// on the pitch and resets it. The module accessors (feM1Of, feM3) call it
// when the slot count changes, so it also covers engines whose players are
// placed after construction.
func (e *MatchEngine) initFeState() {
	n := len(e.players)
	m1 := &e.feM1Data
	*m1 = feM1State{
		shape:      make([]feM1Vec, n),
		shapeSet:   make([]bool, n),
		prevIntent: make([]PlayerIntent, n),
	}
	for i := range m1.sides {
		side := &m1.sides[i]
		side.tasks = make([]feM1Task, n)
		side.effort = make([]float64, n)
		side.individual = make([]bool, n)
		side.episode.primary, side.episode.cover = -1, -1
		side.rest = [2]int{-1, -1}
		side.clearTasks()
	}
	e.feM3Data = feM3State{}
	e.feM4Data = feM4State{}
	e.feM5Data = feM5State{}
}
