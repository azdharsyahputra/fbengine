# Changelog (match-engine behaviour)

Each entry names the config hash. A replay's `meta.config_hash` says which engine behaviour produced it.

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
