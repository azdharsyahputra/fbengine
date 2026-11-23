# Porting notes: Player12 match engine -> Go (`fmengine`)

Source: Player12 (<https://github.com/FlavioMili/Player12>, GPL-3.0), C++ under
`src/`. This stage is the **foundation**: every type, constant and helper the
match engine needs, so that the remaining `match_engine.cpp` functions can be
ported in parallel (stages B1-B6) into the same package.

Build: `go build ./... && go vet ./...` (Go 1.26, no cgo, no dependencies).
Tests: `go test ./...` (currently only the RNG port).

## Module layout

```
go.mod                 module fmengine, go 1.26
LICENSE                verbatim copy of Player12's GPL-3.0 license
NOTICE.md              attribution and license statement
METHODS.md             every MatchEngine method: C++ decl -> Go signature, owner, status
PORTING.md             this file
engine/                package engine (one package, several files)
  types.go             global/types.h, global/stats_config.h, bits of global/global.h
  player.go            model/player.h/.cpp (engine-relevant part), InjuryType from model/injury.h
  role_utils.go        model/role_utils.h/.cpp (no localised names)
  tactics.go           model/tactics.h/.cpp (all), TacticsTuning
  strategy.go          model/strategy.h/.cpp (all)
  lineup.go            model/lineup.h/.cpp (all but swapPlayers/bringIn/toString/generateStartingXI), SetPieces
  match_rules.go       model/match_rules.h/.cpp (all)
  match_events.go      model/match_events.h, matchEventTypeName
  match_context.go     model/match_context.h
  match_scenario.go    model/match_scenario.h
  match_tracking.go    model/match_tracking.h (types only)
  medical.go           model/medical_centre.h/.cpp: MedicalFlag, MINUTE_LIMIT, substitutionDue only
  tuning.go            model/match_tuning.h: ALL 836 values (generated, see "Tuning convention")
  match_engine.go      model/match_engine.h: all enums, structs and the MatchEngine fields
  engine_inline.go     accessors defined inline in match_engine.h
  helpers.go           match_engine.cpp anonymous-namespace helpers (44-515), noise (1337-1375),
                       helper methods (8694-8997), hInitDefaults and Go-only h* helpers
  rng.go               hand-written std::mt19937
  support.go           Optional[T] (std::optional), Pair[A,B] (std::pair)
  controlled.go        play-mode seam: faithful "nobody controlled" behaviour (see below)
  skipped.go           no-op hooks: recordHighlight, captureInterpolationFrame,
                       describePendingEvents, track* (match_engine_tracking.cpp)
  stubs_b1.go          TEMPORARY panicking stubs: teamEdge, effectiveFamiliarity  -> B1 deletes the file
  stubs_b2.go          TEMPORARY panicking stub: weakFootPressure                 -> B2 deletes the file
  rng_test.go          checks the mt19937 port against the C++ standard value
```

Every Go file starts with `// Ported from Player12 <file(s)> (GPL-3.0)`. Later
stages: put each stage in its own file(s), e.g. `engine_b3_movement.go`, with
the same first line naming the C++ lines ported.

## Naming convention

The goal is a mechanical C++ -> Go translation.

| C++ | Go | Example |
|---|---|---|
| `MatchEngine` member function | method on `*MatchEngine`, **same lowerCamel name** | `refreshTacticalTargets(dt)` -> `func (e *MatchEngine) refreshTacticalTargets(dt float64)` |
| `MatchEngine` data member | unexported field, **same name** | `stepCounter`, `homePhase`, `players` |
| any other struct/class field | exported: first letter upper-cased, snake_case -> UpperCamel | `MatchPlayer::isHomeTeam` -> `IsHomeTeam`; `MatchEvent::type` -> `Type`; `PlayerTraits::injury_proneness` -> `InjuryProneness` |
| instance method of a model class (Player, Lineup, Strategy, MatchTracker) | exported, first letter upper-cased | `player->getId()` -> `player.GetId()`; `lineup.getOutfieldPlayers()` -> `lineup.GetOutfieldPlayers()` |
| namespace function / static member function | `Namespace_name` (`::` -> `_`, name kept) | `MatchRules::tackleWinChance` -> `MatchRules_tackleWinChance`; `Tactics::profile` -> `Tactics_profile`; `Lineup::roleAt` -> `Lineup_roleAt`; `RoleUtils::getBroadCategory` -> `RoleUtils_getBroadCategory` |
| enum class enumerator | `Enum_ENUMERATOR` | `MatchState::KICK_OFF` -> `MatchState_KICK_OFF`; `TacticalRole::Poacher` -> `TacticalRole_Poacher`; `Foot::Left` -> `Foot_Left` |
| type nested in a namespace/class | plain type name | `MatchRules::FoulSanction` -> `FoulSanction` (`FoulSanction_RED`), `MatchRules::Knockout` -> `Knockout`, `Lineup::PositionedPlayer` -> `PositionedPlayer`, `MatchEngine::PassOption` -> `PassOption`, `MatchEngine::TacticalArrival` -> `TacticalArrival` |
| namespace / static constant | `Namespace_NAME` | `TacticsTuning::TIGHT_MARK_WEIGHT` -> `TacticsTuning_TIGHT_MARK_WEIGHT`; `Lineup::MAX_SUBSTITUTES` -> `Lineup_MAX_SUBSTITUTES`; `MedicalCentre::MINUTE_LIMIT` -> `MedicalCentre_MINUTE_LIMIT` |
| public free function | first letter upper-cased | `goalkeeperStateName` -> `GoalkeeperStateName`, `matchEventTypeName` -> `MatchEventTypeName`, `toRowCol` -> `ToRowCol` |
| `match_engine.cpp` anonymous-namespace function/constant | **same name**, package-level | `distance`, `toMetres`, `stepFlight`, `EPSILON`, `AIR_TRAVEL_SHARE`, `RUN_TIMING_EPOCH_STEPS` |
| anonymous-namespace item of another .cpp | prefixed with the file stem | tactics.cpp `stat` -> `tacticsStat`, `DEFAULT_STAT` -> `tactics_DEFAULT_STAT`; lineup.cpp `stat` -> `lineupStat`; strategy.cpp `sanitized` -> `strategySanitized` |
| constructor, incl. the implicit default constructor of a struct with default member initializers | `New<Type>(...)` | `MatchPlayer{}` -> `NewMatchPlayer()`, `MatchEvent{}` -> `NewMatchEvent()`, `Strategy()` -> `NewStrategy()`, `Lineup()` -> `NewLineup()` |
| a Go-only free helper that is not in the C++ | prefixed `h` | `hClamp`, `hClampInt`, `hLerp`, `hAbsInt`, `hIsFinite`, `hBoolIndex`, `hSome`, method `hInitDefaults` |
| C++ identifier that is a Go keyword | parameter `type` -> `t` | `logEvent(MatchEventType type)` -> `logEvent(t MatchEventType)` |

Overloads: `simulateToEnd(MatchFidelity)` -> `simulateToEndFidelity`,
`logEvent(type, actor)` -> `logEventActor`; constructors -> `NewMatchEngine` /
`NewMatchEngineWithSeed` (B1). Static `groundLaunchSpeed` / `loftVerticalSpeed`
become package-level funcs (B4). C++ default arguments become explicit
arguments (callers pass the default). Full list: METHODS.md.

Defaults: Go zero values are not C++ default member initializers. Build structs
with `New<Type>()` whenever the C++ writes `T{}`/`T x;` for a type that has
non-zero defaults (MatchPlayer, MatchBall, MatchEvent, PlayerMatchStats,
MatchStats, PassDecision, ScenarioDecision, PassOption, MatchCommandRecord,
MatchPlayerInput, MatchInputRecord, MatchContext, Knockout, FoulContext,
TackleContext, Shootout, PendingAdvantage, GoalkeeperControl, ShoutState,
StrategySliders, RoleWeights, SlotInstruction, MatchSubstitution,
MatchScenario(Player), TrackedPlayer, MatchTracker, PlayerTraits,
PlayerDynamics). The MatchEngine's own member initializers are applied by
`e.hInitDefaults()`, which the B1 constructor must call before anything else.
Note `markAssignments{}` is zero-initialised in C++ (not -1), as in Go.

## Types and signature rules

- **Floating point: `float64` everywhere** (C++ `float` and `double`). Formulas
  and constants are unchanged; results are therefore not bit-identical to the
  C++ build, only statistically equivalent. `std::numeric_limits<float>::max()`
  -> `math.MaxFloat64`, `infinity()` -> `math.Inf(1)`, `std::numbers::pi_v<float>`
  -> `math.Pi`, `std::lround` -> `math.Round`, `std::clamp` -> `hClamp`,
  `std::lerp` -> `hLerp`, `std::min/max` -> builtin `min/max` (or `math.Min/Max`).
- Integers: `int` -> `int`; `std::size_t` -> `int`; `std::uint32_t` ->
  `uint32`; `std::uint64_t` -> `uint64`; `std::uint8_t` -> `uint8`;
  `std::int8_t` -> `int8`; `PlayerID` = `uint32`, `TeamID` = `uint16`.
- `Vector2F` is `struct{ X, Y float64 }`.
- `const Player*` / `Player*` -> `*Player` (persistent, borrowed, compared by
  pointer identity exactly as in C++). `MatchPlayer*` and `MatchPlayer&` /
  `const MatchPlayer&` -> `*MatchPlayer`, always pointing into `e.players`
  (`&e.players[i]`; iterate with `for i := range e.players { p := &e.players[i] ... }`,
  never `for _, p := range` when mutating or taking addresses). Do not append to
  `e.players` while holding such pointers (the C++ reserves 22 and substitutes in
  place).
- References to the large structs (`Lineup`, `Strategy`, `StatsConfig`,
  `PlayerMatchStats`, `MatchStats`, `MatchEvent`, `PassOption`, `PassDecision`,
  `ScenarioDecision`, `RoleProfile`, `MatchScenario`, `MatchCommandRecord`,
  `MatchBall`, `MatchTracker`) -> pointers; returned `T&` -> `*T`. Small value
  structs (`Vector2F`, `StrategySliders`, `MatchContext`, `Knockout`,
  `FoulContext`, `TackleContext`, `StoppageLog`, `MatchPlayerInput`) -> by value.
- Non-const `float&` out-parameters -> `*float64`.
- `std::vector<T>` / `std::span<const T>` -> `[]T`; `std::array<T, N>` -> `[N]T`
  (array sizes use the typed tuning consts, e.g. `[Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer`).
- `std::optional<T>` -> `Optional[T]` (value type: `.HasValue`, `.Value`,
  `hSome(v)`, `Optional[T]{}` = nullopt, `.ValueOr(x)`); `std::pair<A,B>` -> `Pair[A,B]`.
- `std::string` / `std::string_view` -> `string`; attribute names are looked up
  in `Player.Stats` (`map[string]float64`).
- `Strategy` is copied by value in C++; in Go use `s.Clone()` for an
  independent copy (`setStrategy`, `MatchCommandRecord.Strategy`, the
  constructor's `homeStrategy`/`awayStrategy`).
- `side index`: C++ `homeTeam ? 0 : 1` may be written `hBoolIndex(homeTeam)`.

## RNG and determinism

`std::mt19937` is ported by hand (`rng.go`); its raw output stream equals the
C++ one for the same seed (checked by `rng_test.go`). `uniform01`, `gaussian`,
`randomFloat`, `incidentRoll`, `splitMix64`, `counterNoise`, `hashNoise` and
`epochNoise` are faithful. `rng` is seeded with `seed`, `incidentRng` with
`uint32(splitMix64(uint64(seed)))` (B1 constructor). `mt19937` is a value type,
so copying a `MatchEngine` copies its random state. Same seed -> same match in
Go; Go and C++ matches differ (float64 vs float), as allowed for this beta.
Never use `math/rand` or map iteration order in simulation code.

## Tuning convention

`match_tuning.h` is a struct of nested structs. In Go every value is a
package-level **typed constant `<Group>_<CPP_NAME>`** (the outer `MatchTuning::`
is dropped, `::` becomes `_`, spelling kept verbatim):
`MatchTuning::Shooting::ERROR_BASE_METRES` -> `Shooting_ERROR_BASE_METRES`, and
the common `using S = MatchTuning::Shooting; S::X` -> `Shooting_X`. Types:
float/double -> `float64`, int/long/size_t -> `int`, uint32 -> `uint32`, uint64
-> `uint64`. The two `std::array<float,3>` values are `var`s
(`Control_ASSIST_MIN_ALIGNMENT`, `Control_ASSIST_COMPLETION_WEIGHT`). All 27
groups / 836 values are present; tuning.go was generated from the header and
keeps its comments. Other tuning namespaces use the same rule
(`TacticsTuning_*`, `MatchTracking_*`).

## Ported from files outside the requested list (minimum only)

- `global/global.h`: `MIN_STAT_VAL`, `MAX_STAT_VAL`, `PLAYER_STAT_INCREASE_BASE`,
  `LINEUP_GRID_*`, `ToRowCol` (types.go; Strategy's legacy grid weights).
- `model/injury.h`: the `InjuryType` enum (PlayerDynamics).
- `model/medical_centre.h/.cpp`: `MedicalFlag` bits, `MedicalCentre_MINUTE_LIMIT`,
  `MedicalCentre_substitutionDue` (used by `runMedicalSubstitutions`, B6).
- `model/match_rules.cpp`: `matchEventTypeName` lives there (match_events.go).

Not ported: `Language`/nationality, wages, contracts, market value, ageing and
training (`Player`), localised names (`RoleUtils::shortName/longName`),
`MatchCommentary` (event descriptions stay empty), `GameData`.

## Foundation-provided seams (do not re-implement)

- `controlled.go`: play mode is out of scope. `isControlled`,
  `getControlledPlayer`, `applyDueInputs` and the play-mode inline accessors are
  faithful; `setControlledPlayer` always returns false, so nobody is ever
  controlled and `integrateControlled`, `performControlledAction`,
  `controlledAim`, `controlledPassReceiver`, `playControlledPass`,
  `shapeControlledShot`, `controlledAssistTarget`, `switchSeconds` just panic if
  reached. Call sites in other stages keep the C++ `if (isControlled(...))`
  branches unchanged.
- `skipped.go`: `recordHighlight`, `captureInterpolationFrame`,
  `describePendingEvents` and the five `track*` methods are no-ops (output-only
  in C++). `logEvent` (B6) still calls `recordHighlight` as in C++.
- `stubs_b1.go` / `stubs_b2.go`: TEMPORARY panicking stubs for methods the
  foundation calls (`teamEdge`, `effectiveFamiliarity`; `weakFootPressure`).
  B1/B2 delete the whole file when adding the real methods.

## Owned by later stages

Line numbers are `src/model/match_engine.cpp`. Each stage implements the
listed MatchEngine methods (signatures in METHODS.md) plus any
anonymous-namespace helpers inside its ranges (same C++ names).

### B1 - core / step / clock / phases (516-2397, minus helpers 1337-1374; plus 3940)
Constructors `NewMatchEngine`/`NewMatchEngineWithSeed` (516, 524),
drawRefereeStrictness (565), setMatchContext (573), initializePlayers (603),
loadAttributes (641), addPlayerStats (679), statsOf (694), findPlayerStats (699),
getPlayerCondition (707), setPlayerCondition (719), setTacticalFamiliarity (748),
refreshTargetBlends (757), setStepTicks (774), stepSeconds (782),
periodElapsed (788), familiarityOf (794), effectiveFamiliarity (799),
setStrategy (809), applyShout (818), startShout (827), getShoutStrength (838),
getActiveShout (847), getFormation (854), setFormation (859),
getFormationSlot (868), movePlayerToSlot (880), loadCommandReplay (892),
recordCommand (902), applyDueCommands (914), executeCommand (921),
placeFormation (1064), getEffectiveSliders (1091), refreshEffectiveSliders (1096),
computeUnderdogShares (1102), computeEffectiveSliders (1131), shoutShotBias (1236),
shoutWorkRate (1248), setTeamTalkModifier (1258), getTeamTalkModifier (1270),
talkOf (1276), teamEdge (1294), runAiTouchline (1308), refreshRatings (1322),
update (1375), advance (1403), simulateToEnd (1422), simulateToEndFidelity (1424),
backgroundStepTicks (1442), getSimulatedSeconds (1464), advanceClock (1620),
simulateStep (1630), simulateStepBody (1636), isInAddedTime (1797),
updateMatchClock (1805), announceAddedTime (1835), endPeriod (1845),
beginBreak (1885), tieLevel (1901), setKnockout (1907), getTieWinnerHome (1913),
maxSubstitutions (1924), startShootout (1930), beginShootoutKick (2001),
updateShootout (2044), finishShootoutKick (2113), finishMatch (2146),
setTransitionPhases (2159), setControlledPhases (2166), updateTeamPhases (2177),
updateMovement (2231), estimatedArrivalTime (2282), closestOutfieldPair (2312),
runPriority (2354), stepsWithin (3940). Delete `stubs_b1.go`.
Skipped inside this range: playback/highlights 1469-1600
(setPlaybackMode, setPlaybackSpeed, setHighlightPlaybackSpeed, advancePlayback,
isInHighlight, predictNextHighlight), captureInterpolationFrame (1601, no-op
provided), getInterpolationAlpha (1614).

### B2 - tactical targets, runner/supporter selection, assignMarks, tactics resolution (2398-3364, 3971-4342)
selectAttackingRunners (2398), runChannel (2456), selectFinalThirdPlayer (2474),
selectActiveSupporters (2535), findCoverOutlet (2583), refreshTacticalTargets
(2615), cutsInside (3337), anonymous `NO_ROLE` (3973), resolveTactics (3976),
getSlotRole (4006), roleProfileOf (4012), possessionAnchor (4023),
defensiveRoleShift (4053), instructionAgainst (4061), tightMarkTarget (4072),
tightMarkPoint (4085), tightMarkPenalty (4099), passingRoleBias (4118),
pressEagerness (4131), engageBoost (4141), pressStandOffShare (4154),
weakFootShade (4164), weakFootPressure (4178), doubleUpPoint (4189),
doubleUpPenalty (4213), talkSwing (4230), getTeamTalkEffect (4236),
assignMarks (4241). Delete `stubs_b2.go`.

### B3 - movement, restart movement, separation, load, possession/dribble/take-on/tackle/fouls/sanctions (3941-3970, 4343-5442)
currentTopSpeed (3952), integrateMovements (4343), stepKinematics (4570),
updateRestartSupport (4657), updateRestartMovement (4700), separatePlayers (4728),
accumulatePlayerLoad (4820), resolvePossessionAndActions (4951),
dribbleExposureShare (5044), updateDribble (5053), attemptTakeOn (5134),
attemptTackle (5177), deniesGoalChance (5277), commitFoul (5303),
applySanction (5383), updatePendingAdvantage (5427).

### B4 - decideAction, pass evaluation/choice, passBall, strikeShot, takeShot, launch, loft (5443-6469)
decideAction (5443), evaluatePassOption (5678), choosePassTarget (5866),
passBall (5946), strikeShot (6107), takeShot (6244), launchBall (6298),
groundLaunchSpeed (6316, package func), anonymous `flightHeightAtDistance`
(6363), loftVerticalSpeed (6391, package func).

### B5 - keeper dive/saves, updateBall/integrateBall, loose ball, aerial contest, clears/headers, setPossession, out of bounds, goals (6470-7944)
planGoalkeeperDive (6470), diveGoalkeeper (6545), goalkeeperTarget (6604),
updateBall (6762), integrateBall (6811), attemptSave (6843),
resolveShotAtGoalkeeper (6884), parryShot (6910), recordPassCompletion (6954),
resolveLooseBall (6977), resolveAerialContest (7254), tryOwnGoalTouch (7411),
clearBehind (7449), clearBall (7469), headBall (7498), setPossession (7632),
clearFlightState (7674), checkOutOfBounds (7692), scoreGoal (7778),
updateBallInNet (7874), makeSave (7894), extendRestart (7923),
beginStoppage (7933).

### B6 - restarts/set pieces, substitutions, injuries, events/logging (7945-9701, minus helpers 8694-8997)
setupKickOff (7945), kickOffPosition (7985), setupThrowIn (7997),
setupGoalKick (8035), setupCorner (8061), setupFreeKick (8098),
setupPenalty (8140), placeTaker (8195), restartTaker (8228),
setRestartTaker (8235), slotOf (8243), bestSetPieceTaker (8248),
dutyTaker (8270), anonymous `setPieceBoxThreat` (8300), arrangeSetPiece (8328),
userRestartPassRequested (8419), completeRestart (8445), takeSetPiece (8508),
takeCorner (8574), takeDirectFreeKick (8639), getSubstitutionsUsed (8999),
getSubstitutionWindowsUsed (9004), canSubstitute (9009),
setAutoSubstitutions (9028), substitutePlayer (9036), performSubstitution (9049),
chooseReplacement (9124), runAiSubstitutions (9151), setMedicalFlags (9167),
runMedicalSubstitutions (9178), runAiSubstitutionsFor (9205),
checkInjuries (9336), injurePlayer (9355), removeFromPitch (9369),
rebalanceShape (9423), ensureGoalkeeper (9466), applyScenario (9514),
logEvent (9604), setTeamNames (9624), squadPlayerName (9629),
logEventActor (9692). Skipped inside this range: describePendingEvents (9636,
no-op provided), recordHighlight (9648, no-op provided).

### Skipped entirely
Controlled-player input 3365-3939 (seam in controlled.go; setPlayHalfMinutes,
submitInput, loadInputReplay, suggestActivePlayer, nextSwitchCandidate not
provided), playback/highlight/interpolation (above), debug JSON 9702-9904
(getDebugSnapshotJson, writeDebugSnapshot, jsonFloat, jsonEscape), commentary
text (MatchCommentary), match_tracking.cpp / match_engine_tracking.cpp logic,
GUI.

## Exported API

The engine's public C++ API keeps lowerCamel (unexported) Go names by design,
so it is usable inside package `engine` only. Exported wrappers (e.g.
`func (e *MatchEngine) SimulateToEnd()`) or a facade package can be added once
B1-B6 are in.
