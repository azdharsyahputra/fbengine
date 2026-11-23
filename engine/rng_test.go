// Ported from Player12 (Go-only test of the std::mt19937 port) (GPL-3.0)

package engine

import "testing"

// The C++ standard requires the 10000th invocation of a default-constructed
// std::mt19937 (seed 5489) to produce 4123659995.
func TestMt19937MatchesStandard(t *testing.T) {
	engine := newMt19937(5489)
	var value uint32
	for i := 0; i < 10000; i++ {
		value = engine.next()
	}
	if value != 4123659995 {
		t.Fatalf("10000th output = %d, want 4123659995", value)
	}
}

func TestMt19937CopyIsIndependent(t *testing.T) {
	a := newMt19937(42)
	a.next()
	b := a
	if a.next() != b.next() {
		t.Fatal("copied generator diverged")
	}
}

func TestRunTimingEpochSteps(t *testing.T) {
	if RUN_TIMING_EPOCH_STEPS != 50 {
		t.Fatalf("RUN_TIMING_EPOCH_STEPS = %d, want 50", RUN_TIMING_EPOCH_STEPS)
	}
}
