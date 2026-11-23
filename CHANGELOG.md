# Changelog (match-engine behaviour)

Each entry names the config hash. A replay's `meta.config_hash` says which engine behaviour produced it.

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
