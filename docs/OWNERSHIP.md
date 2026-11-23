# Decision ownership and spatial model (Phase 2 / items A and B, 2026-11-23)

Every player target records three things:
- `TargetOwner`: the layer that decided the job and the spot.
- `TargetReason`: why (the branch or task).
- `TargetRefiner`: the layer that only fine-tuned the spot, or none.

The audit probe (`FMENGINE_AUDIT=1`) prints the live distribution as `owner:reason(+refiner)`.

## Single owner per decision

| Decision | Owner | Reasons |
|---|---|---|
| Team phase | P12 `updateTeamPhases` | — |
| Block shift anchor | M4 `feM4ShapeAnchor` | — |
| **Defending: opponent holds the ball, or an opponent pass (ground or open-play cross) is in flight** | **M1** (`feM1Assign`/`feM1Apply`) | m1-first-defender, m1-cover, m1-screen, m1-contain, m1-protect-depth, m1-recover-shape, m1-marker, m1-shape |
| ↳ where the Contain stands | M1 (`m1.contain_distance`) | m1-contain |
| ↳ when a defender steps in to tackle | M6 (`feChallenges`) | — |
| ↳ against an open-play cross | M1. Engagers mark the receiver goal-side (no sprint); the aerial contest decides the ball. | m1-* |
| Loose-ball contest (nobody controls, no pass) | LOOSE resolver (`loose_ball.go`) | loose-claim, loose-cover, loose-delay |
| ↳ everyone else during a loose ball | P12 transition branch | transition-run, transition-support, recover |
| Set-piece delivery in flight (corner, free kick) | M9 arrangement, held by P12 | aerial-hold |
| Shot in flight | P12 | flight-hold, flight-recover, flight-mark |
| Own pass in flight | P12 (receiver meets the ball, others drift) | receive, flight-support |
| Carrier's path | P12 (M4 byline guard) | carry, byline |
| **Attacking off-ball job** | P12 branch or M5 duty (`compose_attack.go`) | runner, runner-box, arrival, far-post, overlap, duty-link/runner/security, support-near/square/trailing, short-forward, shape |
| ↳ duty spot | M5, final (not refined) | duty-* |
| ↳ holder's structure, one rule per role | Full-back being pushed → M4 push. Other full-backs, ST, wingers → M10 structure. No.10 / central midfielders → M4 compaction. | structure |
| ↳ spacing / getting open | M4 refine (refiner only). M10's getting-open is a candidate set + lane score inside it. | (+M4) |
| Carrier's action | M2 (`feM2Decide`) | — |
| Movement execution | M3 steering | — |
| Restart walk | M9 (P12 with M9 off) | restart, half-time-walk |
| Goalkeeper | P12 | gk |

## What changed versus go6

1. **A2:** `TargetReason` everywhere. Behaviour unchanged.
2. **A1:** the attack chain (P12 → M5 → M4 push → M4 compact → M4 refine → M10 structure → M10 getting-open) became one composition, each step with a defined owner.
   - M10 no longer overwrites M4 or M5 afterwards.
   - The full-back push, compaction and M10 structure no longer stack.
3. **A3:** M1 also owns open-play crosses (was P12 "hold marks").
   - Set-piece deliveries stay with M9.
   - First defender, Contain and Intercept mark the receiver instead of sprinting at him.
   - Mistake made and fixed while doing this: splitting M1's role switch let its `default:` overwrite engager targets.

## Results (10 seeds, live mode)

| | go6 (5 seeds) | after A (10 seeds) | Real |
|---|---|---|---|
| Ping-pong | 14.4 | 15.1 | 8 |
| chase≥3 | 8.4 | 7.9 | 5.8 |
| Loose ≥5 | 8.2 | 8.8 | 6.8 |
| Box free (probe) | 17.6% | 16.2% | — |
| DM–AM | 18.3 | 17.7 | 8.6 |
| Fouls per side | 21 | 21 | ~10 |
| Backward | 31 | 31 | 24 |

## Still not single-owner (by design or open)

- **Positions of non-claimers during a loose ball:** P12 transition logic. The resolver owns only the contest.
- **Set-piece deliveries:** M9 / P12 hold.
- **The P12 out-of-possession branch** (press, double, cover-lane, tight/zonal mark) runs only with M1 switched off.

## Spatial model (item B)

There is one source for each spatial fact. All of it is read-only, with no RNG.

| Fact | Single source | Used by |
|---|---|---|
| Who gets where, when | `kinematics.go`: `bodyTimeTo` (time to a point at an effort) / `bodyReach` (distance in t). Uses the player's own top speed (fatigue, sprint reserve, injury), acceleration and current velocity. | M1 slot choice and windows; M2 pass race; M4 ball-control anchor; M5 duty arrivals; P12 presser order (plus its eagerness/continuity bias); loose-ball resolver |
| Who controls a point | `spatial.go` `SpatialControl(side, p)`: logistic in the two sides' earliest arrivals, τ 0.45 s, exact per query | M4 refine (space/openness terms); M5 duty space |
| Is a lane open | `spatial.go` `spatialLaneBlockers` (opponents incl. keepers projecting onto the segment within r) | Positioning wants 0 blockers (`spatialLaneOpen`: M4, M5, M10). M2 rejects a ground pass at ≥2 within 2.5 m; its race prices single bodies. |
| Does a pass arrive | M2 ball-vs-bodies race (uses `bodyReach`) | M2 only |

- **Replaced:**
  - `feM1ArrivalTime` / `feM2Reach`, moved unchanged into kinematics.
  - `feM4BodyReach` (uniform 8.5 m/s from standstill).
  - P12 distance / pursuit-speed estimate.
  - `feM4LaneClear` and M2's own `feM2LaneClear` copy.
  - FE distance proxies in `feM4Score` and `feM5Space`.
- **Only on the M2-off path:** P12 `passingLaneRisk`.
- **Tried and rejected:**
  - A pitch-control grid rebuilt every tactical refresh: 7× slower, since refreshes happen nearly every step.
  - Dropping M2's lane filter entirely (loose ≥5 8.7 → 10.2).

## Defensive behaviour (item C)

### C1: one box metric

The probe and `viewer/cek/crit.js` now use the same zones and definitions:
- **Ball depth `u`** is measured from the defended goal line.
- **Zones:**
  - "front of box": 16–35 m, y 14–54; this is cek's "bertahan depan kotak";
  - wide final third;
  - in the box.
- **Counts** are outfielders only. An attacker in the box is free when no defending outfielder is within 3 m.

On the same match the two agree. For example, go8 seed 19, front-of-box zone:

| | Defenders in box | Free attackers in box |
|---|---|---|
| Probe | 3.52 | 16% |
| cek | 3.5–3.6 | 12–17% |

The old "2.7" came from go4. The real-data box numbers (1.2 defenders, 42% free) are camera-limited and are **not** a target.

### C2: box baseline (no reliable real target, so not tuned)

go9, 10 seeds, front-of-box zone:
- defenders in box about 3.0;
- free attackers about 18%.
- The wide zone (crossing positions) has ~30% free attackers. The in-box zone has ~25%.

### C3: organised press (M1 owns it)

- **When:** the ball is in the opponent's third (`press.depth` 17.5, team frame) and there is no near-goal engagement.
- **What:** the M1 Contain is sent at any carrier someone reaches within `press.window` (6 s), instead of `m1.contain_window` (2 s). He still jockeys at `m1.contain_distance`, and M6 decides the tackle.
- **Off by default, measured:**
  - Block step-up (`press.forward_gap` / `mid_gap`, 99 = off).
  - Lower presser effort (`press.effort`): no effect, because M3 drives PRESS at full effort.
- **Trade-off found:** any stronger press (wider zone, step-up) raises chase≥3. Real teams get pressure from already-close positions, not long runs.
