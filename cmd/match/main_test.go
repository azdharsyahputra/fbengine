package main

// Phase 2 / F: reproduction from a replay's metadata, with data checks.
// Needs the player CSV (kept outside the repo); skipped without it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fmengine/engine"
)

func needCSV(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(defaultCSV); err != nil {
		t.Skip("CSV not available:", err)
	}
}

func writeTestReplay(t *testing.T) string {
	t.Helper()
	v, _, err := play(matchSpec{Home: "Liverpool", Away: "Man City", Formation: "4-2-3-1", Seed: 7,
		Config: engine.DefaultConfig(), CSV: defaultCSV})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "r.js")
	if err := writeReplay(p, v); err != nil {
		t.Fatal(err)
	}
	return p
}

// rewrite applies a text edit to a replay file.
func rewrite(t *testing.T, path, old, new string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, old) {
		t.Fatalf("%q not in replay", old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(s, old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReproduceRoundTrip(t *testing.T) {
	needCSV(t)
	p := writeTestReplay(t)
	out := filepath.Join(t.TempDir(), "again.js")
	ok, report, err := reproduce(p, out, defaultCSV, false)
	if err != nil || !ok {
		t.Fatalf("reproduce: ok=%v err=%v report=%s", ok, err, report)
	}
	a, _, _ := readReplay(p)
	b, _, _ := readReplay(out)
	if same, why := samePayload(a, b); !same {
		t.Fatal(why)
	}
	if a.Meta.Data.LineupSHA256 == "" || a.Meta.Data.SHA256 == "" {
		t.Fatal("data hashes not recorded")
	}
}

func TestReproduceRejectsChangedData(t *testing.T) {
	needCSV(t)
	p := writeTestReplay(t)
	_, m, _ := readReplay(p)
	rewrite(t, p, m.Data.SHA256, strings.Repeat("0", 64))
	if _, _, err := reproduce(p, "", defaultCSV, false); err == nil || !strings.Contains(err.Error(), "CSV") {
		t.Fatalf("changed CSV not rejected: %v", err)
	}
	ok, report, err := reproduce(p, "", defaultCSV, true)
	if err != nil || !ok || !strings.Contains(report, "CSV") {
		t.Fatalf("--allow-data-change: ok=%v err=%v report=%s", ok, err, report)
	}
	q := writeTestReplay(t)
	_, m, _ = readReplay(q)
	rewrite(t, q, m.Data.LineupSHA256, strings.Repeat("1", 64))
	if _, _, err := reproduce(q, "", defaultCSV, false); err == nil || !strings.Contains(err.Error(), "line-ups") {
		t.Fatalf("changed line-ups not rejected: %v", err)
	}
}

func TestReproduceDetectsMismatchAndBadConfig(t *testing.T) {
	needCSV(t)
	p := writeTestReplay(t)
	rewrite(t, p, `"frames":[`, `"frames":[1,`)
	ok, report, err := reproduce(p, "", defaultCSV, false)
	if err != nil || ok || !strings.Contains(report, "frames") {
		t.Fatalf("tampered frames: ok=%v err=%v report=%s", ok, err, report)
	}
	q := writeTestReplay(t)
	rewrite(t, q, `"contain_distance":2`, `"contain_distance":3`)
	if _, _, err := reproduce(q, "", defaultCSV, false); err == nil || !strings.Contains(err.Error(), "config hash") {
		t.Fatalf("edited config not rejected: %v", err)
	}
}

func TestLineupHashDeterministic(t *testing.T) {
	needCSV(t)
	h1, _ := loadSquad(defaultCSV, "Liverpool", 1, "4-2-3-1")
	a1, _ := loadSquad(defaultCSV, "Man City", 2, "4-2-3-1")
	h2, _ := loadSquad(defaultCSV, "Liverpool", 1, "4-2-3-1")
	a2, _ := loadSquad(defaultCSV, "Man City", 2, "4-2-3-1")
	if lineupHash(h1, a1) != lineupHash(h2, a2) {
		t.Fatal("line-up hash not deterministic")
	}
	b, _ := loadSquad(defaultCSV, "Man City", 2, "4-3-3")
	if lineupHash(h1, a1) == lineupHash(h1, b) {
		t.Fatal("formation change not seen by the line-up hash")
	}
}
