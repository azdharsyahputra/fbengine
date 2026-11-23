# Changelog (match-engine behaviour)

Each entry names the config hash. A replay's `meta.config_hash` says which engine behaviour produced it.

## go13: carriers left to dribble in midfield (2026-11-23)

From the go11 critique, problem 2.

- **Cause (new probe lines `AUDIT long-carry …`):**
  - In a ≥25 m carry the carrier had no defender within 5 m for 86% of the spell.
  - In 62% of its steps no M1 engager was assigned at all.
  - The midfield Contain was sent only at a carrier someone could reach within 2 s (FE local window). A winger in space was out of reach, so the block just held.
  - M2 then chose to carry at every decision (7–8 per spell). Carrying beat the best pass by ~0.02 goal value, so the hold limit (8 s) ended many spells.
- **Change:** `m1.contain_window` 2 → 4 s. A carrier in midfield space gets a goal-side defender out. He still jockeys at `m1.contain_distance`, and M6 decides the tackle.
- **Tried and dropped:**
  - Jockey time (Contain closes after N s): carries 29–37 per match.
  - Capped back-pedal speed: 31–38 per match.
  - A separate, larger midfield contain distance: fewer fouls, but less midfield pressure.
- **Result (10 seeds vs go12; probe carries ≥25 m, cek for the rest):**

  | | go12 | go13 | Real |
  |---|---|---|---|
  | Carries ≥25 m (probe) | 34 | 9.4 | — |
  | Carries ≥25 m (replay holder test, 3 seeds) | 28–34 | 4–6 | 0–2 |
  | Carries ≥15 m (same) | 55–77 | 24–25 | 3–12 |
  | Midfield ≥1 defender within 5 m | 31% | 47% | 47–54% |
  | Receptions under pressure | 19.5% | 21% | 26–37% |
  | Backward passes, middle third | 22% | 25% | 24–27% |
  | Backward passes, final third | 36% | 35% | 27–34% |
  | chase≥3 | 4.7 | 5.9 | 5.8 |
  | Engine ping-pong | 5.6 | 6.5 | ~8 (cek) |
  | cek ping-pong | 12.3 | 17.0 | 8.3 |
  | Fouls per side | 16.4 | 18.4 | ~10 |

- **Cost:** more midfield duels. The cek ping-pong counts duel contact as a flip, so it rises. Real possession flips (engine truth) stay below the real figure. Fouls rise by ~2 per side.
- **Reproduce go12:** `FMENGINE_M1=cw=2`.

## go12: midfield hole (2026-11-23)

From watching go11; see the notes file `kritik-coach-go11.md`.

- **Cause:** in a final-third attack the pivots trailed the ball by ~29 m (DM–AM 23 m) although their support slots were near the ball. M3 moved settled supporters at ~0.19 of top speed (max ~1.7 m/s toward the ball), so they never arrived.
- **Change:** a settled supporter or holder whose spot is more than `m3.catch_up_distance` (8 m) ahead in the attacking direction moves up at `m3.catch_up_share` 0.6 of top speed.
- **Result (10 seeds vs go11):**

  | | go11 | go12 |
  |---|---|---|
  | DM–AM | 21 m | 14.7 m |
  | DM behind ball, final third | −29 m | −17 m |
  | Final-third backward | 39% | 36% |
  | Ping-pong | 13.7 | 12.3 |
  | cek loose ≥5 | 10.8 | 13.3 |
  | Probe loose ≥5 | 31 | 30 |

- The cek loose ≥5 count rises because the support runs that follow a forward pass count as "running at the ball". The probe's truly-loose count does not change.
- **Also fixed** (`de9620c`): own-goal team in replay events, save/block texts, CLI shot attribution, and the engine match clock in replays (the viewer reads it).
- **Reproduce go11:** `FMENGINE_M1=cs=0`.

## Phase 2, item F (data validation) (2026-11-23)

No behaviour change.

- **Reproduction:**
  - Replays record `meta.data.lineup_sha256`: both built squads, every attribute, numbers, labels, bench.
  - `--reproduce=replay.js [out.js]` replays a match from its meta (clubs, formation, seed, hash-checked EngineConfig).
  - It first checks the CSV sha256 and the line-up hash. A changed CSV or line-up is an error unless `--allow-data-change`.
  - It then compares the gameplay payload and prints REPRODUCED or MISMATCH (exit 0 / 1).
  - `--config=replay.js` also refuses a changed CSV.
- **Clear errors for:**
  - non-engine replays (real tracking data);
  - schema-1 replays;
  - replays whose config fields have since changed (use the engine version from git).
- **Tests:** `cmd/match/main_test.go` covers the round trip, a changed CSV, changed line-ups, `--allow-data-change`, tampered frames, an edited config and line-up hash determinism.
- **Real data:** SkillCorner replays carry the heuristic live/dead mask (`viewer/cek/_deadball.js`, validated 97–99% against engine labels).

## go11: Phase 2, item E (action decisions) (2026-11-23)

Config `749dbb80…`.

- **E2:** M2 owns the pass value as named terms. Progress and to-feet moved from M10 into `m2.*`. Behaviour is unchanged (go10 replays identical).
- **E3:**
  - `m2.duel_loss` 0.5 (was 0.3).
  - `m2.progress_final` 0.003 (was 0.0012).
- **Harness:** gains real-comparable decision metrics (`audit/press.js`, `zone.js`).
- **Result (10 seeds vs go10):**

  | | go10 | go11 | Real |
  |---|---|---|---|
  | Backward | 27% | 24% | 24% |
  | Final-third backward | 48% | 39% | 27–34% |
  | Fouls per side | 18.8 | 15.9 | — |
  | chase≥3 | 5.1 | 4.6 | — |
  | Box free | 21% | 28% | — |
  | Loose ≥5 | 10.3 | 10.8 | — |

- **Unchanged by these levers:** receptions under pressure (~19–20%, real ~32%) and possession median (~10 s, real 6.3 s).
- **Reproduce go10:** `FMENGINE_M2TUNE=dw=0.3 FMENGINE_M10=pf=0.0012`.

## go10: Phase 2, item D (ball behaviour) (2026-11-23)

Config `ad85f0ba…`.

- **Change:** a bypassed forward trots back. `m1.forward_bypassed_effort` 0; he sprints only inside `m1.bypassed_sprint_goal` 25 m (go9: 0.6 / 40 m).
- **Result (20 seeds vs go9):**

  | | go9 | go10 | Real |
  |---|---|---|---|
  | chase≥3 | 10.1 | 5.2 | 5.8 |
  | Backward | 30 | 27 | 24 |
  | Ping-pong | 14.7 | 13.0 | — |
  | Fouls per side | 20.8 | 18.9 | — |
  | Box free | 18% | 22% | — |
  | Middle-third pressure ≥1 in 5 m | 35.5% | 31.5% | — |
  | Loose ≥5 | 9.4 | 9.9 | — |

- **Ping-pong:** engine-truth ping-pongs are 4–7 per match. The rest of the cek count is duel contact (visual steals).
- `m1.recover_clearance` was added and is off (no effect).
- **Reproduce go9:** `FMENGINE_M1=fb=0.6,fg=40`.

## go9: Phase 2, item C (defending) (2026-11-23)

Config `1f84b21c…`.

- **C1:** the audit probe uses the same box zones as cek `crit.js`. The two now agree (≈3.5 defenders in box, 12–17% free, front-of-box zone).
- **C3:** M1 organised press. With the ball in the opponent's third, the Contain is sent within 6 s.
- **Result (10 seeds vs go8):**

  | | go8 | go9 | Real |
  |---|---|---|---|
  | Nearest defender to a carrier in his own third | 13.0 m | 7.2 m | 5–6.6 m |
  | ≥1 defender within 5 m (own third) | 13.5% | 25% | 41–51% |
  | Ping-pong | 15.4 | 13.3 | — |
  | Fouls per side | 19.9 | 19.6 | — |
  | chase≥3 | 8.0 | 10.0 | 5.8 |
  | Loose ≥5 | 8.0 | 8.8 | 6.8 |

- **Tried and left off:**
  - Block step-up: own-third pressure 31%, but chase≥3 11.6 and loose 10.9.
  - Lower presser effort: no effect.
  - Press zone from the halfway line: middle-third pressure 41%, but fouls 23.
- **Reproduce go8:** `FMENGINE_M1=pz=99`.

## go8: Phase 2, item B (spatial model) (2026-11-23)

Config `9f961677…` (`m2.lane_clear_radius` removed).

- **B0/B1: one arrival model** (`kinematics.go`). M4's control anchor and Player12's presser order now use it too.
- **B2: one lane definition** (`spatialLaneBlockers`), with per-consumer tolerance.
- **B3: pitch control** (`SpatialControl`) feeds M4 spacing and M5 duty space.
- Details in `docs/OWNERSHIP.md` → Spatial model.
- **Result (10 seeds vs go7):**

  | | go7 | go8 |
  |---|---|---|
  | chase≥3 | 7.9 | 8.0 |
  | Loose ≥5 | 8.8 | 8.0 |
  | Probe loose ≥5 | 27 | 23 |
  | Fouls per side | 21 | 19.9 |
  | Box free | 16.2% | 18.9% |
  | Ping-pong | 15.1 | 15.4 |

- The ping-pong metric swings 10.5–15.4 between seed sets for the same config. Judge it on 20+ seeds.

## go7: Phase 2, item A (decision ownership) (2026-11-23)

Config `83a5d924…` (unchanged; code-level).

- **A2:** `TargetReason` and `TargetRefiner` are recorded at every target writer.
- **A1:** single-owner attack composition (`compose_attack.go`).
  - M5 duties are final.
  - One structure rule per role.
  - M4 refine is the only spacing step, with getting-open folded in.
- **A3:** M1 owns open-play crosses. Engagers mark the receiver goal-side.
- The full matrix is in `docs/OWNERSHIP.md`.
- **Result (10 seeds):** chase≥3 7.9, loose≥5 8.8, box free 16.2%, DM–AM 17.7. Ping-pong 15.1 and fouls 21 are within seed noise of go6.

## go6: Phase 2, step 2 (2026-11-23)

Config `83a5d924…`.

- **Loose-ball resolver** (`loose_ball.go`, owner tag `LOOSE`):
  - Ball nobody controls and no pass carrying it: both sides race to M4's control point using M1's arrival model.
  - A side losing by more than `loose.contest_gap` (0.25 s) doesn't sprint at the ball. Its first man waits `loose.delay_distance` (4 m) goal-side.
  - `loose.support_pull` makes Player12's transition pull toward the ball configurable (unchanged, 0.12).
- **Result (5 seeds, live):**

  | | go5 | go6 | Real |
  |---|---|---|---|
  | Ping-pong | 20 | 14.4 | 8 |
  | Loose ball ≥5 | 9.0 | 8.2 | 6.8 |
  | Box free | 21% | 17.6% | — |
  | chase≥3 | 8.4 | 8.4 | 5.8 |
  | Possession median | 8.8 s | 9.7 s | 6.3 s, moving away |

- **Not changed, measured:** `m1.contain_window` (2 s). Widening it (3–10 s) raised chase≥3 to 13 and fouls to 29 per side without moving defenders-within-5 m (median stays 0, real 1). The pressing deficit is not Contain assignment; it is still open.
- **Probe finding:** most players running at a truly loose ball are recoveries and transition support, not the claimers.
- **Reproduce go5:** `FMENGINE_M1=lg=-1` (payload identical).

## go5: Phase 2, step 1 (2026-11-23)

Config `85cee7dd…`.

- **What changed:** an M1 Contain now jockeys 2 m goal-side of the predicted ball (`m1.contain_distance`; go4 = 0.75 m, i.e. on the ball). Contain decides where the defender stands; the M6 challenge decides when he steps in.
- **Diagnosis (audit probe):**
  - The cek "ping-pong" (34/match) was mostly *visual steals*: ~39 per match, 84–91% by Contain, where a defender is nearer the ball than the carrier without winning it.
  - Real possession flips (engine truth) were only ~9 per match.
- **Result (5 seeds, live mode):**

  | | go4 | go5 | Real |
  |---|---|---|---|
  | Ping-pong | 34 | 20 | 8 |
  | chase≥3 | 10.6 | 8.4 | 5.8 |
  | Loose ball ≥5 | 11.6 | 9.0 | 6.8 |
  | Box free | 27% | 21% | — |
  | Fouls per side | 24 | 21 | — |
  | Possession median | 7.1 s | 8.8 s | 6.3 s |

- **Reproduce go4:** `FMENGINE_M1=cd=0.75`. The payload is byte-identical to go4.

## Phase 1 (2026-11-23)

Config `4c4a29ff…` / `56567dbd…`; behaviour = go4.

- Instrumentation and per-match configuration only.
- See fm-game/docs/phase1-instrumentasi.md.
