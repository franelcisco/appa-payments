package domains

import "testing"

func TestTolerance(t *testing.T) {
	if got := Tolerance(742.2292); got != 74.22292 {
		t.Fatalf("Tolerance(742.2292) = %v, want 74.22292", got)
	}
}

func TestClassifyCharge(t *testing.T) {
	const rate = 100.0 // tolerance 10.0 VES
	cases := []struct {
		name                     string
		expectedVES, receivedVES float64
		want                     Verdict
	}{
		{"exact", 1000, 1000, Exact},
		{"short, within tolerance", 1000, 991, Exact},
		{"over, within tolerance", 1000, 1009, Exact},
		{"lower boundary", 1000, 990, Exact},
		{"upper boundary", 1000, 1010, Exact},
		{"short, past tolerance", 1000, 989, Underpaid},
		{"over, past tolerance", 1000, 1011, Overpaid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyCharge(tc.expectedVES, tc.receivedVES, rate); got != tc.want {
				t.Fatalf("ClassifyCharge(%v, %v, %v) = %v, want %v",
					tc.expectedVES, tc.receivedVES, rate, got, tc.want)
			}
		})
	}
}
