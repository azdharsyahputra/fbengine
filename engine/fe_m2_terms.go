// M2 pass value as named terms (ours, Phase 2 / E2). M2 owns the whole pass
// valuation; formerly the M10 layer added its progress bonus (and gated the
// to-feet retry) from outside. The terms, all in goal-probability units:
//   - core: FootballEngine's expected value (DecisionValue.pass / cross,
//     with the receiver-loss adjustment), from the ball-vs-bodies race;
//   - cooperation: the M5 collective duty bias (Decision.fs:331-376);
//   - progress: our progress appetite, per metre gained, rising up the
//     pitch (m2.progress_own / _mid / _final).
package engine

type feM2PassTerms struct {
	core, cooperation, progress float64
}

func (t feM2PassTerms) sum() float64 { return t.core + t.cooperation + t.progress }

// feM2ProgressTerm is the progress appetite at the carrier's team-frame x
// (own third → middle → final third; the own third ends at −17.5 m).
func (e *MatchEngine) feM2ProgressTerm(tx float64) float64 {
	m := &e.cfg.M2
	const third = 105.0 / 6.0
	switch {
	case tx <= -third:
		return m.ProgressOwn
	case tx < 0:
		return hLerp(m.ProgressOwn, m.ProgressMid, (tx+third)/third)
	case tx < third:
		return hLerp(m.ProgressMid, m.ProgressFinal, tx/third)
	}
	return m.ProgressFinal
}
