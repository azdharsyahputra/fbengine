package engine

import (
	"encoding/json"
	"testing"
)

// The go4 defaults, frozen: any change to a default (or to the JSON field
// set) changes this hash and must be deliberate. History: 4c4a29ff… go4 /
// Phase 1; 56567dbd… Phase 2 added m1.contain_distance (default = go4
// behaviour, replays unchanged); 85cee7dd… go5: contain_distance 2.0;
// 83a5d924… go6: loose.contest_gap 0.25 (+ m1.contain_window, loose.* fields);
// 9f961677… B2: m2.lane_clear_radius removed (lane is spatialLaneBlockers);
// 1f84b21c… go9: press.* (organised press in the opponent's third);
// ad85f0ba… go10: m1.forward_bypassed_effort 0, bypassed_sprint_goal 25, recover_clearance;
// 2963b878… E2: progress_* moved m10 → m2, m2.to_feet (behaviour = go10);
// 749dbb80… go11: m2.duel_loss 0.5, m2.progress_final 0.003.
const goldenDefaultHash = "749dbb80eb4a06c55e634f12c937a6694414b37defb8e75dbb8a47025ab9c511"

func TestDefaultConfigGoldenHash(t *testing.T) {
	if h := DefaultConfig().Hash(); h != goldenDefaultHash {
		t.Fatalf("default config hash %s, want %s:\n%s", h, goldenDefaultHash, DefaultConfig().JSON())
	}
}

func TestConfigHashDeterministicAndSensitive(t *testing.T) {
	a, b := DefaultConfig(), DefaultConfig()
	if a.Hash() != b.Hash() {
		t.Fatal("hash not deterministic")
	}
	b.M2.ProgressFinal = 0.002
	if a.Hash() == b.Hash() {
		t.Fatal("hash ignores a field change")
	}
}

func TestConfigJSONRoundTrip(t *testing.T) {
	c := DefaultConfig()
	ApplyOverrides(&c, "M8", "M3", "dw=0.5,mm=-10,sh=1", "pf=0.002,rm=4")
	var back EngineConfig
	if err := json.Unmarshal(c.JSON(), &back); err != nil {
		t.Fatal(err)
	}
	if back != c || back.Hash() != c.Hash() {
		t.Fatalf("round trip changed the config: %+v vs %+v", back, c)
	}
}

// ApplyOverrides keeps the semantics of the former FMENGINE_* init()
// parsers: ON before OFF, int truncation for mm, f != 0 for booleans,
// silently skipped malformed pairs, reported unknown keys.
func TestApplyOverridesLegacySemantics(t *testing.T) {
	c := DefaultConfig()
	ignored := ApplyOverrides(&c, "m8", "m1, M3,bogus", "dw=0.5,mm=-10.7,sh=1,mx=0,mc=0.4,zz=3,broken,lc=x", "pf=0.002,rm=4,bn=0.1,qq=1")
	m := c.Modules
	if !m.M8Planner || m.M1Defence || m.M3Steering || !m.M2Decision {
		t.Fatalf("modules %+v", m)
	}
	if c.M2.DuelLoss != 0.5 || c.M2.PassMinimumMargin != -10 || !c.M2.Shield || c.M8.ExpandCarry || c.M8.Confidence != 0.4 {
		t.Fatalf("m2/m8 %+v %+v", c.M2, c.M8)
	}
	if c.M2.LoftedCeiling != DefaultConfig().M2.LoftedCeiling {
		t.Fatal("non-numeric value must be skipped")
	}
	if c.M2.ProgressFinal != 0.002 || c.M10.RetreatMid != 4 || c.M10.BlockLength != 0.1 {
		t.Fatalf("m10 %+v", c.M10)
	}
	want := map[string]bool{"OFF:bogus": true, "M2TUNE:zz": true, "M10:qq": true}
	if len(ignored) != len(want) {
		t.Fatalf("ignored %v", ignored)
	}
	for _, k := range ignored {
		if !want[k] {
			t.Fatalf("ignored %v", ignored)
		}
	}
	all := DefaultConfig()
	ApplyOverrides(&all, "M8", "ALL", "", "")
	if all.Modules != (ModuleConfig{}) {
		t.Fatalf("OFF=ALL after ON=M8 must clear every module, got %+v", all.Modules)
	}
}

func TestTargetOwnerNames(t *testing.T) {
	for o := TargetOwner(0); o < TargetOwner_COUNT; o++ {
		if o.String() == "?" || o.String() == "" {
			t.Fatalf("owner %d has no name", o)
		}
	}
}
