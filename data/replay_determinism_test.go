package data_test

// Determinism, configuration isolation and concurrency of whole matches
// (Phase 1 foundation). Needs the CSV; skipped without it.

import (
	"os"
	"slices"
	"sync"
	"testing"

	"fmengine/data"
	"fmengine/engine"
)

const csvPath = "/Users/csadeveloper/fm/efem_epl_full.csv"

type fixture struct{ home, away *data.Squad }

func load(t *testing.T) fixture {
	t.Helper()
	if _, err := os.Stat(csvPath); err != nil {
		t.Skip("CSV not available:", err)
	}
	sq := func(club string, id engine.TeamID) *data.Squad {
		rows, err := data.LoadClub(csvPath, club)
		if err != nil {
			t.Fatal(err)
		}
		s, err := data.BuildSquad(rows, id, "4-2-3-1")
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	return fixture{sq("Liverpool", 1), sq("Man City", 2)}
}

func (f fixture) run(t *testing.T, seed uint32, opt engine.RunOptions) *engine.ReplayData {
	// Fresh squads per run: line-ups are not shared between concurrent matches.
	rep, err := engine.RunReplayWith(f.home.Lineup, f.away.Lineup, f.home.Club, f.away.Club, seed, 4, opt)
	if err != nil {
		t.Error(err)
		return nil
	}
	return rep
}

func sameGameplay(a, b *engine.ReplayData) bool {
	return a != nil && b != nil && a.Ticks == b.Ticks && slices.Equal(a.Frames, b.Frames) &&
		slices.Equal(a.Events, b.Events) && slices.Equal(a.Cond, b.Cond) && slices.Equal(a.Live, b.Live) &&
		a.Stats == b.Stats
}

func variant() engine.EngineConfig {
	c := engine.DefaultConfig()
	engine.ApplyOverrides(&c, "", "M1", "dw=0.6", "pf=0.003")
	return c
}

func TestSameSeedSameConfigIdentical(t *testing.T) {
	f := load(t)
	a := f.run(t, 19, engine.RunOptions{Config: engine.DefaultConfig()})
	b := f.run(t, 19, engine.RunOptions{Config: engine.DefaultConfig()})
	if !sameGameplay(a, b) {
		t.Fatal("same seed + same config produced different matches")
	}
	if a.ConfigHash != engine.DefaultConfig().Hash() || len(a.Live) != a.Ticks {
		t.Fatalf("metadata: hash %s live %d/%d", a.ConfigHash, len(a.Live), a.Ticks)
	}
	live := 0
	for _, l := range a.Live {
		if l {
			live++
		}
	}
	if live == 0 || live == a.Ticks {
		t.Fatalf("live flag degenerate: %d/%d", live, a.Ticks)
	}
}

func TestDiagnosticsDoNotChangeGameplay(t *testing.T) {
	f := load(t)
	plain := f.run(t, 19, engine.RunOptions{Config: engine.DefaultConfig()})
	for _, d := range []engine.Diagnostics{{Audit: true}, {M2Trace: 1}, {M2Trace: 2}, {M2Trace: 3, Audit: true}} {
		d.Out = discard{}
		got := f.run(t, 19, engine.RunOptions{Config: engine.DefaultConfig(), Diagnostics: d})
		if !sameGameplay(plain, got) {
			t.Fatalf("diagnostics %+v changed the match", d)
		}
		if d.Audit && got.Audit == "" {
			t.Fatal("audit enabled but empty report")
		}
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// Different configs on the same seed give different matches, and running
// them concurrently (with audit and trace on, the formerly global state)
// reproduces each one's sequential result exactly.
func TestConcurrentMatchesAreIsolated(t *testing.T) {
	f := load(t)
	def, alt := engine.DefaultConfig(), variant()
	refA := f.run(t, 19, engine.RunOptions{Config: def})
	refB := f.run(t, 19, engine.RunOptions{Config: alt})
	if sameGameplay(refA, refB) {
		t.Fatal("a different config produced the same match: config not applied")
	}
	if refA.ConfigHash == refB.ConfigHash {
		t.Fatal("different configs share a hash")
	}
	const n = 4
	got := make([]*engine.ReplayData, n)
	fixtures := make([]fixture, n)
	for i := range fixtures {
		fixtures[i] = load(t) // independent line-ups per goroutine, loaded up front
	}
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg := def
			if i%2 == 1 {
				cfg = alt
			}
			got[i] = fixtures[i].run(t, 19, engine.RunOptions{Config: cfg,
				Diagnostics: engine.Diagnostics{Audit: true, M2Trace: 3, Out: discard{}}})
		}(i)
	}
	wg.Wait()
	for i, r := range got {
		want := refA
		if i%2 == 1 {
			want = refB
		}
		if !sameGameplay(r, want) {
			t.Fatalf("concurrent match %d differs from its sequential reference", i)
		}
	}
}
