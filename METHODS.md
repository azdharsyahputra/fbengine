# MatchEngine methods: C++ -> Go

Every member function declared in `src/model/match_engine.h`, with its planned Go
signature, its C++ definition line and the stage that owns it. Generated from the
header; Go signatures follow the rules in PORTING.md ("Signature rules").

Conventions (summary):

- Go method on `*MatchEngine` with the SAME lowerCamel name as C++, e.g.
  `func (e *MatchEngine) refreshTacticalTargets(dt float64)`.
- Overloads: `simulateToEnd()` -> `simulateToEnd()`, `simulateToEnd(MatchFidelity)` ->
  `simulateToEndFidelity(f)`; `logEvent(type)` -> `logEvent(t)`, `logEvent(type, actor)` ->
  `logEventActor(t, actor)`. Constructors: `NewMatchEngine(home, away *Lineup, homeStrat,
  awayStrat *Strategy, config *StatsConfig) *MatchEngine` and `NewMatchEngineWithSeed(...,
  seed uint32) *MatchEngine` (stage B1; both call `e.hInitDefaults()` first). `NewMatchEngine`
  uses `makeNonBlockingMatchSeed()`.
- C++ default arguments do not exist in Go: callers pass every argument (defaults listed).
- `static` members (`groundLaunchSpeed`, `loftVerticalSpeed`) become package-level funcs.
- A parameter named `type` is renamed `t` (Go keyword).
- Status `present` = a Go definition exists now (real, inline, no-op, or TEMPORARY stub).
  TEMPORARY stubs: `teamEdge`, `effectiveFamiliarity` (stubs_b1.go, B1 deletes the file) and
  `weakFootPressure` (stubs_b2.go, B2 deletes the file).

## Foundation: implemented in helpers.go (19)

Ported faithfully in this stage.

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `float hashNoise(std::uint32_t salt, std::uint32_t key) const` | `(e) hashNoise(salt uint32, key uint32) float64` | match_engine.cpp:1352 |  | present |
| `float epochNoise(std::uint32_t salt, std::uint32_t key, std::uint32_t epochSteps) const` | `(e) epochNoise(salt uint32, key uint32, epochSteps uint32) float64` | match_engine.cpp:1357 |  | present |
| `float executionErrorScale(const MatchPlayer& player) const` | `(e) executionErrorScale(player *MatchPlayer) float64` | match_engine.cpp:1364 |  | present |
| `MatchPlayer* findClosestPlayer(Vector2F position, bool homeTeam, bool includeGoalkeeper = true)` | `(e) findClosestPlayer(position Vector2F, homeTeam bool, includeGoalkeeper bool) *MatchPlayer` | match_engine.cpp:8709 | C++ default(s): includeGoalkeeper=true (pass explicitly) | present |
| `MatchPlayer* findGoalkeeper(bool homeTeam)` | `(e) findGoalkeeper(homeTeam bool) *MatchPlayer` | match_engine.cpp:8731 |  | present |
| `MatchPlayer* findMatchPlayer(const Player* player)` | `(e) findMatchPlayer(player *Player) *MatchPlayer` | match_engine.cpp:8743 |  | present |
| `float nearestOpponentDistance(const MatchPlayer& player) const` | `(e) nearestOpponentDistance(player *MatchPlayer) float64` | match_engine.cpp:8752 |  | present |
| `float openSpaceAhead(const MatchPlayer& carrier) const` | `(e) openSpaceAhead(carrier *MatchPlayer) float64` | match_engine.cpp:8766 |  | present |
| `float passingLaneRisk(const MatchPlayer& passer, const MatchPlayer& receiver) const` | `(e) passingLaneRisk(passer *MatchPlayer, receiver *MatchPlayer) float64` | match_engine.cpp:8789 |  | present |
| `float estimateShotXG(const MatchPlayer& shooter) const` | `(e) estimateShotXG(shooter *MatchPlayer) float64` | match_engine.cpp:8831 |  | present |
| `bool isOffside(const MatchPlayer& receiver, bool attackingHome, float defenderLine, float lineTolerance = 0.0f) const` | `(e) isOffside(receiver *MatchPlayer, attackingHome bool, defenderLine float64, lineTolerance float64) bool` | match_engine.cpp:8920 | C++ default(s): lineTolerance=0.0f (pass explicitly) | present |
| `float offsideLine(bool attackingHome) const` | `(e) offsideLine(attackingHome bool) float64` | match_engine.cpp:8879 |  | present |
| `void resetPositions()` | `(e) resetPositions()` | match_engine.cpp:8694 |  | present |
| `float attribute(const Player* player, std::string_view name) const` | `(e) attribute(player *Player, name string) float64` | match_engine.cpp:8949 |  | present |
| `float rawAttribute(const Player* player, std::string_view name) const` | `(e) rawAttribute(player *Player, name string) float64` | match_engine.cpp:8939 |  | present |
| `void computeLevel(const Lineup& home, const Lineup& away)` | `(e) computeLevel(home *Lineup, away *Lineup)` | match_engine.cpp:8955 |  | present |
| `float randomFloat(float minimum, float maximum)` | `(e) randomFloat(minimum float64, maximum float64) float64` | match_engine.cpp:8983 |  | present |
| `float incidentRoll()` | `(e) incidentRoll() float64` | match_engine.cpp:8988 |  | present |
| `bool isHomePlayer(const Player* player) const` | `(e) isHomePlayer(player *Player) bool` | match_engine.cpp:8990 |  | present |

## Foundation: header inline accessors (engine_inline.go, controlled.go) (43)

Defined inline in match_engine.h, so ported with the header.

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `float getTacticalFamiliarity(bool homeTeam) const` | `(e) getTacticalFamiliarity(homeTeam bool) float64` | match_engine.h (inline) |  | present |
| `int getPlayHalfMinutes() const` | `(e) getPlayHalfMinutes() int` | match_engine.h (inline) |  | present |
| `const std::vector<MatchInputRecord>& getInputLog() const` | `(e) getInputLog() []MatchInputRecord` | match_engine.h (inline) |  | present |
| `bool wasControlled(bool homeTeam) const` | `(e) wasControlled(homeTeam bool) bool` | match_engine.h (inline) |  | present |
| `int getControlledActions(bool homeTeam) const` | `(e) getControlledActions(homeTeam bool) int` | match_engine.h (inline) |  | present |
| `const std::vector<MatchCommandRecord>& getCommandLog() const` | `(e) getCommandLog() []MatchCommandRecord` | match_engine.h (inline) |  | present |
| `std::uint64_t getSimulatedSteps() const` | `(e) getSimulatedSteps() uint64` | match_engine.h (inline) |  | present |
| `const std::vector<MatchPlayer>& getPlayers() const` | `(e) getPlayers() []MatchPlayer` | match_engine.h (inline) |  | present |
| `const MatchBall& getBall() const` | `(e) getBall() *MatchBall` | match_engine.h (inline) |  | present |
| `const std::vector<MatchEvent>& getEvents() const` | `(e) getEvents() []MatchEvent` | match_engine.h (inline) |  | present |
| `MatchState getState() const` | `(e) getState() MatchState` | match_engine.h (inline) |  | present |
| `const MatchStats& getStats() const` | `(e) getStats() *MatchStats` | match_engine.h (inline) |  | present |
| `TeamPhase getHomePhase() const` | `(e) getHomePhase() TeamPhase` | match_engine.h (inline) |  | present |
| `TeamPhase getAwayPhase() const` | `(e) getAwayPhase() TeamPhase` | match_engine.h (inline) |  | present |
| `float getTransitionSecondsRemaining() const` | `(e) getTransitionSecondsRemaining() float64` | match_engine.h (inline) |  | present |
| `const PassDecision& getLastPassDecision() const` | `(e) getLastPassDecision() *PassDecision` | match_engine.h (inline) |  | present |
| `const std::vector<MatchSubstitution>& getSubstitutions() const` | `(e) getSubstitutions() []MatchSubstitution` | match_engine.h (inline) |  | present |
| `const std::vector<PlayerMatchStats>& getPlayerStats() const` | `(e) getPlayerStats() []PlayerMatchStats` | match_engine.h (inline) |  | present |
| `const MatchTracker& getTracker() const` | `(e) getTracker() *MatchTracker` | match_engine.h (inline) |  | present |
| `void setTracking(bool on)` | `(e) setTracking(on bool)` | match_engine.h (inline) |  | present |
| `int getPeriod() const` | `(e) getPeriod() int` | match_engine.h (inline) |  | present |
| `int getAddedMinutes(int half) const` | `(e) getAddedMinutes(half int) int` | match_engine.h (inline) |  | present |
| `const MatchRules::Knockout& getKnockout() const` | `(e) getKnockout() Knockout` | match_engine.h (inline) |  | present |
| `const MatchContext& getMatchContext() const` | `(e) getMatchContext() MatchContext` | match_engine.h (inline) |  | present |
| `bool wentToExtraTime() const` | `(e) wentToExtraTime() bool` | match_engine.h (inline) |  | present |
| `bool hasShootout() const` | `(e) hasShootout() bool` | match_engine.h (inline) |  | present |
| `int getShootoutScore(bool homeTeam) const` | `(e) getShootoutScore(homeTeam bool) int` | match_engine.h (inline) |  | present |
| `int getShootoutKicks(bool homeTeam) const` | `(e) getShootoutKicks(homeTeam bool) int` | match_engine.h (inline) |  | present |
| `bool isAbandoned() const` | `(e) isAbandoned() bool` | match_engine.h (inline) |  | present |
| `float getRefereeStrictness() const` | `(e) getRefereeStrictness() float64` | match_engine.h (inline) |  | present |
| `int getHomeScore() const` | `(e) getHomeScore() int` | match_engine.h (inline) |  | present |
| `int getAwayScore() const` | `(e) getAwayScore() int` | match_engine.h (inline) |  | present |
| `float getMatchTimeMinutes() const` | `(e) getMatchTimeMinutes() float64` | match_engine.h (inline) |  | present |
| `float getElapsedMatchMinutes() const` | `(e) getElapsedMatchMinutes() float64` | match_engine.h (inline) |  | present |
| `int getLastUpdateStepCount() const` | `(e) getLastUpdateStepCount() int` | match_engine.h (inline) |  | present |
| `std::uint64_t getDroppedSimulationSteps() const` | `(e) getDroppedSimulationSteps() uint64` | match_engine.h (inline) |  | present |
| `bool getGoalScoredByHome() const` | `(e) getGoalScoredByHome() bool` | match_engine.h (inline) |  | present |
| `float getGoalCelebrationRemaining() const` | `(e) getGoalCelebrationRemaining() float64` | match_engine.h (inline) |  | present |
| `float getGoalCelebrationDuration() const` | `(e) getGoalCelebrationDuration() float64` | match_engine.h (inline) |  | present |
| `GoalkeeperState getHomeGoalkeeperState() const` | `(e) getHomeGoalkeeperState() GoalkeeperState` | match_engine.h (inline) |  | present |
| `GoalkeeperState getAwayGoalkeeperState() const` | `(e) getAwayGoalkeeperState() GoalkeeperState` | match_engine.h (inline) |  | present |
| `const ScenarioDecision& getLastScenarioDecision() const` | `(e) getLastScenarioDecision() *ScenarioDecision` | match_engine.h (inline) |  | present |
| `float matchClockRate() const` | `(e) matchClockRate() float64` | match_engine.h (inline) |  | present |

## Foundation: out-of-scope hooks provided as no-ops (skipped.go) (8)

Output-only in the C++ (highlights, interpolation, commentary, tracker feeds). Do NOT re-implement.

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `void captureInterpolationFrame()` | `(e) captureInterpolationFrame()` | match_engine.cpp:1601 |  | present |
| `void trackTouch(const MatchPlayer& player)` | `(e) trackTouch(player *MatchPlayer)` | match_engine_tracking.cpp:27 |  | present |
| `void trackPassRelease(const MatchPlayer& passer)` | `(e) trackPassRelease(passer *MatchPlayer)` | match_engine_tracking.cpp:34 |  | present |
| `void trackPassCompletion(const MatchPlayer& passer, const MatchPlayer& receiver)` | `(e) trackPassCompletion(passer *MatchPlayer, receiver *MatchPlayer)` | match_engine_tracking.cpp:40 |  | present |
| `void trackShot(const MatchPlayer& shooter, float xg, bool setPiece, bool header, bool penalty)` | `(e) trackShot(shooter *MatchPlayer, xg float64, setPiece bool, header bool, penalty bool)` | match_engine_tracking.cpp:48 |  | present |
| `void trackPressures()` | `(e) trackPressures()` | match_engine_tracking.cpp:59 |  | present |
| `void recordHighlight(MatchEventType type)` | `(e) recordHighlight(t MatchEventType)` | match_engine.cpp:9648 |  | present |
| `void describePendingEvents()` | `(e) describePendingEvents()` | match_engine.cpp:9636 |  | present |

## Foundation: play-mode seam (controlled.go) (12)

Faithful "nobody controlled" behaviour; setControlledPlayer always refuses, the rest panic if ever reached.

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `bool setControlledPlayer(PlayerID playerId)` | `(e) setControlledPlayer(playerId PlayerID) bool` | match_engine.cpp:3384 |  | present |
| `PlayerID getControlledPlayer() const` | `(e) getControlledPlayer() PlayerID` | match_engine.cpp:3402 |  | present |
| `void applyDueInputs()` | `(e) applyDueInputs()` | match_engine.cpp:3436 |  | present |
| `bool isControlled(const MatchPlayer& player) const` | `(e) isControlled(player *MatchPlayer) bool` | match_engine.cpp:3365 |  | present |
| `void integrateControlled(MatchPlayer& player, float dt)` | `(e) integrateControlled(player *MatchPlayer, dt float64)` | match_engine.cpp:3493 |  | present |
| `bool performControlledAction(MatchPlayer& carrier)` | `(e) performControlledAction(carrier *MatchPlayer) bool` | match_engine.cpp:3634 |  | present |
| `Vector2F controlledAim(const MatchPlayer& carrier) const` | `(e) controlledAim(carrier *MatchPlayer) Vector2F` | match_engine.cpp:3621 |  | present |
| `MatchPlayer* controlledPassReceiver(MatchPlayer& carrier, Vector2F aim, bool throughBall)` | `(e) controlledPassReceiver(carrier *MatchPlayer, aim Vector2F, throughBall bool) *MatchPlayer` | match_engine.cpp:3681 |  | present |
| `void playControlledPass(MatchPlayer& carrier, MatchInputAction action)` | `(e) playControlledPass(carrier *MatchPlayer, action MatchInputAction)` | match_engine.cpp:3722 |  | present |
| `void shapeControlledShot(const MatchPlayer& shooter, float halfGoal, float inset, float& aimY, float& aimZ, float& spread) const` | `(e) shapeControlledShot(shooter *MatchPlayer, halfGoal float64, inset float64, aimY *float64, aimZ *float64, spread *float64)` | match_engine.cpp:3804 |  | present |
| `std::optional<Vector2F> controlledAssistTarget( const MatchPlayer& player) const` | `(e) controlledAssistTarget(player *MatchPlayer) Optional[Vector2F]` | match_engine.cpp:3577 |  | present |
| `float switchSeconds(const MatchPlayer& candidate) const` | `(e) switchSeconds(candidate *MatchPlayer) float64` | match_engine.cpp:3824 |  | present |

## Stage B1: core / step / clock / phases (cpp 516-2397, plus stepsWithin 3940) (73)

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `void update(float deltaTime)` | `(e) update(deltaTime float64)` | match_engine.cpp:1375 |  | todo |
| `float advance(float seconds)` | `(e) advance(seconds float64) float64` | match_engine.cpp:1403 |  | todo |
| `void simulateToEnd()` | `(e) simulateToEnd()` | match_engine.cpp:1422 |  | todo |
| `void simulateToEnd(MatchFidelity fidelity)` | `(e) simulateToEndFidelity(fidelity MatchFidelity)` | match_engine.cpp:1424 | overload suffix "Fidelity" | todo |
| `double getSimulatedSeconds() const` | `(e) getSimulatedSeconds() float64` | match_engine.cpp:1464 |  | todo |
| `void setTacticalFamiliarity(bool homeTeam, float familiarity)` | `(e) setTacticalFamiliarity(homeTeam bool, familiarity float64)` | match_engine.cpp:748 |  | todo |
| `void setStrategy(bool homeTeam, const Strategy& strategy)` | `(e) setStrategy(homeTeam bool, strategy *Strategy)` | match_engine.cpp:809 |  | todo |
| `void applyShout(bool homeTeam, MatchShout shout)` | `(e) applyShout(homeTeam bool, shout MatchShout)` | match_engine.cpp:818 |  | todo |
| `float getShoutStrength(bool homeTeam) const` | `(e) getShoutStrength(homeTeam bool) float64` | match_engine.cpp:838 |  | todo |
| `std::optional<MatchShout> getActiveShout(bool homeTeam) const` | `(e) getActiveShout(homeTeam bool) Optional[MatchShout]` | match_engine.cpp:847 |  | todo |
| `std::vector<Vector2F> getFormation(bool homeTeam) const` | `(e) getFormation(homeTeam bool) []Vector2F` | match_engine.cpp:854 |  | todo |
| `bool setFormation(bool homeTeam, std::span<const Vector2F> shape)` | `(e) setFormation(homeTeam bool, shape []Vector2F) bool` | match_engine.cpp:859 |  | todo |
| `std::optional<std::size_t> getFormationSlot(PlayerID playerId) const` | `(e) getFormationSlot(playerId PlayerID) Optional[int]` | match_engine.cpp:868 |  | todo |
| `bool movePlayerToSlot(PlayerID playerId, std::size_t slot)` | `(e) movePlayerToSlot(playerId PlayerID, slot int) bool` | match_engine.cpp:880 |  | todo |
| `StrategySliders getEffectiveSliders(bool homeTeam) const` | `(e) getEffectiveSliders(homeTeam bool) StrategySliders` | match_engine.cpp:1091 |  | todo |
| `void setTeamTalkModifier(bool homeTeam, int half, float modifier)` | `(e) setTeamTalkModifier(homeTeam bool, half int, modifier float64)` | match_engine.cpp:1258 |  | todo |
| `float getTeamTalkModifier(bool homeTeam, int half) const` | `(e) getTeamTalkModifier(homeTeam bool, half int) float64` | match_engine.cpp:1270 |  | todo |
| `bool stepsWithin(float deltaSeconds) const` | `(e) stepsWithin(deltaSeconds float64) bool` | match_engine.cpp:3940 |  | todo |
| `void loadCommandReplay(std::vector<MatchCommandRecord> log)` | `(e) loadCommandReplay(log []MatchCommandRecord)` | match_engine.cpp:892 |  | todo |
| `const PlayerMatchStats* findPlayerStats(PlayerID playerId) const` | `(e) findPlayerStats(playerId PlayerID) *PlayerMatchStats` | match_engine.cpp:699 |  | todo |
| `std::optional<float> getPlayerCondition(PlayerID playerId) const` | `(e) getPlayerCondition(playerId PlayerID) Optional[float64]` | match_engine.cpp:707 |  | todo |
| `bool setPlayerCondition(PlayerID playerId, float condition)` | `(e) setPlayerCondition(playerId PlayerID, condition float64) bool` | match_engine.cpp:719 |  | todo |
| `void setKnockout(const MatchRules::Knockout& rules)` | `(e) setKnockout(rules Knockout)` | match_engine.cpp:1907 |  | todo |
| `void setMatchContext(const MatchContext& context)` | `(e) setMatchContext(context MatchContext)` | match_engine.cpp:573 |  | todo |
| `std::optional<bool> getTieWinnerHome() const` | `(e) getTieWinnerHome() Optional[bool]` | match_engine.cpp:1913 |  | todo |
| `bool isInAddedTime() const` | `(e) isInAddedTime() bool` | match_engine.cpp:1797 |  | todo |
| `void initializePlayers(const Lineup& lineup, bool isHomeTeam)` | `(e) initializePlayers(lineup *Lineup, isHomeTeam bool)` | match_engine.cpp:603 |  | todo |
| `void loadAttributes(MatchPlayer& matchPlayer, const Player* player) const` | `(e) loadAttributes(matchPlayer *MatchPlayer, player *Player)` | match_engine.cpp:641 |  | todo |
| `std::size_t addPlayerStats(const MatchPlayer& matchPlayer, bool started)` | `(e) addPlayerStats(matchPlayer *MatchPlayer, started bool) int` | match_engine.cpp:679 |  | todo |
| `PlayerMatchStats& statsOf(const MatchPlayer& matchPlayer)` | `(e) statsOf(matchPlayer *MatchPlayer) *PlayerMatchStats` | match_engine.cpp:694 |  | todo |
| `void refreshRatings()` | `(e) refreshRatings()` | match_engine.cpp:1322 |  | todo |
| `void advanceClock(float dt)` | `(e) advanceClock(dt float64)` | match_engine.cpp:1620 |  | todo |
| `void updateMatchClock()` | `(e) updateMatchClock()` | match_engine.cpp:1805 |  | todo |
| `void announceAddedTime()` | `(e) announceAddedTime()` | match_engine.cpp:1835 |  | todo |
| `void endPeriod()` | `(e) endPeriod()` | match_engine.cpp:1845 |  | todo |
| `void beginBreak(float seconds, float recovery)` | `(e) beginBreak(seconds float64, recovery float64)` | match_engine.cpp:1885 |  | todo |
| `bool tieLevel() const` | `(e) tieLevel() bool` | match_engine.cpp:1901 |  | todo |
| `void finishMatch()` | `(e) finishMatch()` | match_engine.cpp:2146 |  | todo |
| `void startShootout()` | `(e) startShootout()` | match_engine.cpp:1930 |  | todo |
| `void updateShootout(float dt)` | `(e) updateShootout(dt float64)` | match_engine.cpp:2044 |  | todo |
| `void beginShootoutKick()` | `(e) beginShootoutKick()` | match_engine.cpp:2001 |  | todo |
| `void finishShootoutKick(MatchEventDetail outcome)` | `(e) finishShootoutKick(outcome MatchEventDetail)` | match_engine.cpp:2113 |  | todo |
| `int maxSubstitutions() const` | `(e) maxSubstitutions() int` | match_engine.cpp:1924 |  | todo |
| `float familiarityOf(const MatchPlayer& player) const` | `(e) familiarityOf(player *MatchPlayer) float64` | match_engine.cpp:794 |  | todo |
| `float talkOf(bool homeTeam) const` | `(e) talkOf(homeTeam bool) float64` | match_engine.cpp:1276 |  | todo |
| `StrategySliders computeEffectiveSliders(bool homeTeam) const` | `(e) computeEffectiveSliders(homeTeam bool) StrategySliders` | match_engine.cpp:1131 |  | todo |
| `void computeUnderdogShares()` | `(e) computeUnderdogShares()` | match_engine.cpp:1102 |  | todo |
| `void refreshEffectiveSliders()` | `(e) refreshEffectiveSliders()` | match_engine.cpp:1096 |  | todo |
| `float teamEdge(bool homeTeam) const` | `(e) teamEdge(homeTeam bool) float64` | match_engine.cpp:1294 |  | present |
| `float drawRefereeStrictness() const` | `(e) drawRefereeStrictness() float64` | match_engine.cpp:565 |  | todo |
| `float shoutShotBias(bool homeTeam) const` | `(e) shoutShotBias(homeTeam bool) float64` | match_engine.cpp:1236 |  | todo |
| `float shoutWorkRate(bool homeTeam) const` | `(e) shoutWorkRate(homeTeam bool) float64` | match_engine.cpp:1248 |  | todo |
| `void runAiTouchline(bool homeTeam)` | `(e) runAiTouchline(homeTeam bool)` | match_engine.cpp:1308 |  | todo |
| `void refreshTargetBlends()` | `(e) refreshTargetBlends()` | match_engine.cpp:757 |  | todo |
| `void setStepTicks(std::uint32_t ticks)` | `(e) setStepTicks(ticks uint32)` | match_engine.cpp:774 |  | todo |
| `std::uint32_t backgroundStepTicks() const` | `(e) backgroundStepTicks() uint32` | match_engine.cpp:1442 |  | todo |
| `float stepSeconds() const` | `(e) stepSeconds() float64` | match_engine.cpp:782 |  | todo |
| `bool periodElapsed(std::uint64_t ticks) const` | `(e) periodElapsed(ticks uint64) bool` | match_engine.cpp:788 |  | todo |
| `void simulateStep(float dt)` | `(e) simulateStep(dt float64)` | match_engine.cpp:1630 |  | todo |
| `void updateTeamPhases()` | `(e) updateTeamPhases()` | match_engine.cpp:2177 |  | todo |
| `void setTransitionPhases(bool attackingHome)` | `(e) setTransitionPhases(attackingHome bool)` | match_engine.cpp:2159 |  | todo |
| `void setControlledPhases(bool attackingHome, float ballX)` | `(e) setControlledPhases(attackingHome bool, ballX float64)` | match_engine.cpp:2166 |  | todo |
| `void updateMovement(float dt)` | `(e) updateMovement(dt float64)` | match_engine.cpp:2231 |  | todo |
| `float estimatedArrivalTime(const MatchPlayer& candidate, Vector2F pressurePosition, const MatchPlayer* carrier)` | `(e) estimatedArrivalTime(candidate *MatchPlayer, pressurePosition Vector2F, carrier *MatchPlayer) float64` | match_engine.cpp:2282 |  | todo |
| `std::array<MatchPlayer*, 2> closestOutfieldPair(bool homeTeam, const MatchPlayer* carrier, Vector2F pressurePosition)` | `(e) closestOutfieldPair(homeTeam bool, carrier *MatchPlayer, pressurePosition Vector2F) [2]*MatchPlayer` | match_engine.cpp:2312 |  | todo |
| `float runPriority(const MatchPlayer& candidate, const MatchPlayer* carrier)` | `(e) runPriority(candidate *MatchPlayer, carrier *MatchPlayer) float64` | match_engine.cpp:2354 |  | todo |
| `bool executeCommand(const MatchCommandRecord& command)` | `(e) executeCommand(command *MatchCommandRecord) bool` | match_engine.cpp:921 |  | todo |
| `bool recordCommand(const MatchCommandRecord& command)` | `(e) recordCommand(command *MatchCommandRecord) bool` | match_engine.cpp:902 |  | todo |
| `void applyDueCommands()` | `(e) applyDueCommands()` | match_engine.cpp:914 |  | todo |
| `void startShout(bool homeTeam, MatchShout shout)` | `(e) startShout(homeTeam bool, shout MatchShout)` | match_engine.cpp:827 |  | todo |
| `void placeFormation(bool homeTeam)` | `(e) placeFormation(homeTeam bool)` | match_engine.cpp:1064 |  | todo |
| `float effectiveFamiliarity(std::size_t team) const` | `(e) effectiveFamiliarity(team int) float64` | match_engine.cpp:799 |  | present |
| `void simulateStepBody(float dt)` | `(e) simulateStepBody(dt float64)` | match_engine.cpp:1636 |  | todo |

## Stage B2: tactical targets, runners/supporters, assignMarks, tactics resolution (cpp 2398-3364, 3971-4342) (27)

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `float getTeamTalkEffect(bool homeTeam) const` | `(e) getTeamTalkEffect(homeTeam bool) float64` | match_engine.cpp:4236 |  | todo |
| `TacticalRole getSlotRole(bool homeTeam, std::size_t slot) const` | `(e) getSlotRole(homeTeam bool, slot int) TacticalRole` | match_engine.cpp:4006 |  | todo |
| `void resolveTactics(bool homeTeam)` | `(e) resolveTactics(homeTeam bool)` | match_engine.cpp:3976 |  | todo |
| `const RoleProfile& roleProfileOf(const MatchPlayer& player) const` | `(e) roleProfileOf(player *MatchPlayer) *RoleProfile` | match_engine.cpp:4012 |  | todo |
| `Vector2F possessionAnchor(const MatchPlayer& player) const` | `(e) possessionAnchor(player *MatchPlayer) Vector2F` | match_engine.cpp:4023 |  | todo |
| `float defensiveRoleShift(const MatchPlayer& player) const` | `(e) defensiveRoleShift(player *MatchPlayer) float64` | match_engine.cpp:4053 |  | todo |
| `OppositionInstruction instructionAgainst(const MatchPlayer& target) const` | `(e) instructionAgainst(target *MatchPlayer) OppositionInstruction` | match_engine.cpp:4061 |  | todo |
| `const MatchPlayer* tightMarkTarget(const MatchPlayer& marker) const` | `(e) tightMarkTarget(marker *MatchPlayer) *MatchPlayer` | match_engine.cpp:4072 |  | todo |
| `Vector2F tightMarkPoint(const MatchPlayer& marker, const MatchPlayer& target, Vector2F current) const` | `(e) tightMarkPoint(marker *MatchPlayer, target *MatchPlayer, current Vector2F) Vector2F` | match_engine.cpp:4085 |  | todo |
| `float tightMarkPenalty(const MatchPlayer& receiver) const` | `(e) tightMarkPenalty(receiver *MatchPlayer) float64` | match_engine.cpp:4099 |  | todo |
| `float passingRoleBias(const MatchPlayer& passer, const MatchPlayer& receiver, float progression, float completion) const` | `(e) passingRoleBias(passer *MatchPlayer, receiver *MatchPlayer, progression float64, completion float64) float64` | match_engine.cpp:4118 |  | todo |
| `float pressEagerness(const MatchPlayer& candidate, const MatchPlayer* carrier) const` | `(e) pressEagerness(candidate *MatchPlayer, carrier *MatchPlayer) float64` | match_engine.cpp:4131 |  | todo |
| `float engageBoost(const MatchPlayer& defender, const MatchPlayer& carrier) const` | `(e) engageBoost(defender *MatchPlayer, carrier *MatchPlayer) float64` | match_engine.cpp:4141 |  | todo |
| `float pressStandOffShare(const MatchPlayer& presser, const MatchPlayer& carrier) const` | `(e) pressStandOffShare(presser *MatchPlayer, carrier *MatchPlayer) float64` | match_engine.cpp:4154 |  | todo |
| `float weakFootShade(const MatchPlayer& carrier) const` | `(e) weakFootShade(carrier *MatchPlayer) float64` | match_engine.cpp:4164 |  | todo |
| `float weakFootPressure(const MatchPlayer& carrier) const` | `(e) weakFootPressure(carrier *MatchPlayer) float64` | match_engine.cpp:4178 |  | present |
| `std::optional<Vector2F> doubleUpPoint(const MatchPlayer& helper, const MatchPlayer& carrier, const MatchPlayer* firstPresser) const` | `(e) doubleUpPoint(helper *MatchPlayer, carrier *MatchPlayer, firstPresser *MatchPlayer) Optional[Vector2F]` | match_engine.cpp:4189 |  | todo |
| `float doubleUpPenalty(const MatchPlayer& carrier, const MatchPlayer& defender) const` | `(e) doubleUpPenalty(carrier *MatchPlayer, defender *MatchPlayer) float64` | match_engine.cpp:4213 |  | todo |
| `float talkSwing(bool homeTeam) const` | `(e) talkSwing(homeTeam bool) float64` | match_engine.cpp:4230 |  | todo |
| `void assignMarks()` | `(e) assignMarks()` | match_engine.cpp:4241 |  | todo |
| `void refreshTacticalTargets(float dt)` | `(e) refreshTacticalTargets(dt float64)` | match_engine.cpp:2615 |  | todo |
| `std::array<MatchPlayer*, MatchTuning::Shape::MAX_COMMITTED_RUNNERS> selectAttackingRunners(bool homeTeam, const MatchPlayer* carrier, const MatchPlayer* transitionSource)` | `(e) selectAttackingRunners(homeTeam bool, carrier *MatchPlayer, transitionSource *MatchPlayer) [Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer` | match_engine.cpp:2398 |  | todo |
| `float runChannel(const MatchPlayer& runner) const` | `(e) runChannel(runner *MatchPlayer) float64` | match_engine.cpp:2456 |  | todo |
| `MatchPlayer* selectFinalThirdPlayer( bool homeTeam, TacticalArrival arrival, const MatchPlayer* carrier, const std::array<MatchPlayer*, MatchTuning::Shape::MAX_COMMITTED_RUNNERS>& homeRunners, const std::array<MatchPlayer*, MatchTuning::Shape::MAX_COMMITTED_RUNNERS>& awayRunners)` | `(e) selectFinalThirdPlayer(homeTeam bool, arrival TacticalArrival, carrier *MatchPlayer, homeRunners [Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer, awayRunners [Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer) *MatchPlayer` | match_engine.cpp:2474 |  | todo |
| `std::array<MatchPlayer*, MatchTuning::Shape::MAX_ACTIVE_SUPPORTERS> selectActiveSupporters( bool homeTeam, const MatchPlayer* carrier, const std::array<MatchPlayer*, MatchTuning::Shape::MAX_COMMITTED_RUNNERS>& runners, const MatchPlayer* midfieldArrival, const MatchPlayer* farPostRunner, const MatchPlayer* overlappingFullback)` | `(e) selectActiveSupporters(homeTeam bool, carrier *MatchPlayer, runners [Shape_MAX_COMMITTED_RUNNERS]*MatchPlayer, midfieldArrival *MatchPlayer, farPostRunner *MatchPlayer, overlappingFullback *MatchPlayer) [Shape_MAX_ACTIVE_SUPPORTERS]*MatchPlayer` | match_engine.cpp:2535 |  | todo |
| `const MatchPlayer* findCoverOutlet(bool defendingHome, const MatchPlayer* carrier) const` | `(e) findCoverOutlet(defendingHome bool, carrier *MatchPlayer) *MatchPlayer` | match_engine.cpp:2583 |  | todo |
| `bool cutsInside(const MatchPlayer& player) const` | `(e) cutsInside(player *MatchPlayer) bool` | match_engine.cpp:3337 |  | todo |

## Stage B3: movement, restart movement, separation, load, possession/dribble/take-on/tackle/fouls/sanctions (cpp 3941-3970, 4343-5442) (16)

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `void accumulatePlayerLoad(float dt)` | `(e) accumulatePlayerLoad(dt float64)` | match_engine.cpp:4820 |  | todo |
| `void commitFoul(MatchPlayer& offender, MatchPlayer& victim, bool ballWon, bool reckless)` | `(e) commitFoul(offender *MatchPlayer, victim *MatchPlayer, ballWon bool, reckless bool)` | match_engine.cpp:5303 |  | todo |
| `void applySanction(MatchPlayer& offender, MatchRules::FoulSanction sanction)` | `(e) applySanction(offender *MatchPlayer, sanction FoulSanction)` | match_engine.cpp:5383 |  | todo |
| `bool deniesGoalChance(const MatchPlayer& victim, const MatchPlayer& offender) const` | `(e) deniesGoalChance(victim *MatchPlayer, offender *MatchPlayer) bool` | match_engine.cpp:5277 |  | todo |
| `void updatePendingAdvantage(float dt)` | `(e) updatePendingAdvantage(dt float64)` | match_engine.cpp:5427 |  | todo |
| `float currentTopSpeed(const MatchPlayer& player) const` | `(e) currentTopSpeed(player *MatchPlayer) float64` | match_engine.cpp:3952 |  | todo |
| `void integrateMovements(std::span<const std::uint8_t> slots, float dt, bool walking)` | `(e) integrateMovements(slots []uint8, dt float64, walking bool)` | match_engine.cpp:4343 |  | todo |
| `void updateRestartMovement(float dt)` | `(e) updateRestartMovement(dt float64)` | match_engine.cpp:4700 |  | todo |
| `void updateRestartSupport(MatchPlayer& player, const MatchPlayer* taker) const` | `(e) updateRestartSupport(player *MatchPlayer, taker *MatchPlayer)` | match_engine.cpp:4657 |  | todo |
| `void separatePlayers()` | `(e) separatePlayers()` | match_engine.cpp:4728 |  | todo |
| `void resolvePossessionAndActions(float dt)` | `(e) resolvePossessionAndActions(dt float64)` | match_engine.cpp:4951 |  | todo |
| `bool updateDribble(const MatchPlayer& carrier, float dt)` | `(e) updateDribble(carrier *MatchPlayer, dt float64) bool` | match_engine.cpp:5053 |  | todo |
| `float dribbleExposureShare() const` | `(e) dribbleExposureShare() float64` | match_engine.cpp:5044 |  | todo |
| `bool attemptTakeOn(MatchPlayer& carrier, MatchPlayer& defender)` | `(e) attemptTakeOn(carrier *MatchPlayer, defender *MatchPlayer) bool` | match_engine.cpp:5134 |  | todo |
| `void attemptTackle(MatchPlayer& carrier, MatchPlayer& defender, bool sliding)` | `(e) attemptTackle(carrier *MatchPlayer, defender *MatchPlayer, sliding bool)` | match_engine.cpp:5177 |  | todo |
| `void stepKinematics(MatchPlayer& player, Vector2F desired, float desiredSpeed, float topSpeed, float fatigue, float dt) const` | `(e) stepKinematics(player *MatchPlayer, desired Vector2F, desiredSpeed float64, topSpeed float64, fatigue float64, dt float64)` | match_engine.cpp:4570 |  | todo |

## Stage B4: decideAction, pass evaluation/choice, passBall, strikeShot, takeShot, launch, loft (cpp 5443-6469) (9)

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `void launchBall(const MatchPlayer& kicker, Vector2F origin, Vector2F target, float horizontalSpeed, float verticalSpeed, float curve)` | `(e) launchBall(kicker *MatchPlayer, origin Vector2F, target Vector2F, horizontalSpeed float64, verticalSpeed float64, curve float64)` | match_engine.cpp:6298 |  | todo |
| `static float groundLaunchSpeed(float distanceMetres, float arrivalSpeed)` | `func groundLaunchSpeed(distanceMetres float64, arrivalSpeed float64) float64` | match_engine.cpp:6316 | static in C++: package-level func | todo |
| `static float loftVerticalSpeed(float distanceMetres, float horizontalSpeed, float startHeight, float arrivalHeight)` | `func loftVerticalSpeed(distanceMetres float64, horizontalSpeed float64, startHeight float64, arrivalHeight float64) float64` | match_engine.cpp:6391 | static in C++: package-level func | todo |
| `void decideAction(MatchPlayer& carrier)` | `(e) decideAction(carrier *MatchPlayer)` | match_engine.cpp:5443 |  | todo |
| `void passBall(MatchPlayer& passer, const PassOption& option, bool forceLofted = false)` | `(e) passBall(passer *MatchPlayer, option *PassOption, forceLofted bool)` | match_engine.cpp:5946 | C++ default(s): forceLofted=false (pass explicitly) | todo |
| `void takeShot(MatchPlayer& shooter, float forcedXG = -1.0f, bool header = false, bool freeKick = false)` | `(e) takeShot(shooter *MatchPlayer, forcedXG float64, header bool, freeKick bool)` | match_engine.cpp:6244 | C++ default(s): forcedXG=-1.0f, header=false, freeKick=false (pass explicitly) | todo |
| `float strikeShot(const MatchPlayer& shooter, float forcedXG, bool header, bool penalty, bool freeKick = false)` | `(e) strikeShot(shooter *MatchPlayer, forcedXG float64, header bool, penalty bool, freeKick bool) float64` | match_engine.cpp:6107 | C++ default(s): freeKick=false (pass explicitly) | todo |
| `std::optional<PassOption> choosePassTarget(MatchPlayer& passer)` | `(e) choosePassTarget(passer *MatchPlayer) Optional[PassOption]` | match_engine.cpp:5866 |  | todo |
| `PassOption evaluatePassOption(const MatchPlayer& passer, MatchPlayer& receiver) const` | `(e) evaluatePassOption(passer *MatchPlayer, receiver *MatchPlayer) PassOption` | match_engine.cpp:5678 |  | todo |

## Stage B5: keeper, ball integration, loose ball, aerials, clears/headers, possession, out of bounds, goals (cpp 6470-7944) (23)

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `void beginStoppage()` | `(e) beginStoppage()` | match_engine.cpp:7933 |  | todo |
| `void extendRestart(float seconds)` | `(e) extendRestart(seconds float64)` | match_engine.cpp:7923 |  | todo |
| `Vector2F goalkeeperTarget(MatchPlayer& keeper, const MatchPlayer* carrier)` | `(e) goalkeeperTarget(keeper *MatchPlayer, carrier *MatchPlayer) Vector2F` | match_engine.cpp:6604 |  | todo |
| `void diveGoalkeeper(MatchPlayer& keeper, float dt)` | `(e) diveGoalkeeper(keeper *MatchPlayer, dt float64)` | match_engine.cpp:6545 |  | todo |
| `void planGoalkeeperDive(bool defendingHome, bool penalty)` | `(e) planGoalkeeperDive(defendingHome bool, penalty bool)` | match_engine.cpp:6470 |  | todo |
| `void resolveShotAtGoalkeeper(MatchPlayer& goalkeeper)` | `(e) resolveShotAtGoalkeeper(goalkeeper *MatchPlayer)` | match_engine.cpp:6884 |  | todo |
| `bool resolveAerialContest()` | `(e) resolveAerialContest() bool` | match_engine.cpp:7254 |  | todo |
| `void headBall(MatchPlayer& header)` | `(e) headBall(header *MatchPlayer)` | match_engine.cpp:7498 |  | todo |
| `bool tryOwnGoalTouch(MatchPlayer& defender)` | `(e) tryOwnGoalTouch(defender *MatchPlayer) bool` | match_engine.cpp:7411 |  | todo |
| `void clearBehind(MatchPlayer& defender)` | `(e) clearBehind(defender *MatchPlayer)` | match_engine.cpp:7449 |  | todo |
| `void clearBall(MatchPlayer& defender)` | `(e) clearBall(defender *MatchPlayer)` | match_engine.cpp:7469 |  | todo |
| `void parryShot(MatchPlayer& goalkeeper, bool overTheBar)` | `(e) parryShot(goalkeeper *MatchPlayer, overTheBar bool)` | match_engine.cpp:6910 |  | todo |
| `void recordPassCompletion(MatchPlayer& receiver)` | `(e) recordPassCompletion(receiver *MatchPlayer)` | match_engine.cpp:6954 |  | todo |
| `void updateBall(float dt)` | `(e) updateBall(dt float64)` | match_engine.cpp:6762 |  | todo |
| `void integrateBall(float dt)` | `(e) integrateBall(dt float64)` | match_engine.cpp:6811 |  | todo |
| `void updateBallInNet(float dt)` | `(e) updateBallInNet(dt float64)` | match_engine.cpp:7874 |  | todo |
| `void resolveLooseBall()` | `(e) resolveLooseBall()` | match_engine.cpp:6977 |  | todo |
| `SaveAttempt attemptSave(const MatchPlayer& keeper)` | `(e) attemptSave(keeper *MatchPlayer) SaveAttempt` | match_engine.cpp:6843 |  | todo |
| `void setPossession(MatchPlayer& player)` | `(e) setPossession(player *MatchPlayer)` | match_engine.cpp:7632 |  | todo |
| `void clearFlightState()` | `(e) clearFlightState()` | match_engine.cpp:7674 |  | todo |
| `void checkOutOfBounds()` | `(e) checkOutOfBounds()` | match_engine.cpp:7692 |  | todo |
| `void scoreGoal(bool homeTeam)` | `(e) scoreGoal(homeTeam bool)` | match_engine.cpp:7778 |  | todo |
| `void makeSave(MatchPlayer& goalkeeper)` | `(e) makeSave(goalkeeper *MatchPlayer)` | match_engine.cpp:7894 |  | todo |

## Stage B6: restarts/set pieces, substitutions, injuries, events/logging (cpp 7945-9701) (40)

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `void setTeamNames(std::string homeTeam, std::string awayTeam)` | `(e) setTeamNames(homeTeam string, awayTeam string)` | match_engine.cpp:9624 |  | todo |
| `bool substitutePlayer(uint32_t outPlayerId, const Player* inPlayer, std::optional<std::size_t> slot = std::nullopt)` | `(e) substitutePlayer(outPlayerId uint32, inPlayer *Player, slot Optional[int]) bool` | match_engine.cpp:9036 | C++ default(s): slot=std::nullopt (pass explicitly) | todo |
| `bool canSubstitute(bool homeTeam) const` | `(e) canSubstitute(homeTeam bool) bool` | match_engine.cpp:9009 |  | todo |
| `int getSubstitutionsUsed(bool homeTeam) const` | `(e) getSubstitutionsUsed(homeTeam bool) int` | match_engine.cpp:8999 |  | todo |
| `int getSubstitutionWindowsUsed(bool homeTeam) const` | `(e) getSubstitutionWindowsUsed(homeTeam bool) int` | match_engine.cpp:9004 |  | todo |
| `void setAutoSubstitutions(bool home, bool away)` | `(e) setAutoSubstitutions(home bool, away bool)` | match_engine.cpp:9028 |  | todo |
| `void setMedicalFlags(PlayerID playerId, std::uint8_t flags)` | `(e) setMedicalFlags(playerId PlayerID, flags uint8)` | match_engine.cpp:9167 |  | todo |
| `bool applyScenario(const MatchScenario& scenario, MatchState scenarioState = MatchState::PLAYING, float scenarioMatchTime = 0.0f, int scenarioHomeScore = 0, int scenarioAwayScore = 0)` | `(e) applyScenario(scenario *MatchScenario, scenarioState MatchState, scenarioMatchTime float64, scenarioHomeScore int, scenarioAwayScore int) bool` | match_engine.cpp:9514 | C++ default(s): scenarioState=MatchState::PLAYING, scenarioMatchTime=0.0f, scenarioHomeScore=0, scenarioAwayScore=0 (pass explicitly) | todo |
| `void checkInjuries()` | `(e) checkInjuries()` | match_engine.cpp:9336 |  | todo |
| `void injurePlayer(MatchPlayer& player, bool fromContact)` | `(e) injurePlayer(player *MatchPlayer, fromContact bool)` | match_engine.cpp:9355 |  | todo |
| `void removeFromPitch(MatchPlayer& player)` | `(e) removeFromPitch(player *MatchPlayer)` | match_engine.cpp:9369 |  | todo |
| `void rebalanceShape(bool homeTeam, Vector2F vacatedBase, std::int8_t vacatedSlot)` | `(e) rebalanceShape(homeTeam bool, vacatedBase Vector2F, vacatedSlot int8)` | match_engine.cpp:9423 |  | todo |
| `void ensureGoalkeeper(bool homeTeam)` | `(e) ensureGoalkeeper(homeTeam bool)` | match_engine.cpp:9466 |  | todo |
| `void runAiSubstitutions()` | `(e) runAiSubstitutions()` | match_engine.cpp:9151 |  | todo |
| `void runAiSubstitutionsFor(bool homeTeam)` | `(e) runAiSubstitutionsFor(homeTeam bool)` | match_engine.cpp:9205 |  | todo |
| `void runMedicalSubstitutions(bool homeTeam)` | `(e) runMedicalSubstitutions(homeTeam bool)` | match_engine.cpp:9178 |  | todo |
| `bool performSubstitution(MatchPlayer& outgoing, const Player* inPlayer, SubstitutionReason reason)` | `(e) performSubstitution(outgoing *MatchPlayer, inPlayer *Player, reason SubstitutionReason) bool` | match_engine.cpp:9049 |  | todo |
| `const Player* chooseReplacement(bool homeTeam, PlayerRole role, bool wantGoalkeeper) const` | `(e) chooseReplacement(homeTeam bool, role PlayerRole, wantGoalkeeper bool) *Player` | match_engine.cpp:9124 |  | todo |
| `bool userRestartPassRequested() const` | `(e) userRestartPassRequested() bool` | match_engine.cpp:8419 |  | todo |
| `void takeSetPiece(MatchPlayer& taker, MatchState restartState)` | `(e) takeSetPiece(taker *MatchPlayer, restartState MatchState)` | match_engine.cpp:8508 |  | todo |
| `void takeCorner(MatchPlayer& taker)` | `(e) takeCorner(taker *MatchPlayer)` | match_engine.cpp:8574 |  | todo |
| `void takeDirectFreeKick(MatchPlayer& taker)` | `(e) takeDirectFreeKick(taker *MatchPlayer)` | match_engine.cpp:8639 |  | todo |
| `void arrangeSetPiece(bool attackingHome, Vector2F ballPosition)` | `(e) arrangeSetPiece(attackingHome bool, ballPosition Vector2F)` | match_engine.cpp:8328 |  | todo |
| `MatchPlayer* bestSetPieceTaker(bool homeTeam, bool shooting)` | `(e) bestSetPieceTaker(homeTeam bool, shooting bool) *MatchPlayer` | match_engine.cpp:8248 |  | todo |
| `MatchPlayer* dutyTaker(bool homeTeam, SetPieceDuty duty)` | `(e) dutyTaker(homeTeam bool, duty SetPieceDuty) *MatchPlayer` | match_engine.cpp:8270 |  | todo |
| `void placeTaker(MatchPlayer& taker, Vector2F spot)` | `(e) placeTaker(taker *MatchPlayer, spot Vector2F)` | match_engine.cpp:8195 |  | todo |
| `MatchPlayer* restartTaker()` | `(e) restartTaker() *MatchPlayer` | match_engine.cpp:8228 |  | todo |
| `void setRestartTaker(MatchPlayer* taker)` | `(e) setRestartTaker(taker *MatchPlayer)` | match_engine.cpp:8235 |  | todo |
| `std::size_t slotOf(const MatchPlayer& player) const` | `(e) slotOf(player *MatchPlayer) int` | match_engine.cpp:8243 |  | todo |
| `void completeRestart()` | `(e) completeRestart()` | match_engine.cpp:8445 |  | todo |
| `void setupKickOff(bool homeKickingOff)` | `(e) setupKickOff(homeKickingOff bool)` | match_engine.cpp:7945 |  | todo |
| `void setupThrowIn(bool homeTeam)` | `(e) setupThrowIn(homeTeam bool)` | match_engine.cpp:7997 |  | todo |
| `void setupGoalKick(bool homeTeam)` | `(e) setupGoalKick(homeTeam bool)` | match_engine.cpp:8035 |  | todo |
| `void setupCorner(bool homeTeam, bool topCorner)` | `(e) setupCorner(homeTeam bool, topCorner bool)` | match_engine.cpp:8061 |  | todo |
| `void setupFreeKick(bool homeTeam, Vector2F foulPos)` | `(e) setupFreeKick(homeTeam bool, foulPos Vector2F)` | match_engine.cpp:8098 |  | todo |
| `void setupPenalty(bool homeTeam)` | `(e) setupPenalty(homeTeam bool)` | match_engine.cpp:8140 |  | todo |
| `Vector2F kickOffPosition(const MatchPlayer& player) const` | `(e) kickOffPosition(player *MatchPlayer) Vector2F` | match_engine.cpp:7985 |  | todo |
| `MatchEvent& logEvent(MatchEventType type)` | `(e) logEvent(t MatchEventType) *MatchEvent` | match_engine.cpp:9604 |  | todo |
| `MatchEvent& logEvent(MatchEventType type, const MatchPlayer& actor)` | `(e) logEventActor(t MatchEventType, actor *MatchPlayer) *MatchEvent` | match_engine.cpp:9692 | overload suffix "Actor" | todo |
| `std::string squadPlayerName(PlayerID playerId) const` | `(e) squadPlayerName(playerId PlayerID) string` | match_engine.cpp:9629 |  | todo |

## Not ported: playback / highlights / interpolation / debug JSON (18)

Do not implement.

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `void setPlaybackMode(MatchPlaybackMode mode)` | `(e) setPlaybackMode(mode MatchPlaybackMode)` | match_engine.cpp:1469 |  | todo |
| `MatchPlaybackMode getPlaybackMode() const` | `(e) getPlaybackMode() MatchPlaybackMode` | match_engine.h (inline) |  | todo |
| `void setPlaybackSpeed(float simulatedSecondsPerWallSecond)` | `(e) setPlaybackSpeed(simulatedSecondsPerWallSecond float64)` | match_engine.cpp:1475 |  | todo |
| `float getPlaybackSpeed() const` | `(e) getPlaybackSpeed() float64` | match_engine.h (inline) |  | todo |
| `void setHighlightPlaybackSpeed(float simulatedSecondsPerWallSecond)` | `(e) setHighlightPlaybackSpeed(simulatedSecondsPerWallSecond float64)` | match_engine.cpp:1483 |  | todo |
| `float getHighlightPlaybackSpeed() const` | `(e) getHighlightPlaybackSpeed() float64` | match_engine.h (inline) |  | todo |
| `bool advancePlayback(float wallSeconds)` | `(e) advancePlayback(wallSeconds float64) bool` | match_engine.cpp:1491 |  | todo |
| `bool isInHighlight() const` | `(e) isInHighlight() bool` | match_engine.cpp:1549 |  | todo |
| `const std::optional<MatchHighlight>& getScheduledHighlight() const` | `(e) getScheduledHighlight() Optional[MatchHighlight]` | match_engine.h (inline) |  | todo |
| `std::optional<MatchHighlight> predictNextHighlight( float horizonSeconds = MatchTuning::Playback::PREDICTION_HORIZON_SECONDS) const` | `(e) predictNextHighlight(horizonSeconds float64) Optional[MatchHighlight]` | match_engine.cpp:1561 | C++ default(s): horizonSeconds=MatchTuning::Playback::PREDICTION_HORIZON_SECONDS (pass explicitly) | todo |
| `const std::vector<MatchHighlight>& getHighlights() const` | `(e) getHighlights() []MatchHighlight` | match_engine.h (inline) |  | todo |
| `float getInterpolationAlpha() const` | `(e) getInterpolationAlpha() float64` | match_engine.cpp:1614 |  | todo |
| `const std::vector<Vector2F>& getPreviousPlayerPositions() const` | `(e) getPreviousPlayerPositions() []Vector2F` | match_engine.h (inline) |  | todo |
| `const std::vector<float>& getPreviousPlayerFacingAngles() const` | `(e) getPreviousPlayerFacingAngles() []float64` | match_engine.h (inline) |  | todo |
| `Vector2F getPreviousBallPosition() const` | `(e) getPreviousBallPosition() Vector2F` | match_engine.h (inline) |  | todo |
| `float getPreviousBallZ() const` | `(e) getPreviousBallZ() float64` | match_engine.h (inline) |  | todo |
| `std::string getDebugSnapshotJson() const` | `(e) getDebugSnapshotJson() string` | match_engine.cpp:9750 |  | todo |
| `bool writeDebugSnapshot(std::string_view path) const` | `(e) writeDebugSnapshot(path string) bool` | match_engine.cpp:9898 |  | todo |

## Not ported: play-mode public API (5)

Do not implement (controlled-player input is out of scope).

| C++ declaration | Go signature | C++ def | Notes | Status |
|---|---|---|---|---|
| `void setPlayHalfMinutes(int minutes)` | `(e) setPlayHalfMinutes(minutes int)` | match_engine.cpp:3370 |  | todo |
| `void submitInput(const MatchPlayerInput& input)` | `(e) submitInput(input MatchPlayerInput)` | match_engine.cpp:3415 |  | todo |
| `void loadInputReplay(std::vector<MatchInputRecord> log)` | `(e) loadInputReplay(log []MatchInputRecord)` | match_engine.cpp:3425 |  | todo |
| `PlayerID suggestActivePlayer(bool homeTeam, PlayerID current) const` | `(e) suggestActivePlayer(homeTeam bool, current PlayerID) PlayerID` | match_engine.cpp:3870 |  | todo |
| `PlayerID nextSwitchCandidate(bool homeTeam, PlayerID current, PlayerID skip = 0) const` | `(e) nextSwitchCandidate(homeTeam bool, current PlayerID, skip PlayerID) PlayerID` | match_engine.cpp:3910 | C++ default(s): skip=0 (pass explicitly) | todo |

