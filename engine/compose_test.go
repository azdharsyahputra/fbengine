package engine

import (
	"math"
	"testing"
)

// a 4-player scene: players 0,1 home, 2,3 away, slots far apart
func composeScene() ([][2]float64, []bool, []bool) {
	slots := [][2]float64{{20, 20}, {40, 20}, {60, 40}, {80, 40}}
	return slots, []bool{true, true, true, true}, []bool{true, true, false, false}
}

func TestComposeHoldsSlotWithoutRequests(t *testing.T) {
	slots, ok, home := composeScene()
	out := compose(defaultComposeConfig(), slots, ok, home, nil)
	for i := range out {
		if out[i].Target != slots[i] || out[i].Class != -1 {
			t.Fatalf("player %d: %+v, want slot %v", i, out[i], slots[i])
		}
	}
}

func TestComposeQuotaPerSide(t *testing.T) {
	slots, ok, home := composeScene()
	cfg := defaultComposeConfig()
	cfg.Quota[composeOverlap] = 1
	reqs := []composeRequest{
		{Player: 1, Class: composeQuota, Quota: composeOverlap, Target: [2]float64{50, 5}, Priority: 1},
		{Player: 0, Class: composeQuota, Quota: composeOverlap, Target: [2]float64{30, 5}, Priority: 2},
		{Player: 2, Class: composeQuota, Quota: composeOverlap, Target: [2]float64{70, 5}, Priority: 1},
	}
	out := compose(cfg, slots, ok, home, reqs)
	if out[0].Class != int(composeQuota) || out[1].Class != -1 {
		t.Fatalf("home quota: the higher priority (player 0) gets it, player 1 holds: %+v %+v", out[0], out[1])
	}
	if out[2].Class != int(composeQuota) {
		t.Fatalf("away has its own quota: %+v", out[2])
	}
}

func TestComposeOffsetClamped(t *testing.T) {
	slots, ok, home := composeScene()
	cfg := defaultComposeConfig()
	cfg.MinSpacing = 0
	reqs := []composeRequest{{Player: 0, Class: composeOffset, Target: [2]float64{20, 50}, MaxDev: 6}}
	out := compose(cfg, slots, ok, home, reqs)
	if d := math.Hypot(out[0].Target[0]-20, out[0].Target[1]-20); math.Abs(d-6) > 1e-9 {
		t.Fatalf("offset not clamped to 6 m: %v (d %.3f)", out[0].Target, d)
	}
}

func TestComposeBallJobUnclampedAndFixedInSpacing(t *testing.T) {
	slots, ok, home := composeScene()
	reqs := []composeRequest{{Player: 0, Class: composeBall, Target: [2]float64{39, 21}}}
	out := compose(defaultComposeConfig(), slots, ok, home, reqs)
	if out[0].Target != [2]float64{39, 21} {
		t.Fatalf("ball job moved: %v", out[0].Target)
	}
	d := math.Hypot(out[1].Target[0]-39, out[1].Target[1]-21)
	if d < 8-1e-6 {
		t.Fatalf("teammate not pushed to 8 m from the ball job: %.2f", d)
	}
}

func TestComposeDeterministicOrder(t *testing.T) {
	slots, ok, home := composeScene()
	a := []composeRequest{
		{Player: 0, Class: composeQuota, Quota: composeRunInBehind, Target: [2]float64{30, 30}, Priority: 1},
		{Player: 1, Class: composeQuota, Quota: composeRunInBehind, Target: [2]float64{45, 30}, Priority: 1},
	}
	b := []composeRequest{a[1], a[0]}
	cfg := defaultComposeConfig()
	cfg.Quota[composeRunInBehind] = 1
	oa, ob := compose(cfg, slots, ok, home, a), compose(cfg, slots, ok, home, b)
	for i := range oa {
		if oa[i] != ob[i] {
			t.Fatalf("input order changed the result for %d: %+v vs %+v", i, oa[i], ob[i])
		}
	}
	if oa[0].Class != int(composeQuota) || oa[1].Class != -1 {
		t.Fatalf("equal priority: lower index wins: %+v %+v", oa[0], oa[1])
	}
}
