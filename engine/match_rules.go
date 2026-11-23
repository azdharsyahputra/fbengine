// Ported from Player12 src/model/match_rules.h and src/model/match_rules.cpp (GPL-3.0)

package engine

import "math"

// Pure, deterministic rules used by the match engine (namespace MatchRules).
// Everything here is a function of its arguments (random rolls are passed
// in). Types nested in the namespace drop the prefix (StoppageLog, Knockout,
// FoulSanction, FoulContext, TackleContext); functions keep it
// (MatchRules_<name>).

// StoppageLog records stoppages during one half; drives the added time.
type StoppageLog struct {
	Goals         int
	Substitutions int
	Cards         int
	Injuries      int
	Penalties     int
}

// MatchRules_computeAddedMinutes returns whole added minutes for a period
// (1-2, or 3-4 in extra time) from its stoppages.
func MatchRules_computeAddedMinutes(log StoppageLog, period int) int {
	base := Stoppage_EXTRA_TIME_BASE_MINUTES
	if period == 1 {
		base = Stoppage_FIRST_HALF_BASE_MINUTES
	} else if period == 2 {
		base = Stoppage_SECOND_HALF_BASE_MINUTES
	}
	minutes := base + float64(log.Goals)*Stoppage_MINUTES_PER_GOAL +
		float64(log.Substitutions)*Stoppage_MINUTES_PER_SUBSTITUTION +
		float64(log.Cards)*Stoppage_MINUTES_PER_CARD +
		float64(log.Injuries)*Stoppage_MINUTES_PER_INJURY +
		float64(log.Penalties)*Stoppage_MINUTES_PER_PENALTY
	return min(max(int(math.Round(minutes)), Stoppage_MIN_ADDED_MINUTES), Stoppage_MAX_ADDED_MINUTES)
}

// MatchRules_periodStartMinute is the clock minute at which a period starts
// (0, 45, 90, 105).
func MatchRules_periodStartMinute(period int) float64 {
	if period <= 1 {
		return 0.0
	}
	if period == 2 {
		return Timing_HALF_TIME_MINUTE
	}
	return Timing_FULL_TIME_MINUTE + float64(period-3)*Timing_EXTRA_TIME_HALF_MINUTES
}

// MatchRules_periodEndMinute is the clock minute at which a period's
// regulation time ends (45, 90, 105, 120).
func MatchRules_periodEndMinute(period int) float64 {
	if period <= 1 {
		return Timing_HALF_TIME_MINUTE
	}
	if period == 2 {
		return Timing_FULL_TIME_MINUTE
	}
	return Timing_FULL_TIME_MINUTE + float64(period-2)*Timing_EXTRA_TIME_HALF_MINUTES
}

// Knockout describes how a level match is settled. League matches (the
// default) may end drawn.
type Knockout struct {
	Required bool
	// Goals scored in earlier legs by this match's home and away sides.
	HomeAggregate int
	AwayAggregate int
	// Two halves of extra time before the shootout.
	ExtraTime bool
}

// NewKnockout returns a Knockout with the C++ default member values.
func NewKnockout() Knockout { return Knockout{ExtraTime: true} }

// MatchRules_shootoutDecided reports whether a penalty shootout is decided.
func MatchRules_shootoutDecided(homeGoals, homeKicks, awayGoals, awayKicks int) bool {
	if KICKS := Timing_SHOOTOUT_KICKS; homeKicks <= KICKS && awayKicks <= KICKS {
		if homeGoals > awayGoals+(KICKS-awayKicks) || awayGoals > homeGoals+(KICKS-homeKicks) {
			return true
		}
		if homeKicks < KICKS || awayKicks < KICKS {
			return false
		}
	}
	return homeKicks == awayKicks && homeGoals != awayGoals
}

// MatchRules_computeMatchRating is the event-driven match rating: 6.0
// baseline, bounded to [3, 10]. teamGoalDifference is the player's team
// score minus the opponent's.
func MatchRules_computeMatchRating(stats *PlayerMatchStats, goalkeeper, defender bool, teamGoalDifference int) float64 {
	failedPasses := max(0, stats.PassesAttempted-stats.PassesCompleted)
	shotsOffTarget := max(0, stats.Shots-stats.ShotsOnTarget)
	rating := Rating_BASELINE + float64(stats.Goals)*Rating_GOAL +
		float64(stats.OwnGoals)*Rating_OWN_GOAL +
		float64(stats.Assists)*Rating_ASSIST +
		float64(stats.ShotsOnTarget)*Rating_SHOT_ON_TARGET +
		float64(shotsOffTarget)*Rating_SHOT_OFF_TARGET +
		float64(stats.KeyPasses)*Rating_KEY_PASS +
		float64(stats.PassesCompleted)*Rating_COMPLETED_PASS +
		float64(failedPasses)*Rating_FAILED_PASS +
		float64(stats.TacklesWon)*Rating_TACKLE_WON +
		float64(stats.Interceptions)*Rating_INTERCEPTION +
		float64(stats.Clearances)*Rating_CLEARANCE +
		float64(stats.AerialDuelsWon)*Rating_AERIAL_WON +
		float64(stats.Saves)*Rating_SAVE +
		float64(stats.FoulsCommitted)*Rating_FOUL +
		float64(stats.YellowCards)*Rating_YELLOW_CARD +
		float64(stats.RedCards)*Rating_RED_CARD

	if goalkeeper || defender {
		conceded := Rating_DEFENDER_GOAL_CONCEDED
		if goalkeeper {
			conceded = Rating_GOALKEEPER_GOAL_CONCEDED
		}
		rating += float64(stats.GoalsConceded) * conceded
		if stats.GoalsConceded == 0 && stats.MinutesPlayed >= Rating_CLEAN_SHEET_MINIMUM_MINUTES {
			cleanSheet := Rating_CLEAN_SHEET_DEFENDER
			if goalkeeper {
				cleanSheet = Rating_CLEAN_SHEET_GOALKEEPER
			}
			rating += cleanSheet * math.Min(1.0, stats.MinutesPlayed/Rating_FULL_MATCH_MINUTES)
		}
	}

	if teamGoalDifference != 0 {
		participation := math.Min(1.0, stats.MinutesPlayed/Rating_FULL_MATCH_MINUTES)
		bonus := -Rating_RESULT_BONUS
		if teamGoalDifference > 0 {
			bonus = Rating_RESULT_BONUS
		}
		rating += bonus * participation
	}
	return hClamp(rating, Rating_MINIMUM, Rating_MAXIMUM)
}

// MatchRules_playerHeightMetres is the player height in metres; unknown (0)
// heights fall back to the default.
func MatchRules_playerHeightMetres(heightCentimetres uint8) float64 {
	// Unknown or implausible heights fall back to the default height.
	if heightCentimetres < 150 {
		return Units_DEFAULT_PLAYER_HEIGHT_METRES
	}
	return float64(heightCentimetres) / 100.0
}

// MatchRules_headerReachMetres is the highest ball height (metres) an
// outfield player can head.
func MatchRules_headerReachMetres(heightMetres, physicality float64) float64 {
	return heightMetres - Aerial_HEAD_OFFSET_METRES + Aerial_BASE_JUMP_METRES +
		hClamp(physicality, 0.0, 1.0)*Aerial_PHYSICALITY_JUMP_METRES
}

// MatchRules_goalkeeperReachMetres is the highest ball height (metres) a
// goalkeeper can claim with the hands.
func MatchRules_goalkeeperReachMetres(heightMetres, goalkeeping float64) float64 {
	return heightMetres*Aerial_GOALKEEPER_ARM_REACH_RATIO +
		Aerial_GOALKEEPER_BASE_JUMP_METRES +
		hClamp(goalkeeping, 0.0, 1.0)*Aerial_GOALKEEPER_SKILL_JUMP_METRES
}

// MatchRules_goalkeeperHandlingRadiusMetres is the horizontal hand reach
// from the current body position at a ball height (C++ default diving =
// false).
func MatchRules_goalkeeperHandlingRadiusMetres(heightMetres, goalkeeping, ballHeightMetres float64, diving bool) float64 {
	verticalReach := MatchRules_goalkeeperReachMetres(heightMetres, goalkeeping)
	if ballHeightMetres < 0.0 || ballHeightMetres > verticalReach {
		return 0.0
	}
	shoulderHeight := heightMetres * Goalkeeper_SHOULDER_HEIGHT_SHARE
	stretch := math.Max(0.0, ballHeightMetres-shoulderHeight) /
		math.Max(Goalkeeper_MIN_VERTICAL_REACH_MARGIN, verticalReach-shoulderHeight)
	// A diving keeper's tilted torso contributes to the hand's reach;
	// lateral travel itself is already reflected in the body position.
	share := Goalkeeper_HAND_REACH_HEIGHT_SHARE
	if diving {
		share = Goalkeeper_DIVE_REACH_HEIGHT_SHARE
	}
	armReach := heightMetres * share
	return armReach * math.Sqrt(math.Max(0.0, 1.0-(stretch*stretch)))
}

// MatchRules_aerialDuelStrength is the relative strength of a player in an
// aerial duel for a ball at a height.
func MatchRules_aerialDuelStrength(heightMetres, physicality, ballHeightMetres float64) float64 {
	reach := MatchRules_headerReachMetres(heightMetres, physicality)
	margin := reach - ballHeightMetres
	if margin < 0.0 {
		return 0.0
	}
	// Being able to meet the ball comfortably matters, but beyond a few
	// centimetres of spare reach it is timing and strength that decide.
	return (0.35 + hClamp(physicality, 0.0, 1.0)) * (0.6 + math.Min(margin, 0.4))
}

// FoulSanction is MatchRules::FoulSanction.
type FoulSanction uint8

const (
	FoulSanction_NONE FoulSanction = iota
	FoulSanction_YELLOW
	FoulSanction_SECOND_YELLOW
	FoulSanction_RED
)

// FoulContext is MatchRules::FoulContext.
type FoulContext struct {
	// Uniform rolls in [0, 1).
	SeverityRoll float64
	CardRoll     float64
	// Per-match referee strictness, ~N(1, 0.13).
	Strictness float64
	// The foul stopped a promising attack (tactical foul).
	Tactical bool
	// Denied an obvious goal-scoring opportunity.
	DenyingGoalChance bool
	// The foul happened inside the offender's penalty area.
	InPenaltyArea         bool
	OffenderAlreadyBooked bool
	OffenderIsAway        bool
	// Scales the referee's lean toward the home side (0 = neutral venue).
	VenueBiasScale float64
}

// NewFoulContext returns a FoulContext with the C++ default member values.
func NewFoulContext() FoulContext { return FoulContext{Strictness: 1.0, VenueBiasScale: 1.0} }

// MatchRules_decideFoulSanction decides the card for a foul.
func MatchRules_decideFoulSanction(context FoulContext) FoulSanction {
	bias := Discipline_HOME_CARD_BIAS
	if context.OffenderIsAway {
		bias = Discipline_AWAY_CARD_BIAS
	}
	strictness := hClamp(context.Strictness, Discipline_MIN_CONTEXT_STRICTNESS, Discipline_MAX_CONTEXT_STRICTNESS) *
		(1.0 + (bias-1.0)*context.VenueBiasScale)

	if context.SeverityRoll >= 1.0-Discipline_SERIOUS_FOUL_PLAY_SHARE*strictness {
		return FoulSanction_RED
	}
	if context.DenyingGoalChance {
		redChance := Discipline_DOGSO_RED_CHANCE
		if context.InPenaltyArea {
			redChance = Discipline_PENALTY_AREA_DOGSO_RED_CHANCE
		}
		if context.CardRoll < redChance {
			return FoulSanction_RED
		}
	}

	reckless := context.SeverityRoll >= 1.0-Discipline_RECKLESS_SHARE
	yellowChance := Discipline_CARELESS_YELLOW_CHANCE
	if reckless {
		yellowChance = Discipline_RECKLESS_YELLOW_CHANCE
	}
	if context.Tactical || context.DenyingGoalChance {
		yellowChance = math.Max(yellowChance, Discipline_TACTICAL_YELLOW_CHANCE)
	}
	yellowChance = hClamp(yellowChance*strictness, 0.0, 0.95)
	if context.CardRoll >= yellowChance {
		return FoulSanction_NONE
	}
	if context.OffenderAlreadyBooked {
		return FoulSanction_SECOND_YELLOW
	}
	return FoulSanction_YELLOW
}

// TackleContext is everything that decides a ground challenge on a dribbler
// (MatchRules::TackleContext).
type TackleContext struct {
	Defending           float64
	Dribbling           float64
	DefenderPhysicality float64
	CarrierPhysicality  float64
	// Team pressing instruction in [0, 1].
	Pressing float64
	// Team risk-taking instruction in [0, 1].
	RiskTaking float64
	// Share of the touch the ball is still away from the carrier's foot.
	Exposure float64
	// The carrier is shielding the ball at walking pace.
	Shielding      bool
	Sliding        bool
	FromBehind     bool
	InPenaltyArea  bool
	DefenderBooked bool
	// AreaFoulScale (ours): the foul propensity scale inside the own penalty
	// area; 0 = Defending_PENALTY_AREA_FOUL_SCALE.
	AreaFoulScale float64
}

// NewTackleContext returns a TackleContext with the C++ default member
// values.
func NewTackleContext() TackleContext {
	return TackleContext{
		Defending: 0.5, Dribbling: 0.5, DefenderPhysicality: 0.5,
		CarrierPhysicality: 0.5, Pressing: 0.5, RiskTaking: 0.5,
	}
}

// MatchRules_tackleWinChance is the chance that a challenge reaches the ball
// first.
func MatchRules_tackleWinChance(context TackleContext) float64 {
	shield := 0.0
	if context.Shielding {
		shield = context.CarrierPhysicality * Defending_SHIELD_PHYSICALITY_PENALTY
	}
	slide := 0.0
	if context.Sliding {
		slide = Defending_SLIDE_WIN_PENALTY
	}
	behind := 0.0
	if context.FromBehind {
		behind = Defending_FROM_BEHIND_WIN_PENALTY
	}
	return hClamp(
		Defending_BASE_WIN_CHANCE+context.Defending*Defending_DEFENDING_WIN_BONUS-
			context.Dribbling*Defending_DRIBBLING_WIN_PENALTY+
			(context.DefenderPhysicality-context.CarrierPhysicality)*Defending_PHYSICALITY_DUEL_WEIGHT+
			context.Pressing*Defending_PRESSING_WIN_EFFECT+
			hClamp(context.Exposure, 0.0, 1.0)*Defending_EXPOSURE_WIN_BONUS-
			shield-slide-behind,
		Defending_MIN_WIN_CHANCE, Defending_MAX_WIN_CHANCE)
}

// MatchRules_tackleFoulPropensity is the propensity for the challenge to be
// a foul: applied in full to a missed challenge and scaled down for one that
// also reached the ball.
func MatchRules_tackleFoulPropensity(context TackleContext) float64 {
	// Defenders are far more careful inside their own penalty area, a booked
	// player picks his challenges, and a challenge while the ball is away from
	// the attacker's foot rarely catches the man.
	booked := 1.0
	if context.DefenderBooked {
		booked = Discipline_BOOKED_PLAYER_CAUTION
	}
	area := 1.0
	if context.InPenaltyArea {
		area = Defending_PENALTY_AREA_FOUL_SCALE
		if context.AreaFoulScale > 0 {
			area = context.AreaFoulScale
		}
	}
	behind := 1.0
	if context.FromBehind {
		behind = Defending_FROM_BEHIND_FOUL_FACTOR
	}
	slide := 1.0
	if context.Sliding {
		slide = Defending_SLIDE_FOUL_FACTOR
	}
	return (Defending_BASE_FOUL_CHANCE + context.RiskTaking*Defending_RISK_FOUL_BONUS +
		context.Pressing*Defending_PRESSING_FOUL_BONUS +
		(1.0-context.Defending)*Defending_TECHNIQUE_FOUL_BONUS) *
		booked * area * behind * slide *
		(1.0 - hClamp(context.Exposure, 0.0, 1.0)*Defending_EXPOSED_FOUL_RELIEF)
}

// MatchRules_staminaDrainPerSecond is the match condition drained per second
// for a player moving at speedRatio of their fresh top speed. endurance is
// the Stamina attribute in [0, 1].
func MatchRules_staminaDrainPerSecond(speedRatio, endurance, pressingIntensity float64) float64 {
	ratio := hClamp(speedRatio, 0.0, 1.5)
	sprint := math.Max(0.0, ratio-Fatigue_SPRINT_THRESHOLD) / (1.0 - Fatigue_SPRINT_THRESHOLD)
	work := Fatigue_IDLE_DRAIN + ratio*ratio*Fatigue_RUNNING_DRAIN +
		sprint*Fatigue_SPRINT_DRAIN +
		hClamp(pressingIntensity, 0.0, 1.0)*Fatigue_PRESSING_DRAIN
	return work * (Fatigue_ENDURANCE_BASE - hClamp(endurance, 0.0, 1.0)*Fatigue_ENDURANCE_RELIEF)
}
