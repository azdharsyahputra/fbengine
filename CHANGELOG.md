# Changelog (match-engine behaviour)

Each entry names the config hash. A replay's `meta.config_hash` says which engine behaviour produced it.

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
