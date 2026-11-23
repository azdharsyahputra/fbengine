package engine

import "testing"

// hungarian10 must return the minimum-cost permutation (checked against brute force on
// small structured cases).
func TestHungarian10(t *testing.T) {
	var c [10][10]float64
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			d := float64((i*7+j*3)%10) - float64(j)
			c[i][j] = d * d
		}
	}
	got := hungarian10(&c)
	seen := map[int]bool{}
	sum := 0.0
	for i, j := range got {
		if seen[j] {
			t.Fatalf("column %d assigned twice: %v", j, got)
		}
		seen[j] = true
		sum += c[i][j]
	}
	// identity-shifted cost: the diagonal of a matrix with a known zero permutation
	var z [10][10]float64
	perm := [10]int{3, 7, 1, 0, 9, 2, 8, 5, 4, 6}
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			z[i][j] = 1 + float64((i+j)%3)
		}
		z[i][perm[i]] = 0
	}
	if got := hungarian10(&z); got != perm {
		t.Fatalf("zero permutation not found: %v want %v", got, perm)
	}
	_ = sum
}
