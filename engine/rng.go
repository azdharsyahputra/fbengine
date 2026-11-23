// Ported from Player12 src/model/match_engine.h (std::mt19937 members) — hand-written MT19937 (GPL-3.0)

package engine

// mt19937 is a hand-written port of std::mt19937 (32-bit Mersenne Twister,
// default seeding with a single 32-bit value). It produces the same raw
// output stream as the C++ standard library for the same seed. It is a
// plain value type, so copying a MatchEngine copies its random state (as
// the C++ engine copy does).
type mt19937 struct {
	state [mt19937_N]uint32
	index int
}

const (
	mt19937_N          = 624
	mt19937_M          = 397
	mt19937_MATRIX_A   = 0x9908b0df
	mt19937_UPPER_MASK = 0x80000000
	mt19937_LOWER_MASK = 0x7fffffff
)

// newMt19937 returns a generator seeded like std::mt19937(seed).
func newMt19937(seed uint32) mt19937 {
	var engine mt19937
	engine.seed(seed)
	return engine
}

// seed re-seeds like std::mt19937::seed(value).
func (m *mt19937) seed(value uint32) {
	m.state[0] = value
	for i := 1; i < mt19937_N; i++ {
		previous := m.state[i-1]
		m.state[i] = 1812433253*(previous^(previous>>30)) + uint32(i)
	}
	m.index = mt19937_N
}

func (m *mt19937) twist() {
	for i := 0; i < mt19937_N; i++ {
		y := (m.state[i] & mt19937_UPPER_MASK) | (m.state[(i+1)%mt19937_N] & mt19937_LOWER_MASK)
		next := m.state[(i+mt19937_M)%mt19937_N] ^ (y >> 1)
		if y&1 != 0 {
			next ^= mt19937_MATRIX_A
		}
		m.state[i] = next
	}
	m.index = 0
}

// next returns the next raw 32-bit output (C++ `engine()`).
func (m *mt19937) next() uint32 {
	if m.index >= mt19937_N {
		m.twist()
	}
	y := m.state[m.index]
	m.index++
	y ^= y >> 11
	y ^= (y << 7) & 0x9d2c5680
	y ^= (y << 15) & 0xefc60000
	y ^= y >> 18
	return y
}
