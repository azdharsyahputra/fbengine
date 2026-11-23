// Ours (not from FE/P12): per-player individuality for M3 steering, so a side
// stops shifting like a rigid table-football bar. Reaction delay before a new
// target is followed (the delay pattern base - skill·attribute follows FE
// KeeperSave.fs / P12 keeper reactionRemaining; the cognitive-fatigue factor
// is FE Systems/Players/Perception.fs:117-123), a shape deadband, per-player and
// per-run effort, gently curved runs, the idle shuffle, the unit shift (a
// unit reads its common target drift together, each player his own offset
// later) and the back-line feed-forward are ours.
package engine

import "math"

// Tunables (ours).
const (
	// Reaction delay (s) = base - anticipation·a - concentration·c, clamped.
	feM3ReactionBase          = 0.85
	feM3ReactionAnticipation  = 0.35
	feM3ReactionConcentration = 0.20
	feM3ReactionMin           = 0.20
	feM3ReactionMax           = 0.80
	// Per-event jitter of the delay (uniform range, multiplier).
	feM3ReactionJitterLow  = 0.55
	feM3ReactionJitterHigh = 1.65
	// Personal low-pass rate = base + anticipation·a (± jitter), times P12's.
	feM3ResponseBase         = 0.65
	feM3ResponseAnticipation = 0.6
	feM3ResponseJitter       = 0.15
	// Shares of the base delay by situation.
	feM3TaskDelayShare   = 0.55 // new task (support, run, cover, mark)
	feM3ReplanDelayShare = 0.30 // already running, target moved again
	// The back four out of possession move as one unit: a common delay (s),
	// Player12's common target rate, no personal pace or curve.
	feM3BackLineDelay = 0.25
	// Deadband (m) before a holding player moves: base + (1-concentration)·span.
	feM3ShapeDeadbandBase = 2.0
	feM3ShapeDeadbandSpan = 1.5
	// Back four, out of possession: depth deadband (m) so the line stays flat.
	feM3BackLineDepthDeadband = 0.5
	feM3TaskDeadband          = 1.0
	// A run is over when the player is this close and this slow.
	feM3SettleMetres = 0.6
	feM3SettleSpeed  = 1.2
	// Personal effort = base + workrate·w + (acceleration-0.6)·a (± 4 % fixed).
	feM3EffortBase         = 0.90
	feM3EffortWorkRate     = 0.18
	feM3EffortAcceleration = 0.10
	feM3EffortFixedJitter  = 0.04
	// Per-run effort multiplier range.
	feM3RunJitterLow  = 0.70
	feM3RunJitterHigh = 1.30
	// Curved runs: |bend| in radians, faded in over the first metres.
	feM3BendMin        = 0.25
	feM3BendSpan       = 0.30
	feM3BendMinRun     = 4.0
	feM3BendFadeMetres = 3.0
	// A tracking run redraws its bend and effort every leg (s).
	feM3LegMin            = 2.0
	feM3LegMax            = 4.5
	feM3StraightBendShare = 0.5
	// Weave: the heading of a tracking run swings slowly, ± amplitude (rad)
	// over a personal period (s), so long runs are gently curved.
	feM3WeaveAmplitudeMin  = 0.22
	feM3WeaveAmplitudeSpan = 0.18
	feM3WeavePeriodMin     = 4.0
	feM3WeavePeriodSpan    = 3.0
	feM3WeaveMinSpeed      = 1.5
	feM3StraightBendStart  = 6.0
	// Recovery runs bend goal-side: weight of the own-goal direction.
	feM3GoalSideWeight = 0.45
	feM3GoalSideFade   = 12.0
	// Idle shuffle while settled: points within a personal radius (m), the
	// nearer part of it skipped, walked to at a drawn pace (m/s) and left
	// for the next on coming within Reach (m).
	feM3IdleRadiusMin     = 1.2
	feM3IdleRadiusMax     = 2.2
	feM3IdleRadiusShare   = 0.4
	feM3IdleSpeedMin      = 0.5
	feM3IdleSpeedMax      = 0.85
	feM3IdleReach         = 0.6
	feM3IdleBackLineDepth = 0.35
	feM3IdleBallMetres    = 6.0
	// Restart walking pace is redrawn every this many steps.
	feM3WalkRedrawSteps = 50
	// Shape adjustments walk or jog by the length of the run at its start:
	// cap = walk + per-metre·run (m/s).
	feM3AdjustWalk     = 0.9
	feM3AdjustPerMetre = 0.20
	// Unit shift: the drift of a player's target that his unit shares (its
	// mean, blended toward the side's mean by TeamShare) is read at his own
	// rate, his own offset from it at OwnShare of that rate (the back four
	// out of possession at BackLineOwnShare, so the line stays flat).
	feM3UnitOwnShare         = 0.07
	feM3UnitBackLineOwnShare = 0.5
	feM3TeamShare            = 0.6
	// Target feed-forward: the steered spot's velocity is low-passed at this
	// rate per step; a spot moving faster than Jump m/s has jumped.
	feM3TargetVelBlend = 0.3
	feM3TargetVelJump  = 12.0
)

type feM3Class int

const (
	feM3Direct feM3Class = iota // press, loose ball, receive, carry, keeper
	feM3Task                    // support, runs, cover, mark
	feM3Shape                   // hold / recover shape
)

func feM3ClassOf(intent PlayerIntent) feM3Class {
	switch intent {
	case PlayerIntent_PRESS_BALL, PlayerIntent_CLAIM_LOOSE_BALL, PlayerIntent_RECEIVE_PASS,
		PlayerIntent_CARRY_BALL, PlayerIntent_GOALKEEP:
		return feM3Direct
	case PlayerIntent_HOLD_SHAPE, PlayerIntent_RECOVER_SHAPE:
		return feM3Shape
	}
	return feM3Task
}

// feM3Slot is the per-slot steering memory.
type feM3Slot struct {
	occupant                              *Player
	draws                                 uint64
	reaction                              float64 // base reaction delay (s)
	deadband                              float64 // shape deadband (m)
	personal                              float64 // personal effort multiplier
	pending                               bool
	timer                                 float64
	moving                                bool
	runScale                              float64
	bend                                  float64 // radians, signed
	runLength                             float64 // metres to the target when the run started
	legTimer                              float64
	weaveAmplitude, weaveRate, weavePhase float64
	bendStart                             float64
	filtered                              Vector2F // perceived target (low-passed TacticalTarget)
	filterReady                           bool
	response                              float64 // personal target-response rate multiplier
	lastIntent                            PlayerIntent
	idleRadius, idleSpeed                 float64
	idling                                bool
	walkScale                             float64
	walkRedraw                            uint64
	idle                                  Vector2F // metres offset while settled
	carryY                                float64  // last FE carry target y (m), for the inward hysteresis
	carrying                              bool
	// Velocity (m/s, low-passed) of the spot he steers at, for the target
	// feed-forward; lastTarget is that spot on the previous step.
	targetVel   Vector2F
	lastTarget  Vector2F
	targetReady bool
}

// trackTarget updates the steered spot's velocity estimate.
func (s *feM3Slot) trackTarget(target Vector2F, dt float64) {
	if !s.targetReady || dt <= 0.0 {
		s.lastTarget, s.targetVel, s.targetReady = target, Vector2F{}, true
		return
	}
	v := toMetres(Vector2F{(target.X - s.lastTarget.X) / dt, (target.Y - s.lastTarget.Y) / dt})
	if math.Hypot(v.X, v.Y) > feM3TargetVelJump {
		// A jump of the target (new task) is not a motion to follow.
		v = Vector2F{}
	}
	s.targetVel.X += (v.X - s.targetVel.X) * feM3TargetVelBlend
	s.targetVel.Y += (v.Y - s.targetVel.Y) * feM3TargetVelBlend
	s.lastTarget = target
}

type feM3State struct {
	lastStep uint64
	started  bool
	slots    [b3Lanes]feM3Slot
	// Mean target drift (raw target - perceived target, pitch units) of each
	// side's units this step: back line, midfield, front (feM3UnitOf).
	units [2][3]feM3UnitDrift
}

type feM3UnitDrift struct {
	sum Vector2F
	n   int
}

// feM3UnitOf is the unit of a player's role: 0 back line, 1 midfield, 2 front.
func feM3UnitOf(player *MatchPlayer) int {
	if player.Player == nil {
		return 1
	}
	switch player.Player.GetRole() {
	case PlayerRole_CB, PlayerRole_LB, PlayerRole_RB:
		return 0
	case PlayerRole_CDM, PlayerRole_CM:
		return 1
	}
	return 2
}

// feM3InUnit: a player whose target moves with his unit. Direct intents
// (press, loose ball, receive, carry) and individual M1 tasks (cover, screen,
// box or tight marking) do not.
func (e *MatchEngine) feM3InUnit(player *MatchPlayer) bool {
	return feM3ClassOf(player.Intent) != feM3Direct && !e.feM1Individual(player) &&
		!e.feM4Pushed(player)
}

func feM3RawTarget(player *MatchPlayer) Vector2F {
	return Vector2F{hClamp(player.TacticalTarget.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
		hClamp(player.TacticalTarget.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
}

// gatherUnits sums each unit's target drift before the players are moved, so
// a shift of the whole unit is perceived as one (see feM3Follow).
func (st *feM3State) gatherUnits(e *MatchEngine, slots []uint8) {
	st.units = [2][3]feM3UnitDrift{}
	for _, index := range slots {
		player := &e.players[index]
		s := &st.slots[index]
		if player.IsGoalkeeper || s.occupant != player.Player || !s.filterReady ||
			!e.feM3InUnit(player) {
			continue
		}
		raw := feM3RawTarget(player)
		u := &st.units[hBoolIndex(player.IsHomeTeam)][feM3UnitOf(player)]
		u.sum.X += raw.X - s.filtered.X
		u.sum.Y += raw.Y - s.filtered.Y
		u.n++
	}
}

// feM3 returns the engine's M3 steering memory (MatchEngine.feM3Data).
func (e *MatchEngine) feM3() *feM3State {
	if len(e.feM1Data.shape) != len(e.players) {
		e.initFeState()
	}
	return &e.feM3Data
}

// beginStep forgets running state after a stoppage: on the first live step
// everyone re-reads the play.
func (st *feM3State) beginStep(e *MatchEngine) {
	gap := e.stepCounter - st.lastStep
	if !st.started || gap > 3*uint64(max(e.stepTicks, 1)) {
		for index := range st.slots {
			st.slots[index].pending = false
			st.slots[index].moving = true
			st.slots[index].idle = Vector2F{}
			st.slots[index].idling = false
			st.slots[index].runLength = 100.0
			st.slots[index].filterReady = false
			st.slots[index].targetReady = false
		}
	}
	st.started = true
	st.lastStep = e.stepCounter
}

// feM3Attr reads an FM attribute (fm.<name>, 1-100) as 0..1.
func feM3Attr(p *Player, name string, fallback float64) float64 {
	if p == nil || p.Stats == nil {
		return fallback
	}
	if v, ok := p.Stats["fm."+name]; ok && v > 0 {
		return hClamp(v/100.0, 0.0, 1.0)
	}
	return fallback
}

// rand is a deterministic per-player draw in [0, 1); it does not touch the
// engine RNG streams.
func (s *feM3Slot) rand(e *MatchEngine, salt uint64) float64 {
	s.draws++
	id := uint64(0)
	if s.occupant != nil {
		id = uint64(s.occupant.Id)
	}
	h := splitMix64(uint64(e.matchSeed)<<32 ^ id*0x9e3779b97f4a7c15 ^ s.draws<<16 ^ salt)
	return float64(h>>11) * 0x1p-53
}

func (st *feM3State) slot(e *MatchEngine, index int, player *MatchPlayer) *feM3Slot {
	s := &st.slots[index]
	if s.occupant == player.Player {
		return s
	}
	*s = feM3Slot{occupant: player.Player, moving: true, runScale: 1.0, lastIntent: player.Intent, runLength: 100.0}
	anticipation := feM3Attr(player.Player, "anticipation", player.Vision)
	concentration := feM3Attr(player.Player, "concentration", 0.6)
	workRate := feM3Attr(player.Player, "workrate", 0.6)
	acceleration := feM3Attr(player.Player, "acceleration", player.Pace)
	s.reaction = hClamp(feM3ReactionBase-feM3ReactionAnticipation*anticipation-
		feM3ReactionConcentration*concentration, feM3ReactionMin, feM3ReactionMax)
	s.response = feM3ResponseBase + feM3ResponseAnticipation*anticipation +
		feM3ResponseJitter*(2.0*s.rand(e, 11)-1.0)
	s.weaveAmplitude = feM3WeaveAmplitudeMin + feM3WeaveAmplitudeSpan*s.rand(e, 13)
	s.weaveRate = TWO_PI / (feM3WeavePeriodMin + feM3WeavePeriodSpan*s.rand(e, 14))
	s.weavePhase = TWO_PI * s.rand(e, 15)
	s.idleRadius = feM3IdleRadiusMin + (feM3IdleRadiusMax-feM3IdleRadiusMin)*s.rand(e, 16)
	s.deadband = feM3ShapeDeadbandBase + (1.0-concentration)*feM3ShapeDeadbandSpan
	s.personal = (feM3EffortBase + feM3EffortWorkRate*workRate +
		feM3EffortAcceleration*(acceleration-0.6)) *
		(1.0 + feM3EffortFixedJitter*(2.0*s.rand(e, 1)-1.0))
	return s
}

// feM3Cognitive is FE's cognitive fatigue (Perception.fs:117-123): 0.5 +
// 0.5·sigmoid(3·(condition-0.5)), normalised so a fresh player is 1.
func feM3Cognitive(stamina float64) float64 {
	sig := func(x float64) float64 { return 1.0 / (1.0 + math.Exp(-x)) }
	return (0.5 + 0.5*sig(3.0*(stamina-0.5))) / (0.5 + 0.5*sig(1.5))
}

func (s *feM3Slot) delay(e *MatchEngine, player *MatchPlayer, share, jitterLow, jitterHigh float64) float64 {
	jitter := jitterLow + (jitterHigh-jitterLow)*s.rand(e, 2)
	return s.reaction * share * jitter / feM3Cognitive(player.Stamina)
}

func (s *feM3Slot) newRun(e *MatchEngine, player *MatchPlayer, class feM3Class, runMetres float64, straight bool) {
	s.moving = true
	s.idle = Vector2F{}
	s.idling = false
	s.runLength = runMetres
	s.newLeg(e, class, runMetres, straight)
}

// newLeg redraws the run's effort and bend; a long tracking run is a chain
// of legs, each gently curved its own way.
func (s *feM3Slot) newLeg(e *MatchEngine, class feM3Class, runMetres float64, straight bool) {
	s.runScale = feM3RunJitterLow + (feM3RunJitterHigh-feM3RunJitterLow)*s.rand(e, 3)
	s.legTimer = feM3LegMin + (feM3LegMax-feM3LegMin)*s.rand(e, 12)
	s.bend = 0.0
	s.bendStart = 1.0
	if class != feM3Direct && runMetres >= feM3BendMinRun {
		sign := 1.0
		if s.rand(e, 4) < 0.5 {
			sign = -1.0
		}
		s.bend = sign * (feM3BendMin + feM3BendSpan*s.rand(e, 5))
		if straight {
			// Back four and markers: a smaller arc, only while far away.
			s.bend *= feM3StraightBendShare
			s.bendStart = feM3StraightBendStart
		}
	}
}

// feM3Follow decides which target the player actually heads for (written to
// MovementTarget). Direct intents follow at once. The others read the play
// through Player12's target low-pass (P12 targetBlends), at a personal rate;
// a settled player only sets off once that perceived target has left his
// deadband and his own reaction delay has passed, then tracks it until he
// settles again.
func (e *MatchEngine) feM3Follow(player *MatchPlayer, s *feM3Slot, raw Vector2F, backLine bool, dt float64) {
	class := feM3ClassOf(player.Intent)
	intentChanged := player.Intent != s.lastIntent
	wasDirect := feM3ClassOf(s.lastIntent) == feM3Direct
	s.lastIntent = player.Intent
	if class == feM3Direct {
		if player.Intent == PlayerIntent_GOALKEEP {
			// Keepers keep Player12's urgent blend.
			blend := e.targetBlends[hBoolIndex(player.IsHomeTeam)*2+1]
			player.MovementTarget.X += (raw.X - player.MovementTarget.X) * blend
			player.MovementTarget.Y += (raw.Y - player.MovementTarget.Y) * blend
		} else {
			player.MovementTarget = raw
		}
		s.filtered = player.MovementTarget
		s.pending = false
		s.moving = true
		s.idle = Vector2F{}
		return
	}
	if wasDirect || !s.filterReady {
		s.filtered = player.MovementTarget
		s.filterReady = true
	}
	blend := e.targetBlends[hBoolIndex(player.IsHomeTeam)*2]
	if !backLine {
		blend = 1.0 - math.Pow(1.0-blend, s.response)
	}
	own := Vector2F{raw.X - s.filtered.X, raw.Y - s.filtered.Y}
	if u := e.feM3Data.units[hBoolIndex(player.IsHomeTeam)][feM3UnitOf(player)]; u.n >= 2 && e.feM3InUnit(player) {
		share := feM3UnitOwnShare
		if backLine {
			share = feM3UnitBackLineOwnShare
		}
		// The unit's common shift is read at the personal rate, a player's
		// own offset from it more slowly: the unit shifts together.
		common := Vector2F{u.sum.X / float64(u.n), u.sum.Y / float64(u.n)}
		tx, ty, tn := 0.0, 0.0, 0
		for _, v := range e.feM3Data.units[hBoolIndex(player.IsHomeTeam)] {
			tx += v.sum.X
			ty += v.sum.Y
			tn += v.n
		}
		common.X += (tx/float64(tn) - common.X) * feM3TeamShare
		common.Y += (ty/float64(tn) - common.Y) * feM3TeamShare
		s.filtered.X += common.X*blend + (own.X-common.X)*blend*share
		s.filtered.Y += common.Y*blend + (own.Y-common.Y)*blend*share
	} else {
		s.filtered.X += own.X * blend
		s.filtered.Y += own.Y * blend
	}

	if s.moving && !s.pending {
		player.MovementTarget = s.filtered
		s.legTimer -= dt
		if s.legTimer <= 0.0 {
			straight := backLine || player.Intent == PlayerIntent_MARK_OPPONENT
			s.newLeg(e, class, distance(player.Position, s.filtered), straight)
		}
		settle := distance(player.Position, player.MovementTarget)
		if settle < feM3SettleMetres && length(player.Velocity) < feM3SettleSpeed {
			s.moving = false
			s.bend = 0.0
		}
		if !(intentChanged && class == feM3Task) {
			return
		}
	}
	drift := toMetres(Vector2F{s.filtered.X - player.MovementTarget.X, s.filtered.Y - player.MovementTarget.Y})
	thresholdX, thresholdY := feM3TaskDeadband, feM3TaskDeadband
	if class == feM3Shape {
		thresholdX, thresholdY = s.deadband, s.deadband
	}
	if backLine {
		thresholdX = math.Min(thresholdX, feM3BackLineDepthDeadband)
	}
	norm := math.Hypot(drift.X/thresholdX, drift.Y/thresholdY)
	if norm > 1.0 || (intentChanged && class == feM3Task) {
		if !s.pending {
			s.pending = true
			share := 1.0
			jitterLow, jitterHigh := feM3ReactionJitterLow, feM3ReactionJitterHigh
			switch {
			case backLine:
				share = feM3BackLineDelay / s.reaction
				jitterLow, jitterHigh = 0.9, 1.1
			case s.moving:
				share = feM3ReplanDelayShare
			case class == feM3Task:
				share = feM3TaskDelayShare
			}
			s.timer = s.delay(e, player, share, jitterLow, jitterHigh)
		}
	} else if norm < 0.5 && !s.moving {
		s.pending = false
	}
	if s.pending {
		s.timer -= dt
		if s.timer <= 0.0 {
			s.pending = false
			wasMoving := s.moving
			player.MovementTarget = s.filtered
			run := distance(player.Position, s.filtered)
			straight := backLine || player.Intent == PlayerIntent_MARK_OPPONENT
			if !wasMoving {
				s.newRun(e, player, class, run, straight)
			} else {
				s.runLength = math.Max(s.runLength, run)
			}
		}
	}
}

// feM3IdleOffset is the shuffle (metres from his spot) of a settled player:
// he walks to a nearby point, and on reaching it picks the next, so nobody
// stands frozen while he waits for the play (spec §1 "never fully still").
// The back four shuffle little in depth, and nobody shuffles near the ball.
func (e *MatchEngine) feM3IdleOffset(player *MatchPlayer, s *feM3Slot, backLine bool) Vector2F {
	if s.moving || feM3ClassOf(player.Intent) == feM3Direct ||
		distance(player.Position, e.ball.Position) < feM3IdleBallMetres {
		s.idling = false
		s.idle = Vector2F{}
		return s.idle
	}
	here := toMetres(Vector2F{player.Position.X - player.MovementTarget.X,
		player.Position.Y - player.MovementTarget.Y})
	if !s.idling || math.Hypot(here.X-s.idle.X, here.Y-s.idle.Y) < feM3IdleReach {
		s.idling = true
		angle := TWO_PI * s.rand(e, 16)
		r := s.idleRadius * (feM3IdleRadiusShare + (1.0-feM3IdleRadiusShare)*s.rand(e, 17))
		s.idle = Vector2F{r * math.Cos(angle), r * math.Sin(angle)}
		if backLine {
			s.idle.X *= feM3IdleBackLineDepth
		}
		s.idleSpeed = feM3IdleSpeedMin + (feM3IdleSpeedMax-feM3IdleSpeedMin)*s.rand(e, 19)
	}
	return s.idle
}

// feM3Bend turns the run direction: a signed bend that fades out as the
// player nears the target (a gentle arc), and recovery runs curving toward
// their own goal (goal-side) while far from the spot.
func (e *MatchEngine) feM3Bend(player *MatchPlayer, s *feM3Slot, dirX, dirY, metres float64, defending bool, dt float64) (float64, float64) {
	weave := 0.0
	if feM3ClassOf(player.Intent) != feM3Direct && length(player.Velocity) > feM3WeaveMinSpeed {
		s.weavePhase = math.Mod(s.weavePhase+s.weaveRate*dt, TWO_PI)
		weave = s.weaveAmplitude * math.Sin(s.weavePhase)
		if s.bendStart > 1.0 {
			weave *= feM3StraightBendShare
		}
	}
	if total := s.bend + weave; math.Abs(total) > EPSILON {
		angle := total * hClamp((metres-s.bendStart)/feM3BendFadeMetres, 0.0, 1.0)
		c, sn := math.Cos(angle), math.Sin(angle)
		dirX, dirY = dirX*c-dirY*sn, dirX*sn+dirY*c
	}
	if defending && feM3ClassOf(player.Intent) == feM3Shape {
		ownGoalX := -0.5
		if !player.IsHomeTeam {
			ownGoalX = 0.5
		}
		pos := toMetres(Vector2F{player.Position.X - 0.5, player.Position.Y - 0.5})
		gx, gy := ownGoalX*Pitch_LENGTH_METRES-pos.X, -pos.Y
		gl := math.Hypot(gx, gy)
		backward := dirX * gx / math.Max(gl, EPSILON)
		if gl > EPSILON && backward > 0.3 {
			w := feM3GoalSideWeight * hClamp((metres-4.0)/feM3GoalSideFade, 0.0, 1.0)
			dirX += w * gx / gl
			dirY += w * gy / gl
			n := math.Hypot(dirX, dirY)
			dirX, dirY = dirX/n, dirY/n
		}
	}
	return dirX, dirY
}

// feM3AdjustCap is the walk/jog cap (m/s) of a shape adjustment.
func (s *feM3Slot) feM3AdjustCap(player *MatchPlayer) float64 {
	if feM3ClassOf(player.Intent) != feM3Shape {
		return math.MaxFloat64
	}
	return (feM3AdjustWalk + feM3AdjustPerMetre*s.runLength) * s.runScale * s.personal
}

// feM3WalkScale is the personal restart walking pace multiplier: the
// player's effort times a slowly changing per-player draw.
func (e *MatchEngine) feM3WalkScale(index uint8, player *MatchPlayer) float64 {
	st := e.feM3()
	s := st.slot(e, int(index), player)
	if e.stepCounter >= s.walkRedraw {
		s.walkRedraw = e.stepCounter + uint64(feM3WalkRedrawSteps)
		s.walkScale = feM3RunJitterLow + (feM3RunJitterHigh-feM3RunJitterLow)*s.rand(e, 18)
	}
	return s.personal * s.walkScale
}
