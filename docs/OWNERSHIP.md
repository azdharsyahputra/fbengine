# Decision ownership (Phase 2 / item A, 2026-11-23)

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
