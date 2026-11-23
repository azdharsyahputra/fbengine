// Ported from Player12 src/model/match_engine.cpp lines 2615–3364 (GPL-3.0)

package engine

import "math"

// b2Sign returns positive when cond holds, else negative (C++ `cond ? a : b`
// for the many ±1 direction factors).
func b2Sign(cond bool, positive, negative float64) float64 {
	if cond {
		return positive
	}
	return negative
}

// b2DoublingFor is the C++ conditional
// `pressers[1] == &player ? doubleUpPoint(player, *carrier, pressers[0]) : std::nullopt`.
func (e *MatchEngine) b2DoublingFor(player *MatchPlayer, carrier *MatchPlayer, pressers [2]*MatchPlayer) Optional[Vector2F] {
	if pressers[1] == player {
		return e.doubleUpPoint(player, carrier, pressers[0])
	}
	return Optional[Vector2F]{}
}

func (e *MatchEngine) refreshTacticalTargets(dt float64) {
	// Team planning happens in two passes: choose shared assignments (pressers,
	// runners, support and marks), then give each player a target and intent.
	// Bodies stay still until updateMovement integrates all targets, so player
	// iteration order cannot make later players see an already-moved opponent.
	// Add an off-ball behavior here; physical acceleration belongs downstream.
	carrier := e.findMatchPlayer(e.ball.PossessedBy)
	transitionSource := e.findMatchPlayer(e.ball.LastPossessor)
	// The players on the pitch of each side, in slot order (index 0 home).
	var sides [2][16]*MatchPlayer
	var sideSizes [2]int
	for i := range e.players {
		player := &e.players[i]
		side := hBoolIndex(player.IsHomeTeam)
		if active(player) && sideSizes[side] < len(sides[side]) {
			sides[side][sideSizes[side]] = player
			sideSizes[side]++
		}
	}
	onPitch := func(homeTeam bool) []*MatchPlayer {
		side := hBoolIndex(homeTeam)
		return sides[side][:sideSizes[side]]
	}
	// Nobody moves while the targets are chosen, so each side's offside line
	// is read once.
	offsideLines := [2]float64{e.offsideLine(true), e.offsideLine(false)}
	lineFor := func(attackingHome bool) float64 {
		return offsideLines[hBoolIndex(attackingHome)]
	}
	pressurePosition := e.ball.Position
	if carrier != nil {
		pressurePosition = carrier.Position
	}
	homeSliders := e.getEffectiveSliders(true)
	awaySliders := e.getEffectiveSliders(false)
	if e.cfg.Modules.M1Defence {
		// FE M1 (fe_m1_*.go): episodes and which side M1 drives this refresh.
		e.feM1Begin(carrier)
	}
	// FE M4 (fe_m4_shape.go): the blocks shift around the ball's predicted
	// control point while it travels (pressers keep the ball itself).
	shapeAnchor := e.feM4ShapeAnchor(carrier, pressurePosition)
	// FE M5 (fe_m5_collective.go): open, advance or close attack episodes
	// before the P12 selections, which leave duty holders alone.
	e.feM5Update(carrier)

	closestOutfieldPair := func(homeTeam bool) [2]*MatchPlayer {
		return e.closestOutfieldPair(homeTeam, carrier, pressurePosition)
	}

	homePressers := closestOutfieldPair(true)
	awayPressers := closestOutfieldPair(false)
	if e.periodElapsed(Shape_MARK_REFRESH_STEPS) {
		e.assignMarks()
	}

	homeRunners := e.selectAttackingRunners(true, carrier, transitionSource)
	awayRunners := e.selectAttackingRunners(false, carrier, transitionSource)

	runChannel := func(runner *MatchPlayer) float64 {
		return e.runChannel(runner)
	}

	selectFinalThirdPlayer := func(homeTeam bool, arrival TacticalArrival) *MatchPlayer {
		return e.selectFinalThirdPlayer(homeTeam, arrival, carrier, homeRunners, awayRunners)
	}

	homeMidfieldArrival := selectFinalThirdPlayer(true, TacticalArrival_MIDFIELDER)
	awayMidfieldArrival := selectFinalThirdPlayer(false, TacticalArrival_MIDFIELDER)
	homeFarPostRunner := selectFinalThirdPlayer(true, TacticalArrival_FAR_POST)
	awayFarPostRunner := selectFinalThirdPlayer(false, TacticalArrival_FAR_POST)
	homeOverlappingFullback := selectFinalThirdPlayer(true, TacticalArrival_FULLBACK)
	awayOverlappingFullback := selectFinalThirdPlayer(false, TacticalArrival_FULLBACK)

	selectActiveSupporters := func(homeTeam bool) [Shape_MAX_ACTIVE_SUPPORTERS]*MatchPlayer {
		runners := awayRunners
		midfieldArrival := awayMidfieldArrival
		farPostRunner := awayFarPostRunner
		overlappingFullback := awayOverlappingFullback
		if homeTeam {
			runners = homeRunners
			midfieldArrival = homeMidfieldArrival
			farPostRunner = homeFarPostRunner
			overlappingFullback = homeOverlappingFullback
		}
		return e.selectActiveSupporters(homeTeam, carrier, runners,
			midfieldArrival, farPostRunner, overlappingFullback)
	}

	homeSupporters := selectActiveSupporters(true)
	awaySupporters := selectActiveSupporters(false)
	var homeShortForward, awayShortForward *MatchPlayer
	if e.cfg.Modules.M5Collective {
		homeShortForward = e.selectShortForward(true, carrier, homeRunners, homeSupporters,
			homeMidfieldArrival, homeFarPostRunner, homeOverlappingFullback)
		awayShortForward = e.selectShortForward(false, carrier, awayRunners, awaySupporters,
			awayMidfieldArrival, awayFarPostRunner, awayOverlappingFullback)
	}

	findCoverOutlet := func(defendingHome bool) *MatchPlayer {
		return e.findCoverOutlet(defendingHome, carrier)
	}

	homeCoverOutlet := findCoverOutlet(true)
	awayCoverOutlet := findCoverOutlet(false)

	for i := range e.players {
		player := &e.players[i]
		if !active(player) {
			continue
		}
		goalkeeper := player.IsGoalkeeper
		sliders := &awaySliders
		teamPhase := e.awayPhase
		if player.IsHomeTeam {
			sliders = &homeSliders
			teamPhase = e.homePhase
		}
		attackDirection := b2Sign(player.IsHomeTeam, 1.0, -1.0)
		ownPossession := carrier != nil && carrier.IsHomeTeam == player.IsHomeTeam
		// While his side has the ball (or its pass is on the way) every slot
		// starts from its attacking-phase spot (role and in-possession shape),
		// otherwise from the formation.
		var ownControl bool
		if carrier != nil {
			ownControl = ownPossession
		} else {
			ownControl = e.lastControlledTeamHome.HasValue &&
				e.lastControlledTeamHome.Value == player.IsHomeTeam
		}
		target := player.BasePosition
		owner := TargetOwner_P12
		if ownControl {
			target = e.possessionAnchor(player)
		}
		previousIntent := player.Intent
		player.Intent = PlayerIntent_HOLD_SHAPE
		player.IsPressing = false
		player.IsMakingRun = false

		// The whole block follows the ball while preserving its formation. This
		// produces recognizable defensive, middle and attacking lines rather than
		// twenty outfield players independently chasing one point.
		widthScale := Shape_MIN_WIDTH_SCALE +
			sliders.WidthUsage*Shape_WIDTH_SLIDER_SCALE
		target.Y = Pitch_CENTRE +
			(target.Y-Pitch_CENTRE)*widthScale
		target.X += (shapeAnchor.X - Pitch_CENTRE) *
			(Shape_BASE_LONGITUDINAL_SHIFT +
				sliders.Compactness*Shape_COMPACTNESS_LONGITUDINAL_SHIFT)
		target.Y += (shapeAnchor.Y - Pitch_CENTRE) *
			(Shape_BASE_LATERAL_SHIFT +
				sliders.Compactness*Shape_COMPACTNESS_LATERAL_SHIFT)

		// Out of possession the block drops and compacts around the ball: the
		// back line sits deeper the closer the ball is to goal, and the lines
		// behind it squeeze into a 20-40 m long, ~45 m wide unit.
		var defending bool
		if !goalkeeper {
			if carrier != nil {
				defending = carrier.IsHomeTeam != player.IsHomeTeam
			} else {
				defending = e.lastControlledTeamHome.HasValue &&
					e.lastControlledTeamHome.Value != player.IsHomeTeam
			}
		}
		if defending {
			ownGoalDepth := func(x float64) float64 {
				if player.IsHomeTeam {
					return x
				}
				return 1.0 - x
			}
			ballDepth := ownGoalDepth(shapeAnchor.X)
			lineDepth := hClamp(
				Shape_BLOCK_LINE_BASE+e.feM10BlockLift()+ballDepth*Shape_BLOCK_LINE_BALL_FACTOR+
					sliders.Pressing*Shape_BLOCK_LINE_PRESSING,
				Shape_BLOCK_LINE_MIN, Shape_BLOCK_LINE_MAX)
			lengthShare := hClamp((ballDepth-Shape_BLOCK_SHORT_BALL_DEPTH)/
				(Shape_BLOCK_LONG_BALL_DEPTH-Shape_BLOCK_SHORT_BALL_DEPTH),
				0.0, 1.0)
			teamLength := (hLerp(Shape_BLOCK_SHORT_LENGTH, Shape_BLOCK_LONG_LENGTH, lengthShare) + e.feM10BlockLength()) *
				(1.0 - sliders.Compactness*Shape_BLOCK_COMPACTNESS_SQUEEZE)
			formationShare := hClamp(
				(ownGoalDepth(player.BasePosition.X)-Shape_BLOCK_FORMATION_BACK)/
					(Shape_BLOCK_FORMATION_FRONT-Shape_BLOCK_FORMATION_BACK),
				0.0, 1.0)
			depth := lineDepth + formationShare*teamLength
			if player.IsHomeTeam {
				target.X = depth
			} else {
				target.X = 1.0 - depth
			}
			target.Y = Pitch_CENTRE +
				(player.BasePosition.Y-Pitch_CENTRE)*
					(Shape_BLOCK_WIDTH_SCALE-
						sliders.Compactness*Shape_BLOCK_COMPACTNESS_NARROWING) +
				(shapeAnchor.Y-Pitch_CENTRE)*
					Shape_BLOCK_BALL_SIDE_SHIFT
			target.X += e.defensiveRoleShift(player)
		}

		if e.cfg.Modules.M1Defence && e.feM1Owns(player) {
			// FE M1 replaces the out-of-possession branch below (P12:3028-3140):
			// keep the block target as the shape target; feM1Assign sets the task.
			e.feM1CaptureShape(player, target)
			player.TacticalTarget = target
			player.TargetOwner = TargetOwner_P12 // feM1Assign takes it over below
			player.UrgentMovement = false
			continue
		}
		if player == carrier {
			player.Intent = PlayerIntent_CARRY_BALL
			target = player.Position
			target.X += attackDirection *
				(Shape_CARRIER_BASE_ADVANCE +
					player.Dribbling*Shape_CARRIER_DRIBBLING_ADVANCE +
					sliders.RiskTaking*Shape_CARRIER_RISK_ADVANCE)
			if teamPhase == TeamPhase_ATTACKING_TRANSITION {
				target.X += attackDirection *
					Shape_ATTACKING_TRANSITION_CARRIER_ADVANCE
			}

			// Carry away from the nearest defender instead of running directly
			// through them, while gradually looking for a central shooting lane.
			var closestOpponent *MatchPlayer
			closestDistance := math.MaxFloat64
			for _, opponent := range onPitch(!player.IsHomeTeam) {
				opponentDistance := distance(opponent.Position, player.Position)
				if opponentDistance < closestDistance {
					closestDistance = opponentDistance
					closestOpponent = opponent
				}
			}
			// A wide forward cutting inside heads for the box; otherwise wide
			// players keep their lane to reach the byline and cross.
			role := player.Player.GetRole()
			wideLane := (role == PlayerRole_LM || role == PlayerRole_RM ||
				role == PlayerRole_LW || role == PlayerRole_RW ||
				role == PlayerRole_LB || role == PlayerRole_RB) &&
				math.Abs(player.Position.Y-Pitch_CENTRE) > Shape_WIDE_LANE_DEVIATION
			if e.cutsInside(player) {
				target.Y = player.Position.Y +
					(Pitch_CENTRE-player.Position.Y)*Shape_CUT_INSIDE_PULL
			} else if closestOpponent != nil &&
				closestDistance < Shape_CARRIER_EVASION_RANGE_METRES {
				evadeDirection := b2Sign(player.Position.Y <= closestOpponent.Position.Y, -1.0, 1.0)
				target.Y += evadeDirection * (Shape_CARRIER_BASE_EVASION +
					player.Dribbling*Shape_CARRIER_DRIBBLING_EVASION)
			} else if wideLane {
				target.Y = player.Position.Y
			} else {
				target.Y += (Pitch_CENTRE - player.Position.Y) *
					Shape_CARRIER_CENTRALITY
			}
			if e.cfg.Modules.M4Shape {
				if guarded := e.feM4BylineGuard(player, target); guarded != target {
					owner = TargetOwner_M4
					target = guarded
				}
			}
		} else if ownPossession {
			player.Intent = PlayerIntent_HOLD_SHAPE
			attackingProgress := carrier.Position.X
			if !player.IsHomeTeam {
				attackingProgress = 1.0 - carrier.Position.X
			}
			target.X += attackDirection *
				math.Max(0.0, attackingProgress-Shape_POSSESSION_PROGRESS_START) *
				Shape_POSSESSION_BLOCK_PROGRESS
			target.X += attackDirection * (Shape_SUPPORT_BASE_ADVANCE +
				sliders.OffensiveBias*Shape_SUPPORT_OFFENSIVE_ADVANCE)
			role := player.Player.GetRole()
			forward := role == PlayerRole_ST || role == PlayerRole_LW ||
				role == PlayerRole_RW || role == PlayerRole_CAM
			runners := awayRunners
			if player.IsHomeTeam {
				runners = homeRunners
			}
			runnerPosition := b2IndexOf(runners[:], player)
			committedRunner := runnerPosition >= 0
			dutyKind, dutyTarget, dutyOK := e.feM5DutyOf(player)
			if committedRunner {
				runnerSlot := runnerPosition
				if runnerSlot == 0 {
					player.Intent = PlayerIntent_RUN_IN_BEHIND
				} else {
					player.Intent = PlayerIntent_ATTACK_BOX
				}
				player.IsMakingRun = true
				defenderLine := lineFor(player.IsHomeTeam)
				var legalRunLine float64
				if player.IsHomeTeam {
					legalRunLine = math.Max(defenderLine, e.ball.Position.X)
				} else {
					legalRunLine = math.Min(defenderLine, e.ball.Position.X)
				}
				target.X += attackDirection *
					(Shape_RUN_BASE_ADVANCE +
						sliders.RiskTaking*Shape_RUN_RISK_ADVANCE)
				// Run timing is imperfect: runners hover around the line and now and
				// then drift beyond it, which is where offsides come from.
				timing := 0.0
				if defenderLine*attackDirection >= e.ball.Position.X*attackDirection {
					timing = (e.epochNoise(player.Player.GetId(), 0x0ff51de,
						RUN_TIMING_EPOCH_STEPS)*
						0.5 +
						0.25) *
						Passing_RUN_TIMING_GAMBLE
				}
				onsideTarget := legalRunLine -
					attackDirection*(Shape_RUN_ONSIDE_BUFFER-timing)
				runDepthTarget := onsideTarget - attackDirection*float64(runnerSlot)*
					Shape_SECONDARY_RUN_DEPTH_STAGGER
				target.X += (runDepthTarget - target.X) *
					Shape_RUN_DEPTH_TARGET_PULL
				if player.IsHomeTeam {
					target.X = math.Min(target.X, onsideTarget)
				} else {
					target.X = math.Max(target.X, onsideTarget)
				}

				channel := runChannel(player)
				if runnerSlot > 0 && runners[0] != nil {
					primaryChannel := runChannel(runners[0])
					if math.Abs(channel-primaryChannel) <
						Shape_MINIMUM_RUN_CHANNEL_SEPARATION {
						if primaryChannel <= Pitch_CENTRE {
							channel = Shape_RIGHT_INSIDE_FORWARD_CHANNEL
						} else {
							channel = Shape_LEFT_INSIDE_FORWARD_CHANNEL
						}
					}
				}
				target.Y += (channel - target.Y) * Shape_RUN_CHANNEL_BLEND

				// Only a runner caught clearly beyond the line checks back; one who
				// is marginally off relies on his timing (and sometimes gets caught).
				if e.isOffside(player, player.IsHomeTeam, defenderLine,
					Passing_RUN_TIMING_GAMBLE*0.75) {
					target.X -= attackDirection * Shape_ONSIDE_RECOVERY
				}
			} else {
				midfieldArrival := awayMidfieldArrival
				farPostRunner := awayFarPostRunner
				overlappingFullback := awayOverlappingFullback
				supporters := awaySupporters
				if player.IsHomeTeam {
					midfieldArrival = homeMidfieldArrival
					farPostRunner = homeFarPostRunner
					overlappingFullback = homeOverlappingFullback
					supporters = homeSupporters
				}
				supportPosition := b2IndexOf(supporters[:], player)
				shortForward := awayShortForward
				if player.IsHomeTeam {
					shortForward = homeShortForward
				}
				if player == midfieldArrival || player == farPostRunner {
					// Late runs into the box: the midfielder to the penalty spot on
					// the side away from the ball, the wide man to the far post; both
					// stay onside.
					player.Intent = PlayerIntent_ATTACK_BOX
					farPost := player == farPostRunner
					goalX := b2Sign(player.IsHomeTeam, 1.0, 0.0)
					ballSide := b2Sign(carrier.Position.Y < Pitch_CENTRE, -1.0, 1.0)
					arrivalX := goalX - attackDirection*b2Sign(farPost,
						Shape_FAR_POST_ARRIVAL_DEPTH, Shape_MIDFIELD_ARRIVAL_DEPTH)
					onside := lineFor(player.IsHomeTeam) -
						attackDirection*Shape_RUN_ONSIDE_BUFFER
					if player.IsHomeTeam {
						target.X = math.Min(arrivalX, onside)
					} else {
						target.X = math.Max(arrivalX, onside)
					}
					target.Y = Pitch_CENTRE -
						ballSide*b2Sign(farPost, Shape_FAR_POST_ARRIVAL_WIDTH,
							Shape_MIDFIELD_ARRIVAL_WIDTH)
				} else if player == overlappingFullback {
					player.Intent = PlayerIntent_OVERLAP
					target.X += attackDirection *
						Shape_FINAL_THIRD_FULLBACK_OVERLAP
				} else if dutyOK {
					// FE M5 duty (Intent.fs:253-262): support at the duty target;
					// the Runner of a depth pattern runs beyond.
					player.Intent = PlayerIntent_OFFER_SUPPORT
					target = dutyTarget
					owner = TargetOwner_M5
					if dutyKind == feM5Runner && e.feM5RunnerRuns(player.IsHomeTeam) {
						player.Intent = PlayerIntent_RUN_IN_BEHIND
						player.IsMakingRun = true
					}
				} else if supportPosition >= 0 {
					player.Intent = PlayerIntent_OFFER_SUPPORT
					supportSlot := supportPosition
					primarySide := b2Sign(
						supporters[Shape_NEAR_SUPPORT_SLOT] != nil &&
							supporters[Shape_NEAR_SUPPORT_SLOT].BasePosition.Y <= carrier.Position.Y,
						-1.0, 1.0)
					supportDepth := Shape_NEAR_SUPPORT_DEPTH
					supportWidth := Shape_NEAR_SUPPORT_WIDTH
					supportSide := primarySide
					if supportSlot == Shape_SQUARE_SUPPORT_SLOT {
						supportDepth = Shape_SQUARE_SUPPORT_DEPTH
						supportWidth = Shape_SQUARE_SUPPORT_WIDTH
						supportSide = -primarySide
					} else if supportSlot == Shape_TRAILING_SUPPORT_SLOT {
						supportDepth = Shape_TRAILING_SUPPORT_DEPTH
						supportWidth = Shape_TRAILING_SUPPORT_WIDTH
					} else if e.cfg.Modules.M10Structure {
						// M10 (ours): the near supporter shows diagonally ahead
						// of the ball, not behind it (real support triangle).
						supportDepth, supportWidth = e.feM10NearSupport()
					}

					if e.cfg.Modules.M5Collective && carrier.IsGoalkeeper {
						// Nobody supports a keeper from behind his own goal line:
						// the slots open up in front of him (ours).
						supportDepth = -(Shape_KEEPER_SUPPORT_DEPTH + supportDepth)
					}
					if linkSide, ok := e.feM5LinkSide(player.IsHomeTeam, carrier); ok {
						// Square option opposite the M5 Link, flipped inside when
						// that side runs off the pitch (ours).
						supportSide = -linkSide
						if y := carrier.Position.Y + supportSide*supportWidth; y < Pitch_PLAYER_MIN_Y || y > Pitch_PLAYER_MAX_Y {
							supportSide = linkSide
						}
					}
					target = carrier.Position
					target.X -= attackDirection * supportDepth
					target.Y += supportSide * supportWidth
					if teamPhase == TeamPhase_ATTACKING_TRANSITION {
						target.X += attackDirection *
							Shape_TRANSITION_SUPPORT_FORWARD_BONUS
					}
				} else if forward && (!e.cfg.Modules.M5Collective || player == shortForward) {
					player.Intent = PlayerIntent_OFFER_SUPPORT
					// Not every attacker runs beyond the defence. A complementary
					// forward checks toward the ball to form a passing triangle and drag
					// a marker.
					shortDepth := Shape_FORWARD_SHORT_OPTION_DEPTH
					if e.cfg.Modules.M5Collective {
						// Spec §3: the forward comes short about 9 m in front of
						// the ball, not behind it (ours; P12 checks back behind).
						shortDepth = -Shape_FORWARD_SHORT_OPTION_DEPTH
					}
					target.X = carrier.Position.X -
						attackDirection*shortDepth
					lateralDirection := b2Sign(player.BasePosition.Y <= carrier.Position.Y, -1.0, 1.0)
					target.Y = carrier.Position.Y +
						lateralDirection*Shape_FORWARD_SHORT_OPTION_LATERAL_SEPARATION
				}
				if !dutyOK && distance(target, carrier.Position) >
					Shape_MAX_SUPPORT_DISTANCE_METRES {
					target.X += (carrier.Position.X - target.X) *
						Shape_SUPPORT_LONGITUDINAL_PULL
					target.Y += (carrier.Position.Y - target.Y) *
						Shape_SUPPORT_LATERAL_PULL
				}
				auditBase := target
				if e.cfg.Modules.M4Shape && !dutyOK && (player.Intent == PlayerIntent_HOLD_SHAPE ||
					player.Intent == PlayerIntent_OFFER_SUPPORT) {
					// FE M4: the ball-side full-back pushes on (ours), then the
					// FieldEvaluator spacing refinement and onside clamp.
					if player.Intent == PlayerIntent_HOLD_SHAPE {
						target = e.feM4FullBackPush(player, target, carrier)
					}
					target = e.feM4Compact(player, target, carrier)
					target = e.feM4Refine(player, target, carrier, lineFor(player.IsHomeTeam))
				}
				auditM4 := target
				// M10 attack structure (fe_m10_structure.go): width and depth.
				target = e.feM10Structure(player, target, carrier, lineFor(player.IsHomeTeam))
				auditM10S := target
				target = e.feM10GetOpen(player, target, carrier, lineFor(player.IsHomeTeam))
				if auditM4 != auditBase {
					owner = TargetOwner_M4
				}
				if auditM10S != auditM4 || target != auditM10S {
					owner = TargetOwner_M10
				}
				if e.diag.Audit {
					e.auditLayers(player, dutyOK, auditBase, auditM4, auditM10S, target)
				}
			}
		} else if carrier != nil {
			pressers := awayPressers
			if player.IsHomeTeam {
				pressers = homePressers
			}
			if pressers[0] == player {
				player.Intent = PlayerIntent_PRESS_BALL
				player.IsPressing = true
				standOff := (Shape_PRESSING_STANDOFF_BASE +
					(1.0-sliders.Pressing)*
						Shape_PRESSING_STANDOFF_CAUTIOUS_BONUS) *
					e.pressStandOffShare(player, carrier)
				target = pressurePosition
				target.X -= attackDirection * standOff
				target.Y += e.weakFootShade(carrier)
			} else if doubling := e.b2DoublingFor(player, carrier, pressers); doubling.HasValue {
				// Ordered to double up: the second man closes the carrier too.
				player.Intent = PlayerIntent_COVER_PRESS
				player.IsPressing = true
				target = doubling.Value
			} else if pressers[1] == player &&
				sliders.Pressing > Shape_COVER_PRESS_MINIMUM {
				player.IsPressing = true
				coverOutlet := awayCoverOutlet
				if player.IsHomeTeam {
					coverOutlet = homeCoverOutlet
				}
				if coverOutlet != nil {
					player.Intent = PlayerIntent_BLOCK_PASSING_LANE
					target.X = pressurePosition.X +
						(coverOutlet.Position.X-pressurePosition.X)*
							Shape_COVER_LANE_INTERCEPTION_POINT
					target.Y = pressurePosition.Y +
						(coverOutlet.Position.Y-pressurePosition.Y)*
							Shape_COVER_LANE_INTERCEPTION_POINT
					target.X -= attackDirection * Shape_COVER_LANE_GOAL_SIDE_OFFSET
				} else {
					player.Intent = PlayerIntent_COVER_PRESS
					lateralSide := b2Sign(player.Position.Y <= pressurePosition.Y, -1.0, 1.0)
					target.X = pressurePosition.X -
						attackDirection*Shape_COVER_FALLBACK_DEPTH
					target.Y = pressurePosition.Y +
						lateralSide*Shape_COVER_FALLBACK_LATERAL_OFFSET
				}
			} else if tight := e.tightMarkTarget(player); tight != nil {
				// Under a tight-marking order he stays with his man in every phase,
				// goal-side and close enough to contest each ball played to him.
				player.Intent = PlayerIntent_MARK_OPPONENT
				target = e.tightMarkPoint(player, tight, target)
			} else if teamPhase == TeamPhase_DEFENSIVE_TRANSITION {
				// The nearest players counter-press above. Everyone else first gets
				// goal-side and narrows toward the danger before settling into the
				// normal defensive block.
				player.Intent = PlayerIntent_RECOVER_SHAPE
				target.X -= attackDirection * Shape_DEFENSIVE_TRANSITION_RECOVERY
				target.Y += (pressurePosition.Y - target.Y) *
					Shape_DEFENSIVE_TRANSITION_BALL_COMPACTNESS
			} else {
				// Zonal marking: shade toward the opponent assigned to this player
				// (one marker per opponent), without abandoning the formation anchor.
				assigned := e.markAssignments[e.slotOf(player)]
				var mark *MatchPlayer
				if assigned >= 0 && int(assigned) < len(e.players) {
					mark = &e.players[int(assigned)]
				}
				if mark != nil && (!active(mark) || mark.IsHomeTeam == player.IsHomeTeam) {
					mark = nil
				}
				if mark != nil {
					player.Intent = PlayerIntent_MARK_OPPONENT
					markWeight := Shape_BASE_MARK_WEIGHT +
						sliders.Compactness*Shape_COMPACTNESS_MARK_WEIGHT
					markPoint := mark.Position
					markDanger := mark.Position.X
					if player.IsHomeTeam {
						markDanger = 1.0 - mark.Position.X
					}
					if markDanger >= Shape_TIGHT_MARK_DANGER_DEPTH {
						// Near the own goal a marker stays tight and goal-side.
						markWeight = math.Max(markWeight, e.feM10TightWeight())
						markPoint.X -= attackDirection *
							e.feM10GoalSideMark() /
							Pitch_LENGTH_METRES
					}
					target.X += (markPoint.X - target.X) * markWeight
					target.Y += (markPoint.Y - target.Y) * markWeight
				}
			}
		} else {
			receivingPass := e.ball.IsPass && e.ball.PassByHome == player.IsHomeTeam
			pressers := awayPressers
			if player.IsHomeTeam {
				pressers = homePressers
			}
			if receivingPass {
				if player.Player == e.ball.IntendedReceiver {
					// Meet the ball on its path instead of chasing where it is: the
					// closest point of the remaining flight line ahead of the ball.
					player.Intent = PlayerIntent_RECEIVE_PASS
					ballSpeed := length(e.ball.Velocity)
					lookahead := toPitch(Vector2F{
						e.ball.Velocity.X * Player_PASS_RECEIVER_LOOKAHEAD_SECONDS,
						e.ball.Velocity.Y * Player_PASS_RECEIVER_LOOKAHEAD_SECONDS})
					meet := Vector2F{e.ball.Position.X + lookahead.X,
						e.ball.Position.Y + lookahead.Y}
					if heightMetres := e.ball.Z * Units_BALL_Z_METRES; heightMetres > Aerial_CONTROL_CEILING_METRES ||
						e.ball.VelocityZ > 0.0 {
						// A lofted ball is met where it drops to a controllable height,
						// not intercepted on the way up.
						const G = Ball_GRAVITY
						drop := math.Max(0.0, heightMetres-
							Passing_LOFTED_ARRIVAL_HEIGHT_METRES)
						fallSeconds := (e.ball.VelocityZ +
							math.Sqrt(e.ball.VelocityZ*e.ball.VelocityZ+2.0*G*drop)) /
							G
						landing := toPitch(Vector2F{
							e.ball.Velocity.X * fallSeconds * AIR_TRAVEL_SHARE,
							e.ball.Velocity.Y * fallSeconds * AIR_TRAVEL_SHARE})
						meet = Vector2F{e.ball.Position.X + landing.X, e.ball.Position.Y + landing.Y}
					} else if ballSpeed > EPSILON {
						heading := Vector2F{e.ball.Velocity.X / ballSpeed,
							e.ball.Velocity.Y / ballSpeed}
						offset := toMetres(Vector2F{player.Position.X - e.ball.Position.X,
							player.Position.Y - e.ball.Position.Y})
						along := offset.X*heading.X + offset.Y*heading.Y
						if along > 0.0 {
							ahead := toPitch(Vector2F{heading.X * along, heading.Y * along})
							meet = Vector2F{e.ball.Position.X + ahead.X, e.ball.Position.Y + ahead.Y}
						}
					}
					target = Vector2F{hClamp(meet.X, Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
						hClamp(meet.Y, Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
				} else {
					player.Intent = PlayerIntent_OFFER_SUPPORT
					// Preserve the lane established before release instead of snapping
					// every supporting player back toward the formation anchor for the
					// duration of each pass; drift gently forward and toward the ball
					// (rates per second, so a long pass does not accumulate a run).
					target = player.MovementTarget
					target.X += attackDirection *
						Shape_IN_FLIGHT_SUPPORT_ADVANCE_PER_SECOND *
						dt
					target.Y += (e.ball.Position.Y - target.Y) *
						Shape_IN_FLIGHT_SUPPORT_BALL_PULL_PER_SECOND * dt
				}
			} else if pressers[0] == player ||
				(pressers[1] == player &&
					sliders.Pressing > Shape_SECOND_LOOSE_BALL_PRESS_THRESHOLD) {
				// For a genuinely loose ball, only the nearest players contest it.
				// Everybody else either joins a selected transition run or recovers the
				// team shape. A normal pass is handled above so it does not trigger a
				// full-team scramble on every release of the ball.
				if pressers[0] == player {
					player.Intent = PlayerIntent_CLAIM_LOOSE_BALL
				} else {
					player.Intent = PlayerIntent_COVER_PRESS
				}
				player.IsPressing = true
				// Run at where the ball will be when he gets there.
				lookaheadSeconds := math.Min(distance(player.Position, pressurePosition)/
					math.Max(e.currentTopSpeed(player), EPSILON),
					Player_MAX_LOOSE_BALL_LOOKAHEAD_SECONDS)
				lead := toPitch(Vector2F{e.ball.Velocity.X * lookaheadSeconds,
					e.ball.Velocity.Y * lookaheadSeconds})
				target = Vector2F{hClamp(pressurePosition.X+lead.X,
					Pitch_PLAYER_MIN_X, Pitch_PLAYER_MAX_X),
					hClamp(pressurePosition.Y+lead.Y,
						Pitch_PLAYER_MIN_Y, Pitch_PLAYER_MAX_Y)}
				if pressers[1] == player {
					target.X -= attackDirection * Player_COVER_LOOSE_BALL_OFFSET
				}
			} else if e.ball.IsPass || e.ball.IsShot {
				var tight *MatchPlayer
				if e.ball.IsPass && e.ball.PassByHome != player.IsHomeTeam {
					tight = e.tightMarkTarget(player)
				}
				if tight != nil {
					// A tight marker goes with his man while the pass is on its way.
					player.Intent = PlayerIntent_MARK_OPPONENT
					target = e.tightMarkPoint(player, tight, target)
				} else if teamPhase == TeamPhase_DEFENSIVE_TRANSITION {
					player.Intent = PlayerIntent_RECOVER_SHAPE
					target.X -= attackDirection * Shape_DEFENSIVE_TRANSITION_RECOVERY
					target.Y += (pressurePosition.Y - target.Y) *
						Shape_DEFENSIVE_TRANSITION_BALL_COMPACTNESS
				} else {
					player.Intent = PlayerIntent_HOLD_SHAPE
				}
			} else {
				attackingTransition := teamPhase == TeamPhase_ATTACKING_TRANSITION
				if attackingTransition {
					runners := awayRunners
					if player.IsHomeTeam {
						runners = homeRunners
					}
					transitionRunner := b2ContainsRunner(runners, player)
					if transitionRunner {
						player.Intent = PlayerIntent_RUN_IN_BEHIND
					} else {
						player.Intent = PlayerIntent_OFFER_SUPPORT
					}
					player.IsMakingRun = transitionRunner
					target.X += attackDirection * b2Sign(transitionRunner,
						Shape_ATTACKING_TRANSITION_RUN_ADVANCE,
						Shape_ATTACKING_TRANSITION_SUPPORT_ADVANCE)
					target.Y += (pressurePosition.Y - target.Y) *
						Shape_ATTACKING_TRANSITION_BALL_PULL
					if transitionRunner &&
						e.isOffside(player, player.IsHomeTeam, lineFor(player.IsHomeTeam), 0.0) {
						target.X -= attackDirection * Shape_ONSIDE_RECOVERY
					}
				} else {
					player.Intent = PlayerIntent_RECOVER_SHAPE
					target.X -= attackDirection * Shape_DEFENSIVE_TRANSITION_RECOVERY
				}
			}
		}

		// A lofted delivery into the box is attacked by the receiver while the
		// defenders hold their marks instead of drifting back into shape.
		if e.ball.IsAerialDelivery && carrier == nil &&
			e.ball.PassByHome != player.IsHomeTeam &&
			player.Intent != PlayerIntent_CLAIM_LOOSE_BALL {
			target = player.MovementTarget
		}

		if goalkeeper {
			receivingBackPass := e.ball.IsPass && e.ball.IntendedReceiver == player.Player &&
				player.Intent == PlayerIntent_RECEIVE_PASS
			player.Intent = PlayerIntent_GOALKEEP
			if e.keepers[hBoolIndex(player.IsHomeTeam)].State == GoalkeeperState_DIVE {
				continue
			}
			// A keeper receiving a team-mate's pass steps onto the ball's path.
			if !receivingBackPass {
				target = e.goalkeeperTarget(player, carrier)
			}
		}

		// Keepers shuffle constantly to stay on the ball-goal line.
		urgentTarget := player.Intent == PlayerIntent_PRESS_BALL ||
			player.Intent == PlayerIntent_CLAIM_LOOSE_BALL ||
			player.Intent == PlayerIntent_GOALKEEP
		if player.Intent == PlayerIntent_PRESS_BALL &&
			previousIntent != PlayerIntent_PRESS_BALL {
			e.statsOf(player).Pressures++
		}
		player.TacticalTarget = target
		player.TargetOwner = owner
		player.UrgentMovement = urgentTarget
	}
	if e.cfg.Modules.M1Defence {
		e.feM1Assign()
	}
}

func (e *MatchEngine) cutsInside(player *MatchPlayer) bool {
	role := player.Player.GetRole()
	wideForward := role == PlayerRole_LM || role == PlayerRole_RM ||
		role == PlayerRole_LW || role == PlayerRole_RW
	depth := player.Position.X
	if !player.IsHomeTeam {
		depth = 1.0 - player.Position.X
	}
	// An inside forward always looks to come inside, a touchline winger
	// never does, whatever his natural position.
	roleCut := e.roleProfileOf(player).CutInside
	if (!wideForward && roleCut == 0.0) ||
		depth < Rules_HOME_FINAL_THIRD_START {
		return false
	}
	// The choice holds for a few seconds at a time.
	share := hClamp(
		hClamp(Shape_CUT_INSIDE_BASE+(player.Shooting-player.Passing)*
			Shape_CUT_INSIDE_PREFERENCE,
			Shape_MIN_CUT_INSIDE, Shape_MAX_CUT_INSIDE)+
			roleCut*TacticsTuning_CUT_INSIDE_SHIFT,
		0.0, 1.0)
	return (e.epochNoise(player.Player.GetId(), 0xc0715e,
		RUN_TIMING_EPOCH_STEPS)+
		1.0)*
		0.5 <
		share
}
